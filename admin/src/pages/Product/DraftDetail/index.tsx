import type { CSSProperties, ReactNode } from 'react';
import type { UploadRequestOption } from 'rc-upload/lib/interface';
import { formatDateTime } from '@/utils/formatTime';
import type { ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails, TaskJsonBlock } from '@/components/ui';
import { commonStatusLabel, publishModeLabel, readinessLevelLabel } from '@/constants/copywriting';
import { formatUserErrorMessage } from '@/constants/errorMessages';
import { EditableProTable, ModalForm, ProForm, ProFormDigit, ProFormSelect, ProFormText, ProFormTextArea, ProTable } from '@ant-design/pro-components';
import {
  Button,
  Card,
  Col,
  Collapse,
  Descriptions,
  Drawer,
  Form,
  Image,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Radio,
  Row,
  Select,
  Space,
  Spin,
  Tabs,
  Tag,
  Tooltip,
  Typography,
  Alert,
  Upload,
  Table,
  message,
  Flex,
} from 'antd';
import {
  DeleteOutlined,
  PlusOutlined,
  PictureOutlined,
  RobotOutlined,
  UnorderedListOutlined,
  StarOutlined,
  ThunderboltOutlined,
  SyncOutlined,
  CloudUploadOutlined,
  ReloadOutlined,
  EyeOutlined,
} from '@ant-design/icons';
import { ProductCollectQualityAlert } from '@/components/ProductCollectQualityAlert';
import { isPinduoduoSource } from '@/utils/pinduoduoCollectAlerts';
import { isTaobaoTmallSource } from '@/utils/taobaoTmallCollectAlerts';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { PRODUCT_STATUS, PLATFORM_PROVIDER_STATUS } from '@/constants/status';
import {
  PRODUCT_IMAGE_OBJECT_KEY_LABEL,
  PRODUCT_IMAGE_ORIGIN_URL_LABEL,
  PRODUCT_IMAGE_PUBLIC_URL_LABEL,
  PRODUCT_IMAGE_SORT_ORDER_LABEL,
  PRODUCT_IMAGE_URL_LABEL,
} from '@/constants/userFriendly';
import { uploadFile } from '@/services/files';
import {
  applyAiDescription,
  applyProductAITitle,
  buildDouyinDraftMapping,
  createProductImage,
  createProductSku,
  deleteProduct,
  deleteProductImage,
  deleteProductSku,
  fetchProductAITasks,
  fetchProductDetail,
  generateDescription,
  optimizeProductTitle,
  reorderProductImages,
  syncProductImages,
  selectBestMainProductImages,
  getProductPlatformPublishConfig,
  getDouyinDraftMapping,
  putProductPlatformPublishConfig,
  retryDouyinImage,
  saveDouyinDraftMapping,
  validateDouyinDraftMapping,
  uploadDouyinImages,
  updateProduct,
  updateProductImage,
  updateProductSku,
  updateProductSkuStockSettings,
  type AITaskRow,
  type GenerateDescriptionResult,
  type OptimizeTitleResult,
  type ProductDetail,
  type DouyinDraftImage,
  type DouyinDraftMapping,
  type DouyinMappingIssue,
  type ProductImageRow,
  type ProductSKURow,
} from '@/services/products';
import { Link } from '@umijs/renderer-react';
import {
  listProductPublications,
  publishProduct,
  createDouyinProductDraft,
  listDouyinPublishTasks,
  retryProductPublishTask,
  getDouyinSkuBindings,
  syncDouyinSkuBindings,
  bindDouyinSku,
  unbindDouyinSku,
  type ProductPublicationRow,
  type ProductPublishTaskDTO,
  type DouyinSkuBindingSummary,
  type DouyinSkuBindingRow,
  type DouyinPlatformSkuCandidate,
} from '@/services/productPublish';
import { getProductReadiness, type ProductReadinessResult, type ReadinessCheckItem } from '@/services/productReadiness';
import PricingApplyModal from '@/components/PricingApplyModal';
import { CreateImageTaskModal, type CreateImageTaskPrefill } from '@/components/CreateImageTaskModal';
import { TranslateImageTextModal, type TranslateImageTextPrefill } from '@/components/TranslateImageTextModal';
import { queryPlatformProviders, queryShops, type PlatformProviderMeta, type ShopListRow } from '@/services/shops';
import {
  queryDouyinCategories,
  queryDouyinCategoryAttributes,
  syncDouyinCategories,
  syncDouyinCategoryAttributes,
  type DouyinCategoryAttribute,
  type DouyinCategoryNode,
} from '@/services/douyinCategories';
import {
  adjustSkuStock,
  batchUpdateStockSettings,
  createInventorySyncBatch,
  listProductPublicationSkus,
  previewBatchStockSettings,
  querySkuInventoryLogs,
  syncPublicationSkuInventory,
  type InventoryChangeLogRow,
  type PublicationSkuListingRow,
} from '@/services/inventory';

function inventorySyncRunnable(cap?: string): boolean {
  const c = (cap || '').trim().toLowerCase();
  return c === 'available' || c === 'beta';
}

const PLATFORM_LABELS: Record<string, string> = {
  douyin_shop: '抖店',
  tiktok: 'TikTok',
  shopee: 'Shopee',
  lazada: 'Lazada',
  amazon: 'Amazon',
  mock: 'Mock',
};

function platformDisplayName(platform?: string): string {
  const key = (platform || '').trim().toLowerCase();
  if (!key) return '—';
  return PLATFORM_LABELS[key] ?? platform ?? '—';
}

function inventorySyncCapabilityTag(cap?: string) {
  if (!cap) return '—';
  const key = cap.trim().toLowerCase() as keyof typeof PLATFORM_PROVIDER_STATUS;
  const meta = PLATFORM_PROVIDER_STATUS[key];
  if (meta) return <Tag color={meta.color}>{meta.text}</Tag>;
  return <Tag>{cap}</Tag>;
}

const SKU_BATCH_STOCK_MAX_HINT = 500;

/** 从采集归一化 JSON（products.raw_data）读取 attributes / attributeCandidates */
function collectedAttributesFromRaw(rawData: unknown): Record<string, string> {
  if (!rawData || typeof rawData !== 'object') return {};
  const root = rawData as Record<string, unknown>;
  const pick = (obj: unknown): Record<string, string> => {
    if (!obj || typeof obj !== 'object' || Array.isArray(obj)) return {};
    const out: Record<string, string> = {};
    for (const [k, v] of Object.entries(obj as Record<string, unknown>)) {
      const key = String(k).trim();
      if (!key) continue;
      if (typeof v === 'string') {
        const val = v.trim();
        if (val) out[key] = val;
      } else if (v != null && (typeof v === 'number' || typeof v === 'boolean')) {
        out[key] = String(v);
      }
    }
    return out;
  };
  const fromTop = pick(root.attributes);
  if (Object.keys(fromTop).length) return fromTop;
  const nested = root.raw;
  if (nested && typeof nested === 'object') {
    return pick((nested as Record<string, unknown>).attributeCandidates);
  }
  return {};
}

function collectQualityWarningsFromRaw(rawData: unknown): string[] {
  if (!rawData || typeof rawData !== 'object') return [];
  const root = rawData as Record<string, unknown>;
  const raw = root.raw;
  if (!raw || typeof raw !== 'object') return [];
  const w = (raw as Record<string, unknown>).qualityWarnings;
  if (!Array.isArray(w)) return [];
  return w.filter((x): x is string => typeof x === 'string' && x.trim().length > 0);
}

/** @deprecated use collectQualityWarningsFromRaw */
function customQualityWarningsFromRaw(rawData: unknown): string[] {
  return collectQualityWarningsFromRaw(rawData);
}

function isCustomCollectIncomplete(data: ProductDetail | null): boolean {
  if (!data || data.source !== 'custom') return false;
  const mainCount = (data.images ?? []).filter((i) => i.imageType === 'main').length;
  const skuCount = (data.skus ?? []).length;
  const attrCount = Object.keys(collectedAttributesFromRaw(data.rawData)).length;
  const raw = data.rawData as Record<string, unknown> | undefined;
  const inner = raw?.raw as Record<string, unknown> | undefined;
  const hasPrice = inner?.productPrice != null;
  return !hasPrice || mainCount <= 1 || skuCount === 0 || attrCount === 0;
}

function isPinduoduoProduct(data: ProductDetail | null): boolean {
  return !!data && isPinduoduoSource(data.source);
}

function isTaobaoTmallProduct(data: ProductDetail | null): boolean {
  return !!data && isTaobaoTmallSource(data.source);
}

function formatInventorySyncTaskCreateError(e: unknown): string {
  const s = (e instanceof Error ? e.message : String(e)).trim() || '提交失败';
  const hints: string[] = [];
  if (/missing warehouse_id|platform inventory config incomplete:\s*missing warehouse_id/i.test(s)) {
    hints.push(
      'TikTok Shop：请到「设置 → 平台刊登配置 → TikTok Shop」填写默认仓库 ID。',
    );
    hints.push(
      'Shopee：请到「设置 → 平台刊登配置 → Shopee」填写默认仓库 ID。',
    );
    hints.push(
      'Lazada：若平台提示与仓库相关，请到「设置 → 平台刊登配置 → Lazada」填写默认仓库代码。',
    );
    hints.push('高级用户可在库存同步任务参数中覆盖默认仓库设置。');
  }
  if (/platform inventory config incomplete:\s*missing (marketplace_id|fulfillment_channel|product_type)/i.test(s)) {
    hints.push(
      'Amazon：请到「设置 → 平台刊登配置 → Amazon」补齐 Marketplace ID、Fulfillment Channel、Product Type；也可在库存同步任务的 options 中逐项覆盖。',
    );
  }
  if (/platform inventory sync permission denied/i.test(s)) {
    hints.push(
      '请确认已在平台侧申请库存 / 商品更新相关权限并重新授权店铺（TikTok Shop Partner Center 或 Shopee Open Platform）。',
    );
    hints.push(
      'Lazada：请确认已在 Lazada Open Platform / Seller Center 申请商品 / 库存更新相关权限并重新授权店铺。',
    );
    hints.push(
      'Amazon：请确认已在 Amazon Seller Central / SP-API Developer Console 申请 Listings / Inventory 相关权限并重新授权。',
    );
  }
  if (/platform config incomplete:\s*please configure settings\.platform_tiktok/i.test(s)) {
    hints.push('请到「设置 → 平台接入设置 → TikTok Shop」补齐平台应用信息。');
  }
  if (/platform config incomplete:\s*please configure settings\.platform_shopee/i.test(s)) {
    hints.push('请到「设置 → 平台接入设置 → Shopee」补齐平台应用信息。');
  }
  if (/platform config incomplete:\s*please configure settings\.platform_lazada/i.test(s)) {
    hints.push('请到「设置 → 平台接入设置 → Lazada」补齐平台应用信息。');
  }
  if (/platform config incomplete:\s*please configure settings\.platform_amazon|please configure platform_amazon/i.test(s)) {
    hints.push('请到「设置 → 平台接入设置 → Amazon」补齐应用信息。');
  }
  if (/DOUYIN_SKU_NOT_BOUND|external sku id missing/i.test(s)) {
    hints.push('该规格尚未绑定抖店规格，请先完成规格绑定后再同步库存。');
  }
  if (/DOUYIN_SKU_BINDING_AMBIGUOUS/i.test(s)) {
    hints.push('该规格存在多个候选抖店 SKU，匹配结果不明确，请人工确认绑定后再同步库存。');
  }
  if (/DOUYIN_PRODUCT_NOT_BOUND|external product id missing/i.test(s)) {
    hints.push('该商品还没有绑定抖店商品 ID。请先在「刊登」Tab 完成抖店商品草稿创建。');
  }
  if (/DOUYIN_INVENTORY_SYNC_NOT_READY|inventory_sync_enabled=false/i.test(s)) {
    hints.push('请到「设置 → 平台接入设置 → 抖店」开启「开启库存同步」后再试。');
  }
  if (/DOUYIN_INVENTORY_PERMISSION_DENIED|DOUYIN_PERMISSION_DENIED/i.test(s)) {
    hints.push('请在抖店开放平台申请商品/库存更新权限并重新授权店铺。');
  }
  if (/DOUYIN_STORE_NOT_AUTHORIZED|DOUYIN_AUTH_EXPIRED|shop is not authorized/i.test(s)) {
    hints.push('抖店店铺未授权或授权已过期，请到「店铺管理」重新完成店铺授权。');
  }
  return hints.length ? `${s}\n${hints.join('\n')}` : s;
}
type SKUEditable = ProductSKURow & { attrsText?: string };

const PRODUCT_STATUS_OPTIONS = Object.entries(PRODUCT_STATUS).map(([value, v]) => ({
  label: v.text,
  value,
}));

const IMAGE_TYPE_OPTIONS = [
  { label: '主图', value: 'main' },
  { label: '详情图', value: 'detail' },
  { label: '规格图', value: 'sku' },
];

function attrsToText(attrs?: Record<string, unknown>): string {
  if (!attrs || typeof attrs !== 'object') return '';
  try {
    return JSON.stringify(attrs);
  } catch {
    return '';
  }
}

function imageTypeLabel(t: string): string {
  if (t === 'main') return '主图';
  if (t === 'detail' || t === 'description') return '详情图';
  if (t === 'sku') return '规格图';
  if (t === 'marketing') return '营销图';
  if (t === 'ai_generated') return 'AI 图';
  return t;
}

const IMAGE_META_TAG_STYLE: CSSProperties = {
  margin: 0,
  fontSize: 12,
  lineHeight: '20px',
  padding: '0 6px',
  borderRadius: 4,
};

function ProductImageMetaTags({ row }: { row: ProductImageRow }) {
  const tags: ReactNode[] = [];
  if (row.isBestMain) {
    tags.push(
      <Tag key="best" color="gold" bordered={false} style={IMAGE_META_TAG_STYLE}>
        最佳主图
      </Tag>,
    );
  }
  if (row.source === 'ai') {
    tags.push(
      <Tag key="ai" color="processing" bordered={false} style={IMAGE_META_TAG_STYLE}>
        AI 生成
      </Tag>,
    );
  } else if (row.source === 'upload') {
    tags.push(
      <Tag key="upload" bordered={false} style={IMAGE_META_TAG_STYLE}>
        上传
      </Tag>,
    );
  } else if (row.source === 'collect') {
    tags.push(
      <Tag key="collect" color="default" bordered={false} style={IMAGE_META_TAG_STYLE}>
        采集
      </Tag>,
    );
  }
  if (tags.length === 0) return null;
  return (
    <Space size={[6, 4]} wrap style={{ marginTop: 2 }}>
      {tags}
    </Space>
  );
}

function ProductImageTypeCell({ row }: { row: ProductImageRow }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4, padding: '2px 0' }}>
      <Typography.Text strong style={{ fontSize: 13, lineHeight: '20px' }}>
        {imageTypeLabel(String(row.imageType ?? ''))}
      </Typography.Text>
      <ProductImageMetaTags row={row} />
    </div>
  );
}

function draftStockStatusTag(raw?: string) {
  if (!raw) return '—';
  const m: Record<string, { t: string; c: string }> = {
    normal: { t: '正常', c: 'green' },
    low_stock: { t: '低库存', c: 'orange' },
    out_of_stock: { t: '售罄', c: 'red' },
    below_safety_stock: { t: '低于安全线', c: 'gold' },
  };
  const x = m[raw];
  if (!x) return <Tag>{raw}</Tag>;
  return <Tag color={x.c}>{x.t}</Tag>;
}

/** When stock_status 尚未落库，用阈值在前端推导展示（与后端 CalculateSKUStockStatus 一致）。 */
function effectiveStockStatus(r: ProductSKURow): string {
  if (r.stockStatus) return r.stockStatus;
  const stock = typeof r.stock === 'number' ? r.stock : 0;
  const warn = typeof r.warningStock === 'number' ? r.warningStock : 5;
  const safe = typeof r.safetyStock === 'number' ? r.safetyStock : 0;
  if (stock <= 0) return 'out_of_stock';
  if (safe > 0 && stock <= safe) return 'below_safety_stock';
  if (stock <= warn) return 'low_stock';
  return 'normal';
}

const READINESS_GROUP_LABEL: Record<string, string> = {
  product: '商品信息',
  sku: '商品规格',
  image: '图片',
  inventory: '库存',
  collect: '采集提示',
  platform: '平台配置',
};

function readinessStatusTag(r: ProductReadinessResult | null) {
  if (!r) return null;
  if (r.status === 'blocked' || (r.errorCount ?? 0) > 0) {
    return <Tag color="red">阻止发布</Tag>;
  }
  if (r.status === 'warning' || (r.warningCount ?? 0) > 0) {
    return <Tag color="orange">有警告</Tag>;
  }
  return <Tag color="green">可发布</Tag>;
}

function readinessLevelTag(level?: string) {
  const l = (level || '').toLowerCase();
  if (l === 'error') return <Tag color="red">{readinessLevelLabel(level)}</Tag>;
  if (l === 'warning') return <Tag color="orange">{readinessLevelLabel(level)}</Tag>;
  return <Tag>{readinessLevelLabel(level)}</Tag>;
}

function readinessCheckList(items: ReadinessCheckItem[], limit?: number) {
  const list = limit != null ? items.slice(0, limit) : items;
  return list.map((c, i) => (
    <div key={`${c.code}-${i}`} style={{ marginBottom: 6 }}>
      {readinessLevelTag(c.level)} {c.message}
    </div>
  ));
}

function douyinIssueTag(level?: string) {
  return <Tag color={level === 'error' ? 'red' : 'orange'}>{level === 'error' ? '校验失败' : '需要确认'}</Tag>;
}

function tagFromPublishStatus(raw?: string) {
  const s = String(raw || '').toLowerCase();
  const label = commonStatusLabel(s);
  if (s === 'success') return <Tag color="green">{label}</Tag>;
  if (s === 'failed') return <Tag color="red">{label}</Tag>;
  if (s === 'running' || s === 'pending') return <Tag color="blue">{label}</Tag>;
  if (s === 'cancelled') return <Tag>{label}</Tag>;
  return <Tag>{label}</Tag>;
}

function douyinMoney(v?: number, currency = 'CNY') {
  return typeof v === 'number' ? `${currency} ${v.toFixed(2)}` : '未填写';
}

function douyinAttrValueText(v: unknown) {
  if (v == null || v === '') return '未填写';
  if (Array.isArray(v)) return v.join(', ');
  if (typeof v === 'object') {
    try {
      return JSON.stringify(v);
    } catch {
      return String(v);
    }
  }
  return String(v);
}

function douyinIssueList(items?: DouyinMappingIssue[]) {
  if (!items?.length) return null;
  return (
    <Space direction="vertical" style={{ width: '100%' }} size={4}>
      {items.map((x, i) => (
        <Tooltip key={`${x.code}-${i}`} title={x.code ? `内部编号：${x.code}` : undefined}>
          <div>
            {douyinIssueTag(x.level)} {x.message}
          </div>
        </Tooltip>
      ))}
    </Space>
  );
}

