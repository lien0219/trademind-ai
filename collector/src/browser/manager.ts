import { chromium, type Browser, type Page } from 'playwright';
import { CustomProfileSessionManager } from './profile-sessions.js';
import { BrowserSessionManager } from './session-manager.js';
import { PAGE_EVALUATE_POLYFILL } from './evaluate-in-page.js';
import { getBrowserHeadless, getDefaultNavigationTimeoutMs } from '../config/env.js';

/**
 * 统一管理 Chromium 实例，避免各 Provider 自行 newBrowser 导致泄漏。
 * 1688 采集使用 BrowserSessionManager 持久化 Profile（collector/data/browser-profiles/1688）。
 */
export class BrowserManager {
  private browser: Browser | null = null;
  readonly sessions = new BrowserSessionManager();
  readonly customProfiles = new CustomProfileSessionManager();

  /** @deprecated 使用 sessions */
  get profile1688() {
    return this.sessions;
  }

  async ensureBrowser(): Promise<Browser> {
    if (this.browser) return this.browser;
    this.browser = await chromium.launch({
      headless: getBrowserHeadless(),
      args: ['--disable-blink-features=AutomationControlled'],
    });
    return this.browser;
  }

  async with1688Page<T>(fn: (page: Page) => Promise<T>): Promise<T> {
    return this.sessions.withProviderPage('1688', fn);
  }

  async withCustomProfilePage<T>(profileKey: string, fn: (page: Page) => Promise<T>): Promise<T> {
    return this.customProfiles.withProfilePage(profileKey, fn);
  }

  async withPage<T>(fn: (page: Page) => Promise<T>): Promise<T> {
    const browser = await this.ensureBrowser();
    const context = await browser.newContext({
      userAgent:
        process.env.COLLECTOR_USER_AGENT ??
        'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
      locale: 'zh-CN',
    });
    await context.addInitScript(PAGE_EVALUATE_POLYFILL);
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
    await this.customProfiles.close();
    await this.sessions.close();
    if (!this.browser) return;
    await this.browser.close();
    this.browser = null;
  }
}
