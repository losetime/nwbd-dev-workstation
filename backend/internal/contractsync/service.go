// Package contractsync 把 03 销售合同签订的审批结果落到项目台账。
package contractsync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"nwbd-dev-workstation/internal/dingtalk"
	"nwbd-dev-workstation/internal/domain"
	"nwbd-dev-workstation/internal/store"
)

// Apply 处理一条合同签订审批实例：通过则建项目，撤销/拒绝则打标。
//
// 幂等：同一实例或同一项目编号只会建一次；重复调用只补关联。
func Apply(ctx context.Context, st *store.Store, approval *dingtalk.ApprovalClient, log *slog.Logger, instanceID string) error {
	inst, err := approval.GetInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("拉取合同审批详情失败: %w", err)
	}
	status := strings.ToUpper(inst.Status)

	switch status {
	case "TERMINATED":
		if id, err := st.MarkContractStatus(ctx, instanceID, "已撤销"); err != nil {
			return err
		} else if id != "" {
			log.Info("合同已撤销，项目已打标", "project", id, "instance", instanceID)
		}
		return nil
	case "COMPLETED":
		if strings.EqualFold(inst.Result, "refuse") {
			if id, err := st.MarkContractStatus(ctx, instanceID, "已拒绝"); err != nil {
				return err
			} else if id != "" {
				log.Info("合同被拒绝，项目已打标", "project", id, "instance", instanceID)
			}
			return nil
		}
	default:
		return nil
	}

	if existing, err := st.ProjectByContractInstance(ctx, instanceID); err != nil {
		return err
	} else if existing != nil {
		return nil
	}

	code := strings.TrimSpace(inst.FormValueByName("项目代号"))
	name := strings.TrimSpace(inst.FormValueByName("合同名称"))
	customer := strings.TrimSpace(inst.FormValueByName("甲方单位"))
	amount := parseMoney(inst.FormValueByName("合同金额（元）"))
	managerID := contractManagerID(inst)

	if code != "" {
		if existing, err := st.ProjectByCode(ctx, code); err == nil && existing != nil {
			if err := st.LinkContract(ctx, existing.ID, instanceID, inst.BusinessID); err != nil {
				return err
			}
			log.Info("合同已关联到已有项目", "project", existing.ID, "code", code)
			return nil
		}
	}
	if name == "" {
		name = "未命名合同-" + shortID(inst.BusinessID, instanceID)
	}

	managers := []string{}
	createdBy := ""
	if managerID != "" {
		if person, err := st.GetPersonByDingID(ctx, managerID); err == nil && person != nil {
			managers = append(managers, person.ID)
			createdBy = person.ID
		}
	}

	project, err := st.CreateProject(ctx, store.ProjectInput{
		Code: code, Name: name, Type: "合同实施项目", Customer: customer,
		ContractAmount: amount, ManagerIDs: managers, CreatedBy: createdBy,
		ContractInstanceID: instanceID, ContractBusinessID: inst.BusinessID,
	}, domain.StageTemplates["合同实施项目"])
	if err != nil {
		return err
	}
	log.Info("合同审批通过，已自动创建项目", "project", project.ID, "code", code, "name", name)
	return nil
}

func contractManagerID(inst *dingtalk.ProcessInstance) string {
	for _, f := range inst.FormValues {
		if !strings.Contains(f.Name, "项目经理") || f.ExtValue == "" {
			continue
		}
		var items []struct {
			EmplID string `json:"emplId"`
			Name   string `json:"name"`
		}
		if err := json.Unmarshal([]byte(f.ExtValue), &items); err == nil && len(items) > 0 {
			return items[0].EmplID
		}
	}
	return ""
}

func parseMoney(raw string) *float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	return &value
}

func shortID(values ...string) string {
	for _, v := range values {
		if v != "" {
			if len(v) > 6 {
				return v[len(v)-6:]
			}
			return v
		}
	}
	return "unknown"
}
