// Package filemanage 对接 nwbd-file-manage 的内部接口。
//
// 任务产出不再进本项目的 MinIO：上传时按合同审批实例找到文件管理里的项目，
// 把文件塞进对应的阶段目录；本系统只保留文件 id、文件名、大小、目录。
package filemanage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrDisabled 表示没有配置内部令牌，归档功能不可用。
var ErrDisabled = errors.New("未配置文件管理内部令牌")

// Folder 是文件管理项目下的一级目录。
type Folder struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// Project 是文件管理里的项目。
type Project struct {
	ID          string   `json:"id"`
	InstanceID  string   `json:"instance_id"`
	DirName     string   `json:"dir_name"`
	Status      string   `json:"status"`
	FolderCount int      `json:"folder_count"`
	Folders     []Folder `json:"folders"`
}

// UploadResult 是内部上传成功后的文件信息。
type UploadResult struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	Folder     string    `json:"folder"`
	Path       string    `json:"path"`
	UploadedBy string    `json:"uploaded_by"`
	CreatedAt  time.Time `json:"created_at"`
}

// APIError 是文件管理返回的业务错误。
type APIError struct {
	Status        int
	Code          string
	Message       string
	LastErr       string
	ProjectStatus string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("文件管理返回 %d", e.Status)
}

// Client 是内部接口客户端。
type Client struct {
	base  string
	token string
	http  *http.Client
	log   *slog.Logger
}

// New 构造客户端；token 为空时视为功能关闭。
func New(base, token string, timeout time.Duration, log *slog.Logger) *Client {
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	return &Client{
		base:  strings.TrimRight(base, "/"),
		token: strings.TrimSpace(token),
		http:  &http.Client{Timeout: timeout},
		log:   log,
	}
}

// Enabled 判断是否配置了内部令牌。
func (c *Client) Enabled() bool { return c.token != "" }

// ProjectByInstance 按合同审批实例号查项目与一级目录。
func (c *Client) ProjectByInstance(ctx context.Context, instanceID string) (*Project, error) {
	var out Project
	if err := c.getJSON(ctx, "/internal/projects/by-instance/"+url.PathEscape(instanceID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Upload 把文件写进指定一级目录；文件名由文件管理统一加时间戳。
func (c *Client) Upload(ctx context.Context, projectID, folder, filename, uploadedBy string, body io.Reader) (*UploadResult, error) {
	if !c.Enabled() {
		return nil, ErrDisabled
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		defer func() { _ = pw.Close() }()
		if err := mw.WriteField("folder", folder); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if strings.TrimSpace(uploadedBy) != "" {
			if err := mw.WriteField("uploaded_by", uploadedBy); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		}
		part, err := mw.CreateFormFile("file", filename)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, body); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if err := mw.Close(); err != nil {
			_ = pw.CloseWithError(err)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.base+"/internal/projects/"+url.PathEscape(projectID)+"/files", pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Token", c.token)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	var out UploadResult
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Download 打开文件流；返回内容类型、文件名与大小。
func (c *Client) Download(ctx context.Context, fileID string) (io.ReadCloser, string, int64, error) {
	if !c.Enabled() {
		return nil, "", 0, ErrDisabled
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.base+"/internal/files/"+url.PathEscape(fileID)+"/download", nil)
	if err != nil {
		return nil, "", 0, err
	}
	req.Header.Set("X-Internal-Token", c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, "", 0, c.decodeError(resp)
	}
	return resp.Body, resp.Header.Get("Content-Type"), resp.ContentLength, nil
}

// Delete 删除本系统上传的文件（回滚用）；404 视为已清理。
func (c *Client) Delete(ctx context.Context, fileID string) error {
	if !c.Enabled() {
		return ErrDisabled
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		c.base+"/internal/files/"+url.PathEscape(fileID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Internal-Token", c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return c.decodeError(resp)
}

func (c *Client) getJSON(ctx context.Context, path string, target any) error {
	if !c.Enabled() {
		return ErrDisabled
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Internal-Token", c.token)
	return c.do(req, target)
}

type envelope struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) do(req *http.Request, target any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.decodeError(resp)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("文件管理返回格式异常: %w", err)
	}
	if env.Code != "OK" {
		return &APIError{Status: resp.StatusCode, Code: env.Code, Message: env.Message}
	}
	if target == nil || len(env.Data) == 0 {
		return nil
	}
	return json.Unmarshal(env.Data, target)
}

func (c *Client) decodeError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	apiErr := &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(raw))}
	var env struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Status  string `json:"status"`
			LastErr string `json:"last_error"`
		} `json:"data"`
	}
	if json.Unmarshal(bytes.TrimSpace(raw), &env) == nil && env.Code != "" {
		apiErr.Code = env.Code
		if strings.TrimSpace(env.Message) != "" {
			apiErr.Message = env.Message
		}
		apiErr.ProjectStatus = env.Data.Status
		apiErr.LastErr = env.Data.LastErr
	}
	return apiErr
}
