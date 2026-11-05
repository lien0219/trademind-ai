import { tr, trFormat } from '@/i18n';
import { Space, Tag, Typography } from 'antd';

import { shopCapabilityLabel } from '@/constants/shopCapabilities';
import { type PlatformProviderMeta } from '@/services/shops';

const PLATFORM_TAG_COLORS: Record<string, string> = {
  manual: 'default',
  tiktok: 'magenta',
  douyin_shop: 'volcano',
  shopee: 'orange',
  lazada: 'blue',
  amazon: 'gold',
};

const STANDARD_AUTH_KEYS = new Set([
  'appKey',
  'appSecret',
  'accessToken',
  'refreshToken',
  'sellerId',
  'merchantId',
  'marketplaceId',
  'redirectUri',
]);

export function isStandardAuthField(name: string) {
  return STANDARD_AUTH_KEYS.has(name);
}

export function providerLabel(list: PlatformProviderMeta[], platform: string) {
  const provider = list.find((item) => item.platform === platform);
  return provider ? `${provider.name} (${provider.platform})` : platform;
}

export function tagFromMap(raw: string, map: Record<string, { text: string; color: string }>) {
  const config = map[raw as keyof typeof map];
  if (!config) return <Tag>{raw}</Tag>;
  return <Tag color={config.color as never}>{tr(config.text)}</Tag>;
}

export function cellText(value?: string | null) {
  const text = (value ?? '').trim();
  return text ? text : <Typography.Text type="secondary">—</Typography.Text>;
}

export function renderPlatformCell(platform: string, providers: PlatformProviderMeta[]) {
  const provider = providers.find((item) => item.platform === platform);
  const label = provider?.name ?? platform;
  const color = PLATFORM_TAG_COLORS[platform] ?? 'processing';
  return (
    <Space size={4} wrap>
      <Tag color={color as never} style={{ margin: 0 }}>
        {label}
      </Tag>
      {provider?.status === 'beta' ? (
        <Tag color="processing" style={{ margin: 0 }}>
          {tr('Beta')}
        </Tag>
      ) : null}
      {provider?.status === 'planned' ? <Tag style={{ margin: 0 }}>{tr("规划中")}</Tag> : null}
    </Space>
  );
}

export function renderCapabilityTags(raw: unknown) {
  const capabilities = Array.isArray(raw) ? raw.map(String).filter(Boolean) : [];
  if (!capabilities.length) return <Typography.Text type="secondary">—</Typography.Text>;

  const visible = capabilities.slice(0, 2);
  const remaining = capabilities.length - visible.length;
  return (
    <Space size={[4, 4]} wrap>
      {visible.map((capability) => (
        <Tag key={capability} style={{ margin: 0 }}>
          {shopCapabilityLabel(capability)}
        </Tag>
      ))}
      {remaining > 0 ? <Tag style={{ margin: 0 }}>+{remaining}</Tag> : null}
    </Space>
  );
}

export function summarizeShopTest(result: {
  ok: boolean;
  message?: string;
  shopName?: string;
  externalShopId?: string;
  region?: string;
}) {
  const parts = [
    result.message,
    result.shopName ? trFormat("店铺 {shopName}", { shopName: result.shopName }) : '',
    result.region ? trFormat("地区 {region}", { region: result.region }) : '',
    result.externalShopId ? trFormat("平台店铺编号 {externalShopId}", { externalShopId: result.externalShopId }) : '',
  ].filter(Boolean);
  return parts.join(' · ') || tr("连接成功");
}

/** Build the API auth envelope without coupling the page to form field details. */
export function buildShopAuthPayload(values: Record<string, unknown>, fallbackAuthType?: string) {
  const authType = String(values.authType || fallbackAuthType || 'token');
  const payload: Record<string, unknown> = { authType };
  const authConfig: Record<string, unknown> = {};

  for (const [key, value] of Object.entries(values)) {
    if (key === 'authType' || value === undefined || value === null || value === '') continue;
    if (STANDARD_AUTH_KEYS.has(key) || key === 'expiresAt' || key === 'refreshExpiresAt') {
      payload[key] = value;
    } else {
      authConfig[key] = value;
    }
  }

  if (Object.keys(authConfig).length) payload.authConfig = authConfig;
  return payload;
}

/** Map incomplete platform Partner Open settings errors to actionable hints. */
export function formatPlatformPartnerErr(err: unknown): string {
  const msg = err instanceof Error ? err.message : String(err);
  const lowerMessage = msg.toLowerCase();

  if (msg.includes('required setting missing:')) {
    return trFormat("{msg}\n请先到「设置 → 平台接入设置」补齐该平台应用信息的必填项。", { msg: msg });
  }
  if (msg.includes('platform config incomplete: please configure platform_tiktok')) {
    return trFormat("{msg}\n请先到「设置 → 平台接入设置 → TikTok Shop」填写应用 Key、应用密钥和授权回调地址。", { msg: msg });
  }
  if (msg.includes('platform config incomplete: please configure platform_shopee')) {
    return trFormat("{msg}\n请先到「设置 → 平台接入设置 → Shopee」填写合作伙伴 ID、合作伙伴密钥和授权回调地址。", { msg: msg });
  }
  if (msg.includes('platform config incomplete: please configure platform_lazada')) {
    return trFormat("{msg}\n请先到「设置 → 平台接入设置 → Lazada」填写应用 Key、应用密钥和授权回调地址。", { msg: msg });
  }
  if (msg.includes('platform customer message permission denied') || msg.includes('platform customer message permission')) {
    return trFormat("{msg}\n平台客服权限不足，请确认已在 TikTok / Shopee / Lazada 等平台开放后台申请客服消息权限并重新授权；Amazon 请在 Seller Central / SP-API Developer Console 申请 Buyer-Seller Messaging（Messaging API）相关权限并重新授权店铺。", { msg: msg });
  }
  if (msg.includes('platform customer message provider not implemented')) {
    return trFormat("{msg}\n当前平台客服消息接口尚未接入，可使用模拟店铺验证拉取与发送联调。", { msg: msg });
  }
  if (msg.includes('manual shop does not support platform customer messages')) {
    return trFormat("{msg}\n手工店铺仅支持会话手工录入，不支持平台客服消息同步。", { msg: msg });
  }
  if (msg.includes('platform config incomplete: please configure platform_amazon')) {
    return trFormat("{msg}\n请先到「设置 → 平台接入设置 → Amazon SP-API」填写客户端 ID、客户端密钥、授权回调地址、站点 ID 和 SP-API 接口地址。", { msg: msg });
  }
  if (msg.includes('platform_amazon.lwa_auth_base_url and lwa_token_url')) {
    return trFormat("{msg}\n请在「Amazon SP-API」配置中补齐 LWA Auth Base URL 与 LWA Token URL。", { msg: msg });
  }
  if (msg.includes('TikTok 平台配置不完整，请先填写应用 Key、应用密钥和授权回调地址。')) {
    return trFormat("{msg}\n请先前往「设置 → 平台接入设置」填写 TikTok Shop（分组 platform_tiktok）必填项后再试。", { msg: msg });
  }
  if (lowerMessage.includes('tiktok platform config is incomplete') || lowerMessage.includes('platform_tiktok')) {
    return trFormat("{msg}\n请到「设置 → 平台接入设置」完成 TikTok Shop 必填项后再试。", { msg: msg });
  }
  return msg;
}
