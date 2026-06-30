import { TmPageContainer } from '@/components/ui';
import { CUSTOMER_CONVERSATION_STATUS } from '@/constants/status';
import { getCustomerDashboard, type CustomerDashboardSummary } from '@/services/customer';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { history } from '@umijs/max';
import { Alert, Button, Card, Col, Row, Spin, Statistic, Tag } from 'antd';
import { useEffect, useState } from 'react';

export default function CustomerHubPage() {
  const emptyLocale = useListEmptyLocale('customerHub', { permissionScoped: true });
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<CustomerDashboardSummary | null>(null);

  useEffect(() => {
    void (async () => {
      setLoading(true);
      try {
        setData(await getCustomerDashboard());
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  return (
    <TmPageContainer
      title="客服中心"
      subTitle="查看待回复会话、AI 建议与消息同步状态。所有回复均需人工确认，系统不会自动发送。"
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="人工确认发送"
        description="AI 仅提供回复建议；确认后将把回复发送给客户，不会自动外发。"
      />
      <Spin spinning={loading}>
        {data ? (
          <>
            <Row gutter={[16, 16]}>
              <Col xs={24} sm={12} md={8} lg={6}>
                <Card hoverable onClick={() => history.push('/customer/conversations?pendingReply=1')}>
                  <Statistic title="待回复会话" value={data.pendingReplyCount} />
                </Card>
              </Col>
              <Col xs={24} sm={12} md={8} lg={6}>
                <Card>
                  <Statistic title="今日新消息" value={data.todayNewMessages} />
                </Card>
              </Col>
              <Col xs={24} sm={12} md={8} lg={6}>
                <Card hoverable onClick={() => history.push('/customer/conversations?hasAiSuggestion=1')}>
                  <Statistic title="AI 建议待确认" value={data.aiSuggestionPendingCount} />
                </Card>
              </Col>
              <Col xs={24} sm={12} md={8} lg={6}>
                <Card hoverable onClick={() => history.push('/customer/conversations?sendFailed=1')}>
                  <Statistic title="发送失败" value={data.sendFailureCount} valueStyle={{ color: data.sendFailureCount ? '#cf1322' : undefined }} />
                </Card>
              </Col>
              <Col xs={24} sm={12} md={8} lg={6}>
                <Card>
                  <Statistic title="未授权店铺" value={data.unauthorizedShopCount} />
                  {data.unauthorizedShopCount > 0 ? (
                    <Button type="link" size="small" onClick={() => history.push('/settings/platforms')}>
                      前往平台授权
                    </Button>
                  ) : null}
                </Card>
              </Col>
              <Col xs={24} sm={12} md={8} lg={6}>
                <Card hoverable onClick={() => history.push('/customer/message-sync-tasks')}>
                  <Statistic title="同步任务异常(7日)" value={data.syncTaskFailureCount} />
                </Card>
              </Col>
            </Row>
            <Card title="快捷入口" style={{ marginTop: 16 }}>
              <Row gutter={[8, 8]}>
                <Col>
                  <Button type="primary" onClick={() => history.push('/customer/conversations')}>
                    会话列表
                  </Button>
                </Col>
                <Col>
                  <Button onClick={() => history.push('/customer/message-sync-tasks')}>消息同步任务</Button>
                </Col>
                <Col>
                  <Button onClick={() => history.push('/ops/task-center/failures?taskType=customer_failure')}>
                    失败任务中心
                  </Button>
                </Col>
              </Row>
            </Card>
            <Card title="会话状态说明" style={{ marginTop: 16 }} size="small">
              {Object.entries(CUSTOMER_CONVERSATION_STATUS).map(([k, v]) => (
                <Tag key={k} color={v.color} style={{ marginBottom: 4 }}>
                  {v.text}
                </Tag>
              ))}
            </Card>
          </>
        ) : (
          emptyLocale.emptyText
        )}
      </Spin>
    </TmPageContainer>
  );
}
