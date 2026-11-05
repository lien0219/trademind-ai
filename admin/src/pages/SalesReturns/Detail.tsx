import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { ArrowLeftOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import { history, useParams } from '@umijs/max';
import {
  Alert,
  Button,
  Descriptions,
  Form,
  Input,
  Modal,
  Space,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PermissionGuard from '@/components/PermissionGuard';
import {
  ErrorAlert,
  SectionCard,
  TmPageContainer,
  TmPageHeaderExtra,
  TmProTable,
} from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import {
  createSalesReturnIdempotencyKey,
  createRefundExecution,
  extractSalesReturnAPIError,
  formatSalesReturnAmount,
  getSalesReturn,
  listRefundExecutions,
  salesReturnErrorMessage,
  transitionSalesReturn,
  type RefundExecution,
  type SalesReturn,
  type SalesReturnItem,
} from '@/services/salesReturns';
import { formatDateTime } from '@/utils/formatTime';
import { PERMISSIONS } from '@/utils/permission';
import { SALES_RETURN_STATUS, SALES_RETURN_TYPE } from './index';
import './index.less';

type ReturnAction = 'submit' | 'approve' | 'complete' | 'cancel';
type ActionState = { action: ReturnAction; label: string; danger?: boolean };
type ActionValues = { reason: string };

export default function SalesReturnDetailPage() {
  const language = useLanguage();
  const { id = '' } = useParams<{ id: string }>();
  const { can, readonly } = usePermission();
  const canManage = !readonly && can(PERMISSIONS.SALES_RETURN_MANAGE);
  const canApprove = !readonly && can(PERMISSIONS.SALES_RETURN_APPROVE);
  const canReceive = !readonly && can(PERMISSIONS.SALES_RETURN_RECEIVE);
  const canRefund = !readonly && can(PERMISSIONS.SALES_RETURN_REFUND);
  const [form] = Form.useForm<ActionValues>();
  const [row, setRow] = useState<SalesReturn>();
  const [refundExecution, setRefundExecution] = useState<RefundExecution>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [action, setAction] = useState<ActionState>();
  const [actionKey, setActionKey] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [creatingRefund, setCreatingRefund] = useState(false);
  const [refundCreateOpen, setRefundCreateOpen] = useState(false);
  const [refundCreateKey, setRefundCreateKey] = useState('');
  const submittingRef = useRef(false);
  const creatingRefundRef = useRef(false);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const [detail, executions] = await Promise.all([
        getSalesReturn(id),
        listRefundExecutions({ page: 1, pageSize: 1, salesReturnId: id }),
      ]);
      setRow(detail);
      setRefundExecution(executions.list?.[0]);
    } catch (nextError) {
      setError(
        salesReturnErrorMessage(
          extractSalesReturnAPIError(nextError),
          '售后详情加载失败，请稍后重试。',
        ),
      );
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  const openAction = (next: ActionState) => {
    form.setFieldsValue({ reason: '' });
    setActionKey(
      createSalesReturnIdempotencyKey(`sales-return-${next.action}`),
    );
    setAction(next);
  };

  const submitAction = async (values: ActionValues) => {
    if (!row || !action || submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    try {
      const updated = await transitionSalesReturn(row.id, action.action, {
        expectedRevision: row.revision,
        idempotencyKey: actionKey,
        reason: values.reason?.trim() || '',
      });
      setRow(updated);
      setAction(undefined);
      message.success(trFormat("{value0}成功", { value0: tr(action.label) }));
    } catch (nextError) {
      const apiError = extractSalesReturnAPIError(nextError);
      message.error(
        tr(salesReturnErrorMessage(apiError, trFormat("{value0}失败，请稍后重试。", { value0: tr(action.label) }))),
      );
      if (apiError.message.toLowerCase().includes('revision conflict'))
        await load();
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  };

  const startRefundExecution = async () => {
    if (!row || creatingRefundRef.current) return;
    creatingRefundRef.current = true;
    setCreatingRefund(true);
    try {
      const execution = await createRefundExecution(row.id, {
        idempotencyKey: refundCreateKey,
      });
      setRefundExecution(execution);
      setRefundCreateOpen(false);
      message.success(tr("退款执行单已创建"));
      history.push(`/orders/refund-executions/${execution.id}`);
    } catch (nextError) {
      const apiError = extractSalesReturnAPIError(nextError);
      message.error(
        tr(salesReturnErrorMessage(apiError, tr('退款执行单创建失败，请稍后重试。'))),
      );
      if (apiError.message.toLowerCase().includes('already exists')) {
        history.push(`/orders/refund-executions?salesReturnId=${encodeURIComponent(row.id)}`);
      }
    } finally {
      creatingRefundRef.current = false;
      setCreatingRefund(false);
    }
  };

  const columns: ProColumns<SalesReturnItem>[] = [
    {
      title: tr("商品"),
      dataIndex: 'productTitle',
      minWidth: 180,
      ellipsis: true,
      render: (_, item) => item.productTitle || tr("商品信息不可用"),
    },
    {
      title: tr("本地规格"),
      dataIndex: 'skuName',
      minWidth: 170,
      ellipsis: true,
      render: (_, item) => item.skuName || item.skuCode || item.productSkuId,
    },
    {
      title: tr("原扣减数量"),
      dataIndex: 'deductedQuantity',
      width: 120,
      align: 'right',
    },
    { title: tr("本次数量"), dataIndex: 'quantity', width: 110, align: 'right' },
    {
      title: tr("入库处置"),
      dataIndex: 'disposition',
      width: 110,
      render: (_, item) =>
        row?.type === 'refund_only'
          ? tr("不入库")
          : item.disposition === 'damaged'
            ? tr("残次品")
            : tr("良品"),
    },
    {
      title: tr("退款金额"),
      dataIndex: 'refundAmountMinor',
      width: 140,
      align: 'right',
      render: (_, item) =>
        formatSalesReturnAmount(item.refundAmountMinor, row?.currency || ''),
    },
  ];

  const actions = useMemo(() => {
    if (!row) return null;
    const completeLabel =
      row.type === 'refund_only' ? tr('确认售后完成') : tr('确认退货收货');
    return (
      <Space wrap>
        <Button
          icon={<ArrowLeftOutlined />}
          onClick={() => history.push('/orders/sales-returns')}
        >
          {tr("返回列表")}
        </Button>
        {row.status === 'draft' && canManage ? (
          <Button
            type="primary"
            onClick={() => openAction({ action: 'submit', label: tr("提交审批") })}
          >
            {tr("提交审批")}
          </Button>
        ) : null}
        {row.status === 'pending_approval' && canApprove ? (
          <Button
            type="primary"
            onClick={() => openAction({ action: 'approve', label: tr("审批通过") })}
          >
            {tr("审批通过")}
          </Button>
        ) : null}
        {row.status === 'approved' && canReceive ? (
          <Button
            type="primary"
            onClick={() =>
              openAction({ action: 'complete', label: completeLabel })
            }
          >
            {completeLabel}
          </Button>
        ) : null}
        {canManage &&
        ['draft', 'pending_approval', 'approved'].includes(row.status) ? (
          <Button
            danger
            onClick={() =>
              openAction({
                action: 'cancel',
                label: tr("取消售后单"),
                danger: true,
              })
            }
          >
            {tr("取消售后单")}
          </Button>
        ) : null}
        {row.status === 'completed' && refundExecution ? (
          <Button
            onClick={() => history.push(`/orders/refund-executions/${refundExecution.id}`)}
          >
            {tr("查看退款执行")}
          </Button>
        ) : null}
        {row.status === 'completed' && !refundExecution && canRefund ? (
          <Button
            type="primary"
            loading={creatingRefund}
            onClick={() => {
              setRefundCreateKey(
                createSalesReturnIdempotencyKey('refund-execution-create'),
              );
              setRefundCreateOpen(true);
            }}
          >
            {tr("创建退款执行单")}
          </Button>
        ) : null}
      </Space>
    );
  }, [canApprove, canManage, canReceive, canRefund, creatingRefund, language, refundExecution, row]);

  const statusMeta = row
    ? SALES_RETURN_STATUS[row.status] || { text: row.status, color: 'default' }
    : undefined;
  const typeMeta = row
    ? SALES_RETURN_TYPE[row.type] || { text: row.type, color: 'default' }
    : undefined;
  const completeHint =
    row?.type === 'refund_only'
      ? tr('确认后仅完成本地售后流程，不代表资金已退款；请随后创建退款执行单登记真实资金结果。')
      : tr('确认后将按明细处置结果收货到原订单仓库；审批人与收货人必须为不同账号。资金退款仍需在退款执行单中登记。');

  return (
    <PermissionGuard require={PERMISSIONS.SALES_RETURN_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-sales-return-page"
        title={row?.returnNo || tr('售后详情')}
        subTitle={tr("退货退款处理记录")}
        extra={<TmPageHeaderExtra>{actions}</TmPageHeaderExtra>}
      >
        {loading ? (
          <Alert type="info" showIcon message={tr("正在加载售后详情…")} />
        ) : null}
        {error ? (
          <ErrorAlert
            title={tr(error)}
            actionHint={<Button onClick={() => void load()}>{tr("重新加载")}</Button>}
          />
        ) : null}
        {readonly ? (
          <Alert
            type="info"
            showIcon
            message={tr("当前账号为只读模式，不能提交、审批、完成或取消售后单。")}
          />
        ) : null}
        {row ? (
          <>
            <SectionCard title={tr("售后单信息")}>
              <Descriptions column={{ xs: 1, sm: 2, lg: 3 }}>
                <Descriptions.Item label={tr("售后单号")}>
                  <Typography.Text copyable>{row.returnNo}</Typography.Text>
                </Descriptions.Item>
                <Descriptions.Item label={tr("类型")}>
                  <Tag color={typeMeta?.color}>{typeMeta?.text}</Tag>
                </Descriptions.Item>
                <Descriptions.Item label={tr("状态")}>
                  <Tag color={statusMeta?.color}>{statusMeta?.text}</Tag>
                </Descriptions.Item>
                <Descriptions.Item label={tr("订单")}>
                  <Button
                    type="link"
                    size="small"
                    onClick={() => history.push(`/orders/${row.orderId}`)}
                  >
                    {row.orderNo || row.orderId}
                  </Button>
                </Descriptions.Item>
                <Descriptions.Item label={tr("原订单仓库")}>
                  {row.warehouseName || row.warehouseId}
                </Descriptions.Item>
                <Descriptions.Item label={tr("退款金额")}>
                  {formatSalesReturnAmount(row.refundAmountMinor, row.currency)}
                </Descriptions.Item>
                <Descriptions.Item label={tr("版本")}>
                  {row.revision}
                </Descriptions.Item>
                <Descriptions.Item label={tr("售后原因")}>
                  {row.reason}
                </Descriptions.Item>
                <Descriptions.Item label={tr("创建时间")}>
                  {formatDateTime(row.createdAt)}
                </Descriptions.Item>
                <Descriptions.Item label={tr("完成时间")}>
                  {row.completedAt ? formatDateTime(row.completedAt) : '—'}
                </Descriptions.Item>
                <Descriptions.Item label={tr("备注")}>
                  {row.remark || '—'}
                </Descriptions.Item>
              </Descriptions>
            </SectionCard>
            <SectionCard title={tr("售后明细")}>
              <TmProTable<SalesReturnItem>
                rowKey="id"
                columns={columns}
                dataSource={row.items || []}
                search={false}
                options={false}
                pagination={false}
                cardBordered={false}
                scroll={{ x: 940 }}
                locale={{ emptyText: tr('售后单没有明细。') }}
              />
            </SectionCard>
          </>
        ) : null}

        <Modal
          title={action?.label || tr('售后操作')}
          open={Boolean(action)}
          confirmLoading={submitting}
          okText={action?.label || tr('确认')}
          okButtonProps={{ danger: action?.danger }}
          cancelText={tr("取消")}
          onCancel={() => !submitting && setAction(undefined)}
          onOk={() => form.submit()}
          forceRender
        >
          <Form
            form={form}
            layout="vertical"
            preserve={false}
            onFinish={(values) => void submitAction(values)}
          >
            <Alert
              type={action?.danger ? 'warning' : 'info'}
              showIcon
              message={
                action?.action === 'complete'
                  ? completeHint
                  : trFormat('本次操作基于版本 {value0}。', { value0: row?.revision || '—' })
              }
            />
            <Form.Item
              label={tr("操作说明")}
              name="reason"
              rules={[{ max: 128, message: tr("操作说明不能超过 128 个字符") }]}
            >
              <Input.TextArea rows={3} maxLength={128} showCount />
            </Form.Item>
          </Form>
        </Modal>
        <Modal
          title={tr("创建退款执行单")}
          open={refundCreateOpen}
          confirmLoading={creatingRefund}
          okText={tr("确认创建")}
          cancelText={tr("取消")}
          onCancel={() => !creatingRefund && setRefundCreateOpen(false)}
          onOk={() => void startRefundExecution()}
        >
          <Alert
            type="info"
            showIcon
            message={tr("创建后将进入待登记状态；本操作不会向平台或支付渠道发起退款。")}
          />
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
