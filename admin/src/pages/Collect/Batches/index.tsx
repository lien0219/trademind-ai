import { useLanguage } from '@/i18n/useLanguage';
import { tr, trFormat } from '@/i18n';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails, TaskJsonBlock, TmProTable as ProTable } from '@/components/ui';
import { formatDateTime } from '@/utils/formatTime';
import { ProCard } from '@ant-design/pro-components';
import { history, useLocation } from '@umijs/max';
import { Link } from '@umijs/renderer-react';
import { Button, Drawer, Form, Input, message, Space, Tag, Alert, Select, Typography } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { COLLECT_BATCH_STATUS, COLLECT_TASK_STATUS } from '@/constants/status';
import { CollectTaskEventDrawer } from '@/pages/Collect/components/CollectTaskEventDrawer';
import {
  createCollectBatch,
  getCollectBatch,
  queryCollectBatches,
  queryCollectBatchTasks,
  retryFailedBatchTasks,
  type CollectBatchRow,
} from '@/services/collectBatches';
import type { CollectProviderRow } from '@/services/collectProviders';
import { queryCollectProviders } from '@/services/collectProviders';
import { retryCollectTask, type CollectTaskRow } from '@/services/collectTasks';
import { checkTaobaoTmallLogin } from '@/services/collectAuth';
import {
  parseTaobaoTmallBatchUrls,
  TAOBAO_TMALL_BATCH_HINT,
  TAOBAO_TMALL_BATCH_LOGIN_BLOCK_MSG,
  TAOBAO_TMALL_BATCH_MAX_ITEMS,
  TAOBAO_TMALL_BATCH_VERIFY_BLOCK_MSG,
} from '@/utils/taobaoTmallUrl';

const POLL_MS = 5000;

function isTaobaoTmallSource(source?: string) {
  const s = source?.trim().toLowerCase();
  return s === 'taobao_tmall' || s === 'taobao';
}

function countDedupedLines(raw: string): number {
  const seen = new Set<string>();
  for (const line of raw.split(/\n/)) {
    const u = line.trim();
    if (!u) continue;
    seen.add(u.toLowerCase());
  }
  return seen.size;
}

function parseUrlsFromTextarea(raw: string): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const line of raw.split(/\n/)) {
    const u = line.trim();
    if (!u) continue;
    const k = u.toLowerCase();
    if (seen.has(k)) continue;
    seen.add(k);
    out.push(u);
  }
  return out;
}

