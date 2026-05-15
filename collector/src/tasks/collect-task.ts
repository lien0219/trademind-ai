import type { BrowserManager } from '../browser/manager.js';
import { getProviderBySource } from '../providers/registry.js';
import type { NormalizedProduct } from '../types/product.js';
import type { CollectTaskErrorCode } from '../types/task.js';

export type CollectTaskSuccess = {
  status: 'success';
  product: NormalizedProduct;
};

export type CollectTaskFailure = {
  status: 'failed';
  error: {
    code: CollectTaskErrorCode;
    message: string;
  };
};

export type CollectTaskResult = CollectTaskSuccess | CollectTaskFailure;

/**
 * 采集任务入口：校验 source → 选择 Provider → 执行 collect（不写库，仅返回结构化 JSON）。
 */
export async function runCollectTask(
  input: { source: string; url: string },
  browser: BrowserManager,
): Promise<CollectTaskResult> {
  const source = input.source?.trim();
  const url = input.url?.trim();
  if (!source || !url) {
    return {
      status: 'failed',
      error: { code: 'INVALID_REQUEST', message: 'fields "source" and "url" are required' },
    };
  }

  const provider = getProviderBySource(source);
  if (!provider) {
    return {
      status: 'failed',
      error: { code: 'PROVIDER_NOT_FOUND', message: `unknown source "${source}"` },
    };
  }

  if (!provider.canHandle(url)) {
    return {
      status: 'failed',
      error: { code: 'INVALID_URL', message: `url is not supported by source "${provider.sourceId}"` },
    };
  }

  try {
    const product = await provider.collect(browser, { url });
    return { status: 'success', product };
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    if (msg.startsWith('NAVIGATION_FAILED:')) {
      return {
        status: 'failed',
        error: { code: 'NAVIGATION_FAILED', message: msg.slice('NAVIGATION_FAILED:'.length) },
      };
    }
    if (msg.startsWith('INVALID_URL:')) {
      return {
        status: 'failed',
        error: { code: 'INVALID_URL', message: msg.slice('INVALID_URL:'.length) },
      };
    }
    return {
      status: 'failed',
      error: { code: 'PROVIDER_ERROR', message: msg },
    };
  }
}
