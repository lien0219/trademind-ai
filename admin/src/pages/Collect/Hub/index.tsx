import { PageContainer } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, Col, Empty, Row, Space, Spin, Tag, Tooltip, Typography } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import type { CollectProviderRow, CollectProviderStatus } from '@/services/collectProviders';
import { queryCollectProviders } from '@/services/collectProviders';

const { Paragraph, Title, Text } = Typography;

const FEATURE_LABEL: Record<string, string> = {
  title: '标题',
  mainImages: '主图',
  descriptionImages: '详情图',
  attributes: '属性',
  skus: 'SKU',
};

function providerStatusPresentation(status: CollectProviderStatus) {
  switch (status) {
    case 'available':
      return { text: '已可用', color: 'success' as const };
    case 'beta':
      return { text: '测试中', color: 'processing' as const };
    case 'planned':
      return { text: '规划中', color: 'default' as const };
    case 'disabled':
      return { text: '已禁用', color: 'error' as const };
    default:
      return { text: status, color: 'default' as const };
  }
}

export default function CollectHubPage() {
  const [loading, setLoading] = useState(true);
  const [providers, setProviders] = useState<CollectProviderRow[]>([]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      setLoading(true);
      try {
        const rows = await queryCollectProviders();
        if (!cancelled) setProviders(Array.isArray(rows) ? rows : []);
      } catch {
        if (!cancelled) setProviders([]);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const sorted = useMemo(() => {
    const order = ['1688', 'pdd', 'taobao', 'aliexpress', 'shein_temu', 'custom'];
    return [...providers].sort(
      (a, b) => order.indexOf(a.source) - order.indexOf(b.source) || a.name.localeCompare(b.name),
    );
  }, [providers]);

  return (
    <PageContainer title="采集中心">
      {loading ? (
        <Spin style={{ display: 'block', marginTop: 48 }} />
      ) : sorted.length === 0 ? (
        <Empty description="暂未获取到采集器配置，请检查采集服务或与管理员确认。" />
      ) : (
        <Row gutter={[16, 16]}>
          {sorted.map((p) => {
            const tag = providerStatusPresentation(p.status);
            const runnable = p.status === 'available';
            return (
              <Col xs={24} sm={24} md={12} lg={12} xl={8} key={p.source}>
                <div
                  style={{
                    height: '100%',
                    padding: '20px 20px 16px',
                    borderRadius: 8,
                    border: '1px solid rgba(0,0,0,0.06)',
                    background: '#fff',
                    display: 'flex',
                    flexDirection: 'column',
                  }}
                >
                  <Space align="start" wrap size="middle" style={{ marginBottom: 12 }}>
                    <Title level={5} style={{ margin: 0 }}>
                      {p.name}
                    </Title>
                    <Tag color={tag.color}>{tag.text}</Tag>
                  </Space>

                  <Paragraph type="secondary" style={{ flex: 1, marginBottom: 12 }}>
                    {p.description}
                  </Paragraph>

                  <div style={{ marginBottom: 8 }}>
                    <Text strong>URL 示例</Text>
                  </div>
                  <Paragraph type="secondary" style={{ fontSize: 12, wordBreak: 'break-all', marginBottom: 12 }}>
                    {(p.urlPatterns?.length ?? 0) > 0 ? p.urlPatterns.join(' · ') : '—'}
                  </Paragraph>

                  <div style={{ marginBottom: 8 }}>
                    <Text strong>支持能力</Text>
                  </div>
                  <div style={{ marginBottom: 16 }}>
                    {p.features?.length ? (
                      <Space wrap size={[4, 8]}>
                        {p.features.map((f) => (
                          <Tag key={f}>{FEATURE_LABEL[f] ?? f}</Tag>
                        ))}
                      </Space>
                    ) : (
                      <Text type="secondary">后续支持更多字段抽取</Text>
                    )}
                  </div>

                  <Space wrap>
                    <Tooltip title={runnable ? undefined : '当前版本暂未开放'}>
                      <Button
                        type="primary"
                        disabled={!runnable}
                        onClick={() => history.push(`/collect/tasks?source=${encodeURIComponent(p.source)}`)}
                      >
                        立即采集
                      </Button>
                    </Tooltip>
                    <Tooltip
                      title={
                        !runnable ? '当前版本暂未开放' : !p.batchSupported ? '该平台暂不支持批量采集' : undefined
                      }
                    >
                      <Button
                        disabled={!runnable || !p.batchSupported}
                        onClick={() => history.push(`/collect/batches?source=${encodeURIComponent(p.source)}`)}
                      >
                        批量采集
                      </Button>
                    </Tooltip>
                    <Button type="link" style={{ paddingLeft: 0 }} onClick={() => history.push('/settings/collector')}>
                      采集服务配置
                    </Button>
                  </Space>
                  {p.notes ? (
                    <Text type="secondary" style={{ display: 'block', marginTop: 12, fontSize: 12 }}>
                      {p.notes}
                    </Text>
                  ) : null}
                </div>
              </Col>
            );
          })}
        </Row>
      )}
    </PageContainer>
  );
}