export default function CollectBatchesPage() {
  const language = useLanguage();
  const location = useLocation();
  const actionRef = useRef<ActionType>();
  const taskActionRef = useRef<ActionType>();
  const [form] = Form.useForm<{ source: string; urls: string }>();
  const [submitting, setSubmitting] = useState(false);
  const [polling, setPolling] = useState<number | undefined>(POLL_MS);
  const [taskPolling, setTaskPolling] = useState<number | undefined>(undefined);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [activeBatch, setActiveBatch] = useState<CollectBatchRow | null>(null);
  const [eventDrawerOpen, setEventDrawerOpen] = useState(false);
  const [eventDrawerTaskId, setEventDrawerTaskId] = useState<string | null>(null);
  const [providers, setProviders] = useState<CollectProviderRow[]>([]);
  const urlsWatch = Form.useWatch('urls', form) as string | undefined;
  const formSourceWatch = Form.useWatch('source', form) as string | undefined;

  const batchProviders = useMemo(
    () => providers.filter((p) => p.batchSupported && p.status === 'available'),
    [providers],
  );

  const sourceFromQuery = useMemo(() => {
    const q = new URLSearchParams(location.search || '');
    return q.get('source')?.trim() ?? '';
  }, [location.search]);

  const batchMaxItems = useMemo(() => {
    if (isTaobaoTmallSource(formSourceWatch)) return TAOBAO_TMALL_BATCH_MAX_ITEMS;
    return 50;
  }, [formSourceWatch]);

  const taobaoUrlPreview = useMemo(() => {
    if (!isTaobaoTmallSource(formSourceWatch)) return null;
    return parseTaobaoTmallBatchUrls(urlsWatch ?? '');
  }, [formSourceWatch, urlsWatch]);

  const displayCount = useMemo(() => {
    if (taobaoUrlPreview) return taobaoUrlPreview.valid.length;
    return countDedupedLines(urlsWatch ?? '');
  }, [taobaoUrlPreview, urlsWatch]);

  useEffect(() => {
    const sync = () => setPolling(document.visibilityState === 'hidden' ? undefined : POLL_MS);
    sync();
    document.addEventListener('visibilitychange', sync);
    return () => document.removeEventListener('visibilitychange', sync);
  }, []);

  useEffect(() => {
    const sync = () =>
      setTaskPolling(document.visibilityState === 'hidden' || !drawerOpen ? undefined : POLL_MS);
    sync();
    document.addEventListener('visibilitychange', sync);
    return () => document.removeEventListener('visibilitychange', sync);
  }, [drawerOpen]);

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
    if (!batchProviders.length) return;
    const qs = sourceFromQuery;
    const picked =
      qs && batchProviders.some((p) => p.source === qs)
        ? qs
        : batchProviders.find((p) => p.source === '1688')?.source ?? batchProviders[0]?.source;
    if (!picked) return;
    form.setFieldsValue({
      source: picked,
      urls: form.getFieldValue('urls') ?? '',
    });
  }, [batchProviders, sourceFromQuery, form]);

  const batchSourcePlaceholder = useMemo(() => {
    const p = batchProviders.find((x) => x.source === formSourceWatch);
    const line =
      (p?.urlPatterns?.length ? p.urlPatterns[0] : undefined) ??
      'https://detail.1688.com/offer/...';
    return `${line}\n${line}`;
  }, [batchProviders, formSourceWatch]);

  const batchTableSourceEnum = useMemo(() => {
    const rec: Record<string, { text: string }> = {};
    providers.forEach((p) => {
      rec[p.source] = { text: `${p.name}` };
    });
    return Object.keys(rec).length ? rec : { '1688': { text: tr("1688采集器") } };
  }, [providers, language]);

  const batchSelectOpts = useMemo(
    () => batchProviders.map((p) => ({ label: `${p.name}`, value: p.source })),
    [batchProviders],
  );

  const openDrawer = useCallback((row: CollectBatchRow) => {
    setActiveBatch(row);
    setDrawerOpen(true);
  }, []);

  const closeDrawer = () => {
    setDrawerOpen(false);
    setActiveBatch(null);
  };

  useEffect(() => {
    const q = new URLSearchParams(location.search || '');
    const bid = q.get('batchId')?.trim();
    if (!bid) return;
    let cancelled = false;
    void (async () => {
      try {
        const row = await getCollectBatch(bid);
        if (cancelled) return;
        openDrawer(row);
      } catch {
        message.error(tr("批次不存在或暂不可用"));
      } finally {
        if (!cancelled) {
          q.delete('batchId');
          const qs = q.toString();
          history.replace(`${location.pathname}${qs ? `?${qs}` : ''}`);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [location.search, location.pathname, openDrawer]);

  const batchColumns: ProColumns<CollectBatchRow>[] = useMemo(
    () => [
      {
        title: tr("创建时间"),
        dataIndex: 'createdAt',
        width: 168,
        search: false,
        render: (_, row) => formatDateTime(row.createdAt),
      },
      {
        title: tr("采集服务"),
        dataIndex: 'source',
        width: 120,
        valueType: 'select',
        valueEnum: batchTableSourceEnum,
        renderText: (_, row) => providers.find((p) => p.source === row.source)?.name ?? row.source,
      },
    {
      title: tr("状态"),
      dataIndex: 'status',
      width: 112,
      valueType: 'select',
      valueEnum: Object.fromEntries(
        Object.entries(COLLECT_BATCH_STATUS).map(([k, v]) => [k, { text: v.text }]),
      ),
      render: (_, row) => {
        const m = COLLECT_BATCH_STATUS[row.status as keyof typeof COLLECT_BATCH_STATUS];
        return <Tag color={m?.color}>{m?.text ?? row.status}</Tag>;
      },
    },
    {
      title: tr("总数"),
      dataIndex: 'totalCount',
      width: 64,
      search: false,
    },
    {
      title: tr("排队"),
      dataIndex: 'pendingCount',
      width: 64,
      search: false,
    },
    {
      title: tr("执行中"),
      dataIndex: 'runningCount',
      width: 76,
      search: false,
    },
    {
      title: tr("成功"),
      dataIndex: 'successCount',
      width: 64,
      search: false,
    },
    {
      title: tr("失败"),
      dataIndex: 'failedCount',
      width: 64,
      search: false,
    },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 200,
      search: false,
      render: (_, row) => [
        <Button key="view" type="link" size="small" onClick={() => openDrawer(row)}>
          {tr("查看任务")}</Button>,
        <Button
          key="retry"
          type="link"
          size="small"
          disabled={row.failedCount <= 0}
          onClick={async () => {
            if (row.failedCount <= 0) return;
            try {
              const r = await retryFailedBatchTasks(row.id);
              message.success(trFormat("已重新入队 {retried} 条失败任务", { retried: r.retried }));
              actionRef.current?.reload();
            } catch (e) {
              message.error(e instanceof Error ? e.message : tr("重试失败"));
            }
          }}
        >
          {tr("重试失败")}</Button>,
      ],
    },
  ],
    [batchTableSourceEnum, language, openDrawer, providers],
  );

  const taskColumns: ProColumns<CollectTaskRow>[] = [
    {
      title: tr("商品标题"),
      width: 160,
      ellipsis: true,
      search: false,
      render: (_, row) => {
        const raw = row.rawResult as { title?: string } | undefined;
        return raw?.title?.trim() || '—';
      },
    },
    {
      title: tr("链接"),
      dataIndex: 'sourceUrl',
      width: 220,
      ellipsis: true,
      copyable: true,
      fixed: 'left',
      search: false,
    },
    {
      title: tr("状态"),
      dataIndex: 'status',
      width: 108,
      valueType: 'select',
      valueEnum: {
        pending: { text: tr("排队") },
        running: { text: tr("执行中") },
        retrying: { text: tr("等待重试") },
        success: { text: tr("成功") },
        failed: { text: tr("失败") },
        cancelled: { text: tr("取消") },
      },
      render: (_, row) => {
        const m = COLLECT_TASK_STATUS[row.status as keyof typeof COLLECT_TASK_STATUS];
        return <Tag color={m?.color}>{m?.text ?? row.status}</Tag>;
      },
    },
    {
      title: tr("重试"),
      width: 88,
      search: false,
      render: (_, row) => (
        <span>
          {row.retryCount ?? 0}/{row.maxRetries ?? '—'}
        </span>
      ),
    },
    {
      title: tr("下次重试"),
      dataIndex: 'nextRetryAt',
      width: 156,
      search: false,
      render: (_, row) => formatDateTime(row.nextRetryAt),
    },
    {
      title: tr("商品草稿"),
      dataIndex: 'resultProductId',
      width: 120,
      ellipsis: true,
      search: false,
      render: (_, row) =>
        row.resultProductId ? (
          <Link to={`/product/drafts/${row.resultProductId}`} title={row.resultProductId}>
            {row.resultProductId.slice(0, 8)}…
          </Link>
        ) : (
          '—'
        ),
    },
    {
      title: tr("错误码"),
      dataIndex: 'collectorErrorCode',
      width: 176,
      ellipsis: true,
      search: false,
      render: (_, row) => row.collectorErrorCode || '—',
    },
    {
      title: tr("可重试"),
      width: 72,
      search: false,
      render: (_, row) =>
        row.retryable === undefined ? '—' : row.retryable ? <Tag color="warning">{tr("是")}</Tag> : <Tag>{tr("否")}</Tag>,
    },
    {
      title: tr("错误"),
      dataIndex: 'errorMessage',
      width: 200,
      ellipsis: true,
      search: false,
      render: (_, row) => {
        const text = [row.errorMessage, row.failureHint].filter(Boolean).join(' · ');
        if (!text) return '—';
        return (
          <Typography.Text ellipsis={{ tooltip: text }} style={{ maxWidth: 188 }}>
            {text}
          </Typography.Text>
        );
      },
    },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 112,
      fixed: 'right',
      search: false,
      render: (_, row) => {
        const actions = [
          <Button
            key="ev"
            type="link"
            size="small"
            onClick={() => {
              setEventDrawerTaskId(row.id);
              setEventDrawerOpen(true);
            }}
          >
            {tr("事件")}</Button>,
        ];
        if (row.status === 'failed') {
          actions.push(
            <Button
              key="r1"
              type="link"
              size="small"
              onClick={async () => {
                try {
                  await retryCollectTask(row.id);
                  message.success(tr("已重新入队"));
                  taskActionRef.current?.reload();
                  actionRef.current?.reload();
                } catch (e) {
                  message.error(e instanceof Error ? e.message : tr("重试失败"));
                }
              }}
            >
              {tr("重试")}</Button>,
          );
        }
        if (row.sourceUrl) {
          actions.unshift(
            <Button
              key="open"
              type="link"
              size="small"
              href={row.sourceUrl}
              target="_blank"
              rel="noreferrer"
            >
              {tr("原链接")}</Button>,
          );
        }
        return actions;
      },
    },
  ];

  return (
    <TmPageContainer
      title={tr("批量采集")}
      subTitle={tr("一次提交多条商品链接，批量采集并生成商品草稿。")}
    >
      <ProCard variant="outlined" style={{ marginBottom: 16 }} bodyStyle={{ paddingBottom: 8 }}>
        {sourceFromQuery &&
        providers.length > 0 &&
        !batchProviders.some((p) => p.source === sourceFromQuery) ? (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message={tr("所选采集服务暂不支持批量采集，已切换到当前可用的批量平台。")}
          />
        ) : null}
        {batchProviders.length === 0 && providers.length > 0 ? (
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 16 }}
            message={tr("当前没有支持批量采集的可用平台。")}
          />
        ) : null}
        <Form
          form={form}
          layout="vertical"
          initialValues={{ source: '1688', urls: '' }}
          onFinish={async (vals) => {
            const src = vals.source?.trim() || '';
            const allowed = batchProviders.some((p) => p.source === src);
            if (!allowed) {
              message.warning(tr("该平台暂不支持批量采集"));
              return;
            }

            let urls: string[] = [];
            if (isTaobaoTmallSource(src)) {
              const parsed = parseTaobaoTmallBatchUrls(vals.urls ?? '');
              urls = parsed.valid;
              if (parsed.invalid.length > 0 || parsed.unsupported.length > 0) {
                const skipped = [...parsed.invalid, ...parsed.unsupported];
                message.warning(
                  trFormat("已跳过 {count} 条无效或不支持的链接，仅提交 {count1} 条有效商品详情页链接", { count: skipped.length, count1: urls.length }),
                );
              }
              if (urls.length === 0) {
                message.warning(tr("请至少填写一条有效的淘宝/天猫商品详情页链接"));
                return;
              }
              if (urls.length > batchMaxItems) {
                message.warning(trFormat("每批最多 {batchMaxItems} 条，请分批提交", { batchMaxItems: batchMaxItems }));
                return;
              }
              setSubmitting(true);
              try {
                const auth = await checkTaobaoTmallLogin({ url: urls[0] });
                if (auth.needVerification || auth.status === 'verify_required') {
                  message.error(TAOBAO_TMALL_BATCH_VERIFY_BLOCK_MSG);
                  return;
                }
                if (!auth.loggedIn && auth.status !== 'logged_in') {
                  message.error(TAOBAO_TMALL_BATCH_LOGIN_BLOCK_MSG);
                  return;
                }
              } catch (e) {
                message.error(e instanceof Error ? e.message : tr("登录状态检测失败"));
                return;
              } finally {
                setSubmitting(false);
              }
            } else {
              urls = parseUrlsFromTextarea(vals.urls ?? '');
              if (urls.length === 0) {
                message.warning(tr("请至少填写一条链接"));
                return;
              }
              if (urls.length > batchMaxItems) {
                message.warning(trFormat("每批最多 {batchMaxItems} 条，请分批提交", { batchMaxItems: batchMaxItems }));
                return;
              }
            }

            setSubmitting(true);
            try {
              const res = await createCollectBatch({ source: src, urls });
              const skipped = res.skippedCount ?? 0;
              message.success(
                skipped > 0
                  ? trFormat("批量采集已提交：有效 {taskCount} 条，跳过 {skipped} 条无效链接", { taskCount: res.taskCount, skipped: skipped })
                  : trFormat("批量采集任务已提交，共 {taskCount} 条，正在后台处理", { taskCount: res.taskCount }),
              );
              form.setFieldsValue({ urls: '' });
              actionRef.current?.reload();
            } catch (e) {
              const msg = e instanceof Error ? e.message : tr("提交失败");
              if (msg.includes('登录')) {
                message.error(TAOBAO_TMALL_BATCH_LOGIN_BLOCK_MSG);
              } else if (msg.includes('安全验证') || msg.includes('验证')) {
                message.error(TAOBAO_TMALL_BATCH_VERIFY_BLOCK_MSG);
              } else {
                message.error(msg);
              }
            } finally {
              setSubmitting(false);
            }
          }}
        >
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            {isTaobaoTmallSource(formSourceWatch) ? (
              <Alert type="info" showIcon message={TAOBAO_TMALL_BATCH_HINT} />
            ) : null}
            {taobaoUrlPreview &&
            (taobaoUrlPreview.invalid.length > 0 || taobaoUrlPreview.unsupported.length > 0) ? (
              <Alert
                type="warning"
                showIcon
                message={trFormat("检测到 {count} 条无效或不支持的链接，提交时将自动跳过", { count: taobaoUrlPreview.invalid.length + taobaoUrlPreview.unsupported.length })}
              />
            ) : null}
            <Form.Item label={tr("采集平台")} name="source" rules={[{ required: true }]}>
              <Select
                style={{ maxWidth: 320 }}
                options={batchSelectOpts}
                placeholder={tr("选择批量采集平台")}
              />
            </Form.Item>
            <Form.Item
              label={
                <span>
                  {tr("商品链接（每行一条）")}<Tag>{tr("有效")}{displayCount} {tr("条")}</Tag>
                  {isTaobaoTmallSource(formSourceWatch) ? (
                    <Tag color="blue">{tr("每批最多")}{batchMaxItems} {tr("条")}</Tag>
                  ) : null}
                </span>
              }
              name="urls"
              rules={[{ required: true, message: tr("请填写链接") }]}
            >
              <Input.TextArea
                rows={12}
                placeholder={batchSourcePlaceholder}
                style={{ fontFamily: 'monospace' }}
              />
            </Form.Item>
            <Form.Item>
              <Button
                type="primary"
                htmlType="submit"
                loading={submitting}
                disabled={batchSelectOpts.length === 0}
              >
                {tr("提交批量采集")}</Button>
            </Form.Item>
          </Space>
        </Form>
      </ProCard>

      <ProTable<CollectBatchRow>
        rowKey="id"
        actionRef={actionRef}
        columns={batchColumns}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        options={{ reload: true, density: true, setting: true }}
        polling={polling}
        headerTitle={tr("批次列表")}
        request={async (params) => {
          const res = await queryCollectBatches({
            page: params.current,
            pageSize: params.pageSize,
            status: params.status as string | undefined,
            source: params.source as string | undefined,
          });
          return {
            data: res.list,
            success: true,
            total: res.pagination.total,
          };
        }}
      />

      <Drawer
        title={activeBatch ? trFormat("批次 · {source} · {value}…", { source: activeBatch.source, value: activeBatch.id.slice(0, 8) }) : tr("批次任务")}
        placement="right"
        width={1100}
        open={drawerOpen}
        onClose={closeDrawer}
        destroyOnHidden
        styles={{ body: { paddingTop: 16, overflow: 'hidden' } }}
      >
        {activeBatch && (
          <>
            {activeBatch.failedCount > 0 && activeBatch.successCount > 0 ? (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 16 }}
                message={tr("本批次部分链接失败（部分成功）")}
                description={
                  isTaobaoTmallSource(activeBatch.source)
                    ? tr("成功的链接已生成商品草稿；失败链接可在下方重试，或到失败任务中心处理。若出现登录/验证失败，请先在采集浏览器完成后再重试。")
                    : tr("若同一链接单独采集成功，批量失败通常与并发、访问频率或目标站点风控有关。可在「采集设置」降低批量并发，或稍后重试失败任务。")
                }
              />
            ) : null}
            {activeBatch.finishedAt && activeBatch.createdAt ? (
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 16 }}
                message={tr("批次汇总")}
                description={
                  <Space wrap>
                    <span>{tr("总链接")}{activeBatch.totalCount}</span>
                    <span>{tr("成功")}{activeBatch.successCount}</span>
                    <span>{tr("失败")}{activeBatch.failedCount}</span>
                    <span>{tr("取消")}{activeBatch.cancelledCount}</span>
                    <span>
                      {tr("耗时")}{' '}
                      {Math.max(
                        0,
                        Math.round(
                          (new Date(activeBatch.finishedAt).getTime() -
                            new Date(activeBatch.createdAt).getTime()) /
                            1000,
                        ),
                      )}
                      {tr("秒")}</span>
                  </Space>
                }
              />
            ) : null}
            <ProCard variant="outlined" size="small" style={{ marginBottom: 16 }} bodyStyle={{ padding: '12px 16px' }}>
              <Space wrap size="middle">
                <Tag>{tr("总数")}{activeBatch.totalCount}</Tag>
                <Tag>{tr("排队")}{activeBatch.pendingCount}</Tag>
                <Tag>{tr("执行中")}{activeBatch.runningCount}</Tag>
                <Tag color="processing">{tr("重试中")}{activeBatch.retryingCount ?? 0}</Tag>
                <Tag color="success">{tr("成功")}{activeBatch.successCount}</Tag>
                <Tag color="error">{tr("失败")}{activeBatch.failedCount}</Tag>
                <Tag>{tr("取消")}{activeBatch.cancelledCount}</Tag>
                {(activeBatch.blockedCount ?? 0) > 0 ? (
                  <Tag color="orange">{tr("风控")}{activeBatch.blockedCount}</Tag>
                ) : null}
                {(activeBatch.timeoutCount ?? 0) > 0 ? (
                  <Tag color="volcano">{tr("超时")}{activeBatch.timeoutCount}</Tag>
                ) : null}
                {(activeBatch.parseFailedCount ?? 0) > 0 ? (
                  <Tag color="magenta">{tr("解析失败")}{activeBatch.parseFailedCount}</Tag>
                ) : null}
              </Space>
              {activeBatch.errorSummary && Object.keys(activeBatch.errorSummary).length > 0 ? (
                <TechnicalDetails label={tr("失败统计详情")}>
                  <TaskJsonBlock title={tr("按错误类型汇总")} value={activeBatch.errorSummary} last />
                </TechnicalDetails>
              ) : null}
            </ProCard>

            <ProTable<CollectTaskRow>
              rowKey="id"
              actionRef={taskActionRef}
              columns={taskColumns}
              search={{ filterType: 'light' }}
              pagination={{ defaultPageSize: 20, showSizeChanger: true }}
              options={{ reload: true, density: true, setting: true }}
              polling={taskPolling}
              headerTitle={false}
              toolBarRender={() => []}
              scroll={{ x: 1252 }}
              tableStyle={{ minWidth: '100%' }}
              style={{ width: '100%' }}
              request={async (params) => {
                const res = await queryCollectBatchTasks(activeBatch.id, {
                  page: params.current,
                  pageSize: params.pageSize,
                  status: params.status as string | undefined,
                });
                try {
                  const snap = await getCollectBatch(activeBatch.id);
                  const bid = activeBatch.id;
                  setActiveBatch((cur) => (cur && cur.id === bid ? { ...snap } : cur));
                } catch {
                  // ignore stale header refresh
                }
                return {
                  data: res.list,
                  success: true,
                  total: res.pagination.total,
                };
              }}
            />
          </>
        )}
      </Drawer>

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
