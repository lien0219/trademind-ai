import { ArrowLeftOutlined } from '@ant-design/icons';
import { history, useParams } from '@umijs/max';
import { Alert, Button, Descriptions, List, Tag, Typography } from 'antd';
import { useEffect, useState } from 'react';
import PermissionGuard from '@/components/PermissionGuard';
import { ErrorAlert, SectionCard, TmPageContainer, TmPageHeaderExtra } from '@/components/ui';
import {
  formatSalesReturnAmount,
  getPlatformAfterSaleReconciliation,
  type PlatformAfterSale,
} from '@/services/salesReturns';
import { PERMISSIONS } from '@/utils/permission';
import '../SalesReturns/index.less';

const STATUS_META: Record<string, { text: string; color: string }> = {
  matched: { text: '已匹配', color: 'success' },
  pending: { text: '待对账', color: 'gold' },
  mismatch: { text: '不一致', color: 'error' },
  blocked: { text: '已阻断', color: 'volcano' },
};

export default function PlatformAfterSaleReconciliationDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [row, setRow] = useState<PlatformAfterSale>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = async () => {
    if (!id) {
      setError('平台售后事实编号缺失。');
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      setRow(await getPlatformAfterSaleReconciliation(id));
      setError('');
    } catch {
      setError('平台售后事实加载失败，请稍后重试。');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, [id]);

  const status = row ? STATUS_META[row.reconciliationStatus] || { text: row.reconciliationStatus, color: 'default' } : undefined;
  return (
    <PermissionGuard require={PERMISSIONS.SALES_RETURN_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-sales-return-page"
        title={row?.externalAfterSaleId || '平台售后事实详情'}
        subTitle="只读平台售后与本地记录核对结果"
        extra={
          <TmPageHeaderExtra>
            <Button icon={<ArrowLeftOutlined />} onClick={() => history.push('/orders/sales-return-reconciliation')}>
              返回对账列表
            </Button>
          </TmPageHeaderExtra>
        }
      >
        {loading ? <Alert type="info" showIcon message="正在加载平台售后事实…" /> : null}
        {error ? <ErrorAlert title={error} actionHint={<Button onClick={() => void load()}>重新加载</Button>} /> : null}
        {row ? (
          <SectionCard title="平台事实">
            <Descriptions column={{ xs: 1, sm: 2, lg: 3 }}>
              <Descriptions.Item label="平台售后单号">{row.externalAfterSaleId}</Descriptions.Item>
              <Descriptions.Item label="平台">{row.platform}</Descriptions.Item>
              <Descriptions.Item label="平台店铺">{row.platformShopId}</Descriptions.Item>
              <Descriptions.Item label="平台订单号">{row.externalOrderId}</Descriptions.Item>
              <Descriptions.Item label="平台类型">{row.platformType}</Descriptions.Item>
              <Descriptions.Item label="平台状态">{row.platformStatus}</Descriptions.Item>
              <Descriptions.Item label="退款金额">{formatSalesReturnAmount(row.refundAmountMinor, row.currency)}</Descriptions.Item>
              <Descriptions.Item label="对账状态"><Tag color={status?.color}>{status?.text}</Tag></Descriptions.Item>
              <Descriptions.Item label="对账说明">{row.reconciliationReason || '—'}</Descriptions.Item>
              <Descriptions.Item label="平台更新时间">{row.platformUpdatedAt || '—'}</Descriptions.Item>
              <Descriptions.Item label="最近事件">{row.lastEventId}</Descriptions.Item>
              <Descriptions.Item label="接收时间">{row.updatedAt}</Descriptions.Item>
            </Descriptions>
          </SectionCard>
        ) : null}
        {row?.orderId || row?.salesReturnId ? (
          <SectionCard title="本地关联">
            <Descriptions column={{ xs: 1, sm: 2 }}>
              <Descriptions.Item label="本地订单">
                {row.orderId ? <Button type="link" onClick={() => history.push(`/orders/${row.orderId}`)}>{row.orderNo || row.orderId}</Button> : '未匹配'}
              </Descriptions.Item>
              <Descriptions.Item label="本地售后">
                {row.salesReturnId ? <Button type="link" onClick={() => history.push(`/orders/sales-returns/${row.salesReturnId}`)}>{row.returnNo || row.salesReturnId}</Button> : '未匹配'}
              </Descriptions.Item>
            </Descriptions>
          </SectionCard>
        ) : null}
        {row?.events?.length ? (
          <SectionCard title="最近事件">
            <List
              size="small"
              dataSource={row.events}
              renderItem={(event) => (
                <List.Item>
                  <List.Item.Meta
                    title={<Typography.Text copyable>{event.eventId}</Typography.Text>}
                    description={`${event.eventType} · ${event.createdAt}`}
                  />
                  <Tag color={event.applied ? 'success' : 'default'}>
                    {event.applied ? '已应用' : event.ignoredReason || '已忽略'}
                  </Tag>
                </List.Item>
              )}
            />
          </SectionCard>
        ) : null}
      </TmPageContainer>
    </PermissionGuard>
  );
}
