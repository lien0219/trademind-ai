import {
  DECIMAL_HANDLING_OPTIONS,
  DETAIL_IMAGE_STRATEGY_OPTIONS,
  INVENTORY_STRATEGY_OPTIONS,
  MAIN_IMAGE_STRATEGY_OPTIONS,
  OUT_OF_STOCK_OPTIONS,
  PRICE_STRATEGY_OPTIONS,
  type PublishConfigLayer,
  validatePublishConfigClient,
  WEIGHT_UNIT_OPTIONS,
} from '@/constants/publishConfig';
import { flattenEffectiveForDisplay, mergeEffectiveConfig } from '@/utils/publishConfigMerge';
import type { PublishConfigOverrides } from '@/services/productPublish';
import TechnicalDetails from '@/components/ui/TechnicalDetails';
import { Alert, Checkbox, Col, Divider, Form, Input, InputNumber, Row, Select, Typography } from 'antd';
import { useEffect, useMemo, useState } from 'react';

export type PublishConfigEditorProps = {
  value: PublishConfigLayer;
  onChange: (next: PublishConfigLayer) => void;
  /** 仅展示这些区块；默认全部 */
  sections?: Array<'price' | 'image' | 'inventory' | 'package' | 'remark'>;
  showPreview?: boolean;
  previewContext?: {
    common?: PublishConfigLayer;
    overrides?: PublishConfigOverrides;
    productId?: string;
    platform?: string;
    shopId?: string;
  };
};

const ALL_SECTIONS: PublishConfigEditorProps['sections'] = ['price', 'image', 'inventory', 'package', 'remark'];

