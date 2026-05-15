/**
 * 环境变量（由 docker / systemd / .env 注入，不写入代码默认值中的密钥）。
 */
export function getHttpPort(): number {
  const raw = process.env.COLLECTOR_HTTP_ADDR ?? ':3100';
  const n = Number(String(raw).replace(/^\:/, ''));
  return Number.isFinite(n) && n > 0 ? n : 3100;
}

export function getDefaultNavigationTimeoutMs(): number {
  const n = Number(process.env.COLLECTOR_GOTO_TIMEOUT_MS ?? '45000');
  return Number.isFinite(n) && n > 0 ? n : 45000;
}

export function getBrowserHeadless(): boolean {
  const v = process.env.COLLECTOR_HEADLESS;
  if (v === '0' || v === 'false') return false;
  return true;
}
