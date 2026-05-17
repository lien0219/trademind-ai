import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, message, Modal, Tag, Typography } from 'antd';
import dayjs from 'dayjs';
import { useMemo, useRef } from 'react';
import type { TaskAlertDTO } from '@/services/taskCenter';
import {
  markTaskAlertHandled,
  markTaskAlertIgnored,
  queryTaskAlerts,
  scanTaskAlerts,
  unmarkTaskAlertRecord,
} from '@/services/taskCenter';

const SEVER_TAG: Record<string, { color: string; text?: string }> = {
  low: { color: 'default' },
  medium: { color: 'blue' },
  high: { color: 'orange' },
  critical: { color: 'red' },
};

function sevTag(sev: string) {
  const m = SEVER_TAG[sev];
  const label = sev.toUpperCase();
  if (!m) return <Tag>{label}</Tag>;
  return <Tag color={m.color}>{label}</Tag>;
}

const STATUS_META: Record<string, { color: string; text: string }> = {
  open: { color: 'gold', text: '待处理' },
  handled: { color: 'green', text: '已处理' },
  ignored: { color: 'default', text: '已忽略' },
};

export default function TaskCenterAlertsPage() {
  const actionRef = useRef<ActionType>();

  const columns: ProColumns<TaskAlertDTO>[] = useMemo(
    () => [
      {
        title: '时间范围',
        dataIndex: 'timeRange',
        valueType: 'dateTimeRange',
        hideInTable: true,
        search: {
          transform: ([start, end]: [unknown, unknown]) => ({
            start: start ? dayjs(start as string).toISOString() : undefined,
            end: end ? dayjs(end as string).toISOString() : undefined,
          }),
        },
      },
      {
        title: '最后出现',
        dataIndex: 'lastSeenAt',
        width: 160,
        search: false,
        render: (_, r) => dayjs(r.lastSeenAt).format('YYYY-MM-DD HH:mm'),
      },
      {
        title: '严重等级',
        dataIndex: 'severity',
        width: 104,
        valueType: 'select',
        valueEnum: {
          critical: { text: 'CRITICAL', status: 'Error' },
          high: { text: 'HIGH' },
          medium: { text: 'MEDIUM' },
          low: { text: 'LOW' },
        },
        render: (_, r) => sevTag(r.severity || ''),
      },
      {
        title: '类别',
        dataIndex: 'failureCategory',
        width: 144,
      },
      {
        title: '任务类型',
        dataIndex: 'taskType',
        width: 120,
      },
      {
        title: 'platform',
        dataIndex: 'platform',
        width: 90,
      },
      {
        title: '摘要',
        dataIndex: 'title',
        ellipsis: true,
        search: false,
      },
      {
        title: '次数',
        dataIndex: 'alertCount',
        width: 72,
        search: false,
      },
      {
        title: '状态',
        dataIndex: 'status',
        width: 96,
        valueType: 'select',
        valueEnum: Object.fromEntries(Object.keys(STATUS_META).map((k) => [k, { text: STATUS_META[k]?.text }])),
        render: (_, r) => {
          const sm = STATUS_META[r.status];
          if (!sm) return <Tag>{r.status}</Tag>;
          return <Tag color={sm.color}>{sm.text}</Tag>;
        },
      },
      {
        title: '建议',
        dataIndex: 'suggestedAction',
        ellipsis: true,
        search: false,
        width: 200,
      },
      {
        title: '操作',
        valueType: 'option',
        width: 220,
        fixed: 'right',
        render: (_, r) => (
          <>
            <Button
              size="small"
              type="link"
              onClick={() =>
                history.push(
                  `/task-center/failures?taskType=${encodeURIComponent(r.taskType)}&jumpId=${encodeURIComponent(r.sourceId)}`,
                )
              }
            >
              失败任务
            </Button>
            <Button
              size="small"
              type="link"
              disabled={r.status === 'handled'}
              onClick={() =>
                Modal.confirm({
                  title: '标记已处理（不修改原始任务状态）',
                  onOk: async () => {
                    try {
                      await markTaskAlertHandled(r.id);
                      message.success('已更新');
                      actionRef.current?.reload?.();
                    } catch (e) {
                      message.error((e as Error).message);
                    }
                  },
                })
              }
            >
              已处理
            </Button>
            <Button
              size="small"
              type="link"
              disabled={r.status === 'ignored'}
              onClick={() =>
                Modal.confirm({
                  title: '忽略此告警（仍可查看失败任务）',
                  onOk: async () => {
                    try {
                      await markTaskAlertIgnored(r.id);
                      message.success('已忽略告警');
                      actionRef.current?.reload?.();
                    } catch (e) {
                      message.error((e as Error).message);
                    }
                  },
                })
              }
            >
              忽略
            </Button>
            {(r.status === 'handled' || r.status === 'ignored') ? (
              <Button
                size="small"
                type="link"
                onClick={() =>
                  Modal.confirm({
                    title: '取消告警标记并重开为「待处理」',
                    onOk: async () => {
                      try {
                        await unmarkTaskAlertRecord(r.id);
                        message.success('已取消标记');
                        actionRef.current?.reload?.();
                      } catch (e) {
                        message.error((e as Error).message);
                      }
                    },
                  })
                }
              >
                取消标记
              </Button>
            ) : null}
          </>
        ),
      },
    ],
    [],
  );

  return (
    <PageContainer
      header={{
        title: '告警中心',
        subTitle: '站内告警（任务失败归类），不向外发送通知渠道',
      }}
      extra={[
        <Button
          key="scan"
          type="primary"
          onClick={() => {
            Modal.confirm({
              title: '根据当前规则扫描近期失败任务并生成/更新告警？',
              okText: '扫描',
              onOk: async () => {
                try {
                  const s = await scanTaskAlerts();
                  message.success(
                    `扫描 ${s.scannedCount} 条，新建 ${s.generatedCount}，更新 ${s.updatedCount}，跳过 ${s.ignoredCount}`,
                  );
                  actionRef.current?.reload?.();
                } catch (e) {
                  message.error((e as Error).message);
                }
              },
            });
          }}
        >
          扫描并生成告警
        </Button>,
      ]}
    >
      <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
        告警与「失败任务的忽略/处理」标记相互独立：处理告警不会自动恢复任务。
      </Typography.Paragraph>

      <ProTable<TaskAlertDTO>
        rowKey="id"
        columns={columns}
        actionRef={actionRef}
        search={{ layout: 'vertical' }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        scroll={{ x: 1200 }}
        request={async (params, _sort, _filter) => {
          try {
            const data = await queryTaskAlerts({
              page: params.current ?? 1,
              pageSize: params.pageSize ?? 20,
              status: (params.status as string | undefined)?.trim(),
              severity: (params.severity as string | undefined)?.trim(),
              failureCategory: (params.failureCategory as string | undefined)?.trim(),
              taskType: (params.taskType as string | undefined)?.trim(),
              platform: (params.platform as string | undefined)?.trim(),
              start: typeof params.start === 'string' ? params.start : undefined,
              end: typeof params.end === 'string' ? params.end : undefined,
            });
            return { data: data.list, total: data.total, success: true };
          } catch (e) {
            message.error((e as Error).message);
            return { data: [], total: 0, success: false };
          }
        }}
      />
    </PageContainer>
  );
}
