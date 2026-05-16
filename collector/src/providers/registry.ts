import type { CollectorProvider } from './collector-provider.js';
import { alibaba1688Provider } from './source1688/alibaba-1688.js';
import {
  sourceAliExpressProvider,
  sourceCustomProvider,
  sourcePddProvider,
  sourceSheinTemuProvider,
  sourceTaobaoProvider,
} from './stub/placeholders.js';
import type { CollectProviderPublic } from '../types/provider-meta.js';

const providers: CollectorProvider[] = [
  alibaba1688Provider,
  sourcePddProvider,
  sourceTaobaoProvider,
  sourceAliExpressProvider,
  sourceSheinTemuProvider,
  sourceCustomProvider,
];

const bySource = new Map<string, CollectorProvider>(
  providers.map((p) => [p.sourceId.toLowerCase(), p]),
);

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
