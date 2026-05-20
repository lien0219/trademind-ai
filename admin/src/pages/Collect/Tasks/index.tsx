import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProCard, ProTable } from '@ant-design/pro-components';
import { useLocation } from '@umijs/max';
import { Link } from '@umijs/renderer-react';
import { Alert, Button, Form, Input, Select, Space, Switch, Tag, Typography, message } from 'antd';
import dayjs from 'dayjs';
import { useEffect, useMemo, useRef, useState } from 'react';
import { COLLECT_TASK_STATUS } from '@/constants/status';
import { CollectTaskEventDrawer } from '@/pages/Collect/components/CollectTaskEventDrawer';
import type { CollectProviderRow, CollectProviderStatus } from '@/services/collectProviders';
import { queryCollectProviders } from '@/services/collectProviders';
import {
  createCollectTask,
  fetchCollectTasks,
  retryCollectTask,
  type CollectTaskRow,
} from '@/services/collectTasks';
import type { CollectRuleRow } from '@/services/collectRules';
import { queryCollectRules } from '@/services/collectRules';
import {
  mapCollectErrorMessage,
  mapCollectorErrorCodeDetail,
  mapCollectorErrorCodeLabel,
} from '@/constants/collectErrors';
import {
  formatRuleDomainMismatchMessage,
  ruleMatchesURL,
  suggestRuleDomainForHost,
} from '@/utils/collectRuleMatch';
import { BrowserProfileLoginPanel } from '@/pages/Collect/components/BrowserProfileLoginPanel';
import { classifyPinduoduoUrl, pinduoduoProfileDomain, pinduoduoUrlHint } from '@/utils/pinduoduoUrl';

function providerAllowsSingleCollect(status: CollectProviderStatus) {
  return status === 'available' || status === 'beta';
}

