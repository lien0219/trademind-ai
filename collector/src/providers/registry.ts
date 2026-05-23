import type { CollectorProvider } from './collector-provider.js';
import { aliExpressCollectorProvider } from './sourceAliExpress/index.js';
import { alibaba1688Provider } from './source1688/alibaba-1688.js';
import { sourceSheinTemuProvider, sourceTaobaoProvider } from './stub/placeholders.js';
import type { CollectProviderPublic } from '../types/provider-meta.js';
import { sourceCustomCollectorProvider } from './sourceCustom/index.js';
import { pinduoduoCollectorProvider } from './sourcePinduoduo/index.js';

const providers: CollectorProvider[] = [
  alibaba1688Provider,
  pinduoduoCollectorProvider,
  sourceTaobaoProvider,
  aliExpressCollectorProvider,
  sourceSheinTemuProvider,
  sourceCustomCollectorProvider,
];

const bySource = new Map<string, CollectorProvider>(
  providers.map((p) => [p.sourceId.toLowerCase(), p]),
);
/** Legacy alias: historical tasks / settings used `pdd`. */
bySource.set('pdd', pinduoduoCollectorProvider);

export function getProviderBySource(source: string): CollectorProvider | undefined {
  return bySource.get(source.trim().toLowerCase());
}

export function listRegisteredSources(): string[] {
  return providers.map((p) => p.sourceId);
}

/** 对外列表：顺序稳定，便于管理端展示 */
export function listProviderPublicMetas(): CollectProviderPublic[] {
  return providers.map((p) => ({
    source: p.sourceId,
    ...p.meta,
  }));
}
