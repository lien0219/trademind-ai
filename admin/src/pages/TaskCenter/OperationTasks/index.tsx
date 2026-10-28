import { tr, trFormat } from '@/i18n';
import { EyeOutlined, FilterOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, Col, Form, Input, Modal, Row, Segmented, Select, Space, Typography, message } from 'antd';
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { ErrorAlert, OperationToolbar, SectionCard, TmPageContainer, TmProTable } from '@/components/ui';
import { PAGE_COPY } from '@/constants/copywriting';
import {
  OPERATION_PLATFORM_LABELS,
  OPERATION_TASK_STATUS_LABELS,
  OPERATION_TASK_TYPE_LABELS,
} from '@/constants/operationTasks';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { useLanguage } from '@/i18n/useLanguage';
import { usePermission } from '@/hooks/usePermission';
import { useUrlQueryState } from '@/hooks/useUrlState';
import {
  createOperationIdempotencyKey,
  createTask,
  extractOperationTaskAPIError,
  listTasks,
  type OperationTaskAPIError,
  type OperationTaskSummary,
} from '@/services/operationTasks';
import { fetchProducts, type ProductListRow } from '@/services/products';
import {
  getProductionRuntimeStatus,
  type ProductionRuntimeStatus,
} from '@/services/productionControl';
import { queryShops, type ShopListRow } from '@/services/shops';
import { formatDateTime } from '@/utils/formatTime';
import { PERMISSIONS } from '@/utils/permission';
import { parseProductionCreatePrefill } from '@/utils/urlState';
import {
  ProductionRuntimeBoundary,
  OperationAttemptStatusTag,
  OperationPriorityTag,
  OperationTaskStatusTag,
  copyableText,
  operationErrorMessage,
  platformLabel,
  taskTypeLabel,
} from './components/OperationTaskShared';
import './index.less';

const { Text } = Typography;

const QUERY_KEYS = ['status', 'platform', 'taskType', 'cursor', 'cursorHistory', 'create', 'productId', 'shopId'] as const;
const LIMIT = 20;
const MAX_CURSOR_HISTORY_ITEMS = 5;
const MAX_CURSOR_HISTORY_LENGTH = 4096;

type QueryState = Record<(typeof QUERY_KEYS)[number], string | undefined>;

function parseCursorHistory(raw?: string) {
  if (!raw || raw.length > MAX_CURSOR_HISTORY_LENGTH) return [];
  try {
    const value = JSON.parse(raw);
    if (!Array.isArray(value)) return [];
    return value
      .filter((cursor): cursor is string => typeof cursor === 'string' && cursor.length <= 2048)
      .slice(-MAX_CURSOR_HISTORY_ITEMS);
  } catch {
    return [];
  }
}

function serializeCursorHistory(cursors: string[]) {
  const next = cursors.slice(-MAX_CURSOR_HISTORY_ITEMS);
  while (next.length > 0 && JSON.stringify(next).length > MAX_CURSOR_HISTORY_LENGTH) next.shift();
  return next.length > 0 ? JSON.stringify(next) : undefined;
}

function optionsFromLabels(labels: Record<string, string | { source: string }>) {
  return Object.entries(labels).map(([value, label]) => ({
    value,
    label: tr(typeof label === 'string' ? label : label.source),
  }));
}

