package cnb

import (
	"encoding/base64"
	"errors"
	"fmt"
)

// ErrContentNotDecodable 当 Content 不是可解码的文件内容 (如目录/lfs 指针) 时返回.
var ErrContentNotDecodable = errors.New("cnb: 该内容不是 base64 文件内容 (type 应为 blob/text)")

// DecodedContent 返回文件内容的 base64 解码结果.
//
// CNB 的 Git GetContent 接口对文件 (type=blob) 返回 base64 编码的内容,
// 本方法省去手工解码:
//
//	content, _, _ := client.Git.GetContent(ctx, "org/repo", "main.go", nil)
//	src, err := content.DecodedContent() // []byte 源码内容
//
// 目录 (type=tree) 与 LFS 指针请分别使用 Entries / LfsDownloadUrl.
func (c *Content) DecodedContent() ([]byte, error) {
	if c.Type != "blob" && c.Encoding != "base64" {
		return nil, ErrContentNotDecodable
	}
	if c.Content == "" {
		return nil, nil
	}
	if b, err := base64.StdEncoding.DecodeString(c.Content); err == nil {
		return b, nil
	}
	// 宽容处理无填充 base64
	b, err := base64.RawStdEncoding.DecodeString(c.Content)
	if err != nil {
		return nil, fmt.Errorf("cnb: base64 解码失败: %w", err)
	}
	return b, nil
}
