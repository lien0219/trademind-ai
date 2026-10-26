import { tr } from '@/i18n';
import {
  DollarOutlined,
  MoreOutlined,
  PictureOutlined,
  PlusOutlined,
  RobotOutlined,
  SafetyCertificateOutlined,
  ShopOutlined,
} from '@ant-design/icons';
import { ModalForm, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import DouyinE2EPrecheckBanner from '@/components/platform/DouyinE2EPrecheckBanner';
import { EmptyState, OperationToolbar, TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import type { ActionType, ProColumns, ProFormInstance } from '@ant-design/pro-components';
import { formatDateTime } from '@/utils/formatTime';
import { isProductionBuild } from '@/utils/runtimeEnvironment';

import {
  Alert,
  Button,
  Checkbox,
  Drawer,
  Dropdown,
  Form,
  Image,
  Input,
  InputNumber,
  Progress,
  Radio,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
  type MenuProps,
} from 'antd';
import { useRef, useState, useMemo, useEffect } from 'react';
import { history, useLocation } from '@umijs/max';
import { PAGE_COPY } from '@/constants/copywriting';
import {
  AI_LANGUAGE_OPTIONS,
  AI_TARGET_PLATFORM_OPTIONS,
  AI_TONE_OPTIONS,
} from '@/constants/aiPrompts';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import {
  PUBLISH_BATCH_LIMIT_MESSAGE,
  PUBLISH_BATCH_MAX_PRODUCTS,
} from '@/constants/publishLimits';
import { PRODUCT_STATUS } from '@/constants/status';
import { PRODUCT_SOURCE_LABEL, productSourceLabel } from '@/constants/userFriendly';
import { createProductImagesBatch, createProductTextBatch } from '@/services/aiBatches';
import { createProduct, fetchProducts, type ProductListRow } from '@/services/products';
import { batchCheckProductReadiness, type ProductReadinessResult } from '@/services/productReadiness';
import { queryShops, type ShopListRow } from '@/services/shops';
import PricingApplyModal from '@/components/PricingApplyModal';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { useKeywordSearchField } from '@/hooks/useKeywordSearchField';
import KeywordSafetyHint from '@/components/common/KeywordSafetyHint';
import { normalizeSource, parsePositiveInt } from '@/utils/urlState';
import './index.less';

const DRAFT_QUERY_KEYS = [
  'page',
  'pageSize',
  'keyword',
  'status',
  'platform',
  'shopId',
  'publishStatus',
  'aiStatus',
  'source',
  'missingAiTitle',
  'missingAiDescription',
  'readiness',
  'publishable',
] as const;

function readDraftLegacyFilters(search: string) {
  const sp = new URLSearchParams(search);
  const aiStatus = sp.get('aiStatus')?.trim();
  const publishStatus = sp.get('publishStatus')?.trim();
  return {
    missingAiTitle:
      sp.get('missingAiTitle') === '1' || aiStatus === 'missing_title',
    missingAiDescription:
      sp.get('missingAiDescription') === '1' || aiStatus === 'missing_description',
    readinessBlocked: sp.get('readiness') === 'blocked' || publishStatus === 'blocked',
    publishable: sp.get('publishable') === '1' || publishStatus === 'publishable',
    status:
      sp.get('status')?.trim() ||
      (publishStatus === 'published' ? 'published' : undefined),
    keyword: sp.get('keyword')?.trim() || undefined,
    platform: sp.get('platform')?.trim() || undefined,
    shopId: sp.get('shopId')?.trim() || undefined,
    navSource: normalizeSource(sp.get('source') || undefined),
  };
}

const OPERATION_STEP_OPTIONS = [
  { label: '全部', value: '' },
  { label: '待检查采集结果', value: 'collect_review' },
  { label: '待优化标题', value: 'title' },
  { label: '待生成描述', value: 'description' },
  { label: '待处理图片', value: 'images' },
  { label: '待设置价格', value: 'pricing' },
  { label: '发布检查未通过', value: 'publish_check' },
  { label: '可以生成刊登草稿', value: 'ready' },
];

function operationStepColor(step?: string) {
  if (step === 'ready') return 'green';
  if (step === 'publish_check') return 'orange';
  if (step === 'pricing' || step === 'images') return 'gold';
  return 'blue';
}

export default function ProductDraftsPage() {
  const location = useLocation();
  const { state: urlState, setState: setUrlState, clearState: clearUrlState } =
    useUrlQueryState<Record<(typeof DRAFT_QUERY_KEYS)[number], string | undefined>>(DRAFT_QUERY_KEYS);
  const urlFilters = useMemo(() => readDraftLegacyFilters(location.search), [location.search]);
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(20);

  const emptyLocale = useListEmptyLocale('productDrafts', { permissionScoped: true });

  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const {
    fieldProps: keywordFieldProps,
    prepareKeyword,
    showSensitiveHint,
  } = useKeywordSearchField({
    setUrlState,
    formRef,
    actionRef,
    setTablePage,
  });
  const [createOpen, setCreateOpen] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
  const [selectedRows, setSelectedRows] = useState<ProductListRow[]>([]);
  const [batchOpen, setBatchOpen] = useState(false);
  const [batchLoading, setBatchLoading] = useState(false);
  const [batchPlat, setBatchPlat] = useState<string>('tiktok');
  const [batchShopId, setBatchShopId] = useState<string>('');
  const [batchResult, setBatchResult] = useState<ProductReadinessResult[]>([]);
  const [shopsList, setShopsList] = useState<ShopListRow[]>([]);
  const [listFilters, setListFilters] = useState<{ keyword?: string; status?: string; source?: string; operationStep?: string }>({});
  const [bulkOpen, setBulkOpen] = useState(false);
  const [bulkLoading, setBulkLoading] = useState(false);
  const [bulkForm] = Form.useForm();
  const [bulkOp, setBulkOp] = useState<string>('title_optimize');
  const [bulkConfirmFiltered, setBulkConfirmFiltered] = useState(false);
  const [pricingBatchOpen, setPricingBatchOpen] = useState(false);
  const [listLoadError, setListLoadError] = useState<string>();

  const selectedCount = selectedRowKeys.length;
  const selectedScopeText =
    selectedRows.length === selectedCount
      ? '当前选择将用于 AI、发布检查、刊登和定价操作'
      : '当前选择已保留，刷新后仍按商品 ID 执行';

  const clearSelection = () => {
    setSelectedRowKeys([]);
    setSelectedRows([]);
  };

  const ensureBatchSelection = () => {
    if (selectedRowKeys.length === 0) {
      message.warning('请先勾选商品');
      return false;
    }
    if (selectedRowKeys.length > PUBLISH_BATCH_MAX_PRODUCTS) {
      message.error(PUBLISH_BATCH_LIMIT_MESSAGE);
      return false;
    }
    return true;
  };

  const openLegacyBulkAI = () => {
    bulkForm.resetFields();
    bulkForm.setFieldsValue({
      language: 'en',
      platform: 'TikTok Shop',
      maxLength: 120,
      tone: 'professional',
      applyMode: 'save_ai_field',
      provider: 'removebg',
    });
    setBulkOp('title_optimize');
    setBulkConfirmFiltered(false);
    setBulkOpen(true);
  };

  const moreActionItems: MenuProps['items'] = [
    {
      key: 'pricing',
      icon: <DollarOutlined />,
      label: '批量设置发布价',
    },
    ...(!isProductionBuild
      ? [
          {
            key: 'legacyBulkAi',
            icon: <RobotOutlined />,
            label: '开发版批量 AI',
          },
        ]
      : []),
  ];

  const onMoreActionClick: MenuProps['onClick'] = ({ key }) => {
    if (key === 'pricing') {
      setPricingBatchOpen(true);
      return;
    }
    if (key === 'legacyBulkAi') {
      openLegacyBulkAI();
    }
  };

  const hasActiveDraftFilters = Boolean(
    listFilters.keyword ||
      listFilters.status ||
      listFilters.source ||
      listFilters.operationStep ||
      urlState.keyword ||
      urlState.status ||
      urlState.source ||
      urlFilters.missingAiTitle ||
      urlFilters.missingAiDescription ||
      urlFilters.readinessBlocked ||
      urlFilters.publishable ||
      urlFilters.status ||
      urlFilters.navSource,
  );

  const draftTableLocale = useMemo(() => {
    if (listLoadError) {
      return {
        emptyText: (
          <EmptyState
            title="商品草稿加载失败"
            description={`列表数据暂时无法获取：${listLoadError}`}
            actionLabel="重试加载"
            onAction={() => actionRef.current?.reload()}
          />
        ),
      };
    }
    if (hasActiveDraftFilters) {
      return {
        emptyText: (
          <EmptyState
            title="没有匹配的商品草稿"
            description="当前筛选条件下没有商品草稿，可以调整关键字、状态或来源后重新查询。"
            actionLabel="重置筛选"
            onAction={() => {
              setTablePage(1);
              setTablePageSize(20);
              formRef.current?.resetFields?.();
              clearUrlState(DRAFT_QUERY_KEYS, { replace: true });
              actionRef.current?.reload();
            }}
          />
        ),
      };
    }
    return emptyLocale;
  }, [clearUrlState, emptyLocale, hasActiveDraftFilters, listLoadError]);

  useEffect(() => {
    setTablePage(parsePositiveInt(urlState.page, 1));
    setTablePageSize(parsePositiveInt(urlState.pageSize, 20));
    formRef.current?.setFieldsValue?.({
      keyword: urlState.keyword || urlFilters.keyword,
      status: urlState.status || urlFilters.status,
    });
    actionRef.current?.reload();
  }, [
    location.search,
    urlFilters.keyword,
    urlFilters.status,
    urlState.keyword,
    urlState.page,
    urlState.pageSize,
    urlState.status,
  ]);

  const columns: ProColumns<ProductListRow>[] = useMemo(
    () => [
    {
      title: '商品图',
      dataIndex: 'coverUrl',
      width: 96,
      fixed: 'left',
      search: false,
      render: (_, row) =>
        row.coverUrl ? (
          <Image
            src={row.coverUrl}
            width={56}
            height={56}
            className="product-drafts-table__image"
          />
        ) : (
          <div className="product-drafts-table__image-placeholder">无图</div>
        ),
    },
    {
      title: '标题',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '搜索标题', ...keywordFieldProps },
      search: {
        transform: (v) => ({ keyword: v }),
      },
    },
    {
      title: '标题',
      dataIndex: 'title',
      width: 320,
      ellipsis: true,
      search: false,
      render: (_, row) => (
        <div className="product-drafts-table__title-cell">
          <Tooltip title={row.title}>
            <Typography.Link
              className="product-drafts-table__title"
              href={`/product/drafts/${row.id}`}
              onClick={(event) => event.stopPropagation()}
            >
              {row.title || '未命名商品'}
            </Typography.Link>
          </Tooltip>
          <Space size={8} wrap className="product-drafts-table__meta">
            <Typography.Text type="secondary">ID {row.id}</Typography.Text>
            {row.updatedAt ? (
              <Typography.Text type="secondary">更新 {formatDateTime(row.updatedAt)}</Typography.Text>
            ) : null}
          </Space>
        </div>
      ),
    },
    {
      title: '来源',
      dataIndex: 'source',
      width: 110,
      ellipsis: true,
      valueType: 'select',
      valueEnum: Object.fromEntries(
        Object.entries(PRODUCT_SOURCE_LABEL).map(([k, v]) => [k, { text: v }]),
      ),
      render: (_, row) => {
        const label = productSourceLabel(row.source);
        return (
          <Tag
            className="product-drafts-table__source-tag"
            title={label !== row.source ? row.source : undefined}
          >
            {label}
          </Tag>
        );
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 112,
      valueType: 'select',
      valueEnum: Object.fromEntries(
        Object.entries(PRODUCT_STATUS).map(([k, v]) => [k, { text: v.text }]),
      ),
      render: (_, row) => {
        const m = PRODUCT_STATUS[row.status as keyof typeof PRODUCT_STATUS];
        return <Tag color={m?.color}>{(m?.text ?? row.status) || '未知'}</Tag>;
      },
    },
    {
      title: '运营进度',
      dataIndex: 'operationStep',
      width: 220,
      valueType: 'select',
      fieldProps: {
        options: OPERATION_STEP_OPTIONS,
      },
      search: {
        transform: (v) => ({ operationStep: v }),
      },
      render: (_, row) => {
        const p = row.operationProgress;
        if (!p) return <Typography.Text type="secondary">—</Typography.Text>;
        return (
          <Space direction="vertical" size={4} className="product-drafts-progress">
            <Progress percent={p.completionPercent ?? 0} size="small" showInfo={false} />
            <Space size={6} wrap>
              <Typography.Text>{p.completionPercent ?? 0}%</Typography.Text>
              <Tag color={operationStepColor(p.currentStep)}>{p.currentStepLabel || '继续完善'}</Tag>
            </Space>
            {(p.blockerCount || p.warningCount) ? (
              <Typography.Text type="secondary" className="product-drafts-progress__issues">
                待处理 {p.blockerCount ?? 0}，建议检查 {p.warningCount ?? 0}
              </Typography.Text>
            ) : null}
          </Space>
        );
      },
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 168,
      search: false,
      valueType: 'dateTime',
      render: (_, row) => formatDateTime(row.createdAt),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 132,
      fixed: 'right',
      render: (_, row) => [
        <Typography.Link
          key="detail"
          href={row.operationProgress?.nextActionUrl || `/product/drafts/${row.id}`}
          onClick={(event) => event.stopPropagation()}
        >
          {row.operationProgress?.nextActionLabel || '继续完善'}
        </Typography.Link>,
      ],
    },
  ],
    [keywordFieldProps],
  );

  const eligibleBatchPlatforms = [
    'tiktok',
    'shopee',
    'lazada',
    'amazon',
    ...(!isProductionBuild ? ['mock'] : []),
  ];

  const shopsForBatchPlat = shopsList.filter(
    (s) =>
      (s.platform || '').toLowerCase() === batchPlat.toLowerCase() && s.authStatus === 'authorized',
  );

  const openBatchDrawer = async () => {
    if (!ensureBatchSelection()) {
      return;
    }
    setBatchOpen(true);
    setBatchResult([]);
    try {
      const shops = await queryShops({ page: 1, pageSize: 500, authStatus: 'authorized' });
      setShopsList(Array.isArray(shops.list) ? shops.list : []);
    } catch {
      setShopsList([]);
    }
  };

  const runBatchReadiness = async () => {
    if (!batchShopId) {
      message.error('请选择店铺');
      return;
    }
    setBatchLoading(true);
    try {
      const { list } = await batchCheckProductReadiness({
        productIds: selectedRowKeys,
        platform: batchPlat,
        shopId: batchShopId,
      });
      setBatchResult(Array.isArray(list) ? list : []);
      message.success('检查完成');
    } catch (e: unknown) {
      message.error((e as Error)?.message || '检查失败');
    } finally {
      setBatchLoading(false);
    }
  };

  const submitBulkAI = async () => {
    try {
      const vals = await bulkForm.validateFields();
      const productIds = [...selectedRowKeys];
      if (productIds.length === 0 && !bulkConfirmFiltered) {
        message.error('未勾选商品时，请勾选「按当前筛选」确认项');
        return;
      }
      const narrow = !!(listFilters.keyword || listFilters.status || listFilters.source);
      if (productIds.length === 0 && !narrow) {
        message.error('当前无任何列表筛选；若需全表批量，请先在列表中设置状态/来源/关键字筛选，或勾选商品。');
        return;
      }
      setBulkLoading(true);
      const filtBase =
        productIds.length === 0
          ? {
              keyword: listFilters.keyword ?? '',
              status: listFilters.status ?? '',
              source: listFilters.source ?? '',
            }
          : {};
      if (
        bulkOp === 'title_optimize' ||
        bulkOp === 'description_generate'
      ) {
        await createProductTextBatch({
          operationType: bulkOp,
          productIds,
          filters: filtBase,
          options: {
            language: vals.language,
            platform: vals.platform,
            maxLength: vals.maxLength,
            tone: vals.tone,
          },
          applyMode: vals.applyMode,
          confirmAll: productIds.length === 0,
        });
      } else {
        await createProductImagesBatch({
          operationType: bulkOp,
          productIds,
          filters: {
            ...filtBase,
            onlyHasMainImage: true,
          },
          options: {
            provider: vals.provider,
            prompt: vals.prompt,
            backgroundPrompt: vals.backgroundPrompt,
            style: vals.style,
          },
          confirmAll: productIds.length === 0,
        });
      }
      message.success('批次已创建');
      setBulkOpen(false);
      history.push('/ai/batches');
      actionRef.current?.reload();
    } catch (e: unknown) {
      if ((e as { errorFields?: unknown })?.errorFields) return;
      message.error((e as Error)?.message || '创建失败');
    } finally {
      setBulkLoading(false);
    }
  };

  return (
    <TmPageContainer
      className="product-drafts-page"
      title={PAGE_COPY.productDrafts.title}
      subTitle={PAGE_COPY.productDrafts.description}
    >
      <OperationToolbar className="product-drafts-page__toolbar">
        <Button icon={<PlusOutlined />} type="primary" onClick={() => setCreateOpen(true)}>
          新建草稿
        </Button>
        <Dropdown
          menu={{ items: moreActionItems, onClick: onMoreActionClick }}
          trigger={['click']}
        >
          <Button icon={<MoreOutlined />}>更多</Button>
        </Dropdown>
      </OperationToolbar>
      <DouyinE2EPrecheckBanner blockedByCredentials compact />
      <KeywordSafetyHint visible={showSensitiveHint} />
      {selectedCount > 0 ? (
        <OperationToolbar
          className="product-drafts-page__selection-toolbar"
          extra={
            <Button type="link" onClick={clearSelection}>
              清空选择
            </Button>
          }
        >
          <div className="product-drafts-page__selection-summary">
            <Typography.Text strong>已选择 {selectedCount} 个商品</Typography.Text>
            <Typography.Text type="secondary">{selectedScopeText}</Typography.Text>
          </div>
          <Button
            icon={<RobotOutlined />}
            type="primary"
            onClick={() => {
              if (!ensureBatchSelection()) return;
              history.push(`/product/ai-text-batch?productIds=${selectedRowKeys.join(',')}`);
            }}
          >
            批量 AI 优化
          </Button>
          <Button
            icon={<PictureOutlined />}
            onClick={() => {
              if (!ensureBatchSelection()) return;
              history.push(`/product/ai-image-batch?productIds=${selectedRowKeys.join(',')}`);
            }}
          >
            批量 AI 图片处理
          </Button>
          <Button
            icon={<SafetyCertificateOutlined />}
            onClick={() => void openBatchDrawer()}
          >
            批量发布检查
          </Button>
          <Button
            icon={<ShopOutlined />}
            onClick={() => {
              if (!ensureBatchSelection()) return;
              history.push(`/product/publish-batch?productIds=${selectedRowKeys.join(',')}`);
            }}
          >
            批量创建刊登草稿
          </Button>
          <Button icon={<DollarOutlined />} onClick={() => setPricingBatchOpen(true)}>
            批量设置发布价
          </Button>
        </OperationToolbar>
      ) : null}
      <ProTable<ProductListRow>
        className="product-drafts-table"
        rowKey="id"
        locale={draftTableLocale}
        actionRef={actionRef}
        formRef={formRef}
        params={{
          current: tablePage,
          pageSize: tablePageSize,
          keyword: urlState.keyword || urlFilters.keyword,
          status: urlState.status || urlFilters.status,
          platform: urlState.platform || urlFilters.platform,
          shopId: urlState.shopId || urlFilters.shopId,
          source: urlState.source || urlFilters.navSource,
        }}
        onReset={() => {
          setTablePage(1);
          setTablePageSize(20);
          clearUrlState(DRAFT_QUERY_KEYS, { replace: true });
        }}
        rowSelection={{
          type: 'checkbox',
          selectedRowKeys,
          onChange: (keys, rows) => {
            setSelectedRowKeys(keys as string[]);
            setSelectedRows(rows);
          },
          getCheckboxProps: (row) => ({
            disabled: row.status === 'archived' || row.status === 'deleted',
            title:
              row.status === 'archived' || row.status === 'deleted'
                ? '已归档或已删除的商品不能批量刊登'
                : undefined,
          }),
        }}
        tableAlertRender={false}
        columns={columns}
        search={{ labelWidth: 'auto' }}
        scroll={{ x: 980 }}
        pagination={{
          current: tablePage,
          pageSize: tablePageSize,
          showSizeChanger: true,
          onChange: (page, pageSize) => {
            setTablePage(page);
            setTablePageSize(pageSize);
            setUrlState({
              page: page > 1 ? page : undefined,
              pageSize: pageSize !== 20 ? pageSize : undefined,
            });
          },
        }}
        options={{ reload: true, density: true, setting: true }}
        headerTitle="商品草稿列表"
        request={async (params) => {
          setListFilters({
            keyword: params.keyword as string | undefined,
            status: params.status as string | undefined,
            source: params.source as string | undefined,
            operationStep: params.operationStep as string | undefined,
          });
          const qp = {
            page: params.current ?? tablePage,
            pageSize: params.pageSize ?? tablePageSize,
            keyword: prepareKeyword(params.keyword),
            status: urlFilters.status || (params.status as string | undefined)?.trim(),
            source: (params.source as string | undefined)?.trim(),
            operationStep: (params.operationStep as string | undefined)?.trim(),
          };
          const nextAiStatus = urlFilters.missingAiTitle
            ? 'missing_title'
            : urlFilters.missingAiDescription
              ? 'missing_description'
              : undefined;
          const nextPublishStatus = urlFilters.readinessBlocked
            ? 'blocked'
            : urlFilters.publishable
              ? 'publishable'
              : qp.status === 'published'
                ? 'published'
                : undefined;
          setUrlState(
            {
              page: Number(qp.page) > 1 ? qp.page : undefined,
              pageSize: Number(qp.pageSize) !== 20 ? qp.pageSize : undefined,
              keyword: qp.keyword,
              status: qp.status,
              platform: urlState.platform || urlFilters.platform,
              shopId: urlState.shopId || urlFilters.shopId,
              publishStatus: nextPublishStatus,
              aiStatus: nextAiStatus,
              source: qp.source || urlFilters.navSource || undefined,
              missingAiTitle: urlFilters.missingAiTitle ? '1' : undefined,
              missingAiDescription: urlFilters.missingAiDescription ? '1' : undefined,
              readiness: urlFilters.readinessBlocked ? 'blocked' : undefined,
              publishable: urlFilters.publishable ? '1' : undefined,
            },
            { replace: true },
          );
          try {
            const res = await fetchProducts({
              page: qp.page,
              pageSize: qp.pageSize,
              status: qp.status,
              source: qp.source,
              keyword: qp.keyword,
              operationStep: qp.operationStep,
              missingAiTitle: urlFilters.missingAiTitle || undefined,
              missingAiDescription: urlFilters.missingAiDescription || undefined,
              readinessBlocked: urlFilters.readinessBlocked || undefined,
              publishable: urlFilters.publishable || undefined,
            });
            setListLoadError(undefined);
            return {
              data: res.list,
              success: true,
              total: res.pagination.total,
            };
          } catch (e: unknown) {
            setListLoadError((e as Error)?.message || '列表请求失败');
            return {
              data: [],
              success: false,
              total: 0,
            };
          }
        }}
      />

      <ModalForm
        title="新建商品草稿"
        open={createOpen}
        modalProps={{
          destroyOnHidden: true,
          width: 640,
          className: 'product-drafts-create-modal',
          onCancel: () => setCreateOpen(false),
        }}
        onFinish={async (vals) => {
          await createProduct({
            title: vals.title,
            source: vals.source || 'manual',
            sourceUrl: vals.sourceUrl,
            description: vals.description,
          });
          setCreateOpen(false);
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormText name="title" label="标题" rules={[{ required: true, message: '必填' }]} />
        <ProFormText name="source" label="来源" initialValue="manual" />
        <ProFormText name="sourceUrl" label="来源链接" />
        <ProFormTextArea name="description" label="描述" fieldProps={{ rows: 3 }} />
      </ModalForm>

      <Drawer
        title="批量发布检查"
        width="min(720px, calc(100vw - 32px))"
        className="product-drafts-drawer"
        open={batchOpen}
        onClose={() => setBatchOpen(false)}
        destroyOnHidden
        extra={
          <Button type="primary" loading={batchLoading} onClick={() => void runBatchReadiness()}>
            开始检查
          </Button>
        }
      >
        <Space direction="vertical" className="product-drafts-drawer__body" size="large">
          <Form layout="vertical">
            <Form.Item label="平台">
              <Select
                value={batchPlat}
                onChange={(v) => {
                  setBatchPlat(String(v));
                  setBatchShopId('');
                }}
                options={eligibleBatchPlatforms.map((p) => ({ label: p, value: p }))}
              />
            </Form.Item>
            <Form.Item label={tr("店铺")}>
              <Select
                placeholder="选择已授权店铺"
                value={batchShopId || undefined}
                onChange={(v) => setBatchShopId(v ? String(v) : '')}
                options={shopsForBatchPlat.map((s) => ({
                  label: `${s.shopName} (${s.platform})`,
                  value: s.id,
                }))}
                showSearch
                optionFilterProp="label"
              />
            </Form.Item>
          </Form>
          <Typography.Paragraph type="secondary" className="product-drafts-drawer__hint">
            已选 {selectedRowKeys.length} 个商品；单次最多 100 个。检查不修改商品数据，不调用平台 API。
          </Typography.Paragraph>
          <Table<ProductReadinessResult>
            size="small"
            rowKey="productId"
            dataSource={batchResult}
            pagination={false}
            columns={[
              {
                title: '商品 ID',
                dataIndex: 'productId',
                ellipsis: true,
                render: (v: string) => (
                  <Typography.Link href={`/product/drafts/${v}?tab=readiness`}>{v}</Typography.Link>
                ),
              },
              {
                title: '状态',
                width: 100,
                render: (_, r) => {
                  if (!r.canPublish) return <Tag color="red">阻止</Tag>;
                  if (r.warningCount > 0) return <Tag color="orange">警告</Tag>;
                  return <Tag color="green">就绪</Tag>;
                },
              },
              { title: '分', dataIndex: 'score', width: 64 },
              { title: '错', dataIndex: 'errorCount', width: 56 },
              { title: '警', dataIndex: 'warningCount', width: 56 },
            ]}
          />
        </Space>
      </Drawer>

      {!isProductionBuild ? (
        <Drawer
          title="开发版批量 AI（商品草稿）"
          width="min(640px, calc(100vw - 32px))"
          className="product-drafts-drawer"
          open={bulkOpen}
          onClose={() => setBulkOpen(false)}
          destroyOnHidden
          extra={
            <Button
              type="primary"
              loading={bulkLoading}
              onClick={() => void submitBulkAI()}
            >
              创建批次
            </Button>
          }
        >
          <Alert
            type="info"
            showIcon
            className="product-drafts-drawer__alert"
            message="该入口仅用于开发环境兼容验证。正式批量任务请使用「批量 AI 优化」或「批量 AI 图片处理」。"
          />
          <Typography.Paragraph type="secondary">
            已勾选 <strong>{selectedRowKeys.length}</strong> 个商品。
            {selectedRowKeys.length === 0 ? (
              <>未勾选时将使用下方「与列表相同的筛选条件」；必须勾选确认项。</>
            ) : null}
          </Typography.Paragraph>
          <Form form={bulkForm} layout="vertical">
          <Form.Item label="操作类型" required>
            <Radio.Group
              value={bulkOp}
              onChange={(e) => setBulkOp(e.target.value)}
              options={[
                { label: 'AI 标题优化', value: 'title_optimize' },
                { label: 'AI 描述生成', value: 'description_generate' },
                { label: '去背景（主图）', value: 'image_remove_background' },
                { label: '生成场景图（主图）', value: 'image_generate_scene' },
                { label: '批量主图生成', value: 'image_batch_generate_main' },
                { label: '批量图片评分', value: 'image_score' },
                { label: '批量自动选主图', value: 'image_select_best_main' },
              ]}
            />
          </Form.Item>
          {(bulkOp === 'title_optimize' || bulkOp === 'description_generate') && (
            <>
              <Form.Item name="language" label={tr("语言")} rules={[{ required: true }]}>
                <Select options={AI_LANGUAGE_OPTIONS} placeholder="请选择生成语言" />
              </Form.Item>
              <Form.Item name="platform" label="平台口径" rules={[{ required: true }]}>
                <Select
                  options={AI_TARGET_PLATFORM_OPTIONS}
                  placeholder="请选择目标平台"
                  showSearch
                  optionFilterProp="label"
                />
              </Form.Item>
              {bulkOp === 'title_optimize' && (
                <Form.Item name="maxLength" label="最大长度">
                  <InputNumber min={20} max={300} className="product-drafts-form__full-control" />
                </Form.Item>
              )}
              <Form.Item name="tone" label="语气 / 风格">
                <Select options={AI_TONE_OPTIONS} placeholder="请选择文案语气" />
              </Form.Item>
              <Form.Item name="applyMode" label="应用策略" tooltip="成功后写入草稿中的 AI 优化标题 / AI 优化描述">
                <Select
                  options={[
                    { label: '仅任务记录（不写 AI 草稿字段）', value: 'task_only' },
                    { label: '写入 AI 优化标题 / AI 优化描述', value: 'save_ai_field' },
                  ]}
                />
              </Form.Item>
            </>
          )}
          {(bulkOp === 'image_remove_background' || bulkOp === 'image_generate_scene') && (
            <>
              <Form.Item
                name="provider"
                label="图片处理服务"
                tooltip={bulkOp === 'image_remove_background' ? '后端会强制 removebg' : '如 openai_image / comfyui'}
              >
                <Input placeholder={bulkOp === 'image_remove_background' ? 'removebg' : 'openai_image'} />
              </Form.Item>
              <Form.Item name="prompt" label="画面描述（可选，写入任务摘要）">
                <Input.TextArea rows={3} placeholder="场景/风格提示；勿在公开场合粘贴完整商业秘密" />
              </Form.Item>
            </>
          )}
          {selectedRowKeys.length === 0 && (
            <Form.Item>
              <Checkbox checked={bulkConfirmFiltered} onChange={(e) => setBulkConfirmFiltered(e.target.checked)}>
                我确认：对当前列表<strong>完全相同</strong>的筛选条件下的商品执行批量 AI（关键字 / 状态 / 来源）。
              </Checkbox>
            </Form.Item>
          )}
          </Form>
        </Drawer>
      ) : null}

      <PricingApplyModal
        open={pricingBatchOpen}
        onClose={() => setPricingBatchOpen(false)}
        mode="batch"
        productIds={selectedRowKeys.length > 0 ? selectedRowKeys : undefined}
        listFilters={selectedRowKeys.length === 0 ? listFilters : undefined}
        onApplied={() => actionRef.current?.reload()}
      />
    </TmPageContainer>
  );
}
