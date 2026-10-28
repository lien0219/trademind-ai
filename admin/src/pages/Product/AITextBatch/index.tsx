import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import TechnicalDetails from '@/components/ui/TechnicalDetails';
import { TmPageContainer } from '@/components/ui';
import {
  AI_TEXT_BATCH_MAX_PRODUCTS,
  AI_TEXT_OPERATION_OPTIONS,
} from '@/constants/aiProductText';
import {
  AI_LANGUAGE_OPTIONS,
  AI_TARGET_PLATFORM_OPTIONS,
  AI_TONE_OPTIONS,
} from '@/constants/aiPrompts';
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
  Select,
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
  useLanguage();
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
      message.warning(tr("没有可检查的商品"));
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
      message.error((e as Error)?.message || tr("检查失败"));
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
      message.success(tr("批量 AI 任务已创建，请前往复核工作台"));
      history.push(`/product/ai-text-batches/${batch.id}`);
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("创建失败"));
    } finally {
      setCreating(false);
    }
  };

  const stepItems = [
    { title: tr("选择商品") },
    { title: tr("选择优化内容") },
    { title: tr("设置生成要求") },
    { title: tr("确认并开始") },
  ];

  if (!initialIds.length) {
    return (
      <TmPageContainer title={tr("批量 AI 优化")} subTitle={tr("请从商品草稿列表勾选商品后发起")}>
        <Alert type="info" showIcon message={tr("请返回商品草稿列表，勾选商品后点击「批量 AI 优化」。")} />
        <Button type="link" onClick={() => history.push('/product/drafts')}>
          {tr("返回商品草稿")}
        </Button>
      </TmPageContainer>
    );
  }

  return (
    <TmPageContainer title={tr("批量 AI 优化")} subTitle={tr("生成后需人工复核，不会自动覆盖商品内容")}>
      <Steps current={step} items={stepItems} style={{ marginBottom: 24 }} />

      {step === 0 && (
        <Card title={trFormat("已选商品（{value0}）", { value0: String(products.length) })}>
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
                { title: tr("商品标题"), dataIndex: 'title', ellipsis: true },
                {
                  title: tr("来源"),
                  dataIndex: 'source',
                  width: 100,
                  render: (v) => productSourceLabel(String(v || '')),
                },
                {
                  title: tr("状态"),
                  dataIndex: 'status',
                  width: 100,
                  render: (v) =>
                    PRODUCT_STATUS[v as keyof typeof PRODUCT_STATUS]?.text || String(v ?? ''),
                },
                {
                  title: tr("AI 标题"),
                  dataIndex: 'aiTitle',
                  width: 120,
                  ellipsis: true,
                  render: (v) => (v ? tr("已有") : tr("缺失")),
                },
                {
                  title: tr("AI 描述"),
                  dataIndex: 'aiDescription',
                  width: 120,
                  ellipsis: true,
                  render: (v) => (v ? tr("已有") : tr("缺失")),
                },
              ]}
            />
          )}
          <Space style={{ marginTop: 16 }}>
            <Button onClick={() => history.push('/product/drafts')}>{tr("返回列表")}</Button>
            <Button type="primary" disabled={!products.length} onClick={() => setStep(1)}>
              {tr("下一步")}
            </Button>
          </Space>
        </Card>
      )}

      {step === 1 && (
        <Card title={tr("选择优化内容")}>
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
            <Button onClick={() => setStep(0)}>{tr("上一步")}</Button>
            <Button type="primary" onClick={() => setStep(2)}>
              {tr("下一步")}
            </Button>
          </Space>
        </Card>
      )}

      {step === 2 && (
        <Card title={tr("设置生成要求")}>
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
            <Typography.Title level={5}>{tr("标题配置")}</Typography.Title>
            <Form.Item name="titleStyle" label={tr("标题风格")}>
              <Input placeholder={tr("如：简洁专业、突出卖点")} />
            </Form.Item>
            <Form.Item name="maxLength" label={tr("标题长度上限")}>
              <InputNumber min={20} max={200} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="highlightSelling" valuePropName="checked">
              <Checkbox>{tr("是否突出卖点")}</Checkbox>
            </Form.Item>
            <Form.Item name="keepBrandWords" valuePropName="checked">
              <Checkbox>{tr("是否保留品牌词")}</Checkbox>
            </Form.Item>
            <Form.Item name="keepSpecWords" valuePropName="checked">
              <Checkbox>{tr("是否保留规格词")}</Checkbox>
            </Form.Item>
            <Form.Item name="removeCollectNoise" valuePropName="checked">
              <Checkbox>{tr("是否去除采集噪声")}</Checkbox>
            </Form.Item>

            <Typography.Title level={5}>{tr("描述配置")}</Typography.Title>
            <Form.Item name="descStyle" label={tr("描述风格")}>
              <Input placeholder={tr("如：场景化、专业说明")} />
            </Form.Item>
            <Form.Item name="descStructure" label={tr("描述结构")}>
              <Input placeholder={tr("如：卖点列表 + 规格参数")} />
            </Form.Item>
            <Form.Item name="highlightScenarios" valuePropName="checked">
              <Checkbox>{tr("是否突出使用场景")}</Checkbox>
            </Form.Item>
            <Form.Item name="generateBullets" valuePropName="checked">
              <Checkbox>{tr("是否生成卖点列表")}</Checkbox>
            </Form.Item>
            <Form.Item name="keepOriginalParams" valuePropName="checked">
              <Checkbox>{tr("是否保留原参数")}</Checkbox>
            </Form.Item>
            <Form.Item name="crossBorderReady" valuePropName="checked">
              <Checkbox>{tr("是否适合跨境平台")}</Checkbox>
            </Form.Item>

            <Typography.Title level={5}>{tr("通用配置")}</Typography.Title>
            <Form.Item name="language" label={tr("目标语言")}>
              <Select options={AI_LANGUAGE_OPTIONS} placeholder={tr("请选择生成语言")} />
            </Form.Item>
            <Form.Item name="platform" label={tr("目标平台")}>
              <Select
                options={AI_TARGET_PLATFORM_OPTIONS}
                placeholder={tr("请选择目标平台")}
                showSearch
                optionFilterProp="label"
              />
            </Form.Item>
            <Form.Item name="tone" label={tr("语气风格")}>
              <Select options={AI_TONE_OPTIONS} placeholder={tr("请选择文案语气")} />
            </Form.Item>
            <Form.Item name="keywords" label={tr("关键词（逗号分隔）")}>
              <Input placeholder={tr("可选")} />
            </Form.Item>
            <Form.Item name="forbiddenWords" label={tr("禁用词（逗号分隔）")}>
              <Input placeholder={tr("可选")} />
            </Form.Item>
            <Form.Item name="remark" label={tr("备注")}>
              <Input.TextArea rows={2} placeholder={tr("可选，不会写入 Prompt 日志")} />
            </Form.Item>
          </Form>
          <Space>
            <Button onClick={() => setStep(1)}>{tr("上一步")}</Button>
            <Button type="primary" onClick={() => { setCheckResult(null); setStep(3); }}>
              {tr("下一步")}
            </Button>
          </Space>
        </Card>
      )}

      {step === 3 && (
        <Card title={tr("确认并开始")}>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message={tr("AI 结果生成后不会自动覆盖商品内容，需要人工复核后再应用。")}
          />
          <Descriptions column={1} bordered size="small" style={{ marginBottom: 16 }}>
            {operationTypes.includes('title') && (
              <Descriptions.Item label={tr("标题生成")}>
                {tr("将为")} {productIds.length} {tr("个商品生成 AI 标题")}
              </Descriptions.Item>
            )}
            {operationTypes.includes('description') && (
              <Descriptions.Item label={tr("描述生成")}>
                {tr("将为")} {productIds.length} {tr("个商品生成 AI 描述")}
              </Descriptions.Item>
            )}
            <Descriptions.Item label={tr("预计结果数")}>{expectedItems} {tr("个")}</Descriptions.Item>
          </Descriptions>

          {checking ? (
            <Spin tip={tr("正在检查…")} />
          ) : checkResult ? (
            <>
              <Descriptions size="small" column={4} style={{ marginBottom: 12 }}>
                <Descriptions.Item label={tr("可生成")}>{checkResult.summary.readyCount}</Descriptions.Item>
                <Descriptions.Item label={tr("有提醒")}>{checkResult.summary.warningCount}</Descriptions.Item>
                <Descriptions.Item label={tr("不可生成")}>{checkResult.summary.blockedCount}</Descriptions.Item>
              </Descriptions>
              <Table<CheckBatchItem>
                rowKey={(r) => `${r.productId}-${r.operationType}`}
                size="small"
                pagination={false}
                scroll={{ x: 900, y: 320 }}
                dataSource={checkResult.items}
                columns={[
                  { title: tr("商品"), dataIndex: 'productTitle', ellipsis: true },
                  { title: tr("类型"), dataIndex: 'operationLabel', width: 100 },
                  {
                    title: tr("状态"),
                    dataIndex: 'statusLabel',
                    width: 120,
                    render: (_, row) => (
                      <Tag color={checkStatusColor(row.status)}>{row.statusLabel}</Tag>
                    ),
                  },
                  {
                    title: tr("说明"),
                    dataIndex: 'issues',
                    render: (issues: string[]) =>
                      issues?.length ? issues.join('；') : '—',
                  },
                ]}
              />
              <TechnicalDetails label={tr("技术详情")}>
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
            <Button onClick={() => setStep(2)}>{tr("上一步")}</Button>
            <Button onClick={() => void runCheck()} loading={checking}>
              {tr("重新检查")}
            </Button>
            <Button
              type="primary"
              loading={creating}
              disabled={!!checkResult && checkResult.summary.blockedCount === checkResult.summary.itemCount}
              onClick={() => void onCreate()}
            >
              {tr("开始生成")}
            </Button>
          </Space>
        </Card>
      )}
    </TmPageContainer>
  );
}
