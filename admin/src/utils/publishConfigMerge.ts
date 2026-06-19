import type { PublishConfigLayer } from '@/constants/publishConfig';
import { CONFIG_FIELD_LABEL, CONFIG_SOURCE_LABEL, labelForConfigValue } from '@/constants/publishConfig';
import type { PublishConfigOverrides } from '@/services/productPublish';

export type EffectivePublishConfig = {
  effectiveConfig: PublishConfigLayer;
  configSources: Record<string, string>;
};

function deepMergeLayer(
  dst: Record<string, unknown>,
  sources: Record<string, string>,
  layer: Record<string, unknown>,
  source: string,
  prefix: string,
) {
  for (const [k, v] of Object.entries(layer)) {
    if (v === undefined || v === null) continue;
    const path = prefix ? `${prefix}.${k}` : k;
    if (typeof v === 'object' && !Array.isArray(v)) {
      const existing = dst[k];
      if (existing && typeof existing === 'object' && !Array.isArray(existing)) {
        deepMergeLayer(existing as Record<string, unknown>, sources, v as Record<string, unknown>, source, path);
      } else {
        const clone: Record<string, unknown> = {};
        deepMergeLayer(clone, sources, v as Record<string, unknown>, source, path);
        dst[k] = clone;
      }
    } else if (v !== '') {
      dst[k] = v;
      sources[path] = source;
    }
  }
}

export function mergeEffectiveConfig(
  common: PublishConfigLayer | Record<string, unknown>,
  overrides: PublishConfigOverrides,
  productId: string,
  platform: string,
  shopId?: string,
): EffectivePublishConfig {
  const out: Record<string, unknown> = {};
  const sources: Record<string, string> = {};
  const plat = (platform || '').trim().toLowerCase();
  const sid = (shopId || '').trim();
  const pid = (productId || '').trim();

  const apply = (layer: Record<string, unknown> | undefined, source: string) => {
    if (!layer || Object.keys(layer).length === 0) return;
    deepMergeLayer(out, sources, layer, source, '');
  };

  apply(common as Record<string, unknown>, 'commonConfig');
  apply(overrides.products?.[pid] as Record<string, unknown>, 'productOverride');
  apply(overrides.platforms?.[plat] as Record<string, unknown>, 'platformOverride');
  if (sid) apply(overrides.shops?.[sid] as Record<string, unknown>, 'shopOverride');
  const ptKey = sid ? `${pid}:${plat}:${sid}` : `${pid}:${plat}`;
  apply(overrides.productTargets?.[ptKey] as Record<string, unknown>, 'productTargetOverride');

  return { effectiveConfig: out as PublishConfigLayer, configSources: sources };
}

export function configSourceLabel(source: string): string {
  return CONFIG_SOURCE_LABEL[source] ?? source;
}

export type ConfigReminder = {
  key: string;
  message: string;
  technical?: string;
};

export function detectConfigReminders(
  common: PublishConfigLayer,
  overrides: PublishConfigOverrides,
  productId: string,
  platform: string,
  shopId?: string,
  productTitle?: string,
  platformLabel?: string,
  shopName?: string,
): ConfigReminder[] {
  const reminders: ConfigReminder[] = [];
  const eff = mergeEffectiveConfig(common, overrides, productId, platform, shopId);
  const ctx = [productTitle || productId, platformLabel || platform, shopName].filter(Boolean).join(' / ');

  const pricePaths = ['price.strategy', 'price.markupValue', 'priceRule'];
  const priceSources = new Set(pricePaths.map((p) => eff.configSources[p]).filter(Boolean));
  if (priceSources.size > 1) {
    reminders.push({
      key: `price-multi-${productId}-${platform}-${shopId}`,
      message: `${ctx ? `「${ctx}」` : '该目标'}存在多层价格覆盖，最终以优先级最高的配置为准。`,
      technical: JSON.stringify(Object.fromEntries(pricePaths.map((p) => [p, eff.configSources[p]]))),
    });
  }

  const invStrategy = eff.effectiveConfig.inventory?.strategy;
  if (invStrategy && invStrategy !== 'skip_when_unknown') {
    reminders.push({
      key: `inv-unknown-${productId}`,
      message: `${ctx ? `「${ctx}」` : '该商品'}库存未知时不会自动跳过，请确认库存数据。`,
    });
  }

  const imgMain = eff.effectiveConfig.image?.mainImageStrategy;
  if (imgMain === 'platform_synced_only') {
    reminders.push({
      key: `img-sync-${productId}-${platform}`,
      message: `${ctx ? `「${ctx}」` : '该目标'}要求使用已同步平台图片，请确认图片已同步。`,
    });
  }

  const pkg = eff.effectiveConfig.package;
  if (!pkg?.weight && !eff.effectiveConfig.packageWeight) {
    reminders.push({
      key: `pkg-missing-${productId}`,
      message: `${ctx ? `「${ctx}」` : '该目标'}未设置包裹重量，将使用平台或店铺默认值。`,
    });
  }

  const hasShopOverride = shopId && overrides.shops?.[shopId];
  const ptKey = shopId ? `${productId}:${platform}:${shopId}` : `${productId}:${platform}`;
  const hasTargetOverride = overrides.productTargets?.[ptKey];
  if (hasShopOverride && hasTargetOverride) {
    reminders.push({
      key: `shop-target-${ptKey}`,
      message: `${ctx ? `「${ctx}」` : '该目标'}同时存在店铺覆盖与商品目标覆盖，商品目标覆盖优先生效。`,
    });
  }

  const minMargin = eff.effectiveConfig.price?.minProfitMargin;
  const markup = eff.effectiveConfig.price?.markupValue;
  if (minMargin != null && markup != null && markup < minMargin) {
    reminders.push({
      key: `profit-${productId}`,
      message: `${ctx ? `「${ctx}」` : '该目标'}加价规则可能低于利润保护线，请人工核对售价。`,
    });
  }

  return reminders;
}

export function flattenEffectiveForDisplay(eff: EffectivePublishConfig) {
  const rows: { field: string; label: string; value: string; source: string }[] = [];
  const cfg = eff.effectiveConfig as Record<string, unknown>;

  const walk = (obj: Record<string, unknown>, prefix: string) => {
    for (const [k, v] of Object.entries(obj)) {
      const path = prefix ? `${prefix}.${k}` : k;
      if (v && typeof v === 'object' && !Array.isArray(v)) {
        walk(v as Record<string, unknown>, path);
      } else if (v !== undefined && v !== null && v !== '') {
        rows.push({
          field: path,
          label: CONFIG_FIELD_LABEL[path] || path,
          value: labelForConfigValue(path, v),
          source: configSourceLabel(eff.configSources[path] || ''),
        });
      }
    }
  };
  walk(cfg, '');
  return rows;
}

export function productTargetKey(productId: string, platform: string, shopId?: string) {
  const plat = (platform || '').trim().toLowerCase();
  const sid = (shopId || '').trim();
  return sid ? `${productId}:${plat}:${sid}` : `${productId}:${plat}`;
}

export function simpleHash(input: string): string {
  let h = 0;
  for (let i = 0; i < input.length; i += 1) {
    h = (h << 5) - h + input.charCodeAt(i);
    h |= 0;
  }
  return Math.abs(h).toString(36);
}
