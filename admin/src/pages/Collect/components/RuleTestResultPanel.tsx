import type { ReactNode } from 'react';
import { Alert, Collapse, Descriptions, Space, Tag, Typography } from 'antd';
import type { CollectRuleTestResult } from '@/services/collectRules';
import { accessStatusHint, accessStatusLabel } from '@/constants/collectAccess';
import {
  mapCollectFieldLabel,
  mapCollectorErrorCodeDetail,
  mapCollectorErrorCodeLabel,
} from '@/constants/collectErrors';

const { Paragraph, Text } = Typography;

type Props = {
  result: CollectRuleTestResult;
  showProduct?: boolean;
};

function fieldOk(v: boolean | number | undefined): string {
  if (typeof v === 'boolean') return v ? '已识别' : '未识别';
  if (typeof v === 'number') return `已识别 ${v} 项`;
  return '—';
}

export function RuleTestResultPanel({ result, showProduct = true }: Props) {
  const st = accessStatusLabel(result.accessStatus);
  const hint = accessStatusHint(result.accessStatus);
  const ef = result.extractedFields ?? {};
  const errorLabel = result.errorCode ? mapCollectorErrorCodeLabel(result.errorCode) : '';
  const errorDetail = result.errorCode ? mapCollectorErrorCodeDetail(result.errorCode) : '';

  const technicalItems: { key: string; label: string; children: ReactNode }[] = [];
  if (result.errorCode) {
    technicalItems.push({
      key: 'errorCode',
      label: '错误码',
      children: <Text code>{result.errorCode}</Text>,
    });
  }
  if (result.accessStatus) {
    technicalItems.push({
      key: 'accessStatus',
      label: '访问状态标识',
      children: <Text code>{result.accessStatus}</Text>,
    });
  }
  if (result.httpStatus) {
    technicalItems.push({
      key: 'http',
      label: 'HTTP 状态',
      children: String(result.httpStatus),
    });
  }

  return (
    <div style={{ marginTop: 16 }}>
      <Paragraph strong>采集效果测试</Paragraph>
      <Descriptions bordered size="small" column={1}>
        <Descriptions.Item label="页面访问状态">
          <Tag color={st.color}>{st.text}</Tag>
        </Descriptions.Item>
        {result.finalUrl ? (
          <Descriptions.Item label="实际打开链接">
            <Text copyable ellipsis>
              {result.finalUrl}
            </Text>
          </Descriptions.Item>
        ) : null}
        {errorLabel ? (
          <Descriptions.Item label="问题说明">
            <div>
              <Text strong>{errorLabel}</Text>
              {errorDetail ? (
                <Paragraph type="secondary" style={{ marginBottom: 0, marginTop: 4 }}>
                  {errorDetail}
                </Paragraph>
              ) : null}
            </div>
          </Descriptions.Item>
        ) : null}
        <Descriptions.Item label="商品标题">{fieldOk(ef.title)}</Descriptions.Item>
        <Descriptions.Item label="商品价格">{fieldOk(ef.price)}</Descriptions.Item>
        <Descriptions.Item label="商品主图">{fieldOk(ef.mainImage)}</Descriptions.Item>
        <Descriptions.Item label="详情图片">{fieldOk(ef.detailImagesCount)}</Descriptions.Item>
        <Descriptions.Item label="商品参数">{fieldOk(ef.attributesCount)}</Descriptions.Item>
        {result.missingFields?.length ? (
          <Descriptions.Item label="未识别项">
            <Space wrap>
              {result.missingFields.map((f) => (
                <Tag key={f}>{mapCollectFieldLabel(f)}</Tag>
              ))}
            </Space>
          </Descriptions.Item>
        ) : null}
        {result.warnings?.length ? (
          <Descriptions.Item label="提示">{result.warnings.join(' · ')}</Descriptions.Item>
        ) : null}
        {result.suggestion ? (
          <Descriptions.Item label="建议">{result.suggestion}</Descriptions.Item>
        ) : null}
      </Descriptions>

      {hint ? (
        <Alert type="warning" showIcon style={{ marginTop: 12 }} message={hint} />
      ) : null}

      {technicalItems.length > 0 ? (
        <Collapse
          ghost
          size="small"
          style={{ marginTop: 12 }}
          items={[
            {
              key: 'tech',
              label: '展开查看技术信息',
              children: (
                <Descriptions bordered size="small" column={1}>
                  {technicalItems.map((item) => (
                    <Descriptions.Item key={item.key} label={item.label}>
                      {item.children}
                    </Descriptions.Item>
                  ))}
                </Descriptions>
              ),
            },
          ]}
        />
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
        识别预览
      </Paragraph>
      <Descriptions bordered size="small" column={1}>
        <Descriptions.Item label="商品标题">{String(product.title ?? '—')}</Descriptions.Item>
        <Descriptions.Item label="商品价格">
          {raw?.productPrice != null ? String(raw.productPrice) : '—'}
        </Descriptions.Item>
        <Descriptions.Item label="商品主图">
          {Array.isArray(product.mainImages) ? `${product.mainImages.length} 张` : '—'}
        </Descriptions.Item>
      </Descriptions>
    </div>
  );
}
