import { AuditOutlined, EyeOutlined, PlusOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { history, useSearchParams } from '@umijs/max';
import { Alert, Button, Select, Space, Tag } from 'antd';
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
  listSalesReturns,
  salesReturnErrorMessage,
  type SalesReturn,
} from '@/services/salesReturns';
import { PERMISSIONS } from '@/utils/permission';
import './index.less';

export const SALES_RETURN_STATUS: Record<
  string,
  { text: string; color: string }
> = {
  draft: { text: '草稿', color: 'default' },
  pending_approval: { text: '待审批', color: 'gold' },
  approved: { text: '待处理', color: 'blue' },
  completed: { text: '已完成', color: 'success' },
  cancelled: { text: '已取消', color: 'error' },
};

export const SALES_RETURN_TYPE: Record<
  string,
  { text: string; color: string }
> = {
  refund_only: { text: '仅退款', color: 'cyan' },
  return_refund: { text: '退货退款', color: 'purple' },
};

export default function SalesReturnsPage() {
  const { can, readonly } = usePermission();
  const canManage = !readonly && can(PERMISSIONS.SALES_RETURN_MANAGE);
  const actionRef = useRef<ActionType>();
  const [searchParams, setSearchParams] = useSearchParams();
  const orderId = searchParams.get('orderId')?.trim() || undefined;
  const [status, setStatus] = useState<string>();
  const [returnType, setReturnType] = useState<string>();
  const [error, setError] = useState('');

  useEffect(() => {
    void actionRef.current?.reload();
  }, [orderId, returnType, status]);

  const columns: ProColumns<SalesReturn>[] = [
    {
      title: '售后单号',
      dataIndex: 'returnNo',
      width: 190,
      copyable: true,
      ellipsis: true,
      render: (_, row) => (
        <Button
          type="link"
          size="small"
          onClick={() => history.push(`/orders/sales-returns/${row.id}`)}
        >
          {row.returnNo}
        </Button>
      ),
    },
    {
      title: '订单号',
      dataIndex: 'orderNo',
      width: 190,
      copyable: true,
      ellipsis: true,
      render: (_, row) => row.orderNo || row.orderId,
    },
    {
      title: '类型',
      dataIndex: 'type',
      width: 112,
      render: (_, row) => {
        const meta = SALES_RETURN_TYPE[row.type] || {
          text: row.type,
          color: 'default',
        };
        return <Tag color={meta.color}>{meta.text}</Tag>;
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 105,
      render: (_, row) => {
        const meta = SALES_RETURN_STATUS[row.status] || {
          text: row.status,
          color: 'default',
        };
        return <Tag color={meta.color}>{meta.text}</Tag>;
      },
    },
    {
      title: '退款金额',
      dataIndex: 'refundAmountMinor',
      width: 140,
      align: 'right',
      render: (_, row) =>
        formatSalesReturnAmount(row.refundAmountMinor, row.currency),
    },
    {
      title: '收货仓库',
      dataIndex: 'warehouseName',
      minWidth: 150,
      ellipsis: true,
      render: (_, row) => row.warehouseName || row.warehouseId,
    },
    { title: '明细', dataIndex: 'itemCount', width: 72, align: 'right' },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      valueType: 'dateTime',
      width: 180,
    },
    {
      title: '操作',
      valueType: 'option',
      width: 88,
      render: (_, row) => (
        <Button
          type="link"
          size="small"
          icon={<EyeOutlined />}
          onClick={() => history.push(`/orders/sales-returns/${row.id}`)}
        >
          查看
        </Button>
      ),
    },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.SALES_RETURN_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-sales-return-page"
        title="退货退款"
        subTitle="订单售后记录"
        extra={
          <TmPageHeaderExtra>
            <Button icon={<AuditOutlined />} onClick={() => history.push('/orders/sales-return-reconciliation')}>
              平台售后对账
            </Button>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              disabled={!canManage}
              onClick={() => history.push('/orders/list')}
            >
              选择订单
            </Button>
          </TmPageHeaderExtra>
        }
      >
        {readonly ? (
          <Alert
            type="info"
            showIcon
            message="当前账号为只读模式，可查看售后记录但不能发起或执行操作。"
          />
        ) : null}
        {orderId ? (
          <Alert
            type="info"
            showIcon
            message="当前仅显示指定订单的售后记录。"
            action={
              <Button size="small" onClick={() => setSearchParams({})}>
                查看全部
              </Button>
            }
          />
        ) : null}
        {error ? (
          <ErrorAlert
            title={error}
            actionHint={
              <Button onClick={() => actionRef.current?.reload()}>
                重新加载
              </Button>
            }
          />
        ) : null}
        <TmProTable<SalesReturn>
          className="tm-sales-return-table"
          rowKey="id"
          actionRef={actionRef}
          columns={columns}
          search={false}
          cardBordered
          scroll={{ x: 1320 }}
          locale={{ emptyText: error ? '售后列表暂不可用' : '暂无售后记录。' }}
          toolBarRender={() => [
            <Select
              key="type"
              allowClear
              placeholder="全部类型"
              value={returnType}
              style={{ width: 140 }}
              options={Object.entries(SALES_RETURN_TYPE).map(
                ([value, meta]) => ({ value, label: meta.text }),
              )}
              onChange={setReturnType}
            />,
            <Select
              key="status"
              allowClear
              placeholder="全部状态"
              value={status}
              style={{ width: 140 }}
              options={Object.entries(SALES_RETURN_STATUS).map(
                ([value, meta]) => ({ value, label: meta.text }),
              )}
              onChange={setStatus}
            />,
          ]}
          request={async (params) => {
            try {
              const result = await listSalesReturns({
                page: params.current,
                pageSize: params.pageSize,
                status,
                type: returnType,
                orderId,
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
                  '售后列表加载失败，请稍后重试。',
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
