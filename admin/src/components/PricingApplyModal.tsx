import { Alert, Button, Form, InputNumber, Modal, Select, Space, Table, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import {
  applyProductPricing,
  batchApplyProductPricing,
  type PricingApplySummary,
  type PricingPreviewLine,
  type PricingRule,
} from '@/services/pricing';

type Props = {
  open: boolean;
  onClose: () => void;
  onApplied?: () => void;
  mode: 'product' | 'batch';
  productId?: string;
  productIds?: string[];
  listFilters?: { status?: string; source?: string; keyword?: string };
};

const defaultRule = (): PricingRule => ({
  markupType: 'percent',
  markupPercent: 30,
  roundingMode: '.99',
});

export default function PricingApplyModal({
  open,
  onClose,
  onApplied,
  mode,
  productId,
  productIds,
  listFilters,
}: Props) {
  const [form] = Form.useForm();
  const [step, setStep] = useState<'form' | 'preview' | 'done'>('form');
  const [loading, setLoading] = useState(false);
  const [preview, setPreview] = useState<PricingPreviewLine[]>([]);
  const [summary, setSummary] = useState<PricingApplySummary | null>(null);
  const [confirmApply, setConfirmApply] = useState(false);

  const reset = useCallback(() => {
    setStep('form');
    setPreview([]);
    setSummary(null);
    setConfirmApply(false);
    form.setFieldsValue({
      platform: 'tiktok',
      markupType: 'percent',
      markupPercent: 30,
      markupAmount: 0,
      roundingMode: '.99',
      minPublishPrice: undefined,
    });
  }, [form]);

  useEffect(() => {
    if (open) reset();
  }, [open, reset]);

  const buildRule = (vals: Record<string, unknown>): PricingRule => ({
    markupType: vals.markupType as PricingRule['markupType'],
    markupPercent: vals.markupPercent as number | undefined,
    markupAmount: vals.markupAmount as number | undefined,
    minPublishPrice: vals.minPublishPrice as number | undefined,
    roundingMode: vals.roundingMode as PricingRule['roundingMode'],
  });

  const runPreview = async () => {
    const vals = await form.validateFields();
    const rule = buildRule(vals);
    setLoading(true);
    try {
      let res: PricingApplySummary;
      if (mode === 'product' && productId) {
        res = await applyProductPricing(productId, {
          platform: vals.platform,
          rule,
          confirm: false,
        });
      } else {
        const hasScope =
          (productIds?.length ?? 0) > 0 ||
          Boolean(listFilters?.keyword || listFilters?.status || listFilters?.source);
        res = await batchApplyProductPricing({
          productIds: productIds?.length ? productIds : undefined,
          filters: listFilters,
          platform: vals.platform,
          rule,
          confirm: false,
          confirmAll: !hasScope && (productIds?.length ?? 0) === 0,
        });
      }
      setPreview(res.preview ?? []);
      setStep('preview');
      if ((res.skuCount ?? 0) === 0) {
        message.warning('没有可计算的 SKU');
      }
    } catch (e: unknown) {
      message.error((e as Error)?.message || '试算失败');
    } finally {
      setLoading(false);
    }
  };

  const runApply = async () => {
    if (!confirmApply) {
      message.warning('请勾选确认后再应用');
      return;
    }
    const vals = form.getFieldsValue();
    const rule = buildRule(vals);
    setLoading(true);
    try {
      let res: PricingApplySummary;
      if (mode === 'product' && productId) {
        res = await applyProductPricing(productId, {
          platform: vals.platform,
          rule,
          confirm: true,
        });
      } else {
        const hasScope =
          (productIds?.length ?? 0) > 0 ||
          Boolean(listFilters?.keyword || listFilters?.status || listFilters?.source);
        res = await batchApplyProductPricing({
          productIds: productIds?.length ? productIds : undefined,
          filters: listFilters,
          platform: vals.platform,
          rule,
          confirm: true,
          confirmAll: !hasScope && (productIds?.length ?? 0) === 0,
        });
      }
      setSummary(res);
      setStep('done');
      message.success(`已更新 ${res.updated ?? 0} 个 SKU 销售价`);
      onApplied?.();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '应用失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title={mode === 'product' ? '应用定价规则' : '批量设置发布价'}
      open={open}
      onCancel={onClose}
      width={880}
      destroyOnHidden
      footer={
        step === 'form' ? (
          <Space>
            <Button onClick={onClose}>取消</Button>
            <Button type="primary" loading={loading} onClick={() => void runPreview()}>
              预览试算
            </Button>
          </Space>
        ) : step === 'preview' ? (
          <Space>
            <Button onClick={() => setStep('form')}>返回修改</Button>
            <Button type="primary" loading={loading} disabled={!confirmApply} onClick={() => void runApply()}>
              确认应用
            </Button>
          </Space>
        ) : (
          <Button type="primary" onClick={onClose}>
            关闭
          </Button>
        )
      }
    >
      {step === 'form' && (
        <>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="仅更新本地 SKU 销售价，不会自动刊登到平台"
          />
          <Form form={form} layout="vertical" initialValues={defaultRule()}>
            <Form.Item name="platform" label="目标平台（用于读取平台默认加价）" rules={[{ required: true }]}>
              <Select
                options={[
                  { label: 'TikTok', value: 'tiktok' },
                  { label: 'Shopee', value: 'shopee' },
                  { label: 'Lazada', value: 'lazada' },
                  { label: 'Amazon', value: 'amazon' },
                ]}
              />
            </Form.Item>
            <Form.Item name="markupType" label="加价方式" rules={[{ required: true }]}>
              <Select
                options={[
                  { label: '百分比加价', value: 'percent' },
                  { label: '固定金额加价', value: 'fixed' },
                  { label: '不加价', value: 'none' },
                ]}
              />
            </Form.Item>
            <Form.Item name="markupPercent" label="加价比例（%）">
              <InputNumber min={0} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="markupAmount" label="固定加价金额">
              <InputNumber min={0} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="minPublishPrice" label="最低发布价（可选，覆盖 SKU 级保护）">
              <InputNumber min={0} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="roundingMode" label="尾数规则">
              <Select
                options={[
                  { label: '不处理', value: 'none' },
                  { label: '取整', value: 'integer' },
                  { label: '.9', value: '.9' },
                  { label: '.99', value: '.99' },
                  { label: '.95', value: '.95' },
                ]}
              />
            </Form.Item>
          </Form>
        </>
      )}
      {step === 'preview' && (
        <>
          <Typography.Paragraph>
            将影响 <Typography.Text strong>{preview.length}</Typography.Text> 个 SKU。确认后将写入本地{' '}
            <Typography.Text code>price</Typography.Text> 字段。
          </Typography.Paragraph>
          <Table<PricingPreviewLine>
            size="small"
            rowKey="productSkuId"
            pagination={{ pageSize: 10 }}
            dataSource={preview}
            scroll={{ x: 720 }}
            columns={[
              { title: 'SKU', dataIndex: 'skuName', ellipsis: true },
              { title: '成本价', dataIndex: 'costPrice', width: 90, render: (v) => (v != null ? Number(v).toFixed(2) : '—') },
              { title: '当前价', dataIndex: 'currentPrice', width: 90, render: (v) => (v != null ? Number(v).toFixed(2) : '—') },
              { title: '计算后', dataIndex: 'calculatedPrice', width: 90, render: (v) => Number(v).toFixed(2) },
              {
                title: '差额',
                dataIndex: 'delta',
                width: 80,
                render: (v) => {
                  const n = Number(v);
                  const color = n > 0 ? '#389e0d' : n < 0 ? '#cf1322' : undefined;
                  return <span style={{ color }}>{n >= 0 ? `+${n.toFixed(2)}` : n.toFixed(2)}</span>;
                },
              },
            ]}
          />
          <Alert
            type="warning"
            showIcon
            style={{ marginTop: 12 }}
            message={
              <Checkbox checked={confirmApply} onChange={(e) => setConfirmApply(e.target.checked)}>
                我确认将上述计算结果写入本地 SKU 销售价（不自动刊登）
              </Checkbox>
            }
          />
        </>
      )}
      {step === 'done' && summary && (
        <Typography.Paragraph>
          已更新 <Typography.Text strong>{summary.updated ?? 0}</Typography.Text> / {summary.skuCount ?? 0} 个 SKU。
        </Typography.Paragraph>
      )}
    </Modal>
  );
}
