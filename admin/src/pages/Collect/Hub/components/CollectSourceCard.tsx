import { Button, Space, Tag, Tooltip, Typography } from 'antd';
import type { ReactNode } from 'react';
import type { CollectProviderRow } from '@/services/collectProviders';
import type { CollectProviderStatusPresentation } from '@/utils/collectProviderStatus';

const { Paragraph, Text, Title } = Typography;

export type CollectSourceCardCopy = {
  description: string;
  notes: string;
  typeLabel: string;
  typeHint: string;
};

export type CollectSourceCardFeature = {
  key: string;
  label: string;
};

export type CollectSourceCardProps = {
  provider: CollectProviderRow;
  copy: CollectSourceCardCopy;
  statusTag: CollectProviderStatusPresentation;
  features: CollectSourceCardFeature[];
  singleDisabled: boolean;
  singleTooltip?: string;
  batchDisabled: boolean;
  batchTooltip?: string;
  onSingleCollect: () => void;
  onBatchCollect: () => void;
  onSettings: () => void;
  settingsLabel: ReactNode;
  showSettings?: boolean;
};

export default function CollectSourceCard({
  provider,
  copy,
  statusTag,
  features,
  singleDisabled,
  singleTooltip,
  batchDisabled,
  batchTooltip,
  onSingleCollect,
  onBatchCollect,
  onSettings,
  settingsLabel,
  showSettings = true,
}: CollectSourceCardProps) {
  return (
    <article className="tm-collect-source-card">
      <div className="tm-collect-source-card__head">
        <div className="tm-collect-source-card__title-group">
          <Title level={5} className="tm-collect-source-card__title">
            {provider.name}
          </Title>
          <Text type="secondary" className="tm-collect-source-card__source">
            {provider.source}
          </Text>
        </div>
        <Space wrap size={6}>
          <Tag color={provider.source === 'custom' ? 'blue' : 'purple'}>{copy.typeLabel}</Tag>
          <Tag color={statusTag.color}>{statusTag.text}</Tag>
        </Space>
      </div>

      {copy.typeHint ? (
        <Text type="secondary" className="tm-collect-source-card__hint">
          {copy.typeHint}
        </Text>
      ) : null}

      <Paragraph type="secondary" className="tm-collect-source-card__desc">
        {copy.description}
      </Paragraph>

      <div className="tm-collect-source-card__meta">
        <Text strong>URL 示例</Text>
        <Text type="secondary" className="tm-collect-source-card__url">
          {(provider.urlPatterns?.length ?? 0) > 0 ? provider.urlPatterns.join(' · ') : '暂无示例'}
        </Text>
      </div>

      <div className="tm-collect-source-card__features">
        <Text strong>支持能力</Text>
        {features.length ? (
          <Space wrap size={[4, 8]}>
            {features.map((feature) => (
              <Tag key={feature.key}>{feature.label}</Tag>
            ))}
          </Space>
        ) : (
          <Text type="secondary">后续支持更多字段抽取</Text>
        )}
      </div>

      <div className="tm-collect-source-card__footer">
        <Space wrap size={8}>
          <Tooltip title={singleTooltip}>
            <Button type="primary" disabled={singleDisabled} onClick={onSingleCollect}>
              开始采集
            </Button>
          </Tooltip>
          <Tooltip title={batchTooltip}>
            <Button disabled={batchDisabled} onClick={onBatchCollect}>
              批量采集
            </Button>
          </Tooltip>
          {showSettings ? <Button type="link" onClick={onSettings}>
            {settingsLabel}
          </Button> : null}
        </Space>
        {copy.notes ? (
          <Text type="secondary" className="tm-collect-source-card__notes">
            {copy.notes}
          </Text>
        ) : null}
      </div>
    </article>
  );
}
