/** Custom collector generic access status (not 1688-specific login probe). */
export const ACCESS_STATUS_LABELS: Record<string, { text: string; color: string }> = {
  public: { text: '可公开访问', color: 'success' },
  login_required: { text: '疑似需登录', color: 'warning' },
  verify_required: { text: '疑似验证码/风控', color: 'error' },
  blocked: { text: '访问被拦截', color: 'error' },
  timeout: { text: '页面超时', color: 'default' },
  navigation_failed: { text: '导航失败', color: 'error' },
  unknown: { text: '未知', color: 'default' },
};

export function accessStatusLabel(status?: string): { text: string; color: string } {
  const key = (status ?? '').trim().toLowerCase();
  return ACCESS_STATUS_LABELS[key] ?? { text: status || '—', color: 'default' };
}

export function accessStatusHint(status?: string): string | null {
  switch ((status ?? '').trim().toLowerCase()) {
    case 'login_required':
      return '该页面疑似需要登录，自定义采集器不一定能采集成功。请确认页面是否公开可访问，或使用带登录态的采集浏览器。';
    case 'verify_required':
      return '目标页面疑似触发验证码或风控，请稍后重试或降低采集频率。';
    default:
      return null;
  }
}
