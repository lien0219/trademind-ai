import type { ActionType, ProColumns, ProFormInstance } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { formatDateTime } from '@/utils/formatTime';
import { ProCard } from '@ant-design/pro-components';
import { useLocation } from '@umijs/max';
import { Link } from '@umijs/renderer-react';
import {
  Alert,
  Button,
  Col,
  Form,
  Input,
  Row,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
  message,
} from 'antd';
import { useEffect, useMemo, useRef, useState } from 'react';
import { COLLECT_TASK_STATUS } from '@/constants/status';
import { COLLECT_SUCCESS_SHOP_HINT, COLLECT_TARGET_SHOP_HINT } from '@/constants/copywriting';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
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
import { PinduoduoLoginPanel } from '@/pages/Collect/components/PinduoduoLoginPanel';
import { classifyPinduoduoUrl, pinduoduoUrlHint } from '@/utils/pinduoduoUrl';

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

  const statusFromQuery = useMemo(() => {
    const q = new URLSearchParams(location.search || '');
    return q.get('status')?.trim() || undefined;
  }, [location.search]);

  const actionRef = useRef<ActionType>();
  const emptyLocale = useListEmptyLocale('collectTasks');
  const formRef = useRef<ProFormInstance>();
  const [form] = Form.useForm<{ source: string; url: string; ruleId?: string }>();
  const [submitting, setSubmitting] = useState(false);
  const [polling, setPolling] = useState<number | undefined>(4000);
  const [eventDrawerOpen, setEventDrawerOpen] = useState(false);
  const [eventDrawerTaskId, setEventDrawerTaskId] = useState<string | null>(null);

  const [providers, setProviders] = useState<CollectProviderRow[]>([]);
  const [enabledRules, setEnabledRules] = useState<CollectRuleRow[]>([]);
  const formSource = Form.useWatch('source', form);
  const formUrl = Form.useWatch('url', form);
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
    if (!statusFromQuery) return;
    formRef.current?.setFieldsValue?.({ status: statusFromQuery });
    actionRef.current?.reload?.();
  }, [statusFromQuery]);

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
      title: '采集服务',
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
      width: 240,
      search: false,
      responsive: ['md'],
      render: (_, row) => {
        const url = row.sourceUrl?.trim();
        if (!url) return '—';
        return (
          <div style={{ display: 'flex', alignItems: 'center', gap: 4, width: '100%', maxWidth: 220 }}>
            <Typography.Text ellipsis style={{ flex: 1, minWidth: 0 }}>
              {url}
            </Typography.Text>
            <Typography.Text
              copyable={{ text: url, tooltips: false }}
              style={{ flexShrink: 0, margin: 0 }}
            />
          </div>
        );
      },
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
      width: 100,
      responsive: ['lg'],
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
      responsive: ['lg'],
      render: (_, row) => formatDateTime(row.nextRetryAt),
    },
    {
      title: '商品草稿',
      dataIndex: 'resultProductId',
      width: 200,
      search: false,
      ellipsis: true,
      responsive: ['md'],
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
      responsive: ['md'],
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
      responsive: ['xl'],
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
      responsive: ['xl'],
      render: (_, row) => formatDateTime(row.startedAt),
    },
    {
      title: '结束时间',
      dataIndex: 'finishedAt',
      width: 168,
      search: false,
      responsive: ['xxl'],
      render: (_, row) => formatDateTime(row.finishedAt),
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 168,
      search: false,
      responsive: ['lg'],
      render: (_, row) => formatDateTime(row.createdAt),
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
    <TmPageContainer
      title="采集任务"
      subTitle={
        batchIdFromQuery ? (
          <span>
            <Tag color="processing" style={{ marginRight: 8 }}>
              批次筛选
            </Tag>
            <Link to="/collect/tasks">清除筛选</Link>
          </span>
        ) : (
          '提交单条商品链接采集任务，查看执行进度与结果。'
        )
      }
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="店铺归属与权限提示"
        description={COLLECT_TARGET_SHOP_HINT}
      />
      <ProCard variant="outlined" style={{ marginBottom: 16 }} bodyStyle={{ paddingBottom: 8 }}>
        {formSource === 'custom' ? (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message="未选择规则时，系统会尝试按域名匹配启用规则。"
            description="已支持专用采集服务的平台链接无法通过自定义采集提交，请在采集中心使用对应采集方式。"
          />
        ) : null}
        <Form
          form={form}
          layout="vertical"
          className="tm-collect-task-form"
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
                      useBrowserProfile:
                        pddUseBrowserProfile || pddUrlType === 'wholesale_detail',
                    }
                  : {}),
              });
              message.success(COLLECT_SUCCESS_SHOP_HINT, 6);
              actionRef.current?.reload();
            } catch (e) {
              message.error(mapCollectErrorMessage(e, vals.source));
            } finally {
              setSubmitting(false);
            }
          }}
        >
          <Row gutter={[16, 0]} align="bottom">
            <Col xs={24} sm={12} md={8} lg={6}>
              <Form.Item
                label="采集平台"
                name="source"
                rules={[{ required: true, message: '请选择采集平台' }]}
              >
                <Select options={providerSelectOptions} placeholder="选择采集服务" />
              </Form.Item>
            </Col>
            {formSource === 'custom' ? (
              <Col xs={24} sm={12} md={8} lg={8}>
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
                    options={(formUrl?.trim() ? rulesForUrl : enabledRules).map((r) => ({
                      label: `${r.name}（${r.domain} · p${r.priority}）`,
                      value: r.id,
                    }))}
                  />
                </Form.Item>
              </Col>
            ) : null}
            <Col xs={24} md={16} lg={10} xl={12}>
              <Form.Item label="链接" name="url" rules={[{ required: true, message: '必填' }]}>
                <Input placeholder={placeholderUrl} />
              </Form.Item>
            </Col>
            <Col xs={24} sm="auto">
              <Form.Item label=" ">
                <Button type="primary" htmlType="submit" loading={submitting}>
                  提交
                </Button>
              </Form.Item>
            </Col>
          </Row>
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
              style={{ marginTop: 4, marginBottom: 0 }}
            />
          ) : null}
          {isPddSource ? (
            <div style={{ marginTop: 16, marginBottom: 12 }}>
              <PinduoduoLoginPanel loginUrl={formUrl?.trim() || undefined} compact />
              <Space style={{ marginTop: 16 }} wrap>
                <Switch
                  checked={pddUseBrowserProfile || pddUrlType === 'wholesale_detail'}
                  disabled={pddUrlType === 'wholesale_detail'}
                  onChange={setPddUseBrowserProfile}
                />
                <Typography.Text>使用已登录的采集浏览器</Typography.Text>
              </Space>
            </div>
          ) : null}
        </Form>
      </ProCard>

      <ProTable<CollectTaskRow>
        rowKey="id"
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        options={{ reload: true, density: true, setting: true }}
        polling={polling}
        headerTitle={false}
        toolBarRender={() => []}
        locale={emptyLocale}
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
    </TmPageContainer>
  );
}

