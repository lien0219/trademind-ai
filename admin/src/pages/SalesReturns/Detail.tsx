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
  extractSalesReturnAPIError,
  formatSalesReturnAmount,
  getSalesReturn,
  salesReturnErrorMessage,
  transitionSalesReturn,
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
  const { id = '' } = useParams<{ id: string }>();
  const { can, readonly } = usePermission();
  const canManage = !readonly && can(PERMISSIONS.SALES_RETURN_MANAGE);
  const canApprove = !readonly && can(PERMISSIONS.SALES_RETURN_APPROVE);
  const canReceive = !readonly && can(PERMISSIONS.SALES_RETURN_RECEIVE);
  const [form] = Form.useForm<ActionValues>();
  const [row, setRow] = useState<SalesReturn>();
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
      setRow(await getSalesReturn(id));
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
      message.success(`${action.label}成功`);
    } catch (nextError) {
      const apiError = extractSalesReturnAPIError(nextError);
      message.error(
        salesReturnErrorMessage(apiError, `${action.label}失败，请稍后重试。`),
      );
      if (apiError.message.toLowerCase().includes('revision conflict'))
        await load();
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  };

  const columns: ProColumns<SalesReturnItem>[] = [
    {
      title: '商品',
      dataIndex: 'productTitle',
      minWidth: 180,
      ellipsis: true,
      render: (_, item) => item.productTitle || '商品信息不可用',
    },
    {
      title: '本地规格',
      dataIndex: 'skuName',
      minWidth: 170,
      ellipsis: true,
      render: (_, item) => item.skuName || item.skuCode || item.productSkuId,
    },
    {
      title: '原扣减数量',
      dataIndex: 'deductedQuantity',
      width: 120,
      align: 'right',
    },
    { title: '本次数量', dataIndex: 'quantity', width: 110, align: 'right' },
    {
      title: '入库处置',
      dataIndex: 'disposition',
      width: 110,
      render: (_, item) =>
        row?.type === 'refund_only'
          ? '不入库'
          : item.disposition === 'damaged'
            ? '残次品'
            : '良品',
    },
    {
      title: '退款金额',
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
      row.type === 'refund_only' ? '确认退款记录' : '确认退货收货';
    return (
      <Space wrap>
        <Button
          icon={<ArrowLeftOutlined />}
          onClick={() => history.push('/orders/sales-returns')}
        >
          返回列表
        </Button>
        {row.status === 'draft' && canManage ? (
          <Button
            type="primary"
            onClick={() => openAction({ action: 'submit', label: '提交审批' })}
          >
            提交审批
          </Button>
        ) : null}
        {row.status === 'pending_approval' && canApprove ? (
          <Button
            type="primary"
            onClick={() => openAction({ action: 'approve', label: '审批通过' })}
          >
            审批通过
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
                label: '取消售后单',
                danger: true,
              })
            }
          >
            取消售后单
          </Button>
        ) : null}
      </Space>
    );
  }, [canApprove, canManage, canReceive, row]);

  const statusMeta = row
    ? SALES_RETURN_STATUS[row.status] || { text: row.status, color: 'default' }
    : undefined;
  const typeMeta = row
    ? SALES_RETURN_TYPE[row.type] || { text: row.type, color: 'default' }
    : undefined;
  const completeHint =
    row?.type === 'refund_only'
      ? '确认后将完成退款记录，不写入库存。'
      : '确认后将按明细处置结果收货到原订单仓库；审批人与收货人必须为不同账号。';

  return (
    <PermissionGuard require={PERMISSIONS.SALES_RETURN_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-sales-return-page"
        title={row?.returnNo || '售后详情'}
        subTitle="退货退款处理记录"
        extra={<TmPageHeaderExtra>{actions}</TmPageHeaderExtra>}
      >
        {loading ? (
          <Alert type="info" showIcon message="正在加载售后详情…" />
        ) : null}
        {error ? (
          <ErrorAlert
            title={error}
            actionHint={<Button onClick={() => void load()}>重新加载</Button>}
          />
        ) : null}
        {readonly ? (
          <Alert
            type="info"
            showIcon
            message="当前账号为只读模式，不能提交、审批、完成或取消售后单。"
          />
        ) : null}
        {row ? (
          <>
            <SectionCard title="售后单信息">
              <Descriptions column={{ xs: 1, sm: 2, lg: 3 }}>
                <Descriptions.Item label="售后单号">
                  <Typography.Text copyable>{row.returnNo}</Typography.Text>
                </Descriptions.Item>
                <Descriptions.Item label="类型">
                  <Tag color={typeMeta?.color}>{typeMeta?.text}</Tag>
                </Descriptions.Item>
                <Descriptions.Item label="状态">
                  <Tag color={statusMeta?.color}>{statusMeta?.text}</Tag>
                </Descriptions.Item>
                <Descriptions.Item label="订单">
                  <Button
                    type="link"
                    size="small"
                    onClick={() => history.push(`/orders/${row.orderId}`)}
                  >
                    {row.orderNo || row.orderId}
                  </Button>
                </Descriptions.Item>
                <Descriptions.Item label="原订单仓库">
                  {row.warehouseName || row.warehouseId}
                </Descriptions.Item>
                <Descriptions.Item label="退款金额">
                  {formatSalesReturnAmount(row.refundAmountMinor, row.currency)}
                </Descriptions.Item>
                <Descriptions.Item label="版本">
                  {row.revision}
                </Descriptions.Item>
                <Descriptions.Item label="售后原因">
                  {row.reason}
                </Descriptions.Item>
                <Descriptions.Item label="创建时间">
                  {formatDateTime(row.createdAt)}
                </Descriptions.Item>
                <Descriptions.Item label="完成时间">
                  {row.completedAt ? formatDateTime(row.completedAt) : '—'}
                </Descriptions.Item>
                <Descriptions.Item label="备注">
                  {row.remark || '—'}
                </Descriptions.Item>
              </Descriptions>
            </SectionCard>
            <SectionCard title="售后明细">
              <TmProTable<SalesReturnItem>
                rowKey="id"
                columns={columns}
                dataSource={row.items || []}
                search={false}
                options={false}
                pagination={false}
                cardBordered={false}
                scroll={{ x: 940 }}
                locale={{ emptyText: '售后单没有明细。' }}
              />
            </SectionCard>
          </>
        ) : null}

        <Modal
          title={action?.label || '售后操作'}
          open={Boolean(action)}
          confirmLoading={submitting}
          okText={action?.label || '确认'}
          okButtonProps={{ danger: action?.danger }}
          cancelText="取消"
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
                  : `本次操作基于版本 ${row?.revision || '—'}。`
              }
            />
            <Form.Item
              label="操作说明"
              name="reason"
              rules={[{ max: 128, message: '操作说明不能超过 128 个字符' }]}
            >
              <Input.TextArea rows={3} maxLength={128} showCount />
            </Form.Item>
          </Form>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
