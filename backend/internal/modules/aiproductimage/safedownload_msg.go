package aiproductimage

import (
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/safedownload"
)

func safeDownloadUserMessage(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, safedownload.ErrResponseTooLarge):
		return "图片文件过大，请压缩后再处理。"
	case strings.Contains(msg, safedownload.ErrInvalidContentType), strings.Contains(msg, safedownload.ErrImageDecodeFailed):
		return "图片格式暂不支持，请更换为 JPG、PNG 或 WebP。"
	case strings.Contains(msg, safedownload.ErrSchemeNotAllowed),
		strings.Contains(msg, safedownload.ErrPrivateHost),
		strings.Contains(msg, safedownload.ErrPrivateIP):
		return "图片链接不安全或无效。"
	}
	if s := strings.TrimSpace(msg); s != "" {
		return "图片无法访问，请检查图片链接是否有效。"
	}
	return "图片无法访问，请检查图片链接是否有效。"
}
