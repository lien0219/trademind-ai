package collect

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/pkg/collectdomain"
)

func validateTaobaoTmallCollectURL(urlStr string) error {
	switch collectdomain.ClassifyTaobaoTmallURL(urlStr) {
	case "product_detail":
		return nil
	case "unsupported_taobao":
		return fmt.Errorf("UNSUPPORTED_TAOBAO_URL:当前链接不是标准淘宝/天猫商品详情页，请复制商品详情页链接后重试")
	default:
		return fmt.Errorf("INVALID_URL:请输入淘宝/天猫商品详情页链接（item.taobao.com、detail.tmall.com 等）")
	}
}
