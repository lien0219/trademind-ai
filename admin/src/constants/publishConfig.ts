/** 批量刊登配置字段与选项（Phase A2.2） */

export type PublishPriceConfig = {
  strategy?: string;
  markupType?: string;
  markupValue?: number;
  minProfitMargin?: number;
  decimalHandling?: string;
};

export type PublishImageConfig = {
  mainImageStrategy?: string;
  detailImageStrategy?: string;
  preferProcessedImages?: boolean;
  skipFailedImages?: boolean;
};

export type PublishInventoryConfig = {
  strategy?: string;
  safetyStock?: number;
  fixedQuantity?: number;
  outOfStockAction?: string;
};

export type PublishPackageConfig = {
  weight?: number;
  weightUnit?: string;
  length?: number;
  width?: number;
  height?: number;
  sizeUnit?: string;
};

export type PublishConfigLayer = {
  price?: PublishPriceConfig;
  image?: PublishImageConfig;
  inventory?: PublishInventoryConfig;
  package?: PublishPackageConfig;
  remark?: string;
  /** @deprecated A2 legacy */
  priceRule?: string;
  imageStrategy?: string;
  stockStrategy?: string;
  packageWeight?: number;
  packageSize?: string;
};

export const PRICE_STRATEGY_OPTIONS = [
  { label: '使用商品当前售价', value: 'use_current_price' },
  { label: '按成本加固定金额', value: 'cost_plus_fixed' },
  { label: '按成本加百分比', value: 'cost_plus_percent' },
  { label: '按倍率计算', value: 'multiplier' },
];

export const MARKUP_TYPE_OPTIONS = [
  { label: '固定金额', value: 'fixed' },
  { label: '百分比', value: 'percent' },
];

export const DECIMAL_HANDLING_OPTIONS = [
  { label: '四舍五入', value: 'round' },
  { label: '向上取整', value: 'ceil' },
  { label: '向下取整', value: 'floor' },
  { label: '保留两位小数', value: 'two_decimal' },
];

export const MAIN_IMAGE_STRATEGY_OPTIONS = [
  { label: '使用当前商品图片', value: 'use_current' },
  { label: '优先使用 AI 处理后的图片', value: 'prefer_ai_processed' },
  { label: '仅使用已同步到平台的图片', value: 'platform_synced_only' },
];

export const DETAIL_IMAGE_STRATEGY_OPTIONS = [
  { label: '使用当前商品图片', value: 'use_current' },
  { label: '优先使用 AI 处理后的图片', value: 'prefer_ai_processed' },
  { label: '仅使用已同步到平台的图片', value: 'platform_synced_only' },
  { label: '不使用详情图', value: 'skip' },
];

export const INVENTORY_STRATEGY_OPTIONS = [
  { label: '使用当前库存', value: 'use_current' },
  { label: '统一设置库存', value: 'fixed_quantity' },
  { label: '库存未知时跳过', value: 'skip_when_unknown' },
  { label: '库存未知时标记为需要检查', value: 'mark_needs_check' },
];

export const OUT_OF_STOCK_OPTIONS = [
  { label: '跳过刊登', value: 'skip' },
  { label: '标记为需要检查', value: 'mark_needs_check' },
  { label: '库存设为 0', value: 'zero' },
];

export const WEIGHT_UNIT_OPTIONS = [
  { label: '千克 (kg)', value: 'kg' },
  { label: '克 (g)', value: 'g' },
];

export const CONFIG_SOURCE_LABEL: Record<string, string> = {
  systemDefault: '系统默认',
  platformDefault: '平台默认',
  shopDefault: '店铺默认',
  commonConfig: '统一配置',
  productOverride: '商品覆盖',
  platformOverride: '平台覆盖',
  shopOverride: '店铺覆盖',
  productTargetOverride: '商品目标覆盖',
};

