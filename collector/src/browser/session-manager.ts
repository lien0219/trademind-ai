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
  buildPinduoduoAuthCheckResult,
  evaluatePinduoduoAuthPage,
  evaluatePinduoduoProductEvidence,
  logPinduoduoAuthDebug,
  resolvePinduoduoAuthResult,
  type PinduoduoAuthCheckResult,
} from '../providers/sourcePinduoduo/auth-detect.js';
import {
  resolvePinduoduoAuthCheckPlan,
  resolvePinduoduoOpenLoginUrl,
} from '../providers/sourcePinduoduo/login-url.js';
import { PINDUODUO_PROFILE_KEY } from '../providers/sourcePinduoduo/profile.js';
import {
  buildTaobaoAuthCheckResult,
  evaluateTaobaoAuthPage,
  logTaobaoAuthDebug,
  resolveTaobaoAuthResult,
  type TaobaoAuthCheckResult,
} from '../providers/sourceTaobaoTmall/auth-detect.js';
import { TAOBAO_TMALL_PROFILE_KEY } from '../providers/sourceTaobaoTmall/profile.js';
import {
  ensureBrowserDataDirs,
  get1688UserDataDir,
  getPinduoduoUserDataDir,
  getTaobaoTmallUserDataDir,
} from './browser-paths.js';
import { PAGE_EVALUATE_POLYFILL } from './evaluate-in-page.js';
import { getBrowserHeadless, getDefaultNavigationTimeoutMs } from '../config/env.js';

const PROVIDER_1688 = '1688';
const PROVIDER_PINDUODUO = 'pinduoduo';
const PROVIDER_TAOBAO_TMALL = 'taobao_tmall';

export { PINDUODUO_PROFILE_KEY, TAOBAO_TMALL_PROFILE_KEY };

export type AuthStatusTaobaoTmall = TaobaoAuthCheckResult & {
  profilePath?: string;
};

export type AuthStatusPinduoduo = PinduoduoAuthCheckResult & {
  profilePath?: string;
};

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

const PDD_LOGIN_VIEWPORT = { width: 1280, height: 900 };

