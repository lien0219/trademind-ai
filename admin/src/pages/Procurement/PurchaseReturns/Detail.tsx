import { ArrowLeftOutlined } from '@ant-design/icons';
import { history, useParams } from '@umijs/max';
import type { ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Descriptions, Form, Input, Modal, Space, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PermissionGuard from '@/components/PermissionGuard';
import { ErrorAlert, SectionCard, TmPageContainer, TmPageHeaderExtra, TmProTable } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import {
  createProcurementIdempotencyKey,
  extractProcurementAPIError,
  getPurchaseReturn,
  procurementErrorMessage,
  transitionPurchaseReturn,
  type PurchaseReturn,
  type PurchaseReturnItem,
} from '@/services/procurement';
import { PERMISSIONS } from '@/utils/permission';
import { formatDateTime } from '@/utils/formatTime';
import '../index.less';

type ReturnAction = 'submit' | 'approve' | 'complete' | 'cancel';
type ActionState = { action: ReturnAction; label: string; danger?: boolean };
type ActionValues = { reason: string };

const RETURN_STATUS: Record<string, { text: string; color: string }> = {
  draft: { text: '草稿', color: 'default' },
  pending_approval: { text: '待审批', color: 'gold' },
  approved: { text: '待执行', color: 'blue' },
  completed: { text: '已完成', color: 'success' },
  cancelled: { text: '已取消', color: 'error' },
};

export default function PurchaseReturnDetailPage() {
  const { id = '' } = useParams<{ id: string }>();
  const { can, readonly } = usePermission();
  const canManage = !readonly && can(PERMISSIONS.PROCUREMENT_MANAGE);
  const canApprove = !readonly && can(PERMISSIONS.PROCUREMENT_APPROVE);
  const canReturn = !readonly && can(PERMISSIONS.PROCUREMENT_RETURN);
  const [form] = Form.useForm<ActionValues>();
  const [row, setRow] = useState<PurchaseReturn>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [action, setAction] = useState<ActionState>();
  const [actionKey, setActionKey] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      setRow(await getPurchaseReturn(id));
    } catch (nextError) {
      setError(procurementErrorMessage(extractProcurementAPIError(nextError), '采购退货详情加载失败，请稍后重试。'));
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { void load(); }, [load]);

  const openAction = (next: ActionState) => {
    form.setFieldsValue({ reason: '' });
    setActionKey(createProcurementIdempotencyKey(`purchase-return-${next.action}`));
    setAction(next);
  };

  const submitAction = async (values: ActionValues) => {
    if (!row || !action || submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    try {
      const updated = await transitionPurchaseReturn(row.id, action.action, {
        expectedRevision: row.revision,
        idempotencyKey: actionKey,
        reason: values.reason?.trim() || '',
      });
      setRow(updated);
      setAction(undefined);
      message.success(`${action.label}成功`);
    } catch (nextError) {
      const apiError = extractProcurementAPIError(nextError);
      message.error(procurementErrorMessage(apiError, `${action.label}失败，请稍后重试。`));
      if (apiError.message.toLowerCase().includes('revision conflict')) await load();
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  };

  const columns: ProColumns<PurchaseReturnItem>[] = [
    { title: '收货单号', dataIndex: 'receiptNo', width: 190, copyable: true, ellipsis: true },
    { title: '商品', dataIndex: 'productTitle', minWidth: 170, ellipsis: true, render: (_, item) => item.productTitle || '商品信息不可用' },
    { title: '本地规格', dataIndex: 'skuName', minWidth: 170, ellipsis: true, render: (_, item) => item.skuName || item.skuCode || item.productSkuId },
    { title: '原收货数量', dataIndex: 'receiptQuantity', width: 120, align: 'right' },
    { title: '本次退货', dataIndex: 'quantity', width: 110, align: 'right' },
  ];

  const actions = useMemo(() => {
    if (!row) return null;
    return <Space wrap>
      <Button icon={<ArrowLeftOutlined />} onClick={() => history.push('/procurement/purchase-returns')}>返回列表</Button>
      {row.status === 'draft' && canManage ? <Button type="primary" onClick={() => openAction({ action: 'submit', label: '提交审批' })}>提交审批</Button> : null}
      {row.status === 'pending_approval' && canApprove ? <Button type="primary" onClick={() => openAction({ action: 'approve', label: '审批通过' })}>审批通过</Button> : null}
      {row.status === 'approved' && canReturn ? <Button type="primary" danger onClick={() => openAction({ action: 'complete', label: '执行退货', danger: true })}>执行退货</Button> : null}
      {canManage && ['draft', 'pending_approval', 'approved'].includes(row.status) ? <Button danger onClick={() => openAction({ action: 'cancel', label: '取消退货单', danger: true })}>取消退货单</Button> : null}
    </Space>;
  }, [canApprove, canManage, canReturn, row]);

  const meta = row ? RETURN_STATUS[row.status] || { text: '未知状态', color: 'default' } : undefined;
  return (
    <PermissionGuard require={PERMISSIONS.PROCUREMENT_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-procurement-page"
        title={row?.returnNo || '采购退货详情'}
        subTitle="核对原收货事实、审批状态和库存执行结果；完成后的退货单不可修改。"
        extra={<TmPageHeaderExtra>{actions}</TmPageHeaderExtra>}
      >
        {loading ? <Alert type="info" showIcon message="正在加载采购退货详情…" /> : null}
        {error ? <ErrorAlert title={error} actionHint={<Button onClick={() => void load()}>重新加载</Button>} /> : null}
        {readonly ? <Alert type="info" showIcon message="当前账号为只读模式，不能提交、审批、执行或取消采购退货。" /> : null}
        {row ? <>
          <SectionCard title="退货单信息" description="所有状态写入均提交当前版本号和稳定操作编号，重复请求不会重复扣减库存。">
            <Descriptions column={{ xs: 1, sm: 2, lg: 3 }}>
              <Descriptions.Item label="退货单号"><Typography.Text copyable>{row.returnNo}</Typography.Text></Descriptions.Item>
              <Descriptions.Item label="状态"><Tag color={meta?.color}>{meta?.text}</Tag></Descriptions.Item>
              <Descriptions.Item label="版本">{row.revision}</Descriptions.Item>
              <Descriptions.Item label="采购单"><Button type="link" size="small" onClick={() => history.push(`/procurement/purchase-orders/${row.purchaseOrderId}`)}>{row.purchaseOrderNo || row.purchaseOrderId}</Button></Descriptions.Item>
              <Descriptions.Item label="供应商">{row.supplierName || row.supplierId}</Descriptions.Item>
              <Descriptions.Item label="退货仓库">{row.warehouseName || row.warehouseId}</Descriptions.Item>
              <Descriptions.Item label="退货原因">{row.reason || '—'}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{formatDateTime(row.createdAt)}</Descriptions.Item>
              <Descriptions.Item label="完成时间">{row.completedAt ? formatDateTime(row.completedAt) : '—'}</Descriptions.Item>
              <Descriptions.Item label="备注">{row.remark || '—'}</Descriptions.Item>
            </Descriptions>
          </SectionCard>
          <SectionCard title="退货明细" description="每条明细都关联原收货记录；已完成数量持续占用原记录的可退额度。">
            <TmProTable<PurchaseReturnItem> rowKey="id" columns={columns} dataSource={row.items || []} search={false} options={false} pagination={false} cardBordered={false} scroll={{ x: 900 }} locale={{ emptyText: '退货单没有明细。' }} />
          </SectionCard>
        </> : null}

        <Modal
          title={action?.label || '采购退货操作'}
          open={Boolean(action)}
          confirmLoading={submitting}
          okText={action?.label || '确认'}
          okButtonProps={{ danger: action?.danger }}
          cancelText="取消"
          onCancel={() => !submitting && setAction(undefined)}
          onOk={() => form.submit()}
          forceRender
        >
          <Form form={form} layout="vertical" preserve={false} onFinish={(values) => void submitAction(values)}>
            <Alert
              type={action?.danger ? 'warning' : 'info'}
              showIcon
              message={action?.action === 'complete' ? '确认后会在同一事务中扣减仓库可用库存并写入不可变流水；审批人不能执行本次退货。' : `本次操作基于版本 ${row?.revision || '—'}，过期版本会被拒绝。`}
            />
            <Form.Item label="操作说明" name="reason" rules={[{ max: 128, message: '操作说明不能超过 128 个字符' }]}>
              <Input.TextArea rows={3} maxLength={128} showCount placeholder="可选，用于操作日志追溯" />
            </Form.Item>
          </Form>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
