import { tr } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { AuditOutlined, EyeOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { history, useSearchParams } from '@umijs/max';
import { Alert, Button, Select, Tag } from 'antd';
import { useEffect, useRef, useState } from 'react';
import PermissionGuard from '@/components/PermissionGuard';
import {
  ErrorAlert,
  TmPageContainer,
  TmPageHeaderExtra,
  TmProTable,
} from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import {
  extractSalesReturnAPIError,
  formatSalesReturnAmount,
  listRefundExecutions,
  salesReturnErrorMessage,
  type RefundExecution,
} from '@/services/salesReturns';
import { PERMISSIONS } from '@/utils/permission';
import './index.less';

export const REFUND_EXECUTION_STATUS: Record<
  string,
  { text: string; color: string }
> = {
  pending: { get text() { return tr("待登记"); }, color: 'processing' },
  succeeded: { get text() { return tr("退款成功"); }, color: 'success' },
  failed: { get text() { return tr("退款失败"); }, color: 'error' },
  unknown: { get text() { return tr("结果待确认"); }, color: 'warning' },
  cancelled: { get text() { return tr("已取消"); }, color: 'default' },
};

export const REFUND_REVIEW_STATUS: Record<
  string,
  { text: string; color: string }
> = {
  matched: { get text() { return tr("账实一致"); }, color: 'success' },
  pending: { get text() { return tr("待平台复核"); }, color: 'processing' },
  mismatch: { get text() { return tr("账实不一致"); }, color: 'error' },
  blocked: { get text() { return tr("平台事实阻塞"); }, color: 'warning' },
};

export function getRefundExecutionStatusMeta(status: string) {
  const meta = REFUND_EXECUTION_STATUS[status];
  return meta ? { text: meta.text, color: meta.color } : { text: status, color: 'default' };
}

export function getRefundReviewStatusMeta(status: string) {
  const meta = REFUND_REVIEW_STATUS[status];
  return meta ? { text: meta.text, color: meta.color } : { text: status, color: 'default' };
}

export default function RefundExecutionsPage() {
  useLanguage();
  const { readonly } = usePermission();
  const actionRef = useRef<ActionType>();
  const [searchParams, setSearchParams] = useSearchParams();
  const salesReturnId = searchParams.get('salesReturnId')?.trim() || undefined;
  const [status, setStatus] = useState<string>();
  const [error, setError] = useState('');

  useEffect(() => {
    void actionRef.current?.reload();
  }, [salesReturnId, status]);

  const columns: ProColumns<RefundExecution>[] = [
    {
      title: tr("退款执行单号"),
      dataIndex: 'executionNo',
      width: 190,
      copyable: true,
      ellipsis: true,
      render: (_, row) => (
        <Button
          type="link"
          size="small"
          onClick={() => history.push(`/orders/refund-executions/${row.id}`)}
        >
          {row.executionNo}
        </Button>
      ),
    },
    {
      title: tr("售后单号"),
      dataIndex: 'returnNo',
      width: 180,
      ellipsis: true,
      render: (_, row) => row.returnNo || row.salesReturnId,
    },
    {
      title: tr("订单号"),
      dataIndex: 'orderNo',
      width: 180,
      ellipsis: true,
      render: (_, row) => row.orderNo || row.orderId,
    },
    {
      title: tr("执行状态"),
      dataIndex: 'status',
      width: 120,
      render: (_, row) => {
        const meta = getRefundExecutionStatusMeta(row.status);
        return <Tag color={meta.color}>{meta.text}</Tag>;
      },
    },
    {
      title: tr("平台复核"),
      dataIndex: ['platformReview', 'status'],
      width: 130,
      render: (_, row) => {
        const value = row.platformReview?.status || 'pending';
        const meta = getRefundReviewStatusMeta(value);
        return <Tag color={meta.color}>{meta.text}</Tag>;
      },
    },
    {
      title: tr("退款金额"),
      dataIndex: 'refundAmountMinor',
      width: 140,
      align: 'right',
      render: (_, row) =>
        formatSalesReturnAmount(row.refundAmountMinor, row.currency),
    },
    {
      title: tr("结果来源"),
      dataIndex: 'source',
      width: 120,
      render: (_, row) =>
        row.source === 'platform_fact'
          ? tr("平台事实")
          : row.source === 'manual'
            ? tr("人工登记")
            : '—',
    },
    {
      title: tr("创建时间"),
      dataIndex: 'createdAt',
      valueType: 'dateTime',
      width: 180,
    },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 88,
      render: (_, row) => (
        <Button
          type="link"
          size="small"
          icon={<EyeOutlined />}
          onClick={() => history.push(`/orders/refund-executions/${row.id}`)}
        >
          {tr("查看")}
        </Button>
      ),
    },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.SALES_RETURN_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-sales-return-page"
        title={tr("退款执行")}
        subTitle={tr("记录外部退款结果并用平台只读事实复核，不会向平台发起退款")}
        extra={
          <TmPageHeaderExtra>
            <Button
              icon={<AuditOutlined />}
              onClick={() => history.push('/orders/sales-return-reconciliation')}
            >
              {tr("平台售后对账")}
            </Button>
            <Button onClick={() => history.push('/orders/sales-returns')}>
              {tr("返回退货退款")}
            </Button>
          </TmPageHeaderExtra>
        }
      >
        {readonly ? (
          <Alert
            type="info"
            showIcon
            message={tr("当前账号为只读模式，可查看退款执行与平台复核结果，但不能登记或确认退款。")}
          />
        ) : null}
        {salesReturnId ? (
          <Alert
            type="info"
            showIcon
            message={tr("当前仅显示指定售后单的退款执行记录。")}
            action={
              <Button size="small" onClick={() => setSearchParams({})}>
                {tr("查看全部")}
              </Button>
            }
          />
        ) : null}
        {error ? (
          <ErrorAlert
            title={tr(error)}
            actionHint={
              <Button onClick={() => actionRef.current?.reload()}>
                {tr("重新加载")}
              </Button>
            }
          />
        ) : null}
        <TmProTable<RefundExecution>
          className="tm-sales-return-table"
          rowKey="id"
          actionRef={actionRef}
          columns={columns}
          search={false}
          cardBordered
          scroll={{ x: 1370 }}
          locale={{ emptyText: error ? tr('退款执行列表暂不可用') : tr('暂无退款执行记录。') }}
          filterBar={
            <Select
              aria-label={tr("退款执行状态")}
              allowClear
              placeholder={tr("全部执行状态")}
              value={status}
              style={{ width: 160 }}
              options={Object.entries(REFUND_EXECUTION_STATUS).map(
                ([value, meta]) => ({ value, label: meta.text }),
              )}
              onChange={setStatus}
            />
          }
          request={async (params) => {
            try {
              const result = await listRefundExecutions({
                page: params.current,
                pageSize: params.pageSize,
                status,
                salesReturnId,
              });
              setError('');
              return {
                data: result.list || [],
                success: true,
                total: result.total || 0,
              };
            } catch (nextError) {
              setError(
                salesReturnErrorMessage(
                  extractSalesReturnAPIError(nextError),
                  '退款执行列表加载失败，请稍后重试。',
                ),
              );
              return { data: [], success: false, total: 0 };
            }
          }}
        />
      </TmPageContainer>
    </PermissionGuard>
  );
}
