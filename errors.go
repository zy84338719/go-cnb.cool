package cnb

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// ErrorResponse 是 CNB API 返回非 2xx/3xx 状态码时的错误.
//
// CNB 错误体结构: {"errcode": 404, "errmsg": "not found", "errparam": {...}}.
// 非 JSON 错误体时原文保存在 Message 字段.
type ErrorResponse struct {
	Response *http.Response // 对应的 HTTP 响应

	ErrCode  int            `json:"errcode"` // CNB 业务错误码
	ErrMsg   string         `json:"errmsg"`  // 错误信息
	ErrParam map[string]any `json:"errparam"`

	// RawBody 是原始错误响应体 (已读入内存, 便于排查).
	RawBody string
}

func (e *ErrorResponse) Error() string {
	if e.ErrMsg != "" || e.ErrCode != 0 {
		return fmt.Sprintf("cnb: API error %d (errcode=%d): %s",
			e.Response.StatusCode, e.ErrCode, e.ErrMsg)
	}
	body := e.RawBody
	if len(body) > 256 {
		body = body[:256] + "..."
	}
	return fmt.Sprintf("cnb: API error %d: %s", e.Response.StatusCode, body)
}

// IsNotFound 判断是否为资源不存在 (HTTP 404 或 errcode 404).
func (e *ErrorResponse) IsNotFound() bool {
	return e.Response != nil && e.Response.StatusCode == http.StatusNotFound
}

// IsUnauthorized 判断是否为令牌无效/未登录 (HTTP 401).
func (e *ErrorResponse) IsUnauthorized() bool {
	return e.Response != nil && e.Response.StatusCode == http.StatusUnauthorized
}

// IsForbidden 判断是否为无权限 (HTTP 403).
func (e *ErrorResponse) IsForbidden() bool {
	return e.Response != nil && e.Response.StatusCode == http.StatusForbidden
}

func newErrorResponse(resp *Response, body []byte) *ErrorResponse {
	errResp := &ErrorResponse{
		Response: resp.Response,
		RawBody:  string(body),
	}
	if len(body) > 0 {
		_ = json.Unmarshal(body, errResp)
	}
	return errResp
}

// AsErrorResponse 是 errors.As(err, &*ErrorResponse) 的便捷封装.
func AsErrorResponse(err error, target **ErrorResponse) bool {
	return errors.As(err, target)
}