export default function PublishConfigEditor({
  value,
  onChange,
  sections = ALL_SECTIONS,
  showPreview = false,
  previewContext,
}: PublishConfigEditorProps) {
  const [error, setError] = useState<string | null>(null);
  const show = (s: string) => sections?.includes(s as 'price') ?? true;

  useEffect(() => {
    setError(validatePublishConfigClient(value));
  }, [value]);

  const patch = (partial: PublishConfigLayer) => onChange({ ...value, ...partial });

  const previewRows = useMemo(() => {
    if (!showPreview || !previewContext?.productId || !previewContext.platform) return [];
    const eff = mergeEffectiveConfig(
      previewContext.common ?? {},
      previewContext.overrides ?? {},
      previewContext.productId,
      previewContext.platform,
      previewContext.shopId,
    );
    return flattenEffectiveForDisplay(eff);
  }, [showPreview, previewContext, value]);

  return (
    <Form layout="vertical" style={{ maxWidth: '100%' }}>
      {error ? <Alert type="error" showIcon message={error} style={{ marginBottom: 12 }} /> : null}

      {show('price') && (
        <>
          <Typography.Text strong>价格配置</Typography.Text>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 8, fontSize: 13 }}>
            用于批量生成平台草稿时的售价。不会直接修改商品原始成本。
          </Typography.Paragraph>
          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item label="价格策略">
                <Select
                  allowClear
                  placeholder="选择价格策略"
                  value={value.price?.strategy}
                  options={PRICE_STRATEGY_OPTIONS}
                  onChange={(v) => patch({ price: { ...value.price, strategy: v } })}
                />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item label="加价数值">
                <InputNumber
                  style={{ width: '100%' }}
                  min={0}
                  value={value.price?.markupValue}
                  onChange={(v) => patch({ price: { ...value.price, markupValue: v ?? undefined } })}
                />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item label="最低利润率保护 (%)">
                <InputNumber
                  style={{ width: '100%' }}
                  min={0}
                  value={value.price?.minProfitMargin}
                  onChange={(v) => patch({ price: { ...value.price, minProfitMargin: v ?? undefined } })}
                />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item label="小数处理方式">
                <Select
                  allowClear
                  value={value.price?.decimalHandling}
                  options={DECIMAL_HANDLING_OPTIONS}
                  onChange={(v) => patch({ price: { ...value.price, decimalHandling: v } })}
                />
              </Form.Item>
            </Col>
          </Row>
          <Divider style={{ margin: '12px 0' }} />
        </>
      )}

      {show('image') && (
        <>
          <Typography.Text strong>图片配置</Typography.Text>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 8, fontSize: 13 }}>
            图片策略只影响本次刊登草稿，不会删除商品图库中的图片。
          </Typography.Paragraph>
          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item label="主图策略">
                <Select
                  allowClear
                  value={value.image?.mainImageStrategy}
                  options={MAIN_IMAGE_STRATEGY_OPTIONS}
                  onChange={(v) => patch({ image: { ...value.image, mainImageStrategy: v } })}
                />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item label="详情图策略">
                <Select
                  allowClear
                  value={value.image?.detailImageStrategy}
                  options={DETAIL_IMAGE_STRATEGY_OPTIONS}
                  onChange={(v) => patch({ image: { ...value.image, detailImageStrategy: v } })}
                />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item>
                <Checkbox
                  checked={!!value.image?.preferProcessedImages}
                  onChange={(e) => patch({ image: { ...value.image, preferProcessedImages: e.target.checked } })}
                >
                  优先使用已处理图片
                </Checkbox>
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item>
                <Checkbox
                  checked={!!value.image?.skipFailedImages}
                  onChange={(e) => patch({ image: { ...value.image, skipFailedImages: e.target.checked } })}
                >
                  跳过失败图片
                </Checkbox>
              </Form.Item>
            </Col>
          </Row>
          <Divider style={{ margin: '12px 0' }} />
        </>
      )}

      {show('inventory') && (
        <>
          <Typography.Text strong>库存配置</Typography.Text>
          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item label="库存策略">
                <Select
                  allowClear
                  value={value.inventory?.strategy}
                  options={INVENTORY_STRATEGY_OPTIONS}
                  onChange={(v) => patch({ inventory: { ...value.inventory, strategy: v } })}
                />
              </Form.Item>
            </Col>
            {value.inventory?.strategy === 'fixed_quantity' && (
              <Col xs={24} md={12}>
                <Form.Item label="统一库存">
                  <InputNumber
                    style={{ width: '100%' }}
                    min={0}
                    precision={0}
                    value={value.inventory?.fixedQuantity}
                    onChange={(v) => patch({ inventory: { ...value.inventory, fixedQuantity: v ?? undefined } })}
                  />
                </Form.Item>
              </Col>
            )}
            <Col xs={24} md={12}>
              <Form.Item label="安全库存">
                <InputNumber
                  style={{ width: '100%' }}
                  min={0}
                  precision={0}
                  value={value.inventory?.safetyStock}
                  onChange={(v) => patch({ inventory: { ...value.inventory, safetyStock: v ?? undefined } })}
                />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item label="缺库存时处理方式">
                <Select
                  allowClear
                  value={value.inventory?.outOfStockAction}
                  options={OUT_OF_STOCK_OPTIONS}
                  onChange={(v) => patch({ inventory: { ...value.inventory, outOfStockAction: v } })}
                />
              </Form.Item>
            </Col>
          </Row>
          <Divider style={{ margin: '12px 0' }} />
        </>
      )}

      {show('package') && (
        <>
          <Typography.Text strong>包裹配置</Typography.Text>
          <Row gutter={16}>
            <Col xs={12} md={6}>
              <Form.Item label="重量">
                <InputNumber
                  style={{ width: '100%' }}
                  min={0}
                  value={value.package?.weight}
                  onChange={(v) => patch({ package: { ...value.package, weight: v ?? undefined } })}
                />
              </Form.Item>
            </Col>
            <Col xs={12} md={6}>
              <Form.Item label="单位">
                <Select
                  allowClear
                  value={value.package?.weightUnit ?? 'kg'}
                  options={WEIGHT_UNIT_OPTIONS}
                  onChange={(v) => patch({ package: { ...value.package, weightUnit: v } })}
                />
              </Form.Item>
            </Col>
            <Col xs={8} md={4}>
              <Form.Item label="长 (cm)">
                <InputNumber
                  style={{ width: '100%' }}
                  min={0}
                  value={value.package?.length}
                  onChange={(v) => patch({ package: { ...value.package, length: v ?? undefined, sizeUnit: 'cm' } })}
                />
              </Form.Item>
            </Col>
            <Col xs={8} md={4}>
              <Form.Item label="宽 (cm)">
                <InputNumber
                  style={{ width: '100%' }}
                  min={0}
                  value={value.package?.width}
                  onChange={(v) => patch({ package: { ...value.package, width: v ?? undefined, sizeUnit: 'cm' } })}
                />
              </Form.Item>
            </Col>
            <Col xs={8} md={4}>
              <Form.Item label="高 (cm)">
                <InputNumber
                  style={{ width: '100%' }}
                  min={0}
                  value={value.package?.height}
                  onChange={(v) => patch({ package: { ...value.package, height: v ?? undefined, sizeUnit: 'cm' } })}
                />
              </Form.Item>
            </Col>
          </Row>
          <Divider style={{ margin: '12px 0' }} />
        </>
      )}

      {show('remark') && (
        <Form.Item label="备注">
          <Input.TextArea
            rows={2}
            placeholder="本次批量刊登备注（仅内部记录，不会发送到平台）"
            value={value.remark}
            onChange={(e) => patch({ remark: e.target.value })}
          />
        </Form.Item>
      )}

      {showPreview && previewRows.length > 0 && (
        <TechnicalDetails label="保存前预览（示例生效项）">
          <ul style={{ margin: 0, paddingLeft: 18, fontSize: 13 }}>
            {previewRows.slice(0, 8).map((r) => (
              <li key={r.field}>
                {r.label}：{r.value}
                {r.source ? `（来源：${r.source}）` : ''}
              </li>
            ))}
          </ul>
        </TechnicalDetails>
      )}
    </Form>
  );
}
