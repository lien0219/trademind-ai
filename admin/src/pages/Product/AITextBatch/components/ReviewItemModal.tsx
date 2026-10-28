import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import TechnicalDetails from '@/components/ui/TechnicalDetails';
import type { AIProductTextItemRow } from '@/services/aiProductText';
import { Link } from '@umijs/max';
import { Alert, Button, Input, Modal, Space, Typography, message } from 'antd';
import { useEffect, useState } from 'react';

type Props = {
  open: boolean;
  item: AIProductTextItemRow | null;
  loading?: boolean;
  onClose: () => void;
  onApply: (text: string) => Promise<void>;
  onRegenerate: () => Promise<void>;
  onReject: () => Promise<void>;
};

export default function ReviewItemModal({
  open,
  item,
  loading,
  onClose,
  onApply,
  onRegenerate,
  onReject,
}: Props) {
  useLanguage();
  const [applyText, setApplyText] = useState('');

  useEffect(() => {
    if (item) {
      setApplyText(item.prepareApplyText || item.editedText || item.generatedText || '');
    }
  }, [item]);

  if (!item) return null;

  const canApply = item.status === 'pending_review' || item.status === 'success';

  return (
    <Modal
      open={open}
      title={trFormat("复核：{value0}", { value0: String(item.operationLabel) })}
      width={Math.min(1100, window.innerWidth - 48)}
      onCancel={onClose}
      footer={
        <Space wrap>
          <Button onClick={onClose}>{tr("关闭")}</Button>
          <Button danger disabled={!canApply || loading} onClick={() => void onReject()}>
            {tr("放弃该建议")}
          </Button>
          <Button disabled={loading} onClick={() => void onRegenerate()}>
            {tr("重新生成")}
          </Button>
          <Button
            type="primary"
            disabled={!canApply || loading || !applyText.trim()}
            onClick={() => void onApply(applyText)}
          >
            {tr("应用")}
          </Button>
        </Space>
      }
    >
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
          gap: 16,
        }}
      >
        <div>
          <Typography.Text type="secondary">{tr("当前商品内容")}</Typography.Text>
          <Input.TextArea value={item.currentContent || '—'} rows={8} readOnly />
        </div>
        <div>
          <Typography.Text type="secondary">{tr("AI 建议内容")}</Typography.Text>
          <Input.TextArea value={item.generatedText || '—'} rows={8} readOnly />
          <Button
            type="link"
            size="small"
            onClick={() => {
              void navigator.clipboard.writeText(item.generatedText || '');
              message.success(tr("已复制 AI 文案"));
            }}
          >
            {tr("复制 AI 文案")}
          </Button>
        </div>
        <div>
          <Typography.Text type="secondary">{tr("准备应用内容")}</Typography.Text>
          <Input.TextArea
            value={applyText}
            rows={8}
            onChange={(e) => setApplyText(e.target.value)}
            disabled={!canApply}
          />
        </div>
      </div>

      {item.qualityWarnings?.length ? (
        <Alert
          type="warning"
          showIcon
          style={{ marginTop: 12 }}
          message={tr("质量提醒")}
          description={
            <ul style={{ margin: 0, paddingLeft: 20 }}>
              {item.qualityWarnings.map((w) => (
                <li key={w.code}>{w.message}</li>
              ))}
            </ul>
          }
        />
      ) : null}

      {item.status === 'conflict' && (
        <Alert
          type="error"
          showIcon
          style={{ marginTop: 12 }}
          message={tr("商品内容在 AI 建议生成后已经被修改")}
          description={tr("为避免覆盖人工修改，请重新对比后再应用，或重新生成、放弃该建议。")}
        />
      )}

      <Space style={{ marginTop: 12 }}>
        <Link to={`/product/drafts/${item.productId}?tab=ai`}>{tr("查看商品详情")}</Link>
      </Space>

      <TechnicalDetails label={tr("技术详情")}>
        <pre style={{ fontSize: 12, margin: 0 }}>
          {JSON.stringify(
            {
              处理类型: item.operationType,
              内容快照: item.sourceSnapshotHash,
              内容已变化: item.status === 'conflict' ? '是' : '否',
              aiTaskId: item.aiTaskId,
            },
            null,
            2,
          )}
        </pre>
      </TechnicalDetails>
    </Modal>
  );
}
