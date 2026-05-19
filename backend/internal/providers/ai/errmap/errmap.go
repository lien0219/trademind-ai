package errmap

import (
	"errors"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/providers/ai/compatclient"
)

// MapChatError converts compatclient errors to user-facing Chinese messages.
func MapChatError(providerLabel string, err error) error {
	if err == nil {
		return nil
	}
	if compatclient.IsTimeout(err) {
		return fmt.Errorf("%s 请求超时", providerLabel)
	}
	var he *compatclient.HTTPError
	if errors.As(err, &he) {
		msg := compatclient.APIErrorMessage(he.Body)
		switch he.StatusCode {
		case 401:
			return fmt.Errorf("API Key 无效或未授权")
		case 403:
			return fmt.Errorf("当前账号无权限访问该模型")
		case 404:
			if compatclient.IsInvalidModel(he.Body, msg) {
				return fmt.Errorf("模型不存在或无权限")
			}
			return fmt.Errorf("base_url 不可访问或接口路径错误")
		case 429:
			return fmt.Errorf("请求过于频繁或额度受限")
		case 502, 503, 504:
			return fmt.Errorf("服务商暂时不可用，请稍后重试")
		default:
			if compatclient.IsInvalidModel(he.Body, msg) {
				return fmt.Errorf("模型不存在或无权限")
			}
			if he.StatusCode >= 400 && he.StatusCode < 500 {
				if msg != "" {
					return fmt.Errorf("%s: %s", providerLabel, msg)
				}
				return fmt.Errorf("%s 返回 HTTP %d", providerLabel, he.StatusCode)
			}
			return fmt.Errorf("%s 服务异常（HTTP %d）", providerLabel, he.StatusCode)
		}
	}
	low := strings.ToLower(err.Error())
	if strings.Contains(low, "base_url empty") || strings.Contains(low, "base url") {
		return fmt.Errorf("请配置 base_url")
	}
	if strings.Contains(low, "api_key empty") {
		return fmt.Errorf("请配置 API Key")
	}
	if strings.Contains(low, "decode") || strings.Contains(low, "unmarshal") {
		return fmt.Errorf("响应格式不兼容")
	}
	if strings.Contains(low, "connection refused") || strings.Contains(low, "no such host") {
		return fmt.Errorf("base_url 不可访问")
	}
	return fmt.Errorf("%s: %w", providerLabel, err)
}
