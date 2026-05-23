import type { AppConfigFieldDTO } from '@/services/shops';

/** 刊登预设字段中文标签（按 field.name） */
export const PLATFORM_PUBLISH_FIELD_LABEL: Record<string, string> = {
  default_category_id: '默认类目 ID',
  default_brand_id: '默认品牌 ID',
  default_browse_node_id: '默认 Browse Node ID',
  shipping_template_id: '物流模板 ID',
  logistic_channel_id: '物流渠道 ID',
  warehouse_id: '默认仓库 ID',
  default_weight: '默认包裹重量',
  default_length: '默认长度',
  default_width: '默认宽度',
  default_height: '默认高度',
  package_weight: '包裹重量',
  package_length: '包裹长度',
  package_width: '包裹宽度',
  package_height: '包裹高度',
  weight_unit: '重量单位',
  dimension_unit: '尺寸单位',
  publish_as_draft: '默认发布为草稿',
  product_status: '商品状态',
  size_chart_id: '尺码表 ID',
  return_policy_id: '退货/售后模板 ID',
  condition: '商品成色',
  days_to_ship: '发货天数',
  variation_tier_name: '规格维度名称',
  warranty_type: '保修类型',
  warranty_period: '保修期',
  delivery_option: '配送选项',
  marketplace_id: 'Marketplace ID',
  product_type: 'Product Type',
  merchant_shipping_group: 'Merchant Shipping Group',
  condition_type: 'Condition Type',
  fulfillment_channel: 'Fulfillment Channel',
  brand: 'Brand',
  manufacturer: 'Manufacturer',
  issue_locale: 'Issue Locale',
  requirements: 'Requirements',
  amazon_attributes: 'Amazon Attributes JSON',
  default_vendor: '默认 Vendor',
  default_product_type: '默认 Product Type',
  default_tags: '默认 Tags',
  default_status: '商品状态字符串',
  default_business_policies_hint: '政策包/策略说明',
  shipping_hint: '物流/履约说明',
  endpoint_hint: '对接 Endpoint',
  default_payload_hint: 'JSON 模板说明',
};

/** 字段说明（优先于后端 help） */
export const PLATFORM_PUBLISH_FIELD_HELP: Record<string, string> = {
  default_category_id: '在平台卖家后台或 Partner Center 查询类目 ID，刊登时未单独指定则使用此默认值',
  default_brand_id: '无品牌时可留空或填平台规定的「无品牌」标识',
  shipping_template_id: 'TikTok 物流模板 ID，可在 Seller Center 物流设置中查看',
  logistic_channel_id: 'Shopee 物流渠道 ID，可在 Open Platform 文档或卖家中心查询',
  warehouse_id: '多仓店铺需指定默认发货仓；单次刊登可在商品详情覆盖',
  default_weight: '与平台度量单位保持一致，仅作草稿默认值',
  default_length: '包裹长度，单位见平台要求（如 cm）',
  default_width: '包裹宽度，单位见平台要求（如 cm）',
  default_height: '包裹高度，单位见平台要求（如 cm）',
  package_weight: 'Lazada 包裹重量，单位 kg',
  package_length: 'Lazada 包裹长度，单位 cm',
  package_width: 'Lazada 包裹宽度，单位 cm',
  package_height: 'Lazada 包裹高度，单位 cm',
  weight_unit: '填写默认重量时使用，按 Product Type Definitions 要求填写',
  dimension_unit: '填写默认长宽高时使用，按 Product Type Definitions 要求填写',
  product_status: '平台定义的状态取值，以文本形式填写',
  size_chart_id: '服饰等类目可能需要尺码表，可在 TikTok 后台创建后填入 ID',
  return_policy_id: '售后/退货政策模板 ID，可选',
  condition: '商品新旧程度，可在下拉中选择或自行填写',
  days_to_ship: '承诺发货天数，留空使用平台或店铺默认',
  variation_tier_name: '多 SKU 未单独配置规格维度时使用的默认名称（如 Color、Size）',
  warranty_type: '保修类型说明，按 Lazada 类目要求填写',
  warranty_period: '保修期描述，如「1 年」',
  delivery_option: 'Lazada 配送选项的平台取值，必填',
  marketplace_id: 'Amazon 目标站点 Marketplace ID，如 ATVPDKIKX0DER',
  product_type: 'Amazon Product Type，决定 Listing 必填属性结构',
  default_browse_node_id: 'Amazon 类目 Browse Node ID，可在卖家后台查询',
  merchant_shipping_group: 'Amazon 运费模板组名称或 ID',
  condition_type: 'Amazon 商品成色类型，如 New',
  fulfillment_channel: '履约渠道，如 MFN（自发货）或 FBA',
  brand: 'Amazon Listing 必填品牌字段',
  manufacturer: 'Amazon Listing 必填制造商字段',
  issue_locale: 'Amazon Issue Locale，可选',
  requirements: '如 LISTING；留空使用默认 LISTING',
  amazon_attributes: '按 Product Type Definitions 补充必填 attributes；商品详情 options 中同名字段优先',
  default_vendor: 'Shopify 商品 Vendor 默认值',
  default_product_type: 'Shopify Product Type 默认值',
  default_tags: 'Shopify 商品 Tags，多个用逗号分隔',
  default_status: 'WooCommerce 商品状态，如 draft 或 publish',
  default_business_policies_hint: 'eBay 政策包或刊登策略的文字摘录，便于团队查阅',
  shipping_hint: '物流/履约相关说明摘录，对接启用后参考',
  endpoint_hint: '自定义平台 API Endpoint 摘录',
  default_payload_hint: '自定义平台 JSON 模板或字段说明摘录',
};

