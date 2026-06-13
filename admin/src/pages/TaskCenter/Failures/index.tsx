import { ProCard, type ActionType, type ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails, TaskJsonBlock, TmProTable as ProTable } from '@/components/ui';
import { history, useLocation } from '@umijs/max';
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
import {
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
  none: { color: 'default', text: '无' },
  generated: { color: 'gold', text: '告警中' },
  handled: { color: 'green', text: '告警已处理' },
  ignored: { color: 'default', text: '告警忽略' },
};

function alertStatusUi(st?: string) {
  const m = ALERT_ST_META[st || 'none'];
  if (!m) return <Tag>{st}</Tag>;
  return <Tag color={m.color}>{m.text}</Tag>;
}

function relatedHref(row: UnifiedTaskDTO): string | undefined {
  const t = row.relatedResourceType || '';
  const id = row.relatedResourceId || '';
  if (!id) return undefined;
  if (t === 'product') return `/product/drafts/${id}`;
  return undefined;
}

export default function TaskCenterFailuresPage() {
  const location = useLocation();
  const actionRef = useRef<ActionType>();
  const listFilterRef = useRef({ includeResolved: false, includeMarked: false });
  const [includeResolved, setIncludeResolved] = useState(false);
  const [includeMarked, setIncludeMarked] = useState(false);
  const [failureCatOpts, setFailureCatOpts] = useState<{ label: string; value: string }[]>([]);
  const [summary, setSummary] = useState<FailuresSummary | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState<FailureDetailDTO | null>(null);
  const [sel, setSel] = useState<UnifiedTaskDTO[]>([]);
  const [pddLoginOpening, setPddLoginOpening] = useState(false);
  const [tbLoginOpening, setTbLoginOpening] = useState(false);
  const [recovering, setRecovering] = useState(false);

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
    const jumpId = sp.get('jumpId');
    const taskType = sp.get('taskType');
    if (jumpId && taskType) {
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
    }
  }, [location.search]);

  const columns: ProColumns<UnifiedTaskDTO>[] = useMemo(
    () => [
      {
        title: '更新时间',
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
        title: '任务类型',
        dataIndex: 'taskType',
        width: 120,
        valueType: 'select',
        fieldProps: {
          options: Object.keys(TASK_CENTER_TASK_TYPE_LABEL).map((k) => ({
            label: TASK_CENTER_TASK_TYPE_LABEL[k],
            value: k,
          })),
          allowClear: true,
          placeholder: '请选择',
        },
        render: (_, r) => TASK_CENTER_TASK_TYPE_LABEL[r.taskType] || r.taskType,
      },
      {
        title: '状态(归一)',
        dataIndex: 'normalizedStatus',
        width: 110,
        valueType: 'select',
        fieldProps: {
          options: Object.keys(TASK_NORMALIZED_STATUS).map((k) => ({
            label: TASK_NORMALIZED_STATUS[k].text,
            value: k,
          })),
          allowClear: true,
          placeholder: '请选择',
        },
        render: (_, r) => normTag(r.normalizedStatus),
      },
      {
        title: '失败类别',
        dataIndex: 'failureCategory',
        width: 132,
        valueType: 'select',
        fieldProps: {
          options: failureCatOpts,
          allowClear: true,
          showSearch: true,
          placeholder: '请选择',
        },
        render: (_, r) => failureCategoryLabel(r.failureCategory),
      },
      {
        title: '恢复状态',
        dataIndex: 'recoveryStatus',
        width: 148,
        valueType: 'select',
        fieldProps: {
          options: TASK_RECOVERY_STATUS_OPTIONS,
          allowClear: true,
          placeholder: '请选择',
        },
        render: (_, r) => {
          const label = recoveryStatusLabel(r.recoveryStatus);
          if (label === '—') return '—';
          return <Tag color="gold">{label}</Tag>;
        },
      },
      {
        title: '严重等级',
        dataIndex: 'severity',
        width: 106,
        valueType: 'select',
        fieldProps: {
          options: TASK_FAILURE_SEVERITY_OPTIONS,
          allowClear: true,
          placeholder: '请选择',
        },
        render: (_, r) => severityCell(r.severity),
      },
      {
        title: '平台',
        dataIndex: 'platform',
        width: 90,
      },
      {
        title: '店铺 ID',
        dataIndex: 'shopId',
        width: 120,
        hideInTable: true,
      },
      {
        title: '店铺',
        dataIndex: 'shopName',
        width: 120,
        search: false,
        ellipsis: true,
      },
      {
        title: '关键词',
        dataIndex: 'keyword',
        hideInTable: true,
      },
      {
        title: '创建时间',
        dataIndex: 'createdAt',
        width: 156,
        search: false,
        render: (_, r) => formatDateTime(r.createdAt),
      },
      {
        title: '标题',
        dataIndex: 'title',
        ellipsis: true,
        search: false,
      },
      {
        title: '关联',
        search: false,
        width: 140,
        render: (_, r) => {
          const href = relatedHref(r);
          if (!href) return r.relatedResourceTitle || '—';
          return (
            <Typography.Link onClick={() => history.push(href)}>
              {(r.relatedResourceTitle || '').slice(0, 32) || r.relatedResourceId}
            </Typography.Link>
          );
        },
      },
      {
        title: '重试次数',
        dataIndex: 'retryCount',
        width: 76,
        search: false,
      },
      {
        title: '建议动作',
        dataIndex: 'suggestedAction',
        width: 160,
        search: false,
        ellipsis: true,
      },
      {
        title: '错误摘要',
        dataIndex: 'errorMessage',
        ellipsis: true,
        search: false,
        width: 180,
      },
      {
        title: '告警',
        search: false,
        width: 100,
        render: (_, r) => alertStatusUi(r.alertStatus),
      },
      {
        title: '标记',
        search: false,
        width: 100,
        render: (_, r) => (
          <Space size={[0, 4]} wrap>
            {r.ignored ? <Tag>忽略</Tag> : null}
            {r.handled ? <Tag color="blue">已处理</Tag> : null}
          </Space>
        ),
      },
      {
        title: '操作',
        valueType: 'option',
        width: 320,
        fixed: 'right',
        render: (_, r) => (
          <Space wrap size={4}>
            <Button size="small" type="link" onClick={() => void openDetail(r)}>
              详情
            </Button>
            <Button size="small" type="link" onClick={() => void doGenerateAlert(r)}>
              生成告警
            </Button>
            <Button
              size="small"
              type="link"
              disabled={!r.relatedAlertId}
              onClick={() => history.push('/ops/task-center/alerts')}
            >
              告警列表
            </Button>
            {r.detailUrl ? (
              <Button size="small" type="link" onClick={() => history.push(r.detailUrl!)}>
                原页面
              </Button>
            ) : null}
            <Button
              size="small"
              type="link"
              disabled={!r.retryable}
              onClick={() => confirmRetry(r)}
            >
              重试
            </Button>
            <Dropdown
              menu={{
                items: [
                  {
                    key: 'ignore',
                    label: '忽略',
                    onClick: () => promptMark('ignore', r),
                  },
                  {
                    key: 'handle',
                    label: '标记已处理',
                    onClick: () => promptMark('handle', r),
                  },
                  {
                    key: 'unmark',
                    label: '取消标记',
                    disabled: !r.ignored && !r.handled,
                    onClick: () => void doUnmark(r),
                  },
                ],
              }}
            >
              <Button size="small" type="link">
                更多
              </Button>
            </Dropdown>
          </Space>
        ),
      },
    ],
    [failureCatOpts],
  );

  async function doGenerateAlert(row: UnifiedTaskDTO) {
    Modal.confirm({
      title: '为该失败任务手动生成站内告警（可覆盖告警状态）',
      onOk: async () => {
        try {
          await generateTaskFailureAlert(row.taskType, row.id);
          message.success('已生成/刷新告警');
          actionRef.current?.reload?.();
        } catch (e) {
          message.error((e as Error).message);
        }
      },
    });
  }

  async function openDetail(row: UnifiedTaskDTO) {
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
    Modal.confirm({
      title: '确认重试此任务？',
      content: `${TASK_CENTER_TASK_TYPE_LABEL[row.taskType] || row.taskType} · ${row.id}`,
      okText: '重试',
      onOk: async () => {
        try {
          await retryTaskFailure(row.taskType, row.id);
          message.success('已提交重试');
          actionRef.current?.reload?.();
        } catch (e) {
          message.error((e as Error).message);
        }
      },
    });
  }

  function promptMark(kind: 'ignore' | 'handle', row: UnifiedTaskDTO) {
    let txt = '';
    Modal.confirm({
      title: kind === 'ignore' ? '忽略此失败任务（列表默认隐藏）' : '标记为已线下处理（列表默认隐藏）',
      content: (
        <Input.TextArea
          placeholder="可选备注（不记录敏感信息）"
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
          message.success('已保存标记');
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
      message.success('已取消标记');
      actionRef.current?.reload?.();
    } catch (e) {
      message.error((e as Error).message);
    }
  }

  const batchRows = sel.slice(0, 50);

  const latestFailedText = useMemo(
    () => (summary?.latestFailedAt ? formatDateTime(summary.latestFailedAt, '') : ''),
    [summary?.latestFailedAt],
  );

  return (
    <TmPageContainer
      title={PAGE_COPY.taskFailures.title}
      subTitle={PAGE_COPY.taskFailures.description}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Alert
          showIcon
          type="info"
          message="抖店相关失败"
          description="刊登、图片上传、创建草稿、订单同步、库存同步、SKU 绑定失败会聚合到本页。错误信息已脱敏，不含 token 或 App Secret。抖店授权/类目/图片/SKU 未绑定类问题请按提示回到对应页面处理后再重试。"
        />
        <ProCard variant="outlined" size="small">
          {summary ? (
            <Space direction="vertical" size={16} style={{ width: '100%' }}>
              <Row gutter={[16, 16]}>
                <Col xs={12} sm={8} md={6} lg={4}>
                  <Statistic title="归一失败" value={summary.totalFailed ?? 0} />
                </Col>
                <Col xs={12} sm={8} md={6} lg={4}>
                  <Statistic title="可重试" value={summary.retryableCount ?? 0} />
                </Col>
                <Col xs={24} sm={16} md={12} lg={6}>
                  <Statistic
                    title="重试中 / 停滞 / 租约过期"
                    value={`${summary.retryingTotal ?? 0} / ${summary.staleTotal ?? 0} / ${summary.leaseExpiredTotal ?? 0}`}
                  />
                </Col>
                <Col xs={12} sm={8} md={6} lg={4}>
                  <Statistic title="忽略标记" value={summary.ignoredCount ?? 0} />
                </Col>
                <Col xs={12} sm={8} md={6} lg={4}>
                  <Statistic title="已处理标记" value={summary.handledCount ?? 0} />
                </Col>
                {latestFailedText ? (
                  <Col xs={24} sm={24} md={12} lg={6}>
                    <Statistic title="最近失败" value={latestFailedText} valueStyle={{ fontSize: 16 }} />
                  </Col>
                ) : null}
              </Row>

              {Object.keys(summary.byType || {}).length ? (
                <div>
                  <Typography.Text type="secondary" style={{ marginRight: 8 }}>
                    按类型：
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
                    <Typography.Text type="secondary">批量操作</Typography.Text>
                    <Button
                      disabled={!batchRows.length}
                      type="primary"
                      onClick={() => {
                        Modal.confirm({
                          title: `批量重试（${batchRows.length} 条，最多 50）？`,
                          onOk: async () => {
                            try {
                              const res = await batchRetryTaskFailures(
                                batchRows.map((r) => ({ taskType: r.taskType, id: r.id })),
                              );
                              message.info(`成功 ${res.successCount}，失败 ${res.failedCount}`);
                              actionRef.current?.reload?.();
                            } catch (e) {
                              message.error((e as Error).message);
                            }
                          },
                        });
                      }}
                    >
                      批量重试
                    </Button>
                    <Button
                      disabled={!batchRows.length}
                      onClick={() =>
                        Modal.confirm({
                          title: `批量忽略 ${batchRows.length} 条任务？`,
                          onOk: async () => {
                            try {
                              const res = await batchIgnoreTaskFailures(
                                batchRows.map((r) => ({ taskType: r.taskType, id: r.id })),
                              );
                              message.info(`忽略成功 ${res.successCount}，失败 ${res.failedCount}`);
                              actionRef.current?.reload?.();
                            } catch (e) {
                              message.error((e as Error).message);
                            }
                          },
                        })
                      }
                    >
                      批量忽略
                    </Button>
                    <Button
                      disabled={!batchRows.length}
                      onClick={() =>
                        Modal.confirm({
                          title: `批量标记已处理（${batchRows.length} 条）？`,
                          onOk: async () => {
                            try {
                              const res = await batchHandleTaskFailures(
                                batchRows.map((r) => ({ taskType: r.taskType, id: r.id })),
                              );
                              message.info(`成功 ${res.successCount}，失败 ${res.failedCount}`);
                              actionRef.current?.reload?.();
                            } catch (e) {
                              message.error((e as Error).message);
                            }
                          },
                        })
                      }
                    >
                      批量已处理
                    </Button>
                    <Typography.Text type="secondary">
                      已选 {sel.length} 条{sel.length > 50 ? '（批量操作仅前 50 条）' : ''}
                    </Typography.Text>
                  </Space>
                </Col>
                <Col xs={24} lg={10}>
                  <Space wrap align="center" style={{ justifyContent: 'flex-end', width: '100%' }}>
                    <Typography.Text type="secondary">列表范围</Typography.Text>
                    <Space size={6}>
                      <Typography.Text>含已恢复</Typography.Text>
                      <Switch
                        checked={includeResolved}
                        checkedChildren="是"
                        unCheckedChildren="否"
                        onChange={(checked) => {
                          listFilterRef.current.includeResolved = checked;
                          setIncludeResolved(checked);
                          actionRef.current?.reload?.();
                        }}
                      />
                    </Space>
                    <Space size={6}>
                      <Typography.Text>含已标记</Typography.Text>
                      <Switch
                        checked={includeMarked}
                        checkedChildren="是"
                        unCheckedChildren="否"
                        onChange={(checked) => {
                          listFilterRef.current.includeMarked = checked;
                          setIncludeMarked(checked);
                          actionRef.current?.reload?.();
                        }}
                      />
                    </Space>
                    <Typography.Link onClick={() => history.push('/ops/workers/monitor')}>
                      后台任务监控
                    </Typography.Link>
                  </Space>
                </Col>
              </Row>
            </Space>
          ) : (
            <Badge status="processing" text="载入统计..." />
          )}
        </ProCard>

        <ProTable<UnifiedTaskDTO>
          rowKey={(r) => `${r.taskType}:${r.id}`}
          columns={columns}
          actionRef={actionRef}
          search={{
            labelWidth: 'auto',
            onReset: () => {
              listFilterRef.current = { includeResolved: false, includeMarked: false };
              setIncludeResolved(false);
              setIncludeMarked(false);
            },
          }}
          rowSelection={{
            selections: true,
            onChange: (_, rows) => setSel(rows),
          }}
          pagination={{ pageSize: 20, showSizeChanger: true }}
          scroll={{ x: 1680 }}
          tableAlertRender={false}
          request={async (params, sort, filter) => {
            const kw = typeof params.keyword === 'string' ? params.keyword.trim() : '';
            try {
              const qp: Record<string, string | number | undefined> = {
                page: params.current ?? 1,
                pageSize: params.pageSize ?? 20,
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
        onClose={() => setDetailOpen(false)}
        title="失败任务详情（摘要）"
      >
        {detailLoading ? <Typography.Paragraph type="secondary">载入中...</Typography.Paragraph> : null}
        {detail ? (
          <Space direction="vertical" style={{ width: '100%' }} size={12}>
            <Typography.Title level={5}>{detail.title}</Typography.Title>
            <div>{normTag(detail.normalizedStatus)}</div>
            <div>
              <Typography.Text strong>失败类别：</Typography.Text> {failureCategoryLabel(detail.failureCategory)}{' '}
              {severityCell(detail.severity)}
            </div>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>归类说明：</Typography.Text>
              <br />
              {detail.classificationReason || '—'}
            </Typography.Paragraph>
            <Typography.Paragraph style={{ marginBottom: 0 }} type="secondary">
              <Typography.Text strong>命中规则：</Typography.Text> {detail.matchedRule || '—'}
            </Typography.Paragraph>
            <Typography.Paragraph ellipsis={{ rows: 5, expandable: true }}>
              <Typography.Text strong>建议处理：</Typography.Text>
              <br />
              {detail.suggestedAction || '—'}
            </Typography.Paragraph>
            <div>
              <Typography.Text strong>告警状态：</Typography.Text> {alertStatusUi(detail.alertStatus)}{' '}
              {detail.relatedAlertId ? (
                <Typography.Link onClick={() => history.push('/ops/task-center/alerts')}>
                  （打开告警中心）
                </Typography.Link>
              ) : null}
            </div>
            <Typography.Paragraph ellipsis={{ rows: 4, expandable: true }}>
              <strong>错误摘要</strong>
              <br />
              {detail.errorMessage || '—'}
            </Typography.Paragraph>
            {detail.recoveryStatus || detail.extra?.recoveryStatus ? (
              <Typography.Paragraph style={{ marginBottom: 0 }}>
                <Typography.Text strong>恢复状态：</Typography.Text>{' '}
                {recoveryStatusLabel(
                  (detail.recoveryStatus || detail.extra?.recoveryStatus) as string | undefined,
                )}
              </Typography.Paragraph>
            ) : null}
            {typeof detail.extra?.userMessage === 'string' && detail.extra.userMessage ? (
              <Typography.Paragraph style={{ marginBottom: 0 }}>
                <Typography.Text strong>恢复说明：</Typography.Text> {detail.extra.userMessage}
              </Typography.Paragraph>
            ) : null}
            {detail.extra?.retryAttempts != null ? (
              <Typography.Paragraph style={{ marginBottom: 0 }} type="secondary">
                <Typography.Text strong>恢复尝试次数：</Typography.Text> {String(detail.extra.retryAttempts)}
              </Typography.Paragraph>
            ) : null}
            {typeof detail.extra?.lastErrorCode === 'string' && detail.extra.lastErrorCode ? (
              <Typography.Paragraph style={{ marginBottom: 0 }} type="secondary">
                <Typography.Text strong>最近错误码：</Typography.Text> {detail.extra.lastErrorCode}
              </Typography.Paragraph>
            ) : null}
            {detail.relatedResourceTitle ? (
              <Typography.Paragraph type="secondary">关联：{detail.relatedResourceTitle}</Typography.Paragraph>
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
                      message.success(res.message || '已打开淘宝/天猫采集浏览器');
                    } catch (e) {
                      message.error((e as Error).message);
                    } finally {
                      setTbLoginOpening(false);
                    }
                  }}
                >
                  打开淘宝/天猫采集浏览器
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
                      message.success(res.message || '已打开拼多多采集浏览器');
                    } catch (e) {
                      message.error((e as Error).message);
                    } finally {
                      setPddLoginOpening(false);
                    }
                  }}
                >
                  打开拼多多采集浏览器登录
                </Button>
              ) : null}
              {canRecoverDouyinDraft(detail) ? (
                <Button
                  type="primary"
                  loading={recovering}
                  onClick={() => {
                    Modal.confirm({
                      title: '尝试从抖店回查恢复草稿？',
                      content: '将通过 product.detail 确认平台侧是否已创建草稿，不会盲目重复提交 product.addV2。',
                      okText: '尝试恢复',
                      onOk: async () => {
                        setRecovering(true);
                        try {
                          await recoverDouyinDraftTask(detail.id);
                          message.success('已提交恢复，请刷新查看任务状态');
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
                  尝试恢复
                </Button>
              ) : null}
              {detail.retryable ? (
                <Button
                  onClick={() => {
                    Modal.confirm({
                      title: '重试该采集任务？',
                      okText: '重试',
                      onOk: async () => {
                        try {
                          await retryTaskFailure(detail.taskType, detail.id);
                          message.success('已提交重试');
                          actionRef.current?.reload?.();
                        } catch (e) {
                          message.error((e as Error).message);
                        }
                      },
                    });
                  }}
                >
                  重试任务
                </Button>
              ) : null}
              {typeof detail.extra?.sourceUrl === 'string' && detail.extra.sourceUrl ? (
                <Button
                  onClick={() => {
                    window.open(detail.extra!.sourceUrl as string, '_blank', 'noopener,noreferrer');
                  }}
                >
                  打开原始失败页面
                </Button>
              ) : null}
              {detail.taskType ? (
                <Button
                  onClick={() => {
                    Modal.confirm({
                      title: '生成站内告警记录',
                      onOk: async () => {
                        try {
                          await generateTaskFailureAlert(detail.taskType, detail.id);
                          message.success('已生成/刷新告警');
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
                  生成告警
                </Button>
              ) : null}
              {detail.detailUrl ? (
                <Button onClick={() => history.push(detail.detailUrl!)}>
                  打开任务详情
                </Button>
              ) : null}
            </Space>
            {detail.extra?.urlTypeLabel ? (
              <Typography.Paragraph style={{ marginBottom: 0 }}>
                <Typography.Text strong>链接类型：</Typography.Text>{' '}
                {String(detail.extra.urlTypeLabel)}
              </Typography.Paragraph>
            ) : null}
            {detail.extra?.accessStatusLabel ? (
              <Typography.Paragraph style={{ marginBottom: 0 }}>
                <Typography.Text strong>页面访问状态：</Typography.Text>{' '}
                {String(detail.extra.accessStatusLabel)}
              </Typography.Paragraph>
            ) : null}
            {detail.extra?.suggestedActionDetail ? (
              <Typography.Paragraph style={{ marginBottom: 0 }}>
                <Typography.Text strong>建议操作：</Typography.Text>{' '}
                {String(detail.extra.suggestedActionDetail)}
              </Typography.Paragraph>
            ) : null}
            {detail.extra && Object.keys(detail.extra).length > 0 ? (
              <TechnicalDetails>
                {detail.errorCode ? (
                  <TaskJsonBlock title="错误码" value={detail.errorCode} />
                ) : null}
                <TaskJsonBlock title="附加信息" value={detail.extra} last />
              </TechnicalDetails>
            ) : null}
          </Space>
        ) : null}
      </Drawer>
    </TmPageContainer>
  );
}
