package api

import (
	"errors"
	"net/http"
)

// fail 统一记录错误并返回 500。
func (a *API) fail(w http.ResponseWriter, message string, err error) {
	if err == nil {
		err = errors.New(message)
	}
	a.log.Error(message, "error", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", message+"："+err.Error(), nil)
}
