import { chromium, type Browser, type Page } from 'playwright';
import { getBrowserHeadless, getDefaultNavigationTimeoutMs } from '../config/env.js';

/**
 * 统一管理 Chromium 实例，避免各 Provider 自行 newBrowser 导致泄漏。
 */
export class BrowserManager {
  private browser: Browser | null = null;

  /** 懒加载启动浏览器（单例）。 */
  async ensureBrowser(): Promise<Browser> {
    if (this.browser) return this.browser;
    this.browser = await chromium.launch({
      headless: getBrowserHeadless(),
      args: ['--disable-blink-features=AutomationControlled'],
    });
    return this.browser;
  }

  /**
   * 创建独立 BrowserContext + Page，执行完自动关闭 page/context。
   * UA / 代理 / Cookie 可后续在 newContext 中扩展。
   */
  async withPage<T>(fn: (page: Page) => Promise<T>): Promise<T> {
    const browser = await this.ensureBrowser();
    const context = await browser.newContext({
      userAgent:
        process.env.COLLECTOR_USER_AGENT ??
        'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
      locale: 'zh-CN',
    });
    const page = await context.newPage();
    page.setDefaultNavigationTimeout(getDefaultNavigationTimeoutMs());
    page.setDefaultTimeout(getDefaultNavigationTimeoutMs());
    try {
      return await fn(page);
    } finally {
      await page.close().catch(() => undefined);
      await context.close().catch(() => undefined);
    }
  }

  async close(): Promise<void> {
    if (!this.browser) return;
    await this.browser.close();
    this.browser = null;
  }
}
