import TechnicalDetails from '@/components/ui/TechnicalDetails';
import { TmPageContainer } from '@/components/ui';
import {
  AI_TEXT_BATCH_MAX_PRODUCTS,
  AI_TEXT_OPERATION_OPTIONS,
} from '@/constants/aiProductText';
import { PUBLISH_BATCH_LIMIT_MESSAGE } from '@/constants/publishLimits';
import { PRODUCT_STATUS } from '@/constants/status';
import { productSourceLabel } from '@/constants/userFriendly';
import {
  checkAiProductTextBatch,
  createAiProductTextBatch,
  type CheckBatchItem,
  type CheckBatchResponse,
  type TextGenerationOptions,
} from '@/services/aiProductText';
import { fetchProductDetail, type ProductListRow } from '@/services/products';
import { history, useLocation } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Descriptions,
  Form,
  Input,
  InputNumber,
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

function parseProductIds(search: string): string[] {
  try {
    const raw = new URLSearchParams(search).get('productIds')?.trim();
    if (!raw) return [];
    return raw.split(',').map((s) => s.trim()).filter(Boolean);
  } catch {
    return [];
  }
}

function operationTypesFromChoice(choice: string): string[] {
  if (choice === 'both') return ['title', 'description'];
  return [choice];
}

function checkStatusColor(status: string) {
  if (status === 'ready') return 'green';
  if (status === 'warning') return 'orange';
  if (status === 'blocked') return 'red';
  return 'default';
}

