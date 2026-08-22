import { ArrowLeftOutlined } from '@ant-design/icons';
import { history, useParams } from '@umijs/max';
import type { ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Descriptions, Form, Input, InputNumber, Modal, Space, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PermissionGuard from '@/components/PermissionGuard';
import { EmptyState, ErrorAlert, SectionCard, TmPageContainer, TmPageHeaderExtra, TmProTable } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import {
  createProcurementIdempotencyKey,
  createPurchaseReturn,
  extractProcurementAPIError,
  getPurchaseOrder,
  listSuppliers,
  listReturnableReceiptItems,
  listWarehouses,
  procurementErrorMessage,
  receivePurchaseOrder,
  transitionPurchaseOrder,
  type PurchaseOrder,
  type PurchaseOrderItem,
  type ReturnableReceiptItem,
} from '@/services/procurement';
import { PERMISSIONS } from '@/utils/permission';
import { formatDateTime } from '@/utils/formatTime';
import { canReceivePurchaseOrder, formatMinorAmount, PURCHASE_ORDER_STATUS, remainingQuantity } from '../helpers';
import '../index.less';

type TransitionAction = 'submit' | 'approve' | 'cancel' | 'close';
type TransitionState = { action: TransitionAction; label: string; danger?: boolean };
type TransitionValues = { reason: string };
type ReceiptValues = { quantities: Record<string, number> };
type ReturnValues = { reason: string; remark: string; quantities: Record<string, number> };

function statusTag(status: string) {
  const meta = PURCHASE_ORDER_STATUS[status] || { text: '未知状态', color: 'default' };
  return <Tag color={meta.color}>{meta.text}</Tag>;
}

export default function PurchaseOrderDetailPage() {
  const { id = '' } = useParams<{ id: string }>();
  const { can, readonly } = usePermission();
  const canManage = !readonly && can(PERMISSIONS.PROCUREMENT_MANAGE);
  const canApprove = !readonly && can(PERMISSIONS.PROCUREMENT_APPROVE);
  const canReceive = !readonly && can(PERMISSIONS.PROCUREMENT_RECEIVE);
  const [transitionForm] = Form.useForm<TransitionValues>();
  const [receiptForm] = Form.useForm<ReceiptValues>();
  const [returnForm] = Form.useForm<ReturnValues>();
  const [order, setOrder] = useState<PurchaseOrder>();
  const [supplierName, setSupplierName] = useState('');
  const [warehouseName, setWarehouseName] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [transition, setTransition] = useState<TransitionState>();
  const [transitionSubmitting, setTransitionSubmitting] = useState(false);
  const [receiptOpen, setReceiptOpen] = useState(false);
  const [receiptSubmitting, setReceiptSubmitting] = useState(false);
  const [receiptKey, setReceiptKey] = useState('');
  const [returnOpen, setReturnOpen] = useState(false);
  const [returnLoading, setReturnLoading] = useState(false);
  const [returnSubmitting, setReturnSubmitting] = useState(false);
  const [returnError, setReturnError] = useState('');
  const [returnItems, setReturnItems] = useState<ReturnableReceiptItem[]>([]);
  const [returnKey, setReturnKey] = useState('');
  const returnSubmittingRef = useRef(false);
  const returnLoadSequenceRef = useRef(0);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError(undefined);
    try {
      const [nextOrder, supplierResult, warehouseResult] = await Promise.all([
        getPurchaseOrder(id), listSuppliers(), listWarehouses(),
      ]);
      setOrder(nextOrder);
      setSupplierName(supplierResult.list.find((row) => row.id === nextOrder.supplierId)?.name || nextOrder.supplierId);
      setWarehouseName(warehouseResult.list.find((row) => row.id === nextOrder.warehouseId)?.name || nextOrder.warehouseId);
    } catch (nextError) {
      setError(procurementErrorMessage(extractProcurementAPIError(nextError), '采购单详情加载失败，请稍后重试。'));
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => () => {
    returnLoadSequenceRef.current += 1;
  }, []);

  const openTransition = (state: TransitionState) => {
    transitionForm.setFieldsValue({ reason: '' });
    setTransition(state);
  };

  const submitTransition = async (values: TransitionValues) => {
    if (!order || !transition) return;
    setTransitionSubmitting(true);
    try {
      const updated = await transitionPurchaseOrder(order.id, transition.action, {
        expectedRevision: order.revision,
        reason: values.reason?.trim() || '',
      });
      setOrder(updated);
      setTransition(undefined);
      message.success(`${transition.label}成功`);
    } catch (nextError) {
      const apiError = extractProcurementAPIError(nextError);
      message.error(procurementErrorMessage(apiError));
      if (apiError.message.toLowerCase().includes('revision conflict')) await load();
    } finally {
      setTransitionSubmitting(false);
    }
  };

  const openReceipt = () => {
    if (!order?.items) return;
    const quantities = Object.fromEntries(order.items.map((item) => [item.id, 0]));
    receiptForm.setFieldsValue({ quantities });
    setReceiptKey(createProcurementIdempotencyKey('goods-receipt'));
    setReceiptOpen(true);
  };

  const submitReceipt = async (values: ReceiptValues) => {
    if (!order?.items) return;
    const items = order.items
      .map((item) => ({ purchaseOrderItemId: item.id, quantity: Number(values.quantities?.[item.id] || 0) }))
      .filter((item) => item.quantity > 0);
    if (items.length === 0) {
      message.warning('请至少填写一项本次收货数量');
      return;
    }
    setReceiptSubmitting(true);
    try {
      const result = await receivePurchaseOrder(order.id, {
        expectedRevision: order.revision,
        idempotencyKey: receiptKey,
        items,
      });
      setOrder(result.purchaseOrder);
      setReceiptOpen(false);
      message.success(`收货已确认，收货单号：${result.receipt.receiptNo}`);
    } catch (nextError) {
      const apiError = extractProcurementAPIError(nextError);
      message.error(procurementErrorMessage(apiError, '收货确认失败，请核对数量后重试。'));
      if (apiError.message.toLowerCase().includes('revision conflict')) await load();
    } finally {
      setReceiptSubmitting(false);
    }
  };

  const openReturn = async () => {
    if (!order) return;
    const loadSequence = ++returnLoadSequenceRef.current;
    setReturnOpen(true);
    setReturnLoading(true);
    setReturnError('');
    setReturnItems([]);
    returnForm.resetFields();
    setReturnKey(createProcurementIdempotencyKey('purchase-return-create'));
    try {
      const result = await listReturnableReceiptItems(order.id);
      if (returnLoadSequenceRef.current !== loadSequence) return;
      const items = result.list || [];
      setReturnItems(items);
      returnForm.setFieldsValue({
        reason: '',
        remark: '',
        quantities: Object.fromEntries(items.map((item) => [item.goodsReceiptItemId, 0])),
      });
    } catch (nextError) {
      if (returnLoadSequenceRef.current !== loadSequence) return;
      setReturnError(procurementErrorMessage(extractProcurementAPIError(nextError), '可退收货记录加载失败，请稍后重试。'));
    } finally {
      if (returnLoadSequenceRef.current === loadSequence) setReturnLoading(false);
    }
  };

  const closeReturn = () => {
    if (returnSubmittingRef.current) return;
    returnLoadSequenceRef.current += 1;
    setReturnOpen(false);
    setReturnLoading(false);
  };

  const submitReturn = async (values: ReturnValues) => {
    if (!order || returnSubmittingRef.current) return;
    const items = returnItems
      .map((item) => ({ goodsReceiptItemId: item.goodsReceiptItemId, quantity: Number(values.quantities?.[item.goodsReceiptItemId] || 0) }))
      .filter((item) => item.quantity > 0);
    if (items.length === 0) {
      message.warning('请至少填写一项本次退货数量');
      return;
    }
    returnSubmittingRef.current = true;
    setReturnSubmitting(true);
    try {
      const purchaseReturn = await createPurchaseReturn({
        idempotencyKey: returnKey,
        purchaseOrderId: order.id,
        reason: values.reason.trim(),
        remark: values.remark?.trim() || '',
        items,
      });
      setReturnOpen(false);
      message.success(`采购退货草稿已创建：${purchaseReturn.returnNo}`);
      history.push(`/procurement/purchase-returns/${purchaseReturn.id}`);
    } catch (nextError) {
      message.error(procurementErrorMessage(extractProcurementAPIError(nextError), '采购退货创建失败，请核对数量后重试。'));
    } finally {
      returnSubmittingRef.current = false;
      setReturnSubmitting(false);
    }
  };

  const itemColumns: ProColumns<PurchaseOrderItem>[] = [
    {
      title: '商品', dataIndex: 'productTitle', minWidth: 170, ellipsis: true,
      render: (_, row) => row.productTitle || '商品信息不可用',
    },
    {
      title: '本地规格', dataIndex: 'skuName', minWidth: 170, ellipsis: true,
      render: (_, row) => row.skuName || row.skuCode || row.productSkuId,
    },
    { title: '订购数量', dataIndex: 'quantity', width: 100, align: 'right' },
    { title: '已收数量', dataIndex: 'receivedQuantity', width: 100, align: 'right' },
    { title: '未收数量', width: 100, align: 'right', render: (_, row) => remainingQuantity(row.quantity, row.receivedQuantity) },
    { title: '采购单价', dataIndex: 'unitCostMinor', width: 140, align: 'right', render: (_, row) => formatMinorAmount(row.unitCostMinor, order?.currency) },
    { title: '明细金额', dataIndex: 'lineAmountMinor', width: 150, align: 'right', render: (_, row) => formatMinorAmount(row.lineAmountMinor, order?.currency) },
  ];

  const actionButtons = useMemo(() => {
    if (!order) return null;
    return (
      <Space wrap>
        <Button icon={<ArrowLeftOutlined />} onClick={() => history.push('/procurement/purchase-orders')}>返回列表</Button>
        {order.status === 'draft' && canManage ? <Button type="primary" onClick={() => openTransition({ action: 'submit', label: '提交审批' })}>提交审批</Button> : null}
        {order.status === 'pending_approval' && canApprove ? <Button type="primary" onClick={() => openTransition({ action: 'approve', label: '审批通过' })}>审批通过</Button> : null}
        {canReceive && canReceivePurchaseOrder(order.status) ? <Button type="primary" onClick={openReceipt}>确认收货</Button> : null}
        {canManage && ['partially_received', 'received', 'closed'].includes(order.status) ? <Button onClick={() => void openReturn()}>发起退货</Button> : null}
        {['partially_received', 'received', 'closed'].includes(order.status) ? <Button onClick={() => history.push(`/procurement/purchase-returns?purchaseOrderId=${encodeURIComponent(order.id)}`)}>退货记录</Button> : null}
        {canManage && ['approved', 'partially_received'].includes(order.status) ? <Button onClick={() => openTransition({ action: 'close', label: '关闭采购单', danger: true })}>关闭采购单</Button> : null}
        {canManage && ['draft', 'pending_approval', 'approved'].includes(order.status) && (order.items || []).every((item) => item.receivedQuantity === 0) ? (
          <Button danger onClick={() => openTransition({ action: 'cancel', label: '取消采购单', danger: true })}>取消采购单</Button>
        ) : null}
      </Space>
    );
  }, [canApprove, canManage, canReceive, order]);

  return (
    <PermissionGuard require={PERMISSIONS.PROCUREMENT_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-procurement-page"
        title={order?.purchaseOrderNo || '采购单详情'}
        subTitle="核对采购明细、版本和收货进度。审批与执行权限分离，过期版本会被拒绝。"
        extra={<TmPageHeaderExtra>{actionButtons}</TmPageHeaderExtra>}
      >
        {loading ? <Alert type="info" showIcon message="正在加载采购单详情…" /> : null}
        {error ? <ErrorAlert title={error} actionHint={<Button onClick={() => void load()}>重新加载</Button>} /> : null}
        {order ? (
          <>
            {readonly ? <Alert type="info" showIcon message="当前账号为只读模式，不能提交、审批、确认收货或发起退货。" /> : null}
            <SectionCard title="采购单信息" description="状态变更使用当前版本号提交，避免覆盖他人已完成的操作。">
              <Descriptions column={{ xs: 1, sm: 2, lg: 3 }}>
                <Descriptions.Item label="采购单号"><Typography.Text copyable>{order.purchaseOrderNo}</Typography.Text></Descriptions.Item>
                <Descriptions.Item label="状态">{statusTag(order.status)}</Descriptions.Item>
                <Descriptions.Item label="版本">{order.revision}</Descriptions.Item>
                <Descriptions.Item label="供应商">{supplierName}</Descriptions.Item>
                <Descriptions.Item label="收货仓库">{warehouseName}</Descriptions.Item>
                <Descriptions.Item label="采购金额">{formatMinorAmount(order.totalAmountMinor, order.currency)}</Descriptions.Item>
                <Descriptions.Item label="创建时间">{formatDateTime(order.createdAt)}</Descriptions.Item>
                <Descriptions.Item label="审批时间">{order.approvedAt ? formatDateTime(order.approvedAt) : '—'}</Descriptions.Item>
                <Descriptions.Item label="备注">{order.remark || '—'}</Descriptions.Item>
              </Descriptions>
            </SectionCard>
            <SectionCard title="采购明细" description="已收数量来自不可重复记账的收货事务；未收数量不能小于零。">
              <TmProTable<PurchaseOrderItem>
                rowKey="id" columns={itemColumns} dataSource={order.items || []} search={false} options={false}
                pagination={false} cardBordered={false} scroll={{ x: 980 }} locale={{ emptyText: '采购单没有明细。' }}
              />
            </SectionCard>
          </>
        ) : null}

        <Modal
          title={transition?.label || '采购单操作'}
          open={Boolean(transition)}
          confirmLoading={transitionSubmitting}
          okText={transition?.label || '确认'}
          okButtonProps={{ danger: transition?.danger }}
          cancelText="取消"
          onCancel={() => !transitionSubmitting && setTransition(undefined)}
          onOk={() => transitionForm.submit()}
          forceRender
        >
          <Form form={transitionForm} layout="vertical" preserve={false} onFinish={(values) => void submitTransition(values)}>
            <Alert type={transition?.danger ? 'warning' : 'info'} showIcon message={`本次操作基于版本 ${order?.revision || '—'}，若采购单已更新将自动拒绝。`} />
            <Form.Item label="操作说明" name="reason" rules={[{ max: 500, message: '操作说明不能超过 500 个字符' }]}>
              <Input.TextArea rows={3} maxLength={500} showCount placeholder="可选，用于操作日志追溯" />
            </Form.Item>
          </Form>
        </Modal>

        <Modal
          title="确认收货"
          open={receiptOpen}
          width={760}
          confirmLoading={receiptSubmitting}
          okText="确认本次收货"
          cancelText="取消"
          onCancel={() => !receiptSubmitting && setReceiptOpen(false)}
          onOk={() => receiptForm.submit()}
          forceRender
        >
          <Form form={receiptForm} layout="vertical" preserve={false} onFinish={(values) => void submitReceipt(values)}>
            <Alert type="warning" showIcon message="确认后会在同一事务中增加本地聚合库存并记录仓库库存流水，不能在此页面撤销。" />
            <div className="tm-procurement-receipt-list">
              {(order?.items || []).map((item) => {
                const remaining = remainingQuantity(item.quantity, item.receivedQuantity);
                return (
                  <div className="tm-procurement-receipt-row" key={item.id}>
                    <div>
                      <Typography.Text strong>{item.productTitle || item.productSkuId}</Typography.Text>
                      <Typography.Text type="secondary">{item.skuName || item.skuCode || '未命名规格'} · 未收 {remaining}</Typography.Text>
                    </div>
                    <Form.Item
                      label="本次收货"
                      name={['quantities', item.id]}
                      rules={[{ type: 'number', min: 0, max: remaining, message: `可收数量为 0–${remaining}` }]}
                    >
                      <InputNumber min={0} max={remaining} precision={0} disabled={remaining === 0} />
                    </Form.Item>
                  </div>
                );
              })}
            </div>
          </Form>
        </Modal>

        <Modal
          title="发起采购退货"
          open={returnOpen}
          width={780}
          confirmLoading={returnSubmitting}
          okText="创建退货单草稿"
          okButtonProps={{ disabled: returnLoading || Boolean(returnError) || returnItems.length === 0 }}
          cancelText="取消"
          onCancel={closeReturn}
          onOk={() => returnForm.submit()}
          forceRender
        >
          <Form form={returnForm} layout="vertical" preserve={false} onFinish={(values) => void submitReturn(values)}>
            <Alert type="info" showIcon message="创建后为草稿，不会立即扣减库存；提交审批并由独立执行岗确认后才会完成出库。" />
            <Form.Item label="退货原因" name="reason" rules={[{ required: true, whitespace: true, message: '请输入退货原因' }, { max: 128, message: '退货原因不能超过 128 个字符' }]}>
              <Input maxLength={128} placeholder="例如：到货质量异常" />
            </Form.Item>
            <Form.Item label="备注" name="remark" rules={[{ max: 520, message: '备注不能超过 520 个字符' }]}>
              <Input.TextArea rows={2} maxLength={520} showCount />
            </Form.Item>
            {returnLoading ? <Alert type="info" showIcon message="正在加载可退收货记录…" /> : null}
            {returnError ? <ErrorAlert title={returnError} actionHint={<Button onClick={() => void openReturn()}>重新加载</Button>} /> : null}
            {!returnLoading && !returnError && returnItems.length === 0 ? <EmptyState compact title="没有可退数量" description="该采购单的收货数量已全部分配给有效退货单，或当前没有收货记录。" /> : null}
            {!returnLoading && !returnError && returnItems.length > 0 ? <div className="tm-procurement-receipt-list">
              {returnItems.map((item) => <div className="tm-procurement-receipt-row" key={item.goodsReceiptItemId}>
                <div>
                  <Typography.Text strong>{item.productTitle || item.productSkuId}</Typography.Text>
                  <Typography.Text type="secondary">{item.skuName || item.skuCode || '未命名规格'} · 收货单 {item.receiptNo} · 可退 {item.remainingQuantity}</Typography.Text>
                </div>
                <Form.Item
                  label="本次退货"
                  name={['quantities', item.goodsReceiptItemId]}
                  rules={[{ type: 'number', min: 0, max: item.remainingQuantity, message: `可退数量为 0–${item.remainingQuantity}` }]}
                >
                  <InputNumber min={0} max={item.remainingQuantity} precision={0} />
                </Form.Item>
              </div>)}
            </div> : null}
          </Form>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
