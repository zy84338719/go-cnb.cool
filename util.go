package cnb

import (
	"fmt"
	"net/url"
	"strings"
)

// Ptr 返回 v 的指针, 用于构造请求表单的可选字段:
//
//	issue, _, err := client.Issues.UpdateIssue(ctx, "org/repo", 1,
//	    cnb.PatchIssueForm{Title: cnb.Ptr("新标题")})
func Ptr[T any](v T) *T {
	return &v
}

// escapePath 对路径模板逐参数做 URL 转义后拼接.
//
// 参数中的 "/" 保留 (如 repo="org/repo"、file_path="a/b/c.txt"),
// 每一段内的特殊字符才转义 —— 与 CNB 路由规则一致.
func escapePath(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	escaped := make([]any, len(args))
	for i, a := range args {
		if s, ok := a.(string); ok {
			escaped[i] = escapePathSegments(s)
		} else {
			escaped[i] = a
		}
	}
	return fmt.Sprintf(format, escaped...)
}

func escapePathSegments(s string) string {
	segs := strings.Split(s, "/")
	for i, seg := range segs {
		segs[i] = url.PathEscape(seg)
	}
	return strings.Join(segs, "/")
}
