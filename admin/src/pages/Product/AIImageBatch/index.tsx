import TechnicalDetails from '@/components/ui/TechnicalDetails';
import { TmPageContainer } from '@/components/ui';
import {
  AI_IMAGE_BATCH_MAX_IMAGES,
  AI_IMAGE_BATCH_MAX_PRODUCTS,
  AI_IMAGE_IMAGE_FILTERS,
  AI_IMAGE_OPERATION_OPTIONS,
} from '@/constants/aiProductImage';
import {
  checkAiProductImageBatch,
  createAiProductImageBatch,
  type CheckBatchResponse,
  type ImageGenerationOptions,
} from '@/services/aiProductImage';
import { fetchProductDetail, type ProductImageRow } from '@/services/products';
import { history, useLocation } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Descriptions,
  Form,
  Image,
  Input,
  Radio,
  Space,
  Spin,
  Steps,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';

type ProductWithImages = {
  id: string;
  title: string;
  source?: string;
  images: ProductImageRow[];
};

function parseProductIds(search: string): string[] {
  try {
    const raw = new URLSearchParams(search).get('productIds')?.trim();
    if (!raw) return [];
    return raw.split(',').map((s) => s.trim()).filter(Boolean);
  } catch {
    return [];
  }
}

function checkStatusColor(status: string) {
  if (status === 'ready') return 'green';
  if (status === 'warning') return 'orange';
  if (status === 'blocked') return 'red';
  return 'default';
}

