import type { CollectProviderStatus } from '@/services/collectProviders';

export type CollectProviderStatusPresentation = {
  text: string;
  color: 'success' | 'processing' | 'default' | 'error' | 'blue';
};

const COLLECT_PROVIDER_STATUS_TEXT = {
  available: '已可用',
  customBeta: '基础可用',
  beta: '测试中',
  planned: '规划中',
  disabled: '已禁用',
} as const;

/** Hub / 采集设置：按 source + status 展示采集器状态文案（custom+beta → 基础可用）。 */
export function collectProviderStatusPresentation(
  source: string,
  status: CollectProviderStatus,
): CollectProviderStatusPresentation {
  const src = source.trim().toLowerCase();
  switch (status) {
    case 'available':
      return { text: COLLECT_PROVIDER_STATUS_TEXT.available, color: 'success' };
    case 'beta':
      if (src === 'custom') {
        return { text: COLLECT_PROVIDER_STATUS_TEXT.customBeta, color: 'blue' };
      }
      return { text: COLLECT_PROVIDER_STATUS_TEXT.beta, color: 'processing' };
    case 'planned':
      return { text: COLLECT_PROVIDER_STATUS_TEXT.planned, color: 'default' };
    case 'disabled':
      return { text: COLLECT_PROVIDER_STATUS_TEXT.disabled, color: 'error' };
    default:
      return { text: status, color: 'default' };
  }
}

/** 自定义链接采集器展示的能力标签（不含 SKU / 库存）。 */
const CUSTOM_COLLECT_TITLE_LABEL = '商品标题';

export const CUSTOM_COLLECT_FEATURE_LABEL: Record<string, string> = {
  title: CUSTOM_COLLECT_TITLE_LABEL,
  price: '商品价格',
  mainImages: '商品主图',
  descriptionImages: '详情图片',
  attributes: '商品参数',
};

export const CUSTOM_COLLECT_DISPLAY_FEATURES = [
  'title',
  'price',
  'mainImages',
  'descriptionImages',
  'attributes',
] as const;

export const NO_COLLECT_RULE_MESSAGE = '请先创建采集规则，或使用 AI 帮你生成规则。';
