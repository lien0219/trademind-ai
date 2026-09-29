import {
  DownloadOutlined,
  EyeOutlined,
  ImportOutlined,
  PlusOutlined,
  ReloadOutlined,
  RollbackOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Descriptions,
  Drawer,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  Tooltip,
  Typography,
  Upload,
  message,
  type TableColumnsType,
  type UploadFile,
} from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PermissionGuard from '@/components/PermissionGuard';
import { EmptyState, ErrorAlert, TmPageContainer, TmPageHeaderExtra } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import { useUrlDrawerState } from '@/hooks/useUrlState';
import {
  confirmFreightFeeImport,
  createFreightFeeAdjustment,
  downloadFreightFeeCSVTemplate,
  freightFeeIdempotencyKey,
  getFreightFee,
  previewFreightFeeImport,
  queryFreightFeeImports,
  queryFreightFees,
  reverseFreightFeeAdjustment,
  type FreightAdjustment,
  type FreightCharge,
  type FreightChargeDetail,
  type FreightChargePreviewRow,
  type FreightImport,
  type FreightImportPreview,
  type FreightValidationIssue,
} from '@/services/freightFees';
import { formatProfitAmount } from '@/services/profitability';
import { queryShops, type ShopListRow } from '@/services/shops';
import { formatDateTime } from '@/utils/formatTime';
import { PERMISSIONS } from '@/utils/permission';

type AdjustmentFormValues = { amountMinor?: number; reason: string };

function nativeFile(fileList: UploadFile[]) {
  const entry = fileList[0];
  return (entry?.originFileObj || entry) as File | undefined;
}

function statusTag(status: string) {
  if (status === 'confirmed') return <Tag color="success">已确认</Tag>;
  if (status === 'mismatch') return <Tag color="warning">币种不一致</Tag>;
  return <Tag color="error">已阻断</Tag>;
}

function issueTable(rows: FreightValidationIssue[]) {
  return rows.map((row) => ({ ...row, key: `${row.line}:${row.field || ''}:${row.code}` }));
}

