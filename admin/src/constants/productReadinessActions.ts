export type ProductReadinessAction = {
  actionKey: string;
  label: string;
  tab?: string;
  section?: string;
  href?: string;
};

function action(input: ProductReadinessAction): ProductReadinessAction {
  return input;
}

export function getProductReadinessAction(code?: string | null): ProductReadinessAction | null {
  const c = (code || '').trim().toLowerCase();
  if (!c) return null;

  if (c === 'product.ai_title_missing') {
    return action({ actionKey: 'generate_ai_title', label: '去 AI 标题', tab: 'ai' });
  }
  if (c.startsWith('product.title')) {
    return action({ actionKey: 'edit_title', label: '去基础信息', tab: 'basic', section: 'title' });
  }
  if (c.startsWith('product.description')) {
    return action({ actionKey: 'edit_description', label: '去基础信息', tab: 'basic', section: 'description' });
  }
  if (c === 'product.currency_missing' || c === 'product.archived') {
    return action({ actionKey: 'review_basic_info', label: '去基础信息', tab: 'basic', section: 'title' });
  }

  if (
    c.startsWith('collect.warning') ||
    c.startsWith('collect.taobao_tmall.') ||
    c.startsWith('collect.pinduoduo.') ||
    c.endsWith('.attributes_missing') ||
    c.endsWith('.attributes_empty') ||
    c.endsWith('.stock_unknown') ||
    c.endsWith('.detail_images_incomplete')
  ) {
    return action({ actionKey: 'review_collect', label: '检查采集结果', tab: 'basic', section: 'collect-review' });
  }
  if (
    c.endsWith('.detail_images_missing') ||
    c === 'collect.taobao_tmall.external_image' ||
    c.endsWith('.external_image')
  ) {
    return action({ actionKey: 'review_images', label: '去图片管理', tab: 'images', section: 'image-list' });
  }
  if (c.endsWith('.price_missing') || c.endsWith('.sku_incomplete')) {
    return action({ actionKey: 'review_pricing', label: '去商品规格', tab: 'skus', section: 'pricing' });
  }

  if (c.startsWith('image.')) {
    return action({ actionKey: 'fix_images', label: '去图片管理', tab: 'images', section: 'image-list' });
  }
  if (c.startsWith('pricing.') || c.startsWith('sku.')) {
    return action({ actionKey: 'fix_pricing', label: '去商品规格', tab: 'skus', section: 'pricing' });
  }
  if (c.startsWith('inventory.')) {
    return action({ actionKey: 'review_inventory', label: '去库存', tab: 'inventory', section: 'local-skus' });
  }

  if (
    c === 'category_required' ||
    c === 'platform_attributes_required' ||
    c === 'publish_config_missing' ||
    c === 'platform_not_supported'
  ) {
    return action({ actionKey: 'fix_publish_config', label: '去刊登配置', tab: 'publish', section: 'publish-config' });
  }

  if (
    c === 'platform.shop_required' ||
    c === 'platform.shop_not_found' ||
    c === 'platform.shop_inactive' ||
    c === 'platform.shop_not_authorized' ||
    c === 'platform.shop_token_missing' ||
    c === 'douyin_shop_not_authorized'
  ) {
    return action({ actionKey: 'manage_shops', label: '店铺管理', href: '/shops' });
  }
  if (c.startsWith('platform.') || c.startsWith('douyin_')) {
    return action({ actionKey: 'fix_publish_config', label: '去刊登配置', tab: 'publish', section: 'publish-config' });
  }

  return null;
}