export default function AIImageBatchWizardPage() {
  const location = useLocation();
  const initialIds = useMemo(() => parseProductIds(location.search), [location.search]);
  const [step, setStep] = useState(0);
  const [products, setProducts] = useState<ProductWithImages[]>([]);
  const [loadingProducts, setLoadingProducts] = useState(true);
  const [selectedImageIds, setSelectedImageIds] = useState<string[]>([]);
  const [selectedOps, setSelectedOps] = useState<string[]>(['quality_check', 'white_background']);
  const [imageFilter, setImageFilter] = useState('all');
  const [checkResult, setCheckResult] = useState<CheckBatchResponse | null>(null);
  const [checking, setChecking] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form] = Form.useForm<ImageGenerationOptions>();

  const productIds = useMemo(() => products.map((p) => p.id), [products]);
  const allImages = useMemo(
    () => products.flatMap((p) => p.images.map((img) => ({ ...img, productTitle: p.title, productId: p.id }))),
    [products],
  );
  const filteredImages = useMemo(() => {
    if (imageFilter === 'all') return allImages;
    return allImages.filter((img) => img.imageType === imageFilter);
  }, [allImages, imageFilter]);
  const expectedItems = selectedImageIds.length * selectedOps.length;

  useEffect(() => {
    if (!initialIds.length) {
      setLoadingProducts(false);
      return;
    }
    if (initialIds.length > AI_IMAGE_BATCH_MAX_PRODUCTS) {
      message.error(`最多选择 ${AI_IMAGE_BATCH_MAX_PRODUCTS} 个商品`);
      setLoadingProducts(false);
      return;
    }
    (async () => {
      setLoadingProducts(true);
      const rows: ProductWithImages[] = [];
      for (const id of initialIds) {
        try {
          const d = await fetchProductDetail(id);
          rows.push({
            id: d.id,
            title: d.title,
            source: d.source,
            images: d.images ?? [],
          });
        } catch {
          /* skip */
        }
      }
      setProducts(rows);
      const defaultIds = rows.flatMap((p) => p.images.map((i) => i.id));
      setSelectedImageIds(defaultIds);
      setLoadingProducts(false);
    })();
  }, [initialIds]);

  const runCheck = useCallback(async () => {
    if (!productIds.length || !selectedImageIds.length) {
      message.warning('请选择商品和图片');
      return;
    }
    setChecking(true);
    try {
      const res = await checkAiProductImageBatch({
        productIds,
        imageIds: selectedImageIds,
        operationTypes: selectedOps,
        options: form.getFieldsValue(),
      });
      setCheckResult(res);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '检查失败');
    } finally {
      setChecking(false);
    }
  }, [productIds, selectedImageIds, selectedOps, form]);

  const onCreate = async () => {
    if (!selectedImageIds.length || !selectedOps.length) {
      message.warning('请选择图片和处理方式');
      return;
    }
    if (expectedItems > AI_IMAGE_BATCH_MAX_IMAGES) {
      message.error(`本次选择的图片较多，请分批处理（最多 ${AI_IMAGE_BATCH_MAX_IMAGES} 个子项）`);
      return;
    }
    setCreating(true);
    try {
      const batch = await createAiProductImageBatch({
        productIds,
        imageIds: selectedImageIds,
        operationTypes: selectedOps,
        options: form.getFieldsValue(),
      });
      message.success('批量任务已创建');
      history.push(`/product/ai-image-batches/${batch.id}`);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '创建失败');
    } finally {
      setCreating(false);
    }
  };

  const toggleImage = (id: string) => {
    setSelectedImageIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  };

  return (
    <TmPageContainer title="批量 AI 图片处理" subTitle="选择图片 → 处理方式 → 确认后开始（结果需人工复核后再应用）">
      <Steps
        current={step}
        style={{ marginBottom: 24, maxWidth: 900 }}
        items={[
          { title: '选择商品' },
          { title: '选择图片' },
          { title: '处理方式' },
          { title: '处理要求' },
          { title: '确认并开始' },
        ]}
      />

      {loadingProducts ? (
        <Spin />
      ) : !products.length ? (
        <Alert type="warning" message="未找到有效商品，请从商品列表重新选择。" />
      ) : (
        <>
          {step === 0 && (
            <Card title="已选商品">
              <Table
                size="small"
                rowKey="id"
                pagination={false}
                dataSource={products}
                columns={[
                  { title: '商品标题', dataIndex: 'title', ellipsis: true },
                  { title: '来源', dataIndex: 'source', width: 100 },
                  { title: '主图', render: (_, r) => r.images.filter((i) => i.imageType === 'main').length },
                  { title: '详情图', render: (_, r) => r.images.filter((i) => i.imageType === 'detail').length },
                  {
                    title: '可处理',
                    render: (_, r) =>
                      r.images.length ? <Tag color="green">是</Tag> : <Tag color="orange">暂无可处理图片</Tag>,
                  },
                ]}
              />
            </Card>
          )}

          {step === 1 && (
            <Card
              title="选择图片"
              extra={
                <Space wrap>
                  <Radio.Group
                    optionType="button"
                    value={imageFilter}
                    options={AI_IMAGE_IMAGE_FILTERS}
                    onChange={(e) => setImageFilter(e.target.value)}
                  />
                  <Button size="small" onClick={() => setSelectedImageIds(filteredImages.map((i) => i.id))}>
                    全选当前筛选
                  </Button>
                  <Button size="small" onClick={() => setSelectedImageIds([])}>
                    取消选择
                  </Button>
                </Space>
              }
            >
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12 }}>
                {filteredImages.map((img) => {
                  const selected = selectedImageIds.includes(img.id);
                  return (
                    <Card
                      key={img.id}
                      size="small"
                      hoverable
                      style={{ width: 160, borderColor: selected ? '#1677ff' : undefined }}
                      onClick={() => toggleImage(img.id)}
                    >
                      <Image src={img.publicUrl || img.originUrl} height={100} style={{ objectFit: 'cover' }} fallback="/placeholder.png" />
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        {img.imageType === 'main' ? '主图' : img.imageType === 'detail' ? '详情图' : '图片'}
                      </Typography.Text>
                      <div>
                        <Checkbox checked={selected} />
                      </div>
                    </Card>
                  );
                })}
              </div>
              {!filteredImages.length && <Alert type="info" message="当前筛选下没有图片" />}
            </Card>
          )}

          {step === 2 && (
            <Card title="选择处理方式">
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message="部分处理方式可能耗时较长，结果生成后不会自动替换原图，需要人工复核后再应用。"
              />
              <Checkbox.Group
                options={AI_IMAGE_OPERATION_OPTIONS}
                value={selectedOps}
                onChange={(v) => setSelectedOps(v as string[])}
              />
            </Card>
          )}

          {step === 3 && (
            <Card title="设置处理要求">
              <Form form={form} layout="vertical" initialValues={{ language: 'en', backgroundStyle: 'white', keepSubject: true, outputFormat: 'webp' }}>
                <Form.Item name="language" label="目标语言">
                  <Input placeholder="如 en / th / vi" />
                </Form.Item>
                <Form.Item name="backgroundStyle" label="背景风格">
                  <Input placeholder="如 white / studio" />
                </Form.Item>
                <Form.Item name="keepSubject" valuePropName="checked" label="保留商品主体">
                  <Checkbox />
                </Form.Item>
                <Form.Item name="keepBrandLogo" valuePropName="checked" label="保留品牌标识">
                  <Checkbox />
                </Form.Item>
                <Form.Item name="outputFormat" label="输出格式">
                  <Radio.Group options={[{ value: 'webp', label: 'WebP' }, { value: 'png', label: 'PNG' }, { value: 'jpg', label: 'JPG' }]} />
                </Form.Item>
                <Form.Item name="remark" label="备注">
                  <Input.TextArea rows={2} />
                </Form.Item>
              </Form>
            </Card>
          )}

          {step === 4 && (
            <Card title="确认并开始">
              <Descriptions column={1} size="small" bordered>
                <Descriptions.Item label="商品数">{productIds.length}</Descriptions.Item>
                <Descriptions.Item label="已选图片">{selectedImageIds.length}</Descriptions.Item>
                <Descriptions.Item label="预计子项">{expectedItems}</Descriptions.Item>
                <Descriptions.Item label="处理方式">
                  {selectedOps.map((op) => AI_IMAGE_OPERATION_OPTIONS.find((x) => x.value === op)?.label || op).join('、')}
                </Descriptions.Item>
              </Descriptions>
              <Alert
                style={{ marginTop: 12 }}
                type="warning"
                showIcon
                message="AI 图片处理结果不会自动覆盖商品图片，需要人工复核后再应用。"
              />
              {checkResult && (
                <Card size="small" style={{ marginTop: 12 }} title="检查结果">
                  <Space wrap>
                    <Tag color="green">可处理 {checkResult.summary.readyCount}</Tag>
                    <Tag color="orange">提醒 {checkResult.summary.warningCount}</Tag>
                    <Tag color="red">阻断 {checkResult.summary.blockedCount}</Tag>
                  </Space>
                  <div style={{ marginTop: 8 }}>
                    <TechnicalDetails label="检查明细">
                      <pre style={{ margin: 0, fontSize: 12 }}>{JSON.stringify(checkResult.summary, null, 2)}</pre>
                    </TechnicalDetails>
                  </div>
                </Card>
              )}
            </Card>
          )}

          <Space style={{ marginTop: 16 }}>
            <Button disabled={step === 0} onClick={() => setStep((s) => s - 1)}>
              上一步
            </Button>
            {step < 4 ? (
              <Button type="primary" onClick={() => setStep((s) => s + 1)}>
                下一步
              </Button>
            ) : (
              <>
                <Button loading={checking} onClick={() => void runCheck()}>
                  预检查
                </Button>
                <Button type="primary" loading={creating} onClick={() => void onCreate()}>
                  开始处理
                </Button>
              </>
            )}
          </Space>
        </>
      )}
    </TmPageContainer>
  );
}
