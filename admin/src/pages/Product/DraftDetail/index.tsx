import type { CSSProperties, ReactNode } from 'react';
import type { UploadRequestOption } from 'rc-upload/lib/interface';
import { formatDateTime } from '@/utils/formatTime';
import type { ProColumns } from '@ant-design/pro-components';
import {
  EditableProTable,
  ModalForm,
  PageContainer,
  ProForm,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import {
  Button,
  Card,
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
} from '@ant-design/icons';
import { ProductCollectQualityAlert } from '@/components/ProductCollectQualityAlert';
import { isPinduoduoSource } from '@/utils/pinduoduoCollectAlerts';
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
  selectBestMainProductImages,
  updateProduct,
  updateProductImage,
  updateProductSku,
  updateProductSkuStockSettings,
  type AITaskRow,
  type GenerateDescriptionResult,
  type OptimizeTitleResult,
  type ProductDetail,
  type ProductImageRow,
  type ProductSKURow,
} from '@/services/products';
import { Link } from '@umijs/renderer-react';
import {
  listProductPublications,
  publishProduct,
  type ProductPublicationRow,
} from '@/services/productPublish';
import { getProductReadiness, type ProductReadinessResult, type ReadinessCheckItem } from '@/services/productReadiness';
import PricingApplyModal from '@/components/PricingApplyModal';
import { CreateImageTaskModal, type CreateImageTaskPrefill } from '@/components/CreateImageTaskModal';
import { TranslateImageTextModal, type TranslateImageTextPrefill } from '@/components/TranslateImageTextModal';
import { queryPlatformProviders, queryShops, type PlatformProviderMeta, type ShopListRow } from '@/services/shops';
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

