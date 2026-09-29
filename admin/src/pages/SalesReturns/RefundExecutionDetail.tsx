import { ArrowLeftOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import { history, useParams } from '@umijs/max';
import {
  Alert,
  Button,
  Descriptions,
  Form,
  Space,
  Tag,
  Typography,
  message,
} from 'antd';
import dayjs from 'dayjs';
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
  cancelRefundExecution,
  confirmRefundFromPlatform,
  createSalesReturnIdempotencyKey,
  extractSalesReturnAPIError,
  formatSalesReturnAmount,
  getRefundExecution,
  recordRefundResult,
  salesReturnErrorMessage,
  type RefundExecution,
  type RefundExecutionEvent,
} from '@/services/salesReturns';
import { formatDateTime } from '@/utils/formatTime';
import { PERMISSIONS } from '@/utils/permission';
import {
  REFUND_EXECUTION_STATUS,
  REFUND_REVIEW_STATUS,
} from './RefundExecutions';
import RefundExecutionModals, {
  type ManualRefundValues,
  type RefundActionValues,
  type RefundLocalAction,
} from './RefundExecutionModals';
import './index.less';

const REVIEW_REASON: Record<string, string> = {
  platform_fact_missing: '尚未收到可关联的平台退款事实',
  linked_platform_fact_missing: '已关联的平台退款事实当前不可用',
  multiple_platform_refund_facts: '同一售后存在多个平台退款事实，需要人工指定后再确认',
  execution_result_pending: '平台事实已关联，等待本地登记或确认结果',
  platform_status_pending: '平台退款状态尚未终结',
  platform_fact_link_blocked: '平台事实未能安全关联到本地售后单',
  platform_fact_reconciliation_mismatch: '平台售后事实与本地售后单不一致',
  platform_fact_reconciliation_pending: '平台售后事实仍在等待安全关联',
  platform_refund_amount_or_currency_mismatch: '平台金额或币种与退款执行单不一致',
  refund_result_matched: '本地退款结果与平台事实一致',
  refund_result_mismatch: '本地退款结果与平台事实不一致',
};

