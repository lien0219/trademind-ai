import TechnicalDetails from '@/components/ui/TechnicalDetails';
import { AI_IMAGE_APPLY_MODES } from '@/constants/aiProductImage';
import {
  applyAiProductImageItem,
  regenerateAiProductImageItem,
  rejectAiProductImageItem,
  type AIProductImageItemRow,
} from '@/services/aiProductImage';
import { history } from '@umijs/max';
import { Alert, Button, Image, Modal, Radio, Space, Tag, Typography, message } from 'antd';
import { useState } from 'react';

type Props = {
  open: boolean;
  item: AIProductImageItemRow | null;
  onClose: () => void;
  onDone: () => void;
};

export default function ReviewImageItemModal({ open, item, onClose, onDone }: Props) {
  const [applyMode, setApplyMode] = useState('save_to_gallery');
  const [acting, setActing] = useState(false);

  if (!item) return null;

  const canApply = item.status === 'pending_review' || item.status === 'success';
  const isReplace = applyMode === 'replace_image';

  const onApply = async () => {
    if (isReplace) {
      Modal.confirm({
        title: '确认替换原图？',
        content: '替换操作会修改当前商品图片展示，可在安全条件下撤销本批次应用。',
        onOk: doApply,
      });
      return;
    }
    await doApply();
  };

  const doApply = async () => {
    setActing(true);
    try {
      const res = await applyAiProductImageItem(item.id, applyMode);
      if (res.status === 'conflict') {
        message.error(res.errorMessage || '图片有冲突');
      } else {
        message.success('已应用');
        onDone();
        onClose();
      }
    } catch (e: unknown) {
      message.error((e as Error)?.message || '应用失败');
    } finally {
      setActing(false);
    }
  };

  return (
    <Modal
      open={open}
      title={`复核 · ${item.productTitle}`}
      width={920}
      onCancel={onClose}
      footer={
        <Space wrap>
          <Button onClick={() => history.push(`/product/drafts/${item.productId}`)}>查看商品详情</Button>
          <Button
            loading={acting}
            onClick={async () => {
              setActing(true);
              try {
                await regenerateAiProductImageItem(item.id);
                message.success('已重新处理');
                onDone();
              } catch (e: unknown) {
                message.error((e as Error)?.message || '重新处理失败');
              } finally {
                setActing(false);
              }
            }}
          >
            重新处理
          </Button>
          <Button
            onClick={async () => {
              await rejectAiProductImageItem(item.id);
              message.success('已放弃');
              onDone();
              onClose();
            }}
          >
            放弃
          </Button>
          <Button type="primary" loading={acting} disabled={!canApply} onClick={() => void onApply()}>
            应用
          </Button>
        </Space>
      }
    >
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 16 }}>
        <div style={{ flex: '1 1 280px' }}>
          <Typography.Text strong>原图</Typography.Text>
          <Image src={item.sourceImageUrl} style={{ maxHeight: 280, objectFit: 'contain' }} />
        </div>
        <div style={{ flex: '1 1 280px' }}>
          <Typography.Text strong>AI 处理结果</Typography.Text>
          {item.resultImageUrl ? (
            <Image src={item.resultImageUrl} style={{ maxHeight: 280, objectFit: 'contain' }} />
          ) : (
            <Alert type="info" message="质量检查类任务无新结果图，请查看质量提醒。" />
          )}
        </div>
      </div>

      <div style={{ marginTop: 12 }}>
        <Typography.Text strong>应用方式</Typography.Text>
        <Radio.Group
          style={{ display: 'block', marginTop: 8 }}
          value={applyMode}
          options={AI_IMAGE_APPLY_MODES}
          onChange={(e) => setApplyMode(e.target.value)}
        />
      </div>

      {item.qualityWarnings?.length > 0 && (
        <Alert
          style={{ marginTop: 12 }}
          type="warning"
          message="质量提醒"
          description={
            <ul style={{ margin: 0, paddingLeft: 18 }}>
              {item.qualityWarnings.map((w) => (
                <li key={w.code}>{w.message}</li>
              ))}
            </ul>
          }
        />
      )}

      <Space wrap style={{ marginTop: 12 }}>
        <Tag>{item.operationLabel}</Tag>
        <Tag>{item.imageTypeLabel}</Tag>
        <Tag>{item.statusLabel}</Tag>
      </Space>

      <div style={{ marginTop: 12 }}>
        <TechnicalDetails label="技术详情">
          <pre style={{ margin: 0, fontSize: 12 }}>
            {JSON.stringify(
              {
                itemId: item.id,
                imageTaskId: item.imageTaskId,
                operationType: item.operationType,
                applyMode,
              },
              null,
              2,
            )}
          </pre>
        </TechnicalDetails>
      </div>
    </Modal>
  );
}
