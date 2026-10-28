import { tr, trFormat } from '@/i18n';
import { useEffect, useRef, useState } from 'react';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Col, Modal, Row, Select, Space, Tag, Typography, message } from 'antd';
import { Link } from '@umijs/renderer-react';
import { MetricCard, TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import { PERMISSIONS } from '@/utils/permission';
import {
  migrateLegacyStock,
  queryWarehouseLedgerReconciliation,
  type WarehouseLedgerReconciliation,
  type WarehouseLedgerReconciliationRow,
} from '@/services/inventory';

const STATUS_META: Record<string, { source: string; color: string }> = {
  matched: { source: "一致", color: 'green' },
  unmigrated: { source: "待迁移", color: 'gold' },
  mismatch: { source: "不一致", color: 'red' },
};

export default function WarehouseLedgerPage() {
  const actionRef = useRef<ActionType>();
  const { can, readonly } = usePermission();
  const canOperate = can(PERMISSIONS.INVENTORY_OPERATE) && !readonly;
  const [summary, setSummary] = useState<WarehouseLedgerReconciliation | null>(null);
  const [requestError, setRequestError] = useState('');
  const [migrating, setMigrating] = useState(false);
  const [status, setStatus] = useState<string>();
  const statusInitialized = useRef(false);

  useEffect(() => {
    if (!statusInitialized.current) {
      statusInitialized.current = true;
      return;
    }
    void actionRef.current?.reload();
  }, [status]);

  const columns: ProColumns<WarehouseLedgerReconciliationRow>[] = [
    {
      title: tr("商品 / 规格"),
      dataIndex: 'productTitle',
      search: false,
      render: (_, row) => (
        <Space direction="vertical" size={0}>
          <Link to={`/product/drafts/${row.productId}?tab=inventory`}>{row.productTitle || tr("未命名商品")}</Link>
          <Typography.Text type="secondary">{row.skuCode || row.skuName || row.productSkuId}</Typography.Text>
        </Space>
      ),
    },
    { title: tr("兼容投影"), dataIndex: 'aggregateStock', width: 108, search: false },
    { title: tr("仓库在手"), dataIndex: 'warehouseOnHand', width: 108, search: false },
    { title: tr("仓库残次"), dataIndex: 'warehouseDamaged', width: 108, search: false },
    { title: tr("仓库可售"), dataIndex: 'warehouseSellable', width: 108, search: false },
    {
      title: tr("差异"),
      dataIndex: 'difference',
      width: 96,
      search: false,
      render: (_, row) => <Typography.Text type={row.difference === 0 ? undefined : 'danger'}>{row.difference}</Typography.Text>,
    },
    { title: tr("仓库数"), dataIndex: 'balanceCount', width: 88, search: false },
    {
      title: tr("对账状态"),
      dataIndex: 'status',
      width: 108,
      search: false,
      render: (_, row) => {
        const meta = STATUS_META[row.status] ?? { source: row.status || tr("未知"), color: 'default' };
        return <Tag color={meta.color}>{tr(meta.source)}</Tag>;
      },
    },
  ];

  const runMigration = () => {
    Modal.confirm({
      title: tr("迁移一批历史库存？"),
      content: tr('每次最多迁移 100 个尚未建立仓库余额的规格。优先使用启用的默认仓；没有默认仓时写入“待分配仓”。该操作不调用平台接口，也不创建后台任务进程。'),
      okText: tr("确认迁移"),
      cancelText: tr("取消"),
      onOk: async () => {
        setMigrating(true);
        try {
          const result = await migrateLegacyStock(100);
          message.success(trFormat("已迁移 {value0} 个规格，剩余 {value1} 个", { value0: result.migratedCount, value1: result.remainingCount }));
          await actionRef.current?.reload();
        } catch (error) {
          message.error((error as Error)?.message || tr('历史库存迁移失败'));
          throw error;
        } finally {
          setMigrating(false);
        }
      },
    });
  };

  return (
    <TmPageContainer
      title={tr("仓库库存账")}
      subTitle={tr("迁移历史兼容库存，并核对仓库可售合计与商品规格库存投影。")}
      extra={[
        <Button key="migrate" type="primary" disabled={!canOperate} loading={migrating} onClick={runMigration}>
          {tr("迁移一批历史库存")}
        </Button>,
      ]}
    >
      <Typography.Paragraph type="secondary">
        {tr("人工库存调整已按仓库记账。对账以“在手减残次”的可售库存为准；商品规格库存仅保留为兼容投影。本页不会同步真实平台库存，也不会自动补货。")}
      </Typography.Paragraph>
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} sm={8}>
          <MetricCard title={tr("一致")} value={summary?.matched ?? '—'} description={tr("仓库可售合计等于兼容投影")} intent="success" />
        </Col>
        <Col xs={24} sm={8}>
          <MetricCard title={tr("待迁移")} value={summary?.unmigrated ?? '—'} description={tr("尚未建立任何仓库余额")} intent="warning" />
        </Col>
        <Col xs={24} sm={8}>
          <MetricCard title={tr("不一致")} value={summary?.mismatch ?? '—'} description={tr("需停止继续迁移并人工核对")} intent="danger" />
        </Col>
      </Row>
      {requestError ? (
        <Alert
          type="error"
          showIcon
          message={tr("库存账对账加载失败")}
          description={requestError}
          action={<Button onClick={() => actionRef.current?.reload()}>{tr("重试")}</Button>}
          style={{ marginBottom: 16 }}
        />
      ) : null}
      <ProTable<WarehouseLedgerReconciliationRow>
        rowKey="productSkuId"
        actionRef={actionRef}
        columns={columns}
        search={false}
        scroll={{ x: 1100 }}
        locale={{ emptyText: tr("暂无库存账记录") }}
        filterBar={
          <Select
            aria-label={tr("库存账状态")}
            allowClear
            placeholder={tr("全部状态")}
            value={status}
            style={{ width: 140 }}
            options={Object.entries(STATUS_META).map(([value, meta]) => ({ value, label: tr(meta.source) }))}
            onChange={setStatus}
          />
        }
        request={async (params) => {
          try {
            const result = await queryWarehouseLedgerReconciliation({
              page: params.current,
              pageSize: params.pageSize,
              status,
            });
            setRequestError('');
            setSummary(result);
            return { data: result.list ?? [], success: true, total: result.total ?? 0 };
          } catch (error) {
            const errorMessage = (error as Error)?.message || '库存账对账失败';
            setRequestError(errorMessage);
            message.error(errorMessage);
            return { data: [], success: false, total: 0 };
          }
        }}
      />
    </TmPageContainer>
  );
}
