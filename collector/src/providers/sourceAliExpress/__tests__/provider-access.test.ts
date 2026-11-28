import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { BrowserManager } from '../../../browser/manager.js';
import type { Page } from 'playwright';

const parserMocks = vi.hoisted(() => ({
  extractBrowserPayload: vi.fn(),
  assembleAeProduct: vi.fn(),
}));

vi.mock('../parser.js', () => parserMocks);

import { aliExpressCollectorProvider } from '../index.js';

describe('AliExpress provider access gate', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    parserMocks.extractBrowserPayload.mockResolvedValue({
      finalUrl: 'https://www.aliexpress.com/item/100500123.html',
      docTitle: 'Verification',
      meta: {},
      headingText: '',
      galleryUrls: [],
      detailUrls: [],
      paramPairs: [],
      scriptSnippets: [],
      __blocked__: 1,
    });
    parserMocks.assembleAeProduct.mockReturnValue({
      title: 'Verification',
      currency: 'USD',
      mainImages: [],
      descriptionImages: [],
      attributes: {},
      skus: [],
      rawShell: {},
      blocked: true,
      stateDigest: {},
    });
  });

  it('rejects a flagged challenge even when the document title is non-empty', async () => {
    const page = {
      goto: vi.fn().mockResolvedValue(undefined),
      waitForLoadState: vi.fn().mockResolvedValue(undefined),
      waitForSelector: vi.fn().mockResolvedValue(undefined),
      url: vi.fn(() => 'https://www.aliexpress.com/item/100500123.html'),
    };
    const browser = {
      withPage: vi.fn(async (run: (page: Page) => Promise<unknown>) => run(page as unknown as Page)),
    };

    await expect(aliExpressCollectorProvider.collect(browser as unknown as BrowserManager, {
      url: 'https://www.aliexpress.com/item/100500123.html',
    })).rejects.toThrow('PAGE_BLOCKED_OR_VERIFY_REQUIRED:verification_or_login_challenge');
  });
});
