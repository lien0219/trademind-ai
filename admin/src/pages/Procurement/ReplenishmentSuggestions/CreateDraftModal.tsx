import { Alert, Form, Input, InputNumber, Modal, Select, Typography } from 'antd';
import { useEffect, useMemo } from 'react';
import type {
  CreateReplenishmentPurchaseOrderBody,
  ReplenishmentSuggestion,
  ReplenishmentSupplierOption,
} from '@/services/procurement';
import { formatMinorAmount } from '../helpers';

type DraftLineValues = {
  productSkuId: string;
  supplierSkuId?: string;
  quantity: number;
  suggestionHash: string;
};

type DraftFormValues = {
  supplierId?: string;
  remark: string;
  items: DraftLineValues[];
};

type CreateDraftModalProps = {
  open: boolean;
  warehouseId: string;
  warehouseLabel: string;
  rows: ReplenishmentSuggestion[];
  idempotencyKey: string;
  submitting: boolean;
  onCancel: () => void;
  onSubmit: (body: CreateReplenishmentPurchaseOrderBody) => void;
};

function optionsFor(row: ReplenishmentSuggestion, supplierId?: string) {
  return (row.supplierOptions ?? []).filter((option) => option.supplierId === supplierId);
}

function roundedQuantity(deficit: number, minOrderQty: number, current = 0) {
  const moq = Math.max(1, minOrderQty);
  const minimum = Math.ceil(Math.max(1, deficit) / moq) * moq;
  const candidate = Math.max(minimum, current);
  return Math.ceil(candidate / moq) * moq;
}

function commonSuppliers(rows: ReplenishmentSuggestion[]) {
  if (!rows.length) return [];
  const common = new Map<string, string>();
  for (const option of rows[0].supplierOptions ?? []) {
    common.set(option.supplierId, option.supplierName);
  }
  for (const row of rows.slice(1)) {
    const ids = new Set((row.supplierOptions ?? []).map((option) => option.supplierId));
    for (const supplierId of common.keys()) {
      if (!ids.has(supplierId)) common.delete(supplierId);
    }
  }
  return [...common].map(([value, label]) => ({ value, label }));
}