function persistentContextOptions(headless: boolean, provider: string = PROVIDER_1688) {
  if (provider === PROVIDER_PINDUODUO) {
    return {
      headless,
      locale: 'zh-CN' as const,
      userAgent: defaultUserAgent(),
      args: [
        '--disable-blink-features=AutomationControlled',
        `--window-size=${PDD_LOGIN_VIEWPORT.width},${PDD_LOGIN_VIEWPORT.height}`,
      ],
      viewport: PDD_LOGIN_VIEWPORT,
    };
  }
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

  getPinduoduoUserDataDir(): string {
    return getPinduoduoUserDataDir();
  }

  getTaobaoTmallUserDataDir(): string {
    return getTaobaoTmallUserDataDir();
  }

  private providerUserDataDir(provider: string): string {
    if (provider === PROVIDER_1688) return get1688UserDataDir();
    if (provider === PROVIDER_PINDUODUO) return getPinduoduoUserDataDir();
    if (provider === PROVIDER_TAOBAO_TMALL) return getTaobaoTmallUserDataDir();
    return `${get1688UserDataDir()}/../${provider}`;
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
    const userDataDir = this.providerUserDataDir(provider);

    const context = await chromium.launchPersistentContext(
      userDataDir,
      persistentContextOptions(wantHeadless, provider),
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
    const loginUrl =
      provider === PROVIDER_PINDUODUO
        ? 'https://mobile.yangkeduo.com/'
        : provider === PROVIDER_TAOBAO_TMALL
          ? 'https://www.taobao.com/'
          : 'https://www.1688.com/';
    const alreadyMsg =
      provider === PROVIDER_PINDUODUO
        ? '采集浏览器已打开，请在窗口中完成拼多多登录或安全验证'
        : provider === PROVIDER_TAOBAO_TMALL
          ? '采集浏览器已打开，请在窗口中完成淘宝/天猫登录或安全验证'
          : '采集浏览器已打开，请在窗口中完成 1688 登录或安全验证';
    const openedMsg =
      provider === PROVIDER_PINDUODUO
        ? '已打开拼多多采集浏览器。若跳转到微信页面，请用微信扫码完成授权（系统不保存账号密码），完成后点击「重新检测」。'
        : provider === PROVIDER_TAOBAO_TMALL
          ? '已打开淘宝/天猫采集浏览器。请在窗口中完成登录或安全验证（系统不保存账号密码），完成后点击「重新检测」。'
          : '已打开采集浏览器，请在此窗口完成 1688 登录与安全验证（普通 Chrome/Edge 登录无效）';

    return this.runLocked(async () => {
      const userDataDir = this.providerUserDataDir(provider);
      const existing = this.contexts.get(provider);
      if (this.loginSessionActive.has(provider) && existing && !existing.isClosed()) {
        const page = existing.pages()[0] ?? (await existing.newPage());
        await page.bringToFront().catch(() => undefined);
        return {
          profilePath: userDataDir,
          message: alreadyMsg,
          alreadyOpen: true,
        };
      }

      this.loginSessionActive.add(provider);
      const context = await this.getOrCreateProviderContext(provider, { headless: false });
      const page = context.pages()[0] ?? (await context.newPage());
      await page.goto(loginUrl, {
        waitUntil: 'domcontentloaded',
        timeout: getDefaultNavigationTimeoutMs(),
      });

      return {
        profilePath: userDataDir,
        message: openedMsg,
        alreadyOpen: false,
      };
    });
  }

  async openPinduoduoLoginBrowser(contextUrl?: string): Promise<{
    profilePath: string;
    message: string;
    alreadyOpen: boolean;
  }> {
    const { url, hasContext } = resolvePinduoduoOpenLoginUrl(contextUrl);
    const openedHint = hasContext
      ? '已打开目标商品页，请在采集浏览器中完成登录或微信授权，完成后点击「重新检测」。'
      : '已打开拼多多批发入口。建议从失败任务或采集弹窗打开具体商品链接登录；若跳转微信请扫码授权。';
    return this.runLocked(async () => {
      const userDataDir = getPinduoduoUserDataDir();
      const existing = this.contexts.get(PROVIDER_PINDUODUO);
      if (this.loginSessionActive.has(PROVIDER_PINDUODUO) && existing && !existing.isClosed()) {
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
          message:
            '采集浏览器已打开。若页面跳转到微信，请扫码完成授权后再点击「重新检测」。',
          alreadyOpen: true,
        };
      }

      this.loginSessionActive.add(PROVIDER_PINDUODUO);
      const context = await this.getOrCreateProviderContext(PROVIDER_PINDUODUO, { headless: false });
      const page = context.pages()[0] ?? (await context.newPage());
      await page.goto(url, {
        waitUntil: 'domcontentloaded',
        timeout: getDefaultNavigationTimeoutMs(),
      });

      return {
        profilePath: userDataDir,
        message: `${openedHint}（系统不保存账号密码）`,
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

  async checkPinduoduoAuthStatus(
    contextUrl?: string,
    settingsTestUrl?: string,
  ): Promise<AuthStatusPinduoduo> {
    return this.runLocked(async () => {
      const lastCheckedAt = new Date().toISOString();
      const userDataDir = getPinduoduoUserDataDir();
      const timeoutMs = getDefaultNavigationTimeoutMs();
      const headless = this.loginSessionActive.has(PROVIDER_PINDUODUO) ? false : true;
      const context = await this.getOrCreateProviderContext(PROVIDER_PINDUODUO, { headless });
      const plan = resolvePinduoduoAuthCheckPlan(contextUrl, settingsTestUrl);
      const checkUrl = plan.checkUrl;

      const page = await context.newPage();
      page.setDefaultNavigationTimeout(timeoutMs);
      page.setDefaultTimeout(timeoutMs);

      try {
        await page.goto(checkUrl, { waitUntil: 'domcontentloaded', timeout: timeoutMs });
        await page
          .waitForLoadState('networkidle', { timeout: Math.min(timeoutMs, 12_000) })
          .catch(() => undefined);

        const finalUrl = page.url();
        const signals = await evaluatePinduoduoAuthPage(page);
        const evidence = await evaluatePinduoduoProductEvidence(page);
        const resolved = resolvePinduoduoAuthResult({
          signals,
          evidence,
          finalUrl,
          checkUrl,
          checkMode: plan.mode,
        });

        const message =
          plan.hint && resolved.status === 'homepage_only'
            ? plan.hint
            : resolved.message;

        logPinduoduoAuthDebug({
          userDataDir,
          profileKey: PINDUODUO_PROFILE_KEY,
          checkUrl,
          finalUrl,
          checkMode: plan.mode,
          urlType: buildPinduoduoAuthCheckResult({
            status: resolved.status,
            loggedIn: resolved.loggedIn,
            needVerification: resolved.needVerification,
            message,
            lastCheckedAt,
            checkedUrl: checkUrl,
            finalUrl,
            checkMode: plan.mode,
            evidence,
          }).urlType,
          evidence,
          result: resolved.status,
          message,
        });

        await page.close().catch(() => undefined);
        return {
          ...buildPinduoduoAuthCheckResult({
            status: resolved.status,
            loggedIn: resolved.loggedIn,
            needVerification: resolved.needVerification,
            message,
            lastCheckedAt,
            checkedUrl: checkUrl,
            finalUrl,
            checkMode: plan.mode,
            evidence,
          }),
          profilePath: userDataDir,
        };
      } catch (e) {
        await page.close().catch(() => undefined);
        const err = e instanceof Error ? e.message : String(e);
        const emptyEvidence = {
          hasProductTitle: false,
          hasPrice: false,
          hasMainImage: false,
          hasLoginText: false,
          hasWechatAuth: /weixin\.qq\.com/i.test(err),
          hasAppRedirect: false,
        };
        logPinduoduoAuthDebug({
          userDataDir,
          profileKey: PINDUODUO_PROFILE_KEY,
          checkUrl,
          result: 'unknown',
          message: err,
        });
        return {
          ...buildPinduoduoAuthCheckResult({
            status: 'unknown',
            loggedIn: false,
            needVerification: /verify|captcha|验证/i.test(err),
            message: `检测页异常：${err}`,
            lastCheckedAt,
            checkedUrl: checkUrl,
            finalUrl: '',
            checkMode: plan.mode,
            evidence: emptyEvidence,
          }),
          profilePath: userDataDir,
        };
      }
    });
  }

  async openTaobaoTmallLoginBrowser(contextUrl?: string): Promise<{
    profilePath: string;
    message: string;
    alreadyOpen: boolean;
  }> {
    const loginUrl = contextUrl?.trim() || 'https://www.taobao.com/';
    return this.runLocked(async () => {
      const userDataDir = getTaobaoTmallUserDataDir();
      const existing = this.contexts.get(PROVIDER_TAOBAO_TMALL);
      if (this.loginSessionActive.has(PROVIDER_TAOBAO_TMALL) && existing && !existing.isClosed()) {
        const page = existing.pages()[0] ?? (await existing.newPage());
        await page.bringToFront().catch(() => undefined);
        if (contextUrl?.trim()) {
          await page.goto(loginUrl, {
            waitUntil: 'domcontentloaded',
            timeout: getDefaultNavigationTimeoutMs(),
          });
        }
        return {
          profilePath: userDataDir,
          message: '采集浏览器已打开，请在窗口中完成淘宝/天猫登录或安全验证',
          alreadyOpen: true,
        };
      }

      this.loginSessionActive.add(PROVIDER_TAOBAO_TMALL);
      const context = await this.getOrCreateProviderContext(PROVIDER_TAOBAO_TMALL, { headless: false });
      const page = context.pages()[0] ?? (await context.newPage());
      await page.goto(loginUrl, {
        waitUntil: 'domcontentloaded',
        timeout: getDefaultNavigationTimeoutMs(),
      });

      return {
        profilePath: userDataDir,
        message: contextUrl?.trim()
          ? '已打开目标商品页，请在采集浏览器中完成登录或安全验证，完成后点击「重新检测」。'
          : '已打开淘宝/天猫采集浏览器。建议从失败任务或采集弹窗打开具体商品链接登录。（系统不保存账号密码）',
        alreadyOpen: false,
      };
    });
  }

  async checkTaobaoTmallAuthStatus(contextUrl?: string): Promise<AuthStatusTaobaoTmall> {
    return this.runLocked(async () => {
      const lastCheckedAt = new Date().toISOString();
      const userDataDir = getTaobaoTmallUserDataDir();
      const timeoutMs = getDefaultNavigationTimeoutMs();
      const headless = this.loginSessionActive.has(PROVIDER_TAOBAO_TMALL) ? false : true;
      const context = await this.getOrCreateProviderContext(PROVIDER_TAOBAO_TMALL, { headless });
      const checkUrl = contextUrl?.trim() || 'https://www.taobao.com/';

      const page = await context.newPage();
      page.setDefaultNavigationTimeout(timeoutMs);
      page.setDefaultTimeout(timeoutMs);

      try {
        await page.goto(checkUrl, { waitUntil: 'domcontentloaded', timeout: timeoutMs });
        await page
          .waitForLoadState('networkidle', { timeout: Math.min(timeoutMs, 12_000) })
          .catch(() => undefined);

        const finalUrl = page.url();
        const signals = await evaluateTaobaoAuthPage(page);
        const resolved = resolveTaobaoAuthResult(signals);

        logTaobaoAuthDebug({
          userDataDir,
          profileKey: TAOBAO_TMALL_PROFILE_KEY,
          checkUrl,
          finalUrl,
          result: resolved.status,
          message: resolved.message,
        });

        await page.close().catch(() => undefined);
        return {
          ...buildTaobaoAuthCheckResult({
            status: resolved.status,
            loggedIn: resolved.loggedIn,
            needVerification: resolved.needVerification,
            message: resolved.message,
            lastCheckedAt,
            checkedUrl: checkUrl,
            finalUrl,
          }),
          profilePath: userDataDir,
        };
      } catch (e) {
        await page.close().catch(() => undefined);
        const err = e instanceof Error ? e.message : String(e);
        logTaobaoAuthDebug({
          userDataDir,
          profileKey: TAOBAO_TMALL_PROFILE_KEY,
          checkUrl,
          result: 'unknown',
          message: err,
        });
        return {
          ...buildTaobaoAuthCheckResult({
            status: 'unknown',
            loggedIn: false,
            needVerification: /verify|captcha|验证/i.test(err),
            message: `检测页异常：${err}`,
            lastCheckedAt,
            checkedUrl: checkUrl,
            finalUrl: '',
          }),
          profilePath: userDataDir,
        };
      }
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