export const PLATFORM_PUBLISH_FIELD_PLACEHOLDER: Record<string, string> = {
  default_category_id: '在卖家后台查询后填入',
  shipping_template_id: '物流模板 ID',
  warehouse_id: '仓库 ID 或 Warehouse Code',
  default_weight: '如 0.5',
  default_length: '如 20',
  default_width: '如 15',
  default_height: '如 10',
  marketplace_id: 'ATVPDKIKX0DER',
  product_type: '如 PRODUCT',
  brand: '品牌名称',
  manufacturer: '制造商名称',
  delivery_option: '平台定义的配送选项取值',
  variation_tier_name: 'Variant',
  days_to_ship: '如 3',
};

/** 字段分组：用于表单分区展示 */
export type PublishFieldSectionKey = 'category' | 'logistics' | 'package' | 'product' | 'publish' | 'advanced';

export const PUBLISH_FIELD_SECTION_META: Record<
  PublishFieldSectionKey,
  { title: string; description?: string }
> = {
  category: {
    title: '类目与品牌',
    description: '刊登时未在商品详情单独指定，则使用以下默认值',
  },
  logistics: {
    title: '物流与履约',
    description: '运费模板、物流渠道、仓库等履约相关参数',
  },
  package: {
    title: '包裹尺寸与重量',
    description: '用于平台运费计算与物流校验，单位请与平台要求一致',
  },
  product: {
    title: '商品属性',
    description: '成色、保修、Amazon 属性等平台要求的商品级参数',
  },
  publish: {
    title: '发布行为',
    description: '控制默认上架方式；单次刊登可在商品详情覆盖',
  },
  advanced: {
    title: '高级参数',
    description: 'JSON 模板、Endpoint 摘录等扩展配置',
  },
};

const FIELD_SECTION: Record<string, PublishFieldSectionKey> = {
  default_category_id: 'category',
  default_brand_id: 'category',
  default_browse_node_id: 'category',
  default_vendor: 'category',
  default_product_type: 'category',
  default_tags: 'category',
  size_chart_id: 'category',
  marketplace_id: 'category',
  product_type: 'category',
  shipping_template_id: 'logistics',
  logistic_channel_id: 'logistics',
  warehouse_id: 'logistics',
  delivery_option: 'logistics',
  merchant_shipping_group: 'logistics',
  fulfillment_channel: 'logistics',
  shipping_hint: 'logistics',
  default_business_policies_hint: 'logistics',
  default_weight: 'package',
  default_length: 'package',
  default_width: 'package',
  default_height: 'package',
  package_weight: 'package',
  package_length: 'package',
  package_width: 'package',
  package_height: 'package',
  weight_unit: 'package',
  dimension_unit: 'package',
  condition: 'product',
  days_to_ship: 'product',
  variation_tier_name: 'product',
  warranty_type: 'product',
  warranty_period: 'product',
  brand: 'product',
  manufacturer: 'product',
  condition_type: 'product',
  product_status: 'product',
  return_policy_id: 'product',
  issue_locale: 'product',
  requirements: 'product',
  default_status: 'product',
  publish_as_draft: 'publish',
  endpoint_hint: 'advanced',
  default_payload_hint: 'advanced',
  amazon_attributes: 'advanced',
};

const SECTION_ORDER: PublishFieldSectionKey[] = ['category', 'logistics', 'package', 'product', 'publish', 'advanced'];

export function publishFieldSection(fieldName: string): PublishFieldSectionKey {
  return FIELD_SECTION[fieldName.trim().toLowerCase()] ?? 'advanced';
}

export function groupPublishFields(fields: AppConfigFieldDTO[]): { section: PublishFieldSectionKey; fields: AppConfigFieldDTO[] }[] {
  const buckets = new Map<PublishFieldSectionKey, AppConfigFieldDTO[]>();
  for (const f of fields) {
    if (f.type === 'switch') continue;
    const sec = publishFieldSection(f.name);
    const list = buckets.get(sec) ?? [];
    list.push(f);
    buckets.set(sec, list);
  }
  return SECTION_ORDER.filter((k) => (buckets.get(k)?.length ?? 0) > 0).map((k) => ({
    section: k,
    fields: buckets.get(k)!,
  }));
}

export function publishSwitchFields(fields: AppConfigFieldDTO[]): AppConfigFieldDTO[] {
  return fields.filter((f) => f.type === 'switch');
}

export function platformPublishFieldLabel(field: AppConfigFieldDTO): string {
  const mapped = PLATFORM_PUBLISH_FIELD_LABEL[field.name.trim().toLowerCase()];
  if (mapped) return mapped;
  return field.label.replace(/（平台取值字符串）/g, '').replace(/（可选）/g, '（可选）').trim();
}

export function platformPublishFieldHelp(field: AppConfigFieldDTO): string | undefined {
  return PLATFORM_PUBLISH_FIELD_HELP[field.name.trim().toLowerCase()] || field.help;
}

export function platformPublishFieldPlaceholder(field: AppConfigFieldDTO): string {
  if (field.placeholder) return field.placeholder;
  return PLATFORM_PUBLISH_FIELD_PLACEHOLDER[field.name.trim().toLowerCase()] || '';
}

export function isFullWidthPublishField(f: AppConfigFieldDTO): boolean {
  return f.type === 'textarea' || f.name === 'amazon_attributes' || f.name === 'default_tags';
}

/** 配置分组标识的用户友好说明 */
export function publishGroupKeyHint(groupKey: string): string {
  return `系统内配置标识：${groupKey}`;
}
