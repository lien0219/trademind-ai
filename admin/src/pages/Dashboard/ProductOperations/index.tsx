import {
  BellOutlined,
  InboxOutlined,
  RobotOutlined,
  SendOutlined,
  ShoppingOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { history, useRequest } from '@umijs/max';
import { Button, Col, Descriptions, Empty, List, Row, Space, Statistic, Tag, Typography } from 'antd';
import dayjs from 'dayjs';
import type { ReactNode } from 'react';
import { useMemo } from 'react';
import { queryProductOperationDashboard, type ProductOperationDashboard } from '@/services/dashboard';

function severityColor(sev: string) {
  switch (sev) {
    case 'critical':
      return 'red';
    case 'high':
      return 'orange';
    case 'medium':
      return 'gold';
    default:
      return 'blue';
  }
}

function SummaryCard(props: {
  title: string;
  value: number;
  icon: ReactNode;
  tone?: string;
  onClick?: () => void;
}) {
  return (
    <ProCard bordered hoverable={!!props.onClick} bodyStyle={{ padding: '14px 16px' }} onClick={props.onClick}>
      <Space align="center" size={12}>
        <span
          style={{
            width: 40,
            height: 40,
            borderRadius: 10,
            background: (props.tone ?? '#2563eb') + '18',
            color: props.tone ?? '#2563eb',
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: 18,
          }}
        >
          {props.icon}
        </span>
        <Statistic title={props.title} value={props.value} valueStyle={{ fontSize: 22, fontWeight: 600 }} />
      </Space>
    </ProCard>
  );
}

export default function ProductOperationsDashboardPage() {
  const { data, loading, refresh } = useRequest(() => queryProductOperationDashboard(), { refreshDeps: [] });

  const board = data as ProductOperationDashboard | undefined;
  const summary = board?.summary;

  const recentFlat = useMemo(() => {
    if (!board?.recent) return [];
    const buckets = [
      ...(board.recent.collectedProducts ?? []).map((x) => ({ ...x, bucket: '采集' })),
      ...(board.recent.aiBatches ?? []).map((x) => ({ ...x, bucket: 'AI 批次' })),
      ...(board.recent.publishTasks ?? []).map((x) => ({ ...x, bucket: '刊登' })),
      ...(board.recent.inventoryAlerts ?? []).map((x) => ({ ...x, bucket: '库存' })),
      ...(board.recent.customerConversations ?? []).map((x) => ({ ...x, bucket: '客服' })),
      ...(board.recent.failedTasks ?? []).map((x) => ({ ...x, bucket: '失败' })),
      ...(board.recent.alerts ?? []).map((x) => ({ ...x, bucket: '告警' })),
    ];
    return buckets
      .sort((a, b) => dayjs(b.occurredAt).valueOf() - dayjs(a.occurredAt).valueOf())
      .slice(0, 12);
  }, [board]);

  return (
    <PageContainer
      title="商品运营看板"
      subTitle="只读汇总与待办入口；不调用平台、不创建任务"
      loading={loading}
      extra={
        <Button type="link" onClick={() => refresh()}>
          刷新
        </Button>
      }
    >
      <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
        聚焦 AI 商品运营 MVP：草稿、AI、发布检查、刊登、库存、客服与失败任务。点击卡片跳转到对应列表处理。
      </Typography.Paragraph>

      {summary ? (
        <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
          <Col xs={24} sm={12} md={8} lg={6} xl={4}>
            <SummaryCard
              title="商品总数"
              value={summary.totalProducts}
              icon={<ShoppingOutlined />}
              onClick={() => history.push('/product/drafts')}
            />
          </Col>
          <Col xs={24} sm={12} md={8} lg={6} xl={4}>
            <SummaryCard
              title="待 AI 处理（估）"
              value={summary.aiPendingProducts}
              icon={<RobotOutlined />}
              tone="#7c3aed"
              onClick={() => history.push('/product/drafts?missingAiTitle=1')}
            />
          </Col>
          <Col xs={24} sm={12} md={8} lg={6} xl={4}>
            <SummaryCard
              title="发布检查异常（近似）"
              value={summary.readinessBlockedProducts}
              icon={<WarningOutlined />}
              tone="#ea580c"
              onClick={() => history.push('/product/drafts?readiness=blocked')}
            />
          </Col>
          <Col xs={24} sm={12} md={8} lg={6} xl={4}>
            <SummaryCard
              title="刊登失败任务"
              value={summary.publishFailedTasks}
              icon={<SendOutlined />}
              tone="#dc2626"
              onClick={() => history.push('/product/publish-tasks?status=failed')}
            />
          </Col>
          <Col xs={24} sm={12} md={8} lg={6} xl={4}>
            <SummaryCard
              title="库存预警 SKU"
              value={summary.lowStockSkus + summary.outOfStockSkus}
              icon={<InboxOutlined />}
              tone="#0891b2"
              onClick={() => history.push('/inventory/alerts')}
            />
          </Col>
          <Col xs={24} sm={12} md={8} lg={6} xl={4}>
            <SummaryCard
              title="客服待跟进"
              value={summary.customerPendingConversations}
              icon={<BellOutlined />}
              tone="#0d9488"
              onClick={() => history.push('/customer/conversations')}
            />
          </Col>
          <Col xs={24} sm={12} md={8} lg={6} xl={4}>
            <SummaryCard
              title="失败任务（汇总）"
              value={summary.failedTaskTotal}
              icon={<WarningOutlined />}
              tone="#b91c1c"
              onClick={() => history.push('/task-center/failures')}
            />
          </Col>
        </Row>
      ) : null}

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={14}>
          <ProCard title="待办" bordered>
            <List
              dataSource={board?.todos ?? []}
              locale={{ emptyText: <Empty description="暂无待办数据" /> }}
              renderItem={(item) => (
                <List.Item
                  actions={[
                    <Button type="primary" key="go" size="small" onClick={() => history.push(item.link)}>
                      立即处理
                    </Button>,
                  ]}
                >
                  <List.Item.Meta
                    title={
                      <Space>
                        <span>{item.title}</span>
                        <Tag color={severityColor(item.severity)}>{item.severity}</Tag>
                        <Typography.Text strong>{item.count}</Typography.Text>
                      </Space>
                    }
                    description={item.description}
                  />
                </List.Item>
              )}
            />
          </ProCard>
        </Col>
        <Col xs={24} lg={10}>
          <ProCard title="快捷操作" bordered style={{ marginBottom: 16 }}>
            <Space wrap size={[8, 8]}>
              {(board?.quickLinks ?? []).map((l) => (
                <Button key={l.link} onClick={() => history.push(l.link)}>
                  {l.title}
                </Button>
              ))}
            </Space>
          </ProCard>
          {summary ? (
            <ProCard title="更多指标" bordered>
              <Descriptions column={1} size="small">
                <Descriptions.Item label="缺 AI 标题">{summary.missingAiTitleCount}</Descriptions.Item>
                <Descriptions.Item label="缺 AI 描述">{summary.missingAiDescriptionCount}</Descriptions.Item>
                <Descriptions.Item label="AI 文本任务失败">{summary.aiTaskFailedCount}</Descriptions.Item>
                <Descriptions.Item label="AI 批次运行中">{summary.aiBatchRunningCount}</Descriptions.Item>
                <Descriptions.Item label="开放会话 / 待回复">
                  {summary.customerOpenConversations} / {summary.customerPendingReplyCount}
                </Descriptions.Item>
                <Descriptions.Item label="待处理 AI 建议">{summary.aiReplySuggestionPendingCount}</Descriptions.Item>
                <Descriptions.Item label="开放告警 / 严重">
                  {summary.openAlertCount} / {summary.criticalAlertCount}
                </Descriptions.Item>
              </Descriptions>
            </ProCard>
          ) : null}
        </Col>
      </Row>

      <ProCard title="最近动态（合并时间线）" bordered style={{ marginTop: 16 }}>
        <List
          size="small"
          dataSource={recentFlat}
          locale={{ emptyText: <Empty description="暂无动态" /> }}
          renderItem={(item) => (
            <List.Item>
              <Typography.Link onClick={() => history.push(item.link)}>
                <Tag>{item.bucket}</Tag> {item.title}
                {item.subtitle ? (
                  <Typography.Text type="secondary">
                    {' '}
                    — {item.subtitle}
                  </Typography.Text>
                ) : null}
                <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
                  {dayjs(item.occurredAt).format('YYYY-MM-DD HH:mm')}
                </Typography.Text>
              </Typography.Link>
            </List.Item>
          )}
        />
      </ProCard>
    </PageContainer>
  );
}