function formatInventorySyncTaskCreateError(e: unknown): string {
  const s = (e instanceof Error ? e.message : String(e)).trim() || '提交失败';
  const hints: string[] = [];
  if (/missing warehouse_id|platform inventory config incomplete:\s*missing warehouse_id/i.test(s)) {
    hints.push(
      'TikTok Shop：请到「设置 → 平台刊登配置 → TikTok Shop」填写默认仓库 ID，或通过任务 options.warehouse_id 覆盖。',
    );
    hints.push(
      'Shopee：请到「设置 → 平台刊登配置 → Shopee」填写默认仓库 ID，或任务 options.warehouse_id / location_id 覆盖。',
    );
    hints.push(
      'Lazada：若平台提示与仓库 / WarehouseCode 相关，请到「设置 → 平台刊登配置 → Lazada」填写默认仓库代码（warehouse_id），或任务 options.warehouse_id 覆盖。',
    );
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
    hints.push('请到「设置 → 平台开放配置 → TikTok Shop」补齐开放平台应用字段。');
  }
  if (/platform config incomplete:\s*please configure settings\.platform_shopee/i.test(s)) {
    hints.push('请到「设置 → 平台开放配置 → Shopee」补齐开放平台应用字段。');
  }
  if (/platform config incomplete:\s*please configure settings\.platform_lazada/i.test(s)) {
    hints.push('请到「设置 → 平台开放配置 → Lazada」补齐开放平台应用字段。');
  }
  if (/platform config incomplete:\s*please configure settings\.platform_amazon|please configure platform_amazon/i.test(s)) {
    hints.push('请到「设置 → 平台开放配置 → Amazon」补齐 SP-API / LWA 应用字段。');
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
  const [publishSubmitting, setPublishSubmitting] = useState(false);

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
    () => collectQualityWarningsFromRaw(data?.rawData),
    [data?.rawData],
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
      const [pubs, prov, shops] = await Promise.all([
        listProductPublications(id),
        queryPlatformProviders(),
        queryShops({ page: 1, pageSize: 500, authStatus: 'authorized' }),
      ]);
      setPubRows(Array.isArray(pubs.list) ? pubs.list : []);
      setPlatformsMeta(Array.isArray(prov.list) ? prov.list : []);
      setShopsList(Array.isArray(shops.list) ? shops.list : []);
    } catch {
      setPubRows([]);
    } finally {
      setPubCtxLoading(false);
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
  }, [draftTabKey, id, publishForm, refreshPublishReadiness]);

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
      <PageContainer title="商品详情">
        <Typography.Text type="danger">无效的商品 ID</Typography.Text>
      </PageContainer>
    );
  }

  return (
    <PageContainer
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
                  {isPinduoduoProduct(data) ? <ProductCollectQualityAlert product={data} /> : null}
                  {showCustomIncompleteHint ? (
                    <Alert
                      type="info"
                      showIcon
                      style={{ marginBottom: 16 }}
                      message="该商品来自自定义链接采集，部分字段可能需要人工补充。建议检查标题、价格、图片和 SKU 后再发布。"
                    />
                  ) : null}
                  {!isPinduoduoProduct(data) && collectQualityWarnings.length > 0 ? (
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
                          title: 'Tokens',
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
                    <Card title="原始数据（高级）" variant="borderless">
                      <pre style={{ maxHeight: 360, overflow: 'auto', fontSize: 12 }}>{JSON.stringify(data.rawData, null, 2)}</pre>
                    </Card>
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
                          仅当平台已开放「库存同步」且映射完整时可发起同步（TikTok、Shopee、Lazada、Amazon 已支持）。
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
                          return { disabled: missing || !ok };
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
                            const sku = data.skus?.find((s) => s.id === r.productSkuId);
                            const fallback = typeof sku?.stock === 'number' ? sku.stock : 0;
                            const suggested =
                              typeof r.platformStock === 'number' ? r.platformStock : fallback;
                            const btn = (
                              <Button
                                type="link"
                                size="small"
                                disabled={!ok}
                                style={{ padding: 0 }}
                                onClick={() => {
                                  if (!ok) return;
                                  setSyncRow(r);
                                  syncForm.setFieldsValue({ stock: suggested });
                                  setSyncOpen(true);
                                }}
                              >
                                同步库存
                              </Button>
                            );
                            return ok ? btn : (
                              <Tooltip title="当前平台未开放库存同步、店铺未授权，或该映射行不可用">
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
                        options={['tiktok', 'shopee', 'lazada', 'amazon', 'mock'].map((p) => ({
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
                          defaultActiveKey={['product', 'sku', 'image', 'inventory', 'platform']}
                          items={['product', 'sku', 'image', 'inventory', 'platform'].map((g) => ({
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
                                    render: (_: unknown, row: ReadinessCheckItem) => (
                                      <Tag color={row.level === 'error' ? 'red' : 'orange'}>{row.level}</Tag>
                                    ),
                                  },
                                  { title: '说明', dataIndex: 'message', ellipsis: true },
                                  { title: '代码', dataIndex: 'code', width: 200, ellipsis: true },
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
                            <Typography.Text code>product_publish</Typography.Text>{' '}
                            为「可用」的店铺（如 mock）或「测试中 / beta」（如 TikTok Shop、Shopee、Lazada、Amazon）可提交刊登任务。请到{' '}
                            <Link to="/settings/platform-publish">设置 · 平台刊登预设</Link> 补齐对应平台预设（如 TikTok{' '}
                            <Typography.Text code>platform_publish_tiktok</Typography.Text>、Shopee{' '}
                            <Typography.Text code>platform_publish_shopee</Typography.Text>、Lazada{' '}
                            <Typography.Text code>platform_publish_lazada</Typography.Text>
                            、Amazon <Typography.Text code>platform_publish_amazon</Typography.Text>
                            的类目、品牌、制造商、包裹重量尺寸、配送选项等）；队列见 <Link to="/product/publish-tasks">刊登任务</Link>。
                          </>
                        }
                      />
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
                                {publishReadiness.checks.slice(0, 5).map((c, i) => (
                                  <div key={`${c.code}-${i}`} style={{ marginBottom: 4 }}>
                                    <Tag color={c.level === 'error' ? 'red' : 'orange'}>{c.level}</Tag> {c.message}
                                  </div>
                                ))}
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
                                content: (
                                  <div>
                                    {r.checks.map((c, i) => (
                                      <div key={`${c.code}-${i}`} style={{ marginBottom: 6 }}>
                                        <Tag color={c.level === 'error' ? 'red' : 'orange'}>{c.level}</Tag> {c.message}
                                      </div>
                                    ))}
                                  </div>
                                ),
                              });
                              return;
                            }
                            const task = await publishProduct(id, { shopId, options: {} });
                            if (task.readiness) setPublishReadiness(task.readiness);
                            message.success('已提交刊登任务');
                            publishForm.resetFields();
                            setPublishReadiness(null);
                            await reloadPublishContext();
                          } catch (e: unknown) {
                            const ex = e as Error & { data?: unknown };
                            if (ex.message === 'product readiness check failed' && ex.data && typeof ex.data === 'object') {
                              const r = ex.data as ProductReadinessResult;
                              setPublishReadiness(r);
                              Modal.error({
                                title: '发布检查未通过',
                                width: 600,
                                content: (
                                  <div>
                                    {(r.checks || []).map((c, i) => (
                                      <div key={`${c.code}-${i}`} style={{ marginBottom: 6 }}>
                                        <Tag color={c.level === 'error' ? 'red' : 'orange'}>{c.level}</Tag> {c.message}
                                      </div>
                                    ))}
                                  </div>
                                ),
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
            <Input placeholder="en" />
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
            <Input placeholder="en" />
          </Form.Item>
          <Form.Item name="platform" label="平台" rules={[{ required: true }]}>
            <Input placeholder="TikTok Shop" />
          </Form.Item>
          <Form.Item name="tone" label="语气" rules={[{ required: true }]}>
            <Input placeholder="professional" />
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
        {(syncRow?.platform || '').trim().toLowerCase() === 'tiktok' ? (
          <Typography.Paragraph type="secondary" style={{ marginTop: 0, marginBottom: 12 }}>
            TikTok 会使用「设置 → 平台刊登配置 → TikTok Shop」中的默认仓库 ID（也可在接口层通过任务{' '}
            <Typography.Text code>options.warehouse_id</Typography.Text> 覆盖）。若推送失败并提示权限不足，请在 TikTok Shop
            Partner Center 申请库存更新相关权限后重新授权店铺。
          </Typography.Paragraph>
        ) : null}
        {(syncRow?.platform || '').trim().toLowerCase() === 'shopee' ? (
          <Typography.Paragraph type="secondary" style={{ marginTop: 0, marginBottom: 12 }}>
            Shopee 默认使用 <Typography.Text code>normal_stock</Typography.Text> 更新；若你的卖家中心要求按仓/位置维护库存，请在「设置
            → 平台刊登配置 → Shopee」填写默认仓库 ID（对应 Open API{' '}
            <Typography.Text code>seller_stock[].location_id</Typography.Text>），或通过任务{' '}
            <Typography.Text code>options.warehouse_id</Typography.Text> /
            <Typography.Text code>location_id</Typography.Text>{' '}
            覆盖。若推送失败并提示权限不足，请在 Shopee Open Platform 申请库存/商品更新相关权限后重新授权店铺。
          </Typography.Paragraph>
        ) : null}
        {(syncRow?.platform || '').trim().toLowerCase() === 'lazada' ? (
          <Typography.Paragraph type="secondary" style={{ marginTop: 0, marginBottom: 12 }}>
            Lazada 通过 Open Platform 的 <Typography.Text code>price_quantity</Typography.Text> 接口更新数量；多仓或平台要求指定{' '}
            <Typography.Text code>WarehouseCode</Typography.Text> 时，请在「设置 → 平台刊登配置 → Lazada」填写默认{' '}
            <Typography.Text code>warehouse_id</Typography.Text>（仓库代码），或通过任务{' '}
            <Typography.Text code>options.warehouse_id</Typography.Text> 覆盖。若推送失败并提示权限不足，请在 Lazada Open Platform /
            Seller Center 申请库存 / 商品更新相关权限后重新授权店铺。
          </Typography.Paragraph>
        ) : null}
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
    </PageContainer>
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
