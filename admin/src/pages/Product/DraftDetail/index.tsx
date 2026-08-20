import type { CSSProperties, Key, ReactNode } from 'react';
import type { UploadRequestOption } from 'rc-upload/lib/interface';
import { formatDateTime } from '@/utils/formatTime';
import { isProductionBuild } from '@/utils/runtimeEnvironment';
import type { ProColumns } from '@ant-design/pro-components';
import {
  OperationToolbar,
  EmptyState,
  ErrorAlert,
  MetricCard,
  SectionCard,
  StatusTag,
  TmPageContainer,
  TechnicalDetails,
  TaskJsonBlock,
  TmProTable as ProTable,
} from '@/components/ui';
import { commonStatusLabel, publishModeLabel, readinessLevelLabel } from '@/constants/copywriting';
import { formatUserErrorMessage } from '@/constants/errorMessages';
import { layoutTokens } from '@/constants/layoutTokens';
import MultiPlatformPublishCenter from '@/components/MultiPlatformPublishCenter';
import {
  localizeCollectWarningCode,
  localizePublishCheckItem,
  readinessStatusLabel,
} from '@/constants/productOperationLabels';
import {
  AI_LANGUAGE_OPTIONS,
  AI_TARGET_PLATFORM_OPTIONS,
  AI_TONE_OPTIONS,
  aiPromptCodeLabel,
  aiTaskTypeLabel,
  aiTextProviderLabel,
} from '@/constants/aiPrompts';
import { platformDisplayLabel } from '@/constants/platformLabels';
import { getProductReadinessAction } from '@/constants/productReadinessActions';
import { EditableProTable, ModalForm, ProForm, ProFormDigit, ProFormSelect, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
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
  Popover,
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
  Progress,
  Upload,
  Table,
  message,
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
  ArrowLeftOutlined,
  CheckCircleOutlined,
  FileTextOutlined,
  UndoOutlined,
  EditOutlined,
  MoreOutlined,
  TranslationOutlined,
} from '@ant-design/icons';
import { ProductCollectQualityAlert } from '@/components/ProductCollectQualityAlert';
import { isPinduoduoSource } from '@/utils/pinduoduoCollectAlerts';
import { isTaobaoTmallSource } from '@/utils/taobaoTmallCollectAlerts';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
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
  fetchProductOperationProgress,
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
  type ProductOperationProgress,
  type ProductDetail,
  type DouyinDraftImage,
  type DouyinDraftAttribute,
  type DouyinDraftMapping,
  type DouyinMappingIssue,
  type ProductImageRow,
  type ProductSKURow,
  undoAiDescription,
  undoProductAITitle,
} from '@/services/products';
import { Link } from '@umijs/renderer-react';
import { history } from '@umijs/max';
import {
  listProductPublications,
  publishProduct,
  listDouyinPublishTasks,
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
  createInventoryIdempotencyKey,
  createInventorySyncBatch,
  listInventoryWarehouses,
  listProductPublicationSkus,
  listSkuWarehouseBalances,
  previewBatchStockSettings,
  querySkuInventoryLogs,
  syncPublicationSkuInventory,
  type InventoryChangeLogRow,
  type InventoryWarehouse,
  type PublicationSkuListingRow,
  type WarehouseBalance,
} from '@/services/inventory';
import InventorySyncDisabledBanner from '@/components/inventory/InventorySyncDisabledBanner';
import { usePermission } from '@/hooks/usePermission';
import { PERMISSIONS } from '@/utils/permission';
import {
  confirmApplyAiText,
  confirmInventoryManualAdjust,
  confirmInventorySync,
  confirmPlatformPublishConfigSave,
  confirmSkuManualBind,
  confirmSkuUnbind,
  confirmUndoAiText,
} from '@/constants/sensitiveActions';
import './index.less';

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
  mock: '模拟',
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
  const codes: string[] = [];
  const addList = (v: unknown) => {
    if (!Array.isArray(v)) return;
    for (const x of v) {
      if (typeof x === 'string' && x.trim()) codes.push(x.trim());
    }
  };
  addList(root.qualityWarnings);
  addList(root.warnings);
  addList(root.collectWarnings);
  if (raw && typeof raw === 'object') {
    const inner = raw as Record<string, unknown>;
    addList(inner.qualityWarnings);
    addList(inner.warnings);
    addList(inner.collectWarnings);
  }
  const seen = new Set<string>();
  return codes
    .map((c) => localizeCollectWarningCode(c))
    .filter((line) => {
      const k = line.toLowerCase();
      if (seen.has(k)) return false;
      seen.add(k);
      return true;
    });
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
    hints.push('该规格存在多个候选抖店规格，匹配结果不明确，请人工确认绑定后再同步库存。');
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

function skuTextCell(value?: string | null, fallback = '—'): ReactNode {
  const text = String(value ?? '').trim();
  if (!text) return <Typography.Text type="secondary">{fallback}</Typography.Text>;
  return (
    <Tooltip title={text}>
      <Typography.Text className="product-draft-skus__text-cell">{text}</Typography.Text>
    </Tooltip>
  );
}

function skuPriceCell(value?: number | null): ReactNode {
  return typeof value === 'number' ? value.toFixed(2) : <Typography.Text type="secondary">—</Typography.Text>;
}

function skuAttrsCell(value?: string | null): ReactNode {
  const text = String(value ?? '').trim();
  if (!text) return <Typography.Text type="secondary">未填写</Typography.Text>;
  return (
    <Tooltip title={text}>
      <Typography.Paragraph className="product-draft-skus__attrs-cell">{text}</Typography.Paragraph>
    </Tooltip>
  );
}

const PRODUCT_STATUS_OPTIONS = Object.entries(PRODUCT_STATUS).map(([value, v]) => ({
  label: v.text,
  value,
}));

const IMAGE_TYPE_OPTIONS = [
  { label: '主图', value: 'main' },
  { label: '详情图', value: 'detail' },
  { label: '规格图', value: 'sku' },
];

function isAiTaskFailed(row?: AITaskRow | null): boolean {
  return String(row?.status || '').toLowerCase() === 'failed';
}

function aiTaskNextStep(row?: AITaskRow | null): string {
  const raw = String(row?.errorMessage || '').trim();
  const text = raw.toLowerCase();
  if (!row) return '暂无需要处理的失败任务。';
  if (/quota|credit|balance|billing|insufficient|limit|rate/.test(text)) {
    return '请检查 AI 设置中的模型额度、限流或计费状态，确认后重新生成。';
  }
  if (/timeout|network|connection|connect|econn|gateway|502|503|504/.test(text)) {
    return '请检查 AI 服务连接状态，稍后重新生成。';
  }
  if (/api key|apikey|unauthorized|401|403|permission|forbidden/.test(text)) {
    return '请检查 AI API Key、Base URL 和模型权限后重新生成。';
  }
  if (/parse|json|format|schema/.test(text)) {
    return '模型返回格式不符合预期，建议重新生成；若反复出现，请检查默认 Prompt 模板。';
  }
  return '请根据失败原因检查 AI 设置或商品内容，确认后重新生成。';
}

function aiTaskCostText(row: AITaskRow): string {
  const input = row.tokenInput ?? 0;
  const output = row.tokenOutput ?? 0;
  return `${input}/${output}`;
}

function aiTextPreview(text?: string | null, fallback = '暂无内容'): ReactNode {
  const value = String(text || '').trim();
  if (!value) return <Typography.Text type="secondary">{fallback}</Typography.Text>;
  return <Typography.Paragraph className="product-draft-ai__preview-text">{value}</Typography.Paragraph>;
}

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

function productImageUrl(row: ProductImageRow): string {
  return (row.publicUrl || row.originUrl || '').trim();
}

function isMainProductImage(row: ProductImageRow): boolean {
  return row.imageType === 'main';
}

function isDetailProductImage(row: ProductImageRow): boolean {
  return row.imageType === 'detail' || row.imageType === 'description';
}

function isSyncedProductImage(row: ProductImageRow): boolean {
  return !!String(row.objectKey || row.storageKey || '').trim();
}

const IMAGE_META_TAG_STYLE: CSSProperties = {
  margin: 0,
  fontSize: 12,
  lineHeight: '20px',
  padding: '0 6px',
  borderRadius: 4,
};

function ProductImagePreviewCell({ row }: { row: ProductImageRow }) {
  const [failed, setFailed] = useState(false);
  const url = productImageUrl(row);

  if (!url || failed) {
    return (
      <div className="product-draft-images__thumb product-draft-images__thumb--empty">
        <PictureOutlined />
        <Typography.Text type="secondary">{url ? '加载失败' : '无图片'}</Typography.Text>
      </div>
    );
  }

  return (
    <div className="product-draft-images__thumb-wrap" onClick={(event) => event.stopPropagation()}>
      <Image
        src={url}
        width={64}
        height={64}
        preview={{ src: url }}
        className="product-draft-images__thumb-image"
        onError={() => setFailed(true)}
      />
    </div>
  );
}

function ProductImageMetaTags({ row }: { row: ProductImageRow }) {
  const tags: ReactNode[] = [];
  if (isMainProductImage(row)) {
    tags.push(
      <Tag key="main" color="blue" bordered={false} style={IMAGE_META_TAG_STYLE}>
        主图
      </Tag>,
    );
  }
  if (isDetailProductImage(row)) {
    tags.push(
      <Tag key="detail" color="cyan" bordered={false} style={IMAGE_META_TAG_STYLE}>
        详情图
      </Tag>,
    );
  }
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
    <div className="product-draft-images__type-cell">
      <Typography.Text strong className="product-draft-images__type-title">
        {imageTypeLabel(String(row.imageType ?? ''))}
      </Typography.Text>
      <ProductImageMetaTags row={row} />
    </div>
  );
}

function ProductImageSourceCell({ row }: { row: ProductImageRow }) {
  const synced = isSyncedProductImage(row);
  const source = String(row.source || '').trim();
  const sourceLabel = source === 'ai' ? 'AI 处理图' : source === 'upload' ? '手动上传' : source === 'collect' ? '采集来源' : source || '来源未记录';
  const storageKey = String(row.objectKey || row.storageKey || '').trim();

  return (
    <div className="product-draft-images__source-cell">
      <Space size={[6, 4]} wrap>
        <Tag bordered={false}>{sourceLabel}</Tag>
        <Tag color={synced ? 'success' : 'default'} bordered={false}>
          {synced ? '已同步' : '未同步'}
        </Tag>
      </Space>
      {storageKey ? (
        <Typography.Text type="secondary" className="product-draft-images__source-key" title={storageKey}>
          {storageKey}
        </Typography.Text>
      ) : (
        <Typography.Text type="secondary" className="product-draft-images__source-key">
          暂无存储标识
        </Typography.Text>
      )}
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
  if (typeof r.stock !== 'number') return '';
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
  pricing: '价格',
  attribute: '商品参数',
};

function readinessCheckDisplay(c: ReadinessCheckItem) {
  const loc = localizePublishCheckItem(c);
  return (
    <>
      {loc.title}
      {loc.message && loc.message !== loc.title ? `：${loc.message}` : ''}
    </>
  );
}

function readinessStatusTag(r: ProductReadinessResult | null) {
  if (!r) return null;
  const statusText = r.statusLabel || readinessStatusLabel(r.status);
  if (r.status === 'blocked' || (r.errorCount ?? 0) > 0) {
    return <Tag color="red">{statusText || '暂不能继续'}</Tag>;
  }
  if (r.status === 'warning' || (r.warningCount ?? 0) > 0) {
    return <Tag color="orange">{statusText || '建议检查'}</Tag>;
  }
  return <Tag color="green">{statusText || '已准备好'}</Tag>;
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
      {readinessLevelTag(c.level)} {readinessCheckDisplay(c)}
      {c.technicalDetails?.rawCode ? (
        <TechnicalDetails label="技术详情">
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            内部编号：{String(c.technicalDetails.rawCode)}
          </Typography.Text>
        </TechnicalDetails>
      ) : null}
    </div>
  ));
}

const PRODUCT_DRAFT_TABS = new Set(['basic', 'ai', 'images', 'skus', 'inventory', 'readiness', 'publish']);
const PRODUCT_DRAFT_TAB_LABELS: Record<string, string> = {
  basic: '基础信息',
  ai: 'AI',
  images: '图片管理',
  skus: '商品规格',
  inventory: '库存',
  readiness: '发布检查',
  publish: '刊登',
};

function tabFromOperationUrl(raw?: string): string | null {
  if (!raw) return null;
  try {
    const url = new URL(raw, window.location.origin);
    const tab = url.searchParams.get('tab') || '';
    return PRODUCT_DRAFT_TABS.has(tab) ? tab : null;
  } catch {
    return null;
  }
}

function sectionFromOperationUrl(raw?: string): string | null {
  if (!raw) return null;
  try {
    const url = new URL(raw, window.location.origin);
    return (url.searchParams.get('section') || '').trim() || null;
  } catch {
    return null;
  }
}

function OperationProgressPanel({
  progress,
  loading,
  error,
  onReload,
  onAction,
}: {
  progress: ProductOperationProgress | null;
  loading: boolean;
  error?: string;
  onReload: () => void;
  onAction: (url?: string) => void;
}) {
  if (error && !progress) {
    return (
      <SectionCard
        title="商品运营进度"
        description="商品内容仍可以正常编辑。"
        className="product-draft-progress product-draft-progress--error"
        headerExtra={
          <Button icon={<ReloadOutlined />} onClick={onReload}>
            重新加载
          </Button>
        }
      >
        <Alert
          type="warning"
          showIcon
          message="商品运营进度暂时无法加载"
          description="商品内容仍可以正常编辑，请稍后重新加载进度。"
        />
      </SectionCard>
    );
  }

  if (!progress) {
    return (
      <SectionCard
        title="商品运营进度"
        description="根据商品内容、图片、价格和发布检查实时计算。"
        className="product-draft-progress product-draft-progress--loading"
      >
        <Spin spinning={loading}>
          <div className="product-draft-progress__loading">
            <Progress percent={0} showInfo={false} />
            <Typography.Text type="secondary">正在计算商品运营进度...</Typography.Text>
          </div>
        </Spin>
      </SectionCard>
    );
  }

  const blockerCount = progress.blockerCount ?? progress.blockers?.length ?? 0;
  const warningCount = progress.warningCount ?? progress.warnings?.length ?? 0;
  const priorityTone = blockerCount > 0 ? 'danger' : warningCount > 0 ? 'warning' : progress.publishReady ? 'ready' : 'default';

  return (
    <SectionCard
      title={
        <div className="product-draft-progress__title">
          <span>商品运营进度</span>
          <Tag color={blockerCount > 0 ? 'red' : warningCount > 0 ? 'orange' : progress.publishReady ? 'green' : 'blue'}>
            {blockerCount > 0 ? '存在阻断' : warningCount > 0 ? '建议检查' : progress.publishReady ? '可进入刊登' : '继续完善'}
          </Tag>
        </div>
      }
      description="根据商品内容、图片、价格和发布检查实时计算。"
      className={`product-draft-progress product-draft-progress--${priorityTone}`}
      headerExtra={
        <OperationToolbar>
          <Button type="text" icon={<ReloadOutlined />} onClick={onReload} loading={loading} aria-label="刷新商品运营进度">
            刷新进度
          </Button>
          <Button type="primary" onClick={() => onAction(progress.nextActionUrl)}>
            {progress.nextActionLabel || '继续处理'}
          </Button>
        </OperationToolbar>
      }
    >
      <Spin spinning={loading}>
        <div className="product-draft-progress__grid">
          <div className="product-draft-progress__meter">
            <div className="product-draft-progress__meter-head">
              <Typography.Text type="secondary">运营完成度</Typography.Text>
              <Typography.Text strong>{progress.completionPercent ?? 0}%</Typography.Text>
            </div>
            <Progress
              percent={progress.completionPercent ?? 0}
              status={progress.publishReady ? 'success' : 'active'}
              showInfo={false}
            />
          </div>
          <div className="product-draft-progress__summary" aria-label="商品运营状态概览">
            <div className="product-draft-progress__metric">
              <span>下一步</span>
              <strong className="product-draft-progress__next-step">{progress.nextActionLabel || progress.currentStepLabel || '继续处理'}</strong>
            </div>
            <div className="product-draft-progress__metric">
              <span>阻断问题</span>
              <strong>{blockerCount}</strong>
            </div>
            <div className="product-draft-progress__metric">
              <span>建议检查</span>
              <strong>{warningCount}</strong>
            </div>
          </div>
        </div>
        {blockerCount > 0 || warningCount > 0 ? (
          <Typography.Text type="secondary" className="product-draft-progress__hint">
            具体问题和处理入口已归入「发布检查」，避免在各模块重复展示。
          </Typography.Text>
        ) : null}
      </Spin>
    </SectionCard>
  );
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
  if (st === 'unmatched') return '未匹配到抖店规格，请到刊登 Tab 管理绑定后再同步库存。';
  if (st === 'ambiguous') return '找到多个可能的抖店规格，请到刊登 Tab 确认绑定。';
  if (st === 'failed') return '校准失败，请到刊登 Tab 重新校准或管理绑定。';
  return '尚未校准，请先到刊登 Tab 执行校准或管理绑定。';
}

function douyinSkuSyncBlocked(row: PublicationSkuListingRow): boolean {
  const isDouyin = (row.platform || '').toLowerCase() === 'douyin_shop';
  if (!isDouyin) return false;
  const st = String(row.bindStatus || '').toLowerCase();
  const hasBinding = Boolean((row.externalProductId || '').trim()) && Boolean((row.externalSkuId || '').trim());
  if (!hasBinding) return true;
  return st === 'ambiguous' || st === 'unmatched' || st === 'failed';
}

function platformSkuValue(value?: string | null, fallback = '—'): ReactNode {
  const text = String(value || fallback).trim() || fallback;
  return (
    <Tooltip title={text}>
      <Typography.Text className="product-draft-platform-sku__id">{text}</Typography.Text>
    </Tooltip>
  );
}

function platformSkuCandidateStatusTag(row: DouyinPlatformSkuCandidate, currentPublicationSkuId?: string) {
  if (row.boundToPublicationSkuId && row.boundToPublicationSkuId !== currentPublicationSkuId) {
    return <Tag color="orange">已绑定其他本地规格</Tag>;
  }
  if (row.boundToPublicationSkuId) return <Tag color="blue">已绑定本地规格</Tag>;
  return <Tag>未绑定</Tag>;
}

function douyinImageStatusTag(img: DouyinDraftImage) {
  const st = img.uploadStatus || (img.platformImageId ? 'uploaded' : img.needSync ? 'pending' : 'pending');
  if (st === 'uploaded') return <Tag color="green">已上传抖店</Tag>;
  if (st === 'failed') return <Tag color="red">上传失败</Tag>;
  if (st === 'processing') return <Tag color="blue">上传中</Tag>;
  if (img.needSync) return <Tag color="orange">待同步到存储</Tag>;
  return <Tag color="orange">待上传</Tag>;
}

