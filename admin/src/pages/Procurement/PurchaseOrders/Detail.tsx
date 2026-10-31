import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
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
import { idempotentCommandKey, type IdempotentCommandKey } from '@/utils/idempotentCommandKey';
import { createTenantReadGuard } from '@/utils/tenantAsync';
import { formatDateTime } from '@/utils/formatTime';
import { canReceivePurchaseOrder, formatMinorAmount, PURCHASE_ORDER_STATUS, remainingQuantity } from '../helpers';
import '../index.less';

type TransitionAction = 'submit' | 'approve' | 'cancel' | 'close';
type TransitionState = { action: TransitionAction; label: string; danger?: boolean };
type TransitionValues = { reason: string };
type ReceiptValues = { quantities: Record<string, number> };
type ReturnValues = { reason: string; remark: string; quantities: Record<string, number> };

function statusTag(status: string) {
  const meta = PURCHASE_ORDER_STATUS[status] || { source: "未知状态", color: 'default' };
  return <Tag color={meta.color}>{tr(meta.source)}</Tag>;
}

export default function PurchaseOrderDetailPage() {
  useLanguage();
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
  const receiptSubmittingRef = useRef(false);
  const receiptCommandRef = useRef<IdempotentCommandKey>();
  const returnCommandRef = useRef<IdempotentCommandKey>();
  const detailRead = useMemo(createTenantReadGuard, []);
  const returnItemsRead = useMemo(createTenantReadGuard, []);
  const receiptWriteGuard = useMemo(createTenantReadGuard, []);
  const returnWriteGuard = useMemo(createTenantReadGuard, []);
  const receiptIsCurrentRef = useRef<() => boolean>(() => false);
  const returnIsCurrentRef = useRef<() => boolean>(() => false);
  const returnLoadSequenceRef = useRef(0);

  const load = useCallback(async () => {
    const isCurrent = detailRead.begin();
    setLoading(true);
    setError(undefined);
    setOrder(undefined);
    setSupplierName('');
    setWarehouseName('');
    if (!id) {
      setLoading(false);
      return;
    }
    try {
      const [nextOrder, supplierResult, warehouseResult] = await Promise.all([
        getPurchaseOrder(id), listSuppliers(), listWarehouses(),
      ]);
      if (!isCurrent()) return;
      setOrder(nextOrder);
      setSupplierName(supplierResult.list.find((row) => row.id === nextOrder.supplierId)?.name || nextOrder.supplierId);
      setWarehouseName(warehouseResult.list.find((row) => row.id === nextOrder.warehouseId)?.name || nextOrder.warehouseId);
    } catch (nextError) {
      if (isCurrent()) {
        setError(procurementErrorMessage(extractProcurementAPIError(nextError), '采购单详情加载失败，请稍后重试。'));
      }
    } finally {
      if (isCurrent()) setLoading(false);
    }
  }, [detailRead, id]);

  useEffect(() => {
    void load();
    return () => {
      detailRead.invalidate();
      returnItemsRead.invalidate();
      receiptWriteGuard.invalidate();
      returnWriteGuard.invalidate();
      returnLoadSequenceRef.current += 1;
    };
  }, [detailRead, load, receiptWriteGuard, returnItemsRead, returnWriteGuard]);

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
      message.success(trFormat("{value0}成功", { value0: transition.label }));
    } catch (nextError) {
      const apiError = extractProcurementAPIError(nextError);
      message.error(tr(procurementErrorMessage(apiError)));
      if (apiError.message.toLowerCase().includes('revision conflict')) await load();
    } finally {
      setTransitionSubmitting(false);
    }
  };

  const openReceipt = () => {
    if (!order?.items) return;
    const quantities = Object.fromEntries(order.items.map((item) => [item.id, 0]));
    receiptForm.setFieldsValue({ quantities });
    receiptIsCurrentRef.current = receiptWriteGuard.begin();
    receiptCommandRef.current = undefined;
    setReceiptKey(createProcurementIdempotencyKey('goods-receipt'));
    setReceiptOpen(true);
  };

  const closeReceipt = () => {
    if (receiptSubmittingRef.current) return;
    receiptWriteGuard.invalidate();
    receiptIsCurrentRef.current = () => false;
    receiptCommandRef.current = undefined;
    setReceiptOpen(false);
  };

  const submitReceipt = async (values: ReceiptValues) => {
    if (!order?.items || receiptSubmittingRef.current || !canReceive || !receiptIsCurrentRef.current()) return;
    const items = order.items
      .map((item) => ({ purchaseOrderItemId: item.id, quantity: Number(values.quantities?.[item.id] || 0) }))
      .filter((item) => item.quantity > 0);
    if (items.length === 0) {
      message.warning(tr("请至少填写一项本次收货数量"));
      return;
    }
    const isCurrent = receiptIsCurrentRef.current;
    const payload = { orderId: order.id, expectedRevision: order.revision, items };
    const command = idempotentCommandKey(
      receiptCommandRef.current, 'goods-receipt', payload, createProcurementIdempotencyKey, receiptKey,
    );
    receiptCommandRef.current = command;
    receiptSubmittingRef.current = true;
    setReceiptSubmitting(true);
    try {
      const result = await receivePurchaseOrder(order.id, {
        expectedRevision: order.revision,
        idempotencyKey: command.key,
        items,
      });
      if (!isCurrent()) return;
      setOrder(result.purchaseOrder);
      receiptWriteGuard.invalidate();
      receiptIsCurrentRef.current = () => false;
      receiptCommandRef.current = undefined;
      setReceiptOpen(false);
      message.success(trFormat("收货已确认，收货单号：{value0}", { value0: result.receipt.receiptNo }));
    } catch (nextError) {
      if (!isCurrent()) return;
      const apiError = extractProcurementAPIError(nextError);
      message.error(tr(procurementErrorMessage(apiError, tr('收货确认失败，请核对数量后重试。'))));
      if (apiError.message.toLowerCase().includes('revision conflict')) {
        receiptWriteGuard.invalidate();
        receiptIsCurrentRef.current = () => false;
        receiptCommandRef.current = undefined;
        setReceiptOpen(false);
        await load();
      }
    } finally {
      receiptSubmittingRef.current = false;
      setReceiptSubmitting(false);
    }
  };

  const openReturn = async () => {
    if (!order) return;
    const loadSequence = ++returnLoadSequenceRef.current;
    const isCurrent = returnItemsRead.begin();
    returnIsCurrentRef.current = returnWriteGuard.begin();
    returnCommandRef.current = undefined;
    setReturnOpen(true);
    setReturnLoading(true);
    setReturnError('');
    setReturnItems([]);
    returnForm.resetFields();
    setReturnKey(createProcurementIdempotencyKey('purchase-return-create'));
    try {
      const result = await listReturnableReceiptItems(order.id);
      if (returnLoadSequenceRef.current !== loadSequence || !isCurrent()) return;
      const items = result.list || [];
      setReturnItems(items);
      returnForm.setFieldsValue({
        reason: '',
        remark: '',
        quantities: Object.fromEntries(items.map((item) => [item.goodsReceiptItemId, 0])),
      });
    } catch (nextError) {
      if (returnLoadSequenceRef.current !== loadSequence || !isCurrent()) return;
      setReturnError(procurementErrorMessage(extractProcurementAPIError(nextError), '可退收货记录加载失败，请稍后重试。'));
    } finally {
      if (returnLoadSequenceRef.current === loadSequence && isCurrent()) setReturnLoading(false);
    }
  };

  const closeReturn = () => {
    if (returnSubmittingRef.current) return;
    returnLoadSequenceRef.current += 1;
    returnItemsRead.invalidate();
    returnWriteGuard.invalidate();
    returnIsCurrentRef.current = () => false;
    returnCommandRef.current = undefined;
    setReturnOpen(false);
    setReturnLoading(false);
  };

  const submitReturn = async (values: ReturnValues) => {
    if (!order || returnSubmittingRef.current || !canManage || !returnIsCurrentRef.current() || returnLoading || Boolean(returnError)) return;
    const items = returnItems
      .map((item) => ({ goodsReceiptItemId: item.goodsReceiptItemId, quantity: Number(values.quantities?.[item.goodsReceiptItemId] || 0) }))
      .filter((item) => item.quantity > 0);
    if (items.length === 0) {
      message.warning(tr("请至少填写一项本次退货数量"));
      return;
    }
    const isCurrent = returnIsCurrentRef.current;
    const payload = { purchaseOrderId: order.id, reason: values.reason.trim(), remark: values.remark?.trim() || '', items };
    const command = idempotentCommandKey(
      returnCommandRef.current, 'purchase-return-create', payload, createProcurementIdempotencyKey, returnKey,
    );
    returnCommandRef.current = command;
    returnSubmittingRef.current = true;
    setReturnSubmitting(true);
    try {
      const purchaseReturn = await createPurchaseReturn({ ...payload, idempotencyKey: command.key });
      if (!isCurrent()) return;
      returnWriteGuard.invalidate();
      returnItemsRead.invalidate();
      returnIsCurrentRef.current = () => false;
      returnCommandRef.current = undefined;
      setReturnOpen(false);
      message.success(trFormat("采购退货草稿已创建：{value0}", { value0: purchaseReturn.returnNo }));
      history.push(`/procurement/purchase-returns/${purchaseReturn.id}`);
    } catch (nextError) {
      if (isCurrent()) message.error(tr(procurementErrorMessage(extractProcurementAPIError(nextError), tr('采购退货创建失败，请核对数量后重试。'))));
    } finally {
      returnSubmittingRef.current = false;
      setReturnSubmitting(false);
    }
  };

  const itemColumns: ProColumns<PurchaseOrderItem>[] = [
    {
      title: tr("商品"), dataIndex: 'productTitle', minWidth: 170, ellipsis: true,
      render: (_, row) => row.productTitle || tr('商品信息不可用'),
    },
    {
      title: tr("本地规格"), dataIndex: 'skuName', minWidth: 170, ellipsis: true,
      render: (_, row) => row.skuName || row.skuCode || row.productSkuId,
    },
    { title: tr("订购数量"), dataIndex: 'quantity', width: 100, align: 'right' },
    { title: tr("已收数量"), dataIndex: 'receivedQuantity', width: 100, align: 'right' },
    { title: tr("未收数量"), width: 100, align: 'right', render: (_, row) => remainingQuantity(row.quantity, row.receivedQuantity) },
    { title: tr("采购单价"), dataIndex: 'unitCostMinor', width: 140, align: 'right', render: (_, row) => formatMinorAmount(row.unitCostMinor, order?.currency) },
    { title: tr("明细金额"), dataIndex: 'lineAmountMinor', width: 150, align: 'right', render: (_, row) => formatMinorAmount(row.lineAmountMinor, order?.currency) },
  ];

  const actionButtons = useMemo(() => {
    if (!order) return null;
    return (
      <Space wrap>
        <Button icon={<ArrowLeftOutlined />} onClick={() => history.push('/procurement/purchase-orders')}>{tr("返回列表")}</Button>
        {order.status === 'draft' && canManage ? <Button type="primary" onClick={() => openTransition({ action: 'submit', label: tr("提交审批") })}>{tr("提交审批")}</Button> : null}
        {order.status === 'pending_approval' && canApprove ? <Button type="primary" onClick={() => openTransition({ action: 'approve', label: tr("审批通过") })}>{tr("审批通过")}</Button> : null}
        {canReceive && canReceivePurchaseOrder(order.status) ? <Button type="primary" onClick={openReceipt}>{tr("确认收货")}</Button> : null}
        {canManage && ['partially_received', 'received', 'closed'].includes(order.status) ? <Button onClick={() => void openReturn()}>{tr("发起退货")}</Button> : null}
        {['partially_received', 'received', 'closed'].includes(order.status) ? <Button onClick={() => history.push(`/procurement/purchase-returns?purchaseOrderId=${encodeURIComponent(order.id)}`)}>{tr("退货记录")}</Button> : null}
        {canManage && ['approved', 'partially_received'].includes(order.status) ? <Button onClick={() => openTransition({ action: 'close', label: tr("关闭采购单"), danger: true })}>{tr("关闭采购单")}</Button> : null}
        {canManage && ['draft', 'pending_approval', 'approved'].includes(order.status) && (order.items || []).every((item) => item.receivedQuantity === 0) ? (
          <Button danger onClick={() => openTransition({ action: 'cancel', label: tr("取消采购单"), danger: true })}>{tr("取消采购单")}</Button>
        ) : null}
      </Space>
    );
  }, [canApprove, canManage, canReceive, order]);

  return (
    <PermissionGuard require={PERMISSIONS.PROCUREMENT_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-procurement-page"
        title={order?.purchaseOrderNo || tr('采购单详情')}
        subTitle={tr("核对采购明细、版本和收货进度。审批与执行权限分离，过期版本会被拒绝。")}
        extra={<TmPageHeaderExtra>{actionButtons}</TmPageHeaderExtra>}
      >
        {loading ? <Alert type="info" showIcon message={tr("正在加载采购单详情…")} /> : null}
        {error ? <ErrorAlert title={tr(error)} actionHint={<Button onClick={() => void load()}>{tr("重新加载")}</Button>} /> : null}
        {order ? (
          <>
            {readonly ? <Alert type="info" showIcon message={tr("当前账号为只读模式，不能提交、审批、确认收货或发起退货。")} /> : null}
            <SectionCard title={tr("采购单信息")} description={tr("状态变更使用当前版本号提交，避免覆盖他人已完成的操作。")}>
              <Descriptions column={{ xs: 1, sm: 2, lg: 3 }}>
                <Descriptions.Item label={tr("采购单号")}><Typography.Text copyable>{order.purchaseOrderNo}</Typography.Text></Descriptions.Item>
                <Descriptions.Item label={tr("状态")}>{statusTag(order.status)}</Descriptions.Item>
                <Descriptions.Item label={tr("版本")}>{order.revision}</Descriptions.Item>
                <Descriptions.Item label={tr("供应商")}>{supplierName}</Descriptions.Item>
                <Descriptions.Item label={tr("收货仓库")}>{warehouseName}</Descriptions.Item>
                <Descriptions.Item label={tr("采购金额")}>{formatMinorAmount(order.totalAmountMinor, order.currency)}</Descriptions.Item>
                <Descriptions.Item label={tr("创建时间")}>{formatDateTime(order.createdAt)}</Descriptions.Item>
                <Descriptions.Item label={tr("审批时间")}>{order.approvedAt ? formatDateTime(order.approvedAt) : '—'}</Descriptions.Item>
                <Descriptions.Item label={tr("备注")}>{order.remark || '—'}</Descriptions.Item>
              </Descriptions>
            </SectionCard>
            <SectionCard title={tr("采购明细")} description={tr("已收数量来自不可重复记账的收货事务；未收数量不能小于零。")}>
              <TmProTable<PurchaseOrderItem>
                rowKey="id" columns={itemColumns} dataSource={order.items || []} search={false} options={false}
                pagination={false} cardBordered={false} scroll={{ x: 980 }} locale={{ emptyText: tr('采购单没有明细。') }}
              />
            </SectionCard>
          </>
        ) : null}

        <Modal
          title={transition?.label || tr('采购单操作')}
          open={Boolean(transition)}
          confirmLoading={transitionSubmitting}
          okText={transition?.label || tr('确认')}
          okButtonProps={{ danger: transition?.danger }}
          cancelText={tr("取消")}
          onCancel={() => !transitionSubmitting && setTransition(undefined)}
          onOk={() => transitionForm.submit()}
          forceRender
        >
          <Form form={transitionForm} layout="vertical" preserve={false} onFinish={(values) => void submitTransition(values)}>
            <Alert type={transition?.danger ? 'warning' : 'info'} showIcon message={trFormat("本次操作基于版本 {value0}，若采购单已更新将自动拒绝。", { value0: String(order?.revision || '—') })} />
            <Form.Item label={tr("操作说明")} name="reason" rules={[{ max: 500, message: tr("操作说明不能超过 500 个字符") }]}>
              <Input.TextArea rows={3} maxLength={500} showCount placeholder={tr("可选，用于操作日志追溯")} />
            </Form.Item>
          </Form>
        </Modal>

        <Modal
          title={tr("确认收货")}
          open={receiptOpen}
          width={760}
          confirmLoading={receiptSubmitting}
          okText={tr("确认本次收货")}
          cancelText={tr("取消")}
          onCancel={closeReceipt}
          onOk={() => receiptForm.submit()}
          forceRender
        >
          <Form form={receiptForm} layout="vertical" preserve={false} onFinish={(values) => void submitReceipt(values)}>
            <Alert type="warning" showIcon message={tr("确认后会在同一事务中增加本地聚合库存并记录仓库库存流水，不能在此页面撤销。")} />
            <div className="tm-procurement-receipt-list">
              {(order?.items || []).map((item) => {
                const remaining = remainingQuantity(item.quantity, item.receivedQuantity);
                return (
                  <div className="tm-procurement-receipt-row" key={item.id}>
                    <div>
                      <Typography.Text strong>{item.productTitle || item.productSkuId}</Typography.Text>
                      <Typography.Text type="secondary">{item.skuName || item.skuCode || tr('未命名规格')} {tr("· 未收")} {remaining}</Typography.Text>
                    </div>
                    <Form.Item
                      label={tr("本次收货")}
                      name={['quantities', item.id]}
                      rules={[{ type: 'number', min: 0, max: remaining, message: trFormat("可收数量为 0–{value0}", { value0: remaining }) }]}
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
          title={tr("发起采购退货")}
          open={returnOpen}
          width={780}
          confirmLoading={returnSubmitting}
          okText={tr("创建退货单草稿")}
          okButtonProps={{ disabled: returnLoading || Boolean(returnError) || returnItems.length === 0 }}
          cancelText={tr("取消")}
          onCancel={closeReturn}
          onOk={() => returnForm.submit()}
          forceRender
        >
          <Form form={returnForm} layout="vertical" preserve={false} onFinish={(values) => void submitReturn(values)}>
            <Alert type="info" showIcon message={tr("创建后为草稿，不会立即扣减库存；提交审批并由独立执行岗确认后才会完成出库。")} />
            <Form.Item label={tr("退货原因")} name="reason" rules={[{ required: true, whitespace: true, message: tr("请输入退货原因") }, { max: 128, message: tr("退货原因不能超过 128 个字符") }]}>
              <Input maxLength={128} placeholder={tr("例如：到货质量异常")} />
            </Form.Item>
            <Form.Item label={tr("备注")} name="remark" rules={[{ max: 520, message: tr("备注不能超过 520 个字符") }]}>
              <Input.TextArea rows={2} maxLength={520} showCount />
            </Form.Item>
            {returnLoading ? <Alert type="info" showIcon message={tr("正在加载可退收货记录…")} /> : null}
            {returnError ? <ErrorAlert title={tr(returnError)} actionHint={<Button onClick={() => void openReturn()}>{tr("重新加载")}</Button>} /> : null}
            {!returnLoading && !returnError && returnItems.length === 0 ? <EmptyState compact title={tr("没有可退数量")} description={tr("该采购单的收货数量已全部分配给有效退货单，或当前没有收货记录。")} /> : null}
            {!returnLoading && !returnError && returnItems.length > 0 ? <div className="tm-procurement-receipt-list">
              {returnItems.map((item) => <div className="tm-procurement-receipt-row" key={item.goodsReceiptItemId}>
                <div>
                  <Typography.Text strong>{item.productTitle || item.productSkuId}</Typography.Text>
                  <Typography.Text type="secondary">{item.skuName || item.skuCode || tr('未命名规格')} {tr("· 收货单")} {item.receiptNo} {tr("· 可退")} {item.remainingQuantity}</Typography.Text>
                </div>
                <Form.Item
                  label={tr("本次退货")}
                  name={['quantities', item.goodsReceiptItemId]}
                  rules={[{ type: 'number', min: 0, max: item.remainingQuantity, message: trFormat("可退数量为 0–{value0}", { value0: item.remainingQuantity }) }]}
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
