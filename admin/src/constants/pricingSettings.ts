import { tr } from '@/i18n';
/** 加价方式（存库 default_markup_type） */
export const MARKUP_TYPE_OPTIONS = [
  { get label() { return tr("百分比加价"); }, value: 'percent' },
  { get label() { return tr("固定金额加价"); }, value: 'fixed' },
  { get label() { return tr("倍率加价"); }, value: 'multiplier' },
  { get label() { return tr("不加价"); }, value: 'none' },
];

export function markupTypeLabel(type?: string): string {
  const k = (type || '').trim();
  return MARKUP_TYPE_OPTIONS.find((o) => o.value === k)?.label || type || '—';
}

/** 尾数规则（存库 default_rounding_mode） */
export const ROUNDING_MODE_OPTIONS = [
  { get label() { return tr("不处理"); }, value: 'none' },
  { get label() { return tr("取整（整数）"); }, value: 'integer' },
  { get label() { return tr("尾数 .9"); }, value: '.9' },
  { get label() { return tr("尾数 .99"); }, value: '.99' },
  { get label() { return tr("尾数 .95"); }, value: '.95' },
  { get label() { return tr("尾数 9.99"); }, value: '9.99' },
  { get label() { return tr("尾数 19.90"); }, value: '19.90' },
];

/** 默认币种 */
export const PRICING_CURRENCY_OPTIONS = [
  { get label() { return tr("人民币 CNY"); }, value: 'CNY' },
  { get label() { return tr("美元 USD"); }, value: 'USD' },
  { get label() { return tr("欧元 EUR"); }, value: 'EUR' },
];

/** 平台覆盖加价比例字段 */
export const PRICING_PLATFORM_MARKUPS = [
  { name: 'tiktok_markup_percent', label: 'TikTok' },
  { name: 'shopee_markup_percent', label: 'Shopee' },
  { name: 'lazada_markup_percent', label: 'Lazada' },
  { name: 'amazon_markup_percent', label: 'Amazon' },
] as const;
