import { chromium, type BrowserContext, type Page } from 'playwright';
import { evaluateGenericPageAccess, resolveAccessStatusFromSignals } from '../providers/sourceCustom/access-detect.js';
import type { AccessStatus } from '../types/access-status.js';
import { getCustomProfileUserDataDir } from './browser-paths.js';
import { PAGE_EVALUATE_POLYFILL } from './evaluate-in-page.js';
import { sanitizeProfileKey } from './profile-key.js';
import { getBrowserHeadless, getDefaultNavigationTimeoutMs } from '../config/env.js';

function persistentContextOptions(headless: boolean) {
  return {
    headless,
    locale: 'zh-CN' as const,
    userAgent:
      process.env.COLLECTOR_USER_AGENT ??
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
    args: ['--disable-blink-features=AutomationControlled'],
    viewport: { width: 1280, height: 800 },
  };
}

export type ProfileCheckResult = {
  accessStatus: AccessStatus;
  finalUrl: string;
  httpStatus?: number;
  errorCode?: string;
  message: string;
};

/**
 * Per profile_key persistent Playwright contexts (custom link collector login state).
 * Separate from 1688 provider session — does not alter 1688 login detection.
 */
export class CustomProfileSessionManager {
  private contexts = new Map<string, BrowserContext>();
  private contextHeadless = new Map<string, boolean>();
  private loginActive = new Set<string>();
  private lock: Promise<void> = Promise.resolve();

  private runLocked<T>(fn: () => Promise<T>): Promise<T> {
    const next = this.lock.then(fn, fn);
    this.lock = next.then(
      () => undefined,
      () => undefined,
    );
    return next;
  }

  async getOrCreateContext(profileKey: string, opts?: { headless?: boolean }): Promise<BrowserContext> {
    const key = sanitizeProfileKey(profileKey);
    const wantHeadless = opts?.headless ?? (this.loginActive.has(key) ? false : getBrowserHeadless());

    const existing = this.contexts.get(key);
    if (existing && !existing.isClosed()) {
      const was = this.contextHeadless.get(key) ?? true;
      if (was === wantHeadless) return existing;
      await existing.close().catch(() => undefined);
      this.contexts.delete(key);
      this.contextHeadless.delete(key);
    }

    const userDataDir = getCustomProfileUserDataDir(key);
    const context = await chromium.launchPersistentContext(
      userDataDir,
      persistentContextOptions(wantHeadless),
    );
    await context.addInitScript(PAGE_EVALUATE_POLYFILL);
    const timeout = getDefaultNavigationTimeoutMs();
    context.setDefaultNavigationTimeout(timeout);
    context.setDefaultTimeout(timeout);
    this.contexts.set(key, context);
    this.contextHeadless.set(key, wantHeadless);
    return context;
  }

  async withProfilePage<T>(profileKey: string, fn: (page: Page) => Promise<T>): Promise<T> {
    return this.runLocked(async () => {
      const context = await this.getOrCreateContext(profileKey);
      const page = await context.newPage();
      const timeout = getDefaultNavigationTimeoutMs();
      page.setDefaultNavigationTimeout(timeout);
      page.setDefaultTimeout(timeout);
      try {
        return await fn(page);
      } finally {
        await page.close().catch(() => undefined);
      }
    });
  }

  async openLoginBrowser(
    profileKey: string,
    targetUrl: string,
  ): Promise<{ profilePath: string; message: string; alreadyOpen: boolean }> {
    // 与 1688 open-login 一致：登录窗口强制 headed，不要求全局 COLLECTOR_HEADLESS=0。
    const key = sanitizeProfileKey(profileKey);
    const url = targetUrl.trim();
    if (!url) {
      throw new Error('INVALID_REQUEST:url required');
    }

    return this.runLocked(async () => {
      const userDataDir = getCustomProfileUserDataDir(key);
      const existing = this.contexts.get(key);
      if (this.loginActive.has(key) && existing && !existing.isClosed()) {
        const page = existing.pages()[0] ?? (await existing.newPage());
        await page.bringToFront().catch(() => undefined);
        if (page.url() === 'about:blank' || !page.url().startsWith('http')) {
          await page.goto(url, {
            waitUntil: 'domcontentloaded',
            timeout: getDefaultNavigationTimeoutMs(),
          });
        }
        return {
          profilePath: userDataDir,
          message: '采集浏览器已打开，请在窗口中完成登录或安全验证',
          alreadyOpen: true,
        };
      }

      this.loginActive.add(key);
      const context = await this.getOrCreateContext(key, { headless: false });
      const page = context.pages()[0] ?? (await context.newPage());
      await page.goto(url, {
        waitUntil: 'domcontentloaded',
        timeout: getDefaultNavigationTimeoutMs(),
      });
      return {
        profilePath: userDataDir,
        message: '已打开采集浏览器，请在此窗口手动登录目标网站（系统不保存账号密码）',
        alreadyOpen: false,
      };
    });
  }

  async checkProfileAccess(profileKey: string, targetUrl: string): Promise<ProfileCheckResult> {
    const url = targetUrl.trim();
    if (!url) {
      throw new Error('INVALID_REQUEST:url required');
    }
    const timeoutMs = getDefaultNavigationTimeoutMs();

    return this.withProfilePage(profileKey, async (page) => {
      let httpStatus: number | undefined;
      try {
        const resp = await page.goto(url, {
          waitUntil: 'domcontentloaded',
          timeout: timeoutMs,
        });
        httpStatus = resp?.status();
      } catch (e) {
        const err = e instanceof Error ? e.message : String(e);
        if (/timeout/i.test(err)) {
          return {
            accessStatus: 'timeout',
            finalUrl: url,
            errorCode: 'TIMEOUT',
            message: '页面加载超时',
          };
        }
        return {
          accessStatus: 'navigation_failed',
          finalUrl: url,
          errorCode: 'NAVIGATION_FAILED',
          message: `页面打开失败：${err}`,
        };
      }

      await page
        .waitForLoadState('networkidle', { timeout: Math.min(timeoutMs, 12_000) })
        .catch(() => undefined);

      const signals = await evaluateGenericPageAccess(page, httpStatus);
      const accessStatus = resolveAccessStatusFromSignals(signals);
      let message = '页面可访问';
      let errorCode: string | undefined;
      if (accessStatus === 'login_required') {
        message = '页面疑似需要登录';
        errorCode = 'LOGIN_REQUIRED';
      } else if (accessStatus === 'verify_required' || accessStatus === 'blocked') {
        message = '页面疑似触发验证码或风控';
        errorCode = 'PAGE_BLOCKED_OR_VERIFY_REQUIRED';
      } else if (accessStatus === 'unknown') {
        message = '无法确认访问状态';
      }

      return {
        accessStatus,
        finalUrl: signals.finalUrl,
        httpStatus,
        errorCode,
        message,
      };
    });
  }

  async close(): Promise<void> {
    await this.runLocked(async () => {
      for (const [key, ctx] of this.contexts.entries()) {
        if (!ctx.isClosed()) await ctx.close().catch(() => undefined);
        this.contexts.delete(key);
        this.contextHeadless.delete(key);
      }
      this.loginActive.clear();
    });
  }
}
