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

function mapPrefixedError(msg: string): { code: CollectTaskErrorCode; message: string } | null {
  const prefixes: { p: string; code: CollectTaskErrorCode }[] = [
    { p: 'INVALID_REQUEST:', code: 'INVALID_REQUEST' },
    { p: 'PROVIDER_NOT_IMPLEMENTED:', code: 'PROVIDER_NOT_IMPLEMENTED' },
    { p: 'PAGE_BLOCKED_OR_VERIFY_REQUIRED:', code: 'PAGE_BLOCKED_OR_VERIFY_REQUIRED' },
    { p: 'INVALID_URL:', code: 'INVALID_URL' },
    { p: 'NAVIGATION_FAILED:', code: 'NAVIGATION_FAILED' },
    { p: 'COLLECT_FAILED:', code: 'COLLECT_FAILED' },
    { p: 'PROVIDER_NOT_AVAILABLE:', code: 'PROVIDER_NOT_AVAILABLE' },
  ];
  for (const { p, code } of prefixes) {
    if (msg.startsWith(p)) {
      return { code, message: msg.slice(p.length) };
    }
  }
  return null;
}

/**
 * 采集任务入口：校验 source → 选择 Provider → 执行 collect（不写库，仅返回结构化 JSON）。
 */
export async function runCollectTask(
  input: { source: string; url: string; options?: Record<string, unknown> },
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
      error: {
        code: 'INVALID_URL',
        message: `url is not supported by source "${provider.sourceId}"`,
      },
    };
  }

  try {
    const product = await provider.collect(browser, { url, options: input.options });
    return { status: 'success', product };
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    const mapped = mapPrefixedError(msg);
    if (mapped) {
      return { status: 'failed', error: mapped };
    }
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
      error: { code: 'COLLECT_FAILED', message: msg },
    };
  }
}
