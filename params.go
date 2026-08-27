package cnb

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// addQuery 把 opts (带 json tag 的结构体, 通常是生成的 *XxxOptions) 编码为
// 查询参数附加到 path 后.
//
// 规则:
//   - nil 指针 / omitempty 零值字段不发送
//   - 切片字段用英文逗号连接 (CNB 多值参数风格, 如 labels=git,bug)
func addQuery(path string, opts any) (string, error) {
	if opts == nil {
		return path, nil
	}
	b, err := json.Marshal(opts)
	if err != nil {
		return path, fmt.Errorf("cnb: marshal query options: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil || len(m) == 0 {
		return path, nil
	}
	vals := url.Values{}
	for k, v := range m {
		vals.Set(k, encodeQueryValue(v))
	}
	return path + "?" + vals.Encode(), nil
}

func encodeQueryValue(v any) string {
	switch vv := v.(type) {
	case string:
		return vv
	case bool:
		return strconv.FormatBool(vv)
	case float64:
		return strconv.FormatFloat(vv, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(vv))
		for _, e := range vv {
			parts = append(parts, encodeQueryValue(e))
		}
		return strings.Join(parts, ",")
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", vv)
	}
}