function douyinImageKey(img: DouyinDraftImage, typ: string, idx: number) {
  return img.localImageId || img.storageKey || img.platformImageId || `${typ}:${idx}`;
}

function douyinBindStatusTag(status?: string) {
  const st = String(status || '').toLowerCase();
  if (st === 'bound') return <Tag color="green">已绑定</Tag>;
  if (st === 'skipped') return <Tag color="blue">已跳过</Tag>;
  if (st === 'ambiguous') return <Tag color="orange">待确认</Tag>;
  if (st === 'unmatched') return <Tag color="red">未匹配</Tag>;
  if (st === 'failed') return <Tag color="red">失败</Tag>;
  return <Tag>未校准</Tag>;
}

function douyinBindStatusHint(status?: string): string {
  const st = String(status || '').toLowerCase();
  if (st === 'bound' || st === 'skipped') return '已绑定，可同步库存。';
  if (st === 'unmatched') return '未匹配到抖店 SKU，请手动绑定后再同步库存。';
  if (st === 'ambiguous') return '找到多个可能的抖店 SKU，请人工确认。';
  if (st === 'failed') return '校准失败，请稍后重试或手动绑定。';
  return '尚未校准，请先执行校准或手动绑定。';
}

function douyinSkuSyncBlocked(row: PublicationSkuListingRow): boolean {
  const isDouyin = (row.platform || '').toLowerCase() === 'douyin_shop';
  if (!isDouyin) return false;
  const st = String(row.bindStatus || '').toLowerCase();
  const hasBinding = Boolean((row.externalProductId || '').trim()) && Boolean((row.externalSkuId || '').trim());
  if (!hasBinding) return true;
  return st === 'ambiguous' || st === 'unmatched' || st === 'failed';
}

function douyinImageStatusTag(img: DouyinDraftImage) {
  const st = img.uploadStatus || (img.platformImageId ? 'uploaded' : img.needSync ? 'pending' : 'pending');
  if (st === 'uploaded') return <Tag color="green">已上传抖店</Tag>;
  if (st === 'failed') return <Tag color="red">上传失败</Tag>;
  if (st === 'processing') return <Tag color="blue">上传中</Tag>;
  if (img.needSync) return <Tag color="orange">待同步 Storage</Tag>;
  return <Tag color="orange">待上传</Tag>;
}

function douyinStorageStatusTag(img: DouyinDraftImage) {
  return img.storageKey || img.objectKey || img.storageUrl || img.publicUrl ? <Tag color="green">Storage 已就绪</Tag> : <Tag color="orange">需先同步 Storage</Tag>;
}

function douyinImagePreviewUrl(img: DouyinDraftImage) {
  return img.storageUrl || img.publicUrl || img.url || img.originUrl || img.platformImageUrl || '';
}

function InventorySyncPlatformHint({ platform }: { platform?: string }) {
  const p = (platform || '').trim().toLowerCase();
  if (p === 'tiktok') {
    return (
      <>
        <Typography.Paragraph type="secondary" style={{ marginTop: 0, marginBottom: 8 }}>
          TikTok 会使用「设置 → 平台刊登配置 → TikTok Shop」中的默认仓库。若推送失败并提示权限不足，请在 TikTok Shop
          Partner Center 申请库存更新相关权限后重新授权店铺。
        </Typography.Paragraph>
        <TechnicalDetails label="高级参数说明">
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0, fontSize: 12 }}>
            可在任务参数 options.warehouse_id 中覆盖默认仓库 ID。
          </Typography.Paragraph>
        </TechnicalDetails>
      </>
    );
  }
  if (p === 'shopee') {
    return (
      <>
        <Typography.Paragraph type="secondary" style={{ marginTop: 0, marginBottom: 8 }}>
          Shopee 默认按总库存更新。若你的卖家中心要求按仓/位置维护库存，请在「设置 → 平台刊登配置 → Shopee」填写默认仓库
          ID。若推送失败并提示权限不足，请在 Shopee Open Platform 申请库存/商品更新相关权限后重新授权店铺。
        </Typography.Paragraph>
        <TechnicalDetails label="高级参数说明">
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0, fontSize: 12 }}>
            Open API 字段：normal_stock、seller_stock[].location_id；任务参数 options.warehouse_id / location_id 可覆盖默认仓库。
          </Typography.Paragraph>
        </TechnicalDetails>
      </>
    );
  }
  if (p === 'lazada') {
    return (
      <>
        <Typography.Paragraph type="secondary" style={{ marginTop: 0, marginBottom: 8 }}>
          Lazada 通过 Open Platform 更新可售数量。多仓店铺请在「设置 → 平台刊登配置 → Lazada」填写默认仓库代码。若推送失败并提示权限不足，请在
          Lazada Open Platform / Seller Center 申请库存/商品更新相关权限后重新授权店铺。
        </Typography.Paragraph>
        <TechnicalDetails label="高级参数说明">
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0, fontSize: 12 }}>
            接口 price_quantity；仓库字段 WarehouseCode / warehouse_id；任务参数 options.warehouse_id 可覆盖默认仓库。
          </Typography.Paragraph>
        </TechnicalDetails>
      </>
    );
  }
  if (p === 'douyin_shop') {
    return (
      <Typography.Paragraph type="secondary" style={{ marginTop: 0, marginBottom: 12 }}>
        抖店库存同步会更新各规格的可售数量。请确认「设置 → 平台接入设置 → 抖店」已开启「开启库存同步」，且刊登草稿中已写入抖店商品编号与平台规格编号。若规格编号为空，请先在「刊登」Tab
        完成抖店商品草稿创建。
      </Typography.Paragraph>
    );
  }
  return null;
}

function fixLinkForReadinessCode(code: string): { tab?: string; href?: string; label: string } | null {
  const c = (code || '').toLowerCase();
  if (c.startsWith('product.title') || c === 'product.currency_missing' || c === 'product.archived') {
    return { tab: 'basic', label: '去基础信息' };
  }
  if (c.startsWith('product.description')) {
    return { tab: 'ai', label: '去 AI 描述' };
  }
  if (c.startsWith('sku.')) {
    return { tab: 'skus', label: '去商品规格' };
  }
  if (c.startsWith('image.')) {
    return { tab: 'images', label: '去图片' };
  }
  if (c.startsWith('inventory.')) {
    return { tab: 'inventory', label: '去库存' };
  }
  if (c.startsWith('platform.shop') || c === 'platform.shop_inactive' || c === 'platform.shop_not_authorized') {
    return { href: '/shops', label: '店铺管理' };
  }
  if (c.startsWith('platform.')) {
    return { href: '/settings/platform-publish', label: '平台刊登配置' };
  }
  return null;
}

