package cnb

import (
	"context"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// retryableStatus 可安全重试的状态码:
// 429 被限流 (请求未执行), 502/503/504 网关错误 (上游大概率未执行).
var retryableStatus = map[int]bool{
	http.StatusTooManyRequests:    true,
	http.StatusBadGateway:         true,
	http.StatusServiceUnavailable: true,
	http.StatusGatewayTimeout:     true,
}

// shouldRetry 判定一次请求是否应重试:
//   - 网络层错误 (resp == nil): 仅幂等方法 GET/HEAD
//   - 429/502/503/504: 所有方法
//   - 其余 (含 4xx 业务错误、2xx 成功、解码错误): 不重试
func (c *Client) shouldRetry(req *http.Request, resp *Response, err error) bool {
	if resp == nil {
		if err == nil {
			return false
		}
		return req.Method == http.MethodGet || req.Method == http.MethodHead
	}
	return retryableStatus[resp.StatusCode]
}

// sleepBackoff 指数退避: 200ms << attempt, 上限 3s, 附加 0~100ms 抖动;
// 429 且带 Retry-After (秒) 时优先遵守 (上限 30s).
func (c *Client) sleepBackoff(ctx context.Context, resp *Response, attempt int) error {
	var d time.Duration
	if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
		if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs > 0 {
			d = time.Duration(secs) * time.Second
			if d > 30*time.Second {
				d = 30 * time.Second
			}
		}
	}
	if d == 0 {
		d = 200 * time.Millisecond << attempt
		if d > 3*time.Second {
			d = 3 * time.Second
		}
		d += time.Duration(rand.Int63n(int64(100 * time.Millisecond))) //nolint:gosec // 抖动无需密码学安全
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
