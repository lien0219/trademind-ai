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
import { Button, Popconfirm, Space, Tag, Typography, message } from 'antd';
import dayjs from 'dayjs';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { CollectRuleRow, CollectRuleTestResult } from '@/services/collectRules';
import { BrowserProfileLoginPanel } from '@/pages/Collect/components/BrowserProfileLoginPanel';
import { AIGenerateRuleModal } from '@/pages/Collect/components/AIGenerateRuleModal';
import { RuleTestResultPanel } from '@/pages/Collect/components/RuleTestResultPanel';
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

/** v1 简写：selector + type（保存时后端会规范化为 selectors + attr） */
export const SIMPLE_CUSTOM_RULE_TEMPLATE = `{
  "title": { "selector": "h1", "type": "text" },
  "price": { "selector": ".price", "type": "text" },
  "mainImage": {
    "selector": "#spec-img, img#spec-img, img[data-origin], img[data-lazy-img], meta[property='og:image']",
    "type": "attr",
    "attr": "src"
  },
  "detailImages": { "selector": ".detail img", "type": "attr_all", "attr": "src" },
  "attributes": { "mode": "disabled" },
  "fallbacks": { "jsonLd": true, "openGraph": true, "meta": true }
}`;

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
  const [testResult, setTestResult] = useState<CollectRuleTestResult | null>(null);
  const [testUrl, setTestUrl] = useState('');
  const [testProfileId, setTestProfileId] = useState<string | undefined>();
  const [testUseProfile, setTestUseProfile] = useState(false);
  const [rulesCache, setRulesCache] = useState<CollectRuleRow[]>([]);
  const [aiModalOpen, setAiModalOpen] = useState(false);

  const reload = useCallback(() => actionRef.current?.reload(), []);

  useEffect(() => {
    if (!testOpen) {
      setTestResult(null);
      setTestUrl('');
      setTestProfileId(undefined);
      setTestUseProfile(false);
    }
  }, [testOpen]);

  const testRuleDomain = useMemo(() => {
    if (!testRuleId) return '';
    return rulesCache.find((r) => r.id === testRuleId)?.domain ?? '';
  }, [testRuleId, rulesCache]);

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
              setTestResult(null);
              setTestProfileId(undefined);
              setTestUseProfile(false);
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
      subTitle="适用于自定义链接采集器"
      extra={
        <Space>
          <Button onClick={() => setAiModalOpen(true)}>AI 生成规则</Button>
          <Button
            type="primary"
            onClick={() => {
              setEditingId(null);
              setEditorOpen(true);
            }}
          >
            新建规则
          </Button>
        </Space>
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
          setRulesCache(res.list ?? []);
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
          destroyOnHidden: true,
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
          placeholder="jd.com"
          rules={[{ required: true }]}
          extra="仅填主机名，不要带 https://。填 jd.com 可匹配 item.jd.com；填 1688.com 可匹配 detail.1688.com。勿只填 www.jd.com / www.1688.com（无法匹配 item / detail 子域）。"
          transform={(v) => {
            const s = typeof v === 'string' ? v : '';
            if (!s.trim()) return s;
            try {
              if (s.includes('://')) return new URL(s).hostname.toLowerCase();
            } catch {
              /* keep raw */
            }
            return s.trim().toLowerCase().split('/')[0].split(':')[0];
          }}
        />
        <ProFormText
          name="matchPattern"
          label="URL 正则（可选）"
          placeholder="留空则仅按域名匹配；1688 详情可填 ^https://detail\\.1688\\.com/offer/"
        />
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
          <Space wrap>
            <Button
              size="small"
              onClick={() => {
                void navigator.clipboard.writeText(SIMPLE_CUSTOM_RULE_TEMPLATE).then(
                  () => message.success('已复制简写模板（selector + type）'),
                  () => message.warning('复制失败'),
                );
              }}
            >
              复制简写模板
            </Button>
            <Button
              size="small"
              onClick={() => {
                void navigator.clipboard.writeText(DEFAULT_CUSTOM_RULE_TEMPLATE).then(
                  () => message.success('已复制完整模板（selectors + attr）'),
                  () => message.warning('复制失败'),
                );
              }}
            >
              复制完整模板
            </Button>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              规则格式见仓库 docs/custom-collect-rules.md
            </Typography.Text>
          </Space>
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
          destroyOnHidden: true,
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
            const res = await testCollectRule(testRuleId, {
              url: vals.url.trim(),
              profileId: testUseProfile ? testProfileId : undefined,
              useBrowserProfile: testUseProfile && Boolean(testProfileId),
            });
            setTestResult(res);
            if (res.accessStatus === 'public' && !res.missingFields?.length) {
              message.success('访问与字段提取正常');
            } else {
              message.info(res.suggestion || '测试完成');
            }
            return false;
          } catch (e) {
            message.error(e instanceof Error ? e.message : '测试失败');
            setTestResult(null);
            return false;
          }
        }}
        submitter={{ searchConfig: { submitText: '运行测试' } }}
      >
        <ProFormText
          name="url"
          label="商品页 URL"
          rules={[{ required: true }]}
          fieldProps={{
            onChange: (e) => setTestUrl(e.target.value),
          }}
        />
        {testResult ? <RuleTestResultPanel result={testResult} /> : null}
        {testResult?.accessStatus === 'login_required' ? (
          <BrowserProfileLoginPanel
            url={testUrl}
            domain={testRuleDomain}
            profileId={testProfileId}
            useBrowserProfile={testUseProfile}
            onProfileIdChange={setTestProfileId}
            onUseProfileChange={setTestUseProfile}
          />
        ) : null}
      </ModalForm>

      <AIGenerateRuleModal
        open={aiModalOpen}
        onClose={() => setAiModalOpen(false)}
        onSaved={() => reload()}
      />
    </PageContainer>
  );
}
