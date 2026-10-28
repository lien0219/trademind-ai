/** 自定义采集器页面访问状态（用户可见文案） */
const ACCESS_STATUS_TEXT = {
  public: '可正常访问',
  loginRequired: '页面需要登录',
  verifyRequired: '页面需要验证',
  blocked: '访问被拦截',
  timeout: '页面超时',
  navigationFailed: '页面无法打开',
  unknown: '状态未知',
} as const;

export const ACCESS_STATUS_LABELS: Record<string, { text: string; color: string }> = {
  public: { text: ACCESS_STATUS_TEXT.public, color: 'success' },
  login_required: { text: ACCESS_STATUS_TEXT.loginRequired, color: 'warning' },
  verify_required: { text: ACCESS_STATUS_TEXT.verifyRequired, color: 'error' },
  blocked: { text: ACCESS_STATUS_TEXT.blocked, color: 'error' },
  timeout: { text: ACCESS_STATUS_TEXT.timeout, color: 'default' },
  navigation_failed: { text: ACCESS_STATUS_TEXT.navigationFailed, color: 'error' },
  unknown: { text: ACCESS_STATUS_TEXT.unknown, color: 'default' },
};

export function accessStatusLabel(status?: string): { text: string; color: string } {
  const key = (status ?? '').trim().toLowerCase();
  return ACCESS_STATUS_LABELS[key] ?? { text: ACCESS_STATUS_TEXT.unknown, color: 'default' };
}

export function accessStatusHint(status?: string): string | null {
  switch ((status ?? '').trim().toLowerCase()) {
    case 'login_required':
      return '当前商品页跳转到了登录页面，请先使用采集浏览器登录后再测试。';
    case 'verify_required':
      return '目标网站可能出现验证码或安全验证，请稍后重试，或在采集浏览器中手动完成验证。';
    default:
      return null;
  }
}
