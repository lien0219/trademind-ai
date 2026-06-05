package taskcenter

import (
	"net/url"
	"strings"
)

func pinduoduoURLTypeLabel(sourceURL string) string {
	u, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	path := strings.ToLower(u.Path)
	if host == "pifa.pinduoduo.com" || strings.HasSuffix(host, ".pifa.pinduoduo.com") {
		if strings.Contains(path, "goods") {
			return "拼多多批发页"
		}
	}
	if strings.Contains(host, "pinduoduo.com") || strings.Contains(host, "yangkeduo.com") {
		if strings.Contains(path, "goods") || strings.Contains(u.RawQuery, "goods_id=") {
			return "普通商品页"
		}
		if strings.Contains(path, "login") || strings.Contains(path, "passport") {
			return "登录页"
		}
		if strings.Contains(path, "app") || strings.Contains(path, "download") {
			return "App 引导页"
		}
	}
	return ""
}

func accessStatusLabelFromFailure(category, errMsg string) string {
	msg := strings.ToLower(strings.TrimSpace(errMsg))
	cat := strings.TrimSpace(strings.ToLower(category))
	if strings.Contains(msg, "open.weixin.qq.com") || strings.Contains(msg, "wechat_auth") {
		return "需要微信扫码授权"
	}
	if strings.Contains(msg, "app_redirect") || strings.Contains(msg, "app 引导") {
		return "App 引导页"
	}
	if cat == "login_required" || strings.Contains(msg, "login_required") {
		return "需要登录"
	}
	if strings.Contains(msg, "verify") || strings.Contains(msg, "blocked") || strings.Contains(msg, "captcha") {
		return "需要验证"
	}
	if strings.Contains(msg, "public") {
		return "公开可访问"
	}
	return ""
}

func collectFailureContextExtras(sourceURL, errMsg, failureCategory, classifierSuggest string) (urlType, accessStatus, suggested string) {
	src := strings.TrimSpace(strings.ToLower(sourceURL))
	if strings.Contains(src, "pinduoduo") || strings.Contains(src, "yangkeduo") {
		urlType = pinduoduoURLTypeLabel(sourceURL)
		if urlType == "" {
			urlType = "未识别"
		}
	}
	if strings.Contains(src, "taobao") || strings.Contains(src, "tmall") {
		urlType = taobaoTmallURLTypeLabel(sourceURL)
		if urlType == "" {
			urlType = "淘宝/天猫商品页"
		}
	}
	accessStatus = accessStatusLabelFromFailure(failureCategory, errMsg)
	if accessStatus == "" && strings.Contains(strings.ToLower(errMsg), "login") {
		accessStatus = "需要登录"
	}
	if accessStatus == "" {
		accessStatus = "—"
	}
	suggested = strings.TrimSpace(classifierSuggest)
	if strings.Contains(strings.ToLower(errMsg), "open.weixin.qq.com") ||
		accessStatus == "需要微信扫码授权" {
		suggested = "请打开拼多多采集浏览器，在弹出的微信授权页面完成扫码登录后，再重试采集任务。"
	} else if urlType == "拼多多批发页" && accessStatus == "需要登录" {
		suggested = "请打开采集浏览器登录拼多多后重试，或换用普通商品详情页链接。"
	} else if urlType == "淘宝商品页" && accessStatus == "需要登录" {
		suggested = "请打开淘宝/天猫采集浏览器完成登录后重试采集任务。"
	} else if accessStatus == "需要验证" && strings.Contains(strings.ToLower(errMsg), "taobao") {
		suggested = "请在淘宝/天猫采集浏览器中手动完成安全验证后重试。"
	}
	return urlType, accessStatus, suggested
}

func taobaoTmallURLTypeLabel(sourceURL string) string {
	u, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "item.taobao.com":
		return "淘宝商品页"
	case "detail.tmall.com", "detail.tmall.hk":
		return "天猫商品页"
	case "world.taobao.com":
		return "淘宝全球购商品页"
	default:
		if strings.Contains(host, "taobao") {
			return "淘宝商品页"
		}
		if strings.Contains(host, "tmall") {
			return "天猫商品页"
		}
	}
	return ""
}
