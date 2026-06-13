/** 安全设置字段文案 */

export const SECURITY_FIELD_LABEL = {
  sessionIdleTimeoutMin: '会话空闲超时',
  forceHttps: '强制 HTTPS',
  opsWebhookSecret: '运维回调签名密钥',
} as const;

export const SECURITY_FIELD_HELP = {
  sessionIdleTimeoutMin: '管理员无操作超过该时长后将自动退出登录，建议 30–120 分钟',
  forceHttps: '部署在 Nginx / Caddy 等反向代理后开启；仅当代理已正确转发 X-Forwarded-Proto 时生效',
  opsWebhookSecret: '用于校验外部运维回调请求的 HMAC 签名；无需对接回调通知可留空。保存后脱敏展示，留空则不修改',
} as const;

export const SECURITY_FIELD_PLACEHOLDER = {
  opsWebhookSecret: '保存后脱敏；留空则不修改',
} as const;

export const SECURITY_SESSION_TIMEOUT_PRESETS = [
  { label: '30 分钟', value: 30 },
  { label: '60 分钟', value: 60 },
  { label: '120 分钟', value: 120 },
  { label: '480 分钟（8 小时）', value: 480 },
] as const;
