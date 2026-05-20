/** 自定义采集器页面访问状态（用户可见文案） */
export const ACCESS_STATUS_LABELS: Record<string, { text: string; color: string }> = {
  public: { text: '可正常访问', color: 'success' },
  login_required: { text: '页面需要登录', color: 'warning' },
  verify_required: { text: '页面需要验证', color: 'error' },
  blocked: { text: '访问被拦截', color: 'error' },
  timeout: { text: '页面超时', color: 'default' },
  navigation_failed: { text: '页面无法打开', color: 'error' },
  unknown: { text: '状态未知', color: 'default' },
};

export function accessStatusLabel(status?: string): { text: string; color: string } {
  const key = (status ?? '').trim().toLowerCase();
  return ACCESS_STATUS_LABELS[key] ?? { text: '状态未知', color: 'default' };
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
