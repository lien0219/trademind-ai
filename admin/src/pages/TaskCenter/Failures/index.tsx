import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { ProCard, type ActionType, type ProColumns, type ProFormInstance } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails, TaskJsonBlock, TmProTable as ProTable } from '@/components/ui';
import { history, useLocation } from '@umijs/max';
import { confirmFailureTaskRetry } from '@/constants/sensitiveActions';
import { formatDateTime } from '@/utils/formatTime';
import {
  Badge,
  Button,
  Drawer,
  Divider,
  Dropdown,
  Input,
  Modal,
  Row,
  Col,
  Space,
  Statistic,
  Switch,
  Tag,
  Typography,
  Alert,
  message,
} from 'antd';
import dayjs from 'dayjs';
import { useEffect, useMemo, useRef, useState } from 'react';
import {
  batchHandleTaskFailures,
  batchIgnoreTaskFailures,
  batchRetryTaskFailures,
  generateTaskFailureAlert,
  getTaskFailureDetail,
  handleTaskFailure,
  ignoreTaskFailure,
  queryTaskFailureCategories,
  queryTaskFailures,
  recoverDouyinDraftTask,
  retryTaskFailure,
  unmarkTaskFailure,
  type FailureDetailDTO,
  type UnifiedTaskDTO,
  type FailuresSummary,
} from '@/services/taskCenter';
import { PAGE_COPY } from '@/constants/copywriting';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import {
  canOpenFailureDetail,
  isPlatformAlertTaskType,
  TASK_CENTER_TASK_TYPE_LABEL,
  TASK_FAILURE_CATEGORY_LABEL,
  TASK_FAILURE_SEVERITY,
  TASK_FAILURE_SEVERITY_OPTIONS,
  TASK_NORMALIZED_STATUS,
  TASK_RECOVERY_STATUS_OPTIONS,
  failureCategoryLabel,
  recoveryStatusLabel,
} from '@/constants/taskCenter';
import { openPinduoduoLoginBrowser, openTaobaoTmallLoginBrowser } from '@/services/collectAuth';
import { resolvePinduoduoLoginTargetUrl } from '@/utils/pinduoduoUrl';
import { resolveTaobaoTmallLoginTargetUrl } from '@/utils/taobaoTmallUrl';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { useKeywordSearchField } from '@/hooks/useKeywordSearchField';
import KeywordSafetyHint from '@/components/common/KeywordSafetyHint';
import { appendSourceToUrl } from '@/utils/urlState';

const FAILURE_QUERY_KEYS = [
  'page',
  'pageSize',
  'taskType',
  'normalizedStatus',
  'failureCategory',
  'recoveryStatus',
  'severity',
  'platform',
  'shopId',
  'keyword',
  'start',
  'end',
  'includeResolved',
  'includeMarked',
  'drawer',
  'id',
  'detailTaskType',
  'source',
  'jumpId',
] as const;

function parsePositiveInt(value?: string, fallback = 1) {
  const n = Number(value);
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : fallback;
}

function queryTimeRange(start?: string, end?: string) {
  if (!start || !end) return undefined;
  const s = dayjs(start);
  const e = dayjs(end);
  if (!s.isValid() || !e.isValid()) return undefined;
  return [s, e];
}

function normTag(norm: string) {
  const m = TASK_NORMALIZED_STATUS[norm];
  if (!m) return <Tag>{norm}</Tag>;
  return <Tag color={m.color}>{m.text}</Tag>;
}

function severityCell(sev?: string) {
  if (!sev) return '—';
  const key = sev.trim().toLowerCase();
  const m = TASK_FAILURE_SEVERITY[key];
  if (!m) return <Tag>{sev}</Tag>;
  const strong = key === 'critical' || key === 'high';
  return (
    <Tag color={m.color} style={{ fontWeight: strong ? 700 : undefined }}>
      {m.text}
    </Tag>
  );
}

const ALERT_ST_META: Record<string, { color: string; text: string }> = {
  none: { color: 'default', get text() { return tr("无"); } },
  generated: { color: 'gold', get text() { return tr("告警中"); } },
  handled: { color: 'green', get text() { return tr("告警已处理"); } },
  ignored: { color: 'default', get text() { return tr("告警忽略"); } },
};

function alertStatusUi(st?: string) {
  const m = ALERT_ST_META[st || 'none'];
  if (!m) return <Tag>{st}</Tag>;
  return <Tag color={m.color}>{m.text}</Tag>;
}

function relatedHref(row: UnifiedTaskDTO): string | undefined {
  const detail = (row.detailUrl || '').trim();
  if (detail.includes('/product/publish-batches/') || detail.includes('/product/ai-text-batches/') || detail.includes('/product/ai-image-batches/')) {
    return detail;
  }
  if (detail.includes('/orders/sync-tasks')) {
    return detail;
  }
  const t = row.relatedResourceType || '';
  const id = row.relatedResourceId || '';
  if (!id) return undefined;
  if (t === 'product') return `/product/drafts/${id}`;
  return undefined;
}

function detailLinkLabel(detailUrl?: string | null): string {
  const url = (detailUrl || '').trim();
  if (url.includes('/product/publish-batches/')) {
    return tr('批次详情');
  }
  if (url.includes('/product/ai-text-batches/') || url.includes('/product/ai-image-batches/')) {
    return tr('复核工作台');
  }
  if (url.includes('/orders/sync-tasks')) {
    return tr('订单同步任务');
  }
  return tr('打开任务详情');
}

