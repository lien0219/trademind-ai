import type { ReactNode } from 'react';
import { Alert, Collapse, Descriptions, Progress, Space, Tag, Typography } from 'antd';
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

function confidenceLabel(c?: string): string {
  switch (c) {
    case 'high':
      return '高';
    case 'medium':
      return '中';
    case 'low':
      return '低';
    default:
      return c || '—';
  }
}

export function RuleTestResultPanel({ result, showProduct = true }: Props) {
  const st = accessStatusLabel(result.accessStatus);
  const hint = accessStatusHint(result.accessStatus);
  const ef = result.extractedFields ?? {};
  const qs = result.qualityScore;
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

  const titleSuspect = ef.titleSuspectWrong === true;
  const userWarnings = (result.warnings ?? []).filter(
    (w) => !w.startsWith('title_extracted_but_') && w !== 'title_suspect_wrong' && w !== 'main_images_single_only',
  );

  return (
    <div style={{ marginTop: 16 }}>
      <Paragraph strong>采集效果测试</Paragraph>

      {typeof qs?.score === 'number' ? (
        <div style={{ marginBottom: 12 }}>
          <Text type="secondary">采集质量评分</Text>
          <Progress
            percent={Math.min(100, Math.max(0, qs.score))}
            status={qs.score >= 60 ? 'success' : qs.score >= 30 ? 'normal' : 'exception'}
            size="small"
          />
          {qs.hints?.length ? (
            <ul style={{ margin: '8px 0 0', paddingLeft: 20 }}>
              {qs.hints.slice(0, 6).map((h) => (
                <li key={h}>
                  <Text type="secondary">{h}</Text>
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : null}

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
        <Descriptions.Item label="商品标题">
          {ef.title ? (
            <Space direction="vertical" size={0}>
              <Text>{ef.titleText || '已识别'}</Text>
              {ef.titleSelector ? (
                <Text type="secondary" style={{ fontSize: 12 }}>
                  命中：{ef.titleSelector}
                </Text>
              ) : null}
              {ef.titleConfidence ? (
                <Text type="secondary" style={{ fontSize: 12 }}>
                  可信度：{confidenceLabel(String(ef.titleConfidence))}
                </Text>
              ) : null}
              {titleSuspect ? (
                <Alert
                  type="warning"
                  showIcon
                  message="当前标题可能不是商品标题，请调整商品标题对应的页面位置，或重新使用 AI 生成规则。"
                  style={{ marginTop: 4 }}
                />
              ) : null}
            </Space>
          ) : (
            fieldOk(false)
          )}
        </Descriptions.Item>
        <Descriptions.Item label="商品价格">{fieldOk(ef.price)}</Descriptions.Item>
        <Descriptions.Item label="商品主图">
          {ef.mainImage ?
            `${ef.mainImagesCount ?? 1} 张${(ef.mainImagesCount ?? 0) === 1 ? '（可能未抓全轮播）' : ''}`
          : fieldOk(false)}
        </Descriptions.Item>
        <Descriptions.Item label="详情图片">{fieldOk(ef.detailImagesCount)}</Descriptions.Item>
        <Descriptions.Item label="商品参数">
          {ef.attributesCount ?
            <Space direction="vertical" size={0}>
              <Text>{fieldOk(ef.attributesCount)}</Text>
              {ef.attributeSamples?.length ?
                <Text type="secondary" style={{ fontSize: 12 }}>
                  样例：{ef.attributeSamples.map((s) => `${s.key}=${s.value}`).join('；')}
                </Text>
              : null}
            </Space>
          : fieldOk(0)}
        </Descriptions.Item>
        <Descriptions.Item label="商品规格">
          <Text type="secondary">
            自定义规则通常无法完整识别规格与库存；复杂平台建议使用专用采集器。
          </Text>
        </Descriptions.Item>
        {result.missingFields?.length ? (
          <Descriptions.Item label="未识别项">
            <Space wrap>
              {result.missingFields.map((f) => (
                <Tag key={f}>{mapCollectFieldLabel(f)}</Tag>
              ))}
            </Space>
          </Descriptions.Item>
        ) : null}
        {userWarnings.length ? (
          <Descriptions.Item label="提示">{userWarnings.join(' · ')}</Descriptions.Item>
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
        <Descriptions.Item label="币种">{String(product.currency ?? '—')}</Descriptions.Item>
        <Descriptions.Item label="商品价格">
          {raw?.productPrice != null ? String(raw.productPrice) : '—'}
        </Descriptions.Item>
        <Descriptions.Item label="商品主图">
          {Array.isArray(product.mainImages) ? `${product.mainImages.length} 张` : '—'}
        </Descriptions.Item>
        <Descriptions.Item label="详情图片">
          {Array.isArray(product.descriptionImages) ? `${product.descriptionImages.length} 张` : '—'}
        </Descriptions.Item>
      </Descriptions>
    </div>
  );
}
