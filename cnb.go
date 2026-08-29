// Client 核心逻辑 (NewClient / NewRequest / Do).
// 包文档见 doc.go.
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

// WithRetry 启用自动重试 (默认关闭).
//
// 重试策略 (见 retry.go):
//   - 429/502/503/504: 所有方法重试 (429 优先遵守 Retry-After)
//   - 网络层错误 (连接失败/超时): 仅幂等的 GET/HEAD 重试
//   - 其余 (含 4xx 业务错误): 不重试
//
// 退避为指数级 (200ms 起, 上限 3s) 附加抖动. maxRetries 建议 2~4.
// 请求体由 SDK 构造, 可自动重放; 通过 NewRequest 自带 body 的请求若不可重放则跳过重试.
func WithRetry(maxRetries int) ClientOption {
	return func(c *Client) {
		if maxRetries < 0 {
			maxRetries = 0
		}
		c.retryMax = maxRetries
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
	base := c.BaseURL
	if base == nil {
		base, _ = url.Parse(DefaultBaseURL)
	}
	u := *base.ResolveReference(rel)
	// net/url 语义: 以 "/" 开头的相对路径会丢弃 base 的 path 前缀;
	// 对带路径前缀的网关 (如 https://host/api/) 需要手动补回.
	if strings.HasPrefix(rel.Path, "/") && base.Path != "" && base.Path != "/" {
		u.Path = strings.TrimSuffix(base.Path, "/") + rel.Path
	}
	return &u, nil
}

// Do 执行请求, 按 WithRetry 配置自动重试.
//
// v 非 nil 时把响应体 JSON 解码到 v; v 为 nil 时响应体被完整缓冲,
// 通过 resp.Body 暴露. 非 2xx/3xx 状态码返回 *ErrorResponse.
func (c *Client) Do(ctx context.Context, req *http.Request, v any) (*Response, error) {
	resp, err := c.doOnce(ctx, req, v)
	for attempt := 1; c.retryMax > 0 && attempt <= c.retryMax; attempt++ {
		if !c.shouldRetry(req, resp, err) {
			break
		}
		if waitErr := c.sleepBackoff(ctx, resp, attempt); waitErr != nil {
			return resp, err
		}
		if req.GetBody != nil {
			b, gbErr := req.GetBody()
			if gbErr != nil {
				return resp, err
			}
			req.Body = b
		} else if req.Body != nil {
			break // 请求体不可重放
		}
		resp, err = c.doOnce(ctx, req, v)
	}
	return resp, err
}

// doOnce 执行单次请求 (无重试).
func (c *Client) doOnce(ctx context.Context, req *http.Request, v any) (*Response, error) {
	req = req.WithContext(ctx)

	if c.token != "" && req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	httpResp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = httpResp.Body.Close() }()

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
		// SSE 流式响应: 不能按 JSON 解码 (见 AIService.AiChatCompletionsStream)
		if strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream") {
			return resp, fmt.Errorf("cnb: 流式 (text/event-stream) 响应, 请使用流式方法读取 resp.Body")
		}
		if err := json.Unmarshal(body, v); err != nil {
			// 裸文本兜底: 目标为 string 且响应体不是 JSON 时 (如 GetRaw 按需返回纯文本),
			// 直接采用原文
			if ps, ok := v.(*string); ok && !json.Valid(body) {
				*ps = string(body)
				return resp, nil
			}
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
