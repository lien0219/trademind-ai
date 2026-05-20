import { chromium, type BrowserContext, type Page } from 'playwright';
import {
  countAuthCookies,
  evaluateAuthPage,
  getAuthCheckUrls,
  logAuthDebug,
  resolveAuthResult,
  type AuthCheckStatus,
} from '../providers/source1688/auth-detect.js';
import {
  ensureBrowserDataDirs,
  get1688UserDataDir,
} from './browser-paths.js';
import { PAGE_EVALUATE_POLYFILL } from './evaluate-in-page.js';
import { getBrowserHeadless, getDefaultNavigationTimeoutMs } from '../config/env.js';

const PROVIDER_1688 = '1688';

export type AuthStatus1688 = {
  provider: typeof PROVIDER_1688;
  status: AuthCheckStatus;
  loggedIn: boolean;
  needVerification: boolean;
  message: string;
  lastCheckedAt: string;
  profilePath: string;
};

function defaultUserAgent(): string {
  return (
    process.env.COLLECTOR_USER_AGENT ??
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'
  );
}

function persistentContextOptions(headless: boolean) {
  return {
    headless,
    locale: 'zh-CN' as const,
    userAgent: defaultUserAgent(),
    args: ['--disable-blink-features=AutomationControlled'],
    viewport: { width: 1280, height: 800 },
  };
}

/**
 * 同一 Provider 仅维护一个 launchPersistentContext，供登录 / 检测 / 采集共用。
 */
export class BrowserSessionManager {
  private contexts = new Map<string, BrowserContext>();
  private contextHeadless = new Map<string, boolean>();
  private loginSessionActive = new Set<string>();
  private lock: Promise<void> = Promise.resolve();

  private runLocked<T>(fn: () => Promise<T>): Promise<T> {
    const next = this.lock.then(fn, fn);
    this.lock = next.then(
      () => undefined,
      () => undefined,
    );
    return next;
  }

  get1688UserDataDir(): string {
    return get1688UserDataDir();
  }

  isLoginSessionActive(provider: string = PROVIDER_1688): boolean {
    return this.loginSessionActive.has(provider);
  }

  /** 获取或创建 Provider 持久化上下文（同一 userDataDir 不并发打开）。 */
  async getOrCreateProviderContext(
    provider: string = PROVIDER_1688,
    opts?: { headless?: boolean },
  ): Promise<BrowserContext> {
    const wantHeadless =
      opts?.headless ??
      (this.loginSessionActive.has(provider) ? false : getBrowserHeadless());

    const existing = this.contexts.get(provider);
    if (existing && !existing.isClosed()) {
      const wasHeadless = this.contextHeadless.get(provider) ?? true;
      if (wasHeadless === wantHeadless) {
        return existing;
      }
      await existing.close().catch(() => undefined);
      this.contexts.delete(provider);
      this.contextHeadless.delete(provider);
    }

    ensureBrowserDataDirs();
    const userDataDir =
      provider === PROVIDER_1688 ? get1688UserDataDir() : `${get1688UserDataDir()}/../${provider}`;

    const context = await chromium.launchPersistentContext(
      userDataDir,
      persistentContextOptions(wantHeadless),
    );
    await context.addInitScript(PAGE_EVALUATE_POLYFILL);
    context.setDefaultNavigationTimeout(getDefaultNavigationTimeoutMs());
    context.setDefaultTimeout(getDefaultNavigationTimeoutMs());
    this.contexts.set(provider, context);
    this.contextHeadless.set(provider, wantHeadless);
    return context;
  }

  async withProviderPage<T>(
    provider: string,
    fn: (page: Page) => Promise<T>,
  ): Promise<T> {
    return this.runLocked(async () => {
      const context = await this.getOrCreateProviderContext(provider);
      const page = await context.newPage();
      page.setDefaultNavigationTimeout(getDefaultNavigationTimeoutMs());
      page.setDefaultTimeout(getDefaultNavigationTimeoutMs());
      try {
        return await fn(page);
      } finally {
        await page.close().catch(() => undefined);
      }
    });
  }

  async openLoginBrowser(provider: string = PROVIDER_1688): Promise<{
    profilePath: string;
    message: string;
    alreadyOpen: boolean;
  }> {
    return this.runLocked(async () => {
      const userDataDir = get1688UserDataDir();
      const existing = this.contexts.get(provider);
      if (this.loginSessionActive.has(provider) && existing && !existing.isClosed()) {
        const page = existing.pages()[0] ?? (await existing.newPage());
        await page.bringToFront().catch(() => undefined);
        return {
          profilePath: userDataDir,
          message: '采集浏览器已打开，请在窗口中完成 1688 登录或安全验证',
          alreadyOpen: true,
        };
      }

      this.loginSessionActive.add(provider);
      const context = await this.getOrCreateProviderContext(provider, { headless: false });
      const page = context.pages()[0] ?? (await context.newPage());
      await page.goto('https://www.1688.com/', {
        waitUntil: 'domcontentloaded',
        timeout: getDefaultNavigationTimeoutMs(),
      });

      return {
        profilePath: userDataDir,
        message: '已打开采集浏览器，请在此窗口完成 1688 登录与安全验证（普通 Chrome/Edge 登录无效）',
        alreadyOpen: false,
      };
    });
  }