export default function RefundExecutionDetailPage() {
  const { id = '' } = useParams<{ id: string }>();
  const { can, readonly } = usePermission();
  const canRefund = !readonly && can(PERMISSIONS.SALES_RETURN_REFUND);
  const [manualForm] = Form.useForm<ManualRefundValues>();
  const [actionForm] = Form.useForm<RefundActionValues>();
  const [row, setRow] = useState<RefundExecution>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [manualOpen, setManualOpen] = useState(false);
  const [localAction, setLocalAction] = useState<RefundLocalAction>();
  const [actionKey, setActionKey] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      setRow(await getRefundExecution(id));
    } catch (nextError) {
      setError(
        salesReturnErrorMessage(
          extractSalesReturnAPIError(nextError),
          '退款执行详情加载失败，请稍后重试。',
        ),
      );
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  const openManual = () => {
    manualForm.setFieldsValue({
      result: 'succeeded',
      externalRefundId: '',
      executedAt: dayjs(),
      reason: '',
    });
    setActionKey(createSalesReturnIdempotencyKey('refund-result'));
    setManualOpen(true);
  };

  const openAction = (action: RefundLocalAction) => {
    actionForm.setFieldsValue({ reason: '' });
    setActionKey(createSalesReturnIdempotencyKey(`refund-${action}`));
    setLocalAction(action);
  };

  const submitManual = async (values: ManualRefundValues) => {
    if (!row || submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    try {
      const updated = await recordRefundResult(row.id, {
        expectedRevision: row.revision,
        idempotencyKey: actionKey,
        result: values.result,
        externalRefundId: values.externalRefundId?.trim() || '',
        executedAt: values.executedAt.toISOString(),
        reason: values.reason?.trim() || '',
      });
      setRow(updated);
      setManualOpen(false);
      message.success('退款结果已登记');
    } catch (nextError) {
      const apiError = extractSalesReturnAPIError(nextError);
      message.error(
        salesReturnErrorMessage(apiError, '退款结果登记失败，请核对后重试。'),
      );
      if (apiError.message.toLowerCase().includes('revision conflict')) await load();
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  };

  const submitLocalAction = async (values: RefundActionValues) => {
    if (!row || !localAction || submittingRef.current) return;
    submittingRef.current = true;
    setSubmitting(true);
    try {
      const reason = values.reason?.trim() || '';
      const updated =
        localAction === 'confirm_platform'
          ? await confirmRefundFromPlatform(row.id, {
              expectedRevision: row.revision,
              idempotencyKey: actionKey,
              platformAfterSaleId: row.platformReview?.platformAfterSaleId || '',
              reason,
            })
          : await cancelRefundExecution(row.id, {
              expectedRevision: row.revision,
              idempotencyKey: actionKey,
              reason,
            });
      setRow(updated);
      setLocalAction(undefined);
      message.success(
        localAction === 'confirm_platform'
          ? '已按平台事实确认退款结果'
          : '退款执行单已取消',
      );
    } catch (nextError) {
      const apiError = extractSalesReturnAPIError(nextError);
      message.error(
        salesReturnErrorMessage(
          apiError,
          localAction === 'confirm_platform'
            ? '平台事实确认失败，请核对后重试。'
            : '取消退款执行单失败，请稍后重试。',
        ),
      );
      if (apiError.message.toLowerCase().includes('revision conflict')) await load();
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  };

  const eventColumns: ProColumns<RefundExecutionEvent>[] = [
    {
      title: '动作',
      dataIndex: 'action',
      width: 150,
      render: (_, event) =>
        event.action === 'record_result'
          ? '人工登记结果'
          : event.action === 'confirm_platform'
            ? '按平台事实确认'
            : event.action === 'cancel'
              ? '取消执行单'
              : '其他操作',
    },
    {
      title: '状态变化',
      width: 190,
      render: (_, event) => {
        const from = REFUND_EXECUTION_STATUS[event.fromStatus]?.text || '其他状态';
        const to = REFUND_EXECUTION_STATUS[event.toStatus]?.text || '其他状态';
        return `${from} → ${to}`;
      },
    },
    {
      title: '外部退款编号',
      dataIndex: 'externalRefundId',
      minWidth: 180,
      ellipsis: true,
      render: (_, event) => event.externalRefundId || '—',
    },
    {
      title: '说明',
      dataIndex: 'reason',
      minWidth: 200,
      ellipsis: true,
      render: (_, event) => event.reason || '—',
    },
    {
      title: '记录时间',
      dataIndex: 'createdAt',
      valueType: 'dateTime',
      width: 180,
    },
  ];

  const actions = useMemo(() => {
    if (!row) return null;
    const canFinalize = ['pending', 'unknown'].includes(row.status);
    return (
      <Space wrap>
        <Button
          icon={<ArrowLeftOutlined />}
          onClick={() => history.push('/orders/refund-executions')}
        >
          返回列表
        </Button>
        {canRefund && row.status === 'pending' ? (
          <Button type="primary" onClick={openManual}>
            人工登记结果
          </Button>
        ) : null}
        {canRefund && canFinalize && row.platformReview?.platformAfterSaleId ? (
          <Button onClick={() => openAction('confirm_platform')}>
            按平台事实确认
          </Button>
        ) : null}
        {canRefund && row.status === 'pending' ? (
          <Button danger onClick={() => openAction('cancel')}>
            取消执行单
          </Button>
        ) : null}
      </Space>
    );
  }, [canRefund, row]);

  const statusMeta = row
    ? REFUND_EXECUTION_STATUS[row.status] || { text: row.status, color: 'default' }
    : undefined;
  const reviewStatus = row?.platformReview?.status || 'pending';
  const reviewMeta = REFUND_REVIEW_STATUS[reviewStatus] || {
    text: reviewStatus,
    color: 'default',
  };

  return (
    <PermissionGuard require={PERMISSIONS.SALES_RETURN_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-sales-return-page"
        title={row?.executionNo || '退款执行详情'}
        subTitle="本页面只登记外部结果或确认只读平台事实，不会向平台发起退款"
        extra={<TmPageHeaderExtra>{actions}</TmPageHeaderExtra>}
      >
        {loading ? <Alert type="info" showIcon message="正在加载退款执行详情…" /> : null}
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
            message="当前账号为只读模式，不能登记、确认或取消退款执行单。"
          />
        ) : null}
        {row ? (
          <>
            <SectionCard title="退款执行信息">
              <Descriptions column={{ xs: 1, sm: 2, lg: 3 }}>
                <Descriptions.Item label="执行单号">
                  <Typography.Text copyable>{row.executionNo}</Typography.Text>
                </Descriptions.Item>
                <Descriptions.Item label="执行状态">
                  <Tag color={statusMeta?.color}>{statusMeta?.text}</Tag>
                </Descriptions.Item>
                <Descriptions.Item label="平台复核">
                  <Tag color={reviewMeta.color}>{reviewMeta.text}</Tag>
                </Descriptions.Item>
                <Descriptions.Item label="售后单">
                  <Button
                    type="link"
                    size="small"
                    onClick={() => history.push(`/orders/sales-returns/${row.salesReturnId}`)}
                  >
                    {row.returnNo || row.salesReturnId}
                  </Button>
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
                <Descriptions.Item label="退款金额">
                  {formatSalesReturnAmount(row.refundAmountMinor, row.currency)}
                </Descriptions.Item>
                <Descriptions.Item label="结果来源">
                  {row.source === 'platform_fact'
                    ? '平台事实'
                    : row.source === 'manual'
                      ? '人工登记'
                      : '—'}
                </Descriptions.Item>
                <Descriptions.Item label="外部退款编号">
                  {row.externalRefundId ? (
                    <Typography.Text copyable>{row.externalRefundId}</Typography.Text>
                  ) : (
                    '—'
                  )}
                </Descriptions.Item>
                <Descriptions.Item label="执行时间">
                  {row.executedAt ? formatDateTime(row.executedAt) : '—'}
                </Descriptions.Item>
                <Descriptions.Item label="结果说明">
                  {row.resultReason || '—'}
                </Descriptions.Item>
                <Descriptions.Item label="版本">{row.revision}</Descriptions.Item>
                <Descriptions.Item label="创建时间">
                  {formatDateTime(row.createdAt)}
                </Descriptions.Item>
              </Descriptions>
            </SectionCard>

            <SectionCard title="平台事实复核">
              {row.platformReview?.platformAfterSaleId ? (
                <Descriptions column={{ xs: 1, sm: 2, lg: 3 }}>
                  <Descriptions.Item label="平台">
                    {row.platformReview.platform || '—'}
                  </Descriptions.Item>
                  <Descriptions.Item label="平台状态">
                    {row.platformReview.platformStatus || '—'}
                  </Descriptions.Item>
                  <Descriptions.Item label="平台售后编号">
                    <Button
                      type="link"
                      size="small"
                      onClick={() =>
                        history.push(
                          `/orders/sales-return-reconciliation/${row.platformReview?.platformAfterSaleId}`,
                        )
                      }
                    >
                      {row.platformReview.externalAfterSaleId ||
                        row.platformReview.platformAfterSaleId}
                    </Button>
                  </Descriptions.Item>
                  <Descriptions.Item label="平台退款金额">
                    {formatSalesReturnAmount(
                      row.platformReview.refundAmountMinor || 0,
                      row.platformReview.currency || row.currency,
                    )}
                  </Descriptions.Item>
                  <Descriptions.Item label="平台更新时间">
                    {row.platformReview.platformUpdatedAt
                      ? formatDateTime(row.platformReview.platformUpdatedAt)
                      : '—'}
                  </Descriptions.Item>
                  <Descriptions.Item label="复核说明">
                    {REVIEW_REASON[row.platformReview.reason] ||
                      '需要人工核对平台事实'}
                  </Descriptions.Item>
                </Descriptions>
              ) : (
                <Alert
                  type="info"
                  showIcon
                  message="尚未收到可安全关联的平台退款事实；可先人工登记结果，后续再复核。"
                />
              )}
            </SectionCard>

            <SectionCard title="不可变操作记录">
              <TmProTable<RefundExecutionEvent>
                rowKey="id"
                columns={eventColumns}
                dataSource={row.events || []}
                search={false}
                options={false}
                pagination={false}
                cardBordered={false}
                scroll={{ x: 900 }}
                locale={{ emptyText: '暂无退款执行操作记录。' }}
              />
            </SectionCard>
          </>
        ) : null}

        <RefundExecutionModals
          manualForm={manualForm}
          actionForm={actionForm}
          manualOpen={manualOpen}
          localAction={localAction}
          submitting={submitting}
          onManualCancel={() => !submitting && setManualOpen(false)}
          onManualSubmit={(values) => void submitManual(values)}
          onActionCancel={() => !submitting && setLocalAction(undefined)}
          onActionSubmit={(values) => void submitLocalAction(values)}
        />
      </TmPageContainer>
    </PermissionGuard>
  );
}
