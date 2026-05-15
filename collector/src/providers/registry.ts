import type { CollectorProvider } from './collector-provider.js';
import { alibaba1688Provider } from './source1688/alibaba-1688.js';

const providers: CollectorProvider[] = [alibaba1688Provider];

const bySource = new Map<string, CollectorProvider>(
  providers.map((p) => [p.sourceId.toLowerCase(), p]),
);

export function getProviderBySource(source: string): CollectorProvider | undefined {
  return bySource.get(source.trim().toLowerCase());
}

export function listRegisteredSources(): string[] {
  return providers.map((p) => p.sourceId);
}
