import { tr } from '@/i18n';
import { ArrowLeftOutlined, ReloadOutlined } from '@ant-design/icons';
import { history, useParams } from '@umijs/max';
import { Button, Descriptions, Space, Spin, Table, Timeline, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useRef, useState } from 'react';
import { SectionCard, TmPageContainer } from '@/components/ui';
import {
  extractInventorySyncAPIError,
  getInventoryBindingHistory,
  type BindingCalibration,
  type BindingHistory,
  type InventorySyncAPIError,
  type ManualBindingDecision,
} from '@/services/inventorySync';
import { formatDateTime } from '@/utils/formatTime';
import {
  BINDING_STATUS_LABELS,
  ConfidenceText,
  StatusTag,
  copyableText,
  renderInventorySyncError,
} from './components/InventorySyncShared';

const { Text } = Typography;

export default function InventorySyncBindingDetailPage() {
  const params = useParams<{ bindingId: string }>();
  const bindingId = params.bindingId || '';
  const [historyDetail, setHistoryDetail] = useState<BindingHistory | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<InventorySyncAPIError | null>(null);
  const requestSeq = useRef(0);

  const load = useCallback(async () => {
    if (!bindingId) return;
    const seq = requestSeq.current + 1;
    requestSeq.current = seq;
    setLoading(true);
    setError(null);
    try {
      const detail = await getInventoryBindingHistory(bindingId);
      if (requestSeq.current !== seq) return;
      setHistoryDetail(detail);
    } catch (e) {
      if (requestSeq.current !== seq) return;
      setError(extractInventorySyncAPIError(e));
      setHistoryDetail(null);
    } finally {
      if (requestSeq.current === seq) setLoading(false);
    }
  }, [bindingId]);

  useEffect(() => {
    void load();
  }, [load]);

  const calibrationColumns: ColumnsType<BindingCalibration> = [
    { title: tr("时间"), dataIndex: 'createdAt', width: 170, render: (v) => formatDateTime(String(v || '')) },
    { title: tr("策略"), dataIndex: 'matchStrategy', width: 180 },
    { title: tr("候选规格"), dataIndex: 'candidateLocalSkuId', width: 170, render: (v) => copyableText(String(v || ''), 14) },
    { title: tr("置信度"), dataIndex: 'confidence', width: 100, render: (v) => <ConfidenceText value={typeof v === 'number' ? v : undefined} /> },
    { title: tr("版本"), dataIndex: 'calibrationVersion', width: 90 },
    { title: tr("状态"), dataIndex: 'status', width: 120 },
    { title: tr("原因码"), dataIndex: 'reasonCodes', render: (v) => Array.isArray(v) ? v.join(', ') : '-' },
  ];

  const decisionColumns: ColumnsType<ManualBindingDecision> = [
    { title: tr("时间"), dataIndex: 'createdAt', width: 170, render: (v) => formatDateTime(String(v || '')) },
    { title: tr("操作"), dataIndex: 'operation', width: 110 },
    { title: tr("操作者"), dataIndex: 'actorId', width: 160, render: (v) => copyableText(String(v || ''), 12) },
    { title: tr("选择规格"), dataIndex: 'selectedLocalSkuId', width: 170, render: (v) => copyableText(String(v || ''), 14) },
    { title: tr("原因码"), dataIndex: 'reasonCode', width: 190 },
    { title: 'Revision', dataIndex: 'requestRevision', width: 90 },
    { title: tr("备注"), dataIndex: 'comment', ellipsis: true },
  ];

  if (loading && !historyDetail) return <Spin fullscreen tip={tr("正在加载绑定历史")} />;

  const binding = historyDetail?.binding;

  return (
    <TmPageContainer
      title={tr("SKU 绑定历史")}
      subTitle={binding ? `Binding ${binding.id}` : bindingId}
      extra={
        <Space wrap>
          <Button icon={<ArrowLeftOutlined />} onClick={() => history.push('/ops/inventory-sync')}>{tr("返回中心")}</Button>
          <Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>{tr("刷新")}</Button>
        </Space>
      }
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {renderInventorySyncError(error)}
        {binding ? (
          <SectionCard title={tr("绑定摘要")} description={tr("绑定由库存校准或人工确认产生，不会写入真实平台库存。")} headerExtra={<StatusTag map={BINDING_STATUS_LABELS} value={binding.bindingStatus} />}>
            <Descriptions column={{ xs: 1, sm: 2, lg: 3 }} size="small">
              <Descriptions.Item label={tr("绑定 ID")}>{copyableText(binding.id, 18)}</Descriptions.Item>
              <Descriptions.Item label={tr("店铺连接")}>{copyableText(binding.shopConnectionId, 14)}</Descriptions.Item>
              <Descriptions.Item label={tr("平台")}>{binding.platform}</Descriptions.Item>
              <Descriptions.Item label={tr("外部商品")}>{copyableText(binding.externalProductId, 14)}</Descriptions.Item>
              <Descriptions.Item label={tr("外部 SKU")}>{copyableText(binding.externalSkuId, 14)}</Descriptions.Item>
              <Descriptions.Item label={tr("外部 SKU Code")}>{binding.externalSkuCode || '-'}</Descriptions.Item>
              <Descriptions.Item label={tr("本地商品")}>{copyableText(binding.localProductId, 14)}</Descriptions.Item>
              <Descriptions.Item label={tr("本地 SKU")}>{copyableText(binding.localSkuId, 14)}</Descriptions.Item>
              <Descriptions.Item label={tr("来源")}>{binding.bindingSource}</Descriptions.Item>
              <Descriptions.Item label={tr("置信度")}><ConfidenceText value={binding.confidence} /></Descriptions.Item>
              <Descriptions.Item label={tr("校准版本")}>{binding.calibrationVersion}</Descriptions.Item>
              <Descriptions.Item label="Revision">{binding.revision}</Descriptions.Item>
              <Descriptions.Item label={tr("确认人")}>{copyableText(binding.confirmedBy, 12)}</Descriptions.Item>
              <Descriptions.Item label={tr("确认时间")}>{formatDateTime(binding.confirmedAt)}</Descriptions.Item>
              <Descriptions.Item label={tr("更新时间")}>{formatDateTime(binding.updatedAt)}</Descriptions.Item>
            </Descriptions>
            <Text type="secondary">{binding.calibrationReason || tr("暂无校准说明")}</Text>
          </SectionCard>
        ) : <Text type="secondary">{tr("暂无绑定详情。")}</Text>}
        <SectionCard title={tr("历史时间线")} description={tr("只展示结构化、脱敏的校准和人工决策记录。")}>
          <Timeline
            items={[
              ...(historyDetail?.calibrations || []).map((item) => ({
                key: item.id,
                children: (
                  <Space direction="vertical" size={2}>
                    <Text strong>{tr("校准 v")}{item.calibrationVersion}</Text>
                    <Text type="secondary">{formatDateTime(item.createdAt)} / {item.matchStrategy} / {item.status}</Text>
                    <Text>{tr("候选 SKU：")}{copyableText(item.candidateLocalSkuId, 16)}{tr("；置信度：")}<ConfidenceText value={item.confidence} /></Text>
                  </Space>
                ),
              })),
              ...(historyDetail?.manualDecisions || []).map((item) => ({
                key: item.id,
                children: (
                  <Space direction="vertical" size={2}>
                    <Text strong>{tr("人工决策：")}{item.operation}</Text>
                    <Text type="secondary">{formatDateTime(item.createdAt)} / revision {item.requestRevision}</Text>
                    <Text>{tr("选择 SKU：")}{copyableText(item.selectedLocalSkuId, 16)}{tr("；原因码：")}{item.reasonCode}</Text>
                  </Space>
                ),
              })),
            ]}
          />
        </SectionCard>
        <SectionCard title={tr("校准记录")} compact>
          <Table rowKey="id" columns={calibrationColumns} dataSource={historyDetail?.calibrations || []} pagination={false} scroll={{ x: 1120 }} />
        </SectionCard>
        <SectionCard title={tr("人工决策")} compact>
          <Table rowKey="id" columns={decisionColumns} dataSource={historyDetail?.manualDecisions || []} pagination={false} scroll={{ x: 1120 }} />
        </SectionCard>
      </Space>
    </TmPageContainer>
  );
}