export const CONFIG_FIELD_LABEL: Record<string, string> = {
  'price.strategy': '价格策略',
  'price.markupType': '加价方式',
  'price.markupValue': '加价数值',
  'price.minProfitMargin': '最低利润率保护',
  'price.decimalHandling': '小数处理方式',
  'image.mainImageStrategy': '主图策略',
  'image.detailImageStrategy': '详情图策略',
  'image.preferProcessedImages': '优先使用已处理图片',
  'image.skipFailedImages': '跳过失败图片',
  'inventory.strategy': '库存策略',
  'inventory.safetyStock': '安全库存',
  'inventory.fixedQuantity': '统一库存',
  'inventory.outOfStockAction': '缺库存时处理方式',
  'package.weight': '包裹重量',
  'package.weightUnit': '重量单位',
  'package.length': '包裹长度',
  'package.width': '包裹宽度',
  'package.height': '包裹高度',
  'package.sizeUnit': '尺寸单位',
  remark: '备注',
  priceRule: '价格规则',
  imageStrategy: '图片策略',
  stockStrategy: '库存策略',
  packageWeight: '包裹重量',
  packageSize: '包裹尺寸',
};

export function labelForConfigValue(path: string, value: unknown): string {
  if (value === undefined || value === null || value === '') return '—';
  if (typeof value === 'boolean') return value ? '是' : '否';
  const optsMap: Record<string, { label: string; value: string }[]> = {
    'price.strategy': PRICE_STRATEGY_OPTIONS,
    'image.mainImageStrategy': MAIN_IMAGE_STRATEGY_OPTIONS,
    'image.detailImageStrategy': DETAIL_IMAGE_STRATEGY_OPTIONS,
    'inventory.strategy': INVENTORY_STRATEGY_OPTIONS,
    'inventory.outOfStockAction': OUT_OF_STOCK_OPTIONS,
    'price.decimalHandling': DECIMAL_HANDLING_OPTIONS,
    'package.weightUnit': WEIGHT_UNIT_OPTIONS,
  };
  const opts = optsMap[path];
  if (opts) {
    const hit = opts.find((o) => o.value === value);
    if (hit) return hit.label;
  }
  return String(value);
}

export function countConfigFields(config: Record<string, unknown> | undefined): number {
  if (!config) return 0;
  let n = 0;
  const walk = (obj: Record<string, unknown>) => {
    for (const [k, v] of Object.entries(obj)) {
      if (v === undefined || v === null || v === '') continue;
      if (typeof v === 'object' && !Array.isArray(v)) {
        walk(v as Record<string, unknown>);
      } else {
        n += 1;
      }
    }
  };
  walk(config);
  return n;
}

export function emptyPublishConfig(): PublishConfigLayer {
  return {};
}

export function validatePublishConfigClient(config: PublishConfigLayer): string | null {
  const price = config.price;
  if (price?.markupValue != null && price.markupValue < 0) {
    return '加价数值不能为负数。';
  }
  if (price?.strategy === 'multiplier' && price.markupValue != null && price.markupValue <= 0) {
    return '倍率必须大于 0。';
  }
  if (price?.minProfitMargin != null && price.minProfitMargin < 0) {
    return '最低利润率不能为负数。';
  }
  const inv = config.inventory;
  if (inv?.safetyStock != null && inv.safetyStock < 0) {
    return '安全库存不能小于 0。';
  }
  if (inv?.fixedQuantity != null && (inv.fixedQuantity < 0 || !Number.isInteger(inv.fixedQuantity))) {
    return '统一库存必须是大于或等于 0 的整数。';
  }
  const pkg = config.package;
  for (const [k, v] of Object.entries({ weight: pkg?.weight, length: pkg?.length, width: pkg?.width, height: pkg?.height })) {
    if (v != null && (v <= 0 || v > 10000)) {
      return `包裹${k === 'weight' ? '重量' : '尺寸'}必须是 0 到 10000 之间的正数。`;
    }
  }
  return null;
}
