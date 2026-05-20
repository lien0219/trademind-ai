import { PageContainer } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Alert, Button, Col, Empty, message, Row, Space, Spin, Tag, Tooltip, Typography } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { CustomCollectModal } from '@/pages/Collect/components/CustomCollectModal';
import { PinduoduoCollectModal } from '@/pages/Collect/components/PinduoduoCollectModal';
import type { CollectProviderRow, CollectProviderStatus } from '@/services/collectProviders';
import { queryCollectProviders } from '@/services/collectProviders';
import { queryCollectRules } from '@/services/collectRules';
import {
  COLLECT_HUB_TYPE_HINT,
  CUSTOM_BATCH_DISABLED_TOOLTIP,
  CUSTOM_COLLECT_CARD_DESCRIPTION,
  CUSTOM_COLLECT_CARD_NOTES,
  DEDICATED_COLLECT_CARD_NOTES,
} from '@/utils/customCollectPlatform';
import {
  collectProviderStatusPresentation,
  CUSTOM_COLLECT_DISPLAY_FEATURES,
  CUSTOM_COLLECT_FEATURE_LABEL,
  NO_COLLECT_RULE_MESSAGE,
} from '@/utils/collectProviderStatus';
import {
  collectSettingsConfigButtonLabel,
  collectSettingsPath,
} from '@/utils/collectSettingsProvider';

const { Paragraph, Title, Text } = Typography;

const DEDICATED_FEATURE_LABEL: Record<string, string> = {
  title: '商品标题',
  price: '商品价格',
  mainImages: '商品主图',
  descriptionImages: '详情图片',
  attributes: '商品参数',
  skus: '商品规格',
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
    if (p.source === 'pinduoduo' || p.source === 'pdd') {
      return '拼多多批量采集暂未开放，请先使用单链接采集验证稳定性。';
    }
    return p.status === 'beta' ? '测试阶段暂未开放批量' : '该平台暂不支持批量采集';
  }
  return undefined;
}

function providerCardFeatures(p: CollectProviderRow): string[] {
  if (p.source === 'custom') {
    const fromApi = (p.features ?? []).filter((f) => f !== 'skus');
    if (fromApi.length > 0) return fromApi;
    return [...CUSTOM_COLLECT_DISPLAY_FEATURES];
  }
  if (p.source === 'pinduoduo' || p.source === 'pdd') {
    const fromApi = (p.features ?? []).filter((f) => f !== 'skus');
    if (fromApi.length > 0) return fromApi;
    return ['title', 'price', 'mainImages', 'descriptionImages', 'attributes'];
  }
  return p.features ?? [];
}

function featureLabelForProvider(p: CollectProviderRow, feature: string): string {
  if (p.source === 'custom') {
    return CUSTOM_COLLECT_FEATURE_LABEL[feature] ?? feature;
  }
  return DEDICATED_FEATURE_LABEL[feature] ?? feature;
}

function providerCardCopy(p: CollectProviderRow): { description: string; notes: string; typeLabel: string; typeHint: string } {
  if (p.source === 'custom') {
    return {
      description: CUSTOM_COLLECT_CARD_DESCRIPTION,
      notes: CUSTOM_COLLECT_CARD_NOTES,
      typeLabel: COLLECT_HUB_TYPE_HINT.custom.title,
      typeHint: COLLECT_HUB_TYPE_HINT.custom.summary,
    };
  }
  const notes = [DEDICATED_COLLECT_CARD_NOTES, p.notes?.trim()].filter(Boolean).join(' ');
  return {
    description: p.description,
    notes,
    typeLabel: COLLECT_HUB_TYPE_HINT.dedicated.title,
    typeHint: COLLECT_HUB_TYPE_HINT.dedicated.summary,
  };
}

async function openCustomCollectModal(
  setCustomModalOpen: (open: boolean) => void,
): Promise<void> {
  try {
    const res = await queryCollectRules({ page: 1, pageSize: 1, status: 'enabled' });
    if (!res.list?.length) {
      message.warning(NO_COLLECT_RULE_MESSAGE);
    }
  } catch {
    // 仍打开 Modal，由弹窗内引导创建规则
  }
  setCustomModalOpen(true);
}

export default function CollectHubPage() {
  const [loading, setLoading] = useState(true);
  const [providers, setProviders] = useState<CollectProviderRow[]>([]);
  const [customModalOpen, setCustomModalOpen] = useState(false);
  const [pddModalOpen, setPddModalOpen] = useState(false);

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
    const order = ['1688', 'pinduoduo', 'pdd', 'taobao', 'aliexpress', 'shein_temu', 'custom'];
    return [...providers].sort(
      (a, b) => order.indexOf(a.source) - order.indexOf(b.source) || a.name.localeCompare(b.name),
    );
  }, [providers]);

  return (
    <PageContainer title="采集中心">
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="如何选择采集器？"
        description={
          <ul style={{ margin: '4px 0 0', paddingLeft: 20 }}>
            <li>
              <Text strong>{COLLECT_HUB_TYPE_HINT.dedicated.title}</Text>
              {' — '}
              {COLLECT_HUB_TYPE_HINT.dedicated.summary}
            </li>
            <li>
              <Text strong>{COLLECT_HUB_TYPE_HINT.custom.title}</Text>
              {' — '}
              {COLLECT_HUB_TYPE_HINT.custom.summary}
            </li>
          </ul>
        }
      />
      {loading ? (
        <Spin style={{ display: 'block', marginTop: 48 }} />
      ) : sorted.length === 0 ? (
        <Empty description="暂未获取到采集器列表，请检查采集服务是否启动，或联系管理员。" />
      ) : (
        <Row gutter={[16, 16]}>
          {sorted.map((p) => {
            const tag = collectProviderStatusPresentation(p.source, p.status);
            const runnableSingle = providerRunnableForSingleTask(p.status);
            const cardCopy = providerCardCopy(p);
            const cardFeatures = providerCardFeatures(p);
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
                    <Tag color={p.source === 'custom' ? 'blue' : 'purple'}>{cardCopy.typeLabel}</Tag>
                    <Tag color={tag.color}>{tag.text}</Tag>
                  </Space>

                  <Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 8 }}>
                    {cardCopy.typeHint}
                  </Paragraph>

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
                    {cardFeatures.length ? (
                      <Space wrap size={[4, 8]}>
                        {cardFeatures.map((f) => (
                          <Tag key={f}>{featureLabelForProvider(p, f)}</Tag>
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
                            void openCustomCollectModal(setCustomModalOpen);
                          } else if (p.source === 'pinduoduo' || p.source === 'pdd') {
                            setPddModalOpen(true);
                          } else {
                            history.push(`/collect/tasks?source=${encodeURIComponent(p.source)}`);
                          }
                        }}
                      >
                        开始采集
                      </Button>
                    </Tooltip>
                    <Tooltip title={batchButtonTooltipForProvider(p)}>
                      <Button disabled={batchRowDisabledForProvider(p)} onClick={() => history.push(`/collect/batches?source=${encodeURIComponent(p.source)}`)}>
                        批量采集
                      </Button>
                    </Tooltip>
                    <Button
                      type="link"
                      style={{ paddingLeft: 0 }}
                      onClick={() => history.push(collectSettingsPath(p.source))}
                    >
                      {collectSettingsConfigButtonLabel(p.status)}
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
      <PinduoduoCollectModal open={pddModalOpen} onClose={() => setPddModalOpen(false)} />
    </PageContainer>
  );
}
