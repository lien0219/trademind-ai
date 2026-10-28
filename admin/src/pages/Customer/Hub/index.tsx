import { tr } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { TmPageContainer } from '@/components/ui';
import { CUSTOMER_CONVERSATION_STATUS } from '@/constants/status';
import { PLATFORM_OPTIONS } from '@/constants/userFriendly';
import { getCustomerDashboard, type CustomerDashboardSummary } from '@/services/customer';
import { queryShops } from '@/services/shops';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { appendSourceToUrl } from '@/utils/urlState';
import { history } from '@umijs/max';
import { Alert, Button, Card, Col, Row, Select, Space, Spin, Statistic, Tag } from 'antd';
import { useEffect, useState } from 'react';

const HUB_QUERY_KEYS = ['platform', 'shopId', 'source'] as const;

export default function CustomerHubPage() {
  useLanguage();
  const emptyLocale = useListEmptyLocale('customerHub', { permissionScoped: true });
  const { state: urlState, setState: setUrlState } =
    useUrlQueryState<Record<(typeof HUB_QUERY_KEYS)[number], string | undefined>>(HUB_QUERY_KEYS);
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<CustomerDashboardSummary | null>(null);
  const [shopOptions, setShopOptions] = useState<{ label: string; value: string }[]>([]);

  useEffect(() => {
    void (async () => {
      setLoading(true);
      try {
        setData(await getCustomerDashboard({ platform: urlState.platform, shopId: urlState.shopId }));
      } catch {
        setData(null);
      } finally {
        setLoading(false);
      }
    })();
  }, [urlState.platform, urlState.shopId]);

  useEffect(() => {
    void (async () => {
      try {
        const res = await queryShops({ page: 1, pageSize: 500 });
        setShopOptions(
          res.list.map((s) => ({
            label: `${s.shopName} (${s.platform})`,
            value: s.id,
          })),
        );
      } catch {
        setShopOptions([]);
      }
    })();
  }, []);

  const buildConversationLink = (extraQuery: string) => {
    const sp = new URLSearchParams(extraQuery);
    if (urlState.platform) sp.set('platform', urlState.platform);
    if (urlState.shopId) sp.set('shopId', urlState.shopId);
    const qs = sp.toString();
    return appendSourceToUrl(qs ? `/customer/conversations?${qs}` : '/customer/conversations');
  };

  return (
    <TmPageContainer
      title={tr("客服中心")}
      subTitle={tr("查看待回复会话、AI 建议、受控自动回复与消息同步状态。")}
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message={tr("默认人工确认，自动回复需显式启用")}
        description={tr("默认仅生成 AI 建议；管理员可按店铺启用低风险自动回复，高风险内容始终转人工。")}
      />
      <Card size="small" style={{ marginBottom: 16 }}>
        <Space wrap>
          <Select
            allowClear
            placeholder={tr("平台")}
            style={{ width: 160 }}
            options={PLATFORM_OPTIONS}
            value={urlState.platform}
            onChange={(v) => setUrlState({ platform: v || undefined }, { replace: true })}
          />
          <Select
            allowClear
            placeholder={tr("店铺")}
            style={{ width: 220 }}
            showSearch
            optionFilterProp="label"
            options={shopOptions}
            value={urlState.shopId}
            onChange={(v) => setUrlState({ shopId: v || undefined }, { replace: true })}
          />
        </Space>
      </Card>
      <Spin spinning={loading}>
        {data ? (
          <>
            <Row gutter={[16, 16]}>
              <Col xs={24} sm={12} md={8} lg={6}>
                <Card hoverable onClick={() => history.push(buildConversationLink('replyStatus=pending'))}>
                  <Statistic title={tr("待回复会话")} value={data.pendingReplyCount} />
                </Card>
              </Col>
              <Col xs={24} sm={12} md={8} lg={6}>
                <Card>
                  <Statistic title={tr("今日新消息")} value={data.todayNewMessages} />
                </Card>
              </Col>
              <Col xs={24} sm={12} md={8} lg={6}>
                <Card hoverable onClick={() => history.push(buildConversationLink('aiSuggestionStatus=pending'))}>
                  <Statistic title={tr("AI 建议待确认")} value={data.aiSuggestionPendingCount} />
                </Card>
              </Col>
              <Col xs={24} sm={12} md={8} lg={6}>
                <Card hoverable onClick={() => history.push(buildConversationLink('sendStatus=failed'))}>
                  <Statistic title={tr("发送失败")} value={data.sendFailureCount} valueStyle={{ color: data.sendFailureCount ? 'var(--ant-color-error)' : undefined }} />
                </Card>
              </Col>
              <Col xs={24} sm={12} md={8} lg={6}>
                <Card>
                  <Statistic title={tr("未授权店铺")} value={data.unauthorizedShopCount} />
                  {data.unauthorizedShopCount > 0 ? (
                    <Button type="link" size="small" onClick={() => history.push('/settings/platforms')}>
                      {tr("前往平台授权")}
                    </Button>
                  ) : null}
                </Card>
              </Col>
              <Col xs={24} sm={12} md={8} lg={6}>
                <Card hoverable onClick={() => history.push(appendSourceToUrl('/customer/message-sync-tasks'))}>
                  <Statistic title={tr("同步任务异常(7日)")} value={data.syncTaskFailureCount} />
                </Card>
              </Col>
            </Row>
            <Card title={tr("快捷入口")} style={{ marginTop: 16 }}>
              <Row gutter={[8, 8]}>
                <Col>
                  <Button type="primary" onClick={() => history.push(buildConversationLink(''))}>
                    {tr("会话列表")}
                  </Button>
                </Col>
                <Col>
                  <Button onClick={() => history.push('/customer/message-sync-tasks')}>{tr("消息同步任务")}</Button>
                </Col>
                <Col>
                  <Button onClick={() => history.push('/customer/auto-reply-settings')}>{tr("AI 自动回复")}</Button>
                </Col>
                <Col>
                  <Button onClick={() => history.push(appendSourceToUrl('/ops/task-center/failures?taskType=customer_failure', 'taskcenter'))}>
                    {tr("失败任务中心")}
                  </Button>
                </Col>
              </Row>
            </Card>
            <Card title={tr("会话状态说明")} style={{ marginTop: 16 }} size="small">
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