export default function OperationTasksPage() {
  useLanguage();
  const { can } = usePermission();
  const { state: urlState, setState: setUrlState, clearState, search } = useUrlQueryState<QueryState>(QUERY_KEYS);
  const [form] = Form.useForm();
  const [createForm] = Form.useForm();
  const [items, setItems] = useState<OperationTaskSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<OperationTaskAPIError | null>(null);
  const [showingStaleData, setShowingStaleData] = useState(false);
  const [dataQueryIdentity, setDataQueryIdentity] = useState('');
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [hasMore, setHasMore] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [createSubmitting, setCreateSubmitting] = useState(false);
  const [productionStatus, setProductionStatus] = useState<ProductionRuntimeStatus | null>(null);
  const [productionStatusLoading, setProductionStatusLoading] = useState(false);
  const [productionStatusError, setProductionStatusError] = useState<string>();
  const [productionResourcesLoading, setProductionResourcesLoading] = useState(false);
  const [productionResourcesLoaded, setProductionResourcesLoaded] = useState(false);
  const [products, setProducts] = useState<ProductListRow[]>([]);
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [createPayload, setCreatePayload] = useState('{}');
  const [createPayloadError, setCreatePayloadError] = useState<string | null>(null);
  const requestSeq = useRef(0);
  const createSubmittingRef = useRef(false);
  const createIdempotencyRef = useRef<{ fingerprint: string; key: string } | null>(null);
  const createFormInitializedRef = useRef(false);
  const createPrefillAppliedRef = useRef('');
  const itemsRef = useRef<OperationTaskSummary[]>([]);
  const dataQueryIdentityRef = useRef('');
  const createKind = Form.useWatch('createKind', createForm) as 'production' | 'local' | undefined;
  const canCreate = can(PERMISSIONS.OPERATION_TASK_EDIT);
  const productionCreatePrefill = useMemo(
    () => parseProductionCreatePrefill(urlState),
    [urlState.create, urlState.productId, urlState.shopId],
  );

  const queryIdentity = useMemo(() => JSON.stringify({
    status: urlState.status || '',
    platform: urlState.platform || '',
    taskType: urlState.taskType || '',
    cursor: urlState.cursor || '',
  }), [urlState.cursor, urlState.platform, urlState.status, urlState.taskType]);
  const cursorStack = useMemo(() => parseCursorHistory(urlState.cursorHistory), [urlState.cursorHistory]);
  const visibleItems = dataQueryIdentity === queryIdentity ? items : [];

  useEffect(() => {
    form.setFieldsValue({
      status: urlState.status,
      platform: urlState.platform,
      taskType: urlState.taskType,
    });
  }, [form, urlState.platform, urlState.status, urlState.taskType]);

  useLayoutEffect(() => {
    if (!createOpen) {
      createFormInitializedRef.current = false;
      return;
    }
    if (createFormInitializedRef.current) return;
    createFormInitializedRef.current = true;
    createForm.resetFields();
    createForm.setFieldsValue({ createKind: 'production', platform: 'local', priority: 'normal' });
  }, [createForm, createOpen]);

  useEffect(() => {
    if (!canCreate || !productionCreatePrefill || createOpen) return;
    setCreatePayload('{}');
    setCreatePayloadError(null);
    createIdempotencyRef.current = null;
    setCreateOpen(true);
  }, [canCreate, createOpen, productionCreatePrefill]);

  const loadProductionStatus = useCallback(async () => {
    setProductionStatusLoading(true);
    setProductionStatusError(undefined);
    try {
      setProductionStatus(await getProductionRuntimeStatus());
    } catch (nextError) {
      setProductionStatus(null);
      setProductionStatusError(operationErrorMessage(extractOperationTaskAPIError(nextError)));
    } finally {
      setProductionStatusLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadProductionStatus();
  }, [loadProductionStatus]);

  useEffect(() => {
    if (!createOpen) return;
    let active = true;
    setProductionResourcesLoaded(false);
    setProductionResourcesLoading(true);
    void Promise.all([
      fetchProducts({ page: 1, pageSize: 100 }),
      queryShops({ page: 1, pageSize: 100, platform: 'douyin_shop', status: 'active', authStatus: 'authorized' }),
      getProductionRuntimeStatus(),
    ]).then(([productPage, shopPage, status]) => {
      if (!active) return;
      setProducts(productPage.list || []);
      setShops(shopPage.list || []);
      setProductionStatus(status);
      setProductionStatusError(undefined);
    }).catch((nextError) => {
      if (!active) return;
      setProductionStatus(null);
      setProductionStatusError(operationErrorMessage(extractOperationTaskAPIError(nextError)));
    }).finally(() => {
      if (active) {
        setProductionResourcesLoading(false);
        setProductionResourcesLoaded(true);
      }
    });
    return () => {
      active = false;
    };
  }, [createOpen]);

  useEffect(() => {
    if (!createOpen || productionResourcesLoading || !productionResourcesLoaded || !productionCreatePrefill) return;
    const product = products.find((item) => item.id === productionCreatePrefill.productId);
    const allowlistedShopId = productionStatus?.allowlist?.enabled ? productionStatus.allowlist.shopId : undefined;
    const requestedShopId = productionCreatePrefill.shopId || allowlistedShopId;
    const shop = shops.find((item) => item.id === requestedShopId);
    const fingerprint = `${productionCreatePrefill.productId}:${requestedShopId || ''}`;
    if (!product || !shop || !allowlistedShopId || shop.id !== allowlistedShopId) {
      message.warning(tr("链接中的商品或抖店不在当前可用范围，请从运营任务中心重新选择。"));
      setCreateOpen(false);
      clearState(['create', 'productId', 'shopId'], { replace: true });
      return;
    }
    if (createPrefillAppliedRef.current === fingerprint) return;
    createPrefillAppliedRef.current = fingerprint;
    createForm.setFieldsValue({ createKind: 'production', productId: product.id, shopId: shop.id, priority: 'normal' });
  }, [clearState, createForm, createOpen, productionCreatePrefill, productionResourcesLoaded, productionResourcesLoading, productionStatus, products, shops]);

  useEffect(() => {
    if (urlState.create !== 'production' || productionCreatePrefill) return;
    message.warning(tr("创建链接参数无效，请从运营任务中心重新选择。"));
    clearState(['create', 'productId', 'shopId'], { replace: true });
  }, [clearState, productionCreatePrefill, urlState.create]);

  const load = useCallback(async () => {
    const seq = requestSeq.current + 1;
    requestSeq.current = seq;
    const identityChanged = dataQueryIdentityRef.current !== queryIdentity;
    if (identityChanged) {
      setNextCursor(undefined);
      setHasMore(false);
      setShowingStaleData(false);
    }
    setLoading(true);
    setError(null);
    try {
      const page = await listTasks({
        status: urlState.status,
        platform: urlState.platform,
        taskType: urlState.taskType,
        cursor: urlState.cursor,
        limit: LIMIT,
      });
      if (requestSeq.current !== seq) return;
      const nextItems = page.items || [];
      itemsRef.current = nextItems;
      dataQueryIdentityRef.current = queryIdentity;
      setItems(nextItems);
      setDataQueryIdentity(queryIdentity);
      setNextCursor(page.nextCursor);
      setHasMore(page.hasMore);
      setShowingStaleData(false);
    } catch (e) {
      if (requestSeq.current !== seq) return;
      setError(extractOperationTaskAPIError(e));
      const canKeepCurrentData = dataQueryIdentityRef.current === queryIdentity && itemsRef.current.length > 0;
      setShowingStaleData(canKeepCurrentData);
      if (!canKeepCurrentData) {
        setNextCursor(undefined);
        setHasMore(false);
      }
    } finally {
      if (requestSeq.current === seq) setLoading(false);
    }
  }, [queryIdentity, urlState.cursor, urlState.platform, urlState.status, urlState.taskType]);

  useEffect(() => {
    void load();
    return () => {
      requestSeq.current += 1;
    };
  }, [load]);

  const updateFilters = (values: QueryState) => {
    setUrlState({
      status: values.status,
      platform: values.platform,
      taskType: values.taskType,
      cursor: undefined,
      cursorHistory: undefined,
    }, { replace: true });
  };

  const clearFilters = () => {
    form.resetFields();
    clearState(QUERY_KEYS, { replace: true });
  };

  const goNext = () => {
    if (!nextCursor) return;
    setUrlState({
      cursor: nextCursor,
      cursorHistory: serializeCursorHistory([...cursorStack, urlState.cursor || '']),
    }, { replace: true });
  };

  const goPrev = () => {
    const next = [...cursorStack];
    const prevCursor = next.pop();
    setUrlState({
      cursor: prevCursor || undefined,
      cursorHistory: serializeCursorHistory(next),
    }, { replace: true });
  };

  const openTask = useCallback((taskId: string) => {
    const returnPath = `/ops/task-center/operation-tasks${search}`;
    history.push(`/ops/task-center/operation-tasks/${encodeURIComponent(taskId)}?from=${encodeURIComponent(returnPath)}`);
  }, [search]);

  const columns = useMemo<ProColumns<OperationTaskSummary>[]>(() => [
    {
      title: tr("任务标题"),
      dataIndex: 'title',
      width: 260,
      render: (_, row) => (
        <Space direction="vertical" size={2}>
          <Button type="link" className="operation-tasks-page__title-link" onClick={() => openTask(row.id)}>
            {row.title || tr("未命名任务")}
          </Button>
          <Text type="secondary" ellipsis style={{ maxWidth: 240 }}>{row.summary || '—'}</Text>
        </Space>
      ),
    },
    { title: tr("任务类型"), dataIndex: 'taskType', width: 130, render: (v) => taskTypeLabel(String(v || '')) },
    { title: tr("平台"), dataIndex: 'platform', width: 110, render: (v) => platformLabel(String(v || '')) },
    { title: tr("状态"), dataIndex: 'status', width: 130, render: (v) => <OperationTaskStatusTag status={String(v || '')} /> },
    { title: tr("优先级"), dataIndex: 'priority', width: 100, render: (v) => <OperationPriorityTag priority={String(v || '')} /> },
    { title: tr("最新草稿"), dataIndex: 'latestDraftVersion', width: 110, render: (v) => v ? `v${v}` : '—' },
    { title: tr("最新执行状态"), dataIndex: 'latestExecutionStatus', width: 130, render: (v) => v ? <OperationAttemptStatusTag status={String(v)} /> : '—' },
    { title: tr("创建人"), dataIndex: 'createdBy', width: 130, render: (v) => copyableText(String(v || ''), 10) },
    { title: tr("创建时间"), dataIndex: 'createdAt', width: 170, render: (v) => formatDateTime(String(v || '')) },
    { title: tr("更新时间"), dataIndex: 'updatedAt', width: 170, render: (v) => formatDateTime(String(v || '')) },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 120,
      render: (_, row) => [
        <Button key="detail" type="link" icon={<EyeOutlined />} onClick={() => openTask(row.id)}>
          {tr("查看详情")}
        </Button>,
      ],
    },
  ], [openTask]);

  const filterActive = !!(urlState.status || urlState.platform || urlState.taskType);
  const emptyLocale = useListEmptyLocale('operationTasks', filterActive
    ? { description: tr("当前筛选条件下没有运营任务，请调整筛选条件后重试。") }
    : undefined);
  const openCreateModal = () => {
    setCreatePayload('{}');
    setCreatePayloadError(null);
    createIdempotencyRef.current = null;
    setCreateOpen(true);
  };

  const closeCreateModal = () => {
    if (createSubmittingRef.current) return;
    setCreateOpen(false);
    setCreatePayloadError(null);
    createIdempotencyRef.current = null;
    createPrefillAppliedRef.current = '';
    clearState(['create', 'productId', 'shopId'], { replace: true });
  };

  const submitCreateTask = async () => {
    if (createSubmittingRef.current) return;
    createSubmittingRef.current = true;
    setCreateSubmitting(true);
    setError(null);
    try {
      const values = await createForm.validateFields();
      let requestBody;
      if (values.createKind === 'production') {
        const product = products.find((item) => item.id === values.productId);
        const shop = shops.find((item) => item.id === values.shopId);
        if (!product || !shop || !productionStatus?.allowlist?.enabled || productionStatus.allowlist.shopId !== shop.id) {
          throw new Error(tr('商品或白名单抖店已变化，请刷新后重新选择。'));
        }
        requestBody = {
          sourceType: 'manual',
          sourceReference: product.id,
          taskType: 'product_publish',
          platform: 'douyin',
          title: trFormat("创建抖店平台草稿：{value0}", { value0: product.title }),
          summary: trFormat('冻结 {shop} 的刊登请求，人工审核后创建平台草稿。', { shop: shop.shopName }),
          payload: {
            schemaVersion: 'douyin_draft_v1',
            productId: product.id,
            shopId: shop.id,
            publishMode: 'save_as_platform_draft',
          },
          priority: values.priority,
        };
      } else {
        let payload: unknown;
        try {
          payload = JSON.parse(createPayload);
          if (payload === null) throw new Error(tr('任务载荷不能为 null'));
          setCreatePayloadError(null);
        } catch (jsonError) {
          setCreatePayloadError(jsonError instanceof Error ? jsonError.message : tr('JSON 格式不正确'));
          return;
        }
        requestBody = {
          sourceType: 'manual',
          sourceReference: values.sourceReference?.trim() || '',
          taskType: values.taskType,
          platform: values.platform,
          title: values.title.trim(),
          summary: values.summary?.trim() || undefined,
          payload,
          priority: values.priority,
        };
      }
      const fingerprint = JSON.stringify(requestBody);
      let idempotency = createIdempotencyRef.current;
      if (idempotency?.fingerprint !== fingerprint) {
        idempotency = { fingerprint, key: createOperationIdempotencyKey('create') };
        createIdempotencyRef.current = idempotency;
      }
      const created = await createTask(requestBody, idempotency.key);
      message.success(values.createKind === 'production' ? tr('平台草稿任务已创建，冻结快照等待人工审核。') : tr('运营任务已创建，等待准备草稿。'));
      setCreateOpen(false);
      createIdempotencyRef.current = null;
      history.push(`/ops/task-center/operation-tasks/${encodeURIComponent(created.id)}`);
    } catch (error) {
      if (error && typeof error === 'object' && 'errorFields' in error) return;
      const next = extractOperationTaskAPIError(error);
      setError(next);
      message.error(operationErrorMessage(next));
    } finally {
      createSubmittingRef.current = false;
      setCreateSubmitting(false);
    }
  };

  return (
    <TmPageContainer
      className="operation-tasks-page-shell"
      title={PAGE_COPY.operationTasks.title}
      subTitle={PAGE_COPY.operationTasks.description}
      extra={(
        <Space wrap className="operation-tasks-page__header-actions">
          <Button icon={<ReloadOutlined />} onClick={() => { void load(); void loadProductionStatus(); }} loading={loading || productionStatusLoading}>{tr("刷新")}</Button>
          {canCreate ? <Button type="primary" icon={<PlusOutlined />} onClick={openCreateModal}>{tr("创建运营任务")}</Button> : null}
        </Space>
      )}
    >
      <Space direction="vertical" size={16} className="operation-tasks-page">
        <div className="operation-tasks-page__runtime">
          <ProductionRuntimeBoundary status={productionStatus} loading={productionStatusLoading} error={productionStatusError} />
        </div>
        {error ? (
          <ErrorAlert
            title={operationErrorMessage(error)}
            actionHint={showingStaleData
              ? tr('刷新失败，当前显示上次成功加载的结果，请勿将其视为最新数据。')
              : error.traceId ? trFormat('排查编号：{traceId}', { traceId: error.traceId }) : tr('请稍后重试或联系管理员。')}
          />
        ) : null}
        <SectionCard
          className="operation-tasks-page__filters"
          title={tr("筛选任务")}
          description={tr("按任务状态、平台和任务类型缩小处理范围。")}
          compact
        >
          <Form form={form} layout="vertical" onFinish={updateFilters}>
            <Row gutter={[16, 12]}>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="status" label={tr("任务状态")}>
                  <Select allowClear options={optionsFromLabels(OPERATION_TASK_STATUS_LABELS)} placeholder={tr("全部状态")} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="platform" label={tr("平台")}>
                  <Select allowClear options={optionsFromLabels(OPERATION_PLATFORM_LABELS)} placeholder={tr("全部平台")} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8} lg={6}>
                <Form.Item name="taskType" label={tr("任务类型")}>
                  <Select allowClear options={optionsFromLabels(OPERATION_TASK_TYPE_LABELS)} placeholder={tr("全部类型")} />
                </Form.Item>
              </Col>
              <Col xs={24} lg={6} className="operation-tasks-page__filter-action-column">
                <Form.Item>
                  <OperationToolbar className="operation-tasks-page__filter-actions">
                    <Button icon={<FilterOutlined />} htmlType="submit">{tr("应用筛选")}</Button>
                    <Button onClick={clearFilters}>{tr("清除筛选")}</Button>
                  </OperationToolbar>
                </Form.Item>
              </Col>
            </Row>
          </Form>
        </SectionCard>

        {!error || visibleItems.length > 0 ? (
          <>
            <TmProTable<OperationTaskSummary>
              className="operation-tasks-page__table"
              rowKey="id"
              search={false}
              loading={loading}
              columns={columns}
              dataSource={visibleItems}
              pagination={false}
              locale={emptyLocale}
              scroll={{ x: 1560 }}
              options={false}
              headerTitle={(
                <div className="operation-tasks-page__table-heading">
                  <Text strong className="operation-tasks-page__table-title">{tr("任务列表")}</Text>
                  <Text type="secondary">{tr("查看任务状态、草稿进度和最近执行结果。")}</Text>
                </div>
              )}
              toolBarRender={() => [
                <Text key="batch-summary" type="secondary" className="operation-tasks-page__table-summary">
                  {trFormat('本批 {count} 条{filtered}{stale}', {
                    count: visibleItems.length,
                    filtered: filterActive ? tr('，已应用筛选') : '',
                    stale: showingStaleData ? tr('，数据未更新') : '',
                  })}
                </Text>,
              ]}
            />

            <OperationToolbar className="operation-tasks-page__pagination">
              <Button disabled={cursorStack.length === 0 || loading} onClick={goPrev}>{tr("上一批")}</Button>
              <Button disabled={!hasMore || !nextCursor || loading} onClick={goNext}>{tr("下一批")}</Button>
            </OperationToolbar>
          </>
        ) : null}
      </Space>

      <Modal
        title={tr("创建运营任务")}
        open={createOpen}
        onCancel={closeCreateModal}
        onOk={() => void submitCreateTask()}
        okText={tr("创建任务")}
        cancelText={tr("取消")}
        confirmLoading={createSubmitting}
        okButtonProps={{ disabled: createSubmitting }}
        width={720}
        destroyOnHidden
      >
        <Form form={createForm} layout="vertical" disabled={createSubmitting}>
          <Form.Item name="createKind" label={tr("任务模式")}>
            <Segmented
              block
              options={[
                { value: 'production', label: tr("创建抖店平台草稿") },
                { value: 'local', label: tr("通用本地任务") },
              ]}
            />
          </Form.Item>
          {createKind === 'production' ? (
            <Space direction="vertical" size={12} style={{ width: '100%' }}>
              <ProductionRuntimeBoundary status={productionStatus} loading={productionResourcesLoading} error={productionStatusError} />
              <Row gutter={16}>
                <Col xs={24} md={16}>
                  <Form.Item name="productId" label={tr("商品")} rules={[{ required: true, message: tr("请选择商品") }]}>
                    <Select
                      showSearch
                      loading={productionResourcesLoading}
                      optionFilterProp="label"
                      placeholder={tr("选择已完成抖店映射的商品")}
                      options={products.map((item) => ({ value: item.id, label: item.title || item.id }))}
                    />
                  </Form.Item>
                </Col>
                <Col xs={24} md={8}>
                  <Form.Item name="priority" label={tr("优先级")} rules={[{ required: true, message: tr("请选择优先级") }]}>
                    <Select options={optionsFromLabels({ low: tr('低'), normal: tr('普通'), high: tr('高'), urgent: tr('紧急') })} />
                  </Form.Item>
                </Col>
                <Col xs={24}>
                  <Form.Item name="shopId" label={tr("已授权白名单抖店")} rules={[{ required: true, message: tr("请选择白名单抖店") }]}>
                    <Select
                      loading={productionResourcesLoading}
                      placeholder={tr("选择当前生产白名单内的抖店")}
                      options={shops
                        .filter((item) => productionStatus?.allowlist?.enabled && item.id === productionStatus.allowlist.shopId)
                        .map((item) => ({ value: item.id, label: item.shopName }))}
                    />
                  </Form.Item>
                </Col>
              </Row>
              <Typography.Text type="secondary">{tr("执行方式固定为“保存为平台草稿”。创建时冻结商品、映射和请求内容，后续不可编辑；仍需人工审核后才能提交执行。")}</Typography.Text>
            </Space>
          ) : (
          <>
            <Typography.Paragraph type="secondary">
              {tr("通用任务仅用于本地流程，不会调用真实平台发布或上架能力。")}
            </Typography.Paragraph>
            <Row gutter={16}>
            <Col xs={24} md={16}>
              <Form.Item name="title" label={tr("任务标题")} rules={[{ required: true, whitespace: true, message: tr("请填写任务标题") }, { max: 200, message: tr("任务标题不能超过 200 个字符") }]}>
                <Input placeholder={tr("例如：复核商品标题与卖点")} maxLength={200} showCount />
              </Form.Item>
            </Col>
            <Col xs={24} md={8}>
              <Form.Item name="priority" label={tr("优先级")} rules={[{ required: true, message: tr("请选择优先级") }]}>
                <Select options={optionsFromLabels({ low: tr('低'), normal: tr('普通'), high: tr('高'), urgent: tr('紧急') })} />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item name="taskType" label={tr("任务类型")} rules={[{ required: true, message: tr("请选择任务类型") }]}>
                <Select options={optionsFromLabels(OPERATION_TASK_TYPE_LABELS)} placeholder={tr("选择任务类型")} />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item name="platform" label={tr("平台")} rules={[{ required: true, message: tr("请选择平台") }]}>
                <Select options={optionsFromLabels(OPERATION_PLATFORM_LABELS)} />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="summary" label={tr("任务概要")} rules={[{ max: 1000, message: tr("任务概要不能超过 1000 个字符") }]}>
            <Input.TextArea rows={3} maxLength={1000} showCount placeholder={tr("说明任务目标和审核重点")} />
          </Form.Item>
          <Form.Item name="sourceReference" label={tr("来源引用")} rules={[{ max: 255, message: tr("来源引用不能超过 255 个字符") }]}>
            <Input placeholder={tr("可填写商品、订单或外部记录编号")} maxLength={255} />
          </Form.Item>
          <Form.Item label={tr("任务载荷")} required validateStatus={createPayloadError ? 'error' : undefined} help={createPayloadError ? `${tr("JSON 格式错误：")}${createPayloadError}` : tr('仅填写任务执行所需的非敏感结构化数据。')}>
            <Input.TextArea
              aria-label={tr("任务载荷 JSON")}
              value={createPayload}
              onChange={(event) => {
                setCreatePayload(event.target.value);
                setCreatePayloadError(null);
              }}
              rows={8}
              spellCheck={false}
            />
          </Form.Item>
          </>
          )}
        </Form>
      </Modal>
    </TmPageContainer>
  );
}