export default function CreateDraftModal({
  open,
  warehouseId,
  warehouseLabel,
  rows,
  idempotencyKey,
  submitting,
  onCancel,
  onSubmit,
}: CreateDraftModalProps) {
  const [form] = Form.useForm<DraftFormValues>();
  const supplierId = Form.useWatch('supplierId', form);
  const supplierChoices = useMemo(() => commonSuppliers(rows), [rows]);

  useEffect(() => {
    if (!open) return;
    form.setFieldsValue({
      supplierId: undefined,
      remark: '补货建议人工确认',
      items: rows.map((row) => ({
        productSkuId: row.productSkuId,
        supplierSkuId: undefined,
        quantity: Math.max(1, row.suggestedQuantity || row.deficit),
        suggestionHash: row.suggestionHash,
      })),
    });
  }, [form, open, rows]);

  const changeSupplier = (nextSupplierId: string) => {
    const current = form.getFieldValue('items') ?? [];
    form.setFieldValue('items', rows.map((row, index) => {
      const options = optionsFor(row, nextSupplierId);
      const selected = options.length === 1 ? options[0] : undefined;
      return {
        productSkuId: row.productSkuId,
        supplierSkuId: selected?.supplierSkuId,
        quantity: selected
          ? roundedQuantity(row.deficit, selected.minOrderQty, current[index]?.quantity)
          : Math.max(1, row.deficit, current[index]?.quantity ?? 0),
        suggestionHash: row.suggestionHash,
      };
    }));
  };

  const changeSupplierSKU = (index: number, supplierSkuId: string) => {
    const option = optionsFor(rows[index], supplierId).find((item) => item.supplierSkuId === supplierSkuId);
    if (!option) return;
    const current = form.getFieldValue('items') ?? [];
    const items = [...current];
    items[index] = {
      ...items[index],
      supplierSkuId,
      quantity: roundedQuantity(rows[index].deficit, option.minOrderQty, items[index]?.quantity),
    };
    form.setFieldValue('items', items);
  };

  const selectedOption = (row: ReplenishmentSuggestion, index: number): ReplenishmentSupplierOption | undefined => {
    const supplierSkuId = form.getFieldValue(['items', index, 'supplierSkuId']);
    return optionsFor(row, supplierId).find((option) => option.supplierSkuId === supplierSkuId);
  };

  const submit = (values: DraftFormValues) => {
    if (!values.supplierId) return;
    onSubmit({
      idempotencyKey,
      warehouseId,
      supplierId: values.supplierId,
      remark: values.remark?.trim() || '',
      items: values.items.map((item) => ({
        productSkuId: item.productSkuId,
        supplierSkuId: item.supplierSkuId as string,
        quantity: item.quantity,
        suggestionHash: item.suggestionHash,
      })),
    });
  };

  return (
    <Modal
      title="从补货建议创建采购草稿"
      open={open}
      width={880}
      confirmLoading={submitting}
      okText="确认创建采购草稿"
      cancelText="取消"
      okButtonProps={{ disabled: supplierChoices.length === 0 }}
      style={{ top: 16 }}
      styles={{ body: { maxHeight: 'calc(100dvh - 180px)', overflowY: 'auto' } }}
      onCancel={() => !submitting && onCancel()}
      onOk={() => form.submit()}
      forceRender
    >
      <Form form={form} layout="vertical" preserve={false} onFinish={submit}>
        <Alert
          type="info"
          showIcon
          message="本次只创建一张本地采购草稿"
          description={`收货仓库固定为 ${warehouseLabel || warehouseId}。确认后仍需进入采购单详情提交审批，审批通过并人工收货后才会增加库存。`}
        />
        {supplierChoices.length === 0 ? (
          <Alert
            type="warning"
            showIcon
            message="所选规格没有共同供应商"
            description="请取消后按供应商分批选择，系统不会拆成多次写入。"
          />
        ) : null}
        <Form.Item label="采购供应商" name="supplierId" rules={[{ required: true, message: '请选择本次采购供应商' }]}>
          <Select
            aria-label="采购供应商"
            placeholder="请选择共同供应商"
            options={supplierChoices}
            onChange={changeSupplier}
          />
        </Form.Item>
        <div className="tm-replenishment-draft-lines">
          {rows.map((row, index) => {
            const supplierOptions = optionsFor(row, supplierId);
            const option = selectedOption(row, index);
            return (
              <div className="tm-replenishment-draft-line" key={row.productSkuId}>
                <div className="tm-replenishment-draft-line__identity">
                  <Typography.Text strong>{row.productTitle || '未命名商品'}</Typography.Text>
                  <Typography.Text type="secondary">{row.skuCode || row.skuName || row.productSkuId}</Typography.Text>
                  <Typography.Text type="secondary">缺口 {row.deficit}</Typography.Text>
                </div>
                <Form.Item name={['items', index, 'productSkuId']} hidden><Input /></Form.Item>
                <Form.Item name={['items', index, 'suggestionHash']} hidden><Input /></Form.Item>
                {supplierOptions.length > 1 ? (
                  <Form.Item
                    label={`供应商规格 ${index + 1}`}
                    name={['items', index, 'supplierSkuId']}
                    rules={[{ required: true, message: '请选择供应商规格' }]}
                  >
                    <Select
                      aria-label={`供应商规格 ${index + 1}`}
                      placeholder="请选择供应商规格"
                      options={supplierOptions.map((item) => ({
                        value: item.supplierSkuId,
                        label: `${formatMinorAmount(item.unitCostMinor, item.currency)} · 起订 ${Math.max(1, item.minOrderQty)} · ${item.leadTimeDays} 天`,
                      }))}
                      onChange={(value) => changeSupplierSKU(index, value)}
                    />
                  </Form.Item>
                ) : (
                  <>
                    <Form.Item name={['items', index, 'supplierSkuId']} hidden><Input /></Form.Item>
                    <div className="tm-replenishment-draft-line__binding">
                      <Typography.Text type="secondary">供应条件</Typography.Text>
                      <Typography.Text>{option ? `${formatMinorAmount(option.unitCostMinor, option.currency)} · 起订 ${Math.max(1, option.minOrderQty)} · ${option.leadTimeDays} 天` : '选择供应商后显示'}</Typography.Text>
                    </div>
                  </>
                )}
                <Form.Item
                  label={`采购数量 ${index + 1}`}
                  name={['items', index, 'quantity']}
                  dependencies={['supplierId', ['items', index, 'supplierSkuId']]}
                  rules={[
                    { required: true, type: 'number', min: 1, message: '请输入采购数量' },
                    {
                      validator: async (_, value) => {
                        const binding = selectedOption(row, index);
                        if (!binding) throw new Error('请先选择有效的供应商规格');
                        const minimum = roundedQuantity(row.deficit, binding.minOrderQty);
                        if (value < minimum || value % Math.max(1, binding.minOrderQty) !== 0) {
                          throw new Error(`数量至少为 ${minimum}，且必须是起订量 ${Math.max(1, binding.minOrderQty)} 的整数倍`);
                        }
                      },
                    },
                  ]}
                >
                  <InputNumber aria-label={`采购数量 ${index + 1}`} min={1} max={1000000} precision={0} />
                </Form.Item>
              </div>
            );
          })}
        </div>
        <Form.Item label="备注" name="remark"><Input.TextArea maxLength={1000} rows={3} showCount /></Form.Item>
      </Form>
    </Modal>
  );
}
