/** 邮件服务提供商（存库 provider 字段） */
export const MAIL_PROVIDER_META: Record<string, { label: string; desc: string }> = {
  smtp: {
    label: 'SMTP',
    desc: '企业邮箱、QQ/网易授权码、SendGrid / Mailgun 等标准 SMTP',
  },
};

export const MAIL_PROVIDER_OPTIONS = Object.entries(MAIL_PROVIDER_META).map(([value, meta]) => ({
  value,
  label: meta.label,
}));

/** 常用 SMTP 端口说明 */
export const SMTP_PORT_HINT: Record<number, string> = {
  465: '通常配合 SSL（SMTPS）',
  587: '通常配合 STARTTLS',
  25: '部分自建服务器（不推荐公网直连）',
};

export function smtpPortHint(port?: number | null): string {
  if (port == null || !Number.isFinite(port)) return '常见端口：465（SSL）、587（STARTTLS）';
  return SMTP_PORT_HINT[port] || '请按邮件服务商文档选择端口与加密方式';
}

/** 表单字段中文标签（不暴露 item_key） */
export const MAIL_FIELD_LABEL = {
  host: 'SMTP 服务器',
  port: 'SMTP 端口',
  username: '登录账号',
  password: '密码 / 授权码',
  from: '发件人邮箱',
  fromName: '发件人名称',
  ssl: 'SSL 加密（SMTPS）',
  tls: 'STARTTLS 加密',
} as const;

export const MAIL_FIELD_PLACEHOLDER = {
  host: 'smtp.example.com',
  port: '465',
  username: '通常与发件人邮箱相同',
  password: '保存后脱敏显示；留空则不修改已存密码',
  from: 'noreply@example.com',
  fromName: '贸灵 TradeMind',
  testTo: 'test@example.com',
} as const;
