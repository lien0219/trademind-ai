import { useMemo, useState } from 'react';
import { Alert, Button, Space, Tag, Typography } from 'antd';
import type { ProductDetail } from '@/services/products';
import {
  buildPinduoduoCollectAlertState,
  isPinduoduoSource,
  type CollectAlertItem,
  type CollectStatusTag,
} from '@/utils/pinduoduoCollectAlerts';

const MAX_VISIBLE_WARNINGS = 5;

type Props = {
  product: ProductDetail;
};

function tagColor(tone: CollectStatusTag['tone']): string {
  if (tone === 'success') return 'success';
  if (tone === 'warning') return 'warning';
  return 'default';
}

function WarningList({ items, expanded, onExpand }: { items: CollectAlertItem[]; expanded: boolean; onExpand: () => void }) {
  const visible = expanded ? items : items.slice(0, MAX_VISIBLE_WARNINGS);
  const hiddenCount = items.length - MAX_VISIBLE_WARNINGS;

  return (
    <>
      <ul style={{ margin: '8px 0 0', paddingLeft: 20 }}>
        {visible.map((item) => (
          <li key={item.code}>{item.message}</li>
        ))}
      </ul>
      {!expanded && hiddenCount > 0 ? (
        <Button type="link" size="small" style={{ paddingLeft: 0 }} onClick={onExpand}>
          查看更多（还有 {hiddenCount} 条）
        </Button>
      ) : null}
    </>
  );
}

function PinduoduoCollectAlerts({ product }: { product: ProductDetail }) {
  const [warnExpanded, setWarnExpanded] = useState(false);
  const [errExpanded, setErrExpanded] = useState(false);

  const state = useMemo(() => buildPinduoduoCollectAlertState(product), [product]);

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%', marginBottom: 16 }}>
      <Alert
        type="info"
        showIcon
        message={state.infoMessage}
        description={
          state.statusTags.length > 0 ? (
            <Space size={[8, 8]} wrap style={{ marginTop: 4 }}>
              {state.statusTags.map((t) => (
                <Tag key={t.key} color={tagColor(t.tone)}>
                  {t.label}
                </Tag>
              ))}
            </Space>
          ) : undefined
        }
      />

      {state.errors.length > 0 ? (
        <Alert
          type="error"
          showIcon
          message="发布前必须处理"
          description={
            <>
              <Typography.Text>以下问题未解决前，发布检查将无法通过。</Typography.Text>
              <WarningList items={state.errors} expanded={errExpanded} onExpand={() => setErrExpanded(true)} />
            </>
          }
        />
      ) : null}

      {state.warnings.length > 0 ? (
        <Alert
          type="warning"
          showIcon
          message="采集结果需要补充"
          description={
            <>
              <Typography.Text>部分字段可能需要人工补充，请发布前检查。</Typography.Text>
              <WarningList items={state.warnings} expanded={warnExpanded} onExpand={() => setWarnExpanded(true)} />
            </>
          }
        />
      ) : null}
    </Space>
  );
}

/** Unified collect-quality alerts on product draft detail (per-source). */
export function ProductCollectQualityAlert({ product }: Props) {
  if (isPinduoduoSource(product.source)) {
    return <PinduoduoCollectAlerts product={product} />;
  }
  return null;
}