export default function TaskCenterFailuresPage() {
  const language = useLanguage();
  const emptyLocale = useListEmptyLocale('taskFailures');
  const location = useLocation();
  const { state: urlState, setState: setUrlState, clearState: clearUrlState } =
    useUrlQueryState<Record<(typeof FAILURE_QUERY_KEYS)[number], string | undefined>>(
      FAILURE_QUERY_KEYS,
    );
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const listFilterRef = useRef({ includeResolved: false, includeMarked: false });
  const [includeResolved, setIncludeResolved] = useState(false);
  const [includeMarked, setIncludeMarked] = useState(false);
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(20);
  const {
    fieldProps: keywordFieldProps,
    prepareKeyword,
    showSensitiveHint,
  } = useKeywordSearchField({
    setUrlState,
    formRef,
    actionRef,
    setTablePage,
  });
  const [failureCatOpts, setFailureCatOpts] = useState<{ label: string; value: string }[]>([]);
  const [summary, setSummary] = useState<FailuresSummary | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState<FailureDetailDTO | null>(null);
  const [sel, setSel] = useState<UnifiedTaskDTO[]>([]);
  const [pddLoginOpening, setPddLoginOpening] = useState(false);
  const [tbLoginOpening, setTbLoginOpening] = useState(false);
  const [recovering, setRecovering] = useState(false);

  useEffect(() => {
    const nextIncludeResolved = urlState.includeResolved === 'true';
    const nextIncludeMarked = urlState.includeMarked === 'true';
    listFilterRef.current = {
      includeResolved: nextIncludeResolved,
      includeMarked: nextIncludeMarked,
    };
    setIncludeResolved(nextIncludeResolved);
    setIncludeMarked(nextIncludeMarked);
    setTablePage(parsePositiveInt(urlState.page, 1));
    setTablePageSize(parsePositiveInt(urlState.pageSize, 20));
    formRef.current?.setFieldsValue?.({
      taskType: urlState.taskType,
      normalizedStatus: urlState.normalizedStatus,
      failureCategory: urlState.failureCategory,
      recoveryStatus: urlState.recoveryStatus,
      severity: urlState.severity,
      platform: urlState.platform,
      shopId: urlState.shopId,
      keyword: urlState.keyword,
      timeRange: queryTimeRange(urlState.start, urlState.end),
    });
  }, [
    urlState.end,
    urlState.failureCategory,
    urlState.includeMarked,
    urlState.includeResolved,
    urlState.keyword,
    urlState.normalizedStatus,
    urlState.page,
    urlState.pageSize,
    urlState.platform,
    urlState.recoveryStatus,
    urlState.severity,
    urlState.shopId,
    urlState.start,
    urlState.taskType,
  ]);

  const isTbLoginFailure = (row: UnifiedTaskDTO | FailureDetailDTO | null) => {
    if (!row || row.taskType !== 'collect') return false;
    const pl = (row.platform ?? '').toLowerCase();
    if (pl !== 'taobao_tmall' && pl !== 'taobao') return false;
    const code = (row.errorCode ?? '').toUpperCase();
    const cat = (row.failureCategory ?? '').toLowerCase();
    return (
      code === 'LOGIN_REQUIRED' ||
      code === 'VERIFY_REQUIRED' ||
      cat === 'login_required' ||
      cat === 'collector_platform_login'
    );
  };

  const isPddLoginFailure = (row: UnifiedTaskDTO | FailureDetailDTO | null) => {
    if (!row || row.taskType !== 'collect') return false;
    const pl = (row.platform ?? '').toLowerCase();
    if (pl !== 'pinduoduo' && pl !== 'pdd') return false;
    const code = (row.errorCode ?? '').toUpperCase();
    const cat = (row.failureCategory ?? '').toLowerCase();
    return (
      code === 'LOGIN_REQUIRED' ||
      code === 'WECHAT_AUTH_REQUIRED' ||
      cat === 'login_required' ||
      (row.errorMessage ?? '').includes('open.weixin.qq.com')
    );
  };

  const canRecoverDouyinDraft = (row: UnifiedTaskDTO | FailureDetailDTO | null) => {
    if (!row || row.taskType !== 'product_publish') return false;
    const pl = (row.platform ?? '').toLowerCase();
    if (pl !== 'douyin_shop') return false;
    const rs = (row.recoveryStatus ?? row.extra?.recoveryStatus ?? '').toString().trim();
    const code = (row.errorCode ?? '').toUpperCase();
    return rs === 'result_unknown' || code === 'DOUYIN_TASK_RESULT_UNKNOWN';
  };

  useEffect(() => {
    void (async () => {
      try {
        const c = await queryTaskFailureCategories();
        setFailureCatOpts(
          (c.categories || []).map((x) => ({
            label: TASK_FAILURE_CATEGORY_LABEL[x] || x,
            value: x,
          })),
        );
      } catch {
        setFailureCatOpts([]);
      }
    })();
  }, []);

  useEffect(() => {
    const sp = new URLSearchParams(location.search || '');
    const drawerId = sp.get('drawer') === 'failure' ? sp.get('id') : '';
    const drawerTaskType = sp.get('drawer') === 'failure' ? sp.get('detailTaskType') : '';
    const jumpId = drawerId || sp.get('jumpId');
    const taskType = drawerTaskType || sp.get('taskType');
    if (!jumpId || !taskType) {
      return;
    }
    if (!canOpenFailureDetail(taskType, jumpId)) {
      if (taskType.trim() === 'douyin_platform') {
        message.info(tr("该平台级告警无对应失败任务详情，已跳转到平台运行状态页"));
        history.replace('/ops/platform-runtime?platform=douyin_shop');
        return;
      }
      if (isPlatformAlertTaskType(taskType)) {
        message.info(tr("该平台级告警无对应失败任务详情，请在告警中心查看"));
        sp.delete('jumpId');
        sp.delete('drawer');
        sp.delete('id');
        sp.delete('detailTaskType');
        const qs = sp.toString();
        history.replace(qs ? `${location.pathname}?${qs}` : location.pathname);
        return;
      }
      message.warning(tr("链接中的任务类型或 ID 无效，无法打开失败详情"));
      sp.delete('jumpId');
      sp.delete('drawer');
      sp.delete('id');
      sp.delete('detailTaskType');
      const qs = sp.toString();
      history.replace(qs ? `${location.pathname}?${qs}` : location.pathname);
      return;
    }
    if (detailOpen && detail?.id === jumpId && detail?.taskType === taskType) {
      return;
    }
    void (async () => {
      try {
        setDetail(null);
        setDetailOpen(true);
        setDetailLoading(true);
        const d = await getTaskFailureDetail(taskType, jumpId);
        setDetail(d);
      } catch (e) {
        message.error((e as Error).message);
        setDetailOpen(false);
      } finally {
        setDetailLoading(false);
      }
    })();
  }, [detail?.id, detail?.taskType, detailOpen, location.pathname, location.search]);

  const columns: ProColumns<UnifiedTaskDTO>[] = useMemo(
    () => [
      {
        title: tr("更新时间"),
        dataIndex: 'timeRange',
        hideInTable: true,
        valueType: 'dateTimeRange',
        search: {
          transform: ([start, end]: [unknown, unknown]) => ({
            start: start ? dayjs(start as string).toISOString() : undefined,
            end: end ? dayjs(end as string).toISOString() : undefined,
          }),
        },
      },
      {
        title: tr("任务类型"),
        dataIndex: 'taskType',
        width: 120,
        valueType: 'select',
        fieldProps: {
          options: Object.keys(TASK_CENTER_TASK_TYPE_LABEL).map((k) => ({
            label: TASK_CENTER_TASK_TYPE_LABEL[k],
            value: k,
          })),
          allowClear: true,
          placeholder: tr("请选择"),
        },
        render: (_, r) => TASK_CENTER_TASK_TYPE_LABEL[r.taskType] || r.taskType,
      },
      {
        title: tr("状态(归一)"),
        dataIndex: 'normalizedStatus',
        width: 110,
        valueType: 'select',
        fieldProps: {
          options: Object.keys(TASK_NORMALIZED_STATUS).map((k) => ({
            label: TASK_NORMALIZED_STATUS[k].text,
            value: k,
          })),
          allowClear: true,
          placeholder: tr("请选择"),
        },
        render: (_, r) => normTag(r.normalizedStatus),
      },
      {
        title: tr("失败类别"),
        dataIndex: 'failureCategory',
        width: 132,
        valueType: 'select',
        fieldProps: {
          options: failureCatOpts,
          allowClear: true,
          showSearch: true,
          placeholder: tr("请选择"),
        },
        render: (_, r) => failureCategoryLabel(r.failureCategory),
      },
      {
        title: tr("恢复状态"),
        dataIndex: 'recoveryStatus',
        width: 148,
        valueType: 'select',
        fieldProps: {
          options: TASK_RECOVERY_STATUS_OPTIONS,
          allowClear: true,
          placeholder: tr("请选择"),
        },
        render: (_, r) => {
          const label = recoveryStatusLabel(r.recoveryStatus);
          if (label === '—') return '—';
          return <Tag color="gold">{label}</Tag>;
        },
      },
      {
        title: tr("严重等级"),
        dataIndex: 'severity',
        width: 106,
        valueType: 'select',
        fieldProps: {
          options: TASK_FAILURE_SEVERITY_OPTIONS,
          allowClear: true,
          placeholder: tr("请选择"),
        },
        render: (_, r) => severityCell(r.severity),
      },
      {
        title: tr("平台"),
        dataIndex: 'platform',
        width: 90,
      },
      {
        title: tr("店铺 ID"),
        dataIndex: 'shopId',
        width: 120,
        hideInTable: true,
      },
      {
        title: tr("店铺"),
        dataIndex: 'shopName',
        width: 120,
        search: false,
        ellipsis: true,
      },
      {
        title: tr("关键词"),
        dataIndex: 'keyword',
        hideInTable: true,
        fieldProps: keywordFieldProps,
      },
      {
        title: tr("创建时间"),
        dataIndex: 'createdAt',
        width: 156,
        search: false,
        render: (_, r) => formatDateTime(r.createdAt),
      },
      {
        title: tr("标题"),
        dataIndex: 'title',
        ellipsis: true,
        search: false,
      },
      {
        title: tr("关联"),
        search: false,
        width: 140,
        render: (_, r) => {
          const href = relatedHref(r);
          if (!href) return r.relatedResourceTitle || '—';
          return (
            <Typography.Link onClick={() => history.push(appendSourceToUrl(href, 'taskcenter'))}>
              {(r.relatedResourceTitle || '').slice(0, 32) || r.relatedResourceId}
            </Typography.Link>
          );
        },
      },
      {
        title: tr("重试次数"),
        dataIndex: 'retryCount',
        width: 76,
        search: false,
      },
      {
        title: tr("建议动作"),
        dataIndex: 'suggestedAction',
        width: 160,
        search: false,
        ellipsis: true,
      },
      {
        title: tr("错误摘要"),
        dataIndex: 'errorMessage',
        ellipsis: true,
        search: false,
        width: 180,
      },
      {
        title: tr("告警"),
        search: false,
        width: 100,
        render: (_, r) => alertStatusUi(r.alertStatus),
      },
      {
        title: tr("标记"),
        search: false,
        width: 100,
        render: (_, r) => (
          <Space size={[0, 4]} wrap>
            {r.ignored ? <Tag>{tr("忽略")}</Tag> : null}
            {r.handled ? <Tag color="blue">{tr("已处理")}</Tag> : null}
          </Space>
        ),
      },
      {
        title: tr("操作"),
        valueType: 'option',
        width: 320,
        fixed: 'right',
        render: (_, r) => (
          <Space wrap size={4}>
            <Button size="small" type="link" onClick={() => void openDetail(r)}>
              {tr("详情")}
            </Button>
            <Button size="small" type="link" onClick={() => void doGenerateAlert(r)}>
              {tr("生成告警")}
            </Button>
            <Button
              size="small"
              type="link"
              disabled={!r.relatedAlertId}
              onClick={() => history.push('/ops/task-center/alerts')}
            >
              {tr("告警列表")}
            </Button>
            {r.detailUrl ? (
              <Button size="small" type="link" onClick={() => history.push(appendSourceToUrl(r.detailUrl!, 'taskcenter'))}>
                {detailLinkLabel(r.detailUrl)}
              </Button>
            ) : null}
            <Button
              size="small"
              type="link"
              disabled={!r.retryable}
              onClick={() => confirmRetry(r)}
            >
              {tr("重试")}
            </Button>
            <Dropdown
              menu={{
                items: [
                  {
                    key: 'ignore',
                    label: tr("忽略"),
                    onClick: () => promptMark('ignore', r),
                  },
                  {
                    key: 'handle',
                    label: tr("标记已处理"),
                    onClick: () => promptMark('handle', r),
                  },
                  {
                    key: 'unmark',
                    label: tr("取消标记"),
                    disabled: !r.ignored && !r.handled,
                    onClick: () => void doUnmark(r),
                  },
                ],
              }}
            >
              <Button size="small" type="link">
                {tr("更多")}
              </Button>
            </Dropdown>
          </Space>
        ),
      },
    ],
    [failureCatOpts, keywordFieldProps],
  );

  async function doGenerateAlert(row: UnifiedTaskDTO) {
    Modal.confirm({
      title: tr("为该失败任务手动生成站内告警（可覆盖告警状态）"),
      onOk: async () => {
        try {
          await generateTaskFailureAlert(row.taskType, row.id);
          message.success(tr("已生成/刷新告警"));
          actionRef.current?.reload?.();
        } catch (e) {
          message.error((e as Error).message);
        }
      },
    });
  }

  async function openDetail(row: UnifiedTaskDTO) {
    setUrlState({ drawer: 'failure', id: row.id, detailTaskType: row.taskType });
    setDetail(null);
    setDetailOpen(true);
    setDetailLoading(true);
    try {
      const d = await getTaskFailureDetail(row.taskType, row.id);
      setDetail(d);
    } catch (e) {
      message.error((e as Error).message);
      setDetailOpen(false);
    } finally {
      setDetailLoading(false);
    }
  }

  function confirmRetry(row: UnifiedTaskDTO) {
    confirmFailureTaskRetry(1, async () => {
      await retryTaskFailure(row.taskType, row.id);
      message.success(tr("已提交重试"));
      actionRef.current?.reload?.();
    });
  }

  function promptMark(kind: 'ignore' | 'handle', row: UnifiedTaskDTO) {
    let txt = '';
    Modal.confirm({
      title: kind === 'ignore' ? tr('忽略此失败任务（列表默认隐藏）') : tr('标记为已线下处理（列表默认隐藏）'),
      content: (
        <Input.TextArea
          placeholder={tr("可选备注（不记录敏感信息）")}
          rows={3}
          onChange={(e) => {
            txt = e.target.value;
          }}
        />
      ),
      onOk: async () => {
        try {
          if (kind === 'ignore') {
            await ignoreTaskFailure(row.taskType, row.id, txt);
          } else {
            await handleTaskFailure(row.taskType, row.id, txt);
          }
          message.success(tr("已保存标记"));
          actionRef.current?.reload?.();
        } catch (e) {
          message.error((e as Error).message);
        }
      },
    });
  }

  async function doUnmark(row: UnifiedTaskDTO) {
    try {
      await unmarkTaskFailure(row.taskType, row.id);
      message.success(tr("已取消标记"));
      actionRef.current?.reload?.();
    } catch (e) {
      message.error((e as Error).message);
    }
  }

  const batchRows = sel.slice(0, 50);

  const latestFailedText = useMemo(
    () => (summary?.latestFailedAt ? formatDateTime(summary.latestFailedAt, '') : ''),
    [summary?.latestFailedAt, language],
  );

  return (
    <TmPageContainer
      title={PAGE_COPY.taskFailures.title}
      subTitle={PAGE_COPY.taskFailures.description}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <KeywordSafetyHint visible={showSensitiveHint} />
        <ProCard variant="outlined" size="small">
          {summary ? (
            <Space direction="vertical" size={16} style={{ width: '100%' }}>
              <Row gutter={[16, 16]}>
                <Col xs={12} sm={8} md={6} lg={4}>
                  <Statistic title={tr("归一失败")} value={summary.totalFailed ?? 0} />
                </Col>
                <Col xs={12} sm={8} md={6} lg={4}>
                  <Statistic title={tr("可重试")} value={summary.retryableCount ?? 0} />
                </Col>
                <Col xs={24} sm={16} md={12} lg={6}>
                  <Statistic
                    title={tr("重试中 / 停滞 / 租约过期")}
                    value={`${summary.retryingTotal ?? 0} / ${summary.staleTotal ?? 0} / ${summary.leaseExpiredTotal ?? 0}`}
                  />
                </Col>
                <Col xs={12} sm={8} md={6} lg={4}>
                  <Statistic title={tr("忽略标记")} value={summary.ignoredCount ?? 0} />
                </Col>
                <Col xs={12} sm={8} md={6} lg={4}>
                  <Statistic title={tr("已处理标记")} value={summary.handledCount ?? 0} />
                </Col>
                {latestFailedText ? (
                  <Col xs={24} sm={24} md={12} lg={6}>
                    <Statistic title={tr("最近失败")} value={latestFailedText} valueStyle={{ fontSize: 16 }} />
                  </Col>
                ) : null}
              </Row>

              {Object.keys(summary.byType || {}).length ? (
                <div>
                  <Typography.Text type="secondary" style={{ marginRight: 8 }}>
                    {tr("按类型：")}
                  </Typography.Text>
                  <Space size={[8, 8]} wrap>
                    {Object.entries(summary.byType || {}).map(([k, v]) => (
                      <Tag key={k} color="processing">
                        {(TASK_CENTER_TASK_TYPE_LABEL[k] || k) + ` ${v}`}
                      </Tag>
                    ))}
                  </Space>
                </div>
              ) : null}

              <Divider style={{ margin: 0 }} />

              <Row gutter={[16, 12]} align="middle" justify="space-between">
                <Col xs={24} lg={14}>
                  <Space wrap align="center">
                    <Typography.Text type="secondary">{tr("批量操作")}</Typography.Text>
                    <Button
                      disabled={!batchRows.length}
                      type="primary"
                      onClick={() => {
                        confirmFailureTaskRetry(batchRows.length, async () => {
                          const res = await batchRetryTaskFailures(
                            batchRows.map((r) => ({ taskType: r.taskType, id: r.id })),
                          );
                          message.info(trFormat("成功 {value0}，失败 {value1}", { value0: res.successCount, value1: res.failedCount }));
                          actionRef.current?.reload?.();
                        });
                      }}
                    >
                      {tr("批量重试")}
                    </Button>
                    <Button
                      disabled={!batchRows.length}
                      onClick={() =>
                        Modal.confirm({
                          title: trFormat("批量忽略 {value0} 条任务？", { value0: batchRows.length }),
                          onOk: async () => {
                            try {
                              const res = await batchIgnoreTaskFailures(
                                batchRows.map((r) => ({ taskType: r.taskType, id: r.id })),
                              );
                              message.info(trFormat("忽略成功 {value0}，失败 {value1}", { value0: res.successCount, value1: res.failedCount }));
                              actionRef.current?.reload?.();
                            } catch (e) {
                              message.error((e as Error).message);
                            }
                          },
                        })
                      }
                    >
                      {tr("批量忽略")}
                    </Button>
                    <Button
                      disabled={!batchRows.length}
                      onClick={() =>
                        Modal.confirm({
                          title: trFormat("批量标记已处理（{value0} 条）？", { value0: batchRows.length }),
                          onOk: async () => {
                            try {
                              const res = await batchHandleTaskFailures(
                                batchRows.map((r) => ({ taskType: r.taskType, id: r.id })),
                              );
                              message.info(trFormat("成功 {value0}，失败 {value1}", { value0: res.successCount, value1: res.failedCount }));
                              actionRef.current?.reload?.();
                            } catch (e) {
                              message.error((e as Error).message);
                            }
                          },
                        })
                      }
                    >
                      {tr("批量已处理")}
                    </Button>
                    <Typography.Text type="secondary">
                      {tr("已选")} {sel.length} {tr("条")}{sel.length > 50 ? tr("（批量操作仅前 50 条）") : ''}
                    </Typography.Text>
                  </Space>
                </Col>
                <Col xs={24} lg={10}>
                  <Space wrap align="center" style={{ justifyContent: 'flex-end', width: '100%' }}>
                    <Typography.Text type="secondary">{tr("列表范围")}</Typography.Text>
                    <Space size={6}>
                      <Typography.Text>{tr("含已恢复")}</Typography.Text>
                      <Switch
                        checked={includeResolved}
                        checkedChildren={tr("是")}
                        unCheckedChildren={tr("否")}
                        onChange={(checked) => {
                          listFilterRef.current.includeResolved = checked;
                          setIncludeResolved(checked);
                          setUrlState({ includeResolved: checked, page: undefined });
                          setTablePage(1);
                          actionRef.current?.reload?.();
                        }}
                      />
                    </Space>
                    <Space size={6}>
                      <Typography.Text>{tr("含已标记")}</Typography.Text>
                      <Switch
                        checked={includeMarked}
                        checkedChildren={tr("是")}
                        unCheckedChildren={tr("否")}
                        onChange={(checked) => {
                          listFilterRef.current.includeMarked = checked;
                          setIncludeMarked(checked);
                          setUrlState({ includeMarked: checked, page: undefined });
                          setTablePage(1);
                          actionRef.current?.reload?.();
                        }}
                      />
                    </Space>
                    <Typography.Link onClick={() => history.push('/ops/workers/monitor')}>
                      {tr("后台任务监控")}
                    </Typography.Link>
                  </Space>
                </Col>
              </Row>
            </Space>
          ) : (
            <Badge status="processing" text={tr("载入统计...")} />
          )}
        </ProCard>

        <ProTable<UnifiedTaskDTO>
          rowKey={(r) => `${r.taskType}:${r.id}`}
          columns={columns}
          actionRef={actionRef}
          formRef={formRef}
          params={{
            current: tablePage,
            pageSize: tablePageSize,
            taskType: urlState.taskType,
            normalizedStatus: urlState.normalizedStatus,
            failureCategory: urlState.failureCategory,
            recoveryStatus: urlState.recoveryStatus,
            severity: urlState.severity,
            platform: urlState.platform,
            shopId: urlState.shopId,
            keyword: urlState.keyword,
            start: urlState.start,
            end: urlState.end,
          }}
          search={{
            labelWidth: 'auto',
          }}
          onReset={() => {
            listFilterRef.current = { includeResolved: false, includeMarked: false };
            setIncludeResolved(false);
            setIncludeMarked(false);
            setTablePage(1);
            setTablePageSize(20);
            setDetailOpen(false);
            setDetail(null);
            clearUrlState(FAILURE_QUERY_KEYS, { replace: true });
          }}
          rowSelection={{
            selections: true,
            onChange: (_, rows) => setSel(rows),
          }}
          pagination={{
            current: tablePage,
            pageSize: tablePageSize,
            showSizeChanger: true,
            onChange: (page, pageSize) => {
              setTablePage(page);
              setTablePageSize(pageSize);
              setUrlState({
                page: page > 1 ? page : undefined,
                pageSize: pageSize !== 20 ? pageSize : undefined,
              });
            },
          }}
          scroll={{ x: 1680 }}
          tableAlertRender={false}
          locale={emptyLocale}
          request={async (params) => {
            const kw = prepareKeyword(params.keyword) ?? '';
            try {
              const qp: Record<string, string | number | undefined> = {
                page: params.current ?? tablePage,
                pageSize: params.pageSize ?? tablePageSize,
                taskType: (params.taskType as string | undefined)?.trim(),
                normalizedStatus: (params.normalizedStatus as string | undefined)?.trim(),
                platform: (params.platform as string | undefined)?.trim(),
                shopId: (params.shopId as string | undefined)?.trim(),
                keyword: kw || undefined,
                failureCategory: (params.failureCategory as string | undefined)?.trim(),
                severity: (params.severity as string | undefined)?.trim(),
                recoveryStatus: (params.recoveryStatus as string | undefined)?.trim(),
              };
              if (listFilterRef.current.includeResolved) {
                qp.includeResolved = 'true';
              }
              if (listFilterRef.current.includeMarked) {
                qp.includeMarked = 'true';
              }
              if (typeof params.start === 'string' && params.start) qp.start = params.start;
              if (typeof params.end === 'string' && params.end) qp.end = params.end;
              setUrlState(
                {
                  page: Number(qp.page) > 1 ? qp.page : undefined,
                  pageSize: Number(qp.pageSize) !== 20 ? qp.pageSize : undefined,
                  taskType: qp.taskType,
                  normalizedStatus: qp.normalizedStatus,
                  platform: qp.platform,
                  shopId: qp.shopId,
                  keyword: qp.keyword,
                  failureCategory: qp.failureCategory,
                  severity: qp.severity,
                  recoveryStatus: qp.recoveryStatus,
                  start: qp.start,
                  end: qp.end,
                  includeResolved: listFilterRef.current.includeResolved,
                  includeMarked: listFilterRef.current.includeMarked,
                },
                { replace: true },
              );
              const data = await queryTaskFailures(qp);
              setSummary(data.summary);
              return { data: data.list, total: data.total, success: true };
            } catch (e) {
              message.error((e as Error).message);
              return { data: [], total: 0, success: false };
            }
          }}
        />
      </Space>

      <Drawer
        width={640}
        open={detailOpen}
        onClose={() => {
          setDetailOpen(false);
          setDetail(null);
          setUrlState({ drawer: undefined, id: undefined, detailTaskType: undefined }, { replace: true });
        }}
        title={tr("失败任务详情（摘要）")}
      >
        {detailLoading ? <Typography.Paragraph type="secondary">{tr("载入中...")}</Typography.Paragraph> : null}
        {detail ? (
          <Space direction="vertical" style={{ width: '100%' }} size={12}>
            <Typography.Title level={5}>{detail.title}</Typography.Title>
            <div>{normTag(detail.normalizedStatus)}</div>
            <div>
              <Typography.Text strong>{tr("失败类别：")}</Typography.Text> {failureCategoryLabel(detail.failureCategory)}{' '}
              {severityCell(detail.severity)}
            </div>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>{tr("归类说明：")}</Typography.Text>
              <br />
              {detail.classificationReason || '—'}
            </Typography.Paragraph>
            <Typography.Paragraph style={{ marginBottom: 0 }} type="secondary">
              <Typography.Text strong>{tr("命中规则：")}</Typography.Text> {detail.matchedRule || '—'}
            </Typography.Paragraph>
            <Typography.Paragraph ellipsis={{ rows: 5, expandable: true }}>
              <Typography.Text strong>{tr("建议处理：")}</Typography.Text>
              <br />
              {detail.suggestedAction || '—'}
            </Typography.Paragraph>
            <div>
              <Typography.Text strong>{tr("告警状态：")}</Typography.Text> {alertStatusUi(detail.alertStatus)}{' '}
              {detail.relatedAlertId ? (
                <Typography.Link onClick={() => history.push('/ops/task-center/alerts')}>
                  {tr("（打开告警中心）")}
                </Typography.Link>
              ) : null}
            </div>
            <Typography.Paragraph ellipsis={{ rows: 4, expandable: true }}>
              <strong>{tr("错误摘要")}</strong>
              <br />
              {detail.errorMessage || '—'}
            </Typography.Paragraph>
            {detail.recoveryStatus || detail.extra?.recoveryStatus ? (
              <Typography.Paragraph style={{ marginBottom: 0 }}>
                <Typography.Text strong>{tr("恢复状态：")}</Typography.Text>{' '}
                {recoveryStatusLabel(
                  (detail.recoveryStatus || detail.extra?.recoveryStatus) as string | undefined,
                )}
              </Typography.Paragraph>
            ) : null}
            {detail.deadLetter ? (
              <Typography.Paragraph style={{ marginBottom: 0 }}>
                <Tag color="error">{tr("死信任务")}</Tag>
                {detail.safeRetry === false ? (
                  <Typography.Text type="secondary"> {tr("· 不建议自动重试")}</Typography.Text>
                ) : null}
              </Typography.Paragraph>
            ) : null}
            {detail.nextRunAt ? (
              <Typography.Paragraph style={{ marginBottom: 0 }} type="secondary">
                <Typography.Text strong>{tr("下次执行：")}</Typography.Text> {formatDateTime(detail.nextRunAt)}
              </Typography.Paragraph>
            ) : null}
            {detail.idempotencyScope ? (
              <Typography.Paragraph style={{ marginBottom: 0 }} type="secondary">
                <Typography.Text strong>{tr("幂等作用域：")}</Typography.Text> {detail.idempotencyScope}
              </Typography.Paragraph>
            ) : null}
            {detail.safeRetry === true && !detail.manualReviewRequired && !detail.unknownResult ? (
              <Alert type="success" showIcon message={tr("可安全重试")} style={{ marginBottom: 8 }} />
            ) : null}
            {detail.safeRetry === false && !detail.manualReviewRequired && !detail.unknownResult ? (
              <Alert type="warning" showIcon message={tr("需要先人工确认")} style={{ marginBottom: 8 }} />
            ) : null}
            {typeof detail.errorMessage === 'string' &&
            /TARGET_VERSION_CONFLICT|UNDO_VERSION_CONFLICT|content conflict|目标.*变化|版本冲突/i.test(
              detail.errorMessage,
            ) ? (
              <Alert type="warning" showIcon message={tr("目标数据已变化，不能直接覆盖")} style={{ marginBottom: 8 }} />
            ) : null}
            {typeof detail.errorMessage === 'string' &&
            /TASK_LEASE_LOST|lease lost|租约|已被.*接管/i.test(detail.errorMessage) ? (
              <Alert type="info" showIcon message={tr("任务已被新的后台执行进程接管")} style={{ marginBottom: 8 }} />
            ) : null}
            {detail.unknownResult ? (
              <Alert type="error" showIcon message={tr("结果未知，请先核对外部系统")} style={{ marginBottom: 8 }} />
            ) : null}
            {(detail.heartbeatAt || detail.leaseVersion || detail.executionId) ? (
              <TechnicalDetails>
                {detail.heartbeatAt ? (
                  <Typography.Paragraph style={{ marginBottom: 4 }} type="secondary">
                    <Typography.Text strong>{tr("最近心跳：")}</Typography.Text> {formatDateTime(detail.heartbeatAt)}
                  </Typography.Paragraph>
                ) : null}
                {detail.leaseVersion != null ? (
                  <Typography.Paragraph style={{ marginBottom: 4 }} type="secondary">
                    <Typography.Text strong>{tr("租约版本：")}</Typography.Text> {detail.leaseVersion}
                  </Typography.Paragraph>
                ) : null}
                {detail.executionId ? (
                  <Typography.Paragraph style={{ marginBottom: 0 }} type="secondary">
                    <Typography.Text strong>{tr("执行标识：")}</Typography.Text>{' '}
                    <Typography.Text code copyable={{ text: detail.executionId }}>
                      {detail.executionId.slice(0, 8)}…
                    </Typography.Text>
                  </Typography.Paragraph>
                ) : null}
              </TechnicalDetails>
            ) : null}
            {detail.idempotencyStatus ? (
              <Typography.Paragraph style={{ marginBottom: 0 }} type="secondary">
                <Typography.Text strong>{tr("幂等状态：")}</Typography.Text> {detail.idempotencyStatus}
              </Typography.Paragraph>
            ) : null}
            {detail.manualReviewRequired ? (
              <Alert type="warning" showIcon message={tr("需人工确认后再重试（平台写操作结果未知）")} style={{ marginBottom: 8 }} />
            ) : null}
            {typeof detail.extra?.userMessage === 'string' && detail.extra.userMessage ? (
              <Typography.Paragraph style={{ marginBottom: 0 }}>
                <Typography.Text strong>{tr("恢复说明：")}</Typography.Text> {detail.extra.userMessage}
              </Typography.Paragraph>
            ) : null}
            {detail.extra?.retryAttempts != null ? (
              <Typography.Paragraph style={{ marginBottom: 0 }} type="secondary">
                <Typography.Text strong>{tr("恢复尝试次数：")}</Typography.Text> {String(detail.extra.retryAttempts)}
              </Typography.Paragraph>
            ) : null}
            {typeof detail.extra?.lastErrorCode === 'string' && detail.extra.lastErrorCode ? (
              <Typography.Paragraph style={{ marginBottom: 0 }} type="secondary">
                <Typography.Text strong>{tr("最近错误码：")}</Typography.Text> {detail.extra.lastErrorCode}
              </Typography.Paragraph>
            ) : null}
            {detail.relatedResourceTitle ? (
              <Typography.Paragraph type="secondary">{tr("关联：")}{detail.relatedResourceTitle}</Typography.Paragraph>
            ) : null}
            <Space wrap>
              {isTbLoginFailure(detail) ? (
                <Button
                  type="primary"
                  loading={tbLoginOpening}
                  onClick={async () => {
                    setTbLoginOpening(true);
                    try {
                      const src =
                        typeof detail.extra?.sourceUrl === 'string'
                          ? String(detail.extra.sourceUrl).trim()
                          : '';
                      const loginUrl = resolveTaobaoTmallLoginTargetUrl(src || undefined);
                      const res = await openTaobaoTmallLoginBrowser(loginUrl);
                      message.success(res.message || tr('已打开淘宝/天猫采集浏览器'));
                    } catch (e) {
                      message.error((e as Error).message);
                    } finally {
                      setTbLoginOpening(false);
                    }
                  }}
                >
                  {tr("打开淘宝/天猫采集浏览器")}
                </Button>
              ) : null}
              {isPddLoginFailure(detail) ? (
                <Button
                  type="primary"
                  loading={pddLoginOpening}
                  onClick={async () => {
                    setPddLoginOpening(true);
                    try {
                      const src =
                        typeof detail.extra?.sourceUrl === 'string'
                          ? String(detail.extra.sourceUrl).trim()
                          : '';
                      const loginUrl = resolvePinduoduoLoginTargetUrl(src || undefined);
                      const res = await openPinduoduoLoginBrowser(loginUrl);
                      message.success(res.message || tr('已打开拼多多采集浏览器'));
                    } catch (e) {
                      message.error((e as Error).message);
                    } finally {
                      setPddLoginOpening(false);
                    }
                  }}
                >
                  {tr("打开拼多多采集浏览器登录")}
                </Button>
              ) : null}
              {canRecoverDouyinDraft(detail) ? (
                <Button
                  type="primary"
                  loading={recovering}
                  onClick={() => {
                    Modal.confirm({
                      title: tr("尝试从抖店回查恢复草稿？"),
                      content: trFormat('将通过 {value0} 确认平台侧是否已创建草稿，不会盲目重复提交 {value1}。', { value0: 'product.detail', value1: 'product.addV2' }),
                      okText: tr("尝试恢复"),
                      onOk: async () => {
                        setRecovering(true);
                        try {
                          await recoverDouyinDraftTask(detail.id);
                          message.success(tr("已提交恢复，请刷新查看任务状态"));
                          actionRef.current?.reload?.();
                          const refreshed = await getTaskFailureDetail(detail.taskType, detail.id);
                          setDetail(refreshed);
                        } catch (e) {
                          message.error((e as Error).message);
                        } finally {
                          setRecovering(false);
                        }
                      },
                    });
                  }}
                >
                  {tr("尝试恢复")}
                </Button>
              ) : null}
              {detail.retryable ? (
                <Button
                  onClick={() => {
                    confirmFailureTaskRetry(1, async () => {
                      try {
                        await retryTaskFailure(detail.taskType, detail.id);
                        message.success(tr("已提交重试"));
                        actionRef.current?.reload?.();
                      } catch (e) {
                        message.error((e as Error).message);
                        throw e;
                      }
                    });
                  }}
                >
                  {tr("重试任务")}
                </Button>
              ) : null}
              {typeof detail.extra?.sourceUrl === 'string' && detail.extra.sourceUrl ? (
                <Button
                  onClick={() => {
                    window.open(detail.extra!.sourceUrl as string, '_blank', 'noopener,noreferrer');
                  }}
                >
                  {tr("打开原始失败页面")}
                </Button>
              ) : null}
              {detail.taskType ? (
                <Button
                  onClick={() => {
                    Modal.confirm({
                      title: tr("生成站内告警记录"),
                      onOk: async () => {
                        try {
                          await generateTaskFailureAlert(detail.taskType, detail.id);
                          message.success(tr("已生成/刷新告警"));
                          actionRef.current?.reload?.();
                          const refreshed = await getTaskFailureDetail(detail.taskType, detail.id);
                          setDetail(refreshed);
                        } catch (e) {
                          message.error((e as Error).message);
                        }
                      },
                    });
                  }}
                >
                  {tr("生成告警")}
                </Button>
              ) : null}
              {detail.detailUrl ? (
                <Button onClick={() => history.push(appendSourceToUrl(detail.detailUrl!, 'taskcenter'))}>
                  {detailLinkLabel(detail.detailUrl)}
                </Button>
              ) : null}
              {detail.taskType === 'ai_image' ? (
                <>
                  <Button onClick={() => history.push('/settings/config-status')}>{tr("查看配置状态")}</Button>
                  <Button onClick={() => history.push('/settings/image')}>{tr("查看图片 AI 设置")}</Button>
                  {(detail.failureCategory || '').includes('storage') ? (
                    <Button onClick={() => history.push('/settings/storage')}>{tr("查看存储设置")}</Button>
                  ) : null}
                </>
              ) : null}
            </Space>
            {detail.extra?.urlTypeLabel ? (
              <Typography.Paragraph style={{ marginBottom: 0 }}>
                <Typography.Text strong>{tr("链接类型：")}</Typography.Text>{' '}
                {String(detail.extra.urlTypeLabel)}
              </Typography.Paragraph>
            ) : null}
            {detail.extra?.accessStatusLabel ? (
              <Typography.Paragraph style={{ marginBottom: 0 }}>
                <Typography.Text strong>{tr("页面访问状态：")}</Typography.Text>{' '}
                {String(detail.extra.accessStatusLabel)}
              </Typography.Paragraph>
            ) : null}
            {detail.extra?.suggestedActionDetail ? (
              <Typography.Paragraph style={{ marginBottom: 0 }}>
                <Typography.Text strong>{tr("建议操作：")}</Typography.Text>{' '}
                {String(detail.extra.suggestedActionDetail)}
              </Typography.Paragraph>
            ) : null}
            {detail.extra && Object.keys(detail.extra).length > 0 ? (
              <TechnicalDetails>
                {detail.errorCode ? (
                  <TaskJsonBlock title={tr("错误码")} value={detail.errorCode} />
                ) : null}
                <TaskJsonBlock title={tr("附加信息")} value={detail.extra} last />
              </TechnicalDetails>
            ) : null}
          </Space>
        ) : null}
      </Drawer>
    </TmPageContainer>
  );
}
