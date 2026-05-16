import type { BrowserManager } from '../browser/manager.js';
import type { CollectProviderMeta } from '../types/provider-meta.js';
import type { NormalizedProduct } from '../types/product.js';

/** 单次采集入参 */
export type CollectInput = {
  url: string;
  /** Provider-specific payload（例如 custom：后端下发的 rule） */
  options?: Record<string, unknown>;
};

/**
 * CollectorProvider — 每个采集源一个实现（1688、淘宝…），禁止在框架层写死平台逻辑。
 */
export interface CollectorProvider {
  /** 与统一输出字段 `source` 一致，如 `1688` */
  readonly sourceId: string;

  /** 产品元信息（注册表驱动 /v1/providers） */
  readonly meta: CollectProviderMeta;

  /** 是否接受该 URL（用于快速校验，不必打开浏览器） */
  canHandle(url: string): boolean;

  /** 执行采集：由 BrowserManager 提供页面临时实例 */
  collect(browser: BrowserManager, input: CollectInput): Promise<NormalizedProduct>;
}