export default function CollectTasksPage() {
  const location = useLocation();
  const batchIdFromQuery = useMemo(() => {
    const q = new URLSearchParams(location.search || '');
    const v = q.get('batchId')?.trim();
    return v || undefined;
  }, [location.search]);

  const sourceFromQuery = useMemo(() => {
    const q = new URLSearchParams(location.search || '');
    return q.get('source')?.trim() ?? '';
  }, [location.search]);

  const actionRef = useRef<ActionType>();
  const [form] = Form.useForm<{ source: string; url: string; ruleId?: string }>();
  const [submitting, setSubmitting] = useState(false);
  const [polling, setPolling] = useState(4000);
  const [eventDrawerOpen, setEventDrawerOpen] = useState(false);
  const [eventDrawerTaskId, setEventDrawerTaskId] = useState<string | null>(null);

  const [providers, setProviders] = useState<CollectProviderRow[]>([]);
  const [enabledRules, setEnabledRules] = useState<CollectRuleRow[]>([]);
  const formSource = Form.useWatch('source', form);
  const formUrl = Form.useWatch('url', form);
  const [pddProfileId, setPddProfileId] = useState<string | undefined>();
  const [pddUseBrowserProfile, setPddUseBrowserProfile] = useState(false);
  const isPddSource = formSource === 'pinduoduo' || formSource === 'pdd';
  const pddUrlHint = useMemo(() => {
    const u = formUrl?.trim();
    if (!u || !isPddSource) return null;
    return pinduoduoUrlHint(u);
  }, [formUrl, isPddSource]);
  const pddUrlType = useMemo(
    () => (isPddSource ? classifyPinduoduoUrl(formUrl?.trim() ?? '') : 'unknown'),
    [formUrl, isPddSource],
  );

  useEffect(() => {
    const sync = () => setPolling(document.visibilityState === 'hidden' ? undefined : 4000);
    sync();
    document.addEventListener('visibilitychange', sync);
    return () => document.removeEventListener('visibilitychange', sync);
  }, []);

  useEffect(() => {
    void (async () => {
      try {
        const rows = await queryCollectProviders();
        setProviders(Array.isArray(rows) ? rows : []);
      } catch {
        setProviders([]);
      }
    })();
  }, []);

  useEffect(() => {
    if (formSource !== 'custom') {
      setEnabledRules([]);
      return;
    }
    void (async () => {
      try {
        const res = await queryCollectRules({ page: 1, pageSize: 500, status: 'enabled' });
        setEnabledRules(res.list ?? []);
      } catch {
        setEnabledRules([]);
      }
    })();
  }, [formSource]);

  const rulesForUrl = useMemo(() => {
    const url = formUrl?.trim();
    if (!url || formSource !== 'custom') return enabledRules;
    return enabledRules.filter((r) => ruleMatchesURL(r, url));
  }, [enabledRules, formSource, formUrl]);

  useEffect(() => {
    actionRef.current?.reload();
  }, [batchIdFromQuery]);

  useEffect(() => {
    if (!providers.length) return;
    const qs = sourceFromQuery;
    const fromQs =
      qs && providers.some((p) => p.source === qs && providerAllowsSingleCollect(p.status)) ? qs : undefined;
    const picked =
      fromQs ??
      providers.find((p) => p.source === '1688' && providerAllowsSingleCollect(p.status))?.source ??
      providers.find((p) => providerAllowsSingleCollect(p.status))?.source;
    if (!picked) return;
    form.setFieldsValue({
      source: picked,
      url: form.getFieldValue('url') ?? '',
      ...(picked !== 'custom' ? { ruleId: undefined } : {}),
    });
  }, [providers, sourceFromQuery, form]);

  const placeholderUrl = useMemo(() => {
    const p = providers.find((x) => x.source === formSource);
    const pat = p?.urlPatterns?.[0];
    return pat && pat.length > 0 ? pat : 'https://detail.1688.com/offer/...';
  }, [providers, formSource]);

  const providerSelectOptions = providers.map((p) => ({
    label: `${p.name}（${p.source}）`,
    value: p.source,
    disabled: !providerAllowsSingleCollect(p.status),
  }));

  const tableSourceEnum = useMemo(() => {
    const rec: Record<string, { text: string }> = {};
    providers.forEach((p) => {
      rec[p.source] = { text: `${p.name}` };
    });
    return Object.keys(rec).length ? rec : { '1688': { text: '1688采集器' } };
  }, [providers]);

  const columns: ProColumns<CollectTaskRow>[] = [
    {
      title: '采集器',
      dataIndex: 'source',
      width: 132,
      valueType: 'select',
      valueEnum: tableSourceEnum,
      renderText: (_, row) =>
        `${providers.find((p) => p.source === row.source)?.name ?? row.source}`,
    },
    {
      title: '链接关键词',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '匹配 source_url' },
      search: {
        transform: (v) => ({ keyword: v }),
      },
    },
    {
      title: '来源链接',
      dataIndex: 'sourceUrl',
      ellipsis: true,
      copyable: true,
      search: false,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 112,
      valueType: 'select',
      valueEnum: {
        ...Object.fromEntries(
          Object.entries(COLLECT_TASK_STATUS).map(([k, v]) => [k, { text: v.text }]),
        ),
        pending: { text: '等待处理（排队）' },
        running: { text: '处理中' },
      },
      render: (_, row) => {
        const m = COLLECT_TASK_STATUS[row.status as keyof typeof COLLECT_TASK_STATUS];
        return <Tag color={m?.color}>{m?.text ?? row.status}</Tag>;
      },
    },
    {
      title: '重试',
      search: false,
      width: 180,
      render: (_, row) => (
        <span>
          {row.retryCount ?? 0}/{row.maxRetries ?? '—'}
        </span>
      ),
    },
    {
      title: '下次自动重试',
      dataIndex: 'nextRetryAt',
      width: 172,
      search: false,
      render: (_, row) => formatTs(row.nextRetryAt),
    },
    {
      title: '商品草稿',
      dataIndex: 'resultProductId',
      width: 280,
      search: false,
      ellipsis: true,
      render: (_, row) =>
        row.resultProductId ? (
          <Link to={`/product/drafts/${row.resultProductId}`}>{row.resultProductId}</Link>
        ) : (
          '—'
        ),
    },
    {
      title: '失败原因',
      dataIndex: 'collectorErrorCode',
      width: 140,
      search: false,
      render: (_, row) => {
        if (row.status !== 'failed' && row.status !== 'retrying') return '—';
        return (
          mapCollectorErrorCodeLabel(row.collectorErrorCode) ||
          row.failureHint ||
          '—'
        );
      },
    },
    {
      title: '处理建议',
      dataIndex: 'failureHint',
      ellipsis: true,
      search: false,
      render: (_, row) => {
        if (row.status !== 'failed' && row.status !== 'retrying') return '—';
        const hint =
          row.failureHint ||
          mapCollectorErrorCodeDetail(row.collectorErrorCode, row.source) ||
          (row.errorMessage ? mapCollectErrorMessage(row.errorMessage, row.source) : '');
        return hint || '—';
      },
    },
    {
      title: '开始时间',
      dataIndex: 'startedAt',
      width: 168,
      search: false,
      render: (_, row) => formatTs(row.startedAt),
    },
    {
      title: '结束时间',
      dataIndex: 'finishedAt',
      width: 168,
      search: false,
      render: (_, row) => formatTs(row.finishedAt),
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 168,
      search: false,
      render: (_, row) => formatTs(row.createdAt),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 120,
      search: false,
      render: (_, row) => {
        const actions = [
          <a
            key="events"
            onClick={() => {
              setEventDrawerTaskId(row.id);
              setEventDrawerOpen(true);
            }}
          >
            事件
          </a>,
        ];
        if (row.status === 'failed') {
          actions.push(
            <a
              key="retry"
              onClick={async () => {
                try {
                  await retryCollectTask(row.id);
                  message.success('已重新入队');
                  actionRef.current?.reload();
                } catch (e) {
                  message.error(e instanceof Error ? e.message : '重试失败');
                }
              }}
            >
              重试
            </a>,
          );
        }
        return actions;
      },
    },
  ];

  return (
    <PageContainer
      title="采集任务"
      subTitle={
        batchIdFromQuery ? (
          <span>
            <Tag color="processing" style={{ marginRight: 8 }}>
              批次筛选
            </Tag>
            <Link to="/collect/tasks">清除筛选</Link>
          </span>
        ) : undefined
      }
    >
      <ProCard bordered style={{ marginBottom: 16 }} bodyStyle={{ paddingBottom: 8 }}>
        {formSource === 'custom' ? (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message="未选择规则时，系统会尝试按域名匹配启用规则。"
            description="已支持专用采集器的平台链接无法通过自定义采集提交，请在采集中心使用对应采集器。"
          />
        ) : null}
        <Form
          form={form}
          layout="inline"
          initialValues={{ source: '1688', url: '' }}
          onFinish={async (vals) => {
            const url = vals.url?.trim();
            const src = vals.source?.trim() || '';
            const p = providers.find((x) => x.source === src);
            if (!p || !providerAllowsSingleCollect(p.status)) {
              message.warning('请先选择一个可用的采集平台');
              return;
            }
            if (!url) {
              message.warning('请填写商品链接');
              return;
            }
            if (src === 'custom') {
              const rid = vals.ruleId?.trim();
              if (rid) {
                const rule = enabledRules.find((r) => r.id === rid);
                if (rule && !ruleMatchesURL(rule, url)) {
                  message.error(formatRuleDomainMismatchMessage(url, rule.domain));
                  return;
                }
              } else if (rulesForUrl.length === 0 && enabledRules.length > 0) {
                try {
                  const host = new URL(url).hostname.toLowerCase();
                  const suggested = suggestRuleDomainForHost(host);
                  message.error(
                    `当前链接与已启用规则均不匹配。链接主机名为 ${host}，请在「采集规则」将域名设为 ${suggested}`,
                  );
                } catch {
                  message.error('当前链接与已启用规则均不匹配，请检查链接或规则域名');
                }
                return;
              }
            }
            setSubmitting(true);
            try {
              const rid = vals.ruleId?.trim();
              await createCollectTask({
                source: src,
                url,
                ...(src === 'custom' ? { ruleId: rid || undefined } : {}),
                ...(isPddSource
                  ? {
                      profileId: pddUseBrowserProfile ? pddProfileId : undefined,
                      useBrowserProfile: pddUseBrowserProfile && Boolean(pddProfileId),
                    }
                  : {}),
              });
              message.success('采集任务已提交，正在后台处理');
              actionRef.current?.reload();
            } catch (e) {
              message.error(mapCollectErrorMessage(e, vals.source));
            } finally {
              setSubmitting(false);
            }
          }}
        >
          <Form.Item
            label="采集平台"
            name="source"
            rules={[{ required: true, message: '请选择采集平台' }]}
          >
            <Select style={{ width: 220 }} options={providerSelectOptions} placeholder="选择采集器" />
          </Form.Item>
          {formSource === 'custom' ? (
            <Form.Item label="规则" name="ruleId">
              <Select
                allowClear
                placeholder={
                  formUrl?.trim()
                    ? rulesForUrl.length
                      ? '可选：指定规则（已按链接过滤）'
                      : '无匹配规则，请检查链接或规则域名'
                    : '可选：指定 ruleId'
                }
                style={{ width: 320 }}
                options={(formUrl?.trim() ? rulesForUrl : enabledRules).map((r) => ({
                  label: `${r.name}（${r.domain} · p${r.priority}）`,
                  value: r.id,
                }))}
              />
            </Form.Item>
          ) : null}
          <Form.Item label="链接" name="url" rules={[{ required: true, message: '必填' }]}>
            <Input style={{ width: 480, maxWidth: '100%' }} placeholder={placeholderUrl} />
          </Form.Item>
          {isPddSource && pddUrlHint ? (
            <Alert
              type={
                pddUrlType === 'goods_detail'
                  ? 'success'
                  : pddUrlType === 'wholesale_detail'
                    ? 'warning'
                    : 'info'
              }
              showIcon
              message={pddUrlHint}
              style={{ marginBottom: 12, width: '100%' }}
            />
          ) : null}
          {isPddSource ? (
            <div style={{ width: '100%', marginBottom: 12 }}>
              <Space style={{ marginBottom: 8 }}>
                <Switch checked={pddUseBrowserProfile} onChange={setPddUseBrowserProfile} />
                <Typography.Text>使用已登录的采集浏览器</Typography.Text>
              </Space>
              {pddUseBrowserProfile ? (
                <BrowserProfileLoginPanel
                  url={formUrl?.trim() ?? ''}
                  domain={pinduoduoProfileDomain()}
                  profileProvider="pinduoduo"
                  profileId={pddProfileId}
                  useBrowserProfile={pddUseBrowserProfile}
                  tone="optional"
                  onProfileIdChange={setPddProfileId}
                  onUseProfileChange={setPddUseBrowserProfile}
                />
              ) : null}
            </div>
          ) : null}
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={submitting}>
              提交
            </Button>
          </Form.Item>
        </Form>
      </ProCard>

      <ProTable<CollectTaskRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        options={{ reload: true, density: true, setting: true }}
        polling={polling}
        headerTitle={false}
        toolBarRender={() => []}
        request={async (params) => {
          const res = await fetchCollectTasks({
            page: params.current,
            pageSize: params.pageSize,
            status: params.status as string | undefined,
            source: params.source as string | undefined,
            keyword: params.keyword as string | undefined,
            batchId: batchIdFromQuery,
          });
          return {
            data: res.list,
            success: true,
            total: res.pagination.total,
          };
        }}
      />
      <CollectTaskEventDrawer
        taskId={eventDrawerTaskId}
        open={eventDrawerOpen}
        onClose={() => {
          setEventDrawerOpen(false);
          setEventDrawerTaskId(null);
        }}
      />
    </PageContainer>
  );
}

function formatTs(s?: string) {
  if (!s) return '—';
  const d = dayjs(s);
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : s;
}
