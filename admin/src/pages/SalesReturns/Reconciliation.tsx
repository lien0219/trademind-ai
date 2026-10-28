import { tr } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { AuditOutlined, EyeOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Alert, Button, Input, Select, Space, Tag } from 'antd';
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
  formatSalesReturnAmount,
  listPlatformAfterSaleReconciliation,
  type PlatformAfterSale,
} from '@/services/salesReturns';
import { PERMISSIONS } from '@/utils/permission';
import '../SalesReturns/index.less';

const RECONCILIATION_STATUS: Record<string, { text: string; color: string }> = {
  matched: { get text() { return tr('已匹配'); }, color: 'success' },
  pending: { get text() { return tr('待对账'); }, color: 'gold' },
  mismatch: { get text() { return tr('不一致'); }, color: 'error' },
  blocked: { get text() { return tr('已阻断'); }, color: 'volcano' },
};

function statusTag(status: string) {
  const meta = RECONCILIATION_STATUS[status] || { text: tr(status || '未知'), color: 'default' };
  return <Tag color={meta.color}>{meta.text}</Tag>;
}

export default function PlatformAfterSaleReconciliationPage() {
  useLanguage();
  const { readonly } = usePermission();
  const actionRef = useRef<ActionType>();
  const [platform, setPlatform] = useState<string>();
  const [reconciliationStatus, setReconciliationStatus] = useState<string>();
  const [orderNo, setOrderNo] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    void actionRef.current?.reload();
  }, [platform, reconciliationStatus, orderNo]);

  const columns: ProColumns<PlatformAfterSale>[] = [
    {
      title: tr("平台售后单号"),
      dataIndex: 'externalAfterSaleId',
      width: 190,
      copyable: true,
      ellipsis: true,
      render: (_, row) => (
        <Button
          type="link"
          size="small"
          onClick={() => history.push(`/orders/sales-return-reconciliation/${row.id}`)}
        >
          {row.externalAfterSaleId}
        </Button>
      ),
    },
    { title: tr("平台"), dataIndex: 'platform', width: 120 },
    { title: tr("平台订单号"), dataIndex: 'externalOrderId', width: 190, ellipsis: true },
    {
      title: tr("本地订单"),
      dataIndex: 'orderNo',
      width: 190,
      ellipsis: true,
      render: (_, row) =>
        row.orderId ? (
          <Button type="link" size="small" onClick={() => history.push(`/orders/${row.orderId}`)}>
            {row.orderNo || row.orderId}
          </Button>
        ) : (
          tr("未匹配")
        ),
    },
    {
      title: tr("平台状态"),
      dataIndex: 'platformStatus',
      width: 120,
      render: (_, row) => row.platformStatus || '—',
    },
    {
      title: tr("退款金额"),
      dataIndex: 'refundAmountMinor',
      width: 140,
      align: 'right',
      render: (_, row) => formatSalesReturnAmount(row.refundAmountMinor, row.currency),
    },
    {
      title: tr("对账状态"),
      dataIndex: 'reconciliationStatus',
      width: 110,
      render: (_, row) => statusTag(row.reconciliationStatus),
    },
    {
      title: tr("对账说明"),
      dataIndex: 'reconciliationReason',
      minWidth: 220,
      ellipsis: true,
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
          onClick={() => history.push(`/orders/sales-return-reconciliation/${row.id}`)}
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
        title={tr("平台售后对账")}
        subTitle={tr("平台退款事实与本地售后记录的只读核对")}
        extra={
          <TmPageHeaderExtra>
            <Button icon={<AuditOutlined />} onClick={() => history.push('/orders/sales-returns')}>
              {tr("本地售后记录")}
            </Button>
          </TmPageHeaderExtra>
        }
      >
        {readonly ? (
          <Alert type="info" showIcon message={tr("当前账号为只读模式，对账页面仅可查看平台事实。")} />
        ) : null}
        {error ? (
          <ErrorAlert title={tr(error)} actionHint={<Button onClick={() => actionRef.current?.reload()}>{tr("重新加载")}</Button>} />
        ) : null}
        <TmProTable<PlatformAfterSale>
          className="tm-sales-return-table"
          rowKey="id"
          actionRef={actionRef}
          columns={columns}
          search={false}
          cardBordered
          scroll={{ x: 1420 }}
          locale={{ emptyText: error ? tr('对账数据暂不可用') : tr('暂无平台售后事实。') }}
          filterBar={
            <Space wrap>
              <Select
                aria-label={tr("售后平台")}
                allowClear
                placeholder={tr("全部平台")}
                value={platform}
                style={{ width: 150 }}
                options={[{ value: 'douyin_shop', label: tr("抖店") }]}
                onChange={setPlatform}
              />
              <Select
                aria-label={tr("售后对账状态")}
                allowClear
                placeholder={tr("全部对账状态")}
                value={reconciliationStatus}
                style={{ width: 150 }}
                options={Object.entries(RECONCILIATION_STATUS).map(([value, meta]) => ({ value, label: meta.text }))}
                onChange={setReconciliationStatus}
              />
              <Input
                aria-label={tr("售后订单号")}
                allowClear
                placeholder={tr("搜索订单号")}
                value={orderNo}
                style={{ width: 190 }}
                onChange={(event) => setOrderNo(event.target.value)}
              />
            </Space>
          }
          request={async (params) => {
            try {
              const result = await listPlatformAfterSaleReconciliation({
                page: params.current,
                pageSize: params.pageSize,
                platform,
                reconciliationStatus,
                orderNo: orderNo.trim() || undefined,
              });
              setError('');
              return { data: result.list || [], success: true, total: result.total || 0 };
            } catch {
              setError('平台售后对账加载失败，请稍后重试。');
              return { data: [], success: false, total: 0 };
            }
          }}
        />
      </TmPageContainer>
    </PermissionGuard>
  );
}
