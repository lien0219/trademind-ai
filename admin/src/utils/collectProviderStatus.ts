import type { CollectProviderStatus } from '@/services/collectProviders';

export type CollectProviderStatusPresentation = {
  text: string;
  color: 'success' | 'processing' | 'default' | 'error' | 'blue';
};

/** Hub / 采集设置：按 source + status 展示采集器状态文案（custom+beta → 基础可用）。 */
export function collectProviderStatusPresentation(
  source: string,
  status: CollectProviderStatus,
): CollectProviderStatusPresentation {
  const src = source.trim().toLowerCase();
  switch (status) {
    case 'available':
      return { text: '已可用', color: 'success' };
    case 'beta':
      if (src === 'custom') {
        return { text: '基础可用', color: 'blue' };
      }
      return { text: '测试中', color: 'processing' };
    case 'planned':
      return { text: '规划中', color: 'default' };
    case 'disabled':
      return { text: '已禁用', color: 'error' };
    default:
      return { text: status, color: 'default' };
  }
}

/** 自定义链接采集器展示的能力标签（不含 SKU / 库存）。 */
export const CUSTOM_COLLECT_FEATURE_LABEL: Record<string, string> = {
  title: '商品标题',
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