  async check1688AuthStatus(): Promise<AuthStatus1688> {
    return this.runLocked(async () => {
      const lastCheckedAt = new Date().toISOString();
      const userDataDir = get1688UserDataDir();
      const timeoutMs = getDefaultNavigationTimeoutMs();

      const headless = this.loginSessionActive.has(PROVIDER_1688) ? false : true;
      const context = await this.getOrCreateProviderContext(PROVIDER_1688, { headless });

      const checkUrls = getAuthCheckUrls();
      let lastDebug: Parameters<typeof logAuthDebug>[0] | null = null;

      for (const checkUrl of checkUrls) {
        const page = await context.newPage();
        page.setDefaultNavigationTimeout(timeoutMs);
        page.setDefaultTimeout(timeoutMs);

        try {
          await page.goto(checkUrl, { waitUntil: 'domcontentloaded', timeout: timeoutMs });
          await page
            .waitForLoadState('networkidle', { timeout: Math.min(timeoutMs, 12_000) })
            .catch(() => undefined);

          const signals = await evaluateAuthPage(page);
          const cookieCount = (await context.cookies()).length;
          const authCookieCount = await countAuthCookies(context);
          const resolved = resolveAuthResult({ signals, authCookieCount });

          lastDebug = {
            userDataDir,
            checkUrl,
            finalUrl: page.url(),
            pageTitle: signals.pageTitle,
            loggedInHit: signals.loggedInHit,
            notLoggedInHit: signals.notLoggedInHit,
            verificationHit: signals.verificationHit,
            pageAnomaly: signals.pageAnomaly,
            cookieCount,
            authCookieCount,
            result: resolved.status,
            message: resolved.message,
          };
          logAuthDebug(lastDebug);

          if (signals.pageAnomaly) {
            await page.close().catch(() => undefined);
            continue;
          }

          await page.close().catch(() => undefined);
          return {
            provider: PROVIDER_1688,
            status: resolved.status,
            loggedIn: resolved.loggedIn,
            needVerification: resolved.needVerification,
            message: resolved.message,
            lastCheckedAt,
            profilePath: userDataDir,
          };
        } catch (e) {
          await page.close().catch(() => undefined);
          const err = e instanceof Error ? e.message : String(e);
          lastDebug = {
            userDataDir,
            checkUrl,
            finalUrl: '',
            pageTitle: '',
            loggedInHit: false,
            notLoggedInHit: false,
            verificationHit: /verify|captcha|验证/i.test(err),
            pageAnomaly: true,
            cookieCount: 0,
            authCookieCount: 0,
            result: 'unknown',
            message: `检测页异常：${err}`,
          };
          logAuthDebug(lastDebug);
        }
      }

      return {
        provider: PROVIDER_1688,
        status: 'unknown',
        loggedIn: false,
        needVerification: false,
        message: lastDebug?.message ?? '检测页异常，请重新检测',
        lastCheckedAt,
        profilePath: userDataDir,
      };
    });
  }

  async close(): Promise<void> {
    await this.runLocked(async () => {
      for (const [provider, context] of this.contexts.entries()) {
        if (!context.isClosed()) {
          await context.close().catch(() => undefined);
        }
        this.contexts.delete(provider);
      }
      this.loginSessionActive.clear();
    });
  }
}

/** @deprecated 使用 BrowserSessionManager；保留别名便于渐进迁移。 */
export class BrowserProfile1688 {
  constructor(private readonly sessions = new BrowserSessionManager()) {}

  get profilePath(): string {
    return this.sessions.get1688UserDataDir();
  }

  isLoginSessionActive(): boolean {
    return this.sessions.isLoginSessionActive();
  }

  withPage<T>(fn: (page: Page) => Promise<T>): Promise<T> {
    return this.sessions.withProviderPage(PROVIDER_1688, fn);
  }

  openLoginBrowser() {
    return this.sessions.openLoginBrowser(PROVIDER_1688);
  }

  checkAuthStatus() {
    return this.sessions.check1688AuthStatus();
  }

  close() {
    return this.sessions.close();
  }
}

export function get1688ProfileDir(): string {
  return get1688UserDataDir();
}
