import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { history } from '@umijs/max';
import {
  Badge,
  Button,
  Drawer,
  Dropdown,
  Input,
  Modal,
  Row,
  Col,
  Space,
  Statistic,
  Tag,
  Typography,
  message,
} from 'antd';
import dayjs from 'dayjs';
import { useMemo, useRef, useState } from 'react';
import {
  batchHandleTaskFailures,
  batchIgnoreTaskFailures,
  batchRetryTaskFailures,
  getTaskFailureDetail,
  handleTaskFailure,
  ignoreTaskFailure,
  queryTaskFailures,
  retryTaskFailure,
  unmarkTaskFailure,
  type FailureDetailDTO,
  type UnifiedTaskDTO,
  type FailuresSummary,
} from '@/services/taskCenter';

const TASK_TYPE_LABEL: Record<string, string> = {
  collect: '采集',
  image: 'AI 图片',
  order_sync: '订单同步',
  customer_message_sync: '客服消息同步',
  product_publish: '商品刊登',
  inventory_sync: '库存同步',
};

const NORM_META: Record<string, { text: string; color: string }> = {
  failed: { text: '失败', color: 'error' },
  retrying: { text: '重试中', color: 'processing' },
  stale: { text: '停滞', color: 'warning' },
  lease_expired: { text: '租约过期', color: 'warning' },
  running: { text: '执行中', color: 'processing' },
  pending: { text: '排队', color: 'default' },
  success: { text: '成功', color: 'success' },
  cancelled: { text: '取消', color: 'default' },
};

