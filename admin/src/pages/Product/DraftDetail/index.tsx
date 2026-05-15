import { PageContainer } from '@ant-design/pro-components';
import { useParams } from '@umijs/max';
import { Button, Card, Descriptions, Form, Image, Input, InputNumber, Modal, Spin, Table, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { PRODUCT_STATUS } from '@/constants/status';
import {
  applyProductAITitle,
  fetchProductAITasks,
  fetchProductDetail,
  optimizeProductTitle,
  type AITaskRow,
  type OptimizeTitleResult,
  type ProductDetail,
} from '@/services/products';

export default function ProductDraftDetailPage() {
  const params = useParams<{ id: string }>();
  const id = params.id ?? '';
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<ProductDetail | null>(null);
  const [err, setErr] = useState<string>();
  const [aiOpen, setAiOpen] = useState(false);
  const [aiBusy, setAiBusy] = useState(false);
  const [aiResult, setAiResult] = useState<OptimizeTitleResult | null>(null);
  const [aiTasks, setAiTasks] = useState<AITaskRow[]>([]);
  const [aiForm] = Form.useForm();

  const reloadDetail = useCallback(async () => {
    if (!id) return;
    const d = await fetchProductDetail(id);
    setData(d);
  }, [id]);

  const reloadTasks = useCallback(async () => {
    if (!id) return;
    try {
      const { list } = await fetchProductAITasks(id);
      setAiTasks(list ?? []);
    } catch {
      setAiTasks([]);
    }
  }, [id]);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    (async () => {
      setLoading(true);
      setErr(undefined);
      try {
        const d = await fetchProductDetail(id);
        if (!cancelled) setData(d);
        if (!cancelled) {
          try {
            const { list } = await fetchProductAITasks(id);
            if (!cancelled) setAiTasks(list ?? []);
          } catch {
            if (!cancelled) setAiTasks([]);
          }
        }
      } catch (e) {
        if (!cancelled) setErr(e instanceof Error ? e.message : String(e));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [id]);

  if (!id) {
    return (
      <PageContainer title="商品详情">
        <Typography.Text type="danger">无效的商品 ID</Typography.Text>
      </PageContainer>
    );
  }

  return (
    <PageContainer title="商品详情" subTitle="基础展示与 AI 标题优化；后续再对齐深度编辑与 SKU 管理。">
      {loading ? (
        <Spin />
      ) : err ? (
        <Typography.Text type="danger">{err}</Typography.Text>
      ) : data ? (
        <>
          <Card title="概要" style={{ marginBottom: 16 }}>
            <Descriptions column={2} bordered size="small">
              <Descriptions.Item label="主标题">{data.title}</Descriptions.Item>
              <Descriptions.Item label="原始标题">{data.originalTitle}</Descriptions.Item>
              <Descriptions.Item label="AI 标题">{data.aiTitle || '—'}</Descriptions.Item>
              <Descriptions.Item label="来源">{data.source}</Descriptions.Item>
              <Descriptions.Item label="来源链接" span={2}>
                <Typography.Link href={data.sourceUrl} target="_blank" rel="noreferrer">
                  {data.sourceUrl || '—'}
                </Typography.Link>
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                {PRODUCT_STATUS[data.status as keyof typeof PRODUCT_STATUS]?.text ?? data.status}
              </Descriptions.Item>
              <Descriptions.Item label="币种">{data.currency}</Descriptions.Item>
              <Descriptions.Item label="描述" span={2}>
                {data.description || '—'}
              </Descriptions.Item>
            </Descriptions>
          </Card>

          <Card
            title="AI 标题优化"
            style={{ marginBottom: 16 }}
            extra={
              <Button
                type="primary"
                onClick={() => {
                  setAiResult(null);
                  aiForm.resetFields();
                  aiForm.setFieldsValue({ language: 'en', platform: 'TikTok Shop', maxLength: 120 });
                  setAiOpen(true);
                }}
              >
                AI 优化标题
              </Button>
            }
          >
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
              调用系统 AI 设置与「product_title_optimize」Prompt；结果需手动应用才会写入「AI 标题」字段，不会直接覆盖主标题。
            </Typography.Paragraph>
          </Card>

          <Card title="图片" style={{ marginBottom: 16 }}>
            {data.images?.length ? (
              <Image.PreviewGroup>
                <ThumbGrid urls={data.images.map((i) => i.publicUrl || i.originUrl)} />
              </Image.PreviewGroup>
            ) : (
              <Typography.Text type="secondary">暂无图片</Typography.Text>
            )}
          </Card>

          <Card title="SKU" style={{ marginBottom: 16 }}>
            <Table
              rowKey="id"
              size="small"
              pagination={false}
              dataSource={data.skus ?? []}
              columns={[
                { title: '编码', dataIndex: 'skuCode', width: 140 },
                { title: '名称', dataIndex: 'skuName', ellipsis: true },
                {
                  title: '价格',
                  dataIndex: 'price',
                  width: 100,
                  render: (v: number | undefined) => (v != null ? String(v) : '—'),
                },
                {
                  title: '库存',
                  dataIndex: 'stock',
                  width: 88,
                  render: (v: number | undefined) => (v != null ? String(v) : '—'),
                },
              ]}
            />
          </Card>

          <Card title="最近 AI 任务" style={{ marginBottom: 16 }}>
            <Table
              rowKey="id"
              size="small"
              pagination={false}
              dataSource={aiTasks}
              columns={[
                { title: '类型', dataIndex: 'taskType', width: 120 },
                { title: '状态', dataIndex: 'status', width: 100 },
                { title: '模型', dataIndex: 'model', ellipsis: true },
                {
                  title: 'Tokens',
                  width: 100,
                  render: (_: unknown, row: AITaskRow) => `${row.tokenInput ?? 0}/${row.tokenOutput ?? 0}`,
                },
                { title: 'Prompt', dataIndex: 'promptCode', width: 160, ellipsis: true },
                {
                  title: '时间',
                  dataIndex: 'createdAt',
                  width: 176,
                  render: (v: string) => v?.replace('T', ' ').slice(0, 19) ?? '—',
                },
              ]}
            />
          </Card>

          {data.rawData != null ? (
            <Card title="Raw JSON（采集归一）" style={{ marginTop: 16 }}>
              <pre style={{ maxHeight: 360, overflow: 'auto', fontSize: 12 }}>
                {JSON.stringify(data.rawData, null, 2)}
              </pre>
            </Card>
          ) : null}
        </>
      ) : null}

      <Modal
        title="AI 标题优化"
        open={aiOpen}
        onCancel={() => setAiOpen(false)}
        footer={null}
        destroyOnClose
        width={640}
      >
        <Form
          form={aiForm}
          layout="vertical"
          initialValues={{ language: 'en', platform: 'TikTok Shop', maxLength: 120 }}
          onFinish={async (v) => {
            setAiBusy(true);
            setAiResult(null);
            try {
              const res = await optimizeProductTitle(id, {
                language: String(v.language ?? ''),
                platform: String(v.platform ?? ''),
                maxLength: Number(v.maxLength ?? 120),
              });
              setAiResult(res);
              message.success('优化完成');
              await reloadTasks();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '优化失败');
            } finally {
              setAiBusy(false);
            }
          }}
        >
          <Form.Item name="language" label="语言" rules={[{ required: true }]}>
            <Input placeholder="en" />
          </Form.Item>
          <Form.Item name="platform" label="平台" rules={[{ required: true }]}>
            <Input placeholder="TikTok Shop" />
          </Form.Item>
          <Form.Item name="maxLength" label="最长字符数" rules={[{ required: true }]}>
            <InputNumber min={20} max={500} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={aiBusy}>
              运行优化
            </Button>
          </Form.Item>
        </Form>

        {aiResult ? (
          <div style={{ marginTop: 16 }}>
            <Typography.Title level={5}>结果</Typography.Title>
            <Descriptions bordered size="small" column={1} style={{ marginBottom: 16 }}>
              <Descriptions.Item label="优化标题">{aiResult.optimizedTitle || '—'}</Descriptions.Item>
              <Descriptions.Item label="关键词">
                {(aiResult.keywords ?? []).length ? aiResult.keywords.join('、') : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="说明">{aiResult.reason || '—'}</Descriptions.Item>
              <Descriptions.Item label="任务 ID">{aiResult.taskId}</Descriptions.Item>
            </Descriptions>
            <Button
              type="primary"
              disabled={!aiResult.optimizedTitle}
              loading={aiBusy}
              onClick={async () => {
                if (!aiResult?.taskId) return;
                setAiBusy(true);
                try {
                  await applyProductAITitle(id, {
                    aiTitle: aiResult.optimizedTitle,
                    taskId: aiResult.taskId,
                  });
                  message.success('已应用为 AI 标题');
                  setAiOpen(false);
                  setAiResult(null);
                  await reloadDetail();
                  await reloadTasks();
                } catch (e: unknown) {
                  message.error((e as Error)?.message || '应用失败');
                } finally {
                  setAiBusy(false);
                }
              }}
            >
              应用为 AI 标题
            </Button>
          </div>
        ) : null}
      </Modal>
    </PageContainer>
  );
}

function ThumbGrid({ urls }: { urls: string[] }) {
  const valid = urls.filter(Boolean);
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
      {valid.map((u) => (
        <Image key={u} src={u} width={96} height={96} style={{ objectFit: 'cover', borderRadius: 4 }} />
      ))}
    </div>
  );
}