export default function FreightFeesPage() {
  const { can } = usePermission();
  const canImport = can(PERMISSIONS.FREIGHT_FEE_IMPORT);
  const canManage = can(PERMISSIONS.FREIGHT_FEE_MANAGE);
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [shopError, setShopError] = useState('');
  const [tab, setTab] = useState('charges');
  const [charges, setCharges] = useState<FreightCharge[]>([]);
  const [imports, setImports] = useState<FreightImport[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [listRevision, setListRevision] = useState(0);
  const [filters, setFilters] = useState({ orderNo: '', trackingNo: '', shopId: '', currency: '', status: '' });
  const [draftFilters, setDraftFilters] = useState(filters);
  const [detail, setDetail] = useState<FreightChargeDetail>();
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');
  const [importOpen, setImportOpen] = useState(false);
  const [importShopID, setImportShopID] = useState<string>();
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [preview, setPreview] = useState<FreightImportPreview>();
  const [previewing, setPreviewing] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [importKey, setImportKey] = useState('');
  const [adjustmentOpen, setAdjustmentOpen] = useState(false);
  const [adjustmentMode, setAdjustmentMode] = useState<'adjust' | 'reverse'>('adjust');
  const [reverseTarget, setReverseTarget] = useState<FreightAdjustment>();
  const [adjusting, setAdjusting] = useState(false);
  const [adjustmentKey, setAdjustmentKey] = useState('');
  const [adjustmentForm] = Form.useForm<AdjustmentFormValues>();
  const requestIDRef = useRef(0);
  const listRequestIDRef = useRef(0);
  const detailRequestIDRef = useRef(0);
  const inFlightRef = useRef(false);
  const drawer = useUrlDrawerState('freight-fee');

  const shopNames = useMemo(() => new Map(shops.map((shop) => [shop.id, shop.shopName])), [shops]);
  const shopOptions = useMemo(
    () => shops.map((shop) => ({ value: shop.id, label: `${shop.shopName} · ${shop.platform}` })),
    [shops],
  );

  const loadShops = useCallback(async () => {
    setShopError('');
    try {
      const result = await queryShops({ page: 1, pageSize: 500 });
      setShops(result.list || []);
    } catch (error) {
      setShopError((error as Error)?.message || '店铺列表加载失败');
    }
  }, []);

  useEffect(() => { void loadShops(); }, [loadShops]);

  const loadList = useCallback(async () => {
    const requestID = ++listRequestIDRef.current;
    setLoading(true);
    setLoadError('');
    try {
      if (tab === 'imports') {
        const result = await queryFreightFeeImports({ page, pageSize });
        if (requestID !== listRequestIDRef.current) return;
        setImports(result.list || []);
        setTotal(result.total || 0);
      } else {
        const result = await queryFreightFees({
          page,
          pageSize,
          orderNo: filters.orderNo || undefined,
          trackingNo: filters.trackingNo || undefined,
          shopId: filters.shopId || undefined,
          currency: filters.currency || undefined,
          status: (filters.status || undefined) as FreightCharge['status'] | undefined,
        });
        if (requestID !== listRequestIDRef.current) return;
        setCharges(result.list || []);
        setTotal(result.total || 0);
      }
    } catch (error) {
      if (requestID === listRequestIDRef.current) {
        setLoadError((error as Error)?.message || '运费账单加载失败');
        if (tab === 'imports') setImports([]);
        else setCharges([]);
        setTotal(0);
      }
    } finally {
      if (requestID === listRequestIDRef.current) setLoading(false);
    }
  }, [filters, page, pageSize, tab]);

  useEffect(() => {
    void loadList();
    return () => { listRequestIDRef.current += 1; };
  }, [listRevision, loadList]);

  const loadDetail = useCallback(async (id: string) => {
    const requestID = ++detailRequestIDRef.current;
    setDetailLoading(true);
    setDetailError('');
    try {
      const result = await getFreightFee(id);
      if (requestID === detailRequestIDRef.current) setDetail(result);
    } catch (error) {
      if (requestID === detailRequestIDRef.current) {
        setDetail(undefined);
        setDetailError((error as Error)?.message || '运费账单详情加载失败');
      }
    } finally {
      if (requestID === detailRequestIDRef.current) setDetailLoading(false);
    }
  }, []);

  useEffect(() => {
    if (drawer.id) void loadDetail(drawer.id);
    else {
      detailRequestIDRef.current += 1;
      setDetail(undefined);
      setDetailError('');
    }
    return () => { detailRequestIDRef.current += 1; };
  }, [drawer.id, loadDetail]);

  const invalidatePreview = useCallback(() => {
    requestIDRef.current += 1;
    setPreview(undefined);
    setImportKey('');
    setPreviewing(false);
  }, []);

  const resetImport = useCallback(() => {
    setImportShopID(undefined);
    setFileList([]);
    invalidatePreview();
  }, [invalidatePreview]);

  const runPreview = useCallback(async () => {
    const file = nativeFile(fileList);
    if (!file || !importShopID || previewing) return;
    const requestID = ++requestIDRef.current;
    setPreviewing(true);
    try {
      const result = await previewFreightFeeImport(file, importShopID);
      if (requestID !== requestIDRef.current) return;
      setPreview(result);
      setImportKey(freightFeeIdempotencyKey('import'));
    } catch (error) {
      if (requestID === requestIDRef.current) message.error((error as Error)?.message || 'CSV 校验失败');
    } finally {
      if (requestID === requestIDRef.current) setPreviewing(false);
    }
  }, [fileList, importShopID, previewing]);

  const confirmImport = useCallback(async () => {
    const file = nativeFile(fileList);
    if (!file || !importShopID || !preview?.valid || !importKey || inFlightRef.current) return;
    inFlightRef.current = true;
    setSubmitting(true);
    try {
      await confirmFreightFeeImport(file, importShopID, preview.fileHash, preview.calculationHash, importKey);
      message.success('运费账单已确认');
      setImportOpen(false);
      resetImport();
      setTab('charges');
      setPage(1);
      setListRevision((revision) => revision + 1);
    } catch (error) {
      message.error((error as Error)?.message || '运费账单确认失败');
    } finally {
      inFlightRef.current = false;
      setSubmitting(false);
    }
  }, [fileList, importKey, importShopID, preview, resetImport]);

  const openAdjustment = useCallback((mode: 'adjust' | 'reverse', target?: FreightAdjustment) => {
    setAdjustmentMode(mode);
    setReverseTarget(target);
    setAdjustmentKey(freightFeeIdempotencyKey(mode));
    adjustmentForm.resetFields();
    setAdjustmentOpen(true);
  }, [adjustmentForm]);

  const submitAdjustment = useCallback(async (values: AdjustmentFormValues) => {
    if (!detail || !adjustmentKey || adjusting) return;
    const reason = values.reason.trim();
    if (reason.length < 2) return;
    setAdjusting(true);
    try {
      if (adjustmentMode === 'adjust') {
        const amountMinor = values.amountMinor;
        if (!Number.isSafeInteger(amountMinor) || amountMinor === 0) {
          message.error('调整金额必须为非零安全整数');
          return;
        }
        await createFreightFeeAdjustment(detail.id, { amountMinor: amountMinor!, reason, idempotencyKey: adjustmentKey });
      } else if (reverseTarget) {
        await reverseFreightFeeAdjustment(detail.id, reverseTarget.id, { reason, idempotencyKey: adjustmentKey });
      }
      message.success(adjustmentMode === 'adjust' ? '调整已追加' : '冲正已追加');
      setAdjustmentOpen(false);
      await loadDetail(detail.id);
      await loadList();
    } catch (error) {
      message.error((error as Error)?.message || '运费调整失败');
    } finally {
      setAdjusting(false);
    }
  }, [adjustmentKey, adjustmentMode, adjusting, detail, loadDetail, loadList, reverseTarget]);

  const chargeColumns: TableColumnsType<FreightCharge> = [
    { title: '订单号', dataIndex: 'orderNo', width: 160, ellipsis: true },
    { title: '店铺', dataIndex: 'shopId', width: 170, ellipsis: true, render: (id) => shopNames.get(String(id)) || String(id) },
    { title: '承运商', dataIndex: 'carrier', width: 130, ellipsis: true },
    { title: '运单号', dataIndex: 'trackingNo', width: 180, ellipsis: true },
    { title: '原始费用编号', dataIndex: 'externalLineId', width: 160, ellipsis: true },
    { title: '实账净额', dataIndex: 'netAmountMinor', width: 145, render: (value, row) => formatProfitAmount(value, row.currency) },
    { title: '订单币种', dataIndex: 'orderCurrency', width: 95 },
    { title: '账单时间', dataIndex: 'billedAt', width: 170, render: (value) => formatDateTime(value) },
    { title: '状态', dataIndex: 'status', width: 110, render: (value) => statusTag(String(value)) },
    {
      title: '操作', fixed: 'right', width: 92,
      render: (_, row) => <Button type="link" icon={<EyeOutlined />} onClick={() => drawer.openDrawer(row.id)}>详情</Button>,
    },
  ];

  const previewColumns: TableColumnsType<FreightChargePreviewRow> = [
    { title: '行', dataIndex: 'line', width: 54 },
    { title: '订单号', dataIndex: 'orderNo', width: 150 },
    { title: '运单号', dataIndex: 'trackingNo', width: 160 },
    { title: '承运商', dataIndex: 'carrier', width: 120 },
    { title: '账单金额', dataIndex: 'amountMinor', width: 135, render: (value, row) => formatProfitAmount(value, row.currency) },
    { title: '账单币种', dataIndex: 'currency', width: 90 },
    { title: '导入结果', dataIndex: 'disposition', width: 95, render: (value) => value === 'duplicate' ? <Tag>重复</Tag> : <Tag color="processing">新增</Tag> },
    { title: '对账状态', dataIndex: 'status', width: 120, render: (value) => statusTag(String(value)) },
    { title: '说明', dataIndex: 'reason', width: 240, ellipsis: true },
  ];

  const importColumns: TableColumnsType<FreightImport> = [
    { title: '确认时间', dataIndex: 'confirmedAt', width: 175, render: (value) => formatDateTime(value) },
    { title: '店铺', dataIndex: 'shopId', width: 170, ellipsis: true, render: (id) => shopNames.get(String(id)) || String(id) },
    { title: '文件名', dataIndex: 'fileName', width: 220, ellipsis: true },
    { title: '来源行数', dataIndex: 'sourceRowCount', width: 100 },
    { title: '新增账单', dataIndex: 'importedRows', width: 100 },
    { title: '重复行', dataIndex: 'duplicateRows', width: 100 },
    { title: '文件 SHA-256', dataIndex: 'fileHash', width: 250, ellipsis: true },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.FREIGHT_FEE_VIEW} showForbiddenPage>
      <TmPageContainer
        title="承运商运费账单"
        extra={(
          <TmPageHeaderExtra>
            <Space wrap>
              <Tooltip title="下载 CSV 模板">
                <Button icon={<DownloadOutlined />} onClick={downloadFreightFeeCSVTemplate}>模板</Button>
              </Tooltip>
              <Button icon={<ReloadOutlined />} onClick={() => void loadList()} loading={loading}>刷新</Button>
              {canImport ? <Button type="primary" icon={<UploadOutlined />} onClick={() => setImportOpen(true)}>导入账单</Button> : null}
            </Space>
          </TmPageHeaderExtra>
        )}
      >
        {shopError ? <ErrorAlert title={shopError} actionHint={<Button icon={<ReloadOutlined />} onClick={() => void loadShops()}>重新加载店铺</Button>} /> : null}
        {loadError ? <ErrorAlert title={loadError} actionHint={<Button icon={<ReloadOutlined />} onClick={() => void loadList()}>重新加载</Button>} /> : null}
        <Tabs
          activeKey={tab}
          onChange={(key) => { setPage(1); setTab(key); }}
          items={[
            { key: 'charges', label: '运费账单' },
            { key: 'imports', label: '导入批次' },
          ]}
        />
        {tab === 'charges' ? (
          <>
            <Space wrap className="freight-fee-filters">
              <Input placeholder="订单号" value={draftFilters.orderNo} maxLength={128} onChange={(event) => setDraftFilters({ ...draftFilters, orderNo: event.target.value })} />
              <Input placeholder="运单号" value={draftFilters.trackingNo} maxLength={255} onChange={(event) => setDraftFilters({ ...draftFilters, trackingNo: event.target.value })} />
              <Select allowClear placeholder="全部店铺" value={draftFilters.shopId || undefined} options={shopOptions} onChange={(shopId) => setDraftFilters({ ...draftFilters, shopId: shopId || '' })} />
              <Select allowClear placeholder="全部币种" value={draftFilters.currency || undefined} options={['CNY', 'USD', 'EUR', 'GBP', 'JPY', 'SGD'].map((value) => ({ value, label: value }))} onChange={(currency) => setDraftFilters({ ...draftFilters, currency: currency || '' })} />
              <Select allowClear placeholder="全部状态" value={draftFilters.status || undefined} options={[{ value: 'confirmed', label: '已确认' }, { value: 'mismatch', label: '币种不一致' }]} onChange={(status) => setDraftFilters({ ...draftFilters, status: status || '' })} />
              <Button type="primary" onClick={() => { setPage(1); setFilters(draftFilters); }}>查询</Button>
              <Button onClick={() => { const empty = { orderNo: '', trackingNo: '', shopId: '', currency: '', status: '' }; setDraftFilters(empty); setFilters(empty); setPage(1); }}>重置</Button>
            </Space>
            <Table<FreightCharge>
              rowKey="id"
              size="middle"
              loading={loading}
              columns={chargeColumns}
              dataSource={charges}
              scroll={{ x: 1350 }}
              locale={{ emptyText: <EmptyState title="暂无运费账单" description="确认导入后，承运商实账会出现在这里。" /> }}
              pagination={{ current: page, pageSize, total, showSizeChanger: true, onChange: (nextPage, nextSize) => { setPage(nextPage); setPageSize(nextSize); } }}
            />
          </>
        ) : (
          <Table<FreightImport>
            rowKey="id"
            size="middle"
            loading={loading}
            columns={importColumns}
            dataSource={imports}
            scroll={{ x: 1100 }}
            locale={{ emptyText: <EmptyState title="暂无导入批次" /> }}
            pagination={{ current: page, pageSize, total, showSizeChanger: true, onChange: (nextPage, nextSize) => { setPage(nextPage); setPageSize(nextSize); } }}
          />
        )}

        <Modal
          title="导入承运商运费账单"
          open={importOpen}
          width={1000}
          destroyOnHidden
          closable={!submitting}
          keyboard={!submitting}
          maskClosable={!submitting}
          onCancel={() => { if (!submitting) { setImportOpen(false); resetImport(); } }}
          footer={(
            <Space>
              <Button disabled={submitting} onClick={() => { setImportOpen(false); resetImport(); }}>取消</Button>
              <Button icon={<ImportOutlined />} onClick={() => void runPreview()} loading={previewing} disabled={!importShopID || fileList.length === 0}>校验预览</Button>
              <Button type="primary" onClick={() => Modal.confirm({ title: '确认导入这批运费账单？', content: `新增 ${preview?.newCharges || 0} 条，重复 ${preview?.duplicateCharges || 0} 条。确认后账单事实不可删除。`, onOk: confirmImport, okButtonProps: { loading: submitting, disabled: !preview?.valid } })} loading={submitting} disabled={!preview?.valid || !canImport}>确认导入</Button>
            </Space>
          )}
        >
          <Space direction="vertical" size="middle" className="freight-fee-full-width">
            {shopError ? <ErrorAlert title={shopError} actionHint={<Button icon={<ReloadOutlined />} onClick={() => void loadShops()}>重新加载店铺</Button>} /> : null}
            <Select
              showSearch
              optionFilterProp="label"
              placeholder="选择账单所属店铺"
              value={importShopID}
              options={shopOptions}
              onChange={(shopId) => { setImportShopID(shopId); invalidatePreview(); }}
              disabled={submitting}
            />
            <Upload
              accept=".csv,text/csv"
              maxCount={1}
              fileList={fileList}
              beforeUpload={() => false}
              onChange={({ fileList: next }) => { setFileList(next); invalidatePreview(); }}
              onRemove={() => { setFileList([]); invalidatePreview(); return true; }}
              disabled={submitting}
            >
              <Button icon={<UploadOutlined />} disabled={submitting}>选择 CSV</Button>
            </Upload>
            <Typography.Text type="secondary">CSV 必须包含严格模板表头；金额为整数最小货币单位，账单时间使用带时区的 RFC3339。单文件最多 2 MiB、1000 行。</Typography.Text>
            {preview ? (
              <>
                <Alert
                  type={preview.valid ? (preview.warnings.length ? 'warning' : 'success') : 'error'}
                  showIcon
                  message={preview.valid ? `校验通过：${preview.newCharges} 条新增，${preview.duplicateCharges} 条重复` : '存在阻断问题，整批不能确认'}
                  description={`店铺：${preview.shopName}；来源行：${preview.sourceRows}；文件 SHA-256：${preview.fileHash}`}
                />
                {preview.warnings.length ? <Alert type="warning" message="账单币种与订单币种不一致的行会留档，但不会替代利润中的本地运费估价。" /> : null}
                {preview.issues.length ? <Table size="small" rowKey={(row) => `${row.line}:${row.code}`} pagination={{ pageSize: 5 }} columns={[
                  { title: '行', dataIndex: 'line', width: 60 }, { title: '字段', dataIndex: 'field', width: 130 }, { title: '错误', dataIndex: 'message' },
                ]} dataSource={issueTable(preview.issues)} /> : null}
                {preview.warnings.length ? <Table size="small" rowKey={(row) => `${row.line}:${row.code}`} pagination={{ pageSize: 5 }} columns={[
                  { title: '行', dataIndex: 'line', width: 60 }, { title: '提示', dataIndex: 'message' },
                ]} dataSource={issueTable(preview.warnings)} /> : null}
                <Table<FreightChargePreviewRow> size="small" rowKey="line" pagination={{ pageSize: 8 }} columns={previewColumns} dataSource={preview.rows} scroll={{ x: 1100 }} />
              </>
            ) : null}
          </Space>
        </Modal>

        <Drawer title="运费账单详情" open={drawer.open} width="min(720px, 100vw)" onClose={drawer.closeDrawer} destroyOnHidden>
          {detailError ? <ErrorAlert title={detailError} actionHint={drawer.id ? <Button icon={<ReloadOutlined />} onClick={() => { const id = drawer.id; if (id) void loadDetail(id); }}>重新加载</Button> : undefined} /> : null}
          {detailLoading ? <div className="freight-fee-detail-loading">加载中…</div> : null}
          {detail ? (
            <Space direction="vertical" size="large" className="freight-fee-full-width">
              {detail.status === 'mismatch' ? <Alert type="warning" showIcon message="账单币种不一致，不会进入订单利润实账。" description={detail.reason} /> : null}
              <Descriptions bordered size="small" column={1}>
                <Descriptions.Item label="订单号">{detail.orderNo}</Descriptions.Item>
                <Descriptions.Item label="店铺">{shopNames.get(detail.shopId) || detail.shopId}</Descriptions.Item>
                <Descriptions.Item label="承运商">{detail.carrier}</Descriptions.Item>
                <Descriptions.Item label="运单号">{detail.trackingNo}</Descriptions.Item>
                <Descriptions.Item label="费用编号">{detail.externalLineId}</Descriptions.Item>
                <Descriptions.Item label="账单实额">{formatProfitAmount(detail.amountMinor, detail.currency)}</Descriptions.Item>
                <Descriptions.Item label="调整净额">{formatProfitAmount(detail.netAmountMinor, detail.currency)}</Descriptions.Item>
                <Descriptions.Item label="订单币种">{detail.orderCurrency}</Descriptions.Item>
                <Descriptions.Item label="账单时间">{formatDateTime(detail.billedAt)}</Descriptions.Item>
                <Descriptions.Item label="导入批次">{detail.importId}</Descriptions.Item>
              </Descriptions>
              <Space wrap>
                {canManage ? <Button type="primary" icon={<PlusOutlined />} onClick={() => openAdjustment('adjust')}>追加调整</Button> : null}
                <Typography.Text type="secondary">调整使用整数最小单位；净额不能小于零。</Typography.Text>
              </Space>
              <Table<FreightAdjustment>
                rowKey="id"
                size="small"
                pagination={false}
                dataSource={detail.adjustments || []}
                locale={{ emptyText: '暂无更正事实' }}
                columns={[
                  { title: '类型', dataIndex: 'factType', width: 95, render: (value) => value === 'reversal' ? '冲正' : '调整' },
                  { title: '金额', dataIndex: 'amountMinor', width: 135, render: (value, row) => formatProfitAmount(value, row.currency) },
                  { title: '原因', dataIndex: 'reason', ellipsis: true },
                  { title: '时间', dataIndex: 'createdAt', width: 165, render: (value) => formatDateTime(value) },
                  {
                    title: '操作', width: 92,
                    render: (_, row) => canManage && row.factType === 'adjustment' ? (
                      <Button type="link" icon={<RollbackOutlined />} onClick={() => openAdjustment('reverse', row)}>冲正</Button>
                    ) : null,
                  },
                ]}
              />
            </Space>
          ) : null}
        </Drawer>

        <Modal
          title={adjustmentMode === 'adjust' ? '追加运费调整' : '冲正调整'}
          open={adjustmentOpen}
          confirmLoading={adjusting}
          closable={!adjusting}
          keyboard={!adjusting}
          maskClosable={!adjusting}
          okText={adjustmentMode === 'adjust' ? '追加' : '确认冲正'}
          onCancel={() => { if (!adjusting) setAdjustmentOpen(false); }}
          onOk={() => adjustmentForm.submit()}
        >
          <Form form={adjustmentForm} layout="vertical" onFinish={(values) => void submitAdjustment(values)}>
            {adjustmentMode === 'adjust' ? (
              <Form.Item name="amountMinor" label={`调整金额（${detail?.currency || ''} 最小单位，可正可负）`} rules={[{ required: true, message: '请输入非零整数金额' }]}>
                <InputNumber className="freight-fee-full-width" precision={0} min={-Number.MAX_SAFE_INTEGER} max={Number.MAX_SAFE_INTEGER} />
              </Form.Item>
            ) : <Alert type="warning" message={`将追加 ${reverseTarget ? formatProfitAmount(-Number(reverseTarget.amountMinor), reverseTarget.currency) : ''} 的冲正事实。`} />}
            <Form.Item name="reason" label="更正原因" rules={[{ required: true, min: 2, max: 500, whitespace: true, message: '请输入 2 至 500 个字符的原因' }]}>
              <Input.TextArea rows={3} maxLength={500} showCount />
            </Form.Item>
          </Form>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
