import { PageContainer } from '@ant-design/pro-components';
import { useParams } from '@umijs/max';
import { Card, Descriptions, Image, Spin, Table, Typography } from 'antd';
import { useEffect, useState } from 'react';
import { PRODUCT_STATUS } from '@/constants/status';
import { fetchProductDetail, type ProductDetail } from '@/services/products';

export default function ProductDraftDetailPage() {
  const params = useParams<{ id: string }>();
  const id = params.id ?? '';
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<ProductDetail | null>(null);
  const [err, setErr] = useState<string>();

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    (async () => {
      setLoading(true);
      setErr(undefined);
      try {
        const d = await fetchProductDetail(id);
        if (!cancelled) setData(d);
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
    <PageContainer title="商品详情" subTitle="基础展示；后续再对齐编辑与 SKU 管理。">
      {loading ? (
        <Spin />
      ) : err ? (
        <Typography.Text type="danger">{err}</Typography.Text>
      ) : data ? (
        <>
          <Card title="概要" style={{ marginBottom: 16 }}>
            <Descriptions column={2} bordered size="small">
              <Descriptions.Item label="标题">{data.title}</Descriptions.Item>
              <Descriptions.Item label="原始标题">{data.originalTitle}</Descriptions.Item>
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

          <Card title="图片" style={{ marginBottom: 16 }}>
            {data.images?.length ? (
              <Image.PreviewGroup>
                <ThumbGrid urls={data.images.map((i) => i.publicUrl || i.originUrl)} />
              </Image.PreviewGroup>
            ) : (
              <Typography.Text type="secondary">暂无图片</Typography.Text>
            )}
          </Card>

          <Card title="SKU">
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

          {data.rawData != null ? (
            <Card title="Raw JSON（采集归一）" style={{ marginTop: 16 }}>
              <pre style={{ maxHeight: 360, overflow: 'auto', fontSize: 12 }}>
                {JSON.stringify(data.rawData, null, 2)}
              </pre>
            </Card>
          ) : null}
        </>
      ) : null}
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
