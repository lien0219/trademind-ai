import type { ActionType, ProColumns } from '@ant-design/pro-components';
import {
  ModalForm,
  PageContainer,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { Button, Descriptions, Popconfirm, Space, Tag, Typography, message } from 'antd';
import dayjs from 'dayjs';
import { useCallback, useEffect, useRef, useState } from 'react';
import type { CollectRuleRow } from '@/services/collectRules';
import {
  createCollectRule,
  deleteCollectRule,
  disableCollectRule,
  enableCollectRule,
  getCollectRule,
  queryCollectRules,
  testCollectRule,
  updateCollectRule,
} from '@/services/collectRules';

const { Paragraph } = Typography;

export const DEFAULT_CUSTOM_RULE_TEMPLATE = `{
  "title": {
    "selectors": ["h1", "[property='og:title']"],
    "attr": "text"
  },
  "mainImages": {
    "selectors": [".product-gallery img", "[property='og:image']"],
    "attr": "src",
    "multiple": true,
    "limit": 10
  },
  "descriptionImages": {
    "selectors": [".product-description img"],
    "attr": "src",
    "multiple": true,
    "limit": 30
  },
  "attributes": {
    "mode": "pairs",
    "rowSelector": ".spec-row",
    "keySelector": ".spec-key",
    "valueSelector": ".spec-value"
  },
  "fallbacks": {
    "jsonLd": true,
    "openGraph": true,
    "meta": true
  }
}`;

function formatTs(s?: string) {
  if (!s) return '—';
  const d = dayjs(s);
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : s;
}

export default function CollectRulesPage() {
  const actionRef = useRef<ActionType>();
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [testOpen, setTestOpen] = useState(false);
  const [testRuleId, setTestRuleId] = useState<string | null>(null);
  const [preview, setPreview] = useState<unknown>(null);

  const reload = useCallback(() => actionRef.current?.reload(), []);

  useEffect(() => {
    if (!testOpen) setPreview(null);
  }, [testOpen]);

  const columns: ProColumns<CollectRuleRow>[] = [
    { title: '名称', dataIndex: 'name', ellipsis: true, width: 180 },
    { title: '域名', dataIndex: 'domain', ellipsis: true, width: 200, copyable: true },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (_, row) =>
        row.status === 'enabled' ? <Tag color="success">启用</Tag> : <Tag>停用</Tag>,
    },
    { title: '优先级', dataIndex: 'priority', width: 96, search: false },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      width: 172,
      search: false,
      render: (_, row) => formatTs(row.updatedAt),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 280,
      render: (_, row) => (
        <Space wrap size="small">
          <a
            key="edit"
            onClick={() => {
              setEditingId(row.id);
              setEditorOpen(true);
            }}
          >
            编辑
          </a>
          <a
            key="test"
            onClick={() => {
              setTestRuleId(row.id);
              setPreview(null);
              setTestOpen(true);
            }}
          >
            测试
          </a>
          {row.status === 'enabled' ? (
            <Popconfirm key="dis" title="停用该规则？" onConfirm={() => void toggle(row.id, false)}>
              <a>停用</a>
            </Popconfirm>
          ) : (
            <Popconfirm key="en" title="启用该规则？" onConfirm={() => void toggle(row.id, true)}>
              <a>启用</a>
            </Popconfirm>
          )}
          <Popconfirm key="del" title="确定删除？" onConfirm={() => void remove(row.id)}>
            <a style={{ color: '#cf1322' }}>删除</a>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  async function toggle(id: string, enable: boolean) {
    try {
      if (enable) await enableCollectRule(id);
      else await disableCollectRule(id);
      message.success('已更新');
      reload();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '操作失败');
    }
  }

  async function remove(id: string) {
    try {
      await deleteCollectRule(id);
      message.success('已删除');
      reload();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '删除失败');
    }
  }

  return (
    <PageContainer
      title="采集规则"
      extra={
        <Button
          type="primary"
          onClick={() => {
            setEditingId(null);
            setEditorOpen(true);
          }}
        >
          新建规则
        </Button>
      }
    >
      <ProTable<CollectRuleRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        options={{ reload: true }}
        headerTitle={false}
        request={async (params) => {
          const res = await queryCollectRules({
            page: params.current,
            pageSize: params.pageSize,
            name: params.name as string | undefined,
            domain: params.domain as string | undefined,
            status: params.status as string | undefined,
          });
          return {
            data: res.list,
            success: true,
            total: res.pagination.total,
          };
        }}
      />

      <ModalForm<{
        name: string;
        domain: string;
        matchPattern?: string;
        priority?: number;
        status?: string;
        remark?: string;
        ruleJson: string;
      }>
        key={editingId ?? 'new'}
        title={editingId ? '编辑采集规则' : '新建采集规则'}
        open={editorOpen}
        modalProps={{
          destroyOnClose: true,
          width: 720,
          onCancel: () => {
            setEditorOpen(false);
            setEditingId(null);
          },
        }}
        initialValues={{
          priority: 100,
          status: 'enabled',
          ruleJson: DEFAULT_CUSTOM_RULE_TEMPLATE,
        }}
        onOpenChange={(open) => {
          if (!open) {
            setEditorOpen(false);
            setEditingId(null);
          }
        }}
        onFinish={async (vals) => {
          let parsed: unknown;
          try {
            parsed = JSON.parse(vals.ruleJson || '{}');
          } catch {
            message.error('rule 必须是合法 JSON');
            return false;
          }
          try {
            if (editingId) {
              await updateCollectRule(editingId, {
                name: vals.name,
                domain: vals.domain,
                matchPattern: vals.matchPattern,
                priority: vals.priority,
                status: vals.status,
                remark: vals.remark,
                rule: parsed,
              });
              message.success('已保存');
            } else {
              await createCollectRule({
                name: vals.name,
                domain: vals.domain,
                matchPattern: vals.matchPattern,
                priority: vals.priority,
                status: vals.status,
                remark: vals.remark,
                rule: parsed,
              });
              message.success('已创建');
            }
            setEditorOpen(false);
            reload();
            return true;
          } catch (e) {
            message.error(e instanceof Error ? e.message : '保存失败');
            return false;
          }
        }}
        request={async () => {
          if (!editingId) {
            return {
              priority: 100,
              status: 'enabled',
              ruleJson: DEFAULT_CUSTOM_RULE_TEMPLATE,
            };
          }
          const d = await getCollectRule(editingId);
          return {
            name: d.name,
            domain: d.domain,
            matchPattern: d.matchPattern,
            priority: d.priority,
            status: d.status,
            remark: d.remark,
            ruleJson: JSON.stringify(d.rule ?? {}, null, 2),
          };
        }}
      >
        <ProFormText name="name" label="名称" rules={[{ required: true }]} />
        <ProFormText
          name="domain"
          label="域名"
          placeholder="example.com"
          rules={[{ required: true }]}
          extra="匹配主机名与其子域（如 www.example.com）。"
        />
        <ProFormText name="matchPattern" label="URL 正则（可选）" placeholder="留空则仅按域名匹配整站 URL" />
        <ProFormDigit name="priority" label="优先级" fieldProps={{ min: 0, max: 1_000_000 }} />
        <ProFormSelect
          name="status"
          label="状态"
          options={[
            { label: '启用', value: 'enabled' },
            { label: '停用', value: 'disabled' },
          ]}
          rules={[{ required: true }]}
        />
        <ProFormTextArea name="remark" label="备注" fieldProps={{ rows: 2 }} />
        <div style={{ marginBottom: 8 }}>
          <Button
            size="small"
            onClick={() => {
              void navigator.clipboard.writeText(DEFAULT_CUSTOM_RULE_TEMPLATE).then(
                () => message.success('模板已复制到剪贴板，请粘贴到 rule JSON'),
                () => message.warning('复制失败，请手动复制模板'),
              );
            }}
          >
            复制默认规则模板
          </Button>
        </div>
        <ProFormTextArea
          name="ruleJson"
          label="rule（JSON）"
          rules={[{ required: true }]}
          fieldProps={{ rows: 14, style: { fontFamily: 'monospace', fontSize: 12 } }}
        />
      </ModalForm>

      <ModalForm<{ url: string }>
        title="测试规则"
        open={testOpen}
        modalProps={{
          destroyOnClose: true,
          width: 720,
          onCancel: () => {
            setTestOpen(false);
            setTestRuleId(null);
          },
        }}
        onOpenChange={(o) => {
          if (!o) {
            setTestOpen(false);
            setTestRuleId(null);
          }
        }}
        onFinish={async (vals) => {
          if (!testRuleId) return false;
          try {
            const res = await testCollectRule(testRuleId, { url: vals.url.trim() });
            setPreview(res.product);
            message.success('预览成功');
            return false;
          } catch (e) {
            message.error(e instanceof Error ? e.message : '测试失败');
            setPreview(null);
            return false;
          }
        }}
        submitter={{ searchConfig: { submitText: '运行测试' } }}
      >
        <ProFormText name="url" label="商品页 URL" rules={[{ required: true }]} />
        {preview && typeof preview === 'object' && preview !== null ? (
          <ProductPreviewBlock product={preview as Record<string, unknown>} />
        ) : null}
      </ModalForm>
    </PageContainer>
  );
}

