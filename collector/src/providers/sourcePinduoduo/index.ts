import type { Page } from 'playwright';
import type { BrowserManager } from '../../browser/manager.js';
import { PINDUODUO_PROFILE_KEY } from './profile.js';
import type { CollectInput, CollectorProvider } from '../collector-provider.js';
import type { CollectFeature } from '../../types/provider-meta.js';
import type { NormalizedProduct } from '../../types/product.js';
import { getDefaultNavigationTimeoutMs } from '../../config/env.js';
import { PAGE_EVALUATE_POLYFILL } from '../../browser/evaluate-in-page.js';
import { detectPinduoduoAccessStatus, throwAccessError } from './access-detect.js';
import { extractAndAssemblePinduoduo } from './parser.js';
import { isPlatformTitle } from './wholesale-detail-shared.js';
import { validateWholesaleCollectQuality } from './wholesale-detail.js';
import { normalizePinduoduoNavUrl, validatePinduoduoUrl } from './validate-url.js';
import {
  classifyPinduoduoUrl,
  invalidPinduoduoUrlHint,
  unsupportedPinduoduoUrlMessage,
  type PinduoduoUrlType,
} from './url-type.js';

const MOBILE_UA =
  process.env.COLLECTOR_PDD_USER_AGENT ??
  'Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.38(0x18002633) NetType/WIFI Language/zh_CN';

function resolveGotoTimeoutMs(options?: Record<string, unknown>): number {
  const raw = options?.gotoTimeoutMs ?? options?.timeoutMs;
  const n = typeof raw === 'number' ? raw : Number(raw);
  if (Number.isFinite(n) && n > 0) return Math.min(n, 300_000);
  return getDefaultNavigationTimeoutMs();
}

function accessCheckEnabled(options?: Record<string, unknown>): boolean {
  if (options?.accessCheckEnabled === false) return false;
  if (options?.accessCheckEnabled === '0' || options?.accessCheckEnabled === 'false') {
    return false;
  }
  return true;
}

function rejectUrlTypeBeforeNav(urlType: PinduoduoUrlType): void {
  switch (urlType) {
    case 'wholesale_homepage':
      throw new Error('UNSUPPORTED_PINDUODUO_URL:wholesale_homepage');
    case 'goods_detail':
      throw new Error('UNSUPPORTED_PINDUODUO_URL:goods_detail');
    case 'wechat_auth':
      throw new Error('WECHAT_AUTH_REQUIRED:wechat_auth_url');
    case 'login_page':
      throw new Error('LOGIN_REQUIRED:login_page');
    case 'app_redirect':
      throw new Error('APP_REDIRECT:app_redirect');
    case 'unknown':
      throw new Error(`UNSUPPORTED_PINDUODUO_URL:${urlType}`);
    default:
      break;
  }
}

class PinduoduoCollectorProvider implements CollectorProvider {
  readonly sourceId = 'pinduoduo';
  readonly meta = {
    name: '拼多多采集器',
    description: '采集拼多多批发商品详情，支持标题、价格、主图、规格等基础字段。',
    status: 'available' as const,
    batchSupported: true,
    urlPatterns: [
      'https://pifa.pinduoduo.com/goods/detail/?gid=*',
      'https://mobile.yangkeduo.com/goods.html?goods_id=*',
      'https://yangkeduo.com/goods.html?goods_id=*',
    ],
    features: [
      'title',
      'price',
      'mainImages',
      'descriptionImages',
      'attributes',
      'skus',
    ] satisfies CollectFeature[],
    notes: '批量采集默认限速，建议先少量测试。',
  };

  canHandle(url: string): boolean {
    return validatePinduoduoUrl(url);
  }

