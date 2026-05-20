import type { BrowserManager } from '../browser/manager.js';
import { CustomCollectError } from '../providers/sourceCustom/errors.js';
import { getProviderBySource } from '../providers/registry.js';
import type { NormalizedProduct } from '../types/product.js';
import type { CustomAccessReport } from '../types/access-status.js';
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
  access?: CustomAccessReport;
};

export type CollectTaskResult = CollectTaskSuccess | CollectTaskFailure;

const PREFIX_CODES: { p: string; code: CollectTaskErrorCode }[] = [
  { p: 'LOGIN_REQUIRED:', code: 'LOGIN_REQUIRED' },
  { p: 'CUSTOM_RULE_MISSING:', code: 'CUSTOM_RULE_MISSING' },
  { p: 'CUSTOM_RULE_INVALID:', code: 'CUSTOM_RULE_INVALID' },
  { p: 'PARSE_FAILED_TITLE_MISSING:', code: 'PARSE_FAILED_TITLE_MISSING' },
  { p: 'PARSE_FAILED_IMAGE_MISSING:', code: 'PARSE_FAILED_IMAGE_MISSING' },
  { p: 'INVALID_REQUEST:', code: 'INVALID_REQUEST' },
  { p: 'PROVIDER_NOT_IMPLEMENTED:', code: 'PROVIDER_NOT_IMPLEMENTED' },
  { p: 'PAGE_BLOCKED_OR_VERIFY_REQUIRED:', code: 'PAGE_BLOCKED_OR_VERIFY_REQUIRED' },
  { p: 'VERIFY_REQUIRED:', code: 'PAGE_BLOCKED_OR_VERIFY_REQUIRED' },
  { p: 'PAGE_BLOCKED:', code: 'PAGE_BLOCKED_OR_VERIFY_REQUIRED' },
  { p: 'CAPTCHA:', code: 'PAGE_BLOCKED_OR_VERIFY_REQUIRED' },
  { p: 'INVALID_URL:', code: 'INVALID_URL' },
  { p: 'UNSUPPORTED_URL:', code: 'UNSUPPORTED_URL' },
  { p: 'PRODUCT_NOT_FOUND:', code: 'PRODUCT_NOT_FOUND' },
  { p: 'NAVIGATION_FAILED:', code: 'NAVIGATION_FAILED' },
  { p: 'TIMEOUT:', code: 'TIMEOUT' },
  { p: 'PAGE_LOAD_TIMEOUT:', code: 'TIMEOUT' },
  { p: 'PAGE_TIMEOUT:', code: 'TIMEOUT' },
  { p: 'PARSE_FAILED:', code: 'PARSE_FAILED' },
  { p: 'COLLECT_FAILED:', code: 'COLLECT_FAILED' },
  { p: 'PROVIDER_NOT_AVAILABLE:', code: 'PROVIDER_NOT_AVAILABLE' },
];

function mapPrefixedError(msg: string): { code: CollectTaskErrorCode; message: string } | null {
  for (const { p, code } of PREFIX_CODES) {
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
        code: 'UNSUPPORTED_URL',
        message: `url is not supported by source "${provider.sourceId}"`,
      },
    };
  }

  try {
    const product = await provider.collect(browser, { url, options: input.options });
    return { status: 'success', product };
  } catch (e) {
    if (e instanceof CustomCollectError) {
      return {
        status: 'failed',
        error: { code: e.code as CollectTaskErrorCode, message: e.message },
        access: e.report,
      };
    }
    const msg = e instanceof Error ? e.message : String(e);
    if (/__name is not defined|evaluate_script_error/i.test(msg)) {
      return {
        status: 'failed',
        error: {
          code: 'PARSE_FAILED',
          message: `evaluate_script_error:${msg}`,
        },
      };
    }
    const mapped = mapPrefixedError(msg);
    if (mapped) {
      return { status: 'failed', error: mapped };
    }
    return {
      status: 'failed',
      error: { code: 'COLLECT_FAILED', message: msg },
    };
  }
}