export default function AITextBatchWizardPage() {
  const location = useLocation();
  const initialIds = useMemo(() => parseProductIds(location.search), [location.search]);
  const [step, setStep] = useState(0);
  const [products, setProducts] = useState<ProductListRow[]>([]);
  const [loadingProducts, setLoadingProducts] = useState(true);
  const [opChoice, setOpChoice] = useState('title');
  const [checkResult, setCheckResult] = useState<CheckBatchResponse | null>(null);
  const [checking, setChecking] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form] = Form.useForm<TextGenerationOptions & { operationChoice: string }>();

  const productIds = useMemo(() => products.map((p) => p.id), [products]);
  const operationTypes = useMemo(() => operationTypesFromChoice(opChoice), [opChoice]);
  const expectedItems = productIds.length * operationTypes.length;

  useEffect(() => {
    if (!initialIds.length) {
      setLoadingProducts(false);
      return;
    }
    if (initialIds.length > AI_TEXT_BATCH_MAX_PRODUCTS) {
      message.error(PUBLISH_BATCH_LIMIT_MESSAGE);
      setLoadingProducts(false);
      return;
    }
    (async () => {
      setLoadingProducts(true);
      const rows: ProductListRow[] = [];
      for (const id of initialIds) {
        try {
          const d = await fetchProductDetail(id);
          rows.push(d as unknown as ProductListRow);
        } catch {
          /* skip missing */
        }
      }
      setProducts(rows);
      setLoadingProducts(false);
    })();
  }, [initialIds]);

  const runCheck = useCallback(async () => {
    if (!productIds.length) {
      message.warning('没有可检查的商品');
      return;
    }
    setChecking(true);
    try {
      const opts = form.getFieldsValue();
      const res = await checkAiProductTextBatch({
        productIds,
        operationTypes,
        options: opts,
      });
      setCheckResult(res);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '检查失败');
    } finally {
      setChecking(false);
    }
  }, [form, operationTypes, productIds]);

  useEffect(() => {
    if (step === 3 && !checkResult && productIds.length) {
      void runCheck();
    }
  }, [step, checkResult, productIds.length, runCheck]);

  const onCreate = async () => {
    if (!productIds.length) return;
    setCreating(true);
    try {
      const vals = form.getFieldsValue();
      const splitList = (s?: string) =>
        (s || '')
          .split(/[,，]/)
          .map((x) => x.trim())
          .filter(Boolean);
      const opts: TextGenerationOptions = {
        ...vals,
        keywords: splitList(vals.keywords as unknown as string),
        forbiddenWords: splitList(vals.forbiddenWords as unknown as string),
      };
      const batch = await createAiProductTextBatch({
        productIds,
        operationTypes,
        options: opts,
      });
      message.success('批量 AI 任务已创建，请前往复核工作台');
      history.push(`/product/ai-text-batches/${batch.id}`);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '创建失败');
    } finally {
      setCreating(false);
    }
  };

  const stepItems = [
    { title: '选择商品' },
    { title: '选择优化内容' },
    { title: '设置生成要求' },
    { title: '确认并开始' },
  ];

  if (!initialIds.length) {
    return (
      <TmPageContainer title="批量 AI 优化" subTitle="请从商品草稿列表勾选商品后发起">
        <Alert type="info" showIcon message="请返回商品草稿列表，勾选商品后点击「批量 AI 优化」。" />
        <Button type="link" onClick={() => history.push('/product/drafts')}>
          返回商品草稿
        </Button>
      </TmPageContainer>
    );
  }

  return (
    <TmPageContainer title="批量 AI 优化" subTitle="生成后需人工复核，不会自动覆盖商品内容">
      <Steps current={step} items={stepItems} style={{ marginBottom: 24 }} />

      {step === 0 && (
        <Card title={`已选商品（${products.length}）`}>
          {loadingProducts ? (
            <Spin />
          ) : (
            <Table
              rowKey="id"
              size="small"
              pagination={false}
              scroll={{ x: 960 }}
              dataSource={products}
              columns={[
                { title: '商品标题', dataIndex: 'title', ellipsis: true },
                {
                  title: '来源',
                  dataIndex: 'source',
                  width: 100,
                  render: (v) => productSourceLabel(String(v || '')),
                },
                {
                  title: '状态',
                  dataIndex: 'status',
                  width: 100,
                  render: (v) =>
                    PRODUCT_STATUS[v as keyof typeof PRODUCT_STATUS]?.text || String(v ?? ''),
                },
                {
                  title: 'AI 标题',
                  dataIndex: 'aiTitle',
                  width: 120,
                  ellipsis: true,
                  render: (v) => (v ? '已有' : '缺失'),
                },
                {
                  title: 'AI 描述',
                  dataIndex: 'aiDescription',
                  width: 120,
                  ellipsis: true,
                  render: (v) => (v ? '已有' : '缺失'),
                },
              ]}
            />
          )}
          <Space style={{ marginTop: 16 }}>
            <Button onClick={() => history.push('/product/drafts')}>返回列表</Button>
            <Button type="primary" disabled={!products.length} onClick={() => setStep(1)}>
              下一步
            </Button>
          </Space>
        </Card>
      )}

      {step === 1 && (
        <Card title="选择优化内容">
          <Radio.Group
            value={opChoice}
            onChange={(e) => setOpChoice(e.target.value)}
            style={{ display: 'flex', flexDirection: 'column', gap: 12 }}
          >
            {AI_TEXT_OPERATION_OPTIONS.map((opt) => (
              <Radio key={opt.value} value={opt.value}>
                {opt.label}
              </Radio>
            ))}
          </Radio.Group>
          <Space style={{ marginTop: 24 }}>
            <Button onClick={() => setStep(0)}>上一步</Button>
            <Button type="primary" onClick={() => setStep(2)}>
              下一步
            </Button>
          </Space>
        </Card>
      )}

      {step === 2 && (
        <Card title="设置生成要求">
          <Form
            form={form}
            layout="vertical"
            initialValues={{
              language: 'zh',
              platform: '跨境通用',
              tone: 'professional',
              maxLength: 120,
              highlightSelling: true,
              keepBrandWords: true,
              keepSpecWords: true,
              removeCollectNoise: true,
              generateBullets: true,
              keepOriginalParams: true,
              crossBorderReady: true,
            }}
          >
            <Typography.Title level={5}>标题配置</Typography.Title>
            <Form.Item name="titleStyle" label="标题风格">
              <Input placeholder="如：简洁专业、突出卖点" />
            </Form.Item>
            <Form.Item name="maxLength" label="标题长度上限">
              <InputNumber min={20} max={200} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="highlightSelling" valuePropName="checked">
              <Checkbox>是否突出卖点</Checkbox>
            </Form.Item>
            <Form.Item name="keepBrandWords" valuePropName="checked">
              <Checkbox>是否保留品牌词</Checkbox>
            </Form.Item>
            <Form.Item name="keepSpecWords" valuePropName="checked">
              <Checkbox>是否保留规格词</Checkbox>
            </Form.Item>
            <Form.Item name="removeCollectNoise" valuePropName="checked">
              <Checkbox>是否去除采集噪声</Checkbox>
            </Form.Item>

            <Typography.Title level={5}>描述配置</Typography.Title>
            <Form.Item name="descStyle" label="描述风格">
              <Input placeholder="如：场景化、专业说明" />
            </Form.Item>
            <Form.Item name="descStructure" label="描述结构">
              <Input placeholder="如：卖点列表 + 规格参数" />
            </Form.Item>
            <Form.Item name="highlightScenarios" valuePropName="checked">
              <Checkbox>是否突出使用场景</Checkbox>
            </Form.Item>
            <Form.Item name="generateBullets" valuePropName="checked">
              <Checkbox>是否生成卖点列表</Checkbox>
            </Form.Item>
            <Form.Item name="keepOriginalParams" valuePropName="checked">
              <Checkbox>是否保留原参数</Checkbox>
            </Form.Item>
            <Form.Item name="crossBorderReady" valuePropName="checked">
              <Checkbox>是否适合跨境平台</Checkbox>
            </Form.Item>

            <Typography.Title level={5}>通用配置</Typography.Title>
            <Form.Item name="language" label="目标语言">
              <Input />
            </Form.Item>
            <Form.Item name="platform" label="目标平台">
              <Input />
            </Form.Item>
            <Form.Item name="tone" label="语气风格">
              <Input />
            </Form.Item>
            <Form.Item name="keywords" label="关键词（逗号分隔）">
              <Input placeholder="可选" />
            </Form.Item>
            <Form.Item name="forbiddenWords" label="禁用词（逗号分隔）">
              <Input placeholder="可选" />
            </Form.Item>
            <Form.Item name="remark" label="备注">
              <Input.TextArea rows={2} placeholder="可选，不会写入 Prompt 日志" />
            </Form.Item>
          </Form>
          <Space>
            <Button onClick={() => setStep(1)}>上一步</Button>
            <Button type="primary" onClick={() => { setCheckResult(null); setStep(3); }}>
              下一步
            </Button>
          </Space>
        </Card>
      )}

      {step === 3 && (
        <Card title="确认并开始">
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="AI 结果生成后不会自动覆盖商品内容，需要人工复核后再应用。"
          />
          <Descriptions column={1} bordered size="small" style={{ marginBottom: 16 }}>
            {operationTypes.includes('title') && (
              <Descriptions.Item label="标题生成">
                将为 {productIds.length} 个商品生成 AI 标题
              </Descriptions.Item>
            )}
            {operationTypes.includes('description') && (
              <Descriptions.Item label="描述生成">
                将为 {productIds.length} 个商品生成 AI 描述
              </Descriptions.Item>
            )}
            <Descriptions.Item label="预计结果数">{expectedItems} 个</Descriptions.Item>
          </Descriptions>

          {checking ? (
            <Spin tip="正在检查…" />
          ) : checkResult ? (
            <>
              <Descriptions size="small" column={4} style={{ marginBottom: 12 }}>
                <Descriptions.Item label="可生成">{checkResult.summary.readyCount}</Descriptions.Item>
                <Descriptions.Item label="有提醒">{checkResult.summary.warningCount}</Descriptions.Item>
                <Descriptions.Item label="不可生成">{checkResult.summary.blockedCount}</Descriptions.Item>
              </Descriptions>
              <Table<CheckBatchItem>
                rowKey={(r) => `${r.productId}-${r.operationType}`}
                size="small"
                pagination={false}
                scroll={{ x: 900, y: 320 }}
                dataSource={checkResult.items}
                columns={[
                  { title: '商品', dataIndex: 'productTitle', ellipsis: true },
                  { title: '类型', dataIndex: 'operationLabel', width: 100 },
                  {
                    title: '状态',
                    dataIndex: 'statusLabel',
                    width: 120,
                    render: (_, row) => (
                      <Tag color={checkStatusColor(row.status)}>{row.statusLabel}</Tag>
                    ),
                  },
                  {
                    title: '说明',
                    dataIndex: 'issues',
                    render: (issues: string[]) =>
                      issues?.length ? issues.join('；') : '—',
                  },
                ]}
              />
              <TechnicalDetails label="技术详情">
                <pre style={{ fontSize: 12, margin: 0 }}>
                  {JSON.stringify(
                    {
                      productCount: checkResult.summary.productCount,
                      itemCount: checkResult.summary.itemCount,
                    },
                    null,
                    2,
                  )}
                </pre>
              </TechnicalDetails>
            </>
          ) : null}

          <Space style={{ marginTop: 16 }}>
            <Button onClick={() => setStep(2)}>上一步</Button>
            <Button onClick={() => void runCheck()} loading={checking}>
              重新检查
            </Button>
            <Button
              type="primary"
              loading={creating}
              disabled={!!checkResult && checkResult.summary.blockedCount === checkResult.summary.itemCount}
              onClick={() => void onCreate()}
            >
              开始生成
            </Button>
          </Space>
        </Card>
      )}
    </TmPageContainer>
  );
}
