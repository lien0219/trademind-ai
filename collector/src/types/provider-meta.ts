/** 与管理端 / Go 兜底列表对齐的采集器元信息与能力标签 */

export type ProviderStatus = 'available' | 'beta' | 'planned' | 'disabled';

/** 结构化能力枚举（与服务端入库字段对应，不做解析逻辑变更） */
export type CollectFeature = 'title' | 'mainImages' | 'descriptionImages' | 'attributes' | 'skus';

export type CollectProviderMeta = {
  name: string;
  description: string;
  status: ProviderStatus;
  batchSupported: boolean;
  urlPatterns: string[];
  features: CollectFeature[];
  notes: string;
};

/** GET /v1/providers 单行（含 source） */
export type CollectProviderPublic = {
  source: string;
} & CollectProviderMeta;
