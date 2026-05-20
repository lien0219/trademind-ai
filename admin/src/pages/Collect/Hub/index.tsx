import { PageContainer } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, Col, Empty, Row, Space, Spin, Tag, Tooltip, Typography } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { CustomCollectModal } from '@/pages/Collect/components/CustomCollectModal';
import type { CollectProviderRow, CollectProviderStatus } from '@/services/collectProviders';
import { queryCollectProviders } from '@/services/collectProviders';
import {
  CUSTOM_BATCH_DISABLED_TOOLTIP,
  CUSTOM_COLLECT_CARD_DESCRIPTION,
  CUSTOM_COLLECT_CARD_NOTES,
} from '@/utils/customCollectPlatform';

const { Paragraph, Title, Text } = Typography;

const FEATURE_LABEL: Record<string, string> = {
  title: '标题',
  mainImages: '主图',
  descriptionImages: '详情图',
  attributes: '属性',
  skus: 'SKU',
};

function providerRunnableForSingleTask(status: CollectProviderStatus) {
  return status === 'available' || status === 'beta';
}

function batchRowDisabledForProvider(p: CollectProviderRow): boolean {
  return !providerRunnableForSingleTask(p.status) || !p.batchSupported;
}

function batchButtonTooltipForProvider(p: CollectProviderRow): string | undefined {
  if (!providerRunnableForSingleTask(p.status)) return '当前版本暂未开放';
  if (!p.batchSupported) {
    if (p.source === 'custom') return CUSTOM_BATCH_DISABLED_TOOLTIP;
    return p.status === 'beta' ? '测试阶段暂未开放批量' : '该平台暂不支持批量采集';
  }
  return undefined;
}

function providerCardCopy(p: CollectProviderRow): { description: string; notes: string } {
  if (p.source === 'custom') {
    return {
      description: CUSTOM_COLLECT_CARD_DESCRIPTION,
      notes: CUSTOM_COLLECT_CARD_NOTES,
    };
  }
  return { description: p.description, notes: p.notes ?? '' };
}

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
  const [customModalOpen, setCustomModalOpen] = useState(false);

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
            const runnableSingle = providerRunnableForSingleTask(p.status);
            const cardCopy = providerCardCopy(p);
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
                    {cardCopy.description}
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

                  <Space wrap size="middle" className="tm-action-space">
                    <Tooltip title={runnableSingle ? undefined : '当前版本暂未开放'}>
                      <Button
                        type="primary"
                        disabled={!runnableSingle}
                        onClick={() => {
                          if (p.source === 'custom') {
                            setCustomModalOpen(true);
                          } else {
                            history.push(`/collect/tasks?source=${encodeURIComponent(p.source)}`);
                          }
                        }}
                      >
                        立即采集
                      </Button>
                    </Tooltip>
                    <Tooltip title={batchButtonTooltipForProvider(p)}>
                      <Button disabled={batchRowDisabledForProvider(p)} onClick={() => history.push(`/collect/batches?source=${encodeURIComponent(p.source)}`)}>
                        批量采集
                      </Button>
                    </Tooltip>
                    <Button type="link" style={{ paddingLeft: 0 }} onClick={() => history.push('/settings/collector')}>
                      采集服务配置
                    </Button>
                  </Space>
                  {cardCopy.notes ? (
                    <Text type="secondary" style={{ display: 'block', marginTop: 12, fontSize: 12 }}>
                      {cardCopy.notes}
                    </Text>
                  ) : null}
                </div>
              </Col>
            );
          })}
        </Row>
      )}
      <CustomCollectModal open={customModalOpen} onClose={() => setCustomModalOpen(false)} />
    </PageContainer>
  );
}
