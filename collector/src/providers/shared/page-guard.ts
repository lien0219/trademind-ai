import type { Page } from 'playwright';
import { evaluateAuthPage } from '../source1688/auth-detect.js';

export function is1688CollectHost(host: string): boolean {
  const h = host.trim().toLowerCase();
  return h === '1688.com' || h.endsWith('.1688.com');
}

export function isCaptchaRedirectUrl(href: string): boolean {
  try {
    const u = new URL(href);
    const blob = `${u.hostname}${u.pathname}${u.search}`.toLowerCase();
    if (/punish|x5secdata|captcha|_____tmd_____|sec\.1688\.com\/.*(?:verify|captcha)/i.test(blob)) {
      return true;
    }
    if (/passport\.1688\.com|login\.1688\.com/i.test(u.hostname)) {
      return true;
    }
  } catch {
    return false;
  }
  return false;
}

const VERIFICATION_TITLE_RE =
  /验证码|安全验证|滑块|人机验证|访问受限|请登录|欢迎登录|登录页|账号登录|立即登录|1688首页|验证中心|验证测试/i;

export function isVerificationPlaceholderTitle(title: string): boolean {
  const t = title.trim();
  if (!t || t.length < 3) return true;
  return VERIFICATION_TITLE_RE.test(t);
}

export function isLoginRedirectUrl(href: string): boolean {
  try {
    const u = new URL(href);
    const host = u.hostname.toLowerCase();
    const path = `${u.pathname}${u.search}`.toLowerCase();
    if (/passport\.|login\.|signin|sign-in|auth\./i.test(host)) return true;
    if (/\.jd\.com$/i.test(host) && /\/login|\/passport|newlogin/i.test(path)) return true;
    if (/\.1688\.com$/i.test(host) && /passport|login/i.test(path)) return true;
  } catch {
    return false;
  }
  return false;
}

/** Generic login / gate pages (JD, Taobao, etc.) when not using a logged-in profile. */
export async function assertGenericPageCollectible(page: Page): Promise<void> {
  const href = page.url();
  if (isLoginRedirectUrl(href) || isCaptchaRedirectUrl(href)) {
    throwBlocked('login_or_verification_redirect');
  }
  const snapshot = await page.evaluate(() => {
    const title = document.title?.trim() ?? '';
    const body = document.body?.innerText?.slice(0, 4000) ?? '';
    const hasPrice = !!document.querySelector(
      '[class*="price"], [class*="Price"], [data-price], .p-price, .sku-price',
    );
    const hasGallery = !!document.querySelector(
      'img[src*="360buy"], img[src*="jd.com"], [class*="gallery"], [class*="main-image"], #spec-img',
    );
    return { title, body, hasPrice, hasGallery };
  });
  if (isVerificationPlaceholderTitle(snapshot.title)) {
    throwBlocked('verification_page_title');
  }
  const loginWall =
    /欢迎登录|请登录|账号登录|立即登录|扫码登录|密码登录/.test(snapshot.body) &&
    snapshot.body.length < 2500 &&
    !snapshot.hasPrice &&
    !snapshot.hasGallery;
  if (loginWall) {
    throwBlocked('login_wall_detected');
  }
}

export function throwBlocked(reason: string): never {
  throw new Error(`PAGE_BLOCKED_OR_VERIFY_REQUIRED:${reason}`);
}

/** Detect 1688 login / captcha surfaces before treating generic selectors as product data. */
export async function assert1688PageCollectible(page: Page): Promise<void> {
  const href = page.url();
  if (isCaptchaRedirectUrl(href)) {
    throwBlocked('verification_or_login_page_detected');
  }
  const signals = await evaluateAuthPage(page);
  if (signals.verificationHit) {
    throwBlocked('verification_required');
  }
  const title = signals.pageTitle.trim();
  if (isVerificationPlaceholderTitle(title)) {
    throwBlocked('verification_page_title');
  }
}
