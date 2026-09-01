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
//
// 按 SSE 规范: 同一事件的多行 data 以 "\n" 连接; 空行是事件分隔符.
func ScanSSE(r io.Reader, fn func(SSEEvent) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var (
		event      string
		dataLines  []string
		hasContent bool
	)
	flush := func() error {
		if hasContent {
			if err := fn(SSEEvent{Event: event, Data: strings.Join(dataLines, "\n")}); err != nil {
				return err
			}
		}
		event, dataLines, hasContent = "", nil, false
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if err := flush(); err != nil {
				return err
			}
		case strings.HasPrefix(line, ":"):
			// 注释行, 忽略
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			hasContent = true
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimPrefix(strings.TrimPrefix(line, "event:"), " ")
			hasContent = true
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}