function douyinStorageStatusTag(img: DouyinDraftImage) {
  return img.storageKey || img.objectKey || img.storageUrl || img.publicUrl ? <Tag color="green">存储已就绪</Tag> : <Tag color="orange">需先同步到存储</Tag>;
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

export default function ProductDraftDetailPage() {
  const id = decodeURIComponent(window.location.pathname.split('/').filter(Boolean).pop() ?? '');
  const { readonly, can } = usePermission();
  const publishWriteDisabled = !can(PERMISSIONS.PUBLISH_CREATE_DRAFT);
  const skuBindDisabled = !can(PERMISSIONS.SKU_BIND);
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<ProductDetail | null>(null);
  const [err, setErr] = useState<string>();
  const [aiOpen, setAiOpen] = useState(false);
  const [aiBusy, setAiBusy] = useState(false);
  const [aiResult, setAiResult] = useState<OptimizeTitleResult | null>(null);
  const [aiPreparedTitle, setAiPreparedTitle] = useState('');
  const [aiTasks, setAiTasks] = useState<AITaskRow[]>([]);
  const [aiForm] = Form.useForm();
  const [descOpen, setDescOpen] = useState(false);
  const [descBusy, setDescBusy] = useState(false);
  const [descResult, setDescResult] = useState<GenerateDescriptionResult | null>(null);
  const [descPreparedText, setDescPreparedText] = useState('');
  const [descForm] = Form.useForm();
  const [operationProgress, setOperationProgress] = useState<ProductOperationProgress | null>(null);
  const [operationProgressLoading, setOperationProgressLoading] = useState(false);
  const [operationProgressError, setOperationProgressError] = useState('');
  const [pendingSection, setPendingSection] = useState<string>('');
  const [skuRows, setSkuRows] = useState<SKUEditable[]>([]);
  const [skuEditableKeys, setSkuEditableKeys] = useState<Key[]>([]);
  const [imgModalOpen, setImgModalOpen] = useState(false);
  const [imgEdit, setImgEdit] = useState<ProductImageRow | null>(null);
  const [imgBusy, setImgBusy] = useState(false);
  const [imageSyncingScope, setImageSyncingScope] = useState<'' | 'order' | 'all' | 'main' | 'detail'>('');
  const [imageSyncError, setImageSyncError] = useState('');
  const [lastUpload, setLastUpload] = useState<{ id: string; url: string; objectKey: string } | null>(null);
  const [createImageOpen, setCreateImageOpen] = useState(false);
  const [createImagePrefill, setCreateImagePrefill] = useState<CreateImageTaskPrefill>({});
  const [translateImageOpen, setTranslateImageOpen] = useState(false);
  const [translateImagePrefill, setTranslateImagePrefill] = useState<TranslateImageTextPrefill>({});
  const [translateSourceImage, setTranslateSourceImage] = useState<ProductImageRow | undefined>();

  const [pubRows, setPubRows] = useState<ProductPublicationRow[]>([]);
  const [pubCtxLoading, setPubCtxLoading] = useState(false);
  const [pubCtxError, setPubCtxError] = useState('');
  const [platformsMeta, setPlatformsMeta] = useState<PlatformProviderMeta[]>([]);
  const [shopsList, setShopsList] = useState<ShopListRow[]>([]);
  const [publishForm] = Form.useForm();
  const [douyinForm] = Form.useForm();
  const [douyinMappingForm] = Form.useForm();
  const douyinFormConnectedRef = useRef(false);
  const douyinMappingFormConnectedRef = useRef(false);
  const [publishSubmitting, setPublishSubmitting] = useState(false);
  const [douyinSaving, setDouyinSaving] = useState(false);
  const [douyinConfirmingAction, setDouyinConfirmingActionState] = useState<'' | 'config' | 'mapping'>('');
  const douyinConfirmingActionRef = useRef<'' | 'config' | 'mapping'>('');
  const setDouyinConfirmingAction = useCallback((next: '' | 'config' | 'mapping') => {
    douyinConfirmingActionRef.current = next;
    setDouyinConfirmingActionState(next);
  }, []);
  const [douyinMapping, setDouyinMapping] = useState<DouyinDraftMapping | null>(null);
  const [douyinMappingLoading, setDouyinMappingLoading] = useState(false);
  const [douyinMappingSaving, setDouyinMappingSaving] = useState(false);
  const [douyinMappingValidating, setDouyinMappingValidating] = useState(false);
  const [douyinImageUploading, setDouyinImageUploading] = useState(false);
  const [douyinImageRetryingKey, setDouyinImageRetryingKey] = useState('');
  const [douyinPublishTasks, setDouyinPublishTasks] = useState<ProductPublishTaskDTO[]>([]);
  const [douyinPublishTasksLoading, setDouyinPublishTasksLoading] = useState(false);
  const [douyinPublishTasksError, setDouyinPublishTasksError] = useState('');
  const [douyinSkuBinding, setDouyinSkuBinding] = useState<DouyinSkuBindingSummary | null>(null);
  const [douyinSkuBindingLoading, setDouyinSkuBindingLoading] = useState(false);
  const [douyinSkuBindingError, setDouyinSkuBindingError] = useState('');
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
  const [readinessError, setReadinessError] = useState('');
  const [publishReadiness, setPublishReadiness] = useState<ProductReadinessResult | null>(null);
  const [publishReadinessLoading, setPublishReadinessLoading] = useState(false);

  const [pubSkuRows, setPubSkuRows] = useState<PublicationSkuListingRow[]>([]);
  const [pubSkuLoading, setPubSkuLoading] = useState(false);
  const [pubSkuBulkPlatformFilter, setPubSkuBulkPlatformFilter] = useState('');
  const [pubSkuSelectedKeys, setPubSkuSelectedKeys] = useState<string[]>([]);
  const [adjustOpen, setAdjustOpen] = useState(false);
  const [adjustTarget, setAdjustTarget] = useState<ProductSKURow | null>(null);
  const [adjustForm] = Form.useForm<{ warehouseId: string; stock: number; reason?: string; remark?: string }>();
  const [invAdjustSubmitting, setInvAdjustSubmitting] = useState(false);
  const [invAdjustLoading, setInvAdjustLoading] = useState(false);
  const [invAdjustWarehouses, setInvAdjustWarehouses] = useState<InventoryWarehouse[]>([]);
  const [invAdjustBalances, setInvAdjustBalances] = useState<WarehouseBalance[]>([]);
  const [invAdjustIdempotencyKey, setInvAdjustIdempotencyKey] = useState('');
  const invAdjustLoadSequence = useRef(0);
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

  const collectedAttrRows = useMemo(
    () => Object.entries(collectedAttrs).map(([key, value]) => ({ key, value })),
    [collectedAttrs],
  );

  const hasSourceCollectQualityPanel = useMemo(
    () => isPinduoduoProduct(data) || isTaobaoTmallProduct(data),
    [data],
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

  const missingBasicFields = useMemo(() => {
    if (!data) return [];
    return [
      ['来源平台', data.source],
      ['来源链接', data.sourceUrl],
      ['原始标题', data.originalTitle],
      ['主标题', data.title],
      ['主描述', data.description],
      ['币种', data.currency],
      ['状态', data.status],
    ]
      .filter(([, value]) => !String(value ?? '').trim())
      .map(([label]) => String(label));
  }, [data]);

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

  const reloadOperationProgress = useCallback(async () => {
    if (!id) return;
    setOperationProgressLoading(true);
    setOperationProgressError('');
    try {
      const progress = await fetchProductOperationProgress(id);
      setOperationProgress(progress);
    } catch (e: unknown) {
      setOperationProgressError((e as Error)?.message || 'operation progress load failed');
    } finally {
      setOperationProgressLoading(false);
    }
  }, [id]);

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
        setSkuEditableKeys([]);
    await reloadOperationProgress();
  }, [id, reloadOperationProgress]);

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
    if (!id) return [];
    setPubCtxLoading(true);
    setPubCtxError('');
    try {
      const [pubs, prov, shops, douyinCfg, douyinCats] = await Promise.all([
        listProductPublications(id),
        queryPlatformProviders(),
        queryShops({ page: 1, pageSize: 500, authStatus: 'authorized' }),
        getProductPlatformPublishConfig(id, 'douyin_shop').catch(() => undefined),
        queryDouyinCategories({ onlyLeaf: true }).catch(() => undefined),
      ]);
      const rows = Array.isArray(pubs.list) ? pubs.list : [];
      setPubRows(rows);
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
        if (douyinCfg.mapping) {
          setDouyinMapping(douyinCfg.mapping);
        } else {
          const mapped = await getDouyinDraftMapping(id).catch(() => undefined);
          setDouyinMapping(mapped ?? null);
        }
        if (douyinCfg.categoryId) {
          const ar = await queryDouyinCategoryAttributes(douyinCfg.categoryId).catch(() => undefined);
          setDouyinAttrs(ar?.list ?? []);
        }
      }
      return rows;
    } catch (e: unknown) {
      setPubRows([]);
      setPubCtxError((e as Error)?.message || '刊登上下文加载失败');
      return [];
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
    const attrs = (douyinMapping?.attributes ?? douyinAttrs.map((a): DouyinDraftAttribute => ({
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

  const runBuildDouyinMapping = useCallback(async () => {
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

  const handleBuildDouyinMapping = useCallback(() => {
    if (publishWriteDisabled) {
      message.error('当前账号无刊登草稿写权限');
      return;
    }
    if (douyinConfirmingActionRef.current || douyinMappingLoading) return;
    setDouyinConfirmingAction('mapping');
    window.setTimeout(() => douyinConfirmingActionRef.current === 'mapping' ? setDouyinConfirmingAction('') : undefined, 800);
    confirmPlatformPublishConfigSave(async () => {
      try {
        await runBuildDouyinMapping();
      } finally {
        setDouyinConfirmingAction('');
      }
    });
  }, [douyinMappingLoading, publishWriteDisabled, runBuildDouyinMapping, setDouyinConfirmingAction]);

  const handleSaveDouyinMapping = useCallback(async () => {
    if (publishWriteDisabled) {
      message.error('当前账号无刊登草稿写权限');
      return;
    }
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
  }, [currentDouyinMapping, douyinMapping, douyinMappingForm, id, publishWriteDisabled]);

  const handleValidateDouyinMapping = useCallback(async () => {
    if (publishWriteDisabled) {
      message.error('当前账号无刊登草稿写权限');
      return;
    }
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
      setReadinessShopId(String(douyinConfig.shopId || ''));
    } catch (e: unknown) {
      message.error((e as Error)?.message || '校验抖店刊登草稿失败');
    } finally {
      setDouyinMappingValidating(false);
    }
  }, [currentDouyinMapping, douyinConfig.shopId, douyinMapping, id, publishWriteDisabled]);

  const handleUploadDouyinImages = useCallback(async (force = false) => {
    if (publishWriteDisabled) {
      message.error('当前账号无刊登草稿写权限');
      return;
    }
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
  }, [douyinMapping, id, publishWriteDisabled]);

  const handleRetryDouyinImage = useCallback(async (imageKey: string) => {
    if (publishWriteDisabled) {
      message.error('当前账号无刊登草稿写权限');
      return;
    }
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
  }, [id, publishWriteDisabled]);

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

  const platformSkuMappingSummary = useMemo(() => {
    const rows = filteredPubSkuRowsForBulk;
    const platforms = Array.from(new Set(rows.map((r) => platformDisplayName(r.platform)).filter(Boolean)));
    const shops = Array.from(new Set(rows.map((r) => r.shopName || r.shopId).filter(Boolean)));
    const withProduct = rows.filter((r) => String(r.externalProductId ?? '').trim()).length;
    const withSku = rows.filter((r) => String(r.externalSkuId ?? '').trim()).length;
    const blocked = rows.filter((r) => douyinSkuSyncBlocked(r) || !inventorySyncRunnable(r.inventorySyncCapability)).length;
    return {
      total: rows.length,
      platforms,
      shops,
      withProduct,
      withSku,
      blocked,
    };
  }, [filteredPubSkuRowsForBulk]);

  const localInventoryRows = useMemo(
    () => (data?.skus ?? []).filter((s) => !String(s.id).startsWith('new_')),
    [data?.skus],
  );

  const localInventorySummary = useMemo(() => {
    const rows = localInventoryRows;
    return {
      total: rows.length,
      warningSet: rows.filter((s) => typeof s.warningStock === 'number' || typeof s.safetyStock === 'number').length,
      low: rows.filter((s) => ['low_stock', 'below_safety_stock', 'out_of_stock'].includes(effectiveStockStatus(s))).length,
      missingStock: rows.filter((s) => typeof s.stock !== 'number').length,
    };
  }, [localInventoryRows]);

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
        if (!cancelled) {
          try {
            const progress = await fetchProductOperationProgress(id);
            if (!cancelled) {
              setOperationProgress(progress);
              setOperationProgressError('');
            }
          } catch (e: unknown) {
            if (!cancelled) setOperationProgressError((e as Error)?.message || 'operation progress load failed');
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
      const tab = q.get('tab') || '';
      const section = (q.get('section') || '').trim();
      if (PRODUCT_DRAFT_TABS.has(tab)) {
        setDraftTabKey(tab);
      } else if (tab) {
        setDraftTabKey('basic');
      }
      setPendingSection(section);
      if (tab === 'inventory') {
        void reloadPublicationSkus();
      }
    } catch {
      /* noop */
    }
  }, [id, reloadPublicationSkus]);

  const scrollToSection = useCallback((section?: string) => {
    const target = (section || '').trim();
    if (!target) return false;
    const el =
      document.getElementById(target) ||
      ({
        title: document.querySelector('[id="title"]'),
        description: document.querySelector('textarea[id$="description"], [data-name="description"]'),
        'collect-review': document.getElementById('collect-review'),
        attributes: document.getElementById('attributes'),
        'image-list': document.getElementById('image-list') || document.querySelector('.ant-tabs-tabpane-active .ant-pro-table'),
        pricing:
          document.getElementById('pricing') ||
          document.querySelector('.ant-tabs-tabpane-active .ant-btn-primary, .ant-tabs-tabpane-active .ant-table-wrapper'),
        'local-skus':
          document.getElementById('local-skus') ||
          document.querySelector('.ant-tabs-tabpane-active .ant-alert, .ant-tabs-tabpane-active .ant-table-wrapper'),
        'publish-check':
          document.getElementById('publish-check') || document.querySelector('.ant-tabs-tabpane-active .ant-card'),
        'publish-config':
          document.getElementById('publish-config') || document.querySelector('.ant-tabs-tabpane-active .ant-card'),
        'douyin-sku-bindings':
          document.getElementById('douyin-sku-bindings') || document.querySelector('.product-draft-douyin-bind__card'),
      } as Record<string, Element | null | undefined>)[target] ||
      null;
    if (!el) return false;
    el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    return true;
  }, []);

  const openDraftLocation = useCallback(
    (tab: string, section?: string) => {
      const safeTab = PRODUCT_DRAFT_TABS.has(tab) ? tab : 'basic';
      const safeSection = (section || '').trim();
      setDraftTabKey(safeTab);
      setPendingSection(safeSection);
      if (safeTab === 'inventory') void reloadPublicationSkus();
      const q = new URLSearchParams(window.location.search);
      q.set('tab', safeTab);
      if (safeSection) {
        q.set('section', safeSection);
      } else {
        q.delete('section');
      }
      window.history.replaceState(null, '', `${window.location.pathname}?${q.toString()}`);
      if (safeTab === draftTabKey) {
        window.requestAnimationFrame(() => {
          void scrollToSection(safeSection);
        });
      }
    },
    [draftTabKey, reloadPublicationSkus, scrollToSection],
  );

  const openOperationAction = useCallback(
    (url?: string) => {
      const tab = tabFromOperationUrl(url) || 'basic';
      const section = sectionFromOperationUrl(url) || undefined;
      openDraftLocation(tab, section);
    },
    [openDraftLocation],
  );

  useEffect(() => {
    if (!pendingSection) return;
    const timer = window.setTimeout(() => {
      if (scrollToSection(pendingSection)) {
        setPendingSection('');
      }
    }, 60);
    return () => window.clearTimeout(timer);
  }, [draftTabKey, pendingSection, scrollToSection]);

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

  const aiImageSource = useMemo(
    () => sortedImages.find((image) => (image.publicUrl || image.originUrl || '').trim()),
    [sortedImages],
  );

  const imageOverview = useMemo(() => {
    const rows = data?.images ?? [];
    return {
      total: rows.length,
      main: rows.filter(isMainProductImage).length,
      detail: rows.filter(isDetailProductImage).length,
      best: rows.filter((img) => !!img.isBestMain).length,
      synced: rows.filter(isSyncedProductImage).length,
    };
  }, [data?.images]);

  const handleReorderProductImages = useCallback(async () => {
    setImageSyncError('');
    setImageSyncingScope('order');
    try {
      const ordered = [...sortedImages].sort((a, b) => (a.sortOrder ?? 0) - (b.sortOrder ?? 0));
      await reorderProductImages(id, { imageIds: ordered.map((i) => i.id) });
      message.success('已同步');
      await reloadDetail();
    } catch (e: unknown) {
      const msg = (e as Error)?.message || '排序失败';
      setImageSyncError(msg);
      message.error(msg);
    } finally {
      setImageSyncingScope('');
    }
  }, [id, reloadDetail, sortedImages]);

  const handleSyncProductImages = useCallback(
    async (scope: 'all' | 'main' | 'detail') => {
      setImageSyncError('');
      setImageSyncingScope(scope);
      try {
        const res = await syncProductImages(id, { scope });
        if (scope === 'main') {
          message.success(`已同步 ${res.synced} 张主图`);
        } else if (scope === 'detail') {
          message.success(`已同步 ${res.synced} 张详情图`);
        } else {
          message.success(`已同步 ${res.synced} 张图片到平台存储`);
        }
        await reloadDetail();
      } catch (e: unknown) {
        const msg = (e as Error)?.message || '同步失败';
        setImageSyncError(msg);
        message.error(msg);
      } finally {
        setImageSyncingScope('');
      }
    },
    [id, reloadDetail],
  );

  const eligibleShopsForPublish = useMemo(() => {
    return shopsList.filter((s) => {
	  if ((s.platform || '').toLowerCase() === 'douyin_shop') return false;
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
    setDouyinPublishTasksError('');
    try {
      const res = await listDouyinPublishTasks(id, { page: 1, pageSize: 10 });
      setDouyinPublishTasks(res.list ?? []);
    } catch (e: unknown) {
      setDouyinPublishTasks([]);
      setDouyinPublishTasksError((e as Error)?.message || '抖店刊登任务加载失败');
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

  const reloadDouyinSkuBindingsForPublication = useCallback(async (publicationId?: string) => {
    if (!publicationId) {
      setDouyinSkuBinding(null);
      setDouyinSkuBindingError('');
      return;
    }
    setDouyinSkuBindingLoading(true);
    setDouyinSkuBindingError('');
    try {
      const res = await getDouyinSkuBindings(publicationId);
      setDouyinSkuBinding(res);
    } catch (e: unknown) {
      setDouyinSkuBinding(null);
      setDouyinSkuBindingError((e as Error)?.message || '抖店规格绑定加载失败');
    } finally {
      setDouyinSkuBindingLoading(false);
    }
  }, []);

  const reloadDouyinSkuBindings = useCallback(async () => {
    await reloadDouyinSkuBindingsForPublication(douyinPublication?.id);
  }, [douyinPublication?.id, reloadDouyinSkuBindingsForPublication]);

  const handleSyncDouyinSkuBindings = useCallback(async () => {
    if (skuBindDisabled) {
      message.error('当前账号无规格绑定权限');
      return;
    }
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
  }, [douyinPublication?.id, reloadPublicationSkus, skuBindDisabled]);

  const douyinCreateDraftDisabled = useMemo(() => {
    if (publishWriteDisabled) return true;
    if (!douyinMapping || (douyinMapping.errors?.length ?? 0) > 0) return true;
    if (!douyinConfig.shopId || !douyinConfig.categoryId) return true;
    if (!douyinShops.some((s) => s.id === douyinConfig.shopId)) return true;
    const mainUploaded = (douyinMapping.mainImages ?? []).some((img) => img.uploadStatus === 'uploaded');
    if (!mainUploaded) return true;
    if (publishReadiness && !publishReadiness.canPublish) return true;
    return false;
  }, [douyinConfig.categoryId, douyinConfig.shopId, douyinMapping, douyinShops, publishReadiness, publishWriteDisabled]);

  const handleOpenDouyinOperationReview = useCallback(() => {
    const shopId = String(douyinConfig.shopId || '').trim();
    if (!shopId) {
      message.error('请选择抖店店铺');
      return;
    }
	const query = new URLSearchParams({ create: 'production', productId: id, shopId });
	history.push(`/ops/task-center/operation-tasks?${query.toString()}`);
  }, [douyinConfig.shopId, id]);

  const shopsForReadinessPlat = useMemo(() => {
    const p = readinessPlat.trim().toLowerCase();
    return shopsList.filter((s) => (s.platform || '').toLowerCase() === p && s.authStatus === 'authorized');
  }, [shopsList, readinessPlat]);

  const runReadinessForTab = useCallback(async () => {
    if (!id) return;
    setReadinessLoading(true);
    setReadinessError('');
    try {
      const r = await getProductReadiness(id, {
        platform: readinessPlat,
        shopId: readinessShopId || undefined,
        mode: 'draft',
      });
      setReadinessResult(r);
    } catch (e: unknown) {
      const msg = (e as Error)?.message || '检查失败';
      setReadinessResult(null);
      setReadinessError(msg);
      message.error(msg);
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
    if (draftTabKey !== 'publish') return;
    if (douyinFormConnectedRef.current) {
      douyinForm.setFieldsValue({
        shopId: douyinConfig.shopId,
        categoryId: douyinConfig.categoryId,
        platformAttributes: douyinConfig.platformAttributes ?? {},
      });
    }
    if (douyinMapping && douyinMappingFormConnectedRef.current) {
      douyinMappingForm.setFieldsValue({
        title: douyinMapping.title,
        description: douyinMapping.description,
      });
    }
  }, [douyinConfig, douyinForm, douyinMapping, douyinMappingForm, draftTabKey]);

  useEffect(() => {
    if (draftTabKey !== 'publish' || !id) return;
    void reloadDouyinPublishTasks();
    void reloadDouyinSkuBindings();
  }, [draftTabKey, id, reloadDouyinPublishTasks, reloadDouyinSkuBindings]);

  const progressBlockerCount = operationProgress?.blockerCount ?? operationProgress?.blockers?.length ?? 0;
  const progressWarningCount = operationProgress?.warningCount ?? operationProgress?.warnings?.length ?? 0;
  const readinessChecks = readinessResult?.checks ?? [];
  const readinessErrorItems = readinessChecks.filter((c) => (c.level || '').toLowerCase() === 'error');
  const readinessWarningItems = readinessChecks.filter((c) => (c.level || '').toLowerCase() === 'warning');
  const readinessSuggestionItems = readinessChecks.filter((c) => {
    const level = (c.level || '').toLowerCase();
    return level !== 'error' && level !== 'warning';
  });
  const readinessGroups = Array.from(new Set(readinessChecks.map((c) => c.group || 'other')));
  const readinessDefaultActiveKeys = readinessGroups.filter((g) =>
    readinessChecks.some((c) => (c.group || 'other') === g && (c.level || '').toLowerCase() === 'error'),
  );
  const productTitle = data?.title?.trim() || '商品详情';
  const productUpdatedAt = data?.updatedAt ? formatDateTime(data.updatedAt) : '';
  const currentSkuCount = skuRows.length;
  const platformPublishAvailableCount = platformsMeta.filter((x) => {
    const status = x.capabilityStatus?.product_publish;
    return status === 'available' || status === 'beta';
  }).length;
  const publishReadinessErrors = publishReadiness?.checks.filter((c) => (c.level || '').toLowerCase() === 'error') ?? [];
  const publishReadinessWarnings = publishReadiness?.checks.filter((c) => (c.level || '').toLowerCase() === 'warning') ?? [];
  const douyinRequiredAttrs = douyinAttrs.filter((x) => x.required);
  const douyinMissingRequiredAttrs = douyinRequiredAttrs.filter((x) => {
    const value = douyinConfig.platformAttributes?.[x.attrId] ?? douyinConfig.platformAttributes?.[x.name];
    return value == null || value === '' || (Array.isArray(value) && value.length === 0);
  });
  const douyinMainImages = douyinMapping?.mainImages ?? [];
  const douyinUploadedMainImages = douyinMainImages.filter((img) => img.uploadStatus === 'uploaded').length;
  const douyinDetailImages = douyinMapping?.detailImages ?? [];
  const douyinUploadedDetailImages = douyinDetailImages.filter((img) => img.uploadStatus === 'uploaded').length;
  const douyinMappingErrorCount = douyinMapping?.errors?.length ?? 0;
  const douyinMappingWarningCount = douyinMapping?.warnings?.length ?? 0;
  const selectedDouyinShop = douyinShops.find((shop) => shop.id === douyinConfig.shopId);
  const latestDouyinTask = douyinPublishTasks[0];
  const douyinSkuBindingReady = douyinSkuBinding
    ? (douyinSkuBinding.unmatched ?? 0) === 0 && (douyinSkuBinding.ambiguous ?? 0) === 0 && (douyinSkuBinding.failed ?? 0) === 0
    : false;
  const douyinDraftPrerequisiteItems = [
    {
      label: '店铺',
      status: douyinConfig.shopId ? '已选择' : pubCtxError ? '加载失败' : '未选择',
      tone: douyinConfig.shopId ? 'success' : pubCtxError ? 'error' : 'warning',
      detail: selectedDouyinShop?.shopName || douyinConfig.shopId || (douyinShops.length ? '请选择已授权抖店店铺' : '暂无已授权抖店店铺'),
    },
    {
      label: '配置',
      status: douyinConfig.shopId || douyinConfig.categoryId ? '有配置上下文' : '未配置',
      tone: douyinConfig.shopId || douyinConfig.categoryId ? 'processing' : 'warning',
      detail: '保存配置只写入店铺、类目和属性，不会创建抖店商品草稿。',
    },
    {
      label: '类目',
      status: douyinConfig.categoryId ? '已选择' : douyinCategoryLoading ? '加载中' : '未选择',
      tone: douyinConfig.categoryId ? 'success' : 'warning',
      detail: douyinConfig.categoryPath || selectedDouyinCategory?.path || '需要选择抖店叶子类目。',
    },
    {
      label: '类目属性',
      status: douyinAttrs.length ? `${douyinAttrs.length} 项已加载` : douyinConfig.categoryId ? '未加载或无属性' : '待选择类目',
      tone: douyinMissingRequiredAttrs.length ? 'warning' : douyinAttrs.length ? 'success' : 'default',
      detail: `必填 ${douyinRequiredAttrs.length} 项 · 未填写 ${douyinMissingRequiredAttrs.length} 项`,
    },
    {
      label: '属性映射',
      status: douyinMapping ? '已有映射' : '未生成',
      tone: douyinMapping ? 'processing' : 'warning',
      detail: douyinMapping ? `错误 ${douyinMappingErrorCount} · 警告 ${douyinMappingWarningCount}` : '生成映射不会代表已经通过校验。',
    },
    {
      label: '图片',
      status: douyinUploadedMainImages > 0 ? '主图已上传' : douyinMapping ? '待上传主图' : '待生成映射',
      tone: douyinUploadedMainImages > 0 ? 'success' : 'warning',
      detail: `主图 ${douyinUploadedMainImages}/${douyinMainImages.length} · 详情图 ${douyinUploadedDetailImages}/${douyinDetailImages.length}`,
    },
    {
      label: '规格绑定',
      status: douyinSkuBindingReady ? '已完成' : douyinSkuBinding ? '待确认' : douyinPublication?.id ? '未加载' : '待创建草稿',
      tone: douyinSkuBindingReady ? 'success' : douyinSkuBinding ? 'warning' : 'default',
      detail: `已绑定 ${douyinSkuBinding?.bound ?? '—'} · 未绑定 ${douyinSkuBinding?.unmatched ?? '—'} · 待确认 ${douyinSkuBinding?.ambiguous ?? '—'}`,
    },
    {
      label: '创建草稿',
      status: douyinCreateDraftDisabled ? '暂不可审核' : '可以进入审核',
      tone: douyinCreateDraftDisabled ? 'warning' : 'success',
      detail: '进入运营任务中心冻结内容并人工审核，不会直接创建平台草稿。',
    },
    {
      label: '最近任务',
      status: latestDouyinTask ? commonStatusLabel(latestDouyinTask.status) : douyinPublishTasksLoading ? '加载中' : '暂无任务',
      tone: latestDouyinTask?.status === 'failed' ? 'error' : latestDouyinTask ? 'processing' : 'default',
      detail: latestDouyinTask ? `${publishModeLabel(latestDouyinTask.publishMode)} · ${formatDateTime(latestDouyinTask.createdAt)}` : '创建草稿后会出现任务记录。',
    },
  ];
  const latestFailedAiTask = useMemo(
    () => aiTasks.find((task) => isAiTaskFailed(task)) ?? null,
    [aiTasks],
  );
  const originalTitleText = data?.title?.trim() || data?.originalTitle?.trim() || '';
  const appliedAiTitleText = data?.aiTitle?.trim() || '';
  const originalDescriptionText = data?.description?.trim() || '';
  const appliedAiDescriptionText = data?.aiDescription?.trim() || '';

  const renderDraftTabLabel = (
    label: string,
    options: {
      count?: number;
      tone?: 'default' | 'warning' | 'danger';
      icon?: ReactNode;
      hint?: string;
    } = {},
  ) => {
    const { count, tone = 'default', icon, hint } = options;
    return (
      <span className="product-draft-tabs__label">
        {icon ? <span className="product-draft-tabs__icon">{icon}</span> : null}
        <span className="product-draft-tabs__text">
          <span>{label}</span>
          {hint ? <span>{hint}</span> : null}
        </span>
        {typeof count === 'number' && count > 0 ? (
          <span className={`product-draft-tabs__count product-draft-tabs__count--${tone}`}>{count}</span>
        ) : null}
      </span>
    );
  };

  const tabLabels: Record<string, ReactNode> = {
    basic: renderDraftTabLabel('基础信息', { icon: <FileTextOutlined />, hint: '内容' }),
    ai: renderDraftTabLabel('AI', { count: aiTasks.length, icon: <RobotOutlined />, hint: '文案' }),
    images: renderDraftTabLabel('图片管理', { count: sortedImages.length, icon: <PictureOutlined />, hint: '素材' }),
    skus: renderDraftTabLabel('商品规格', { count: data?.skus?.length ?? 0, icon: <UnorderedListOutlined />, hint: '价格' }),
    inventory: renderDraftTabLabel('库存', { count: pubSkuRows.length, icon: <CloudUploadOutlined />, hint: '同步' }),
    readiness: renderDraftTabLabel('发布检查', {
      count: progressBlockerCount || progressWarningCount,
      tone: progressBlockerCount > 0 ? 'danger' : progressWarningCount > 0 ? 'warning' : 'default',
      icon: <CheckCircleOutlined />,
      hint: '问题',
    }),
    publish: renderDraftTabLabel('刊登', { count: pubRows.length, icon: <SyncOutlined />, hint: '平台' }),
  };

  const imageColumns: ProColumns<ProductImageRow>[] = useMemo(
    () => [
      {
        title: '预览',
        width: 92,
        render: (_, r) => <ProductImagePreviewCell row={r} />,
      },
      {
        title: '类型与标记',
        dataIndex: 'imageType',
        width: 160,
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
        title: '来源与状态',
        dataIndex: 'source',
        width: 180,
        render: (_, r) => <ProductImageSourceCell row={r} />,
      },
      {
        title: PRODUCT_IMAGE_URL_LABEL,
        width: 260,
        ellipsis: true,
        render: (_, r) => (
          productImageUrl(r) ? (
            <Typography.Link
              className="product-draft-images__url"
              href={productImageUrl(r)}
              target="_blank"
              rel="noreferrer"
              title={productImageUrl(r)}
            >
              {productImageUrl(r)}
            </Typography.Link>
          ) : (
            <Typography.Text type="secondary">未提供图片地址</Typography.Text>
          )
        ),
      },
      {
        title: '操作',
        width: 248,
        fixed: 'right',
        render: (_, r) => {
          const imageUrl = productImageUrl(r);
          const moreContent = (
            <div className="product-draft-images__action-menu" onClick={(event) => event.stopPropagation()}>
              <Typography.Text className="product-draft-images__action-group">AI 任务</Typography.Text>
              <Button type="text" size="small" block onClick={() => openTranslateImageText(r)}>
                翻译图片文字
              </Button>
              <Button type="text" size="small" block onClick={() => openQuickImageTask(r, 'remove_watermark')}>
                去除水印
              </Button>
              <Button type="text" size="small" block onClick={() => openQuickImageTask(r, 'remove_logo')}>
                去除 Logo
              </Button>
              <Button type="text" size="small" block onClick={() => openQuickImageTask(r, 'remove_background')}>
                移除背景
              </Button>
              <Button type="text" size="small" block onClick={() => openQuickImageTask(r, 'generate_marketing')}>
                生成营销图
              </Button>
              <Button type="text" size="small" block onClick={() => openQuickImageTask(r, 'score_image')}>
                图片评分
              </Button>
              <div className="product-draft-images__action-divider" />
              <Typography.Text className="product-draft-images__action-group">图片标记</Typography.Text>
              <Button
                type="text"
                size="small"
                block
                onClick={() =>
                  openCreateImageTask({
                    taskType: 'select_best_main',
                    imageSourceMode: 'product',
                    sourceImageId: r.id,
                    sourceImageUrl: imageUrl,
                  })
                }
              >
                设为最佳主图
              </Button>
              <Button
                type="text"
                size="small"
                block
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
              <div className="product-draft-images__action-divider" />
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
                <Button type="text" size="small" block danger icon={<DeleteOutlined />}>
                  删除图片
                </Button>
              </Popconfirm>
            </div>
          );

          return (
            <Space size={4} className="product-draft-images__actions">
              <Button type="link" size="small" icon={<EditOutlined />} onClick={() => setImgEdit(r)}>
                编辑
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
              <Popover
                trigger="click"
                placement="bottomRight"
                content={moreContent}
                overlayClassName="product-draft-images__action-popover"
                getPopupContainer={(triggerNode) =>
                  (triggerNode.closest('.tm-product-draft-detail') as HTMLElement) || document.body
                }
              >
                <Button type="link" size="small" icon={<MoreOutlined />}>
                  更多
                </Button>
              </Popover>
            </Space>
          );
        },
      },
    ],
    [id, reloadDetail, openQuickImageTask, openCreateImageTask, openTranslateImageText],
  );

  const skuColumns = useMemo<ProColumns<SKUEditable>[]>(
    () => [
      {
        title: '编码',
        dataIndex: 'skuCode',
        width: 160,
        ellipsis: true,
        className: 'product-draft-skus__code-col',
        formItemProps: { rules: [] },
        render: (_, record) => skuTextCell(record.skuCode, '未填写'),
      },
      {
        title: '名称',
        dataIndex: 'skuName',
        width: 200,
        ellipsis: true,
        className: 'product-draft-skus__name-col',
        formItemProps: { rules: [{ required: true }] },
        render: (_, record) => skuTextCell(record.skuName, '未填写'),
      },
      {
        title: '成本价',
        dataIndex: 'costPrice',
        width: 112,
        align: 'right' as const,
        className: 'product-draft-skus__number-col',
        valueType: 'digit' as const,
        fieldProps: { min: 0, precision: 2, className: 'product-draft-skus__number-input' },
        readonly: true,
        render: (_, record) => skuPriceCell(record.costPrice),
      },
      {
        title: '销售价',
        dataIndex: 'price',
        width: 112,
        align: 'right' as const,
        className: 'product-draft-skus__number-col',
        valueType: 'digit' as const,
        fieldProps: { min: 0, precision: 2, className: 'product-draft-skus__number-input' },
        render: (_, record) => skuPriceCell(record.price),
      },
      {
        title: '库存',
        dataIndex: 'stock',
        width: 96,
        align: 'right' as const,
        className: 'product-draft-skus__number-col',
        editable: false,
        render: (_, record) => (typeof record.stock === 'number' ? record.stock : <Typography.Text type="secondary">—</Typography.Text>),
      },
      {
        title: '图片 URL',
        dataIndex: 'imageUrl',
        width: 190,
        ellipsis: true,
        className: 'product-draft-skus__url-col',
        render: (_, record) => skuTextCell(record.imageUrl, '未填写'),
      },
      {
        title: '规格属性（高级）',
        dataIndex: 'attrsText',
        valueType: 'textarea' as const,
        width: 260,
        ellipsis: true,
        className: 'product-draft-skus__attrs-col',
        fieldProps: { rows: 2, className: 'product-draft-skus__attrs-input' },
        render: (_, record) => skuAttrsCell(record.attrsText),
      },
      {
        title: '操作',
        valueType: 'option' as const,
        width: 132,
        fixed: 'right' as const,
        className: 'product-draft-skus__action-col',
        render: (_: unknown, record: SKUEditable) => (
          <Space size={4} className="product-draft-skus__row-actions">
            {!record?.id?.startsWith('new_') ? (
              <Button
                type="link"
                size="small"
                icon={<EditOutlined />}
                className="product-draft-skus__edit-action"
                onClick={() => {
                  setSkuEditableKeys((keys) => Array.from(new Set([...keys, record.id])));
                }}
              >
                编辑
              </Button>
            ) : null}
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
              <Button type="link" danger size="small" className="product-draft-skus__danger-action">
                删除
              </Button>
            </Popconfirm>
          </Space>
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
      className="tm-product-draft-detail product-draft-detail-page"
      contentMaxWidth={layoutTokens.dashboardMaxWidth}
      title={
        <div className="product-draft-header">
          <div className="product-draft-header__locator">
            <Link to="/product/drafts" className="product-draft-header__back">
              <ArrowLeftOutlined />
              返回商品草稿
            </Link>
            <span className="product-draft-header__eyebrow">商品运营详情</span>
          </div>
          <div className="product-draft-header__main">
            <div className="product-draft-header__identity">
              <Typography.Title level={3} className="product-draft-header__title" title={productTitle}>
                {productTitle}
              </Typography.Title>
              {data ? <StatusTag status={data.status} /> : null}
            </div>
            {data ? (
              <div className="product-draft-header__meta" aria-label="商品摘要信息">
                <div className="product-draft-header__meta-item">
                  <span>来源平台</span>
                  <strong>{data.source ? platformDisplayName(data.source) : '未记录'}</strong>
                </div>
                <div className="product-draft-header__meta-item">
                  <span>更新时间</span>
                  <strong>{productUpdatedAt || '未记录'}</strong>
                </div>
                <div className="product-draft-header__meta-item product-draft-header__meta-item--id">
                  <span>草稿 ID</span>
                  <Typography.Text copyable={{ text: id }}>{id}</Typography.Text>
                </div>
                <div className={`product-draft-header__meta-item product-draft-header__meta-item--severity product-draft-header__meta-item--${progressBlockerCount > 0 ? 'danger' : progressWarningCount > 0 ? 'warning' : 'ready'}`}>
                  <span>发布检查</span>
                  <strong>
                    {progressBlockerCount > 0
                      ? `阻断 ${progressBlockerCount}`
                      : progressWarningCount > 0
                        ? `建议检查 ${progressWarningCount}`
                        : '暂无阻断'}
                  </strong>
                </div>
              </div>
            ) : null}
          </div>
        </div>
      }
      loading={loading}
      extra={
        data ? (
          <div className="product-draft-header__actions">
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
              <Button danger type="text" icon={<DeleteOutlined />}>
                删除草稿
              </Button>
            </Popconfirm>
          </div>
        ) : null
      }
    >
      {loading ? (
        <div className="product-draft-page-state">
          <Spin />
        </div>
      ) : err ? (
        <Alert
          type="error"
          showIcon
          message="商品详情加载失败"
          description={err}
          className="product-draft-page-state"
        />
      ) : data ? (
        <Space direction="vertical" className="product-draft-detail-shell" size={0}>
          <OperationProgressPanel
            progress={operationProgress}
            loading={operationProgressLoading}
            error={operationProgressError}
            onReload={() => void reloadOperationProgress()}
            onAction={openOperationAction}
          />
          <div className="product-draft-tabs-frame">
            <Tabs
              className="product-draft-tabs"
              activeKey={draftTabKey}
              onChange={(k) => {
                setDraftTabKey(k);
                setPendingSection('');
                const q = new URLSearchParams(window.location.search);
                q.set('tab', k);
                q.delete('section');
                window.history.replaceState(null, '', `${window.location.pathname}?${q.toString()}`);
                if (k === 'inventory') void reloadPublicationSkus();
              }}
              items={[
            {
              key: 'basic',
              label: tabLabels.basic,
              children: (
                <Space direction="vertical" className="product-draft-basic" size={0}>
                  <SectionCard
                    title="采集质量"
                    description="先看采集结果是否需要人工复核，再进入字段补充。"
                    className="product-draft-basic__section product-draft-basic__quality"
                  >
                    <div id="collect-review" />
                    <Space direction="vertical" className="product-draft-basic__stack" size="middle">
                      {hasSourceCollectQualityPanel ? (
                        <ProductCollectQualityAlert product={data} />
                      ) : (
                        <Alert
                          type="info"
                          showIcon
                          message="请人工核对采集结果"
                          description="当前来源未提供专项质量检查，请重点核对来源链接、标题、描述、图片和规格。"
                        />
                      )}
                      {showCustomIncompleteHint ? (
                        <Alert
                          type="info"
                          showIcon
                          message="自定义链接采集需要人工复核"
                          description="该商品来自自定义链接采集，部分字段可能需要人工补充。建议检查标题、价格、图片和规格后再发布。"
                        />
                      ) : null}
                      {collectQualityWarnings.length > 0 ? (
                        <Alert
                          type="warning"
                          showIcon
                          message="采集质量提示"
                          description={
                            <ul className="product-draft-basic__warning-list">
                              {collectQualityWarnings.map((w, index) => (
                                <li key={`${w}-${index}`}>{w}</li>
                              ))}
                            </ul>
                          }
                        />
                      ) : !showCustomIncompleteHint && !hasSourceCollectQualityPanel ? (
                        <Alert
                          type="success"
                          showIcon
                          message="未返回采集质量问题"
                          description="当前商品详情没有携带采集质量提示。仍建议在保存前检查核心字段是否完整。"
                        />
                      ) : null}
                    </Space>
                  </SectionCard>

                  <SectionCard
                    title="商品来源与采集信息"
                    description="这些信息用于追溯采集入口；来源链接和采集原始数据不在本页编辑。"
                    className="product-draft-basic__section product-draft-basic__source"
                  >
                    <div className="product-draft-basic__source-title-block">
                      <Typography.Text type="secondary" className="product-draft-basic__source-title-label">
                        来源商品标题
                      </Typography.Text>
                      {data.originalTitle ? (
                        <Tooltip title={data.originalTitle}>
                          <Typography.Text strong className="product-draft-basic__source-title">
                            {data.originalTitle}
                          </Typography.Text>
                        </Tooltip>
                      ) : (
                        <Typography.Text type="secondary">未记录</Typography.Text>
                      )}
                    </div>
                    <Descriptions
                      column={{ xs: 1, sm: 1, md: 2, xl: 3 }}
                      size="small"
                      className="product-draft-basic__descriptions"
                    >
                      <Descriptions.Item label="来源平台">
                        {data.source ? <Tag>{platformDisplayName(data.source)}</Tag> : <Typography.Text type="secondary">未记录</Typography.Text>}
                      </Descriptions.Item>
                      <Descriptions.Item label="币种（展示）">
                        {data.currency || <Typography.Text type="secondary">未记录</Typography.Text>}
                      </Descriptions.Item>
                      <Descriptions.Item label="本地商品 ID">
                        <Typography.Text type="secondary" copyable={{ text: data.id }}>
                          {data.id}
                        </Typography.Text>
                      </Descriptions.Item>
                      <Descriptions.Item label="采集 / 创建时间">
                        {data.createdAt ? formatDateTime(data.createdAt) : <Typography.Text type="secondary">未记录</Typography.Text>}
                      </Descriptions.Item>
                      <Descriptions.Item label="最近更新时间">
                        {data.updatedAt ? formatDateTime(data.updatedAt) : <Typography.Text type="secondary">未记录</Typography.Text>}
                      </Descriptions.Item>
                      <Descriptions.Item label="当前状态">
                        <StatusTag status={data.status} />
                      </Descriptions.Item>
                    </Descriptions>
                    <div className="product-draft-basic__source-link-row">
                      <Typography.Text type="secondary" className="product-draft-basic__source-link-label">
                        来源链接
                      </Typography.Text>
                      {data.sourceUrl ? (
                        <Typography.Link
                          className="product-draft-basic__source-url"
                          href={data.sourceUrl}
                          target="_blank"
                          rel="noreferrer"
                          title={data.sourceUrl}
                        >
                          {data.sourceUrl}
                        </Typography.Link>
                      ) : (
                        <Typography.Text type="secondary">未提供来源链接</Typography.Text>
                      )}
                    </div>
                    {!data.sourceUrl ? (
                      <Alert
                        className="product-draft-basic__inline-alert"
                        type="info"
                        showIcon
                        message="来源链接缺失"
                        description="无法直接回到原商品页面核对信息。请优先检查标题、描述、图片和规格是否完整。"
                      />
                    ) : null}
                  </SectionCard>

                  <SectionCard
                    title="商品核心信息"
                    description="保存会提交本表单当前字段；图片、SKU、库存和刊登配置仍在对应 Tab 处理。"
                    className="product-draft-basic__section product-draft-basic__form-section"
                  >
                    {missingBasicFields.length > 0 ? (
                      <Alert
                        className="product-draft-basic__inline-alert product-draft-basic__inline-alert--top"
                        type="warning"
                        showIcon
                        message="基础信息仍有缺失"
                        description={`建议补充：${missingBasicFields.join('、')}。`}
                      />
                    ) : (
                      <Alert
                        className="product-draft-basic__inline-alert product-draft-basic__inline-alert--top"
                        type="success"
                        showIcon
                        message="基础字段已具备主要内容"
                        description="保存前仍可继续调整标题、描述、币种和状态。"
                      />
                    )}
                    <ProForm
                      key={`basic-${data.id}-${data.updatedAt}`}
                      className="product-draft-basic__form"
                      submitter={{
                        searchConfig: { submitText: '保存基础信息' },
                        submitButtonProps: { type: 'primary' },
                        resetButtonProps: false,
                        render: (_, dom) => (
                          <div className="product-draft-basic__save-area">
                            <div className="product-draft-basic__save-copy">
                              <Typography.Text strong>保存范围</Typography.Text>
                              <Typography.Text type="secondary">
                                提交标题、原始标题、AI 标题、描述、AI 描述、币种和状态；不会自动执行发布检查、刊登或图片 / SKU / 库存操作。
                              </Typography.Text>
                            </div>
                            <div className="product-draft-basic__save-actions">{dom}</div>
                          </div>
                        ),
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
                      colProps={{ xs: 24, md: 12 }}
                    >
                      <div id="title" className="product-draft-basic__anchor" />
                      <div className="product-draft-basic__form-group-title">商品识别信息</div>
                      <ProFormText
                        name="title"
                        label="主标题"
                        rules={[{ required: true, message: '必填' }]}
                        colProps={{ xs: 24 }}
                        extra="发布和运营默认使用的商品标题。"
                      />
                      <ProFormTextArea
                        name="originalTitle"
                        label="原始标题"
                        fieldProps={{ rows: 2 }}
                        extra="采集时带回的原始标题，用于对照来源内容。"
                      />
                      <ProFormTextArea
                        name="aiTitle"
                        label="AI 标题"
                        fieldProps={{ rows: 2 }}
                        extra="AI 生成结果应用后会写入这里；本页保存只保存当前字段值。"
                      />
                      <div id="description" className="product-draft-basic__anchor" />
                      <div className="product-draft-basic__form-group-title">标题与描述</div>
                      <ProFormTextArea
                        name="description"
                        label="主描述"
                        fieldProps={{ rows: 5 }}
                        colProps={{ xs: 24, lg: 12 }}
                        extra="发布前建议保留完整卖点、材质、尺寸和注意事项。"
                      />
                      <ProFormTextArea
                        name="aiDescription"
                        label="AI 描述"
                        fieldProps={{ rows: 5 }}
                        colProps={{ xs: 24, lg: 12 }}
                        extra="AI 生成结果应用后会写入这里，可与主描述对照。"
                      />
                      <div className="product-draft-basic__form-group-title">流转属性</div>
                      <ProFormText name="currency" label="币种" extra="仅保存商品基础币种展示，不会重新计算 SKU 价格。" />
                      <ProFormSelect name="status" label="状态" options={PRODUCT_STATUS_OPTIONS} extra="状态值保持原有枚举，用于草稿流转。" />
                    </ProForm>
                  </SectionCard>

                  <div id="attributes" className="product-draft-basic__anchor" />
                  {collectedAttrRows.length > 0 ? (
                    <SectionCard
                      title="采集扩展属性"
                      description="从采集原始数据中提取，仅用于核对和后续平台映射参考。"
                      className="product-draft-basic__section product-draft-basic__attributes"
                    >
                      <Table
                        size="small"
                        pagination={collectedAttrRows.length > 12 ? { pageSize: 12, size: 'small' } : false}
                        rowKey="key"
                        dataSource={collectedAttrRows}
                        className="product-draft-basic__attr-table"
                        columns={[
                          {
                            title: '属性',
                            dataIndex: 'key',
                            width: 220,
                            render: (value) => (
                              <Typography.Text strong className="product-draft-basic__attr-key" title={String(value ?? '')}>
                                {String(value ?? '') || '—'}
                              </Typography.Text>
                            ),
                          },
                          {
                            title: '采集值',
                            dataIndex: 'value',
                            render: (value) => {
                              const text = String(value ?? '');
                              return (
                                <Tooltip title={text}>
                                  <Typography.Text className="product-draft-basic__attr-value">
                                    {text || '—'}
                                  </Typography.Text>
                                </Tooltip>
                              );
                            },
                          },
                        ]}
                      />
                    </SectionCard>
                  ) : null}
                </Space>
              ),
            },
            {
              key: 'ai',
              label: tabLabels.ai,
              children: (
                <Space direction="vertical" className="product-draft-ai" size="middle">
                  <SectionCard
                    title="AI 文案工作台"
                    description="先生成建议，再人工确认应用。生成不会保存到商品字段，应用才会写入 AI 标题或 AI 描述。"
                    className="product-draft-ai__workbench"
                    headerExtra={
                      <Space wrap className="product-draft-ai__actions">
                        <Button
                          icon={<ThunderboltOutlined />}
                          onClick={() => {
                            setAiResult(null);
                            setAiPreparedTitle('');
                            aiForm.resetFields();
                            aiForm.setFieldsValue({ language: 'en', platform: 'TikTok Shop', maxLength: 120 });
                            setAiOpen(true);
                          }}
                        >
                          生成标题建议
                        </Button>
                        <Button
                          icon={<FileTextOutlined />}
                          onClick={() => {
                            setDescResult(null);
                            setDescPreparedText('');
                            descForm.resetFields();
                            descForm.setFieldsValue({
                              language: 'en',
                              platform: 'TikTok Shop',
                              tone: 'professional',
                            });
                            setDescOpen(true);
                          }}
                        >
                          生成描述建议
                        </Button>
                      </Space>
                    }
                  >
                    <div className="product-draft-ai__status-strip" aria-label="AI 文案状态">
                      <div className="product-draft-ai__status-item">
                        <Typography.Text type="secondary">当前草稿</Typography.Text>
                        <Typography.Text strong>{originalTitleText || originalDescriptionText ? '已有人工内容' : '待补充内容'}</Typography.Text>
                      </div>
                      <div className="product-draft-ai__status-item">
                        <Typography.Text type="secondary">AI 字段</Typography.Text>
                        <Typography.Text strong>
                          {appliedAiTitleText || appliedAiDescriptionText ? '已有已应用内容' : '暂无已应用内容'}
                        </Typography.Text>
                      </div>
                      <div className="product-draft-ai__status-item">
                        <Typography.Text type="secondary">最近任务</Typography.Text>
                        <Typography.Text strong>
                          {aiTasks.length ? `${aiTasks.length} 条记录` : '暂无记录'}
                        </Typography.Text>
                      </div>
                    </div>
                    <Alert
                      className="product-draft-ai__action-note"
                      type="info"
                      showIcon
                      message="生成只是创建候选文案"
                      description="应用或撤销才会写入商品草稿；如果商品内容在生成后变化，系统会按现有冲突保护阻止静默覆盖。"
                    />
                    <div className="product-draft-ai__guide">
                      <div className="product-draft-ai__guide-item">
                        <RobotOutlined />
                        <div>
                          <Typography.Text strong>生成建议</Typography.Text>
                          <Typography.Text type="secondary">创建 AI 结果，可能消耗模型额度。</Typography.Text>
                        </div>
                      </div>
                      <div className="product-draft-ai__guide-item">
                        <CheckCircleOutlined />
                        <div>
                          <Typography.Text strong>应用文案</Typography.Text>
                          <Typography.Text type="secondary">人工确认后写入商品草稿的 AI 字段。</Typography.Text>
                        </div>
                      </div>
                      <div className="product-draft-ai__guide-item">
                        <UndoOutlined />
                        <div>
                          <Typography.Text strong>撤销应用</Typography.Text>
                          <Typography.Text type="secondary">恢复最近一次应用前的 AI 字段内容。</Typography.Text>
                        </div>
                      </div>
                    </div>

                    <div className="product-draft-ai__copy-grid">
                      <div className="product-draft-ai__copy-panel">
                        <div className="product-draft-ai__copy-head">
                          <div>
                            <Typography.Text strong>标题</Typography.Text>
                            <Typography.Paragraph type="secondary">用于刊登标题候选，不覆盖主标题。</Typography.Paragraph>
                          </div>
                          <Tag color={appliedAiTitleText ? 'success' : 'default'}>
                            {appliedAiTitleText ? '已应用 AI 标题' : '未应用 AI 标题'}
                          </Tag>
                        </div>
                        <div className="product-draft-ai__text-stack">
                          <div className="product-draft-ai__text-box">
                            <span>当前原文</span>
                            {aiTextPreview(originalTitleText, '暂无标题')}
                          </div>
                          <div className="product-draft-ai__text-box product-draft-ai__text-box--ai">
                            <span>已应用 AI 标题</span>
                            {aiTextPreview(appliedAiTitleText, '还没有应用 AI 标题')}
                          </div>
                        </div>
                      </div>

                      <div className="product-draft-ai__copy-panel">
                        <div className="product-draft-ai__copy-head">
                          <div>
                            <Typography.Text strong>描述</Typography.Text>
                            <Typography.Paragraph type="secondary">用于刊登描述候选，不覆盖主描述。</Typography.Paragraph>
                          </div>
                          <Tag color={appliedAiDescriptionText ? 'success' : 'default'}>
                            {appliedAiDescriptionText ? '已应用 AI 描述' : '未应用 AI 描述'}
                          </Tag>
                        </div>
                        <div className="product-draft-ai__text-stack">
                          <div className="product-draft-ai__text-box">
                            <span>当前原文</span>
                            {aiTextPreview(originalDescriptionText, '暂无描述')}
                          </div>
                          <div className="product-draft-ai__text-box product-draft-ai__text-box--ai">
                            <span>已应用 AI 描述</span>
                            {aiTextPreview(appliedAiDescriptionText, '还没有应用 AI 描述')}
                          </div>
                        </div>
                      </div>
                    </div>

                    {latestFailedAiTask ? (
                      <ErrorAlert
                        className="product-draft-ai__failure"
                        title={`最近 AI 文案任务失败：${aiTaskTypeLabel(latestFailedAiTask.taskType)}`}
                        actionHint={
                          <Space direction="vertical" size={2}>
                            <Typography.Text>{latestFailedAiTask.errorMessage || '任务未返回具体失败原因。'}</Typography.Text>
                            <Typography.Text>{aiTaskNextStep(latestFailedAiTask)}</Typography.Text>
                          </Space>
                        }
                      />
                    ) : null}
                  </SectionCard>

                  <SectionCard
                    title="最近 AI 文案任务"
                    description="任务状态只表示 AI 生成过程；成功生成后仍需人工应用到商品。"
                    className="product-draft-ai__task-section"
                  >
                    <ProTable<AITaskRow>
                      rowKey="id"
                      search={false}
                      options={false}
                      pagination={false}
                      dataSource={aiTasks}
                      locale={{
                        emptyText: (
                          <EmptyState
                            compact
                            title="暂无 AI 文案任务"
                            description="可以先生成标题建议或描述建议。"
                          />
                        ),
                      }}
                      columns={[
                        {
                          title: '类型',
                          dataIndex: 'taskType',
                          width: 176,
                          render: (_, row) => (
                            <Tooltip title={row.taskType}>
                              <Typography.Text>{aiTaskTypeLabel(row.taskType)}</Typography.Text>
                            </Tooltip>
                          ),
                        },
                        {
                          title: '状态',
                          dataIndex: 'status',
                          width: 112,
                          render: (_, row) => <StatusTag status={row.status} />,
                        },
                        {
                          title: '模型',
                          dataIndex: 'model',
                          ellipsis: true,
                          render: (_, row) => (
                            <Space size={4} wrap>
                              {row.provider ? <Tag>{aiTextProviderLabel(row.provider) || row.provider}</Tag> : null}
                              <Typography.Text ellipsis>{row.model || '—'}</Typography.Text>
                            </Space>
                          ),
                        },
                        {
                          title: '模型额度',
                          width: 120,
                          render: (_: unknown, row: AITaskRow) => (
                            <Tooltip title="输入 / 输出 token，仅作模型额度参考">
                              <Typography.Text>{aiTaskCostText(row)}</Typography.Text>
                            </Tooltip>
                          ),
                        },
                        {
                          title: '失败原因和下一步',
                          dataIndex: 'errorMessage',
                          ellipsis: true,
                          render: (_, row) =>
                            isAiTaskFailed(row) ? (
                              <Space direction="vertical" size={0}>
                                <Typography.Text type="danger" className="product-draft-ai__task-error">
                                  {row.errorMessage || '任务失败，未返回具体原因'}
                                </Typography.Text>
                                <Typography.Text type="secondary">{aiTaskNextStep(row)}</Typography.Text>
                              </Space>
                            ) : (
                              <Typography.Text type="secondary">—</Typography.Text>
                            ),
                        },
                        {
                          title: '技能模板',
                          dataIndex: 'promptCode',
                          width: 160,
                          ellipsis: true,
                          render: (_, row) => (
                            <Tooltip title={row.promptCode}>
                              <Typography.Text>{aiPromptCodeLabel(row.promptCode)}</Typography.Text>
                            </Tooltip>
                          ),
                        },
                        {
                          title: '时间',
                          dataIndex: 'createdAt',
                          width: 176,
                          render: (v) => formatDateTime(v as string),
                        },
                      ]}
                      size="small"
                    />
                  </SectionCard>

                  <SectionCard
                    title="AI 图片任务"
                    description="面向商品图片的后台处理入口；创建任务不会直接覆盖原图，结果去向在弹窗或任务内确认。"
                    className="product-draft-ai__image-workbench"
                    headerExtra={
                      <Link to={`/ai/image-tasks?productId=${encodeURIComponent(id)}`}>
                        <Button icon={<UnorderedListOutlined />}>查看图片任务</Button>
                      </Link>
                    }
                  >
                    <div className="product-draft-ai__image-status" aria-label="AI 图片任务状态">
                      <div className="product-draft-ai__image-status-item">
                        <Typography.Text type="secondary">当前图片</Typography.Text>
                        <Typography.Text strong>{imageOverview.total ? `${imageOverview.total} 张` : '暂无图片'}</Typography.Text>
                        <Typography.Text type="secondary">
                          {imageOverview.main ? `${imageOverview.main} 张主图` : '发布前建议补齐主图'}
                        </Typography.Text>
                      </div>
                      <div className="product-draft-ai__image-status-item">
                        <Typography.Text type="secondary">可翻译源图</Typography.Text>
                        <Typography.Text strong>{aiImageSource ? '已找到' : '缺少可用图片'}</Typography.Text>
                        <Typography.Text type="secondary">
                          {aiImageSource ? '默认使用当前排序第一张有地址的图片' : '请先在图片管理中添加图片'}
                        </Typography.Text>
                      </div>
                      <div className="product-draft-ai__image-status-item">
                        <Typography.Text type="secondary">结果去向</Typography.Text>
                        <Typography.Text strong>后台任务处理</Typography.Text>
                        <Typography.Text type="secondary">自动保存、设主图或设详情图需显式选择。</Typography.Text>
                      </div>
                    </div>

                    <div className="product-draft-ai__image-grid">
                      <div className="product-draft-ai__image-card product-draft-ai__image-card--primary">
                        <div className="product-draft-ai__image-card-head">
                          <RobotOutlined />
                          <div>
                            <Typography.Text strong>AI 图片生成与处理</Typography.Text>
                            <Typography.Paragraph type="secondary">
                              新建去水印、去背景、营销图、主图优选等图片任务，处理过程在后台执行。
                            </Typography.Paragraph>
                          </div>
                        </div>
                        <Space wrap size={[8, 8]}>
                          <Button type="primary" icon={<RobotOutlined />} onClick={() => openCreateImageTask({})}>
                            新建图片任务
                          </Button>
                          <Button icon={<StarOutlined />} onClick={() => void runSelectBestMain('recommend')}>
                            推荐最佳主图
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
                      </div>

                      <div className="product-draft-ai__image-card">
                        <div className="product-draft-ai__image-card-head">
                          <TranslationOutlined />
                          <div>
                            <Typography.Text strong>图片文字翻译</Typography.Text>
                            <Typography.Paragraph type="secondary">
                              选择当前商品图片创建翻译任务，原图不覆盖；译后图片可保存为商品图片或详情图。
                            </Typography.Paragraph>
                          </div>
                        </div>
                        <Space direction="vertical" size={8} className="product-draft-ai__image-action-stack">
                          <Space wrap size={[8, 8]}>
                            <Button
                              icon={<TranslationOutlined />}
                              disabled={!aiImageSource}
                              onClick={() => {
                                if (aiImageSource) openTranslateImageText(aiImageSource);
                              }}
                            >
                              翻译当前第一张可用图片
                            </Button>
                            <Button icon={<PictureOutlined />} onClick={() => openDraftLocation('images')}>
                              前往图片管理
                            </Button>
                          </Space>
                          <Typography.Text type="secondary" className="product-draft-ai__image-note">
                            需要指定其他源图时，可在图片管理列表中对单张图片发起翻译。
                          </Typography.Text>
                        </Space>
                      </div>
                    </div>

                    <Alert
                      className="product-draft-ai__image-note-alert"
                      type="info"
                      showIcon
                      message="图片任务是异步处理"
                      description="任务提交后不会立即替换页面图片；结果、失败原因和后续保存动作以 AI 图片任务列表和弹窗内配置为准。"
                    />
                  </SectionCard>

                  {data.rawData != null ? (
                    <TechnicalDetails label="原始采集 JSON（技术参考）" className="product-draft-ai__raw">
                      <TaskJsonBlock title="原始信息" value={data.rawData} maxHeight={360} last />
                    </TechnicalDetails>
                  ) : null}
                </Space>
              ),
            },
            {
              key: 'images',
              label: tabLabels.images,
              children: (
                <Space direction="vertical" className="product-draft-images" size="middle">
                  {isPinduoduoProduct(data) ? (
                    <Alert
                      type="info"
                      showIcon
                      message="拼多多图片已按页面区域自动分类，请发布前检查主图和详情图是否正确。"
                    />
                  ) : null}
                  {isTaobaoTmallProduct(data) ? (
                    <Alert
                      type="info"
                      showIcon
                      message="淘宝/天猫采集图片默认为外链，发布前建议同步到平台存储，避免外链失效。"
                    />
                  ) : null}
                  <SectionCard
                    title="图片概览"
                    description="基于当前商品详情已加载的图片数据展示，不额外请求接口。"
                    className="product-draft-images__overview-section"
                  >
                    <div className="product-draft-images__overview-grid">
                      <MetricCard
                        title="图片总数"
                        value={imageOverview.total}
                        description={imageOverview.total > 0 ? '当前商品图片记录' : '暂无商品图片'}
                        icon={<PictureOutlined />}
                        intent="data"
                      />
                      <MetricCard
                        title="主图状态"
                        value={imageOverview.main > 0 ? '已设置' : '缺少'}
                        description={imageOverview.main > 0 ? `${imageOverview.main} 张主图` : '发布前建议补齐主图'}
                        icon={<CheckCircleOutlined />}
                        intent={imageOverview.main > 0 ? 'success' : 'warning'}
                      />
                      <MetricCard
                        title="详情图状态"
                        value={imageOverview.detail > 0 ? `${imageOverview.detail} 张` : '缺少'}
                        description={imageOverview.detail > 0 ? '已识别详情图' : '可将图片设为详情图'}
                        icon={<FileTextOutlined />}
                        intent={imageOverview.detail > 0 ? 'success' : 'warning'}
                      />
                      <MetricCard
                        title="同步状态"
                        value={`${imageOverview.synced} / ${imageOverview.total}`}
                        description={imageOverview.best > 0 ? `含 ${imageOverview.best} 张最佳主图标记` : '暂无最佳主图标记'}
                        icon={<CloudUploadOutlined />}
                        intent={imageOverview.synced === imageOverview.total && imageOverview.total > 0 ? 'success' : 'default'}
                      />
                    </div>
                    {imageOverview.total > 0 && imageOverview.main === 0 ? (
                      <Alert
                        className="product-draft-images__inline-alert"
                        type="warning"
                        showIcon
                        message="当前商品没有主图"
                        description="可在图片列表中选择一张图片设为主图。"
                      />
                    ) : null}
                    {imageOverview.total > 0 && imageOverview.detail === 0 ? (
                      <Alert
                        className="product-draft-images__inline-alert"
                        type="info"
                        showIcon
                        message="当前商品没有详情图"
                        description="可在图片列表中选择图片设为详情图，或继续保留现有业务分类。"
                      />
                    ) : null}
                  </SectionCard>

                  <SectionCard
                    title="页面操作"
                    description="添加、排序和同步都需要手动触发，不会在页面加载时自动写入。"
                    className="product-draft-images__operations-section"
                  >
                    {imageSyncError ? (
                      <Alert
                        className="product-draft-images__inline-alert product-draft-images__inline-alert--top"
                        type="error"
                        showIcon
                        message="图片同步失败"
                        description={imageSyncError}
                      />
                    ) : null}
                    <div className="product-draft-images__operation-grid">
                      <div className="product-draft-images__operation-panel">
                        <Typography.Text type="secondary" className="product-draft-images__operation-title">
                          <PictureOutlined />
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
                              loading={imageSyncingScope === 'order'}
                              onClick={() => void handleReorderProductImages()}
                            >
                              同步顺序
                            </Button>
                          </Tooltip>
                        </Space>
                      </div>
                      <div className="product-draft-images__operation-panel">
                        <Typography.Text type="secondary" className="product-draft-images__operation-title">
                          <CloudUploadOutlined />
                          图片同步
                        </Typography.Text>
                        {isTaobaoTmallProduct(data) ? (
                          <Space wrap size={[8, 8]}>
                            <Button
                              loading={imageSyncingScope === 'all'}
                              onClick={() => void handleSyncProductImages('all')}
                            >
                              同步图片到平台存储
                            </Button>
                            <Button
                              loading={imageSyncingScope === 'main'}
                              onClick={() => void handleSyncProductImages('main')}
                            >
                              批量同步主图
                            </Button>
                            <Button
                              loading={imageSyncingScope === 'detail'}
                              onClick={() => void handleSyncProductImages('detail')}
                            >
                              批量同步详情图
                            </Button>
                          </Space>
                        ) : (
                          <Typography.Text type="secondary">
                            当前来源未启用该同步入口；可继续添加、编辑和标记商品图片。
                          </Typography.Text>
                        )}
                      </div>
                      <div className="product-draft-images__operation-panel product-draft-images__operation-panel--ai">
                        <Typography.Text type="secondary" className="product-draft-images__operation-title">
                          <RobotOutlined />
                          AI 图片任务
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
                        <Typography.Text type="secondary" className="product-draft-images__operation-note">
                          图片服务可用性在任务弹窗内检查；处理结果以后台任务状态为准。
                        </Typography.Text>
                      </div>
                    </div>
                  </SectionCard>

                  <SectionCard
                    title="图片列表"
                    description="每行操作只作用于当前图片，更多菜单中保留低频和危险操作。"
                    className="product-draft-images__list-section"
                  >
                    <ProTable<ProductImageRow>
                      rowKey="id"
                      search={false}
                      options={false}
                      pagination={false}
                      headerTitle={false}
                      toolBarRender={false}
                      dataSource={sortedImages}
                      columns={imageColumns}
                      size="small"
                      scroll={{ x: 1120 }}
                      locale={{
                        emptyText: (
                          <EmptyState
                            compact
                            title="暂无商品图片"
                            description="可以手动添加图片，或从采集结果补充后再回到这里管理。"
                            actionLabel="添加图片"
                            onAction={() => {
                              setLastUpload(null);
                              setImgEdit(null);
                              setImgModalOpen(true);
                            }}
                          />
                        ),
                      }}
                    />
                  </SectionCard>
                </Space>
              ),
            },
            {
              key: 'skus',
              label: tabLabels.skus,
              children: (
                <Space direction="vertical" className="product-draft-skus" size="middle">
                  <SectionCard
                    title="规格与价格"
                    description="维护当前商品的 SKU 编码、规格名称、价格和本地库存；定价只更新本地销售价，不会自动刊登。"
                    className="product-draft-skus__section"
                    headerExtra={<Tag color="blue">当前商品 {currentSkuCount} 个 SKU</Tag>}
                  >
                    <div id="pricing" />
                    {(data.source === 'custom' || isPinduoduoProduct(data)) &&
                    (data.skus ?? []).filter((s) => !String(s.id).startsWith('new_')).length === 0 ? (
                      <Alert
                        type="info"
                        showIcon
                        className="product-draft-skus__alert"
                        message={
                          isPinduoduoProduct(data)
                            ? '当前采集结果没有完整商品规格。你可以手动新增规格，或等待后续版本增强拼多多规格采集。'
                            : '当前采集结果没有商品规格。部分网站的规格和库存需要专用采集器才能完整获取，你也可以手动新增规格。'
                        }
                      />
                    ) : null}
                    {readonly ? (
                      <Alert
                        type="warning"
                        showIcon
                        className="product-draft-skus__alert"
                        message="当前账号处于只读模式"
                        description="本区仅强化只读提示，不改变现有新增、编辑、保存、删除或定价按钮的可用规则。"
                      />
                    ) : null}
                    <div className="product-draft-skus__summary" aria-label="当前商品规格摘要">
                      <div className="product-draft-skus__summary-item">
                        <span>SKU 数量</span>
                        <strong>{currentSkuCount}</strong>
                        <Typography.Text type="secondary">来自当前商品规格列表</Typography.Text>
                      </div>
                      <div className="product-draft-skus__summary-item">
                        <span>编辑方式</span>
                        <strong>行内编辑</strong>
                        <Typography.Text type="secondary">新增行保存后写入接口</Typography.Text>
                      </div>
                      <div className="product-draft-skus__summary-item">
                        <span>定价范围</span>
                        <strong>本商品 SKU</strong>
                        <Typography.Text type="secondary">试算确认后更新销售价</Typography.Text>
                      </div>
                    </div>
                    <OperationToolbar
                      className="product-draft-skus__toolbar"
                      extra={<Typography.Text type="secondary">新增 SKU 使用表格内真实入口；保存和删除仍按行处理。</Typography.Text>}
                    >
                      <Button icon={<ThunderboltOutlined />} onClick={() => setPricingOpen(true)}>
                        应用定价规则
                      </Button>
                    </OperationToolbar>
                    {currentSkuCount === 0 ? (
                      <EmptyState
                        compact
                        title="还没有商品规格"
                        description="使用下方「新增 SKU」添加一行规格，保存后才会创建本地 SKU。"
                        className="product-draft-skus__empty"
                      />
                    ) : null}
                    <div id="local-skus" className="product-draft-skus__table-anchor" />
                    <EditableProTable<SKUEditable>
                      rowKey="id"
                      className="product-draft-skus__table"
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
                          skuName: '新规格',
                          attrsText: '{}',
                        }),
                        style: {
                          marginBottom: 12,
                        },
                        creatorButtonText: '新增 SKU',
                      }}
                      editable={{
                        type: 'multiple',
                        editableKeys: skuEditableKeys,
                        onChange: setSkuEditableKeys,
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
                              imageUrl: row.imageUrl,
                            });
                            message.success('商品规格已创建');
                          } else {
                            await updateProductSku(id, row.id, {
                              skuCode: row.skuCode,
                              skuName: row.skuName,
                              attrs,
                              price: row.price,
                              imageUrl: row.imageUrl,
                            });
                            message.success('商品规格已更新');
                          }
                          await reloadDetail();
                        },
                      }}
                      columns={skuColumns}
                      scroll={{ x: 1260 }}
                    />
                  </SectionCard>
                </Space>
              ),
            },
            {
              key: 'inventory',
              label: tabLabels.inventory,
              children: (
                <Space direction="vertical" className="product-draft-inventory" size="middle">
                  <div className="product-draft-inventory__banner">
                    <InventorySyncDisabledBanner />
                  </div>
                  <SectionCard
                    title="库存状态说明"
                    description="本页只处理本地 SKU 库存、预警线和库存同步任务；平台规格映射仍按原区域展示。"
                    className="product-draft-inventory__overview"
                    headerExtra={
                      readonly ? <Tag color="warning">只读模式</Tag> : <Tag color="blue">本地库存</Tag>
                    }
                  >
                    {readonly ? (
                      <Alert
                        type="warning"
                        showIcon
                        className="product-draft-inventory__alert"
                        message="当前账号处于只读模式"
                        description="本轮不改变现有库存按钮的可用条件；如后端拒绝写操作，会按原提示展示失败原因。"
                      />
                    ) : null}
                    <div className="product-draft-inventory__summary" aria-label="当前商品库存摘要">
                      <div className="product-draft-inventory__summary-item">
                        <span>本地 SKU</span>
                        <strong>{localInventorySummary.total}</strong>
                        <Typography.Text type="secondary">来自当前商品规格列表</Typography.Text>
                      </div>
                      <div className="product-draft-inventory__summary-item">
                        <span>已设置预警线</span>
                        <strong>{localInventorySummary.warningSet}</strong>
                        <Typography.Text type="secondary">预警线或安全线已填写</Typography.Text>
                      </div>
                      <div className="product-draft-inventory__summary-item product-draft-inventory__summary-item--warning">
                        <span>需关注</span>
                        <strong>{localInventorySummary.low}</strong>
                        <Typography.Text type="secondary">低库存、低于安全线或售罄</Typography.Text>
                      </div>
                      <div className="product-draft-inventory__summary-item">
                        <span>库存未记录</span>
                        <strong>{localInventorySummary.missingStock}</strong>
                        <Typography.Text type="secondary">不按 0 展示</Typography.Text>
                      </div>
                    </div>
                    <div className="product-draft-inventory__links" aria-label="库存相关入口">
                      <Link to="/inventory/alerts">库存预警</Link>
                      <Link to="/inventory/sync-tasks">同步任务</Link>
                      <Link to={`/inventory/logs?productId=${data.id}`}>变更记录</Link>
                      <Link to="/inventory/effects">订单扣减</Link>
                    </div>
                  </SectionCard>

                  <SectionCard
                    title="本地 SKU 库存"
                    description="库存调整会写入本地规格库存；预警线只影响预警规则，不修改实际库存。"
                    className="product-draft-stock__section"
                    headerExtra={
                      <div className="product-draft-stock__section-actions">
                        <Typography.Text type="secondary">已选 {skuBatchSelKeys.length} 个 SKU</Typography.Text>
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
                      </div>
                    }
                  >
                  {localInventoryRows.length === 0 ? (
                    <EmptyState
                      compact
                      title="还没有本地 SKU"
                      description="请先在「商品规格」中新增 SKU，保存后才能调整库存和预警线。"
                      className="product-draft-stock__empty"
                    />
                  ) : null}
                  <Table<ProductSKURow>
                    loading={loading}
                    size="small"
                    className="product-draft-stock__table"
                    pagination={false}
                    rowKey="id"
                    dataSource={localInventoryRows}
                    scroll={{ x: 1080 }}
                    rowSelection={{
                      selectedRowKeys: skuBatchSelKeys,
                      onChange: (keys) => setSkuBatchSelKeys(keys.map(String)),
                    }}
                    columns={[
                      {
                        title: '编码',
                        dataIndex: 'skuCode',
                        width: 168,
                        ellipsis: true,
                        render: (v: string | undefined, r) => (
                          <Tooltip title={v || r.id}>
                            <Typography.Text className="product-draft-stock__code">{v || r.id}</Typography.Text>
                          </Tooltip>
                        ),
                      },
                      {
                        title: '规格',
                        dataIndex: 'skuName',
                        width: 240,
                        render: (_v, r) => (
                          <Space direction="vertical" size={2} className="product-draft-stock__sku">
                            <Typography.Text strong className="product-draft-stock__sku-name">
                              {r.skuName || '未填写规格名称'}
                            </Typography.Text>
                            {r.attrs ? (
                              <Typography.Text type="secondary" className="product-draft-stock__attrs">
                                {attrsToText(r.attrs)}
                              </Typography.Text>
                            ) : null}
                          </Space>
                        ),
                      },
                      {
                        title: '库存',
                        dataIndex: 'stock',
                        width: 96,
                        align: 'right' as const,
                        className: 'product-draft-stock__number-col',
                        render: (_v, r) =>
                          typeof r.stock === 'number' ? (
                            <Typography.Text className="product-draft-stock__number">{r.stock}</Typography.Text>
                          ) : (
                            <Typography.Text type="secondary">未记录</Typography.Text>
                          ),
                      },
                      {
                        title: '预警',
                        dataIndex: 'warningStock',
                        width: 88,
                        align: 'right' as const,
                        className: 'product-draft-stock__number-col',
                        render: (_v, r) =>
                          typeof r.warningStock === 'number' ? (
                            <Typography.Text className="product-draft-stock__number">{r.warningStock}</Typography.Text>
                          ) : (
                            <Typography.Text type="secondary">未设置</Typography.Text>
                          ),
                      },
                      {
                        title: '安全',
                        dataIndex: 'safetyStock',
                        width: 88,
                        align: 'right' as const,
                        className: 'product-draft-stock__number-col',
                        render: (_v, r) =>
                          typeof r.safetyStock === 'number' ? (
                            <Typography.Text className="product-draft-stock__number">{r.safetyStock}</Typography.Text>
                          ) : (
                            <Typography.Text type="secondary">未设置</Typography.Text>
                          ),
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
                        width: 236,
                        fixed: 'right' as const,
                        className: 'product-draft-stock__action-col',
                        render: (_x, r) => (
                          <Space wrap size={4} className="product-draft-stock__row-actions">
                            <Button
                              type="link"
                              size="small"
                              className="product-draft-stock__action product-draft-stock__action--primary"
                              onClick={() => {
                                setAdjustTarget(r);
                                setAdjustOpen(true);
                                setInvAdjustIdempotencyKey(createInventoryIdempotencyKey('manual-adjust'));
                                setInvAdjustLoading(true);
                                const sequence = ++invAdjustLoadSequence.current;
                                void Promise.all([listInventoryWarehouses(), listSkuWarehouseBalances(id, r.id)])
                                  .then(([warehouseResult, balanceResult]) => {
                                    if (sequence !== invAdjustLoadSequence.current) return;
                                    const warehouses = (warehouseResult.list ?? []).filter((item) => item.status === 'active');
                                    const balances = balanceResult.list ?? [];
                                    setInvAdjustWarehouses(warehouses);
                                    setInvAdjustBalances(balances);
                                    const selected = warehouses.find((item) => item.isDefault) ?? warehouses[0];
                                    const current = balances.find((item) => item.warehouseId === selected?.id);
                                    adjustForm.setFieldsValue({
                                      warehouseId: selected?.id,
                                      stock: current?.onHand ?? (balances.length === 0 && selected?.isDefault ? Number(r.stock ?? 0) : 0),
                                      reason: 'manual_adjust',
                                      remark: '',
                                    });
                                  })
                                  .catch((error: unknown) => {
                                    if (sequence !== invAdjustLoadSequence.current) return;
                                    message.error((error as Error)?.message || '仓库库存加载失败');
                                    setInvAdjustWarehouses([]);
                                    setInvAdjustBalances([]);
                                  })
                                  .finally(() => {
                                    if (sequence === invAdjustLoadSequence.current) setInvAdjustLoading(false);
                                  });
                              }}
                            >
                              调整库存
                            </Button>
                            <Button
                              type="link"
                              size="small"
                              className="product-draft-stock__action"
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
                              className="product-draft-stock__action product-draft-stock__action--muted"
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
                  </SectionCard>

                  <Modal
                    title="批量设置预警线（本商品）"
                    open={skuBatchStockOpen}
                    forceRender
                    width={640}
                    rootClassName="tm-product-draft-detail product-draft-inventory__modal-root"
                    className="product-draft-inventory__modal"
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
                            message.error('请勾选规格，或改用「本商品全部规格」');
                            return Promise.reject(new Error('validation'));
                          }
                          return new Promise<void>((resolve, reject) => {
                            Modal.confirm({
                              title: '确认仅修改预警线？',
                              content:
                                '不修改实际库存，不同步平台，不写入库存流水。将影响的规格数：' +
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
                    <Typography.Paragraph type="secondary" className="product-draft-inventory__modal-note">
                      匹配数：{skuBatchPreviewLoading ? '计算中…' : skuBatchMatched !== null ? `${skuBatchMatched} 个规格` : '—'}
                    </Typography.Paragraph>
                    <Form form={skuBatchStockForm} layout="vertical" initialValues={{ warningStock: 10, safetyStock: 2 }}>
                      <Form.Item label="应用范围">
                        <Radio.Group
                          value={skuBatchScope}
                          onChange={(e) => setSkuBatchScope(e.target.value as 'selected' | 'all')}
                        >
                          <Radio value="all">本商品全部规格</Radio>
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

                  <SectionCard
                    title="库存同步任务"
                    description="已刊登规格映射只用于识别要同步的本地 SKU 与平台 SKU；创建任务不代表平台库存已经同步完成。"
                    className="product-draft-inventory-sync__section"
                  >
                  <div className="product-draft-platform-sku__brief" aria-label="平台 SKU 映射上下文">
                    <div className="product-draft-platform-sku__brief-main">
                      <Typography.Text strong>映射范围</Typography.Text>
                      <Typography.Paragraph type="secondary">
                        当前表格来自已刊登规格列表，按平台和店铺保留独立映射。缺少平台商品 ID、平台规格编码，或抖店规格处于待确认/未匹配状态时，不能创建该行的库存同步任务。
                      </Typography.Paragraph>
                    </div>
                    <div className="product-draft-platform-sku__metrics">
                      <div className="product-draft-platform-sku__metric">
                        <span>已刊登规格</span>
                        <strong>{platformSkuMappingSummary.total}</strong>
                      </div>
                      <div className="product-draft-platform-sku__metric">
                        <span>平台商品 ID</span>
                        <strong>{platformSkuMappingSummary.withProduct}</strong>
                      </div>
                      <div className="product-draft-platform-sku__metric">
                        <span>平台 SKU</span>
                        <strong>{platformSkuMappingSummary.withSku}</strong>
                      </div>
                      <div className="product-draft-platform-sku__metric product-draft-platform-sku__metric--muted">
                        <span>不可同步</span>
                        <strong>{platformSkuMappingSummary.blocked}</strong>
                      </div>
                    </div>
                    <div className="product-draft-platform-sku__scope">
                      <Typography.Text type="secondary">
                        平台：{platformSkuMappingSummary.platforms.slice(0, 3).join(' / ') || '—'}
                        {platformSkuMappingSummary.platforms.length > 3 ? ` 等 ${platformSkuMappingSummary.platforms.length} 个` : ''}
                      </Typography.Text>
                      <Typography.Text type="secondary">
                        店铺：{platformSkuMappingSummary.shops.slice(0, 2).join(' / ') || '—'}
                        {platformSkuMappingSummary.shops.length > 2 ? ` 等 ${platformSkuMappingSummary.shops.length} 个` : ''}
                      </Typography.Text>
                    </div>
                  </div>
                  <div className="product-draft-inventory-sync__toolbar">
                    <Select
                      allowClear
                      placeholder="按平台筛选（批量同步）"
                      className="product-draft-inventory-sync__platform-filter"
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
                      disabled={pubSkuSelectedKeys.length === 0}
                      onClick={() => {
                        confirmInventorySync(
                          `选中的 ${pubSkuSelectedKeys.length} 条刊登规格`,
                          true,
                          async () => {
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
                        );
                      }}
                    >
                      批量同步到平台
                    </Button>
                    <Typography.Text type="secondary" className="product-draft-inventory-sync__hint">
                      勾选左侧可选行；不可选项表示缺少平台映射或未开放库存同步。
                    </Typography.Text>
                  </div>
                  <Spin spinning={pubSkuLoading}>
                    <Table<PublicationSkuListingRow>
                      size="small"
                      className="product-draft-inventory-sync__table"
                      rowKey={(r) => r.publicationSkuId || `${r.publicationId || 'publication'}-${r.productSkuId || 'sku'}-${r.externalSkuId || 'external'}`}
                      pagination={false}
                      dataSource={filteredPubSkuRowsForBulk}
                      scroll={{ x: 1180 }}
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
                      locale={{
                        emptyText: (
                          <EmptyState
                            compact
                            title="暂无已刊登规格"
                            description="创建平台商品草稿或完成刊登后，这里会显示可用于库存同步的 SKU 映射。"
                          />
                        ),
                      }}
                      columns={[
                        {
                          title: '店铺',
                          width: 152,
                          render: (_, r) => platformSkuValue(r.shopName || r.shopId),
                        },
                        { title: '平台', dataIndex: 'platform', width: 108, render: (v: string) => platformDisplayName(v) },
                        {
                          title: '本地商品规格',
                          width: 190,
                          render: (_, r) => (
                            <Space direction="vertical" size={2} className="product-draft-platform-sku__local">
                              <Typography.Text strong className="product-draft-platform-sku__name">
                                {r.skuCode || '未填写规格编码'}
                              </Typography.Text>
                              <span className="product-draft-platform-sku__sub-id">
                                {platformSkuValue(r.productSkuId)}
                              </span>
                            </Space>
                          ),
                        },
                        {
                          title: '外部商品 ID',
                          dataIndex: 'externalProductId',
                          width: 160,
                          render: (t: string | undefined) => platformSkuValue(t),
                        },
                        {
                          title: '平台规格编码',
                          dataIndex: 'externalSkuId',
                          width: 160,
                          render: (t: string | undefined) => platformSkuValue(t),
                        },
                        {
                          title: '规格绑定',
                          width: 128,
                          render: (_x, r) => {
                            if ((r.platform || '').toLowerCase() !== 'douyin_shop') return '—';
                            const status = r.bindStatus || (r.externalSkuId ? 'bound' : 'unmatched');
                            return (
                              <Space direction="vertical" size={2} className="product-draft-platform-sku__status">
                                {douyinBindStatusTag(status)}
                                <Typography.Text type="secondary">{douyinBindStatusHint(status)}</Typography.Text>
                              </Space>
                            );
                          },
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
                            const shouldManageBinding = isDouyin && (blocked || !hasBinding);
                            const disableReason = isDouyin && st === 'ambiguous'
                              ? '找到多个可能的抖店规格，请到刊登 Tab 确认绑定后再同步库存。'
                              : isDouyin && (st === 'unmatched' || st === 'failed' || !hasBinding)
                                ? '该规格还没有绑定抖店规格，请到刊登 Tab 管理绑定后再同步库存。'
                                : '当前平台未开放库存同步、店铺未授权，或该映射行不可用';
                            const btn = (
                              <Button
                                type="link"
                                size="small"
                                disabled={!canSync}
                                className="product-draft-inventory-sync__action"
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
                            const syncAction = canSync ? btn : (
                              <Tooltip title={disableReason}>
                                <span>{btn}</span>
                              </Tooltip>
                            );
                            return shouldManageBinding ? (
                              <Space direction="vertical" size={2}>
                                {syncAction}
                                <Button
                                  type="link"
                                  size="small"
                                  className="product-draft-inventory-sync__action"
                                  onClick={() => openDraftLocation('publish', 'douyin-sku-bindings')}
                                >
                                  管理绑定
                                </Button>
                              </Space>
                            ) : syncAction;
                          },
                        },
                      ]}
                    />
                  </Spin>
                  </SectionCard>
                </Space>
              ),
            },
            {
              key: 'readiness',
              label: tabLabels.readiness,
              children: (
                <SectionCard
                  title="发布检查"
                  description="检查当前草稿在所选平台下的完整性。检查通过不代表已经刊登，重新检查也不会自动修复商品字段。"
                  id="publish-check"
                  className="product-draft-readiness publish-check"
                  headerExtra={
                    <OperationToolbar>
                      <Button type="primary" icon={<ReloadOutlined />} loading={readinessLoading} onClick={() => void runReadinessForTab()}>
                        重新检查
                      </Button>
                    </OperationToolbar>
                  }
                >
                  <Space direction="vertical" className="product-draft-readiness__stack" size="large">
                    <div className="product-draft-readiness__control-strip" aria-label="发布检查范围">
                      <div className="product-draft-readiness__mode-copy">
                        <Typography.Text strong>草稿完整性检查</Typography.Text>
                        <Typography.Text type="secondary">
                          当前页面固定使用 draft 模式；未选店铺时只校验商品、规格、图片等草稿内容，选定店铺后会把平台和店铺条件一并纳入检查。
                        </Typography.Text>
                      </div>
                      <Space wrap align="center" className="product-draft-readiness__controls">
                        <Typography.Text strong>目标平台</Typography.Text>
                        <Select
                          className="product-draft-readiness__platform-select"
                          value={readinessPlat}
                          onChange={(v) => setReadinessPlat(v)}
                          options={[
                            'douyin_shop',
                            'tiktok',
                            'shopee',
                            'lazada',
                            'amazon',
                            ...(!isProductionBuild ? ['mock'] : []),
                          ].map((p) => ({
                            label: platformDisplayLabel(p),
                            value: p,
                          }))}
                        />
                        <Typography.Text strong>店铺</Typography.Text>
                        <Select
                          className="product-draft-readiness__shop-select"
                          placeholder="选择已授权店铺"
                          allowClear
                          showSearch
                          optionFilterProp="label"
                          value={readinessShopId || undefined}
                          onChange={(v) => setReadinessShopId(v ? String(v) : '')}
                          options={shopsForReadinessPlat.map((s) => ({
                            label: `${s.shopName} (${platformDisplayLabel(s.platform)})`,
                            value: s.id,
                          }))}
                        />
                      </Space>
                    </div>
                    {readinessLoading && !readinessResult ? (
                      <div className="product-draft-readiness__loading">
                        <Spin />
                        <Typography.Text type="secondary">正在请求发布检查结果。</Typography.Text>
                      </div>
                    ) : readinessResult ? (
                      <>
                        <div className="product-draft-readiness__summary" aria-label="发布检查摘要">
                          <MetricCard
                            title="总状态"
                            value={readinessStatusTag(readinessResult)}
                            description={`平台 ${platformDisplayLabel(readinessResult.platform || readinessPlat)} · ${readinessResult.shopId || readinessShopId ? '包含店铺条件' : '未选择店铺'}`}
                            intent={readinessResult.errorCount > 0 ? 'danger' : readinessResult.warningCount > 0 ? 'warning' : 'success'}
                          />
                          <MetricCard
                            title="阻断项"
                            value={readinessResult.errorCount}
                            description={readinessResult.errorCount > 0 ? '需要先处理后再进入下一步。' : '当前检查范围内没有阻断项。'}
                            intent={readinessResult.errorCount > 0 ? 'danger' : 'default'}
                          />
                          <MetricCard
                            title="警告项"
                            value={readinessResult.warningCount}
                            description={readinessResult.warningCount > 0 ? '建议发布前人工确认。' : '当前检查范围内没有警告项。'}
                            intent={readinessResult.warningCount > 0 ? 'warning' : 'default'}
                          />
                          <MetricCard
                            title="下一步"
                            value={readinessResult.canPublish ? '可继续' : '需处理'}
                            description="来自检查接口返回的 canPublish，不代表已刊登。"
                            intent={readinessResult.canPublish ? 'success' : 'default'}
                          />
                        </div>
                        {readinessResult.errorCount === 0 && readinessResult.warningCount === 0 && readinessChecks.length === 0 ? (
                          <Alert
                            type="success"
                            showIcon
                            message="当前检查范围内没有发现问题"
                            description="这只表示草稿检查未返回阻断或警告，不会自动创建刊登草稿，也不代表商品已经发布。"
                          />
                        ) : null}
                        {readinessChecks.length === 0 && (readinessResult.errorCount > 0 || readinessResult.warningCount > 0) ? (
                          <Alert
                            type="info"
                            showIcon
                            message="检查返回了汇总状态，但没有返回检查项列表"
                            description="请根据总状态处理，或点击重新检查再次获取明细。"
                          />
                        ) : null}
                        {readinessErrorItems.length > 0 ? (
                          <div className="product-draft-readiness__issue-band product-draft-readiness__issue-band--danger">
                            <div className="product-draft-readiness__issue-band-head">
                              <Typography.Text strong>阻断项</Typography.Text>
                              <Tag color="red">{readinessErrorItems.length}</Tag>
                            </div>
                            {readinessCheckList(readinessErrorItems, 4)}
                          </div>
                        ) : null}
                        {readinessWarningItems.length > 0 ? (
                          <div className="product-draft-readiness__issue-band product-draft-readiness__issue-band--warning">
                            <div className="product-draft-readiness__issue-band-head">
                              <Typography.Text strong>警告项</Typography.Text>
                              <Tag color="orange">{readinessWarningItems.length}</Tag>
                            </div>
                            {readinessCheckList(readinessWarningItems, 4)}
                          </div>
                        ) : null}
                        {readinessSuggestionItems.length > 0 ? (
                          <div className="product-draft-readiness__issue-band product-draft-readiness__issue-band--muted">
                            <div className="product-draft-readiness__issue-band-head">
                              <Typography.Text strong>建议项</Typography.Text>
                              <Tag>{readinessSuggestionItems.length}</Tag>
                            </div>
                            {readinessCheckList(readinessSuggestionItems, 4)}
                          </div>
                        ) : null}
                        {readinessGroups.length > 0 ? (
                          <Collapse
                            className="product-draft-readiness__groups"
                            defaultActiveKey={readinessDefaultActiveKeys.length > 0 ? readinessDefaultActiveKeys : readinessGroups.slice(0, 1)}
                            items={readinessGroups.map((g) => {
                              const rows = readinessChecks.filter((c) => (c.group || 'other') === g);
                              const groupErrors = rows.filter((c) => (c.level || '').toLowerCase() === 'error').length;
                              const groupWarnings = rows.filter((c) => (c.level || '').toLowerCase() === 'warning').length;
                              return {
                                key: g,
                                label: (
                                  <div className="product-draft-readiness__group-label">
                                    <Typography.Text strong>{READINESS_GROUP_LABEL[g] || g}</Typography.Text>
                                    <Space size={4} wrap>
                                      {groupErrors > 0 ? <Tag color="red">阻断 {groupErrors}</Tag> : null}
                                      {groupWarnings > 0 ? <Tag color="orange">警告 {groupWarnings}</Tag> : null}
                                      {groupErrors === 0 && groupWarnings === 0 ? <Tag>检查项 {rows.length}</Tag> : null}
                                    </Space>
                                  </div>
                                ),
                                children: rows.length > 0 ? (
                                  <Table
                                    className="product-draft-readiness__table"
                                    size="small"
                                    pagination={false}
                                    rowKey={(row) => `${g}-${row.code}-${row.relatedResourceType || ''}-${row.relatedResourceId || ''}-${row.message}`}
                                    dataSource={rows}
                                    columns={[
                                      {
                                        title: '级别',
                                        width: 96,
                                        render: (_: unknown, row: ReadinessCheckItem) => readinessLevelTag(row.level),
                                      },
                                      {
                                        title: '检查项',
                                        render: (_: unknown, row: ReadinessCheckItem) => {
                                          const loc = localizePublishCheckItem(row);
                                          return (
                                            <Space direction="vertical" size={2} className="product-draft-readiness__check-copy">
                                              <Typography.Text strong>{loc.title}</Typography.Text>
                                              <Typography.Text type="secondary">{loc.message}</Typography.Text>
                                              {row.code ? <Typography.Text type="secondary">编号：{row.code}</Typography.Text> : null}
                                            </Space>
                                          );
                                        },
                                      },
                                      {
                                        title: '建议 / 操作',
                                        width: 260,
                                        render: (_: unknown, row: ReadinessCheckItem) => {
                                          const fx = getProductReadinessAction(row.code);
                                          return (
                                            <Space direction="vertical" size={4} className="product-draft-readiness__action-cell">
                                              {row.suggestion ? <Typography.Text type="secondary">{row.suggestion}</Typography.Text> : null}
                                              {fx ? (
                                                fx.tab ? (
                                                  <Button
                                                    type="link"
                                                    size="small"
                                                    className="product-draft-readiness__action"
                                                    onClick={() => openDraftLocation(fx.tab!, fx.section)}
                                                  >
                                                    {fx.label}
                                                    {fx.section ? ` · ${PRODUCT_DRAFT_TAB_LABELS[fx.tab!] || fx.tab}` : ''}
                                                  </Button>
                                                ) : (
                                                  <Link className="product-draft-readiness__action" to={fx.href!}>{fx.label}</Link>
                                                )
                                              ) : (
                                                <Typography.Text type="secondary">查看检查项说明后手动处理</Typography.Text>
                                              )}
                                              {row.technicalDetails ? (
                                                <TechnicalDetails label="技术信息">
                                                  <TaskJsonBlock title="检查项技术信息" value={row.technicalDetails} last />
                                                </TechnicalDetails>
                                              ) : null}
                                            </Space>
                                          );
                                        },
                                      },
                                    ]}
                                  />
                                ) : (
                                  <EmptyState compact title="当前分组没有检查项" description="保留分组顺序，等待检查接口返回明细。" />
                                ),
                              };
                            })}
                          />
                        ) : null}
                        <TechnicalDetails label="完整检查结果">
                          <TaskJsonBlock title="检查结果" value={readinessResult} last />
                        </TechnicalDetails>
                      </>
                    ) : readinessError ? (
                      <Alert
                        type="error"
                        showIcon
                        message="发布检查请求失败"
                        description={
                          <Space direction="vertical" size={8} className="product-draft-readiness__error-copy">
                            <Typography.Text>{readinessError}</Typography.Text>
                            <Typography.Text type="secondary">请保留当前商品内容，稍后点击「重新检查」重试。</Typography.Text>
                            <TechnicalDetails label="检查失败技术信息">
                              <TaskJsonBlock title="错误信息" value={{ message: readinessError }} last />
                            </TechnicalDetails>
                          </Space>
                        }
                      />
                    ) : (
                      <EmptyState
                        compact
                        title="尚未执行发布检查"
                        description="选择平台与店铺后点击「重新检查」。未选店铺时仅校验商品 / 规格 / 图片，不校验店铺与平台配置。"
                      />
                    )}
                  </Space>
                </SectionCard>
              ),
            },
            {
              key: 'publish',
              label: tabLabels.publish,
              children: (
                <Spin spinning={pubCtxLoading || publishReadinessLoading}>
                  <Space direction="vertical" className="product-draft-publish" size="middle">
                    <SectionCard
                      title="刊登流程说明"
                      description="先确认商品内容、图片、规格、类目和平台能力，再选择合适的刊登入口。"
                      className="product-draft-publish__intro"
                    >
                      <div className="product-draft-publish__flow">
                        <div>
                          <Typography.Text strong>创建刊登草稿</Typography.Text>
                          <Typography.Paragraph type="secondary">
                            多平台中心和抖店专项流程会先创建可继续编辑或确认的草稿，不代表商品已经正式提交到平台。
                          </Typography.Paragraph>
                        </div>
                        {!isProductionBuild ? (
                          <div>
                            <Typography.Text strong>提交刊登</Typography.Text>
                            <Typography.Paragraph type="secondary">
                              传统入口会在发布检查通过后调用刊登提交接口，可能产生真实平台写操作和后台刊登任务。
                            </Typography.Paragraph>
                          </div>
                        ) : null}
                        <div>
                          <Typography.Text strong>发布检查</Typography.Text>
                          <Typography.Paragraph type="secondary">
                            检查通过只说明当前资料满足规则，仍需要选择路径并手动触发草稿创建或刊登提交。
                          </Typography.Paragraph>
                        </div>
                      </div>
                    </SectionCard>
                    <SectionCard
                      title="当前刊登条件摘要"
                      description="仅展示当前页面已经加载到的店铺、平台、图片、规格、抖店配置和发布检查状态。"
                      className="product-draft-publish__summary-card"
                      headerExtra={<Button onClick={() => void reloadPublishContext()}>刷新快照</Button>}
                    >
                      <Space direction="vertical" style={{ width: '100%' }} size="middle">
                        {pubCtxError ? (
                          <Alert
                            type="error"
                            showIcon
                            message="刊登上下文加载失败"
                            description={pubCtxError}
                            action={<Button size="small" onClick={() => void reloadPublishContext()}>重新加载</Button>}
                          />
                        ) : null}
                        {publishWriteDisabled ? (
                          <Alert
                            type="info"
                            showIcon
                            message="当前账号无刊登草稿写权限"
                            description="可查看配置、任务和刊登记录；刊登检查、草稿创建、配置保存与图片上传均已禁用。"
                          />
                        ) : null}
                        <div className="product-draft-publish__condition-grid">
                          <div className="product-draft-publish__condition">
                            <span>店铺上下文</span>
                            <strong>{shopsList.length ? `${shopsList.length} 个已授权店铺` : '无已授权店铺'}</strong>
                            <Typography.Text type="secondary">
                              {isProductionBuild
                                ? `${shopsList.length} 个店铺可用于本地草稿或受控平台流程`
                                : eligibleShopsForPublish.length
                                  ? `${eligibleShopsForPublish.length} 个店铺支持传统刊登或 beta`
                                  : pubCtxError
                                    ? '店铺数据加载失败'
                                    : '传统刊登暂无可用店铺'}
                            </Typography.Text>
                          </div>
                          <div className="product-draft-publish__condition">
                            <span>平台能力</span>
                            <strong>{platformPublishAvailableCount ? `${platformPublishAvailableCount} 个平台可刊登` : '未发现可用能力'}</strong>
                            <Typography.Text type="secondary">
                              {platformsMeta.length ? '来自平台接入服务的商品刊登能力' : pubCtxError ? '平台能力加载失败' : '暂无平台能力数据'}
                            </Typography.Text>
                          </div>
                          {!isProductionBuild ? (
                            <div className="product-draft-publish__condition">
                              <span>发布检查</span>
                              <strong>{publishReadiness ? readinessStatusTag(publishReadiness) : '未选择传统刊登店铺'}</strong>
                              <Typography.Text type="secondary">
                                {publishReadiness ? `错误 ${publishReadiness.errorCount} · 警告 ${publishReadiness.warningCount}` : '选择店铺后会加载 publish 模式检查'}
                              </Typography.Text>
                            </div>
                          ) : null}
                          <div className="product-draft-publish__condition">
                            <span>图片准备</span>
                            <strong>{imageSyncSummary.synced} / {imageSyncSummary.total} 已同步</strong>
                            <Typography.Text type="secondary">外链 {imageSyncSummary.external} · 主图 {imageSyncSummary.externalMain} · 详情图 {imageSyncSummary.externalDetail}</Typography.Text>
                          </div>
                          <div className="product-draft-publish__condition">
                            <span>本地规格与库存</span>
                            <strong>{data.skus?.length ?? 0} 个规格</strong>
                            <Typography.Text type="secondary">刊登前需要确认规格编码、价格和库存。</Typography.Text>
                          </div>
                          <div className="product-draft-publish__condition">
                            <span>平台 SKU 映射</span>
                            <strong>{pubSkuRows.length ? `${pubSkuRows.length} 条映射记录` : '暂无映射记录'}</strong>
                            <Typography.Text type="secondary">抖店已绑定 {douyinSkuBinding?.bound ?? '—'} · 未绑定 {douyinSkuBinding?.unmatched ?? '—'}</Typography.Text>
                          </div>
                          <div className="product-draft-publish__condition">
                            <span>抖店类目与属性</span>
                            <strong>{douyinConfig.categoryId ? '已选择类目' : '未选择类目'}</strong>
                            <Typography.Text type="secondary">必填属性 {douyinRequiredAttrs.length} 项 · 未填写 {douyinMissingRequiredAttrs.length} 项</Typography.Text>
                          </div>
                          <div className="product-draft-publish__condition">
                            <span>抖店草稿映射</span>
                            <strong>{douyinMapping ? '已有草稿映射' : '未生成草稿映射'}</strong>
                            <Typography.Text type="secondary">错误 {douyinMappingErrorCount} · 警告 {douyinMappingWarningCount} · 已上传主图 {douyinUploadedMainImages}/{douyinMainImages.length}</Typography.Text>
                          </div>
                        </div>
                      </Space>
                    </SectionCard>
                    <SectionCard
                      title="阻断项和待完善项"
                      description="阻断项需要先处理；待确认项不会被本页自动修复。"
                      className="product-draft-publish__issues-card"
                    >
                      <Space direction="vertical" style={{ width: '100%' }} size="small">
                        {!pubCtxError && shopsList.length === 0 ? <Alert type="warning" showIcon message="暂无已授权店铺" description="请先完成店铺授权后再选择刊登路径。" /> : null}
                        {!isProductionBuild && !pubCtxError && shopsList.length > 0 && eligibleShopsForPublish.length === 0 ? <Alert type="warning" showIcon message="暂无支持传统刊登的店铺" description="平台接入服务未返回可用或测试中的商品刊登能力。" /> : null}
                        {publishReadinessErrors.length ? <Alert type="error" showIcon message="发布检查存在阻断" description={readinessCheckList(publishReadinessErrors, 5)} action={<Button size="small" onClick={() => openDraftLocation('readiness', 'publish-check')}>去发布检查</Button>} /> : null}
                        {publishReadinessWarnings.length ? <Alert type="warning" showIcon message="发布检查有待确认项" description={readinessCheckList(publishReadinessWarnings, 5)} action={<Button size="small" onClick={() => openDraftLocation('readiness', 'publish-check')}>查看明细</Button>} /> : null}
                        {imageSyncSummary.external > 0 ? <Alert type="warning" showIcon message="仍有图片未同步到平台存储" description="抖店创建商品草稿前还需要把需要使用的图片上传到抖店。" action={<Button size="small" onClick={() => openDraftLocation('images', 'image-list')}>去图片管理</Button>} /> : null}
                        {currentSkuCount === 0 ? <Alert type="warning" showIcon message="暂无本地 SKU" description="刊登和库存同步都依赖本地规格、价格和库存数据。" action={<Button size="small" onClick={() => openDraftLocation('skus', 'local-skus')}>去 SKU</Button>} /> : null}
                        {!douyinConfig.categoryId ? <Alert type="warning" showIcon message="抖店未选择类目" description="抖店专项流程需要先选择叶子类目。" /> : null}
                        {douyinMissingRequiredAttrs.length ? <Alert type="warning" showIcon message="抖店必填属性未完整填写" description={`仍有 ${douyinMissingRequiredAttrs.length} 项必填属性未填写。`} /> : null}
                        {douyinMapping && douyinMappingErrorCount > 0 ? <Alert type="error" showIcon message="抖店草稿映射存在错误" description={douyinIssueList(douyinMapping.errors)} /> : null}
                        {douyinMapping && douyinMainImages.length > 0 && douyinUploadedMainImages === 0 ? <Alert type="warning" showIcon message="抖店主图尚未上传成功" description="至少需要一张主图上传到抖店后，才能创建抖店商品草稿。" /> : null}
                        {!pubCtxError && shopsList.length > 0 && !publishReadinessErrors.length && !publishReadinessWarnings.length && !imageSyncSummary.external && currentSkuCount > 0 ? <Alert type="success" showIcon message="当前摘要未发现通用阻断" description="仍需根据所选路径完成对应平台配置、草稿创建或提交刊登确认。" /> : null}
                      </Space>
                    </SectionCard>
                    <SectionCard
                      title="刊登路径选择"
                      description="三条路径适用的场景不同；本页只整理入口，不自动触发任何写操作。"
                      className="product-draft-publish__paths"
                    >
                      <div className="product-draft-publish__path-grid">
                        <div className="product-draft-publish__path product-draft-publish__path--primary">
                          <Typography.Text strong>多平台刊登中心</Typography.Text>
                          <Tag color="blue">主推路径</Tag>
                          <Typography.Text type="secondary">通过中心化流程创建多平台刊登草稿，创建后刷新刊登上下文、抖店任务和平台 SKU 映射。</Typography.Text>
                        </div>
                        <div className="product-draft-publish__path">
                          <Typography.Text strong>抖店专项流程</Typography.Text>
                          <Tag>草稿和配置</Tag>
                          <Typography.Text type="secondary">处理抖店店铺、类目、属性、图片上传、草稿映射和抖店商品草稿创建；创建后仍需到抖店后台确认上架。</Typography.Text>
                        </div>
                        {!isProductionBuild ? (
                          <div className="product-draft-publish__path product-draft-publish__path--compat">
                            <Typography.Text strong>传统提交刊登</Typography.Text>
                            <Tag color="orange">兼容入口</Tag>
                            <Typography.Text type="secondary">执行 publish 模式发布检查后提交刊登任务，是可能触发真实平台写操作的入口。</Typography.Text>
                          </div>
                        ) : null}
                      </div>
                    </SectionCard>
                    <SectionCard title="多平台刊登中心" description="创建多平台刊登草稿，不等同于已经正式提交到平台。" className="product-draft-publish__multi-platform">
                      <div className="product-draft-publish__multi-platform-brief">
                        <div>
                          <Typography.Text strong>当前商品</Typography.Text>
                          <Typography.Paragraph type="secondary">{productTitle}</Typography.Paragraph>
                        </div>
                        <div>
                          <Typography.Text strong>创建后的下一步</Typography.Text>
                          <Typography.Paragraph type="secondary">创建结果会刷新刊登上下文、抖店任务和平台 SKU 映射；请继续查看任务或进入对应平台配置。</Typography.Paragraph>
                        </div>
                        <div>
                          <Typography.Text strong>只读状态</Typography.Text>
                          <Typography.Paragraph type="secondary">{publishWriteDisabled ? '当前账号只可查看状态，刊登检查和草稿创建已禁用。' : '需要手动选择平台和店铺，本页不会自动选择或自动提交。'}</Typography.Paragraph>
                        </div>
                      </div>
                      <MultiPlatformPublishCenter
                        productId={id}
                        disabled={publishWriteDisabled}
                        onDraftsCreated={async () => {
                          const rows = await reloadPublishContext();
                          await reloadDouyinPublishTasks();
                          await reloadPublicationSkus();
                          const douyinRow = rows.find(
                            (p) =>
                              (p.platform || '').toLowerCase() === 'douyin_shop' &&
                              String(p.externalProductId || '').trim() !== '',
                          );
                          await reloadDouyinSkuBindingsForPublication(douyinRow?.id);
                        }}
                      />
                    </SectionCard>
                    <Card variant="borderless" className="product-draft-publish__legacy-stack">
                    <Space direction="vertical" style={{ width: '100%' }} size="middle">
                      <Alert
                        type="info"
                        showIcon
                        message="三、各平台 / 店铺单独配置"
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
                            {commonStatusLabel(data.publishStatus || 'draft')}
                          </Tag>
                        </Descriptions.Item>
                        <Descriptions.Item label="定价结果">
                          {typeof data.salePrice === 'number' ? `${data.salePrice.toFixed(2)} ${data.currency || ''}` : '未设置售价'}
                        </Descriptions.Item>
                        <Descriptions.Item label="规格数">{data.skus?.length ?? 0}</Descriptions.Item>
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
                          同步全部图片
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
                      <div className="product-draft-douyin-flow">
                        <div className="product-draft-douyin-flow__title-block">
                          <Typography.Title level={4}>抖店专项配置与创建商品草稿</Typography.Title>
                          <Typography.Paragraph type="secondary">
                            按抖店草稿创建的真实顺序处理店铺、类目、属性、映射、图片、规格绑定和创建任务；本区域不会自动正式发布。
                          </Typography.Paragraph>
                        </div>
                        <Space direction="vertical" style={{ width: '100%' }} size="middle">
                          <Alert
                            type="info"
                            showIcon
                            message="抖店草稿流程说明"
                            description="配置保存只保存店铺、类目和属性；生成映射只生成待编辑草稿；保存映射不等于校验通过；图片上传只是上传到抖店图片存储；创建抖店商品草稿不等于正式发布或商品上线。"
                          />
                          <div className="product-draft-douyin-flow__status-grid" aria-label="抖店草稿创建前置条件摘要">
                            {douyinDraftPrerequisiteItems.map((item) => (
                              <div key={item.label} className={`product-draft-douyin-flow__status-item product-draft-douyin-flow__status-item--${item.tone}`}>
                                <div className="product-draft-douyin-flow__status-head">
                                  <span>{item.label}</span>
                                  <Tag color={item.tone === 'success' ? 'green' : item.tone === 'error' ? 'red' : item.tone === 'warning' ? 'orange' : item.tone === 'processing' ? 'blue' : undefined}>{item.status}</Tag>
                                </div>
                                <Typography.Text type="secondary">{item.detail}</Typography.Text>
                              </div>
                            ))}
                          </div>
                          <div className="product-draft-douyin-flow__context-panel">
                            <div>
                              <Typography.Text strong>当前抖店上下文</Typography.Text>
                              <Typography.Paragraph type="secondary">
                                店铺来自已授权店铺列表，类目和属性来自抖店类目缓存；请求失败会保留错误提示，不会被显示成未配置。
                              </Typography.Paragraph>
                            </div>
                            <Descriptions size="small" column={{ xs: 1, md: 2, xl: 4 }} className="product-draft-douyin-flow__context-descriptions">
                              <Descriptions.Item label="已授权抖店店铺">{douyinShops.length}</Descriptions.Item>
                              <Descriptions.Item label="当前店铺">{selectedDouyinShop?.shopName || douyinConfig.shopId || '未选择'}</Descriptions.Item>
                              <Descriptions.Item label="当前类目">{douyinConfig.categoryPath || douyinConfig.categoryId || '未选择'}</Descriptions.Item>
                              <Descriptions.Item label="最近任务">{latestDouyinTask ? tagFromPublishStatus(latestDouyinTask.status) : '暂无'}</Descriptions.Item>
                            </Descriptions>
                          </div>
                          <div id="publish-config" className="product-draft-douyin-flow__panel product-draft-douyin-flow__panel--config">
                            <div className="product-draft-douyin-flow__panel-head">
                              <div>
                                <Typography.Text strong>1. 店铺、类目与属性配置</Typography.Text>
                                <Typography.Paragraph type="secondary">
                                  保存配置会写入店铺、类目路径和当前属性值，不会生成映射，也不会创建抖店商品草稿。
                                </Typography.Paragraph>
                              </div>
                              <Space wrap className="product-draft-douyin-flow__panel-actions">
                                <Button
                                  icon={<SyncOutlined />}
                                  loading={douyinCategoryLoading}
                                  onClick={() => void reloadDouyinCategories(douyinConfig.shopId, true)}
                                >
                                  刷新类目
                                </Button>
                                <Button
                                  loading={douyinAttrLoading}
                                  disabled={!douyinConfig.categoryId}
                                  onClick={() =>
                                    void reloadDouyinAttrs(
                                      douyinConfig.categoryId,
                                      douyinConfig.shopId,
                                      true,
                                    )
                                  }
                                >
                                  刷新属性
                                </Button>
                              </Space>
                            </div>
                            <Space direction="vertical" style={{ width: '100%' }} size="middle">
                              {douyinCategoryFlat.length === 0 ? (
                                <Alert
                                  type="warning"
                                  showIcon
                                  message="暂无抖店类目数据，请先点击「刷新类目」。"
                                />
                              ) : null}
                              {douyinShops.length === 0 && !pubCtxError ? (
                                <Alert type="warning" showIcon message="暂无已授权抖店店铺" description="请先完成抖店店铺授权，再回到本页选择店铺和类目。" />
                              ) : null}
                              <Form
                                ref={(instance) => {
                                  douyinFormConnectedRef.current = Boolean(instance);
                                }}
                                form={douyinForm}
                                disabled={publishWriteDisabled}
                                layout="vertical"
                                className="product-draft-douyin-flow__form"
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
                                  if (publishWriteDisabled) {
                                    message.error('当前账号无刊登草稿写权限');
                                    return;
                                  }
                                  const cat = douyinCategoryFlat.find((x) => x.categoryId === vals.categoryId);
                                  if (vals.categoryId && !cat?.isLeaf) {
                                    message.error('只能选择抖店叶子类目');
                                    return;
                                  }
                                  if (douyinConfirmingActionRef.current || douyinSaving) return;
                                  setDouyinConfirmingAction('config');
                                  window.setTimeout(() => douyinConfirmingActionRef.current === 'config' ? setDouyinConfirmingAction('') : undefined, 800);
                                  confirmPlatformPublishConfigSave(async () => {
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
                                      setDouyinConfirmingAction('');
                                    }
                                  });
                                }}
                              >
                                <div className="product-draft-douyin-flow__form-grid">
                                  <Form.Item name="shopId" label="抖店店铺" rules={[{ required: true, message: '请选择抖店店铺' }]}>
                                    <Select
                                      placeholder="选择已授权抖店店铺"
                                      allowClear
                                      showSearch
                                      optionFilterProp="label"
                                      options={douyinShops.map((s) => ({ label: s.shopName, value: s.id }))}
                                    />
                                  </Form.Item>
                                  <Form.Item
                                    name="categoryId"
                                    label="抖店类目"
                                    rules={[{ required: true, message: '请先选择抖店商品类目' }]}
                                    extra={<Typography.Text type="secondary" className="product-draft-douyin-flow__long-text">{selectedDouyinCategory?.path}</Typography.Text>}
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
                                </div>
                                {douyinConfig.categoryId && douyinAttrs.length === 0 ? (
                                  <Alert type="info" showIcon message="该类目暂无本地属性缓存，请点击「刷新属性」。" />
                                ) : null}
                                {douyinAttrs.length > 0 ? (
                                  <Spin spinning={douyinAttrLoading}>
                                    <div className="product-draft-douyin-flow__attr-summary">
                                      <span>必填属性：{douyinAttrs.filter((a) => a.required).length || 0} 项</span>
                                      <span>可选属性：{douyinAttrs.filter((a) => !a.required).length || 0} 项</span>
                                      <span>未填写必填：{douyinMissingRequiredAttrs.length} 项</span>
                                    </div>
                                    <Row gutter={16} className="product-draft-douyin-flow__attr-grid">
                                      {douyinAttrs.map((attr) => {
                                        const opts = Array.isArray(attr.options) ? attr.options : [];
                                        return (
                                          <Col xs={24} md={12} key={attr.attrId}>
                                            <Form.Item
                                              name={['platformAttributes', attr.attrId]}
                                              label={
                                                <Space size={4} wrap className="product-draft-douyin-flow__attr-label">
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
                                <Form.Item className="product-draft-douyin-flow__submit-row">
                                  <Space wrap className="product-draft-douyin-flow__panel-actions">
                                    <Button htmlType="submit" loading={douyinSaving} disabled={publishWriteDisabled || !!douyinConfirmingAction}>
                                      保存抖店配置
                                    </Button>
                                    <Button
                                      onClick={() => {
                                        setReadinessPlat('douyin_shop');
                                        setReadinessShopId(String(douyinConfig.shopId || ''));
                                        openDraftLocation('readiness', 'publish-check');
                                      }}
                                    >
                                      查看抖店发布检查
                                    </Button>
                                  </Space>
                                </Form.Item>
                              </Form>
                            </Space>
                          </div>
                          <div className="product-draft-douyin-flow__panel product-draft-douyin-flow__panel--mapping">
                            <div className="product-draft-douyin-flow__panel-head">
                              <div>
                                <Typography.Text strong>2. 属性映射与草稿校验</Typography.Text>
                                <Typography.Paragraph type="secondary">
                                  生成映射会先保存当前配置再生成草稿；保存映射只保存编辑内容；校验映射只返回错误和警告，不代表平台审核通过。
                                </Typography.Paragraph>
                              </div>
                              {douyinMapping?.lastMappedAt ? (
                                <Typography.Text type="secondary">最近生成：{formatDateTime(douyinMapping.lastMappedAt)}</Typography.Text>
                              ) : null}
                            </div>
                            <Space direction="vertical" style={{ width: '100%' }} size="middle">
                              <Space wrap className="product-draft-douyin-flow__panel-actions">
                                <Button loading={douyinMappingLoading} disabled={publishWriteDisabled || !!douyinConfirmingAction} onClick={() => void handleBuildDouyinMapping()}>
                                  生成抖店刊登草稿
                                </Button>
                                <Button disabled={publishWriteDisabled || !douyinMapping} loading={douyinMappingSaving} onClick={() => void handleSaveDouyinMapping()}>
                                  保存刊登草稿
                                </Button>
                                <Button disabled={publishWriteDisabled} loading={douyinMappingValidating} onClick={() => void handleValidateDouyinMapping()}>
                                  校验刊登草稿
                                </Button>
                              </Space>
                              {!douyinMapping ? (
                                <EmptyState compact title="还没有抖店刊登草稿" description="请先选择抖店店铺和叶子类目，再手动生成映射。" />
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
                                  <Form
                                    ref={(instance) => {
                                      douyinMappingFormConnectedRef.current = Boolean(instance);
                                    }}
                                    form={douyinMappingForm}
                                    disabled={publishWriteDisabled}
                                    layout="vertical"
                                    className="product-draft-douyin-draft__form"
                                  >
                                    <Form.Item name="title" label="抖店标题" rules={[{ required: true, message: '请填写抖店标题' }]}>
                                      <Input showCount maxLength={80} />
                                    </Form.Item>
                                    <Form.Item name="description" label="抖店描述">
                                      <Input.TextArea rows={4} />
                                    </Form.Item>
                                  </Form>
                                  <Table
                                    size="small"
                                    className="product-draft-douyin-draft__table"
                                    rowKey={(r) => r.attrId || `${r.name || 'attr'}-${r.required ? 'required' : 'optional'}-${douyinAttrValueText(r.value)}`}
                                    pagination={false}
                                    scroll={{ x: 720 }}
                                    dataSource={douyinMapping.attributes ?? []}
                                    columns={[
                                      { title: '抖店要求填写的信息', render: (_, r) => <Typography.Text className="product-draft-douyin-flow__long-text">{r.name || r.attrId}</Typography.Text> },
                                      { title: '状态', width: 90, render: (_, r) => (r.required ? <Tag color="red">必填</Tag> : <Tag>可选</Tag>) },
                                      { title: '当前值', render: (_, r) => <Typography.Text className="product-draft-douyin-flow__long-text">{douyinAttrValueText(r.value)}</Typography.Text> },
                                    ]}
                                  />
                                </>
                              )}
                            </Space>
                          </div>
                          <div className="product-draft-douyin-flow__panel product-draft-douyin-flow__panel--images">
                            <div className="product-draft-douyin-flow__panel-head">
                              <div>
                                <Typography.Text strong>3. 商品图片准备与上传</Typography.Text>
                                <Typography.Paragraph type="secondary">
                                  上传范围保持为主图和详情图，顺序来自当前映射；上传到抖店图片存储后才可用于创建抖店商品草稿。
                                </Typography.Paragraph>
                              </div>
                              <Space wrap className="product-draft-douyin-flow__panel-actions">
                                <Button
                                  icon={<CloudUploadOutlined />}
                                  disabled={publishWriteDisabled || !douyinMapping}
                                  loading={douyinImageUploading}
                                  onClick={() => void handleUploadDouyinImages(false)}
                                >
                                  上传图片到抖店
                                </Button>
                                <Button
                                  icon={<ReloadOutlined />}
                                  disabled={publishWriteDisabled || !douyinMapping}
                                  loading={douyinImageUploading}
                                  onClick={() => void handleUploadDouyinImages(true)}
                                >
                                  重新上传全部图片
                                </Button>
                              </Space>
                            </div>
                            {!douyinMapping ? (
                              <EmptyState compact title="待生成草稿映射" description="生成映射后会显示准备上传到抖店的主图和详情图。" />
                            ) : (
                              <Space direction="vertical" style={{ width: '100%' }} size="middle">
                                <div className="product-draft-douyin-flow__image-summary">
                                  <span>主图 {douyinUploadedMainImages}/{douyinMainImages.length} 已上传</span>
                                  <span>详情图 {douyinUploadedDetailImages}/{douyinDetailImages.length} 已上传</span>
                                </div>
                                <div className="product-draft-publish__image-section">
                                  <Typography.Title level={5}>主图</Typography.Title>
                                  <Typography.Text type="secondary">图片需要先上传到抖店后，才能创建抖店商品草稿。</Typography.Text>
                                  {(douyinMapping.mainImages ?? []).length ? (
                                    <Image.PreviewGroup>
                                      <div className="product-draft-douyin-flow__image-grid">
                                        {(douyinMapping.mainImages ?? []).map((img, idx) => (
                                          <div key={douyinImageKey(img, 'main', idx)} className="product-draft-publish__image-card product-draft-douyin-flow__image-card">
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
                                                <Typography.Text type="danger" style={{ fontSize: 12 }} className="product-draft-douyin-flow__long-text">
                                                  {img.errorMessage || formatUserErrorMessage(img.errorCode)}
                                                </Typography.Text>
                                              ) : null}
                                              <Space size={4} wrap>
                                                {douyinImagePreviewUrl(img) ? (
                                                  <Button size="small" icon={<EyeOutlined />} href={douyinImagePreviewUrl(img)} target="_blank" />
                                                ) : null}
                                                {img.platformImageUrl ? (
                                                  <Button size="small" href={img.platformImageUrl} target="_blank">平台图</Button>
                                                ) : null}
                                                <Button
                                                  size="small"
                                                  icon={<ReloadOutlined />}
                                                  disabled={publishWriteDisabled}
                                                  loading={douyinImageRetryingKey === douyinImageKey(img, 'main', idx)}
                                                  onClick={() => void handleRetryDouyinImage(douyinImageKey(img, 'main', idx))}
                                                >
                                                  重试
                                                </Button>
                                              </Space>
                                            </Space>
                                          </div>
                                        ))}
                                      </div>
                                    </Image.PreviewGroup>
                                  ) : (
                                    <Typography.Text type="secondary">暂无主图</Typography.Text>
                                  )}
                                </div>
                                <div className="product-draft-publish__image-section">
                                  <Typography.Title level={5}>详情图</Typography.Title>
                                  {(douyinMapping.detailImages ?? []).length ? (
                                    <Image.PreviewGroup>
                                      <div className="product-draft-douyin-flow__image-grid">
                                        {(douyinMapping.detailImages ?? []).map((img, idx) => (
                                          <div key={douyinImageKey(img, 'detail', idx)} className="product-draft-publish__image-card product-draft-douyin-flow__image-card">
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
                                                <Typography.Text type="danger" style={{ fontSize: 12 }} className="product-draft-douyin-flow__long-text">
                                                  {img.errorMessage || formatUserErrorMessage(img.errorCode)}
                                                </Typography.Text>
                                              ) : null}
                                              <Space size={4} wrap>
                                                {douyinImagePreviewUrl(img) ? (
                                                  <Button size="small" icon={<EyeOutlined />} href={douyinImagePreviewUrl(img)} target="_blank" />
                                                ) : null}
                                                {img.platformImageUrl ? (
                                                  <Button size="small" href={img.platformImageUrl} target="_blank">平台图</Button>
                                                ) : null}
                                                <Button
                                                  size="small"
                                                  icon={<ReloadOutlined />}
                                                  disabled={publishWriteDisabled}
                                                  loading={douyinImageRetryingKey === douyinImageKey(img, 'detail', idx)}
                                                  onClick={() => void handleRetryDouyinImage(douyinImageKey(img, 'detail', idx))}
                                                >
                                                  重试
                                                </Button>
                                              </Space>
                                            </Space>
                                          </div>
                                        ))}
                                      </div>
                                    </Image.PreviewGroup>
                                  ) : (
                                    <Typography.Text type="secondary">暂无详情图</Typography.Text>
                                  )}
                                </div>
                              </Space>
                            )}
                          </div>
                          <div id="douyin-sku-bindings" className="product-draft-douyin-flow__panel product-draft-douyin-flow__panel--sku product-draft-douyin-bind__card">
                            <Spin spinning={douyinSkuBindingLoading}>
                              <div className="product-draft-douyin-flow__panel-head">
                                <div>
                                  <Typography.Text strong>4. 抖店规格绑定和 SKU 映射状态</Typography.Text>
                                  <Typography.Paragraph type="secondary">
                                    绑定区只建立本地规格与已有抖店规格的关系，不创建平台 SKU，也不会自动同步库存。
                                  </Typography.Paragraph>
                                </div>
                                <Space wrap className="product-draft-douyin-bind__actions product-draft-douyin-flow__panel-actions">
                                  <Button size="small" onClick={() => setDouyinSkuCandidatesOpen(true)} disabled={!douyinSkuBinding?.platformSkus?.length}>
                                    查看平台规格候选
                                  </Button>
                                  <Button size="small" onClick={() => void reloadDouyinSkuBindings()}>
                                    刷新绑定状态
                                  </Button>
                                  <Button
                                    size="small"
                                    loading={douyinSkuBindingSyncing}
                                    disabled={skuBindDisabled || !douyinPublication?.id}
                                    onClick={() => void handleSyncDouyinSkuBindings()}
                                  >
                                    重新校准
                                  </Button>
                                </Space>
                              </div>
                              {douyinSkuBindingError ? (
                                <Alert
                                  type="error"
                                  showIcon
                                  message="抖店规格绑定加载失败"
                                  description={douyinSkuBindingError}
                                  action={<Button size="small" onClick={() => void reloadDouyinSkuBindings()}>重新加载</Button>}
                                />
                              ) : !douyinPublication?.id ? (
                                <EmptyState
                                  compact
                                  title="暂无抖店刊登记录"
                                  description="创建抖店商品草稿后，可根据抖店商品详情校准平台规格编号，并对未匹配或待确认的本地规格建立映射。"
                                />
                              ) : (
                                <Space direction="vertical" style={{ width: '100%' }} size="middle">
                                  <div className="product-draft-douyin-bind__brief">
                                    <div className="product-draft-douyin-bind__brief-text">
                                      <Typography.Text strong>抖店规格映射</Typography.Text>
                                      <Typography.Paragraph type="secondary">
                                        候选规格来自当前抖店商品详情。手动绑定只建立本地规格与已有抖店规格的映射，不创建新的平台 SKU，也不会自动同步库存。
                                      </Typography.Paragraph>
                                    </div>
                                    <div className="product-draft-douyin-bind__context">
                                      <span>平台：抖店</span>
                                      <span>店铺：{douyinPublication.shopName || douyinPublication.shopId || '—'}</span>
                                      <span>刊登记录：{platformSkuValue(douyinPublication.id)}</span>
                                      <span>抖店商品：{platformSkuValue(douyinPublication.externalProductId)}</span>
                                    </div>
                                  </div>
                                  {douyinSkuBinding?.inventorySyncReady === false && douyinSkuBinding.inventorySyncBlockReason ? (
                                    <Alert type="warning" showIcon message={douyinSkuBinding.inventorySyncBlockReason} />
                                  ) : douyinSkuBinding?.inventorySyncReady ? (
                                    <Alert type="info" showIcon message="全部规格已有抖店规格映射，可用于创建库存同步任务。" />
                                  ) : null}
                                  <div className="product-draft-douyin-bind__status-grid" aria-label="抖店规格绑定状态摘要">
                                    <div><span>已绑定</span><strong>{douyinSkuBinding?.bound ?? '—'}</strong></div>
                                    <div><span>未绑定</span><strong>{douyinSkuBinding?.unmatched ?? '—'}</strong></div>
                                    <div><span>待确认</span><strong>{douyinSkuBinding?.ambiguous ?? '—'}</strong></div>
                                    <div><span>失败</span><strong>{douyinSkuBinding?.failed ?? '—'}</strong></div>
                                    <div><span>候选规格</span><strong>{douyinSkuBinding?.platformSkus?.length ?? '—'}</strong></div>
                                    <div>
                                      <span>最近校准</span>
                                      <strong>
                                        {douyinSkuBinding?.skuBindingSyncedAt
                                          ? formatDateTime(douyinSkuBinding.skuBindingSyncedAt)
                                          : douyinPublication.skuBindingSyncedAt
                                            ? formatDateTime(douyinPublication.skuBindingSyncedAt)
                                            : '—'}
                                      </strong>
                                    </div>
                                  </div>
                                  {(douyinSkuBinding?.rows?.length ?? 0) > 0 ? (
                                    <Table<DouyinSkuBindingRow>
                                      size="small"
                                      className="product-draft-douyin-bind__table"
                                      rowKey={(r) => r.publicationSkuId || `${r.productSkuId || 'sku'}-${r.externalSkuId || 'external'}-${r.platformSkuName || 'platform'}`}
                                      pagination={false}
                                      scroll={{ x: 1200 }}
                                      dataSource={douyinSkuBinding?.rows ?? []}
                                      columns={[
                                        { title: '本地规格编码', dataIndex: 'skuCode', width: 150, render: (v, r) => <Space direction="vertical" size={2} className="product-draft-douyin-bind__sku-cell"><Typography.Text strong className="product-draft-douyin-bind__text">{v || '未填写规格编码'}</Typography.Text><span className="product-draft-douyin-bind__id">{platformSkuValue(r.productSkuId)}</span></Space> },
                                        { title: '本地规格名称', dataIndex: 'specName', width: 180, render: (v) => <Typography.Text className="product-draft-douyin-bind__text">{v || '—'}</Typography.Text> },
                                        { title: '本地价格', width: 96, render: (_, r) => (typeof r.price === 'number' ? r.price.toFixed(2) : '—') },
                                        { title: '本地库存', width: 88, render: (_, r) => (typeof r.stock === 'number' ? r.stock : '—') },
                                        { title: '平台规格编号', dataIndex: 'externalSkuId', width: 170, render: (v) => platformSkuValue(v) },
                                        { title: '抖店规格名称', dataIndex: 'platformSkuName', width: 180, render: (v) => <Typography.Text className="product-draft-douyin-bind__text">{v || '—'}</Typography.Text> },
                                        { title: '绑定状态', dataIndex: 'bindStatus', width: 96, render: (v) => douyinBindStatusTag(v) },
                                        { title: '置信度', dataIndex: 'bindConfidence', width: 72, render: (v) => (typeof v === 'number' ? v : '—') },
                                        { title: '最近校准', dataIndex: 'lastSyncedAt', width: 156, render: (v) => (v ? formatDateTime(v) : '—') },
                                        { title: '说明', dataIndex: 'bindMessage', width: 220, render: (v, r) => <Typography.Text className="product-draft-douyin-bind__text">{v || douyinBindStatusHint(r.bindStatus)}</Typography.Text> },
                                        {
                                          title: '操作',
                                          width: 220,
                                          fixed: 'right',
                                          render: (_, r) => (
                                            <Space size={4} wrap>
                                              <Button type="link" size="small" className="product-draft-douyin-bind__action" disabled={skuBindDisabled} onClick={() => { setDouyinSkuBindTarget(r); douyinSkuBindForm.setFieldsValue({ platformSkuId: r.externalSkuId || undefined }); setDouyinSkuBindOpen(true); }}>手动绑定</Button>
                                              {r.externalSkuId ? (
                                                <Button type="link" size="small" className="product-draft-douyin-bind__action" danger disabled={skuBindDisabled} onClick={() => confirmSkuUnbind(() => void unbindDouyinSku(r.publicationSkuId).then(async () => { message.success('已解除绑定'); await reloadDouyinSkuBindings(); await reloadPublicationSkus(); }).catch((e: Error) => message.error(e.message || '解除失败')))}>解除绑定</Button>
                                              ) : null}
                                            </Space>
                                          ),
                                        },
                                      ]}
                                    />
                                  ) : (
                                    <EmptyState compact title="暂无规格绑定结果" description="点击「重新校准」从抖店拉取规格并完成匹配；未匹配或待确认规格可手动绑定。" />
                                  )}
                                </Space>
                              )}
                            </Spin>
                          </div>
                          <div className="product-draft-douyin-flow__panel product-draft-douyin-flow__panel--preview">
                            <div className="product-draft-douyin-flow__panel-head">
                              <div>
                                <Typography.Text strong>5. 抖店商品草稿预览</Typography.Text>
                                <Typography.Paragraph type="secondary">这里展示即将提交给抖店草稿创建接口的数据映射，不代表这些字段已经写入抖店平台。</Typography.Paragraph>
                              </div>
                            </div>
                            {!douyinMapping ? (
                              <EmptyState compact title="暂无草稿预览" description="生成抖店刊登草稿后，会在这里展示标题、类目、价格、库存、属性和 SKU 预览。" />
                            ) : (
                              <Space direction="vertical" style={{ width: '100%' }} size="middle">
                                <Descriptions bordered size="small" column={2} className="product-draft-publish__descriptions product-draft-douyin-draft__descriptions">
                                  <Descriptions.Item label="抖店店铺">{douyinMapping.shopId || '未选择'}</Descriptions.Item>
                                  <Descriptions.Item label="抖店类目">{douyinMapping.categoryPath || douyinMapping.categoryId || '未选择'}</Descriptions.Item>
                                  <Descriptions.Item label="价格">{douyinMoney(douyinMapping.price?.min, douyinMapping.price?.currency)}{douyinMapping.price?.max && douyinMapping.price.max !== douyinMapping.price.min ? ` - ${douyinMoney(douyinMapping.price.max, douyinMapping.price.currency)}` : ''}</Descriptions.Item>
                                  <Descriptions.Item label="库存">{douyinMapping.stock?.total ?? '未确认'}{douyinMapping.stock?.unconfirmed ? <Tag color="orange" style={{ marginLeft: 8 }}>库存未确认</Tag> : null}</Descriptions.Item>
                                </Descriptions>
                                <Table
                                  size="small"
                                  className="product-draft-douyin-draft__table"
                                  rowKey={(r) => r.localSkuId || `${r.name || 'sku'}-${douyinAttrValueText(r.attrs ?? {})}-${r.price ?? ''}`}
                                  pagination={false}
                                  scroll={{ x: 820 }}
                                  dataSource={douyinMapping.skus ?? []}
                                  columns={[
                                    { title: '商品规格', dataIndex: 'name', render: (v) => <Typography.Text className="product-draft-douyin-flow__long-text">{v || '—'}</Typography.Text> },
                                    { title: '规格值', render: (_, r) => <Typography.Text className="product-draft-douyin-flow__long-text">{douyinAttrValueText(r.attrs ?? {})}</Typography.Text> },
                                    { title: '售价', width: 110, render: (_, r) => douyinMoney(r.price, douyinMapping.price?.currency) },
                                    { title: '库存', width: 90, render: (_, r) => (r.stock == null ? '未确认' : r.stock) },
                                    { title: '规格图', width: 90, render: (_, r) => (r.imageUrl ? <Image src={r.imageUrl} width={40} height={40} /> : '无') },
                                  ]}
                                />
                              </Space>
                            )}
                          </div>
                          <div className="product-draft-douyin-flow__panel product-draft-douyin-flow__panel--create">
                            <div className="product-draft-douyin-flow__create-copy">
                              <Typography.Text strong>6. 进入运营任务审核</Typography.Text>
                              <Typography.Paragraph type="secondary">在运营任务中心冻结当前商品、映射与请求内容，经人工审核后才能创建抖店平台草稿；本页不会直接调用平台写接口。</Typography.Paragraph>
                            </div>
                            <Space wrap className="product-draft-douyin-flow__panel-actions">
                              <Button type="primary" disabled={douyinCreateDraftDisabled || !!douyinConfirmingAction} onClick={handleOpenDouyinOperationReview}>
                                进入运营任务审核
                              </Button>
                              <Button onClick={() => openDraftLocation('publish', 'douyin-sku-bindings')}>查看 SKU 绑定状态</Button>
                            </Space>
                          </div>
                          <div className="product-draft-douyin-flow__panel product-draft-douyin-flow__panel--tasks">
                            <div className="product-draft-douyin-flow__panel-head">
                              <div>
                                <Typography.Text strong>7. 创建任务与结果</Typography.Text>
                                <Typography.Paragraph type="secondary">任务记录来自抖店刊登任务列表；任务成功只表示草稿创建流程完成，不表示平台商品已经正式上线。</Typography.Paragraph>
                              </div>
                              <Button size="small" onClick={() => void reloadDouyinPublishTasks()}>刷新任务</Button>
                            </div>
                            <Spin spinning={douyinPublishTasksLoading}>
                              {douyinPublishTasksError ? (
                                <Alert type="error" showIcon message="抖店刊登任务加载失败" description={douyinPublishTasksError} action={<Button size="small" onClick={() => void reloadDouyinPublishTasks()}>重新加载</Button>} />
                              ) : douyinPublishTasks.length === 0 ? (
                                <EmptyState compact title="暂无抖店刊登任务" description="创建抖店商品草稿后会在这里显示任务处理状态。" />
                              ) : (
                                <Table
                                  size="small"
                                  className="product-draft-douyin-draft__table"
                                  rowKey="id"
                                  pagination={false}
                                  scroll={{ x: 900 }}
                                  dataSource={douyinPublishTasks}
                                  columns={[
                                    { title: '状态', dataIndex: 'status', width: 100, render: (_, r) => tagFromPublishStatus(r.status) },
                                    { title: '发布模式', dataIndex: 'publishMode', width: 140, render: (v) => publishModeLabel(v) },
                                    { title: '抖店商品 ID', dataIndex: 'platformProductId', ellipsis: true, render: (v) => v || '—' },
                                    { title: '创建时间', dataIndex: 'createdAt', width: 168, render: (v) => formatDateTime(v) },
                                    { title: '失败原因', dataIndex: 'errorMessage', ellipsis: true, render: (v, r) => { const text = (v as string) || formatUserErrorMessage(r.errorCode); return text || '—'; } },
                                    { title: '操作', width: 120, render: () => <Link to={`/product/publish-tasks?productId=${id}`}>详情</Link> },
                                  ]}
                                />
                              )}
                            </Spin>
                          </div>
                        </Space>
                      </div>
                      {!isProductionBuild ? (
                      <>
                      <Alert
                        type="warning"
                        showIcon
                        className="product-draft-publish__legacy-warning"
                        message="传统提交刊登是兼容入口"
                        description="此入口会先打开确认，再执行 publish 模式发布检查；检查通过后提交刊登任务，可能触发真实平台写操作。检查通过不代表平台最终成功，结果以后续任务和刊登记录为准。"
                      />
                      {eligibleShopsForPublish.length === 0 && !pubCtxError ? (
                        <Alert
                          type="warning"
                          showIcon
                          message="暂无可提交刊登的店铺"
                          description="只有已授权且 product_publish 能力为可用或 beta 的店铺会出现在传统提交入口。"
                        />
                      ) : null}
                      {publishReadinessLoading ? (
                        <Alert
                          type="info"
                          showIcon
                          message="正在执行 publish 模式发布检查"
                          description="检查完成前不会提交刊登任务。"
                        />
                      ) : null}
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
                      <div className="product-draft-publish__legacy-panel">
                        <div className="product-draft-publish__legacy-copy">
                          <Typography.Text strong>传统提交刊登</Typography.Text>
                          <Typography.Paragraph type="secondary">
                            选择店铺后会展示 publish readiness；点击提交后先确认，再重新检查并调用传统刊登提交接口。失败不会清空已有任务和刊登记录。
                          </Typography.Paragraph>
                        </div>
                      <Form
                        form={publishForm}
                        layout="vertical"
                        disabled={publishWriteDisabled}
                        className="product-draft-publish__legacy-form"
                        onFinish={async (vals: { shopId?: string }) => {
                          if (publishWriteDisabled) {
                            message.error('当前账号无刊登草稿写权限');
                            return;
                          }
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
                            await new Promise<void>((resolve, reject) => {
                              Modal.confirm({
                                title: '确认提交刊登？',
                                width: 640,
                                okText: '确认提交刊登',
                                cancelText: '取消',
                                okButtonProps: { danger: true },
                                content: (
                                  <Space direction="vertical" size={8}>
                                    <Typography.Text>该操作会执行 publish 模式发布检查，检查通过后提交刊登任务。</Typography.Text>
                                    <Typography.Text type="secondary">
                                      这不是本地保存，也不是只创建草稿；可能触发真实平台写操作，平台最终结果请以任务和刊登记录为准。
                                    </Typography.Text>
                                  </Space>
                                ),
                                onOk: () => resolve(),
                                onCancel: () => reject(new Error('cancelled')),
                              });
                            });
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
                              danger
                              htmlType="submit"
                              loading={publishSubmitting}
                              disabled={publishWriteDisabled || (!!publishReadiness && !publishReadiness.canPublish)}
                            >
                              提交刊登
                            </Button>
                            <Button onClick={() => void reloadPublishContext()}>刷新快照</Button>
                          </Space>
                        </Form.Item>
                      </Form>
                      </div>
                      </>
                      ) : null}
                      <div className="product-draft-publish__records-head">
                        <div>
                          <Typography.Title level={5} style={{ marginTop: 0, marginBottom: 0 }}>
                            本商品刊登记录
                          </Typography.Title>
                          <Typography.Text type="secondary">记录展示草稿、任务或平台返回状态；草稿成功不等于正式上线，正式提交后也以后续状态为准。</Typography.Text>
                        </div>
                        <Button size="small" onClick={() => void reloadPublishContext()}>刷新记录</Button>
                      </div>
                      {pubCtxError ? (
                        <Alert
                          type="error"
                          showIcon
                          message="刊登记录加载失败"
                          description={pubCtxError}
                          action={<Button size="small" onClick={() => void reloadPublishContext()}>重新加载</Button>}
                        />
                      ) : pubRows.length === 0 ? (
                        <EmptyState compact title="暂无刊登记录" description="创建草稿或提交刊登任务后，刊登记录会在这里展示。" />
                      ) : (
                        <Table<ProductPublicationRow>
                          size="small"
                          rowKey="id"
                          loading={pubCtxLoading}
                          dataSource={pubRows}
                          pagination={false}
                          scroll={{ x: 760 }}
                          columns={[
                            { title: '店铺', width: 220, render: (_, r) => <Typography.Text className="product-draft-publish__long-text">{r.shopName || r.shopId}</Typography.Text> },
                            { title: '平台', dataIndex: 'platform', width: 140, render: (v) => platformDisplayLabel(String(v ?? '')) },
                            { title: '状态', dataIndex: 'publishStatus', width: 120, render: (v) => tagFromPublishStatus(String(v ?? '')) },
                            { title: '外部商品 ID', dataIndex: 'externalProductId', width: 180, render: (v) => <Typography.Text className="product-draft-publish__long-text">{v || '—'}</Typography.Text> },
                            {
                              title: '外链',
                              width: 120,
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
                      )}
                    </Space>
                  </Card>
                  </Space>
                </Spin>
              ),
            },
              ]}
            />
          </div>
        </Space>
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
        forceRender
        destroyOnHidden
        width={760}
        className="product-draft-ai-modal"
        rootClassName="tm-product-draft-detail"
      >
        <Alert
          type="info"
          showIcon
          className="product-draft-ai-modal__notice"
          message="AI 生成可能消耗模型额度，生成结果不会自动覆盖商品内容。"
        />
        <Form
          form={aiForm}
          layout="vertical"
          className="product-draft-ai-modal__form"
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
              setAiPreparedTitle(res.optimizedTitle || '');
              message.success('优化完成');
              await reloadTasks();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '优化失败');
            } finally {
              setAiBusy(false);
            }
          }}
        >
          <div className="product-draft-ai-modal__fields">
            <Form.Item name="language" label="语言" rules={[{ required: true }]}>
              <Select options={AI_LANGUAGE_OPTIONS} placeholder="请选择生成语言" />
            </Form.Item>
            <Form.Item name="platform" label="平台" rules={[{ required: true }]}>
              <Select
                options={AI_TARGET_PLATFORM_OPTIONS}
                placeholder="请选择目标平台"
                showSearch
                optionFilterProp="label"
              />
            </Form.Item>
            <Form.Item name="maxLength" label="最长字符数" rules={[{ required: true }]}>
              <InputNumber min={20} max={500} style={{ width: '100%' }} />
            </Form.Item>
          </div>
          <Form.Item className="product-draft-ai-modal__submit">
            <Button type="primary" htmlType="submit" loading={aiBusy}>
              生成标题建议
            </Button>
          </Form.Item>
        </Form>

        {aiResult ? (
          <div className="product-draft-ai-modal__result">
            <div className="product-draft-ai-modal__result-head">
              <div>
                <Typography.Text strong>标题建议结果</Typography.Text>
                <Typography.Paragraph type="secondary">当前内容尚未应用到商品，可在下方微调后应用。</Typography.Paragraph>
              </div>
              <Tag color="processing">AI 建议未应用</Tag>
            </div>
            <div className="product-draft-ai-modal__compare">
              <div className="product-draft-ai-modal__compare-box">
                <span>原文</span>
                {aiTextPreview(data?.title || data?.originalTitle, '暂无标题')}
              </div>
              <div className="product-draft-ai-modal__compare-box product-draft-ai-modal__compare-box--ai">
                <span>AI 建议</span>
                {aiTextPreview(aiResult.optimizedTitle, '暂无建议')}
              </div>
            </div>
            {(aiResult.keywords ?? []).length || aiResult.reason ? (
              <div className="product-draft-ai-modal__meta">
                {(aiResult.keywords ?? []).length ? (
                  <Space wrap size={4}>
                    {(aiResult.keywords ?? []).map((keyword, index) => (
                      <Tag key={`${keyword}-${index}`}>{keyword}</Tag>
                    ))}
                  </Space>
                ) : null}
                {aiResult.reason ? <Typography.Text type="secondary">{aiResult.reason}</Typography.Text> : null}
              </div>
            ) : null}
            <Form.Item
              label="准备应用为 AI 标题"
              extra="应用后写入商品草稿的 AI 标题；若商品在生成后被修改，会提示内容冲突。"
            >
              <Input.TextArea rows={3} value={aiPreparedTitle} onChange={(e) => setAiPreparedTitle(e.target.value)} />
            </Form.Item>
            <Space wrap className="product-draft-ai-modal__actions">
              <Button
                type="primary"
                icon={<CheckCircleOutlined />}
                disabled={!aiPreparedTitle.trim()}
                loading={aiBusy}
                onClick={() => {
                  if (!aiResult?.taskId) return;
                  confirmApplyAiText('标题', async () => {
                    setAiBusy(true);
                    try {
                      await applyProductAITitle(id, {
                        aiTitle: aiPreparedTitle,
                        taskId: aiResult.taskId,
                        expectedUpdatedAt: data?.updatedAt,
                      });
                      message.success('已应用为 AI 标题');
                      setAiOpen(false);
                      setAiResult(null);
                      setAiPreparedTitle('');
                      await reloadDetail();
                      await reloadTasks();
                    } catch (e: unknown) {
                      const msg = (e as Error)?.message || '';
                      if (msg.includes('conflict')) {
                        message.warning('商品标题在 AI 生成后已变化，请重新确认后再应用。');
                        return;
                      }
                      message.error((e as Error)?.message || '应用失败');
                    } finally {
                      setAiBusy(false);
                    }
                  });
                }}
              >
                应用为 AI 标题
              </Button>
              <Button
                icon={<UndoOutlined />}
                loading={aiBusy}
                onClick={() => {
                  confirmUndoAiText('标题', async () => {
                    setAiBusy(true);
                    try {
                      await undoProductAITitle(id, { expectedUpdatedAt: data?.updatedAt });
                      message.success('已撤销最近一次 AI 标题应用');
                      await reloadDetail();
                      await reloadTasks();
                    } catch (e: unknown) {
                      const msg = (e as Error)?.message || '撤销失败';
                      if (msg.includes('conflict')) {
                        message.warning('AI 标题已经被再次修改，不能静默撤销。');
                      } else {
                        message.error(msg);
                      }
                    } finally {
                      setAiBusy(false);
                    }
                  });
                }}
              >
                撤销最近一次应用
              </Button>
              <Typography.Text type="secondary">任务 ID：{aiResult.taskId}</Typography.Text>
            </Space>
          </div>
        ) : null}
      </Modal>

      <Modal
        title="AI 描述生成"
        open={descOpen}
        onCancel={() => setDescOpen(false)}
        footer={null}
        forceRender
        destroyOnHidden
        width={820}
        className="product-draft-ai-modal"
        rootClassName="tm-product-draft-detail"
      >
        <Alert
          type="info"
          showIcon
          className="product-draft-ai-modal__notice"
          message="AI 生成可能消耗模型额度，生成结果不会自动覆盖商品内容。"
        />
        <Form
          form={descForm}
          layout="vertical"
          className="product-draft-ai-modal__form"
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
              setDescPreparedText(buildAiDescriptionText(res));
              message.success('生成完成');
              await reloadTasks();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '生成失败');
            } finally {
              setDescBusy(false);
            }
          }}
        >
          <div className="product-draft-ai-modal__fields">
            <Form.Item name="language" label="语言" rules={[{ required: true }]}>
              <Select options={AI_LANGUAGE_OPTIONS} placeholder="请选择生成语言" />
            </Form.Item>
            <Form.Item name="platform" label="平台" rules={[{ required: true }]}>
              <Select
                options={AI_TARGET_PLATFORM_OPTIONS}
                placeholder="请选择目标平台"
                showSearch
                optionFilterProp="label"
              />
            </Form.Item>
            <Form.Item name="tone" label="语气" rules={[{ required: true }]}>
              <Select options={AI_TONE_OPTIONS} placeholder="请选择文案语气" />
            </Form.Item>
          </div>
          <Form.Item className="product-draft-ai-modal__submit">
            <Button type="primary" htmlType="submit" loading={descBusy}>
              生成描述建议
            </Button>
          </Form.Item>
        </Form>

        {descResult ? (
          <div className="product-draft-ai-modal__result">
            <div className="product-draft-ai-modal__result-head">
              <div>
                <Typography.Text strong>描述建议结果</Typography.Text>
                <Typography.Paragraph type="secondary">当前内容尚未应用到商品，可在下方微调后应用。</Typography.Paragraph>
              </div>
              <Tag color="processing">AI 建议未应用</Tag>
            </div>
            <div className="product-draft-ai-modal__compare product-draft-ai-modal__compare--description">
              <div className="product-draft-ai-modal__compare-box">
                <span>原文</span>
                {aiTextPreview(data?.description, '暂无描述')}
              </div>
              <div className="product-draft-ai-modal__compare-box product-draft-ai-modal__compare-box--ai">
                <span>AI 建议</span>
                {aiTextPreview(descResult.description, '暂无建议')}
              </div>
            </div>
            <div className="product-draft-ai-modal__meta product-draft-ai-modal__meta--grid">
              {(descResult.highlights ?? []).length ? (
                <div>
                  <span>Highlights</span>
                  <Typography.Paragraph>{descResult.highlights.join('；')}</Typography.Paragraph>
                </div>
              ) : null}
              {(descResult.specifications ?? []).length ? (
                <div>
                  <span>Specifications</span>
                  <Typography.Paragraph>{descResult.specifications.join('；')}</Typography.Paragraph>
                </div>
              ) : null}
              {(descResult.packageIncludes ?? []).length ? (
                <div>
                  <span>Package includes</span>
                  <Typography.Paragraph>{descResult.packageIncludes.join('；')}</Typography.Paragraph>
                </div>
              ) : null}
              {descResult.notes ? (
                <div>
                  <span>Notes</span>
                  <Typography.Paragraph>{descResult.notes}</Typography.Paragraph>
                </div>
              ) : null}
              {descResult.reason ? (
                <div>
                  <span>Reason</span>
                  <Typography.Paragraph>{descResult.reason}</Typography.Paragraph>
                </div>
              ) : null}
            </div>
            <Form.Item
              label="准备应用为 AI 描述"
              extra="应用后写入商品草稿的 AI 描述；若商品在生成后被修改，会提示内容冲突。"
            >
              <Input.TextArea rows={6} value={descPreparedText} onChange={(e) => setDescPreparedText(e.target.value)} />
            </Form.Item>
            <Space wrap className="product-draft-ai-modal__actions">
              <Button
                type="primary"
                icon={<CheckCircleOutlined />}
                disabled={!descResult.taskId || !descPreparedText.trim()}
                loading={descBusy}
                onClick={() => {
                  if (!descResult?.taskId) return;
                  const text = descPreparedText.trim();
                  if (!text) return;
                  confirmApplyAiText('描述', async () => {
                    setDescBusy(true);
                    try {
                      await applyAiDescription(id, {
                        aiDescription: text,
                        taskId: descResult.taskId,
                        expectedUpdatedAt: data?.updatedAt,
                      });
                      message.success('已应用为 AI 描述');
                      setDescOpen(false);
                      setDescResult(null);
                      setDescPreparedText('');
                      await reloadDetail();
                      await reloadTasks();
                    } catch (e: unknown) {
                      const msg = (e as Error)?.message || '';
                      if (msg.includes('conflict')) {
                        message.warning('商品描述在 AI 生成后已变化，请重新确认后再应用。');
                        return;
                      }
                      message.error((e as Error)?.message || '应用失败');
                    } finally {
                      setDescBusy(false);
                    }
                  });
                }}
              >
                应用为 AI 描述
              </Button>
              <Button
                icon={<UndoOutlined />}
                loading={descBusy}
                onClick={() => {
                  confirmUndoAiText('描述', async () => {
                    setDescBusy(true);
                    try {
                      await undoAiDescription(id, { expectedUpdatedAt: data?.updatedAt });
                      message.success('已撤销最近一次 AI 描述应用');
                      await reloadDetail();
                      await reloadTasks();
                    } catch (e: unknown) {
                      const msg = (e as Error)?.message || '撤销失败';
                      if (msg.includes('conflict')) {
                        message.warning('AI 描述已经被再次修改，不能静默撤销。');
                      } else {
                        message.error(msg);
                      }
                    } finally {
                      setDescBusy(false);
                    }
                  });
                }}
              >
                撤销最近一次应用
              </Button>
              <Typography.Text type="secondary">任务 ID：{descResult.taskId}</Typography.Text>
            </Space>
          </div>
        ) : null}
      </Modal>

      <Drawer
        title={logsSku ? `库存变更 · ${logsSku.skuCode || logsSku.id}` : '库存变更'}
        open={logsOpen}
        width={720}
        rootClassName="tm-product-draft-detail product-draft-inventory__drawer-root"
        className="product-draft-inventory__drawer"
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
            className="product-draft-inventory__logs-table"
            scroll={{ x: 720 }}
            dataSource={logsRows}
            columns={[
              {
                title: '时间',
                dataIndex: 'createdAt',
                width: 168,
                render: (v: string) => formatDateTime(v),
              },
              { title: '类型', dataIndex: 'changeType', width: 136 },
              { title: '前', width: 64, dataIndex: 'beforeStock', align: 'right' as const },
              { title: '后', width: 64, dataIndex: 'afterStock', align: 'right' as const },
              { title: 'Δ', width: 64, dataIndex: 'delta', align: 'right' as const },
              { title: '原因', width: 160, dataIndex: 'reason', className: 'product-draft-inventory__logs-text' },
              { title: '备注', width: 200, dataIndex: 'remark', className: 'product-draft-inventory__logs-text' },
            ]}
          />
        </Spin>
      </Drawer>

      <Modal
        title={adjustTarget ? `调整库存 · ${adjustTarget.skuCode}` : '调整库存'}
        open={adjustOpen && !!adjustTarget}
        forceRender
        destroyOnHidden
        width={640}
        rootClassName="tm-product-draft-detail product-draft-inventory__modal-root"
        className="product-draft-inventory__modal"
        okText="保存"
        confirmLoading={invAdjustSubmitting}
        onCancel={() => {
          invAdjustLoadSequence.current += 1;
          setAdjustOpen(false);
          setInvAdjustLoading(false);
          setAdjustTarget(null);
          setInvAdjustWarehouses([]);
          setInvAdjustBalances([]);
          setInvAdjustIdempotencyKey('');
          adjustForm.resetFields();
        }}
        onOk={() => {
          if (!adjustTarget) return Promise.reject();
          return adjustForm.validateFields().then((v) => {
            const stock = Number(v.stock);
            return new Promise<void>((resolve, reject) => {
              confirmInventoryManualAdjust(
                async () => {
                  setInvAdjustSubmitting(true);
                  try {
                    await adjustSkuStock(id, adjustTarget.id, {
                      warehouseId: v.warehouseId,
                      stock,
                      idempotencyKey: invAdjustIdempotencyKey,
                      reason: String(v.reason ?? 'manual_adjust').trim(),
                      remark: String(v.remark ?? ''),
                      sync: false,
                    });
                    message.success('库存已更新');
                    invAdjustLoadSequence.current += 1;
                    setAdjustOpen(false);
                    setAdjustTarget(null);
                    setInvAdjustWarehouses([]);
                    setInvAdjustBalances([]);
                    setInvAdjustIdempotencyKey('');
                    adjustForm.resetFields();
                    await reloadDetail();
                    await reloadPublicationSkus();
                    resolve();
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || '调整失败');
                    reject(e);
                  } finally {
                    setInvAdjustSubmitting(false);
                  }
                },
                resolve,
              );
            });
          });
        }}
      >
        <Alert
          type="warning"
          showIcon
          className="product-draft-inventory__modal-alert"
          message="库存调整会覆盖所选仓库的在手库存"
          description="提交后更新所选仓库余额，并在同一事务写入库存流水和商品规格兼容聚合库存；未迁移历史量先进入默认仓或待分配仓，当前表单不会自动同步到平台。"
        />
        <Form form={adjustForm} layout="vertical">
          <Form.Item name="warehouseId" label="仓库" rules={[{ required: true, message: '请选择仓库' }]}>
            <Select
              loading={invAdjustLoading}
              placeholder="请选择启用的仓库"
              options={invAdjustWarehouses.map((item) => ({
                value: item.id,
                label: `${item.name}（${item.code}）${item.isDefault ? ' · 默认' : ''}`,
              }))}
              onChange={(warehouseId) => {
                const balance = invAdjustBalances.find((item) => item.warehouseId === warehouseId);
                const warehouse = invAdjustWarehouses.find((item) => item.id === warehouseId);
                adjustForm.setFieldValue(
                  'stock',
                  balance?.onHand ?? (invAdjustBalances.length === 0 && warehouse?.isDefault ? Number(adjustTarget?.stock ?? 0) : 0),
                );
              }}
            />
          </Form.Item>
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
        title="同步刊登规格库存"
        open={syncOpen && !!syncRow}
        forceRender
        destroyOnHidden
        width={640}
        rootClassName="tm-product-draft-detail product-draft-inventory__modal-root"
        className="product-draft-inventory__modal"
        okText="提交任务"
        confirmLoading={syncSubmitting}
        onCancel={() => {
          setSyncOpen(false);
          setSyncRow(null);
          syncForm.resetFields();
        }}
        onOk={() => {
          if (!syncRow) return Promise.reject();
          return syncForm.validateFields().then((v) => {
            const stock = Number(v.stock);
            const targetLabel = `${syncRow.platform ?? '平台'} / ${syncRow.shopName ?? syncRow.shopId ?? '—'}`;
            const externalCall = inventorySyncRunnable(syncRow.inventorySyncCapability);
            return new Promise<void>((resolve, reject) => {
              confirmInventorySync(targetLabel, externalCall, async () => {
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
                  resolve();
                } catch (e: unknown) {
                  message.error(formatInventorySyncTaskCreateError(e));
                  reject(e);
                } finally {
                  setSyncSubmitting(false);
                }
              });
            });
          });
        }}
      >
        <Typography.Paragraph type="secondary" className="product-draft-inventory__modal-note">
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
        forceRender
        destroyOnHidden
        width={520}
        rootClassName="tm-product-draft-detail product-draft-inventory__modal-root"
        className="product-draft-inventory__modal"
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
        <Typography.Paragraph type="secondary" className="product-draft-inventory__modal-note">
          预警线只用于识别低库存，不会调整实际库存，也不会同步到平台。
        </Typography.Paragraph>
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
        title={douyinSkuBindTarget ? `手动绑定抖店规格 · ${douyinSkuBindTarget.specName || douyinSkuBindTarget.skuCode || ''}` : '手动绑定抖店规格'}
        open={douyinSkuBindOpen && !!douyinSkuBindTarget}
        forceRender
        destroyOnHidden
        width={640}
        rootClassName="tm-product-draft-detail product-draft-douyin-bind__modal-root"
        className="product-draft-douyin-bind__modal"
        okText="确认绑定"
        okButtonProps={{ disabled: skuBindDisabled }}
        confirmLoading={douyinSkuBindSubmitting}
        onCancel={() => {
          setDouyinSkuBindOpen(false);
          setDouyinSkuBindTarget(null);
          douyinSkuBindForm.resetFields();
        }}
        onOk={() => {
          if (skuBindDisabled) {
            message.error('当前账号无规格绑定权限');
            return Promise.reject();
          }
          if (!douyinSkuBindTarget) return Promise.reject();
          return douyinSkuBindForm.validateFields().then((v) => {
            const platformSkuId = String(v.platformSkuId ?? '').trim();
            if (!platformSkuId) {
              message.error('请选择或填写平台规格编号');
              return Promise.reject(new Error('validation'));
            }
            const selected = (douyinSkuBinding?.platformSkus ?? []).find((c) => c.platformSkuId === platformSkuId);
            return new Promise<void>((resolve, reject) => {
              confirmSkuManualBind(async () => {
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
                  resolve();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '绑定失败');
                  reject(e);
                } finally {
                  setDouyinSkuBindSubmitting(false);
                }
              });
            });
          });
        }}
      >
        <div className="product-draft-douyin-bind__modal-brief">
          <Typography.Text strong>建立映射</Typography.Text>
          <Typography.Paragraph type="secondary">
            选择一个已有抖店规格，与当前本地规格建立映射。该操作不会创建平台 SKU，也不会自动触发库存同步。
          </Typography.Paragraph>
          <div className="product-draft-douyin-bind__modal-pair">
            <div>
              <span>本地规格</span>
              <strong>{douyinSkuBindTarget?.specName || douyinSkuBindTarget?.skuCode || '—'}</strong>
              <span className="product-draft-douyin-bind__modal-id">{platformSkuValue(douyinSkuBindTarget?.productSkuId)}</span>
            </div>
            <div>
              <span>当前平台规格</span>
              <strong>{douyinSkuBindTarget?.platformSkuName || '—'}</strong>
              <span className="product-draft-douyin-bind__modal-id">{platformSkuValue(douyinSkuBindTarget?.externalSkuId)}</span>
            </div>
          </div>
        </div>
        <Form form={douyinSkuBindForm} layout="vertical" disabled={skuBindDisabled}>
          <Form.Item name="platformSkuId" label="抖店规格" rules={[{ required: true, message: '请选择抖店规格' }]}>
            <Select
              showSearch
              placeholder="从平台候选中选择"
              optionFilterProp="label"
              className="product-draft-douyin-bind__select"
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
          <Form.Item name="platformSkuName" label="抖店规格名称">
            <Input placeholder="可选，便于识别" />
          </Form.Item>
        </Form>
        <TechnicalDetails label="映射标识">
          <Descriptions size="small" column={1}>
            <Descriptions.Item label="刊登规格">{platformSkuValue(douyinSkuBindTarget?.publicationSkuId)}</Descriptions.Item>
            <Descriptions.Item label="抖店商品">{platformSkuValue(douyinPublication?.externalProductId)}</Descriptions.Item>
          </Descriptions>
        </TechnicalDetails>
      </Modal>

      <Drawer
        title="抖店平台规格候选"
        open={douyinSkuCandidatesOpen}
        width="min(720px, calc(100vw - 16px))"
        rootClassName="tm-product-draft-detail product-draft-douyin-bind__drawer-root"
        className="product-draft-douyin-bind__drawer"
        onClose={() => setDouyinSkuCandidatesOpen(false)}
      >
        <Typography.Paragraph type="secondary" className="product-draft-douyin-bind__drawer-note">
          候选来自当前抖店商品详情，仅用于选择已有平台规格。候选列表不会自动绑定，也不会改变本地库存。
        </Typography.Paragraph>
        {(douyinSkuBinding?.platformSkus?.length ?? 0) === 0 ? (
          <EmptyState
            compact
            title="暂无平台规格候选"
            description="请先执行「重新校准」从抖店拉取商品详情；查询失败不会显示为空候选。"
          />
        ) : (
          <Table<DouyinPlatformSkuCandidate>
            size="small"
            className="product-draft-douyin-bind__candidate-table"
            rowKey="platformSkuId"
            pagination={false}
            scroll={{ x: 680 }}
            dataSource={douyinSkuBinding?.platformSkus ?? []}
            columns={[
              { title: '平台规格编号', dataIndex: 'platformSkuId', width: 180, render: (v) => platformSkuValue(v) },
              {
                title: '规格名称',
                dataIndex: 'specName',
                width: 220,
                render: (v) => <Typography.Text className="product-draft-douyin-bind__text">{v || '—'}</Typography.Text>,
              },
              { title: '价格', width: 96, render: (_, r) => (typeof r.priceYuan === 'number' ? r.priceYuan.toFixed(2) : '—') },
              { title: '库存', width: 72, render: (_, r) => (typeof r.stock === 'number' ? r.stock : '—') },
              {
                title: '绑定状态',
                width: 156,
                render: (_, r) => platformSkuCandidateStatusTag(r, douyinSkuBindTarget?.publicationSkuId),
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