  async collect(browser: BrowserManager, input: CollectInput): Promise<NormalizedProduct> {
    const sourceUrl = input.url.trim();
    const urlType = classifyPinduoduoUrl(sourceUrl);

    if (!this.canHandle(sourceUrl)) {
      throw new Error(`INVALID_URL:${invalidPinduoduoUrlHint()}`);
    }

    rejectUrlTypeBeforeNav(urlType);

    const navUrl = normalizePinduoduoNavUrl(sourceUrl);
    const profileKey = String(input.options?.profileKey ?? '').trim();
    const useDedicatedProfile =
      input.options?.useBrowserProfile === true &&
      profileKey === PINDUODUO_PROFILE_KEY;
    const gotoTimeout = resolveGotoTimeoutMs(input.options);
    const runAccessCheck = accessCheckEnabled(input.options);

    const run = async (page: Page) => {
      try {
        await page.goto(navUrl, { waitUntil: 'domcontentloaded', timeout: gotoTimeout });
      } catch (e) {
        const err = e instanceof Error ? e.message : String(e);
        if (/timeout/i.test(err)) throw new Error(`TIMEOUT:navigation_${err}`);
        throw new Error(`NAVIGATION_FAILED:${err}`);
      }

      await page.waitForLoadState('networkidle', { timeout: Math.min(gotoTimeout, 12_000) }).catch(() => undefined);
      await page.waitForTimeout(800);

      const postNavType = classifyPinduoduoUrl(page.url());
      if (postNavType === 'wechat_auth') {
        throw new Error('WECHAT_AUTH_REQUIRED:wechat_redirect');
      }
      if (postNavType === 'login_page') {
        throw new Error('LOGIN_REQUIRED:login_redirect');
      }
      if (postNavType === 'app_redirect') {
        throw new Error('APP_REDIRECT:app_redirect');
      }
      if (postNavType === 'goods_detail') {
        throw new Error('UNSUPPORTED_PINDUODUO_URL:goods_detail');
      }

      if (runAccessCheck) {
        const access = await detectPinduoduoAccessStatus(page, sourceUrl);
        if (access.status !== 'public') {
          if (access.errorCode) throwAccessError(access, sourceUrl);
        }
      }

      const assembled = await extractAndAssemblePinduoduo(page, sourceUrl, urlType);
      const title = assembled.title.trim();

      const quality = validateWholesaleCollectQuality(assembled);
      if (!quality.ok && quality.error) {
        throw new Error(quality.error);
      }

      const qualityWarnings = [...new Set(assembled.warnings)];
      const rawWarnings = (assembled.raw.warnings as string[] | undefined) ?? [];

      const partial =
        quality.partial ||
        qualityWarnings.length > 0 ||
        rawWarnings.length > 0 ||
        isPlatformTitle(title) ||
        !assembled.price ||
        assembled.price <= 0;

      const raw: Record<string, unknown> = {
        ...assembled.raw,
        productPrice: assembled.price,
        priceText: assembled.priceText,
        qualityWarnings,
        urlType,
        finalUrl: page.url(),
        navUrl,
        collectStatus: partial ? 'partial_success' : 'success',
      };

      return {
        source: this.sourceId,
        sourceUrl,
        title,
        currency: assembled.currency,
        mainDescription: assembled.mainDescription,
        mainImages: assembled.mainImages,
        descriptionImages: assembled.descriptionImages,
        attributes: assembled.attributes,
        skus: assembled.skus,
        raw,
      } satisfies NormalizedProduct;
    };

    if (useDedicatedProfile) {
      return browser.withPinduoduoPage(run);
    }

    const browserInstance = await browser.ensureBrowser();
    const context = await browserInstance.newContext({
      userAgent:
        process.env.COLLECTOR_USER_AGENT ??
        'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
      locale: 'zh-CN',
      viewport: { width: 1280, height: 900 },
      isMobile: false,
      hasTouch: false,
    });
    await context.addInitScript(PAGE_EVALUATE_POLYFILL);
    const page = await context.newPage();
    page.setDefaultNavigationTimeout(gotoTimeout);
    page.setDefaultTimeout(gotoTimeout);
    try {
      return await run(page);
    } finally {
      await page.close().catch(() => undefined);
      await context.close().catch(() => undefined);
    }
  }
}

export const pinduoduoCollectorProvider = new PinduoduoCollectorProvider();
export {
  classifyPinduoduoUrl,
  unsupportedPinduoduoUrlMessage,
  type PinduoduoUrlType,
};