export default function ProductDraftDetailPage() {
  const id = decodeURIComponent(window.location.pathname.split('/').filter(Boolean).pop() ?? '');
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<ProductDetail | null>(null);
  const [err, setErr] = useState<string>();
  const [aiOpen, setAiOpen] = useState(false);
  const [aiBusy, setAiBusy] = useState(false);
  const [aiResult, setAiResult] = useState<OptimizeTitleResult | null>(null);
  const [aiTasks, setAiTasks] = useState<AITaskRow[]>([]);
  const [aiForm] = Form.useForm();
  const [descOpen, setDescOpen] = useState(false);
  const [descBusy, setDescBusy] = useState(false);
  const [descResult, setDescResult] = useState<GenerateDescriptionResult | null>(null);
  const [descForm] = Form.useForm();
  const [skuRows, setSkuRows] = useState<SKUEditable[]>([]);
  const [imgModalOpen, setImgModalOpen] = useState(false);
  const [imgEdit, setImgEdit] = useState<ProductImageRow | null>(null);
  const [imgBusy, setImgBusy] = useState(false);
  const [lastUpload, setLastUpload] = useState<{ id: string; url: string; objectKey: string } | null>(null);
  const [createImageOpen, setCreateImageOpen] = useState(false);
  const [createImagePrefill, setCreateImagePrefill] = useState<CreateImageTaskPrefill>({});
  const [translateImageOpen, setTranslateImageOpen] = useState(false);
  const [translateImagePrefill, setTranslateImagePrefill] = useState<TranslateImageTextPrefill>({});
  const [translateSourceImage, setTranslateSourceImage] = useState<ProductImageRow | undefined>();

  const [pubRows, setPubRows] = useState<ProductPublicationRow[]>([]);
  const [pubCtxLoading, setPubCtxLoading] = useState(false);
  const [platformsMeta, setPlatformsMeta] = useState<PlatformProviderMeta[]>([]);
  const [shopsList, setShopsList] = useState<ShopListRow[]>([]);
  const [publishForm] = Form.useForm();
  const [douyinForm] = Form.useForm();
  const [douyinMappingForm] = Form.useForm();
  const [publishSubmitting, setPublishSubmitting] = useState(false);
  const [douyinSaving, setDouyinSaving] = useState(false);
  const [douyinMapping, setDouyinMapping] = useState<DouyinDraftMapping | null>(null);
  const [douyinMappingLoading, setDouyinMappingLoading] = useState(false);
  const [douyinMappingSaving, setDouyinMappingSaving] = useState(false);
  const [douyinMappingValidating, setDouyinMappingValidating] = useState(false);
  const [douyinImageUploading, setDouyinImageUploading] = useState(false);
  const [douyinImageRetryingKey, setDouyinImageRetryingKey] = useState('');
  const [douyinDraftCreating, setDouyinDraftCreating] = useState(false);
  const [douyinPublishTasks, setDouyinPublishTasks] = useState<ProductPublishTaskDTO[]>([]);
  const [douyinPublishTasksLoading, setDouyinPublishTasksLoading] = useState(false);
  const [douyinSkuBinding, setDouyinSkuBinding] = useState<DouyinSkuBindingSummary | null>(null);
  const [douyinSkuBindingLoading, setDouyinSkuBindingLoading] = useState(false);
  const [douyinSkuBindingSyncing, setDouyinSkuBindingSyncing] = useState(false);
  const [douyinSkuBindOpen, setDouyinSkuBindOpen] = useState(false);
  const [douyinSkuBindTarget, setDouyinSkuBindTarget] = useState<DouyinSkuBindingRow | null>(null);
  const [douyinSkuBindSubmitting, setDouyinSkuBindSubmitting] = useState(false);
  const [douyinSkuCandidatesOpen, setDouyinSkuCandidatesOpen] = useState(false);
  const [douyinSkuBindForm] = Form.useForm<{ platformSkuId: string; platformSkuName?: string }>();
  const [douyinCategoryLoading, setDouyinCategoryLoading] = useState(false);
  const [douyinAttrLoading, setDouyinAttrLoading] = useState(false);
  const [douyinCategoryFlat, setDouyinCategoryFlat] = useState<DouyinCategoryNode[]>([]);
  const [douyinAttrs, setDouyinAttrs] = useState<DouyinCategoryAttribute[]>([]);
  const [douyinConfig, setDouyinConfig] = useState<{
    shopId?: string;
    categoryId?: string;
    categoryPath?: string;
    platformAttributes?: Record<string, unknown>;
  }>({});

  const [draftTabKey, setDraftTabKey] = useState('basic');
  const [readinessPlat, setReadinessPlat] = useState<string>('tiktok');
  const [readinessShopId, setReadinessShopId] = useState<string>('');
  const [readinessResult, setReadinessResult] = useState<ProductReadinessResult | null>(null);
  const [readinessLoading, setReadinessLoading] = useState(false);
  const [publishReadiness, setPublishReadiness] = useState<ProductReadinessResult | null>(null);
  const [publishReadinessLoading, setPublishReadinessLoading] = useState(false);

  const [pubSkuRows, setPubSkuRows] = useState<PublicationSkuListingRow[]>([]);
  const [pubSkuLoading, setPubSkuLoading] = useState(false);
  const [pubSkuBulkPlatformFilter, setPubSkuBulkPlatformFilter] = useState('');
  const [pubSkuSelectedKeys, setPubSkuSelectedKeys] = useState<string[]>([]);
  const [adjustOpen, setAdjustOpen] = useState(false);
  const [adjustTarget, setAdjustTarget] = useState<ProductSKURow | null>(null);
  const [adjustForm] = Form.useForm();
  const [invAdjustSubmitting, setInvAdjustSubmitting] = useState(false);
  const [logsOpen, setLogsOpen] = useState(false);
  const [logsSku, setLogsSku] = useState<ProductSKURow | null>(null);
  const [logsRows, setLogsRows] = useState<InventoryChangeLogRow[]>([]);
  const [logsLoading, setLogsLoading] = useState(false);
  const [syncOpen, setSyncOpen] = useState(false);
  const [syncRow, setSyncRow] = useState<PublicationSkuListingRow | null>(null);
  const [syncForm] = Form.useForm();
  const [syncSubmitting, setSyncSubmitting] = useState(false);
  const [stockSettingsOpen, setStockSettingsOpen] = useState(false);
  const [stockSettingsTarget, setStockSettingsTarget] = useState<ProductSKURow | null>(null);
  const [pricingOpen, setPricingOpen] = useState(false);
  const [stockSettingsForm] = Form.useForm<{ warningStock: number; safetyStock: number }>();
  const [stockSettingsSubmitting, setStockSettingsSubmitting] = useState(false);
  const [skuBatchStockOpen, setSkuBatchStockOpen] = useState(false);
  const [skuBatchScope, setSkuBatchScope] = useState<'selected' | 'all'>('all');
  const [skuBatchSelKeys, setSkuBatchSelKeys] = useState<string[]>([]);
  const [skuBatchMatched, setSkuBatchMatched] = useState<number | null>(null);
  const [skuBatchPreviewLoading, setSkuBatchPreviewLoading] = useState(false);
  const [skuBatchStockForm] = Form.useForm<{ warningStock: number; safetyStock: number }>();

  const collectedAttrs = useMemo(
    () => collectedAttributesFromRaw(data?.rawData),
    [data?.rawData],
  );

  const collectQualityWarnings = useMemo(
    () => {
      const fromDetail = (data?.collectWarnings ?? []).filter((x) => String(x).trim());
      const fromRaw = collectQualityWarningsFromRaw(data?.rawData);
      return Array.from(new Set([...fromDetail, ...fromRaw]));
    },
    [data?.collectWarnings, data?.rawData],
  );

  const imageSyncSummary = useMemo(() => {
    const rows = data?.images ?? [];
    const external = rows.filter((img) => {
      const url = String(img.originUrl || img.publicUrl || '').trim();
      return /^https?:\/\//i.test(url) && !String(img.objectKey || img.storageKey || '').trim();
    });
    return {
      total: rows.length,
      external: external.length,
      synced: rows.filter((img) => String(img.objectKey || img.storageKey || '').trim()).length,
      externalMain: external.filter((img) => img.imageType === 'main').length,
      externalDetail: external.filter((img) => img.imageType === 'detail' || img.imageType === 'description').length,
    };
  }, [data?.images]);

  const skuMappingPreview = useMemo(
    () =>
      (data?.skus ?? []).map((sku) => ({
        id: sku.id,
        skuCode: sku.skuCode || sku.id,
        skuName: sku.skuName || '默认规格',
        price: sku.price,
        stock: sku.stock,
        attrs: sku.attrs,
      })),
    [data?.skus],
  );

  const showCustomIncompleteHint = useMemo(() => isCustomCollectIncomplete(data), [data]);

  const openCreateImageTask = useCallback(
    (prefill: CreateImageTaskPrefill) => {
      setCreateImagePrefill({
        productId: id,
        ...prefill,
      });
      setCreateImageOpen(true);
    },
    [id],
  );

  const openTranslateImageText = useCallback((image: ProductImageRow) => {
    setTranslateSourceImage(image);
    setTranslateImagePrefill({
      productId: id,
      sourceImageId: image.id,
      sourceImageUrl: (image.publicUrl || image.originUrl || '').trim(),
    });
    setTranslateImageOpen(true);
  }, [id]);

  const openQuickImageTask = useCallback(
    (image: ProductImageRow, taskType: string, provider?: string) => {
      openCreateImageTask({
        taskType,
        sourceImageId: image.id,
        sourceImageUrl: (image.publicUrl || image.originUrl || '').trim(),
        imageSourceMode: 'product',
        provider: provider ?? (taskType === 'remove_background' ? 'removebg' : ''),
      });
    },
    [openCreateImageTask],
  );

  const runSelectBestMain = useCallback(
    async (mode: 'score_only' | 'recommend' | 'auto_set') => {
      if (!id) return;
      try {
        await selectBestMainProductImages(id, { mode });
        message.success('已提交自动选主图任务');
      } catch (e: unknown) {
        message.error((e as Error)?.message || '提交失败');
      }
    },
    [id],
  );

  const reloadDetail = useCallback(async () => {
    if (!id) return;
    const d = await fetchProductDetail(id);
    setData(d);
    setSkuRows(
      (d.skus ?? []).map((s) => ({
        ...s,
        attrsText: attrsToText(s.attrs),
      })),
    );
  }, [id]);

  const reloadTasks = useCallback(async () => {
    if (!id) return;
    try {
      const { list } = await fetchProductAITasks(id);
      setAiTasks(list ?? []);
    } catch {
      setAiTasks([]);
    }
  }, [id]);

  const reloadPublishContext = useCallback(async () => {
    if (!id) return;
    setPubCtxLoading(true);
    try {
      const [pubs, prov, shops, douyinCfg, douyinCats] = await Promise.all([
        listProductPublications(id),
        queryPlatformProviders(),
        queryShops({ page: 1, pageSize: 500, authStatus: 'authorized' }),
        getProductPlatformPublishConfig(id, 'douyin_shop').catch(() => undefined),
        queryDouyinCategories({ onlyLeaf: true }).catch(() => undefined),
      ]);
      setPubRows(Array.isArray(pubs.list) ? pubs.list : []);
      setPlatformsMeta(Array.isArray(prov.list) ? prov.list : []);
      setShopsList(Array.isArray(shops.list) ? shops.list : []);
      if (douyinCats?.flat) setDouyinCategoryFlat(douyinCats.flat);
      if (douyinCfg) {
        const attrs = (douyinCfg.platformAttributes && typeof douyinCfg.platformAttributes === 'object'
          ? douyinCfg.platformAttributes
          : {}) as Record<string, unknown>;
        setDouyinConfig({
          shopId: douyinCfg.shopId,
          categoryId: douyinCfg.categoryId,
          categoryPath: douyinCfg.categoryPath,
          platformAttributes: attrs,
        });
        douyinForm.setFieldsValue({
          shopId: douyinCfg.shopId,
          categoryId: douyinCfg.categoryId,
          platformAttributes: attrs,
        });
        if (douyinCfg.mapping) {
          setDouyinMapping(douyinCfg.mapping);
          douyinMappingForm.setFieldsValue({
            title: douyinCfg.mapping.title,
            description: douyinCfg.mapping.description,
          });
        } else {
          const mapped = await getDouyinDraftMapping(id).catch(() => undefined);
          setDouyinMapping(mapped ?? null);
          if (mapped) {
            douyinMappingForm.setFieldsValue({
              title: mapped.title,
              description: mapped.description,
            });
          }
        }
        if (douyinCfg.categoryId) {
          const ar = await queryDouyinCategoryAttributes(douyinCfg.categoryId).catch(() => undefined);
          setDouyinAttrs(ar?.list ?? []);
        }
      }
    } catch {
      setPubRows([]);
    } finally {
      setPubCtxLoading(false);
    }
  }, [douyinForm, douyinMappingForm, id]);

  const reloadDouyinCategories = useCallback(
    async (shopId?: string, refresh?: boolean) => {
      setDouyinCategoryLoading(true);
      try {
        if (refresh) {
          const sid = String(shopId || douyinConfig.shopId || '').trim();
          if (!sid) {
            message.warning('请先选择已授权抖店店铺');
            return;
          }
          await syncDouyinCategories(sid);
          message.success('抖店类目已刷新');
        }
        const res = await queryDouyinCategories({ onlyLeaf: true });
        setDouyinCategoryFlat(res.flat ?? []);
      } catch (e: unknown) {
        message.error((e as Error)?.message || '加载抖店类目失败');
      } finally {
        setDouyinCategoryLoading(false);
      }
    },
    [douyinConfig.shopId],
  );

  const reloadDouyinAttrs = useCallback(
    async (categoryId?: string, shopId?: string, refresh?: boolean) => {
      const cid = String(categoryId || douyinConfig.categoryId || '').trim();
      if (!cid) {
        setDouyinAttrs([]);
        return;
      }
      setDouyinAttrLoading(true);
      try {
        if (refresh) {
          const sid = String(shopId || douyinConfig.shopId || '').trim();
          if (!sid) {
            message.warning('请先选择已授权抖店店铺');
            return;
          }
          const res = await syncDouyinCategoryAttributes(cid, sid);
          setDouyinAttrs(res.list ?? []);
          message.success('抖店属性已刷新');
          return;
        }
        const res = await queryDouyinCategoryAttributes(cid);
        setDouyinAttrs(res.list ?? []);
      } catch (e: unknown) {
        message.error((e as Error)?.message || '加载抖店属性失败');
      } finally {
        setDouyinAttrLoading(false);
      }
    },
    [douyinConfig.categoryId, douyinConfig.shopId],
  );

  const selectedDouyinCategory = useMemo(
    () => douyinCategoryFlat.find((c) => c.categoryId === douyinConfig.categoryId),
    [douyinCategoryFlat, douyinConfig.categoryId],
  );

  const currentDouyinMapping = useCallback((): DouyinDraftMapping => {
    const text = douyinMappingForm.getFieldsValue() as { title?: string; description?: string };
    const vals = douyinForm.getFieldsValue() as {
      shopId?: string;
      categoryId?: string;
      platformAttributes?: Record<string, unknown>;
    };
    const attrValues = vals.platformAttributes ?? douyinConfig.platformAttributes ?? {};
    const attrs = (douyinMapping?.attributes ?? douyinAttrs.map((a) => ({
      attrId: a.attrId,
      name: a.name,
      required: a.required,
      valueType: a.valueType,
      options: a.options,
    }))).map((a) => ({
      ...a,
      value: attrValues[a.attrId] ?? attrValues[a.name] ?? a.value,
    }));
    return {
      ...(douyinMapping ?? { platform: 'douyin_shop' }),
      platform: 'douyin_shop',
      productId: id,
      shopId: vals.shopId || douyinConfig.shopId || douyinMapping?.shopId,
      categoryId: vals.categoryId || douyinConfig.categoryId || douyinMapping?.categoryId,
      categoryPath: selectedDouyinCategory?.path || douyinConfig.categoryPath || douyinMapping?.categoryPath,
      title: text.title ?? douyinMapping?.title ?? '',
      description: text.description ?? douyinMapping?.description ?? '',
      attributes: attrs,
    };
  }, [douyinAttrs, douyinConfig, douyinForm, douyinMapping, douyinMappingForm, id, selectedDouyinCategory?.path]);

  const handleBuildDouyinMapping = useCallback(async () => {
    setDouyinMappingLoading(true);
    try {
      const vals = await douyinForm.validateFields();
      const cat = douyinCategoryFlat.find((x) => x.categoryId === vals.categoryId);
      const saved = await putProductPlatformPublishConfig(id, 'douyin_shop', {
        shopId: vals.shopId,
        categoryId: vals.categoryId,
        categoryPath: cat?.path || douyinConfig.categoryPath,
        platformAttributes: vals.platformAttributes ?? {},
      });
      setDouyinConfig({
        shopId: saved.shopId,
        categoryId: saved.categoryId,
        categoryPath: saved.categoryPath,
        platformAttributes: saved.platformAttributes ?? {},
      });
      const mapped = await buildDouyinDraftMapping(id, { shopId: vals.shopId });
      setDouyinMapping(mapped);
      douyinMappingForm.setFieldsValue({ title: mapped.title, description: mapped.description });
      message.success('抖店刊登草稿已生成');
    } catch (e: unknown) {
      message.error((e as Error)?.message || '生成抖店刊登草稿失败');
    } finally {
      setDouyinMappingLoading(false);
    }
  }, [douyinCategoryFlat, douyinConfig.categoryPath, douyinForm, douyinMappingForm, id]);

  const handleSaveDouyinMapping = useCallback(async () => {
    if (!douyinMapping) {
      message.warning('请先生成抖店刊登草稿');
      return;
    }
    setDouyinMappingSaving(true);
    try {
      await douyinMappingForm.validateFields();
      const saved = await saveDouyinDraftMapping(id, currentDouyinMapping());
      setDouyinMapping(saved);
      douyinMappingForm.setFieldsValue({ title: saved.title, description: saved.description });
      message.success('抖店刊登草稿已保存');
    } catch (e: unknown) {
      message.error((e as Error)?.message || '保存抖店刊登草稿失败');
    } finally {
      setDouyinMappingSaving(false);
    }
  }, [currentDouyinMapping, douyinMapping, douyinMappingForm, id]);

  const handleValidateDouyinMapping = useCallback(async () => {
    setDouyinMappingValidating(true);
    try {
      const res = await validateDouyinDraftMapping(id, douyinMapping ? currentDouyinMapping() : undefined);
      setDouyinMapping((cur) => cur ? {
        ...cur,
        errors: res.checks.filter((x) => x.level === 'error'),
        warnings: res.checks.filter((x) => x.level !== 'error'),
      } : cur);
      if (res.errorCount > 0) {
        message.error('这些信息不完整，暂时不能创建抖店商品');
      } else if (res.warningCount > 0) {
        message.warning('抖店刊登草稿还有需要确认的信息');
      } else {
        message.success('抖店刊登草稿校验通过');
      }
      setReadinessPlat('douyin_shop');
      setReadinessShopId(String(douyinForm.getFieldValue('shopId') || ''));
    } catch (e: unknown) {
      message.error((e as Error)?.message || '校验抖店刊登草稿失败');
    } finally {
      setDouyinMappingValidating(false);
    }
  }, [currentDouyinMapping, douyinForm, douyinMapping, id]);

  const handleUploadDouyinImages = useCallback(async (force = false) => {
    if (!douyinMapping) {
      message.warning('请先生成抖店刊登草稿');
      return;
    }
    setDouyinImageUploading(true);
    try {
      const res = await uploadDouyinImages(id, {
        imageTypes: ['main', 'detail'],
        retryFailed: true,
        force,
      });
      setDouyinMapping(res.mapping);
      message.success(`图片上传完成：成功 ${res.summary.uploaded}，失败 ${res.summary.failed}`);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '上传图片到抖店失败');
    } finally {
      setDouyinImageUploading(false);
    }
  }, [douyinMapping, id]);

  const handleRetryDouyinImage = useCallback(async (imageKey: string) => {
    setDouyinImageRetryingKey(imageKey);
    try {
      const res = await retryDouyinImage(id, imageKey);
      setDouyinMapping(res.mapping);
      message.success('图片重试完成');
    } catch (e: unknown) {
      message.error((e as Error)?.message || '重试图片上传失败');
    } finally {
      setDouyinImageRetryingKey('');
    }
  }, [id]);

  const reloadPublicationSkus = useCallback(async () => {
    if (!id) return;
    setPubSkuLoading(true);
    try {
      const res = await listProductPublicationSkus(id);
      setPubSkuRows(res.list ?? []);
    } catch {
      setPubSkuRows([]);
    } finally {
      setPubSkuLoading(false);
    }
  }, [id]);

  const filteredPubSkuRowsForBulk = useMemo(() => {
    const pf = pubSkuBulkPlatformFilter.trim().toLowerCase();
    if (!pf) return pubSkuRows;
    return pubSkuRows.filter((r) => (r.platform || '').toLowerCase() === pf);
  }, [pubSkuRows, pubSkuBulkPlatformFilter]);

  const buildSkuStockPayload = useCallback(() => {
    const base: { productId: string; includeNormal: boolean; productSkuIds?: string[] } = {
      productId: id,
      includeNormal: true,
    };
    if (skuBatchScope === 'selected' && skuBatchSelKeys.length > 0) {
      base.productSkuIds = skuBatchSelKeys;
    }
    return base;
  }, [id, skuBatchScope, skuBatchSelKeys]);

  const runSkuBatchPreview = useCallback(async () => {
    if (!id) return;
    setSkuBatchPreviewLoading(true);
    try {
      const res = await previewBatchStockSettings({
        ...buildSkuStockPayload(),
        page: 1,
        pageSize: 10,
      });
      setSkuBatchMatched(res.matchedCount);
    } catch (e) {
      setSkuBatchMatched(null);
      message.error((e as Error)?.message || '预览失败');
    } finally {
      setSkuBatchPreviewLoading(false);
    }
  }, [id, buildSkuStockPayload]);

  useEffect(() => {
    if (!skuBatchStockOpen) return;
    void runSkuBatchPreview();
  }, [skuBatchStockOpen, skuBatchScope, skuBatchSelKeys, runSkuBatchPreview]);

  useEffect(() => {
    setPubSkuSelectedKeys([]);
  }, [pubSkuBulkPlatformFilter]);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    (async () => {
      setLoading(true);
      setErr(undefined);
      try {
        const d = await fetchProductDetail(id);
        if (!cancelled) {
          setData(d);
          setSkuRows(
            (d.skus ?? []).map((s) => ({
              ...s,
              attrsText: attrsToText(s.attrs),
            })),
          );
        }
        if (!cancelled) {
          try {
            const { list } = await fetchProductAITasks(id);
            if (!cancelled) setAiTasks(list ?? []);
          } catch {
            if (!cancelled) setAiTasks([]);
          }
        }
      } catch (e) {
        if (!cancelled) setErr(e instanceof Error ? e.message : String(e));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [id]);

  useEffect(() => {
    void reloadPublishContext();
  }, [reloadPublishContext]);

  useEffect(() => {
    try {
      const q = new URLSearchParams(window.location.search);
      if (q.get('tab') === 'inventory') {
        setDraftTabKey('inventory');
        void reloadPublicationSkus();
      }
      if (q.get('tab') === 'readiness') {
        setDraftTabKey('readiness');
      }
    } catch {
      /* noop */
    }
  }, [id, reloadPublicationSkus]);

  const sortedImages = useMemo(() => {
    const typeRank = (t: string) => {
      if (t === 'main') return 0;
      if (t === 'sku') return 1;
      if (t === 'detail' || t === 'description') return 2;
      return 3;
    };
    const list = [...(data?.images ?? [])];
    list.sort((a, b) => {
      const tr = typeRank(String(a.imageType ?? '')) - typeRank(String(b.imageType ?? ''));
      if (tr !== 0) return tr;
      return (a.sortOrder ?? 0) - (b.sortOrder ?? 0);
    });
    return list;
  }, [data?.images]);

  const eligibleShopsForPublish = useMemo(() => {
    return shopsList.filter((s) => {
      const m = platformsMeta.find((x) => x.platform === s.platform);
      const st = m?.capabilityStatus?.product_publish;
      return st === 'available' || st === 'beta';
    });
  }, [shopsList, platformsMeta]);

  const douyinShops = useMemo(
    () => shopsList.filter((s) => (s.platform || '').toLowerCase() === 'douyin_shop' && s.authStatus === 'authorized'),
    [shopsList],
  );

  const reloadDouyinPublishTasks = useCallback(async () => {
    if (!id) return;
    setDouyinPublishTasksLoading(true);
    try {
      const res = await listDouyinPublishTasks(id, { page: 1, pageSize: 10 });
      setDouyinPublishTasks(res.list ?? []);
    } catch {
      setDouyinPublishTasks([]);
    } finally {
      setDouyinPublishTasksLoading(false);
    }
  }, [id]);

  const douyinPublication = useMemo(
    () =>
      pubRows.find(
        (p) =>
          (p.platform || '').toLowerCase() === 'douyin_shop' && String(p.externalProductId || '').trim() !== '',
      ) ?? null,
    [pubRows],
  );

  const reloadDouyinSkuBindings = useCallback(async () => {
    if (!douyinPublication?.id) {
      setDouyinSkuBinding(null);
      return;
    }
    setDouyinSkuBindingLoading(true);
    try {
      const res = await getDouyinSkuBindings(douyinPublication.id);
      setDouyinSkuBinding(res);
    } catch {
      setDouyinSkuBinding(null);
    } finally {
      setDouyinSkuBindingLoading(false);
    }
  }, [douyinPublication?.id]);

  const handleSyncDouyinSkuBindings = useCallback(async () => {
    if (!douyinPublication?.id) {
      message.warning('请先完成抖店商品草稿创建');
      return;
    }
    setDouyinSkuBindingSyncing(true);
    try {
      const res = await syncDouyinSkuBindings(douyinPublication.id);
      setDouyinSkuBinding(res);
      message.success(
        `规格绑定校准完成：已绑定 ${res.bound}，跳过 ${res.skipped}，未匹配 ${res.unmatched}，待确认 ${res.ambiguous}`,
      );
      await reloadPublicationSkus();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '校准规格绑定失败');
    } finally {
      setDouyinSkuBindingSyncing(false);
    }
  }, [douyinPublication?.id, reloadPublicationSkus]);

  const douyinCreateDraftDisabled = useMemo(() => {
    if (!douyinMapping || (douyinMapping.errors?.length ?? 0) > 0) return true;
    if (!douyinConfig.shopId || !douyinConfig.categoryId) return true;
    if (!douyinShops.some((s) => s.id === douyinConfig.shopId)) return true;
    const mainUploaded = (douyinMapping.mainImages ?? []).some((img) => img.uploadStatus === 'uploaded');
    if (!mainUploaded) return true;
    if (publishReadiness && !publishReadiness.canPublish) return true;
    return false;
  }, [douyinConfig.categoryId, douyinConfig.shopId, douyinMapping, douyinShops, publishReadiness]);

  const handleCreateDouyinDraft = useCallback(async () => {
    const shopId = String(douyinForm.getFieldValue('shopId') || douyinConfig.shopId || '').trim();
    if (!shopId) {
      message.error('请选择抖店店铺');
      return;
    }
    if (publishReadiness && (publishReadiness.warningCount ?? 0) > 0) {
      await new Promise<void>((resolve, reject) => {
        Modal.confirm({
          title: '当前商品存在需要人工确认的信息，是否继续创建抖店商品草稿？',
          width: 640,
          okText: '继续创建抖店商品草稿',
          cancelText: '返回处理',
          content: (
            <div>
              {(publishReadiness.checks || [])
                .filter((c) => c.level !== 'error')
                .slice(0, 10)
                .map((c, i) => (
                  <div key={`${c.code}-${i}`} style={{ marginBottom: 6 }}>
                    {readinessLevelTag(c.level)} {c.message}
                  </div>
                ))}
            </div>
          ),
          onOk: () => resolve(),
          onCancel: () => reject(new Error('cancelled')),
        });
      });
    }
    setDouyinDraftCreating(true);
    try {
      const task = await createDouyinProductDraft(id, { shopId, publishMode: 'save_as_platform_draft' });
      message.success('已创建抖店商品草稿，请到抖店后台确认后上架。');
      await reloadDouyinPublishTasks();
      await reloadPublicationSkus();
      await reloadDouyinSkuBindings();
      if (task.platformProductId) {
        message.info(`抖店商品 ID：${task.platformProductId}`);
      }
    } catch (e: unknown) {
      message.error((e as Error)?.message || '创建抖店商品草稿失败');
    } finally {
      setDouyinDraftCreating(false);
    }
  }, [douyinConfig.shopId, douyinForm, id, publishReadiness, reloadDouyinPublishTasks, reloadPublicationSkus, reloadDouyinSkuBindings]);

  const shopsForReadinessPlat = useMemo(() => {
    const p = readinessPlat.trim().toLowerCase();
    return shopsList.filter((s) => (s.platform || '').toLowerCase() === p && s.authStatus === 'authorized');
  }, [shopsList, readinessPlat]);

  const runReadinessForTab = useCallback(async () => {
    if (!id) return;
    setReadinessLoading(true);
    try {
      const r = await getProductReadiness(id, {
        platform: readinessPlat,
        shopId: readinessShopId || undefined,
        mode: 'draft',
      });
      setReadinessResult(r);
    } catch (e: unknown) {
      setReadinessResult(null);
      message.error((e as Error)?.message || '检查失败');
    } finally {
      setReadinessLoading(false);
    }
  }, [id, readinessPlat, readinessShopId]);

  const refreshPublishReadiness = useCallback(
    async (shopId: string) => {
      if (!shopId || !id) {
        setPublishReadiness(null);
        return;
      }
      const row = eligibleShopsForPublish.find((s) => s.id === shopId);
      if (!row) {
        setPublishReadiness(null);
        return;
      }
      setPublishReadinessLoading(true);
      try {
        const r = await getProductReadiness(id, {
          platform: row.platform,
          shopId,
          mode: 'publish',
        });
        setPublishReadiness(r);
      } catch {
        setPublishReadiness(null);
      } finally {
        setPublishReadinessLoading(false);
      }
    },
    [id, eligibleShopsForPublish],
  );

  useEffect(() => {
    if (draftTabKey !== 'publish' || !id) return;
    const sid = publishForm.getFieldValue('shopId') as string | undefined;
    if (sid) void refreshPublishReadiness(String(sid));
    void reloadDouyinPublishTasks();
    void reloadDouyinSkuBindings();
  }, [draftTabKey, id, publishForm, refreshPublishReadiness, reloadDouyinPublishTasks, reloadDouyinSkuBindings]);

  const imageColumns: ProColumns<ProductImageRow>[] = useMemo(
    () => [
      {
        title: '预览',
        width: 96,
        render: (_, r) => (
          <div style={{ padding: '4px 0' }}>
            <Image
              src={r.publicUrl || r.originUrl}
              width={56}
              height={56}
              style={{ objectFit: 'cover', borderRadius: 6, border: '1px solid var(--ant-color-border-secondary)' }}
            />
          </div>
        ),
      },
      {
        title: '类型',
        dataIndex: 'imageType',
        width: 132,
        render: (_, r) => <ProductImageTypeCell row={r} />,
      },
      {
        title: '评分',
        dataIndex: 'score',
        width: 72,
        render: (v) => (typeof v === 'number' ? v.toFixed(1) : '—'),
      },
      {
        title: PRODUCT_IMAGE_SORT_ORDER_LABEL,
        dataIndex: 'sortOrder',
        width: 92,
      },
      {
        title: PRODUCT_IMAGE_URL_LABEL,
        ellipsis: true,
        render: (_, r) => (
          <Typography.Link href={r.publicUrl || r.originUrl} target="_blank" rel="noreferrer">
            {(r.publicUrl || r.originUrl || '').slice(0, 64)}
            {(r.publicUrl || r.originUrl || '').length > 64 ? '…' : ''}
          </Typography.Link>
        ),
      },
      {
        title: '操作',
        width: 520,
        render: (_, r) => (
          <Space wrap size={[8, 4]} style={{ padding: '4px 0' }}>
            <Button type="link" size="small" onClick={() => openTranslateImageText(r)}>
              AI 翻译图片文字
            </Button>
            <Button type="link" size="small" onClick={() => openQuickImageTask(r, 'remove_watermark')}>
              AI 去水印
            </Button>
            <Button type="link" size="small" onClick={() => openQuickImageTask(r, 'remove_logo')}>
              AI 去 Logo
            </Button>
            <Button type="link" size="small" onClick={() => openQuickImageTask(r, 'remove_background')}>
              AI 去背景
            </Button>
            <Button type="link" size="small" onClick={() => openQuickImageTask(r, 'generate_marketing')}>
              AI 营销图
            </Button>
            <Button type="link" size="small" onClick={() => openQuickImageTask(r, 'score_image')}>
              AI 评分
            </Button>
            <Button
              type="link"
              size="small"
              onClick={() =>
                openCreateImageTask({
                  taskType: 'select_best_main',
                  imageSourceMode: 'product',
                  sourceImageId: r.id,
                  sourceImageUrl: (r.publicUrl || r.originUrl || '').trim(),
                })
              }
            >
              设为最佳主图
            </Button>
            <Button
              type="link"
              size="small"
              onClick={async () => {
                try {
                  await updateProductImage(id, r.id, { imageType: 'main', isBestMain: true, sortOrder: 0 });
                  message.success('已设为主图');
                  await reloadDetail();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '操作失败');
                }
              }}
            >
              设为主图
            </Button>
            <Button
              type="link"
              size="small"
              onClick={async () => {
                try {
                  await updateProductImage(id, r.id, { imageType: 'detail' });
                  message.success('已设为详情图');
                  await reloadDetail();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '操作失败');
                }
              }}
            >
              设为详情图
            </Button>
            <Button type="link" size="small" onClick={() => setImgEdit(r)}>
              编辑
            </Button>
            <Popconfirm
              title="删除该关联？"
              description="仅从商品移除关联"
              onConfirm={async () => {
                try {
                  await deleteProductImage(id, r.id);
                  message.success('已删除');
                  await reloadDetail();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '删除失败');
                }
              }}
            >
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [id, reloadDetail, openQuickImageTask, openCreateImageTask, openTranslateImageText],
  );

  const skuColumns = useMemo(
    () => [
      { title: '编码', dataIndex: 'skuCode', width: 140, ellipsis: true, formItemProps: { rules: [] } },
      { title: '名称', dataIndex: 'skuName', width: 180, ellipsis: true, formItemProps: { rules: [{ required: true }] } },
      {
        title: '成本价',
        dataIndex: 'costPrice',
        width: 100,
        valueType: 'digit' as const,
        fieldProps: { min: 0, precision: 2 },
        readonly: true,
      },
      {
        title: '销售价',
        dataIndex: 'price',
        width: 100,
        valueType: 'digit' as const,
        fieldProps: { min: 0, precision: 2 },
      },
      {
        title: '库存',
        dataIndex: 'stock',
        width: 92,
        valueType: 'digit' as const,
        fieldProps: { min: 0 },
      },
      {
        title: '图片 URL',
        dataIndex: 'imageUrl',
        width: 160,
        ellipsis: true,
      },
      {
        title: '规格属性（高级）',
        dataIndex: 'attrsText',
        valueType: 'textarea' as const,
        ellipsis: true,
        fieldProps: { rows: 2 },
      },
      {
        title: '操作',
        valueType: 'option' as const,
        width: 140,
        render: (_: unknown, record: SKUEditable) => (
          <Popconfirm
            title="删除该商品规格？"
            onConfirm={async () => {
              if (!record?.id?.startsWith('new_')) {
                try {
                  await deleteProductSku(id, record.id);
                  message.success('已删除');
                  await reloadDetail();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '删除失败');
                }
              } else {
                setSkuRows((rows) => rows.filter((r) => r.id !== record.id));
              }
            }}
          >
            <Button type="link" danger size="small">
              删除
            </Button>
          </Popconfirm>
        ),
      },
    ],
    [id, reloadDetail],
  );

  if (!id) {
    return (
      <TmPageContainer title="商品详情">
        <Typography.Text type="danger">无效的商品 ID</Typography.Text>
      </TmPageContainer>
    );
  }

  return (
    <TmPageContainer
      title={data?.title || '商品详情'}
      loading={loading}
      extra={
        data ? (
          <Space wrap>
            <Button
              onClick={async () => {
                try {
                  await updateProduct(id, { status: 'ready' });
                  message.success('已设为「可用」');
                  await reloadDetail();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '失败');
                }
              }}
            >
              标记为可用
            </Button>
            <Button
              onClick={async () => {
                try {
                  await updateProduct(id, { status: 'archived' });
                  message.success('已归档');
                  await reloadDetail();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '失败');
                }
              }}
            >
              归档
            </Button>
            <Popconfirm
              title="确定删除草稿？"
              description="软删除，列表不可见"
              onConfirm={async () => {
                try {
                  await deleteProduct(id);
                  message.success('已删除');
                  window.location.href = '/product/drafts';
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '删除失败');
                }
              }}
            >
              <Button danger icon={<DeleteOutlined />}>
                删除草稿
              </Button>
            </Popconfirm>
          </Space>
        ) : null
      }
    >
      {loading ? (
        <Spin />
      ) : err ? (
        <Typography.Text type="danger">{err}</Typography.Text>
      ) : data ? (
        <Tabs
          activeKey={draftTabKey}
          onChange={(k) => {
            setDraftTabKey(k);
            if (k === 'inventory') void reloadPublicationSkus();
          }}
          items={[
            {
              key: 'basic',
              label: '基础信息',
              children: (
                <Card variant="borderless">
                  {(isPinduoduoProduct(data) || isTaobaoTmallProduct(data)) ? (
                    <ProductCollectQualityAlert product={data} />
                  ) : null}
                  {showCustomIncompleteHint ? (
                    <Alert
                      type="info"
                      showIcon
                      style={{ marginBottom: 16 }}
                      message="该商品来自自定义链接采集，部分字段可能需要人工补充。建议检查标题、价格、图片和 SKU 后再发布。"
                    />
                  ) : null}
                  {collectQualityWarnings.length > 0 ? (
                    <Alert
                      type="warning"
                      showIcon
                      style={{ marginBottom: 16 }}
                      message="采集质量提示"
                      description={
                        <ul style={{ margin: 0, paddingLeft: 20 }}>
                          {collectQualityWarnings.map((w) => (
                            <li key={w}>{w}</li>
                          ))}
                        </ul>
                      }
                    />
                  ) : null}
                  <Descriptions column={2} bordered size="small" style={{ marginBottom: 16 }}>
                    <Descriptions.Item label="来源">{data.source}</Descriptions.Item>
                    <Descriptions.Item label="币种（展示）">{data.currency}</Descriptions.Item>
                    <Descriptions.Item label="来源链接" span={2}>
                      <Typography.Link href={data.sourceUrl || undefined} target="_blank" rel="noreferrer">
                        {data.sourceUrl || '—'}
                      </Typography.Link>
                    </Descriptions.Item>
                  </Descriptions>

                  {Object.keys(collectedAttrs).length > 0 ? (
                    <Card title="采集属性" size="small" style={{ marginBottom: 16 }}>
                      <Table
                        size="small"
                        pagination={false}
                        rowKey="key"
                        dataSource={Object.entries(collectedAttrs).map(([key, value]) => ({ key, value }))}
                        columns={[
                          { title: '属性', dataIndex: 'key', width: 180 },
                          { title: '值', dataIndex: 'value', ellipsis: true },
                        ]}
                      />
                    </Card>
                  ) : null}

                  <ProForm
                    key={`basic-${data.id}-${data.updatedAt}`}
                    submitter={{
                      searchConfig: { submitText: '保存基础信息' },
                      submitButtonProps: { type: 'primary' },
                      resetButtonProps: false,
                    }}
                    onFinish={async (vals: Record<string, unknown>) => {
                      try {
                        await updateProduct(id, {
                          title: String(vals.title ?? ''),
                          originalTitle: String(vals.originalTitle ?? ''),
                          aiTitle: String(vals.aiTitle ?? ''),
                          description: String(vals.description ?? ''),
                          aiDescription: String(vals.aiDescription ?? ''),
                          currency: String(vals.currency ?? ''),
                          status: String(vals.status ?? ''),
                        });
                        message.success('已保存');
                        await reloadDetail();
                        return true;
                      } catch (e: unknown) {
                        message.error((e as Error)?.message || '保存失败');
                        return false;
                      }
                    }}
                    layout="vertical"
                    grid
                    initialValues={{
                      title: data.title,
                      originalTitle: data.originalTitle,
                      aiTitle: data.aiTitle ?? '',
                      description: data.description ?? '',
                      aiDescription: data.aiDescription ?? '',
                      currency: data.currency || 'CNY',
                      status: data.status,
                    }}
                    colProps={{ span: 12 }}
                  >
                    <ProFormText name="title" label="主标题" rules={[{ required: true, message: '必填' }]} />
                    <ProFormTextArea name="originalTitle" label="原始标题" fieldProps={{ rows: 2 }} />
                    <ProFormTextArea name="aiTitle" label="AI 标题" fieldProps={{ rows: 2 }} />
                    <ProFormTextArea name="description" label="主描述" fieldProps={{ rows: 5 }} />
                    <ProFormTextArea name="aiDescription" label="AI 描述" fieldProps={{ rows: 5 }} />
                    <ProFormText name="currency" label="币种" />
                    <ProFormSelect name="status" label="状态" options={PRODUCT_STATUS_OPTIONS} />
                  </ProForm>
                </Card>
              ),
            },
            {
              key: 'ai',
              label: 'AI',
              children: (
                <Space direction="vertical" style={{ width: '100%' }} size="middle">
                  <Card variant="borderless" styles={{ body: { paddingBottom: 12 } }}>
                    <Space wrap size="middle">
                      <Button
                        type="primary"
                        onClick={() => {
                          setAiResult(null);
                          aiForm.resetFields();
                          aiForm.setFieldsValue({ language: 'en', platform: 'TikTok Shop', maxLength: 120 });
                          setAiOpen(true);
                        }}
                      >
                        标题优化
                      </Button>
                      <Button
                        type="primary"
                        onClick={() => {
                          setDescResult(null);
                          descForm.resetFields();
                          descForm.setFieldsValue({
                            language: 'en',
                            platform: 'TikTok Shop',
                            tone: 'professional',
                          });
                          setDescOpen(true);
                        }}
                      >
                        描述生成
                      </Button>
                    </Space>
                  </Card>

                  <Card title="最近任务">
                    <ProTable<AITaskRow>
                      rowKey="id"
                      search={false}
                      options={false}
                      pagination={false}
                      dataSource={aiTasks}
                      columns={[
                        { title: '类型', dataIndex: 'taskType', width: 200 },
                        { title: '状态', dataIndex: 'status', width: 100 },
                        { title: '模型', dataIndex: 'model', ellipsis: true },
                        {
                          title: 'Token 用量',
                          width: 100,
                          render: (_: unknown, row: AITaskRow) => `${row.tokenInput ?? 0}/${row.tokenOutput ?? 0}`,
                        },
                        { title: '技能模板', dataIndex: 'promptCode', width: 160, ellipsis: true },
                        {
                          title: '时间',
                          dataIndex: 'createdAt',
                          width: 176,
                          render: (v) => formatDateTime(v as string),
                        },
                      ]}
                      size="small"
                    />
                  </Card>

                  {data.rawData != null ? (
                    <TechnicalDetails label="采集原始信息">
                      <TaskJsonBlock title="原始信息" value={data.rawData} maxHeight={360} last />
                    </TechnicalDetails>
                  ) : null}
                </Space>
              ),
            },
            {
              key: 'images',
              label: '图片管理',
              children: (
                <Card variant="borderless">
                  {isPinduoduoProduct(data) ? (
                    <Alert
                      type="info"
                      showIcon
                      style={{ marginBottom: 16 }}
                      message="拼多多图片已按页面区域自动分类，请发布前检查主图和详情图是否正确。"
                    />
                  ) : null}
                  {isTaobaoTmallProduct(data) ? (
                    <Alert
                      type="info"
                      showIcon
                      style={{ marginBottom: 16 }}
                      message="淘宝/天猫采集图片默认为外链，发布前建议同步到平台存储，避免外链失效。"
                    />
                  ) : null}
                  <Card
                    size="small"
                    style={{
                      marginBottom: 16,
                      background: 'var(--ant-color-fill-alter)',
                      border: '1px solid var(--ant-color-border-secondary)',
                    }}
                    styles={{ body: { padding: '16px 20px' } }}
                  >
                    <Flex vertical gap={16}>
                      <Typography.Paragraph type="secondary" style={{ marginBottom: 0, fontSize: 13, lineHeight: '22px' }}>
                        在下方每张图片旁可一键发起 AI 处理；也可新建任务并选择图片。结果会自动保存到当前存储设置，可在{' '}
                        <Link to="/ai/image-tasks">AI 图片任务</Link> 查看进度。
                      </Typography.Paragraph>
                      <Flex wrap="wrap" gap={16} align="stretch">
                        <Flex
                          vertical
                          gap={10}
                          style={{
                            flex: '1 1 240px',
                            minWidth: 240,
                            paddingRight: 16,
                            borderRight: '1px solid var(--ant-color-border-secondary)',
                          }}
                        >
                          <Typography.Text type="secondary" style={{ fontSize: 12, fontWeight: 500 }}>
                            <PictureOutlined style={{ marginRight: 6 }} />
                            图片管理
                          </Typography.Text>
                          <Space wrap size={[8, 8]}>
                            <Button
                              type="primary"
                              icon={<PlusOutlined />}
                              onClick={() => {
                                setLastUpload(null);
                                setImgEdit(null);
                                setImgModalOpen(true);
                              }}
                            >
                              添加图片
                            </Button>
                            <Tooltip title="按当前列表顺序提交全部图片 ID">
                              <Button
                                icon={<SyncOutlined />}
                                onClick={async () => {
                                  try {
                                    const ordered = [...sortedImages].sort(
                                      (a, b) => (a.sortOrder ?? 0) - (b.sortOrder ?? 0),
                                    );
                                    await reorderProductImages(id, { imageIds: ordered.map((i) => i.id) });
                                    message.success('已同步');
                                    await reloadDetail();
                                  } catch (e: unknown) {
                                    message.error((e as Error)?.message || '排序失败');
                                  }
                                }}
                              >
                                同步顺序
                              </Button>
                            </Tooltip>
                            {isTaobaoTmallProduct(data) ? (
                              <>
                                <Button
                                  onClick={async () => {
                                    try {
                                      const res = await syncProductImages(id, { scope: 'all' });
                                      message.success(`已同步 ${res.synced} 张图片到平台存储`);
                                      await reloadDetail();
                                    } catch (e: unknown) {
                                      message.error((e as Error)?.message || '同步失败');
                                    }
                                  }}
                                >
                                  同步图片到平台存储
                                </Button>
                                <Button
                                  onClick={async () => {
                                    try {
                                      const res = await syncProductImages(id, { scope: 'main' });
                                      message.success(`已同步 ${res.synced} 张主图`);
                                      await reloadDetail();
                                    } catch (e: unknown) {
                                      message.error((e as Error)?.message || '同步失败');
                                    }
                                  }}
                                >
                                  批量同步主图
                                </Button>
                                <Button
                                  onClick={async () => {
                                    try {
                                      const res = await syncProductImages(id, { scope: 'detail' });
                                      message.success(`已同步 ${res.synced} 张详情图`);
                                      await reloadDetail();
                                    } catch (e: unknown) {
                                      message.error((e as Error)?.message || '同步失败');
                                    }
                                  }}
                                >
                                  批量同步详情图
                                </Button>
                              </>
                            ) : null}
                          </Space>
                        </Flex>
                        <Flex vertical gap={10} style={{ flex: '2 1 320px', minWidth: 280 }}>
                          <Typography.Text type="secondary" style={{ fontSize: 12, fontWeight: 500 }}>
                            <RobotOutlined style={{ marginRight: 6 }} />
                            AI 处理
                          </Typography.Text>
                          <Space wrap size={[8, 8]}>
                            <Button type="primary" icon={<RobotOutlined />} onClick={() => openCreateImageTask({})}>
                              新建图片任务
                            </Button>
                            <Link to="/ai/image-tasks">
                              <Button icon={<UnorderedListOutlined />}>查看任务列表</Button>
                            </Link>
                            <Button icon={<StarOutlined />} onClick={() => void runSelectBestMain('recommend')}>
                              设为最佳主图
                            </Button>
                            <Button
                              type="primary"
                              ghost
                              icon={<ThunderboltOutlined />}
                              onClick={() => void runSelectBestMain('auto_set')}
                            >
                              自动设为主图
                            </Button>
                          </Space>
                        </Flex>
                      </Flex>
                    </Flex>
                  </Card>
                  <ProTable<ProductImageRow>
                    rowKey="id"
                    search={false}
                    options={false}
                    pagination={false}
                    headerTitle="图片列表"
                    toolBarRender={false}
                    dataSource={sortedImages}
                    columns={imageColumns}
                    size="small"
                  />
                </Card>
              ),
            },
            {
              key: 'skus',
              label: '商品规格',
              children: (
                <Card variant="borderless">
                  {(data.source === 'custom' || isPinduoduoProduct(data)) &&
                  (data.skus ?? []).filter((s) => !String(s.id).startsWith('new_')).length === 0 ? (
                    <Alert
                      type="info"
                      showIcon
                      style={{ marginBottom: 12 }}
                      message={
                        isPinduoduoProduct(data)
                          ? '当前采集结果没有完整商品规格。你可以手动新增 SKU，或等待后续版本增强拼多多规格采集。'
                          : '当前采集结果没有商品规格。部分网站的规格和库存需要专用采集器才能完整获取，你也可以手动新增 SKU。'
                      }
                    />
                  ) : null}
                  <Space style={{ marginBottom: 12 }}>
                    <Button type="primary" onClick={() => setPricingOpen(true)}>
                      应用定价规则
                    </Button>
                    <Typography.Text type="secondary">
                      按成本价/当前价加价并更新本地销售价，不会自动刊登
                    </Typography.Text>
                  </Space>
                  <EditableProTable<SKUEditable>
                    rowKey="id"
                    headerTitle={false}
                    search={false}
                    options={false}
                    pagination={false}
                    value={skuRows}
                    onChange={(value) => setSkuRows([...value])}
                    recordCreatorProps={{
                      record: (): SKUEditable => ({
                        id: `new_${Date.now()}`,
                        productId: id,
                        skuCode: '',
                        skuName: '新 SKU',
                        attrsText: '{}',
                      }),
                      style: {
                        marginBottom: 16,
                      },
                      creatorButtonText: '新增 SKU',
                    }}
                    editable={{
                      type: 'multiple',
                      onSave: async (_key, row) => {
                        const attrsStr = row.attrsText?.trim() ?? '';
                        let attrs: string | Record<string, unknown> | undefined = attrsStr;
                        if (!attrsStr) attrs = '{}';
                        if (String(row.id).startsWith('new_')) {
                          await createProductSku(id, {
                            skuCode: row.skuCode ?? '',
                            skuName: row.skuName,
                            attrs,
                            price: row.price,
                            stock: row.stock,
                            imageUrl: row.imageUrl,
                          });
                          message.success('商品规格已创建');
                        } else {
                          await updateProductSku(id, row.id, {
                            skuCode: row.skuCode,
                            skuName: row.skuName,
                            attrs,
                            price: row.price,
                            stock: row.stock,
                            imageUrl: row.imageUrl,
                          });
                          message.success('商品规格已更新');
                        }
                        await reloadDetail();
                      },
                    }}
                    columns={skuColumns}
                    scroll={{ x: 1100 }}
                  />
                </Card>
              ),
            },
            {
              key: 'inventory',
              label: '库存',
              children: (
                <Card variant="borderless">
                  <Alert
                    type="info"
                    showIcon
                    style={{ marginBottom: 24 }}
                    message="库存说明"
                    description={
                      <>
                        <Typography.Paragraph style={{ marginBottom: 8 }}>
                          在此调整本地 SKU 库存与预警线；已刊登到各平台的 SKU 可在下方同步到店铺。
                          仅当平台已开放「库存同步」且映射完整时可发起同步（抖店、TikTok、Shopee、Lazada、Amazon 已支持）。
                        </Typography.Paragraph>
                        <Typography.Paragraph style={{ marginBottom: 0 }}>
                          相关入口：
                          <Link to="/inventory/alerts">库存预警</Link>
                          {' · '}
                          <Link to="/inventory/sync-tasks">同步任务</Link>
                          {' · '}
                          <Link to={`/inventory/logs?productId=${data.id}`}>变更记录</Link>
                          {' · '}
                          <Link to="/inventory/effects">订单扣减</Link>
                        </Typography.Paragraph>
                      </>
                    }
                  />

                  <Space align="center" style={{ marginBottom: 12 }} wrap>
                    <Typography.Title level={5} style={{ margin: 0 }}>
                      本地规格
                    </Typography.Title>
                    <Button
                      size="small"
                      onClick={() => {
                        setSkuBatchScope(skuBatchSelKeys.length ? 'selected' : 'all');
                        skuBatchStockForm.setFieldsValue({ warningStock: 10, safetyStock: 2 });
                        setSkuBatchStockOpen(true);
                      }}
                    >
                      批量设置预警线
                    </Button>
                  </Space>
                  <Table<ProductSKURow>
                    loading={loading}
                    size="small"
                    pagination={false}
                    rowKey="id"
                    dataSource={(data.skus ?? []).filter((s) => !String(s.id).startsWith('new_'))}
                    rowSelection={{
                      selectedRowKeys: skuBatchSelKeys,
                      onChange: (keys) => setSkuBatchSelKeys(keys.map(String)),
                    }}
                    columns={[
                      { title: '编码', dataIndex: 'skuCode', width: 120, ellipsis: true },
                      { title: '名称', dataIndex: 'skuName', ellipsis: true },
                      {
                        title: '库存',
                        dataIndex: 'stock',
                        width: 72,
                        render: (_v, r) => (typeof r.stock === 'number' ? r.stock : '—'),
                      },
                      {
                        title: '预警',
                        dataIndex: 'warningStock',
                        width: 64,
                        render: (_v, r) => (typeof r.warningStock === 'number' ? r.warningStock : '—'),
                      },
                      {
                        title: '安全',
                        dataIndex: 'safetyStock',
                        width: 64,
                        render: (_v, r) => (typeof r.safetyStock === 'number' ? r.safetyStock : '—'),
                      },
                      {
                        title: '状态',
                        dataIndex: 'stockStatus',
                        width: 108,
                        render: (_v, r) => draftStockStatusTag(effectiveStockStatus(r)),
                      },
                      {
                        title: '操作',
                        key: 'op',
                        width: 280,
                        render: (_x, r) => (
                          <Space wrap size="small">
                            <Button
                              type="link"
                              size="small"
                              style={{ padding: 0 }}
                              onClick={() => {
                                setAdjustTarget(r);
                                adjustForm.setFieldsValue({
                                  stock: typeof r.stock === 'number' ? r.stock : 0,
                                  reason: 'manual_adjust',
                                  remark: '',
                                });
                                setAdjustOpen(true);
                              }}
                            >
                              调整库存
                            </Button>
                            <Button
                              type="link"
                              size="small"
                              style={{ padding: 0 }}
                              onClick={() => {
                                setStockSettingsTarget(r);
                                stockSettingsForm.setFieldsValue({
                                  warningStock: typeof r.warningStock === 'number' ? r.warningStock : 5,
                                  safetyStock: typeof r.safetyStock === 'number' ? r.safetyStock : 0,
                                });
                                setStockSettingsOpen(true);
                              }}
                            >
                              预警线
                            </Button>
                            <Button
                              type="link"
                              size="small"
                              style={{ padding: 0 }}
                              onClick={async () => {
                                setLogsSku(r);
                                setLogsOpen(true);
                                setLogsLoading(true);
                                try {
                                  const res = await querySkuInventoryLogs(id, r.id, { page: 1, pageSize: 50 });
                                  setLogsRows(res.list ?? []);
                                } catch {
                                  setLogsRows([]);
                                } finally {
                                  setLogsLoading(false);
                                }
                              }}
                            >
                              变更记录
                            </Button>
                          </Space>
                        ),
                      },
                    ]}
                  />

                  <Modal
                    title="批量设置预警线（本商品）"
                    open={skuBatchStockOpen}
                    onCancel={() => {
                      setSkuBatchStockOpen(false);
                      setSkuBatchMatched(null);
                    }}
                    okText="应用"
                    onOk={() => {
                      return skuBatchStockForm
                        .validateFields()
                        .then((v) => {
                          if (v.safetyStock > v.warningStock) {
                            message.error('安全线不能大于预警线');
                            return Promise.reject(new Error('validation'));
                          }
                          if (skuBatchScope === 'selected' && skuBatchSelKeys.length === 0) {
                            message.error('请勾选 SKU，或改用「本商品全部 SKU」');
                            return Promise.reject(new Error('validation'));
                          }
                          return new Promise<void>((resolve, reject) => {
                            Modal.confirm({
                              title: '确认仅修改预警线？',
                              content:
                                '不修改实际库存，不同步平台，不写入库存流水。将影响的 SKU 数：' +
                                String(skuBatchMatched ?? '—'),
                              okText: '确认',
                              onOk: async () => {
                                try {
                                  await batchUpdateStockSettings({
                                    ...buildSkuStockPayload(),
                                    warningStock: v.warningStock,
                                    safetyStock: v.safetyStock,
                                    confirm: true,
                                    confirmLarge: (skuBatchMatched ?? 0) > SKU_BATCH_STOCK_MAX_HINT,
                                  });
                                  message.success('已批量更新预警线');
                                  setSkuBatchStockOpen(false);
                                  setSkuBatchMatched(null);
                                  setSkuBatchSelKeys([]);
                                  await reloadDetail();
                                  resolve();
                                } catch (e) {
                                  message.error((e as Error)?.message || '失败');
                                  reject(e);
                                }
                              },
                            });
                          });
                        })
                        .catch((e: unknown) => {
                          if ((e as Error)?.message === 'validation') return;
                          throw e;
                        });
                    }}
                  >
                    <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
                      匹配数：{skuBatchPreviewLoading ? '计算中…' : skuBatchMatched !== null ? `${skuBatchMatched} 个 SKU` : '—'}
                    </Typography.Paragraph>
                    <Form form={skuBatchStockForm} layout="vertical" initialValues={{ warningStock: 10, safetyStock: 2 }}>
                      <Form.Item label="应用范围">
                        <Radio.Group
                          value={skuBatchScope}
                          onChange={(e) => setSkuBatchScope(e.target.value as 'selected' | 'all')}
                        >
                          <Radio value="all">本商品全部 SKU</Radio>
                          <Radio value="selected" disabled={skuBatchSelKeys.length === 0}>
                            仅选中（{skuBatchSelKeys.length}）
                          </Radio>
                        </Radio.Group>
                      </Form.Item>
                      <Form.Item name="warningStock" label="预警库存线" rules={[{ required: true }]}>
                        <InputNumber min={0} style={{ width: '100%' }} />
                      </Form.Item>
                      <Form.Item name="safetyStock" label="安全库存线" rules={[{ required: true }]}>
                        <InputNumber min={0} style={{ width: '100%' }} />
                      </Form.Item>
                      <Button type="link" size="small" onClick={() => void runSkuBatchPreview()} loading={skuBatchPreviewLoading}>
                        刷新匹配数
                      </Button>
                    </Form>
                  </Modal>

                  <Typography.Title level={5} style={{ marginTop: 24 }}>
                    已刊登 SKU 映射
                  </Typography.Title>
                  <Space wrap style={{ marginBottom: 12 }}>
                    <Select
                      allowClear
                      placeholder="按平台筛选（批量同步）"
                      style={{ minWidth: 200 }}
                      value={pubSkuBulkPlatformFilter || undefined}
                      onChange={(v) => setPubSkuBulkPlatformFilter((v as string | undefined) ?? '')}
                      options={[
                        { label: '抖店', value: 'douyin_shop' },
                        { label: 'TikTok', value: 'tiktok' },
                        { label: 'Shopee', value: 'shopee' },
                        { label: 'Lazada', value: 'lazada' },
                        { label: 'Amazon', value: 'amazon' },
                      ]}
                    />
                    <Button
                      type="primary"
                      disabled={pubSkuSelectedKeys.length === 0}
                      onClick={() => {
                        Modal.confirm({
                          title: '批量同步选中刊登 SKU？',
                          content: `将为选中的 ${pubSkuSelectedKeys.length} 条映射创建库存同步任务。缺少平台映射或未开放库存同步能力的条目将自动跳过。`,
                          okText: '创建批次',
                          onOk: async () => {
                            try {
                              const batch = await createInventorySyncBatch({
                                source: 'product_detail',
                                productId: id,
                                publicationSkuIds: pubSkuSelectedKeys,
                                onlyPublished: true,
                              });
                              message.success(
                                `批次 ${batch.batchNo} 已创建；新建任务 ${batch.totalCount - batch.skippedCount}，跳过 ${batch.skippedCount}`,
                              );
                              setPubSkuSelectedKeys([]);
                              await reloadPublicationSkus();
                              window.location.href = `/inventory/sync-tasks?batchId=${encodeURIComponent(batch.id)}`;
                            } catch (e: unknown) {
                              message.error(formatInventorySyncTaskCreateError(e));
                              throw e;
                            }
                          },
                        });
                      }}
                    >
                      批量同步到平台
                    </Button>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      勾选左侧可选行；不可选项表示缺少平台映射或未开放库存同步。
                    </Typography.Text>
                  </Space>
                  <Spin spinning={pubSkuLoading}>
                    <Table<PublicationSkuListingRow>
                      size="small"
                      rowKey="publicationSkuId"
                      pagination={false}
                      dataSource={filteredPubSkuRowsForBulk}
                      rowSelection={{
                        selectedRowKeys: pubSkuSelectedKeys,
                        onChange: (keys) => setPubSkuSelectedKeys(keys.map(String)),
                        getCheckboxProps: (r) => {
                          const missing =
                            !String(r.externalSkuId ?? '').trim() || !String(r.externalProductId ?? '').trim();
                          const ok = inventorySyncRunnable(r.inventorySyncCapability);
                          return { disabled: missing || douyinSkuSyncBlocked(r) || !ok };
                        },
                      }}
                      columns={[
                        {
                          title: '店铺',
                          ellipsis: true,
                          render: (_, r) => r.shopName || '—',
                        },
                        { title: '平台', dataIndex: 'platform', width: 108, render: (v: string) => platformDisplayName(v) },
                        {
                          title: '本地商品规格',
                          ellipsis: true,
                          render: (_, r) => r.skuCode || r.productSkuId || '—',
                        },
                        {
                          title: '外部商品 ID',
                          dataIndex: 'externalProductId',
                          ellipsis: true,
                          render: (t: string | undefined) => t || '—',
                        },
                        {
                          title: '平台规格编码',
                          dataIndex: 'externalSkuId',
                          ellipsis: true,
                          render: (t: string | undefined) => t || '—',
                        },
                        {
                          title: '规格绑定',
                          width: 108,
                          render: (_x, r) =>
                            (r.platform || '').toLowerCase() === 'douyin_shop'
                              ? douyinBindStatusTag(r.bindStatus || (r.externalSkuId ? 'bound' : 'unmatched'))
                              : '—',
                        },
                        {
                          title: '平台库存快照',
                          width: 168,
                          render: (_x, r) => {
                            const sku = data.skus?.find((s) => s.id === r.productSkuId);
                            const local = typeof sku?.stock === 'number' ? sku.stock : null;
                            const plat = r.platformStock;
                            const nodes: JSX.Element[] = [];
                            if (typeof plat === 'number') {
                              nodes.push(<span key="n">{plat}</span>);
                            } else {
                              nodes.push(<span key="n">—</span>);
                            }
                            if (plat === null || plat === undefined) {
                              nodes.push(
                                <Tag key="u" style={{ marginLeft: 6 }}>
                                  未知
                                </Tag>,
                              );
                            } else if (local !== null && plat !== local) {
                              nodes.push(
                                <Tag key="m" color="orange" style={{ marginLeft: 6 }}>
                                  与本地不一致
                                </Tag>,
                              );
                            }
                            return <span>{nodes}</span>;
                          },
                        },
                        {
                          title: '库存同步',
                          width: 110,
                          render: (_x, r) => inventorySyncCapabilityTag(r.inventorySyncCapability),
                        },
                        {
                          title: '操作',
                          width: 132,
                          render: (_x, r) => {
                            const ok = inventorySyncRunnable(r.inventorySyncCapability);
                            const isDouyin = (r.platform || '').toLowerCase() === 'douyin_shop';
                            const blocked = douyinSkuSyncBlocked(r);
                            const hasBinding =
                              Boolean((r.externalProductId || '').trim()) &&
                              Boolean((r.externalSkuId || '').trim());
                            const canSync = ok && hasBinding && !blocked;
                            const sku = data.skus?.find((s) => s.id === r.productSkuId);
                            const fallback = typeof sku?.stock === 'number' ? sku.stock : 0;
                            const suggested =
                              typeof r.platformStock === 'number' ? r.platformStock : fallback;
                            const st = String(r.bindStatus || '').toLowerCase();
                            const disableReason = isDouyin && st === 'ambiguous'
                              ? '找到多个可能的抖店 SKU，请人工确认绑定后再同步库存。'
                              : isDouyin && (st === 'unmatched' || st === 'failed' || !hasBinding)
                                ? '该规格还没有绑定抖店 SKU，请先完成绑定后再同步库存。'
                                : '当前平台未开放库存同步、店铺未授权，或该映射行不可用';
                            const btn = (
                              <Button
                                type="link"
                                size="small"
                                disabled={!canSync}
                                style={{ padding: 0 }}
                                onClick={() => {
                                  if (!canSync) return;
                                  setSyncRow(r);
                                  syncForm.setFieldsValue({ stock: suggested });
                                  setSyncOpen(true);
                                }}
                              >
                                同步库存
                              </Button>
                            );
                            return canSync ? btn : (
                              <Tooltip title={disableReason}>
                                <span>{btn}</span>
                              </Tooltip>
                            );
                          },
                        },
                      ]}
                    />
                  </Spin>
                </Card>
              ),
            },
            {
              key: 'readiness',
              label: '发布检查',
              children: (
                <Card variant="borderless">
                  <Space direction="vertical" style={{ width: '100%' }} size="large">
                    <Space wrap align="center">
                      <Typography.Text strong>目标平台</Typography.Text>
                      <Select
                        style={{ minWidth: 160 }}
                        value={readinessPlat}
                        onChange={(v) => setReadinessPlat(v)}
                        options={['douyin_shop', 'tiktok', 'shopee', 'lazada', 'amazon', 'mock'].map((p) => ({
                          label: p,
                          value: p,
                        }))}
                      />
                      <Typography.Text strong>店铺</Typography.Text>
                      <Select
                        style={{ minWidth: 240 }}
                        placeholder="选择已授权店铺"
                        allowClear
                        showSearch
                        optionFilterProp="label"
                        value={readinessShopId || undefined}
                        onChange={(v) => setReadinessShopId(v ? String(v) : '')}
                        options={shopsForReadinessPlat.map((s) => ({
                          label: `${s.shopName} (${s.platform})`,
                          value: s.id,
                        }))}
                      />
                      <Button type="primary" loading={readinessLoading} onClick={() => void runReadinessForTab()}>
                        重新检查
                      </Button>
                    </Space>
                    {readinessResult ? (
                      <>
                        <Descriptions bordered size="small" column={2}>
                          <Descriptions.Item label="总状态">{readinessStatusTag(readinessResult)}</Descriptions.Item>
                          <Descriptions.Item label="分数">{readinessResult.score}</Descriptions.Item>
                          <Descriptions.Item label="错误数">{readinessResult.errorCount}</Descriptions.Item>
                          <Descriptions.Item label="警告数">{readinessResult.warningCount}</Descriptions.Item>
                        </Descriptions>
                        <Collapse
                          defaultActiveKey={['product', 'sku', 'image', 'inventory', 'collect', 'platform']}
                          items={['product', 'sku', 'image', 'inventory', 'collect', 'platform'].map((g) => ({
                            key: g,
                            label: READINESS_GROUP_LABEL[g] || g,
                            children: (
                              <Table
                                size="small"
                                pagination={false}
                                rowKey={(_, i) => `${g}-${i}`}
                                dataSource={readinessResult.checks.filter((c) => c.group === g)}
                                columns={[
                                  {
                                    title: '级别',
                                    width: 88,
                                    render: (_: unknown, row: ReadinessCheckItem) => readinessLevelTag(row.level),
                                  },
                                  { title: '说明', dataIndex: 'message', ellipsis: true },
                                  {
                                    title: '建议 / 操作',
                                    width: 220,
                                    render: (_: unknown, row: ReadinessCheckItem) => {
                                      const fx = fixLinkForReadinessCode(row.code);
                                      return (
                                        <Space direction="vertical" size={4}>
                                          <Typography.Text type="secondary">{row.suggestion}</Typography.Text>
                                          {fx ? (
                                            fx.tab ? (
                                              <Button
                                                type="link"
                                                size="small"
                                                style={{ padding: 0 }}
                                                onClick={() => setDraftTabKey(fx.tab!)}
                                              >
                                                {fx.label}
                                              </Button>
                                            ) : (
                                              <Link to={fx.href!}>{fx.label}</Link>
                                            )
                                          ) : null}
                                        </Space>
                                      );
                                    },
                                  },
                                ]}
                              />
                            ),
                          }))}
                        />
                        <TechnicalDetails label="检查项技术详情">
                          <TaskJsonBlock title="完整检查结果" value={readinessResult.checks} last />
                        </TechnicalDetails>
                      </>
                    ) : (
                      <Typography.Text type="secondary">选择平台与店铺后点击「重新检查」。未选店铺时仅校验商品 / SKU / 图片（不校验店铺与平台配置）。</Typography.Text>
                    )}
                  </Space>
                </Card>
              ),
            },
            {
              key: 'publish',
              label: '刊登',
              children: (
                <Spin spinning={pubCtxLoading || publishReadinessLoading}>
                  <Card variant="borderless">
                    <Space direction="vertical" style={{ width: '100%' }} size="middle">
                      <Alert
                        type="info"
                        showIcon
                        message="多平台刊登"
                        description={
                          <>
                            可为已授权且支持刊登的店铺创建刊登任务。提交前请先在{' '}
                            <Link to="/settings/platform-publish">平台刊登预设</Link>{' '}
                            补齐类目、品牌、包裹尺寸等信息；进度可在{' '}
                            <Link to="/product/publish-tasks">刊登任务</Link> 查看。
                            <TechnicalDetails label="预设项说明">
                              <Typography.Paragraph type="secondary" style={{ marginBottom: 0, fontSize: 12 }}>
                                各平台需配置对应刊登模板（如 TikTok、Shopee、Lazada、Amazon 的类目与物流选项）。内部预设键名：
                                product_publish、platform_publish_tiktok、platform_publish_shopee、
                                platform_publish_lazada、platform_publish_amazon。
                              </Typography.Paragraph>
                            </TechnicalDetails>
                          </>
                        }
                      />
                      <Descriptions bordered size="small" column={3}>
                        <Descriptions.Item label="当前发布状态">
                          <Tag color={data.publishStatus === 'success' ? 'green' : data.publishStatus === 'ready' ? 'blue' : 'default'}>
                            {data.publishStatus || 'draft'}
                          </Tag>
                        </Descriptions.Item>
                        <Descriptions.Item label="定价结果">
                          {typeof data.salePrice === 'number' ? `${data.salePrice.toFixed(2)} ${data.currency || ''}` : '未设置售价'}
                        </Descriptions.Item>
                        <Descriptions.Item label="SKU 数">{data.skus?.length ?? 0}</Descriptions.Item>
                        <Descriptions.Item label="图片同步">
                          已同步 {imageSyncSummary.synced} / {imageSyncSummary.total}，外链 {imageSyncSummary.external}
                        </Descriptions.Item>
                        <Descriptions.Item label="主图外链">{imageSyncSummary.externalMain}</Descriptions.Item>
                        <Descriptions.Item label="详情图外链">{imageSyncSummary.externalDetail}</Descriptions.Item>
                      </Descriptions>
                      {collectQualityWarnings.length > 0 ? (
                        <Alert
                          type="warning"
                          showIcon
                          message="采集 warning 发布前需确认"
                          description={collectQualityWarnings.slice(0, 6).join('；')}
                        />
                      ) : null}
                      <Space wrap>
                        <Button
                          onClick={async () => {
                            try {
                              const res = await syncProductImages(id, { scope: 'main' });
                              message.success(`已同步 ${res.synced} 张主图`);
                              await reloadDetail();
                            } catch (e: unknown) {
                              message.error((e as Error)?.message || '同步失败');
                            }
                          }}
                        >
                          同步主图到平台存储
                        </Button>
                        <Button
                          onClick={async () => {
                            try {
                              const res = await syncProductImages(id, { scope: 'detail' });
                              message.success(`已同步 ${res.synced} 张详情图`);
                              await reloadDetail();
                            } catch (e: unknown) {
                              message.error((e as Error)?.message || '同步失败');
                            }
                          }}
                        >
                          同步详情图到平台存储
                        </Button>
                        <Button
                          type="primary"
                          ghost
                          onClick={async () => {
                            try {
                              const res = await syncProductImages(id, { scope: 'all' });
                              message.success(`已同步 ${res.synced} 张图片到平台存储`);
                              await reloadDetail();
                            } catch (e: unknown) {
                              message.error((e as Error)?.message || '同步失败');
                            }
                          }}
                        >
                          一键同步全部图片
                        </Button>
                        <Button onClick={() => setPricingOpen(true)}>应用定价规则</Button>
                      </Space>
                      <Table
                        size="small"
                        rowKey="id"
                        pagination={false}
                        dataSource={skuMappingPreview}
                        columns={[
                          { title: '规格', dataIndex: 'skuName', ellipsis: true },
                          { title: '编码', dataIndex: 'skuCode', width: 160, ellipsis: true },
                          { title: '售价', dataIndex: 'price', width: 100, render: (v) => (v != null ? Number(v).toFixed(2) : '—') },
                          { title: '库存', dataIndex: 'stock', width: 80, render: (v) => (v != null ? v : '—') },
                        ]}
                      />
                      <Card size="small" title="抖店类目与属性" variant="borderless">
                        <Space direction="vertical" style={{ width: '100%' }} size="middle">
                          {douyinCategoryFlat.length === 0 ? (
                            <Alert
                              type="warning"
                              showIcon
                              message="暂无抖店类目数据，请先点击「刷新类目」。"
                            />
                          ) : null}
                          <Form
                            form={douyinForm}
                            layout="vertical"
                            onValuesChange={(changed, all) => {
                              if (Object.prototype.hasOwnProperty.call(changed, 'categoryId')) {
                                const cat = douyinCategoryFlat.find((x) => x.categoryId === all.categoryId);
                                setDouyinConfig((cur) => ({
                                  ...cur,
                                  categoryId: all.categoryId,
                                  categoryPath: cat?.path,
                                  platformAttributes: {},
                                }));
                                douyinForm.setFieldValue('platformAttributes', {});
                                void reloadDouyinAttrs(all.categoryId, all.shopId, false);
                              } else {
                                setDouyinConfig((cur) => ({
                                  ...cur,
                                  shopId: all.shopId,
                                  categoryId: all.categoryId,
                                  categoryPath: selectedDouyinCategory?.path || cur.categoryPath,
                                  platformAttributes: all.platformAttributes ?? cur.platformAttributes ?? {},
                                }));
                              }
                            }}
                            onFinish={async (vals) => {
                              const cat = douyinCategoryFlat.find((x) => x.categoryId === vals.categoryId);
                              if (vals.categoryId && !cat?.isLeaf) {
                                message.error('只能选择抖店叶子类目');
                                return;
                              }
                              setDouyinSaving(true);
                              try {
                                const saved = await putProductPlatformPublishConfig(id, 'douyin_shop', {
                                  shopId: vals.shopId,
                                  categoryId: vals.categoryId,
                                  categoryPath: cat?.path || douyinConfig.categoryPath,
                                  platformAttributes: vals.platformAttributes ?? {},
                                });
                                setDouyinConfig({
                                  shopId: saved.shopId,
                                  categoryId: saved.categoryId,
                                  categoryPath: saved.categoryPath,
                                  platformAttributes: saved.platformAttributes ?? {},
                                });
                                message.success('抖店刊登配置已保存');
                                if (readinessPlat === 'douyin_shop') {
                                  void runReadinessForTab();
                                }
                              } catch (e: unknown) {
                                message.error((e as Error)?.message || '保存失败');
                              } finally {
                                setDouyinSaving(false);
                              }
                            }}
                          >
                            <Form.Item name="shopId" label="抖店店铺" rules={[{ required: true, message: '请选择抖店店铺' }]}>
                              <Select
                                placeholder="选择已授权抖店店铺"
                                allowClear
                                showSearch
                                optionFilterProp="label"
                                options={douyinShops.map((s) => ({ label: s.shopName, value: s.id }))}
                              />
                            </Form.Item>
                            <Space wrap style={{ marginBottom: 12 }}>
                              <Button
                                icon={<SyncOutlined />}
                                loading={douyinCategoryLoading}
                                onClick={() => void reloadDouyinCategories(douyinForm.getFieldValue('shopId'), true)}
                              >
                                刷新类目
                              </Button>
                              <Button
                                loading={douyinAttrLoading}
                                disabled={!douyinForm.getFieldValue('categoryId')}
                                onClick={() =>
                                  void reloadDouyinAttrs(
                                    douyinForm.getFieldValue('categoryId'),
                                    douyinForm.getFieldValue('shopId'),
                                    true,
                                  )
                                }
                              >
                                刷新属性
                              </Button>
                            </Space>
                            <Form.Item
                              name="categoryId"
                              label="抖店类目"
                              rules={[{ required: true, message: '请先选择抖店商品类目' }]}
                              extra={selectedDouyinCategory?.path}
                            >
                              <Select
                                placeholder="搜索并选择叶子类目"
                                loading={douyinCategoryLoading}
                                showSearch
                                allowClear
                                optionFilterProp="label"
                                options={douyinCategoryFlat
                                  .filter((c) => c.isLeaf)
                                  .map((c) => ({
                                    label: `${c.path || c.name} (${c.categoryId})`,
                                    value: c.categoryId,
                                  }))}
                              />
                            </Form.Item>
                            {douyinForm.getFieldValue('categoryId') && douyinAttrs.length === 0 ? (
                              <Alert type="info" showIcon message="该类目暂无本地属性缓存，请点击「刷新属性」。" />
                            ) : null}
                            {douyinAttrs.length > 0 ? (
                              <Spin spinning={douyinAttrLoading}>
                                <Descriptions bordered size="small" column={1}>
                                  <Descriptions.Item label="必填属性">
                                    {douyinAttrs.filter((a) => a.required).length || 0} 项
                                  </Descriptions.Item>
                                  <Descriptions.Item label="可选属性">
                                    {douyinAttrs.filter((a) => !a.required).length || 0} 项
                                  </Descriptions.Item>
                                </Descriptions>
                                <Row gutter={16} style={{ marginTop: 12 }}>
                                  {douyinAttrs.map((attr) => {
                                    const opts = Array.isArray(attr.options) ? attr.options : [];
                                    return (
                                      <Col xs={24} md={12} key={attr.attrId}>
                                        <Form.Item
                                          name={['platformAttributes', attr.attrId]}
                                          label={
                                            <Space size={4}>
                                              <span>{attr.name || attr.attrId}</span>
                                              {attr.required ? <Tag color="red">必填</Tag> : <Tag>可选</Tag>}
                                            </Space>
                                          }
                                          rules={
                                            attr.required
                                              ? [{ required: true, message: `请填写${attr.name || attr.attrId}` }]
                                              : undefined
                                          }
                                        >
                                          {opts.length > 0 ? (
                                            <Select
                                              allowClear={!attr.required}
                                              showSearch
                                              optionFilterProp="label"
                                              options={opts.map((o) => ({
                                                label: o.name || o.id || '',
                                                value: o.id || o.name,
                                              }))}
                                            />
                                          ) : (
                                            <Input placeholder={attr.valueType || '填写属性值'} />
                                          )}
                                        </Form.Item>
                                      </Col>
                                    );
                                  })}
                                </Row>
                              </Spin>
                            ) : null}
                            <Form.Item>
                              <Space wrap>
                                <Button type="primary" htmlType="submit" loading={douyinSaving}>
                                  保存抖店配置
                                </Button>
                                <Button
                                  onClick={() => {
                                    setReadinessPlat('douyin_shop');
                                    setReadinessShopId(String(douyinForm.getFieldValue('shopId') || ''));
                                    setDraftTabKey('readiness');
                                  }}
                                >
                                  查看抖店发布检查
                                </Button>
                              </Space>
                            </Form.Item>
                          </Form>
                        </Space>
                      </Card>
                      <Card size="small" title="抖店刊登草稿预览" variant="borderless">
                        <Space direction="vertical" style={{ width: '100%' }} size="middle">
                          <Alert
                            type="info"
                            showIcon
                            message="系统会根据商品标题、AI 文案、图片、SKU、定价和抖店要求填写的信息生成刊登草稿，发布前仍可人工修改。"
                          />
                          <Space wrap>
                            <Button type="primary" loading={douyinMappingLoading} onClick={() => void handleBuildDouyinMapping()}>
                              生成抖店刊登草稿
                            </Button>
                            <Button disabled={!douyinMapping} loading={douyinMappingSaving} onClick={() => void handleSaveDouyinMapping()}>
                              保存刊登草稿
                            </Button>
                            <Button loading={douyinMappingValidating} onClick={() => void handleValidateDouyinMapping()}>
                              校验刊登草稿
                            </Button>
                            <Button
                              icon={<CloudUploadOutlined />}
                              disabled={!douyinMapping}
                              loading={douyinImageUploading}
                              onClick={() => void handleUploadDouyinImages(false)}
                            >
                              上传图片到抖店
                            </Button>
                            <Button
                              icon={<ReloadOutlined />}
                              disabled={!douyinMapping}
                              loading={douyinImageUploading}
                              onClick={() => void handleUploadDouyinImages(true)}
                            >
                              重新上传全部图片
                            </Button>
                            <Button
                              type="primary"
                              disabled={douyinCreateDraftDisabled}
                              loading={douyinDraftCreating}
                              onClick={() => void handleCreateDouyinDraft()}
                            >
                              创建抖店商品草稿
                            </Button>
                            {douyinMapping?.lastMappedAt ? (
                              <Typography.Text type="secondary">最近生成：{formatDateTime(douyinMapping.lastMappedAt)}</Typography.Text>
                            ) : null}
                          </Space>
                          {!douyinMapping ? (
                            <Typography.Text type="secondary">还没有抖店刊登草稿，请先生成。</Typography.Text>
                          ) : (
                            <>
                              {douyinMapping.errors?.length ? (
                                <Alert
                                  type="error"
                                  showIcon
                                  message="这些信息不完整，暂时不能创建抖店商品"
                                  description={douyinIssueList(douyinMapping.errors)}
                                />
                              ) : null}
                              {douyinMapping.warnings?.length ? (
                                <Alert
                                  type="warning"
                                  showIcon
                                  message="这些信息建议人工确认"
                                  description={douyinIssueList(douyinMapping.warnings)}
                                />
                              ) : null}
                              <Form form={douyinMappingForm} layout="vertical">
                                <Form.Item name="title" label="抖店标题" rules={[{ required: true, message: '请填写抖店标题' }]}>
                                  <Input showCount maxLength={80} />
                                </Form.Item>
                                <Form.Item name="description" label="抖店描述">
                                  <Input.TextArea rows={4} />
                                </Form.Item>
                              </Form>
                              <Descriptions bordered size="small" column={2}>
                                <Descriptions.Item label="抖店店铺">{douyinMapping.shopId || '未选择'}</Descriptions.Item>
                                <Descriptions.Item label="抖店类目">
                                  {douyinMapping.categoryPath || douyinMapping.categoryId || '未选择'}
                                </Descriptions.Item>
                                <Descriptions.Item label="价格">
                                  {douyinMoney(douyinMapping.price?.min, douyinMapping.price?.currency)}
                                  {douyinMapping.price?.max && douyinMapping.price.max !== douyinMapping.price.min
                                    ? ` - ${douyinMoney(douyinMapping.price.max, douyinMapping.price.currency)}`
                                    : ''}
                                </Descriptions.Item>
                                <Descriptions.Item label="库存">
                                  {douyinMapping.stock?.total ?? '未确认'}
                                  {douyinMapping.stock?.unconfirmed ? <Tag color="orange" style={{ marginLeft: 8 }}>库存未确认</Tag> : null}
                                </Descriptions.Item>
                              </Descriptions>
                              <div>
                                <Typography.Title level={5}>主图</Typography.Title>
                                <Typography.Text type="secondary">图片需要先上传到抖店后，才能创建抖店商品草稿。</Typography.Text>
                                <Image.PreviewGroup>
                                  <Space wrap>
                                    {(douyinMapping.mainImages ?? []).map((img, idx) => (
                                      <div key={douyinImageKey(img, 'main', idx)} style={{ width: 180, marginTop: 8 }}>
                                        <Image src={douyinImagePreviewUrl(img)} width={112} height={112} style={{ objectFit: 'cover' }} />
                                        <Space direction="vertical" size={2} style={{ marginTop: 6, width: '100%' }}>
                                          {douyinStorageStatusTag(img)}
                                          {douyinImageStatusTag(img)}
                                          {img.platformImageId ? (
                                            <Tooltip title={`平台图片编号：${img.platformImageId}`}>
                                              <Typography.Text copyable={{ text: img.platformImageId }} type="secondary" style={{ fontSize: 12 }}>
                                                已获平台编号
                                              </Typography.Text>
                                            </Tooltip>
                                          ) : null}
                                          {img.uploadedAt ? <Typography.Text type="secondary" style={{ fontSize: 12 }}>{formatDateTime(img.uploadedAt)}</Typography.Text> : null}
                                          {img.errorMessage || img.errorCode ? (
                                            <Typography.Text type="danger" style={{ fontSize: 12 }}>
                                              {img.errorMessage || formatUserErrorMessage(img.errorCode)}
                                            </Typography.Text>
                                          ) : null}
                                          <Space size={4}>
                                            {douyinImagePreviewUrl(img) ? (
                                              <Button size="small" icon={<EyeOutlined />} href={douyinImagePreviewUrl(img)} target="_blank" />
                                            ) : null}
                                            {img.platformImageUrl ? (
                                              <Button size="small" href={img.platformImageUrl} target="_blank">平台图</Button>
                                            ) : null}
                                            <Button
                                              size="small"
                                              icon={<ReloadOutlined />}
                                              loading={douyinImageRetryingKey === douyinImageKey(img, 'main', idx)}
                                              onClick={() => void handleRetryDouyinImage(douyinImageKey(img, 'main', idx))}
                                            >
                                              重试
                                            </Button>
                                          </Space>
                                        </Space>
                                      </div>
                                    ))}
                                  </Space>
                                </Image.PreviewGroup>
                              </div>
                              <div>
                                <Typography.Title level={5}>详情图</Typography.Title>
                                {(douyinMapping.detailImages ?? []).length ? (
                                  <Image.PreviewGroup>
                                    <Space wrap>
                                      {(douyinMapping.detailImages ?? []).map((img, idx) => (
                                        <div key={douyinImageKey(img, 'detail', idx)} style={{ width: 180 }}>
                                          <Image src={douyinImagePreviewUrl(img)} width={112} height={112} style={{ objectFit: 'cover' }} />
                                          <Space direction="vertical" size={2} style={{ marginTop: 6, width: '100%' }}>
                                            {douyinStorageStatusTag(img)}
                                            {douyinImageStatusTag(img)}
                                            {img.platformImageId ? (
                                            <Tooltip title={`平台图片编号：${img.platformImageId}`}>
                                              <Typography.Text copyable={{ text: img.platformImageId }} type="secondary" style={{ fontSize: 12 }}>
                                                已获平台编号
                                              </Typography.Text>
                                            </Tooltip>
                                          ) : null}
                                            {img.uploadedAt ? <Typography.Text type="secondary" style={{ fontSize: 12 }}>{formatDateTime(img.uploadedAt)}</Typography.Text> : null}
                                            {img.errorMessage || img.errorCode ? (
                                            <Typography.Text type="danger" style={{ fontSize: 12 }}>
                                              {img.errorMessage || formatUserErrorMessage(img.errorCode)}
                                            </Typography.Text>
                                          ) : null}
                                            <Space size={4}>
                                              {douyinImagePreviewUrl(img) ? (
                                                <Button size="small" icon={<EyeOutlined />} href={douyinImagePreviewUrl(img)} target="_blank" />
                                              ) : null}
                                              {img.platformImageUrl ? (
                                                <Button size="small" href={img.platformImageUrl} target="_blank">平台图</Button>
                                              ) : null}
                                              <Button
                                                size="small"
                                                icon={<ReloadOutlined />}
                                                loading={douyinImageRetryingKey === douyinImageKey(img, 'detail', idx)}
                                                onClick={() => void handleRetryDouyinImage(douyinImageKey(img, 'detail', idx))}
                                              >
                                                重试
                                              </Button>
                                            </Space>
                                          </Space>
                                        </div>
                                      ))}
                                    </Space>
                                  </Image.PreviewGroup>
                                ) : (
                                  <Typography.Text type="secondary">暂无详情图</Typography.Text>
                                )}
                              </div>
                              <Table
                                size="small"
                                rowKey={(r) => r.attrId || r.name}
                                pagination={false}
                                dataSource={douyinMapping.attributes ?? []}
                                columns={[
                                  { title: '抖店要求填写的信息', render: (_, r) => r.name || r.attrId },
                                  { title: '状态', width: 90, render: (_, r) => (r.required ? <Tag color="red">必填</Tag> : <Tag>可选</Tag>) },
                                  { title: '当前值', render: (_, r) => douyinAttrValueText(r.value) },
                                ]}
                              />
                              <Table
                                size="small"
                                rowKey={(r) => r.localSkuId || r.name}
                                pagination={false}
                                dataSource={douyinMapping.skus ?? []}
                                columns={[
                                  { title: '商品规格', dataIndex: 'name', ellipsis: true },
                                  { title: '规格值', render: (_, r) => douyinAttrValueText(r.attrs ?? {}) },
                                  { title: '售价', width: 110, render: (_, r) => douyinMoney(r.price, douyinMapping.price?.currency) },
                                  { title: '库存', width: 90, render: (_, r) => (r.stock == null ? '未确认' : r.stock) },
                                  { title: '规格图', width: 90, render: (_, r) => (r.imageUrl ? <Image src={r.imageUrl} width={40} height={40} /> : '无') },
                                ]}
                              />
                            </>
                          )}
                        </Space>
                      </Card>
                      <Card size="small" title="抖店刊登任务" variant="borderless" loading={douyinPublishTasksLoading}>
                        {douyinPublishTasks.length === 0 ? (
                          <Typography.Text type="secondary">暂无抖店刊登任务</Typography.Text>
                        ) : (
                          <Table
                            size="small"
                            rowKey="id"
                            pagination={false}
                            dataSource={douyinPublishTasks}
                            columns={[
                              { title: '状态', dataIndex: 'status', width: 100, render: (_, r) => tagFromPublishStatus(r.status) },
                              { title: '发布模式', dataIndex: 'publishMode', width: 140, render: (v) => publishModeLabel(v) },
                              { title: '抖店商品 ID', dataIndex: 'platformProductId', ellipsis: true, render: (v) => v || '—' },
                              { title: '创建时间', dataIndex: 'createdAt', width: 168, render: (v) => formatDateTime(v) },
                              {
                                title: '失败原因',
                                dataIndex: 'errorMessage',
                                ellipsis: true,
                                render: (v, r) => {
                                  const text = (v as string) || formatUserErrorMessage(r.errorCode);
                                  return text || '—';
                                },
                              },
                              {
                                title: '操作',
                                width: 120,
                                render: (_, r) => (
                                  <Space size={4}>
                                    <Link to={`/product/publish-tasks?productId=${id}`}>详情</Link>
                                    {r.status === 'failed' && r.retryable !== false ? (
                                      <Button
                                        type="link"
                                        size="small"
                                        onClick={() =>
                                          void retryProductPublishTask(r.id)
                                            .then(() => {
                                              message.success('已重试刊登任务');
                                              void reloadDouyinPublishTasks();
                                            })
                                            .catch((e: Error) => message.error(e.message || '重试失败'))
                                        }
                                      >
                                        重试
                                      </Button>
                                    ) : null}
                                  </Space>
                                ),
                              },
                            ]}
                          />
                        )}
                      </Card>
                      <Card
                        size="small"
                        title="抖店规格绑定"
                        variant="borderless"
                        loading={douyinSkuBindingLoading}
                        extra={
                          <Space>
                            <Button size="small" onClick={() => setDouyinSkuCandidatesOpen(true)} disabled={!douyinSkuBinding?.platformSkus?.length}>
                              查看平台 SKU 候选
                            </Button>
                            <Button size="small" onClick={() => void reloadDouyinSkuBindings()}>
                              刷新绑定状态
                            </Button>
                            <Button
                              type="primary"
                              size="small"
                              loading={douyinSkuBindingSyncing}
                              disabled={!douyinPublication?.id}
                              onClick={() => void handleSyncDouyinSkuBindings()}
                            >
                              重新校准
                            </Button>
                          </Space>
                        }
                      >
                        {!douyinPublication?.id ? (
                          <Typography.Text type="secondary">
                            创建抖店商品草稿后，可在此根据抖店商品详情校准 platformSkuId，并对 ambiguous / unmatched 规格手动绑定。
                          </Typography.Text>
                        ) : (
                          <Space direction="vertical" style={{ width: '100%' }} size="middle">
                            {douyinSkuBinding?.inventorySyncReady === false && douyinSkuBinding.inventorySyncBlockReason ? (
                              <Alert type="warning" showIcon message={douyinSkuBinding.inventorySyncBlockReason} />
                            ) : douyinSkuBinding?.inventorySyncReady ? (
                              <Alert type="success" showIcon message="全部规格已绑定抖店 SKU，可同步库存。" />
                            ) : null}
                            <Descriptions bordered size="small" column={2}>
                              <Descriptions.Item label="抖店商品 ID">
                                {douyinPublication.externalProductId || '—'}
                              </Descriptions.Item>
                              <Descriptions.Item label="最近校准时间">
                                {douyinSkuBinding?.skuBindingSyncedAt
                                  ? formatDateTime(douyinSkuBinding.skuBindingSyncedAt)
                                  : douyinPublication.skuBindingSyncedAt
                                    ? formatDateTime(douyinPublication.skuBindingSyncedAt)
                                    : '—'}
                              </Descriptions.Item>
                              <Descriptions.Item label="已绑定">{douyinSkuBinding?.bound ?? '—'}</Descriptions.Item>
                              <Descriptions.Item label="未绑定">{douyinSkuBinding?.unmatched ?? '—'}</Descriptions.Item>
                              <Descriptions.Item label="待确认">{douyinSkuBinding?.ambiguous ?? '—'}</Descriptions.Item>
                              <Descriptions.Item label="失败">{douyinSkuBinding?.failed ?? '—'}</Descriptions.Item>
                            </Descriptions>
                            {(douyinSkuBinding?.rows?.length ?? 0) > 0 ? (
                              <Table<DouyinSkuBindingRow>
                                size="small"
                                rowKey="publicationSkuId"
                                pagination={false}
                                scroll={{ x: 1200 }}
                                dataSource={douyinSkuBinding?.rows ?? []}
                                columns={[
                                  { title: '本地 SKU', dataIndex: 'skuCode', width: 120, ellipsis: true, render: (v, r) => v || r.productSkuId || '—' },
                                  { title: '本地规格', dataIndex: 'specName', width: 140, ellipsis: true, render: (v) => v || '—' },
                                  {
                                    title: '本地价格',
                                    width: 96,
                                    render: (_, r) => (typeof r.price === 'number' ? r.price.toFixed(2) : '—'),
                                  },
                                  {
                                    title: '本地库存',
                                    width: 88,
                                    render: (_, r) => (typeof r.stock === 'number' ? r.stock : '—'),
                                  },
                                  { title: '平台规格编号', dataIndex: 'externalSkuId', width: 140, ellipsis: true, render: (v) => v || '—' },
                                  { title: '抖店 SKU 名称', dataIndex: 'platformSkuName', width: 140, ellipsis: true, render: (v) => v || '—' },
                                  { title: '绑定状态', dataIndex: 'bindStatus', width: 96, render: (v) => douyinBindStatusTag(v) },
                                  { title: '置信度', dataIndex: 'bindConfidence', width: 72, render: (v) => (typeof v === 'number' ? v : '—') },
                                  {
                                    title: '最近校准',
                                    dataIndex: 'lastSyncedAt',
                                    width: 156,
                                    render: (v) => (v ? formatDateTime(v) : '—'),
                                  },
                                  {
                                    title: '说明',
                                    dataIndex: 'bindMessage',
                                    ellipsis: true,
                                    render: (v, r) => v || douyinBindStatusHint(r.bindStatus),
                                  },
                                  {
                                    title: '操作',
                                    width: 220,
                                    fixed: 'right',
                                    render: (_, r) => (
                                      <Space size={4} wrap>
                                        <Button
                                          type="link"
                                          size="small"
                                          style={{ padding: 0 }}
                                          onClick={() => {
                                            setDouyinSkuBindTarget(r);
                                            douyinSkuBindForm.setFieldsValue({ platformSkuId: r.externalSkuId || undefined });
                                            setDouyinSkuBindOpen(true);
                                          }}
                                        >
                                          手动绑定
                                        </Button>
                                        {r.externalSkuId ? (
                                          <Popconfirm
                                            title="确认解除该规格的抖店规格绑定？"
                                            onConfirm={() =>
                                              void unbindDouyinSku(r.publicationSkuId)
                                                .then(async () => {
                                                  message.success('已解除绑定');
                                                  await reloadDouyinSkuBindings();
                                                  await reloadPublicationSkus();
                                                })
                                                .catch((e: Error) => message.error(e.message || '解除失败'))
                                            }
                                          >
                                            <Button type="link" size="small" style={{ padding: 0 }} danger>
                                              解除绑定
                                            </Button>
                                          </Popconfirm>
                                        ) : null}
                                      </Space>
                                    ),
                                  },
                                ]}
                              />
                            ) : (
                              <Typography.Text type="secondary">点击「重新校准」从抖店拉取 SKU 并完成匹配；未匹配或待确认规格可手动绑定。</Typography.Text>
                            )}
                          </Space>
                        )}
                      </Card>
                      {publishReadiness ? (
                        <Alert
                          type={
                            !publishReadiness.canPublish
                              ? 'error'
                              : publishReadiness.warningCount > 0
                                ? 'warning'
                                : 'success'
                          }
                          showIcon
                          message={
                            <Space wrap align="center">
                              <span>发布检查</span>
                              {readinessStatusTag(publishReadiness)}
                              <Typography.Text type="secondary">
                                分 {publishReadiness.score} · 错误 {publishReadiness.errorCount} · 警告{' '}
                                {publishReadiness.warningCount}
                              </Typography.Text>
                              <Button
                                type="link"
                                size="small"
                                style={{ padding: 0 }}
                                onClick={() => setDraftTabKey('readiness')}
                              >
                                查看明细
                              </Button>
                            </Space>
                          }
                          description={
                            publishReadiness.checks.length ? (
                              <div>
                                {readinessCheckList(publishReadiness.checks, 5)}
                                {publishReadiness.checks.length > 5 ? (
                                  <Typography.Text type="secondary">
                                    … 共 {publishReadiness.checks.length} 项
                                  </Typography.Text>
                                ) : null}
                              </div>
                            ) : (
                              '未发现问题'
                            )
                          }
                        />
                      ) : null}
                      <Form
                        form={publishForm}
                        layout="vertical"
                        style={{ maxWidth: 560 }}
                        onFinish={async (vals: { shopId?: string }) => {
                          const shopId = String(vals.shopId ?? '').trim();
                          if (!shopId) {
                            message.error('请选择店铺');
                            return;
                          }
                          const shop = eligibleShopsForPublish.find((s) => s.id === shopId);
                          if (!shop) {
                            message.error('店铺不可用');
                            return;
                          }
                          setPublishSubmitting(true);
                          try {
                            const r = await getProductReadiness(id, {
                              platform: shop.platform,
                              shopId,
                              mode: 'publish',
                            });
                            setPublishReadiness(r);
                            if (!r.canPublish) {
                              Modal.error({
                                title: '发布检查未通过',
                                width: 600,
                                content: <div>{readinessCheckList(r.checks)}</div>,
                              });
                              return;
                            }
                            if ((r.warningCount ?? 0) > 0) {
                              await new Promise<void>((resolve, reject) => {
                                Modal.confirm({
                                  title: '发布检查存在警告，确认继续？',
                                  width: 640,
                                  okText: '确认创建刊登任务',
                                  cancelText: '返回处理',
                                  content: <div>{readinessCheckList((r.checks || []).filter((c) => c.level !== 'error'), 10)}</div>,
                                  onOk: () => resolve(),
                                  onCancel: () => reject(new Error('cancelled')),
                                });
                              });
                            }
                            const task = await publishProduct(id, { shopId, options: {} });
                            if (task.readiness) setPublishReadiness(task.readiness);
                            message.success('已提交刊登任务');
                            publishForm.resetFields();
                            setPublishReadiness(null);
                            await reloadPublishContext();
                          } catch (e: unknown) {
                            const ex = e as Error & { data?: unknown };
                            if (ex.message === 'cancelled') return;
                            if (ex.message === 'product readiness check failed' && ex.data && typeof ex.data === 'object') {
                              const r = ex.data as ProductReadinessResult;
                              setPublishReadiness(r);
                              Modal.error({
                                title: '发布检查未通过',
                                width: 600,
                                content: <div>{readinessCheckList(r.checks || [])}</div>,
                              });
                            } else {
                              message.error((ex as Error)?.message || '提交失败');
                            }
                          } finally {
                            setPublishSubmitting(false);
                          }
                        }}
                      >
                        <Form.Item
                          name="shopId"
                          label="目标店铺（已授权且刊登可用 / beta）"
                          rules={[{ required: true, message: '请选择店铺' }]}
                        >
                          <Select
                            placeholder="选择店铺"
                            allowClear
                            showSearch
                            optionFilterProp="label"
                            onChange={(v) => void refreshPublishReadiness(v ? String(v) : '')}
                            options={eligibleShopsForPublish.map((s) => {
                              const m = platformsMeta.find((x) => x.platform === s.platform);
                              const st = m?.capabilityStatus?.product_publish;
                              const betaTag = st === 'beta' ? ' [测试中/beta]' : '';
                              return {
                                label: `${s.shopName} (${s.platform})${betaTag}`,
                                value: s.id,
                              };
                            })}
                          />
                        </Form.Item>
                        <Form.Item>
                          <Space wrap>
                            <Button
                              type="primary"
                              htmlType="submit"
                              loading={publishSubmitting}
                              disabled={!!publishReadiness && !publishReadiness.canPublish}
                            >
                              提交刊登
                            </Button>
                            <Button onClick={() => void reloadPublishContext()}>刷新快照</Button>
                          </Space>
                        </Form.Item>
                      </Form>
                      <Typography.Title level={5} style={{ marginTop: 0, marginBottom: 0 }}>
                        本商品刊登记录
                      </Typography.Title>
                      <Table<ProductPublicationRow>
                        size="small"
                        rowKey="id"
                        loading={pubCtxLoading}
                        dataSource={pubRows}
                        pagination={false}
                        columns={[
                          { title: '店铺', render: (_, r) => r.shopName || r.shopId },
                          { title: '平台', dataIndex: 'platform', width: 96 },
                          { title: '状态', dataIndex: 'publishStatus', width: 100 },
                          { title: '外部商品 ID', dataIndex: 'externalProductId', ellipsis: true },
                          {
                            title: '外链',
                            render: (_, r) =>
                              r.externalUrl ? (
                                <Typography.Link href={r.externalUrl} target="_blank" rel="noreferrer">
                                  打开
                                </Typography.Link>
                              ) : (
                                '—'
                              ),
                          },
                        ]}
                      />
                    </Space>
                  </Card>
                </Spin>
              ),
            },
          ]}
        />
      ) : null}

      <ModalForm
        title={imgEdit ? '编辑商品图片' : '添加商品图片'}
        open={!!id && (imgModalOpen || !!imgEdit)}
        onOpenChange={(open) => {
          if (!open) {
            setImgModalOpen(false);
            setImgEdit(null);
            setLastUpload(null);
          }
        }}
        key={imgEdit ? `img-${imgEdit.id}` : imgModalOpen ? 'img-add' : 'img-closed'}
        modalProps={{ destroyOnHidden: true, width: 560 }}
        initialValues={{
          imageType: imgEdit ? (imgEdit.imageType === 'description' ? 'detail' : imgEdit.imageType) : 'main',
          sortOrder: imgEdit?.sortOrder ?? sortedImages.length,
          publicUrl: imgEdit?.publicUrl ?? '',
          originUrl: imgEdit?.originUrl ?? '',
          objectKey: imgEdit?.objectKey ?? '',
        }}
        onFinish={async (vals) => {
          setImgBusy(true);
          try {
            const imageType = String(vals.imageType ?? 'main');
            const sortOrder = vals.sortOrder != null ? Number(vals.sortOrder) : undefined;
            if (imgEdit) {
              await updateProductImage(id, imgEdit.id, {
                imageType,
                sortOrder,
                publicUrl: String(vals.publicUrl ?? ''),
                originUrl: String(vals.originUrl ?? ''),
                objectKey: String(vals.objectKey ?? ''),
              });
              message.success('已更新');
            } else {
              const body: Parameters<typeof createProductImage>[1] = {
                imageType,
                sortOrder,
                publicUrl: String(vals.publicUrl ?? '').trim(),
                originUrl: String(vals.originUrl ?? '').trim(),
                objectKey: String(vals.objectKey ?? '').trim(),
              };
              if (lastUpload?.id) {
                body.fileId = lastUpload.id;
                if (!body.publicUrl) body.publicUrl = lastUpload.url;
                if (!body.originUrl) body.originUrl = lastUpload.url;
                if (!body.objectKey) body.objectKey = lastUpload.objectKey;
              }
              await createProductImage(id, body);
              message.success('已添加');
            }
            setImgModalOpen(false);
            setImgEdit(null);
            setLastUpload(null);
            await reloadDetail();
            return true;
          } catch (e: unknown) {
            message.error((e as Error)?.message || '失败');
            return false;
          } finally {
            setImgBusy(false);
          }
        }}
        submitter={{
          searchConfig: { submitText: imgEdit ? '保存' : '添加' },
          submitButtonProps: { loading: imgBusy },
        }}
      >
        <ProFormSelect name="imageType" label="图片类型" options={IMAGE_TYPE_OPTIONS} rules={[{ required: true }]} />
        <ProFormDigit
          name="sortOrder"
          label={PRODUCT_IMAGE_SORT_ORDER_LABEL}
          min={0}
          fieldProps={{ style: { width: '100%' } }}
        />
        {!imgEdit ? (
          <Form.Item label="上传文件（可选）">
            <Upload
              maxCount={1}
              showUploadList
              customRequest={async (opt: UploadRequestOption) => {
                try {
                  const f = opt.file as File;
                  const up = await uploadFile(f);
                  setLastUpload({ id: up.id, url: up.url, objectKey: up.objectKey });
                  opt.onSuccess?.(up, new XMLHttpRequest());
                  message.success('已上传，保存时将关联到商品');
                } catch (e: unknown) {
                  opt.onError?.(e as Error);
                  message.error((e as Error)?.message || '上传失败');
                }
              }}
            >
              <Button icon={<PlusOutlined />}>选择图片并上传</Button>
            </Upload>
          </Form.Item>
        ) : null}
        <ProFormText
          name="publicUrl"
          label={PRODUCT_IMAGE_PUBLIC_URL_LABEL}
          placeholder="https:// 或 /static/…"
        />
        <ProFormText
          name="originUrl"
          label={PRODUCT_IMAGE_ORIGIN_URL_LABEL}
          placeholder="外部原图地址（可选）"
        />
        <ProFormText
          name="objectKey"
          label={PRODUCT_IMAGE_OBJECT_KEY_LABEL}
          placeholder="存储路径（可选）"
        />
      </ModalForm>

      <Modal
        title="AI 标题优化"
        open={aiOpen}
        onCancel={() => setAiOpen(false)}
        footer={null}
        destroyOnHidden
        width={640}
      >
        <Form
          form={aiForm}
          layout="vertical"
          initialValues={{ language: 'en', platform: 'TikTok Shop', maxLength: 120 }}
          onFinish={async (v) => {
            setAiBusy(true);
            setAiResult(null);
            try {
              const res = await optimizeProductTitle(id, {
                language: String(v.language ?? ''),
                platform: String(v.platform ?? ''),
                maxLength: Number(v.maxLength ?? 120),
              });
              setAiResult(res);
              message.success('优化完成');
              await reloadTasks();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '优化失败');
            } finally {
              setAiBusy(false);
            }
          }}
        >
          <Form.Item name="language" label="语言" rules={[{ required: true }]}>
            <Input placeholder="例如 en" />
          </Form.Item>
          <Form.Item name="platform" label="平台" rules={[{ required: true }]}>
            <Input placeholder="TikTok Shop" />
          </Form.Item>
          <Form.Item name="maxLength" label="最长字符数" rules={[{ required: true }]}>
            <InputNumber min={20} max={500} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={aiBusy}>
              运行优化
            </Button>
          </Form.Item>
        </Form>

        {aiResult ? (
          <div style={{ marginTop: 16 }}>
            <Typography.Title level={5} style={{ marginTop: 0 }}>
              输出
            </Typography.Title>
            <Descriptions bordered size="small" column={1} style={{ marginBottom: 16 }}>
              <Descriptions.Item label="优化标题">{aiResult.optimizedTitle || '—'}</Descriptions.Item>
              <Descriptions.Item label="关键词">
                {(aiResult.keywords ?? []).length ? aiResult.keywords.join('、') : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="说明">{aiResult.reason || '—'}</Descriptions.Item>
              <Descriptions.Item label="任务 ID">{aiResult.taskId}</Descriptions.Item>
            </Descriptions>
            <Button
              type="primary"
              disabled={!aiResult.optimizedTitle}
              loading={aiBusy}
              onClick={async () => {
                if (!aiResult?.taskId) return;
                setAiBusy(true);
                try {
                  await applyProductAITitle(id, {
                    aiTitle: aiResult.optimizedTitle,
                    taskId: aiResult.taskId,
                  });
                  message.success('已应用为 AI 标题');
                  setAiOpen(false);
                  setAiResult(null);
                  await reloadDetail();
                  await reloadTasks();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '应用失败');
                } finally {
                  setAiBusy(false);
                }
              }}
            >
              应用为 AI 标题
            </Button>
          </div>
        ) : null}
      </Modal>

      <Modal
        title="AI 描述生成"
        open={descOpen}
        onCancel={() => setDescOpen(false)}
        footer={null}
        destroyOnHidden
        width={720}
      >
        <Form
          form={descForm}
          layout="vertical"
          initialValues={{ language: 'en', platform: 'TikTok Shop', tone: 'professional' }}
          onFinish={async (v) => {
            setDescBusy(true);
            setDescResult(null);
            try {
              const res = await generateDescription(id, {
                language: String(v.language ?? ''),
                platform: String(v.platform ?? ''),
                tone: String(v.tone ?? ''),
              });
              setDescResult(res);
              message.success('生成完成');
              await reloadTasks();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '生成失败');
            } finally {
              setDescBusy(false);
            }
          }}
        >
          <Form.Item name="language" label="语言" rules={[{ required: true }]}>
            <Input placeholder="例如 en" />
          </Form.Item>
          <Form.Item name="platform" label="平台" rules={[{ required: true }]}>
            <Input placeholder="TikTok Shop" />
          </Form.Item>
          <Form.Item name="tone" label="语气" rules={[{ required: true }]}>
            <Input placeholder="例如 professional" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={descBusy}>
              生成描述
            </Button>
          </Form.Item>
        </Form>

        {descResult ? (
          <div style={{ marginTop: 16 }}>
            <Typography.Title level={5} style={{ marginTop: 0 }}>
              输出
            </Typography.Title>
            <Descriptions bordered size="small" column={1} style={{ marginBottom: 16 }}>
              <Descriptions.Item label="描述">{descResult.description || '—'}</Descriptions.Item>
              <Descriptions.Item label="Highlights">
                {(descResult.highlights ?? []).length ? descResult.highlights.join('；') : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="Specifications">
                {(descResult.specifications ?? []).length ? descResult.specifications.join('；') : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="Package includes">
                {(descResult.packageIncludes ?? []).length ? descResult.packageIncludes.join('；') : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="Notes">{descResult.notes || '—'}</Descriptions.Item>
              <Descriptions.Item label="Reason">{descResult.reason || '—'}</Descriptions.Item>
              <Descriptions.Item label="任务 ID">{descResult.taskId}</Descriptions.Item>
            </Descriptions>
            <Button
              type="primary"
              disabled={!descResult.taskId || !buildAiDescriptionText(descResult)}
              loading={descBusy}
              onClick={async () => {
                if (!descResult?.taskId) return;
                const text = buildAiDescriptionText(descResult);
                if (!text) return;
                setDescBusy(true);
                try {
                  await applyAiDescription(id, {
                    aiDescription: text,
                    taskId: descResult.taskId,
                  });
                  message.success('已应用为 AI 描述');
                  setDescOpen(false);
                  setDescResult(null);
                  await reloadDetail();
                  await reloadTasks();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '应用失败');
                } finally {
                  setDescBusy(false);
                }
              }}
            >
              应用为 AI 描述
            </Button>
          </div>
        ) : null}
      </Modal>

      <Drawer
        title={logsSku ? `库存变更 · ${logsSku.skuCode || logsSku.id}` : '库存变更'}
        open={logsOpen}
        width={560}
        destroyOnHidden
        onClose={() => {
          setLogsOpen(false);
          setLogsSku(null);
          setLogsRows([]);
        }}
      >
        <Spin spinning={logsLoading}>
          <Table<InventoryChangeLogRow>
            rowKey="id"
            size="small"
            pagination={false}
            dataSource={logsRows}
            columns={[
              {
                title: '时间',
                dataIndex: 'createdAt',
                width: 168,
                render: (v: string) => formatDateTime(v),
              },
              { title: '类型', dataIndex: 'changeType', width: 136 },
              { title: '前', width: 56, dataIndex: 'beforeStock' },
              { title: '后', width: 56, dataIndex: 'afterStock' },
              { title: 'Δ', width: 56, dataIndex: 'delta' },
              { title: '原因', ellipsis: true, dataIndex: 'reason' },
              { title: '备注', ellipsis: true, dataIndex: 'remark' },
            ]}
          />
        </Spin>
      </Drawer>

      <Modal
        title={adjustTarget ? `调整库存 · ${adjustTarget.skuCode}` : '调整库存'}
        open={adjustOpen && !!adjustTarget}
        destroyOnHidden
        okText="保存"
        confirmLoading={invAdjustSubmitting}
        onCancel={() => {
          setAdjustOpen(false);
          setAdjustTarget(null);
          adjustForm.resetFields();
        }}
        onOk={async () => {
          if (!adjustTarget) return;
          const v = await adjustForm.validateFields();
          const stock = Number(v.stock);
          setInvAdjustSubmitting(true);
          try {
            await adjustSkuStock(id, adjustTarget.id, {
              stock,
              reason: String(v.reason ?? 'manual_adjust').trim(),
              remark: String(v.remark ?? ''),
              sync: false,
            });
            message.success('库存已更新');
            setAdjustOpen(false);
            setAdjustTarget(null);
            adjustForm.resetFields();
            await reloadDetail();
            await reloadPublicationSkus();
          } catch (e: unknown) {
            message.error((e as Error)?.message || '调整失败');
          } finally {
            setInvAdjustSubmitting(false);
          }
        }}
      >
        <Form form={adjustForm} layout="vertical">
          <Form.Item name="stock" label="库存（≥0）" rules={[{ required: true }]}>
            <InputNumber min={0} step={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="reason" label="原因标识">
            <Input placeholder="manual_adjust" />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} placeholder="盘点修正…" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="同步刊登 SKU 库存"
        open={syncOpen && !!syncRow}
        destroyOnHidden
        okText="提交任务"
        confirmLoading={syncSubmitting}
        onCancel={() => {
          setSyncOpen(false);
          setSyncRow(null);
          syncForm.resetFields();
        }}
        onOk={async () => {
          if (!syncRow) return;
          const v = await syncForm.validateFields();
          const stock = Number(v.stock);
          setSyncSubmitting(true);
          try {
            await syncPublicationSkuInventory(syncRow.publicationSkuId, {
              stock,
              options: {},
            });
            message.success('库存同步任务已创建');
            setSyncOpen(false);
            setSyncRow(null);
            syncForm.resetFields();
            await reloadPublicationSkus();
          } catch (e: unknown) {
            message.error(formatInventorySyncTaskCreateError(e));
          } finally {
            setSyncSubmitting(false);
          }
        }}
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          平台：{syncRow?.platform ?? '—'}；店铺：{syncRow?.shopName ?? syncRow?.shopId ?? '—'}
        </Typography.Paragraph>
        <InventorySyncPlatformHint platform={syncRow?.platform} />
        <Form form={syncForm} layout="vertical">
          <Form.Item
            name="stock"
            label="推送到平台的库存数量"
            rules={[{ required: true, message: '必填且 ≥0' }]}
          >
            <InputNumber min={0} step={1} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={stockSettingsTarget ? `预警线 · ${stockSettingsTarget.skuCode}` : '预警线'}
        open={stockSettingsOpen && !!stockSettingsTarget}
        destroyOnHidden
        okText="保存"
        confirmLoading={stockSettingsSubmitting}
        onCancel={() => {
          setStockSettingsOpen(false);
          setStockSettingsTarget(null);
          stockSettingsForm.resetFields();
        }}
        onOk={async () => {
          if (!stockSettingsTarget) return;
          const v = await stockSettingsForm.validateFields();
          if (v.safetyStock > v.warningStock) {
            message.error('安全线不能大于预警线');
            return;
          }
          setStockSettingsSubmitting(true);
          try {
            await updateProductSkuStockSettings(id, stockSettingsTarget.id, {
              warningStock: v.warningStock,
              safetyStock: v.safetyStock,
            });
            message.success('已保存');
            setStockSettingsOpen(false);
            setStockSettingsTarget(null);
            stockSettingsForm.resetFields();
            await reloadDetail();
          } catch (e: unknown) {
            message.error((e as Error)?.message || '失败');
          } finally {
            setStockSettingsSubmitting(false);
          }
        }}
      >
        <Form form={stockSettingsForm} layout="vertical">
          <Form.Item name="warningStock" label="预警库存线" rules={[{ required: true }]}>
            <InputNumber min={0} step={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="safetyStock" label="安全库存线" rules={[{ required: true }]}>
            <InputNumber min={0} step={1} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={douyinSkuBindTarget ? `手动绑定抖店 SKU · ${douyinSkuBindTarget.specName || douyinSkuBindTarget.skuCode || ''}` : '手动绑定抖店 SKU'}
        open={douyinSkuBindOpen && !!douyinSkuBindTarget}
        destroyOnHidden
        okText="确认绑定"
        confirmLoading={douyinSkuBindSubmitting}
        onCancel={() => {
          setDouyinSkuBindOpen(false);
          setDouyinSkuBindTarget(null);
          douyinSkuBindForm.resetFields();
        }}
        onOk={async () => {
          if (!douyinSkuBindTarget) return;
          const v = await douyinSkuBindForm.validateFields();
          const platformSkuId = String(v.platformSkuId ?? '').trim();
          if (!platformSkuId) {
            message.error('请选择或填写平台规格编号');
            return;
          }
          const selected = (douyinSkuBinding?.platformSkus ?? []).find((c) => c.platformSkuId === platformSkuId);
          setDouyinSkuBindSubmitting(true);
          try {
            await bindDouyinSku(douyinSkuBindTarget.publicationSkuId, {
              platformSkuId,
              platformSkuName: String(v.platformSkuName ?? selected?.specName ?? '').trim(),
              bindReason: 'manual',
            });
            message.success('手动绑定成功');
            setDouyinSkuBindOpen(false);
            setDouyinSkuBindTarget(null);
            douyinSkuBindForm.resetFields();
            await reloadDouyinSkuBindings();
            await reloadPublicationSkus();
          } catch (e: unknown) {
            message.error((e as Error)?.message || '绑定失败');
          } finally {
            setDouyinSkuBindSubmitting(false);
          }
        }}
      >
        <Typography.Paragraph type="secondary">
          本地规格：{douyinSkuBindTarget?.specName || douyinSkuBindTarget?.skuCode || '—'}
        </Typography.Paragraph>
        <Form form={douyinSkuBindForm} layout="vertical">
          <Form.Item name="platformSkuId" label="抖店 SKU" rules={[{ required: true, message: '请选择抖店 SKU' }]}>
            <Select
              showSearch
              placeholder="从平台候选中选择"
              optionFilterProp="label"
              options={(douyinSkuBinding?.platformSkus ?? []).map((c: DouyinPlatformSkuCandidate) => ({
                value: c.platformSkuId,
                label: `${c.platformSkuId} · ${c.specName || '—'}${c.boundToPublicationSkuId ? '（已绑定其他规格）' : ''}`,
                disabled: Boolean(c.boundToPublicationSkuId && c.boundToPublicationSkuId !== douyinSkuBindTarget?.publicationSkuId),
              }))}
              onChange={(v) => {
                const c = (douyinSkuBinding?.platformSkus ?? []).find((x) => x.platformSkuId === v);
                if (c?.specName) douyinSkuBindForm.setFieldValue('platformSkuName', c.specName);
              }}
            />
          </Form.Item>
          <Form.Item name="platformSkuName" label="抖店 SKU 名称">
            <Input placeholder="可选，便于识别" />
          </Form.Item>
        </Form>
      </Modal>

      <Drawer
        title="抖店平台 SKU 候选"
        open={douyinSkuCandidatesOpen}
        width={640}
        onClose={() => setDouyinSkuCandidatesOpen(false)}
      >
        {(douyinSkuBinding?.platformSkus?.length ?? 0) === 0 ? (
          <Typography.Text type="secondary">暂无候选，请先执行「重新校准」从抖店拉取商品详情。</Typography.Text>
        ) : (
          <Table<DouyinPlatformSkuCandidate>
            size="small"
            rowKey="platformSkuId"
            pagination={false}
            dataSource={douyinSkuBinding?.platformSkus ?? []}
            columns={[
              { title: '平台规格编号', dataIndex: 'platformSkuId', ellipsis: true },
              { title: '规格名称', dataIndex: 'specName', ellipsis: true, render: (v) => v || '—' },
              { title: '价格', width: 96, render: (_, r) => (typeof r.priceYuan === 'number' ? r.priceYuan.toFixed(2) : '—') },
              { title: '库存', width: 72, render: (_, r) => (typeof r.stock === 'number' ? r.stock : '—') },
              {
                title: '绑定状态',
                width: 120,
                render: (_, r) =>
                  r.boundToPublicationSkuId ? <Tag color="blue">已绑定本地规格</Tag> : <Tag>未绑定</Tag>,
              },
            ]}
          />
        )}
      </Drawer>

      <PricingApplyModal
        open={pricingOpen}
        onClose={() => setPricingOpen(false)}
        mode="product"
        productId={id}
        onApplied={() => void reloadDetail()}
      />

      <CreateImageTaskModal
        open={createImageOpen}
        onOpenChange={setCreateImageOpen}
        prefill={createImagePrefill}
        fixedProductId={id}
        productImages={sortedImages}
        onSuccess={() => void reloadDetail()}
      />

      <TranslateImageTextModal
        open={translateImageOpen}
        onOpenChange={setTranslateImageOpen}
        prefill={translateImagePrefill}
        fixedProductId={id}
        sourceImage={translateSourceImage}
        onSuccess={() => void reloadDetail()}
      />
    </TmPageContainer>
  );
}

function buildAiDescriptionText(r: GenerateDescriptionResult): string {
  const lines: string[] = [];
  const d = (r.description ?? '').trim();
  if (d) lines.push(d);
  const bullets = (title: string, items: string[]) => {
    const trimmed = (items ?? []).map((x) => x.trim()).filter(Boolean);
    if (!trimmed.length) return;
    lines.push('', title);
    for (const x of trimmed) lines.push(`- ${x}`);
  };
  bullets('Product Highlights', r.highlights ?? []);
  bullets('Specifications', r.specifications ?? []);
  bullets('Package Includes', r.packageIncludes ?? []);
  const notes = (r.notes ?? '').trim();
  if (notes) {
    lines.push('', 'Notes', notes);
  }
  return lines.join('\n').trim();
}
