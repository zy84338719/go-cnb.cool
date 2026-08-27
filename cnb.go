// Package cnb 是 CNB (cnb.cool, 云原生构建/代码托管平台) OpenAPI 的 Go SDK.
//
// 全量覆盖 https://api.cnb.cool 公开接口 (259 个操作, 31 个服务分组),
// 纯标准库实现, 无第三方依赖.
//
// 快速上手:
//
//	client := cnb.NewClient("your-access-token")
//	groups, resp, err := client.Organizations.ListTopGroups(ctx, nil)
//
// API 文档: https://docs.cnb.cool/zh/develops/openapi.html
package cnb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	// DefaultBaseURL 是 CNB OpenAPI 的默认服务地址.
	DefaultBaseURL = "https://api.cnb.cool/"

	// defaultAccept 与官方文档示例保持一致.
	defaultAccept = "application/json"

	defaultUserAgent = "go-cnb"
)

// Client 结构体 (含全部 Service 字段) 由 client_gen.go 生成.

// NewClient 使用访问令牌创建客户端.
//
// 令牌的创建见 https://docs.cnb.cool/zh/develops/access-token.html .
// 可选项: WithHTTPClient / WithBaseURL.
func NewClient(token string, opts ...ClientOption) (*Client, error) {
	base, err := url.Parse(DefaultBaseURL)
	if err != nil {
		return nil, err
	}
	c := &Client{
		client:  http.DefaultClient,
		BaseURL: base,
		token:   token,
	}
	for _, opt := range opts {
		opt(c)
	}
	c.initServices()
	return c, nil
}

// NewClientOrDie 同 NewClient, 参数非法时 panic (便于包级初始化).
func NewClientOrDie(token string, opts ...ClientOption) *Client {
	c, err := NewClient(token, opts...)
	if err != nil {
		panic(err)
	}
	return c
}

// initServices 由 client_gen.go 生成.

// ClientOption 配置 Client.
type ClientOption func(*Client)

// WithHTTPClient 自定义底层 *http.Client (默认 http.DefaultClient).
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		if hc != nil {
			c.client = hc
		}
	}
}

// WithBaseURL 自定义服务地址 (用于测试或私有化部署).
// 末尾斜杠可有可无.
func WithBaseURL(raw string) ClientOption {
	return func(c *Client) {
		if raw == "" {
			return
		}
		if !strings.HasSuffix(raw, "/") {
			raw += "/"
		}
		if u, err := url.Parse(raw); err == nil {
			c.BaseURL = u
		}
	}
}

// Response 包装 HTTP 响应.
//
// 对于没有 JSON schema 的接口 (文件/归档/图片/日志下载等, 生成的方法只返回
// *Response), Do 已把响应体完整读入内存:
//
//	resp, err := client.Git.GetArchive(ctx, "org/repo", "main")
//	data, _ := io.ReadAll(resp.Body)      // 顺序读一次
//	data2, _ := resp.BodyBytes()          // 随时再取缓冲副本
type Response struct {
	*http.Response

	raw []byte // 已缓冲的响应体
}

// BodyBytes 返回已缓冲的响应体副本, 可多次调用.
func (r *Response) BodyBytes() ([]byte, error) {
	if r.raw == nil {
		return nil, nil
	}
	out := make([]byte, len(r.raw))
	copy(out, r.raw)
	return out, nil
}

// NewRequest 构造请求. urlStr 为相对路径 (如 /user/groups) 时基于 BaseURL 解析;
// body 非 nil 时以 JSON 编码作为请求体.
func (c *Client) NewRequest(method, urlStr string, body any) (*http.Request, error) {
	u, err := c.resolveURL(urlStr)
	if err != nil {
		return nil, err
	}

	var buf io.ReadWriter
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("cnb: marshal request body: %w", err)
		}
		buf = bytes.NewBuffer(b)
	}

	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	accept := c.Accept
	if accept == "" {
		accept = defaultAccept
	}
	req.Header.Set("Accept", accept)
	ua := c.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	req.Header.Set("User-Agent", ua)
	return req, nil
}

func (c *Client) resolveURL(urlStr string) (*url.URL, error) {
	rel, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("cnb: parse url %q: %w", urlStr, err)
	}
	if rel.IsAbs() {
		return rel, nil
	}
	if c.BaseURL == nil {
		base, _ := url.Parse(DefaultBaseURL)
		return base.ResolveReference(rel), nil
	}
	return c.BaseURL.ResolveReference(rel), nil
}

// Do 执行请求.
//
// v 非 nil 时把响应体 JSON 解码到 v; v 为 nil 时响应体被完整缓冲,
// 通过 resp.Body 暴露. 非 2xx/3xx 状态码返回 *ErrorResponse.
func (c *Client) Do(ctx context.Context, req *http.Request, v any) (*Response, error) {
	req = req.WithContext(ctx)

	if c.token != "" && req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	httpResp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	resp := &Response{Response: httpResp}
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return resp, fmt.Errorf("cnb: read response body: %w", err)
	}
	resp.raw = body
	// 把已读入的 body 挂回 Body, 调用方仍可按流读一次.
	httpResp.Body = io.NopCloser(bytes.NewReader(body))

	if code := httpResp.StatusCode; code < 200 || code > 399 {
		return resp, newErrorResponse(resp, body)
	}

	if v != nil && len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, v); err != nil {
			return resp, fmt.Errorf("cnb: decode response (status %d): %w", httpResp.StatusCode, err)
		}
	}
	return resp, nil
}

// Get 构造并执行一个简单 GET (给脚本/调试用; 常规场景请用各 Service 方法).
func (c *Client) Get(ctx context.Context, urlStr string, v any) (*Response, error) {
	req, err := c.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req, v)
}