function normTag(norm: string) {
  const m = NORM_META[norm];
  if (!m) return <Tag>{norm}</Tag>;
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
  const actionRef = useRef<ActionType>();
  const [summary, setSummary] = useState<FailuresSummary | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState<FailureDetailDTO | null>(null);
  const [sel, setSel] = useState<UnifiedTaskDTO[]>([]);

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
          options: Object.keys(TASK_TYPE_LABEL).map((k) => ({ label: TASK_TYPE_LABEL[k], value: k })),
          allowClear: true,
        },
        render: (_, r) => TASK_TYPE_LABEL[r.taskType] || r.taskType,
      },
      {
        title: '状态(归一)',
        dataIndex: 'normalizedStatus',
        width: 110,
        valueType: 'select',
        fieldProps: {
          options: Object.keys(NORM_META).map((k) => ({ label: NORM_META[k].text, value: k })),
          allowClear: true,
        },
        render: (_, r) => normTag(r.normalizedStatus),
      },
      {
        title: 'platform',
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
        title: '含已恢复',
        dataIndex: 'includeResolved',
        hideInTable: true,
        valueType: 'select',
        valueEnum: {
          '0': { text: '否' },
          '1': { text: '是' },
        },
      },
      {
        title: '含已标记忽略/处理',
        dataIndex: 'includeMarked',
        hideInTable: true,
        valueType: 'select',
        valueEnum: {
          '0': { text: '否' },
          '1': { text: '是' },
        },
      },
      {
        title: '创建时间',
        dataIndex: 'createdAt',
        width: 156,
        search: false,
        render: (_, r) => dayjs(r.createdAt).format('YYYY-MM-DD HH:mm'),
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
        title: '错误摘要',
        dataIndex: 'errorMessage',
        ellipsis: true,
        search: false,
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
        width: 220,
        fixed: 'right',
        render: (_, r) => (
          <Space wrap size={4}>
            <Button size="small" type="link" onClick={() => void openDetail(r)}>
              详情
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
    [],
  );

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
      content: `${TASK_TYPE_LABEL[row.taskType] || row.taskType} · ${row.id}`,
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

  function batchMenus() {
    const rows = sel.slice(0, 50);
    return (
      <Space wrap>
        <Button
          disabled={!rows.length}
          onClick={() => {
            Modal.confirm({
              title: `批量重试(${rows.length} 条，最多 50)？`,
              onOk: async () => {
                try {
                  const res = await batchRetryTaskFailures(
                    rows.map((r) => ({ taskType: r.taskType, id: r.id })),
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
          disabled={!rows.length}
          onClick={() =>
            Modal.confirm({
              title: `批量忽略 ${rows.length} 条任务？`,
              onOk: async () => {
                try {
                  const res = await batchIgnoreTaskFailures(
                    rows.map((r) => ({ taskType: r.taskType, id: r.id })),
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
          disabled={!rows.length}
          type="primary"
          ghost
          onClick={() =>
            Modal.confirm({
              title: `批量标记已处理 (${rows.length} 条)？`,
              onOk: async () => {
                try {
                  const res = await batchHandleTaskFailures(
                    rows.map((r) => ({ taskType: r.taskType, id: r.id })),
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
      </Space>
    );
  }

  return (
    <PageContainer header={{ title: '失败任务中心', subTitle: '聚合展示各模块失败态任务（统一重试入口）' }}>
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {summary ? (
          <Space wrap size={24}>
            <Statistic title="失败(归一 Failed)" value={summary.totalFailed} />
            <Statistic title="可重试" value={summary.retryableCount} />
            <Statistic title="重试中 / 停滞 / 租约过期" value={`${summary.retryingTotal}/${summary.staleTotal}/${summary.leaseExpiredTotal}`} />
            {summary.latestFailedAt ? (
              <Statistic title="最近失败时间" value={summary.latestFailedAt} />
            ) : null}
            <Statistic title="忽略标记总数" value={summary.ignoredCount} />
            <Statistic title="已处理标记总数" value={summary.handledCount} />
            <Typography.Link onClick={() => history.push('/workers/monitor')} strong>
              Worker 监控 →
            </Typography.Link>
          </Space>
        ) : (
          <Badge status="processing" text="载入统计..." />
        )}

        {batchMenus()}

        {summary && Object.keys(summary.byType || {}).length ? (
          <Row gutter={[8, 8]}>
            {Object.entries(summary.byType || {}).map(([k, v]) => (
              <Col key={k}>
                <Tag>
                  {(TASK_TYPE_LABEL[k] || k) + `: ${v}`}
                </Tag>
              </Col>
            ))}
          </Row>
        ) : null}

        <ProTable<UnifiedTaskDTO>
          rowKey={(r) => `${r.taskType}:${r.id}`}
          columns={columns}
          actionRef={actionRef}
          rowSelection={{
            selections: true,
            onChange: (_, rows) => setSel(rows),
          }}
          pagination={{ pageSize: 20, showSizeChanger: true }}
          scroll={{ x: 1300 }}
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
              };
              if (params.includeResolved === '1') qp.includeResolved = 'true';
              if (params.includeMarked === '1') qp.includeMarked = 'true';
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
        width={560}
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
        title="失败任务详情（摘要）"
      >
        {detailLoading ? <Typography.Paragraph type="secondary">载入中...</Typography.Paragraph> : null}
        {detail ? (
          <Space direction="vertical" style={{ width: '100%' }} size={12}>
            <Typography.Title level={5}>{detail.title}</Typography.Title>
            <div>{normTag(detail.normalizedStatus)}</div>
            <Typography.Paragraph ellipsis={{ rows: 4, expandable: true }}>
              <strong>错误</strong>
              <br />
              {detail.errorMessage || '—'}
            </Typography.Paragraph>
            {detail.relatedResourceTitle ? (
              <Typography.Paragraph type="secondary">关联：{detail.relatedResourceTitle}</Typography.Paragraph>
            ) : null}
            {detail.extra && Object.keys(detail.extra).length > 0 ? (
              <pre style={{ whiteSpace: 'pre-wrap', fontSize: 12, margin: 0 }}>
                {JSON.stringify(detail.extra, null, 2)}
              </pre>
            ) : null}
            {detail.detailUrl ? (
              <Button type="primary" onClick={() => history.push(detail.detailUrl!)}>
                打开原模块页面
              </Button>
            ) : null}
          </Space>
        ) : null}
      </Drawer>
    </PageContainer>
  );
}
