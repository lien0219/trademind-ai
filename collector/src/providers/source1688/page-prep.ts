import type { Page } from 'playwright';

const CORE_SELECTORS =
  'h1, h1.d-title, [class*="title"], [class*="price"], [class*="gallery"], [class*="offer-img"], [class*="sku"], [class*="obj-sku"]';

/** 1688 详情页：等待核心区域 + 分段滚动触发懒加载 */
export async function prepare1688OfferPage(page: Page, batchMode: boolean): Promise<void> {
  const delayMs = batchMode ? 2000 + Math.floor(Math.random() * 2000) : 2500 + Math.floor(Math.random() * 2500);
  await page.waitForTimeout(delayMs);

  await page
    .waitForSelector(CORE_SELECTORS, { timeout: batchMode ? 12_000 : 10_000 })
    .catch(() => undefined);

  await scrollPageForLazyLoad(page);

  await page.waitForTimeout(batchMode ? 1200 : 800);
}

async function scrollPageForLazyLoad(page: Page): Promise<void> {
  await page.evaluate(async () => {
    const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));
    const step = Math.max(320, Math.floor(window.innerHeight * 0.75));
    let y = 0;
    const maxScroll = Math.min(document.body?.scrollHeight ?? 0, 12_000);
    while (y < maxScroll) {
      window.scrollTo(0, y);
      await sleep(350);
      y += step;
    }
    window.scrollTo(0, 0);
    await sleep(200);
  });
}
