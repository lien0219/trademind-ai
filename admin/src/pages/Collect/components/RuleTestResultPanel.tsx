import { Alert, Descriptions, Space, Tag, Typography } from 'antd';
import type { CollectRuleTestResult } from '@/services/collectRules';
import { accessStatusHint, accessStatusLabel } from '@/constants/collectAccess';
import { mapCollectorErrorCodeLabel } from '@/constants/collectErrors';

const { Paragraph, Text } = Typography;

type Props = {
  result: CollectRuleTestResult;
  showProduct?: boolean;
};

function fieldOk(v: boolean | number | undefined): string {
  if (typeof v === 'boolean') return v ? '成功' : '未提取';
  if (typeof v === 'number') return String(v);
  return '—';
}

export function RuleTestResultPanel({ result, showProduct = true }: Props) {
  const st = accessStatusLabel(result.accessStatus);
  const hint = accessStatusHint(result.accessStatus);
  const ef = result.extractedFields ?? {};

  return (
    <div style={{ marginTop: 16 }}>
      <Paragraph strong>访问与规则测试</Paragraph>
      <Descriptions bordered size="small" column={1}>
        <Descriptions.Item label="页面访问状态">
          <Tag color={st.color}>{st.text}</Tag>
          {result.accessStatus === 'login_required' ? (
            <Text type="secondary" style={{ marginLeft: 8 }}>
              疑似需要登录
            </Text>
          ) : null}
          {result.accessStatus === 'verify_required' ? (
            <Text type="secondary" style={{ marginLeft: 8 }}>
              疑似风控验证
            </Text>
          ) : null}
        </Descriptions.Item>
        <Descriptions.Item label="最终 URL">
          <Text copyable ellipsis>
            {result.finalUrl || '—'}
          </Text>
        </Descriptions.Item>
        {result.httpStatus ? (
          <Descriptions.Item label="HTTP 状态">{result.httpStatus}</Descriptions.Item>
        ) : null}
        {result.errorCode ? (
          <Descriptions.Item label="错误码">
            {result.errorCode}
            {mapCollectorErrorCodeLabel(result.errorCode) ? (
              <Text type="secondary" style={{ marginLeft: 8 }}>
                {mapCollectorErrorCodeLabel(result.errorCode)}
              </Text>
            ) : null}
          </Descriptions.Item>
        ) : null}
        <Descriptions.Item label="标题">{fieldOk(ef.title)}</Descriptions.Item>
        <Descriptions.Item label="价格">{fieldOk(ef.price)}</Descriptions.Item>
        <Descriptions.Item label="主图">{fieldOk(ef.mainImage)}</Descriptions.Item>
        <Descriptions.Item label="详情图数量">
          {fieldOk(ef.detailImagesCount)}
        </Descriptions.Item>
        <Descriptions.Item label="属性数量">{fieldOk(ef.attributesCount)}</Descriptions.Item>
        {result.missingFields?.length ? (
          <Descriptions.Item label="缺失字段">
            <Space wrap>
              {result.missingFields.map((f) => (
                <Tag key={f}>{f}</Tag>
              ))}
            </Space>
          </Descriptions.Item>
        ) : null}
        {result.warnings?.length ? (
          <Descriptions.Item label="警告">{result.warnings.join(' · ')}</Descriptions.Item>
        ) : null}
        {result.suggestion ? (
          <Descriptions.Item label="建议">{result.suggestion}</Descriptions.Item>
        ) : null}
      </Descriptions>

      {hint ? (
        <Alert type="warning" showIcon style={{ marginTop: 12 }} message={hint} />
      ) : null}

      {showProduct && result.product && typeof result.product === 'object' ? (
        <ProductSnippet product={result.product as Record<string, unknown>} />
      ) : null}
    </div>
  );
}

function ProductSnippet({ product }: { product: Record<string, unknown> }) {
  const raw = product.raw as Record<string, unknown> | undefined;
  return (
    <div style={{ marginTop: 12 }}>
      <Paragraph strong style={{ marginBottom: 8 }}>
        商品预览
      </Paragraph>
      <Descriptions bordered size="small" column={1}>
        <Descriptions.Item label="title">{String(product.title ?? '—')}</Descriptions.Item>
        <Descriptions.Item label="price">
          {raw?.productPrice != null ? String(raw.productPrice) : '—'}
        </Descriptions.Item>
        <Descriptions.Item label="mainImages">
          {Array.isArray(product.mainImages) ? `${product.mainImages.length} 张` : '—'}
        </Descriptions.Item>
      </Descriptions>
    </div>
  );
}
