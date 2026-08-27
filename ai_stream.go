package cnb

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strings"
)

// AiChatCompletionsStream 发起流式 AI 对话 (SSE), 是 AiChatCompletions 的流式版本.
//
// CNB 部分 AI 模型仅支持流式 (AiChatCompletionsReq.Stream), 此时须用本方法:
//
//	resp, err := client.AI.AiChatCompletionsStream(ctx, "org/repo", cnb.AiChatCompletionsReq{
//	    Model:    cnb.Ptr("模型名"),
//	    Stream:   cnb.Ptr(true),
//	    Messages: []cnb.Message{{Role: cnb.Ptr("user"), Content: cnb.Ptr("你好")}},
//	})
//	err = cnb.ScanSSE(resp.Body, func(ev cnb.SSEEvent) error {
//	    // ev.Data 为一条 JSON 增量, 结构同 AiChatCompletionsChoice; "[DONE]" 表示结束
//	    return nil
//	})
//
// CNB API: POST /{repo}/-/ai/chat/completions
func (s *AIService) AiChatCompletionsStream(ctx context.Context, repo string, body AiChatCompletionsReq) (*Response, error) {
	u := escapePath("/%s/-/ai/chat/completions", repo)
	req, err := s.client.NewRequest(http.MethodPost, u, body)
	if err != nil {
		return nil, err
	}
	return s.client.Do(ctx, req, nil)
}

// SSEEvent 表示 SSE 流中的单个事件.
type SSEEvent struct {
	Event string // 事件类型 (可为空)
	Data  string // data 载荷, 通常为 JSON 文本
}

// ScanSSE 从 r 中逐个读取 SSE 事件, 对每个事件调用 fn; fn 返回错误时提前终止.
// 流读取完毕返回 nil, 否则返回 fn 的错误或读取错误.
func ScanSSE(r io.Reader, fn func(SSEEvent) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var ev SSEEvent
	flush := func() error {
		if ev.Data != "" || ev.Event != "" {
			if err := fn(ev); err != nil {
				return err
			}
		}
		ev = SSEEvent{}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if err := flush(); err != nil {
				return err
			}
		case strings.HasPrefix(line, "data:"):
			ev.Data += strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
		case strings.HasPrefix(line, "event:"):
			ev.Event = strings.TrimPrefix(strings.TrimPrefix(line, "event:"), " ")
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}