function ProductPreviewBlock({ product }: { product: Record<string, unknown> }) {
  const raw = product.raw as Record<string, unknown> | undefined;
  const digest = raw?.stateDigest as Record<string, unknown> | undefined;
  return (
    <div style={{ marginTop: 16 }}>
      <Paragraph strong>预览摘要</Paragraph>
      <Descriptions bordered size="small" column={1}>
        <Descriptions.Item label="title">{String(product.title ?? '—')}</Descriptions.Item>
        <Descriptions.Item label="currency">{String(product.currency ?? '—')}</Descriptions.Item>
        <Descriptions.Item label="mainImages">
          {Array.isArray(product.mainImages) ? `${product.mainImages.length} 张` : '—'}
        </Descriptions.Item>
        <Descriptions.Item label="descriptionImages">
          {Array.isArray(product.descriptionImages) ? `${product.descriptionImages.length} 张` : '—'}
        </Descriptions.Item>
        <Descriptions.Item label="attributes">
          {product.attributes && typeof product.attributes === 'object'
            ? JSON.stringify(product.attributes)
            : '—'}
        </Descriptions.Item>
        <Descriptions.Item label="skus">
          {Array.isArray(product.skus) ? `${product.skus.length} 条` : '—'}
        </Descriptions.Item>
        <Descriptions.Item label="raw.stateDigest">{digest ? JSON.stringify(digest) : '—'}</Descriptions.Item>
      </Descriptions>
    </div>
  );
}
