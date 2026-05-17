import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, Drawer, message, Modal, Table, Tag, Typography } from 'antd';
import dayjs from 'dayjs';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { fetchSettingsList } from '@/services/settings';
import { pickGroup } from '@/utils/settingsForm';
import type { TaskAlertDTO, TaskAlertNotificationDTO } from '@/services/taskCenter';
import {
  markTaskAlertHandled,
  markTaskAlertIgnored,
  notifyTaskAlert,
  queryAlertNotifications,
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

const NOTIFY_META: Record<string, { color: string; text: string }> = {
  none: { color: 'default', text: '未通知' },
  ok: { color: 'green', text: '已通知' },
  failed: { color: 'red', text: '通知失败' },
};

function parseChannelList(raw: string | undefined): string[] {
  const s = String(raw ?? '').trim();
  if (!s) return [];
  try {
    const arr = JSON.parse(s) as unknown;
    if (!Array.isArray(arr)) return [];
    return arr
      .map((x) => String(x).trim().toLowerCase())
      .filter((x) =>
        ['mail', 'webhook', 'feishu', 'wecom'].includes(x),
      );
  } catch {
    return [];
  }
}

export default function TaskCenterAlertsPage() {
  const actionRef = useRef<ActionType>();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerAlert, setDrawerAlert] = useState<TaskAlertDTO | null>(null);
  const [notifLoading, setNotifLoading] = useState(false);
  const [notifRows, setNotifRows] = useState<TaskAlertNotificationDTO[]>([]);
  const [configuredNotifyChannels, setConfiguredNotifyChannels] = useState<string[]>([]);

  const loadNotifyChannelConfig = useCallback(async () => {
    try {
      const { items } = await fetchSettingsList();
      const tc = pickGroup(items, 'taskcenter');
      setConfiguredNotifyChannels(parseChannelList(tc.notification_channels));
    } catch {
      setConfiguredNotifyChannels([]);
    }
  }, []);

  useEffect(() => {
    loadNotifyChannelConfig();
  }, [loadNotifyChannelConfig]);

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
        title: '外部通知',
        dataIndex: 'notificationStatus',
        width: 100,
        search: false,
        render: (_, r) => {
          const k = r.notificationStatus || 'none';
          const m = NOTIFY_META[k] ?? { color: 'default', text: k };
          return <Tag color={m.color}>{m.text}</Tag>;
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
        width: 300,
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
              onClick={async () => {
                setDrawerAlert(r);
                setDrawerOpen(true);
                setNotifLoading(true);
                try {
                  const res = await queryAlertNotifications({
                    alertId: r.id,
                    pageSize: 50,
                    page: 1,
                  });
                  setNotifRows(res.list ?? []);
                } catch (e) {
                  message.error((e as Error).message);
                  setNotifRows([]);
                } finally {
                  setNotifLoading(false);
                }
              }}
            >
              通知记录
            </Button>
            <Button
              size="small"
              type="link"
              onClick={() => {
                const chans = configuredNotifyChannels;
                if (!chans.length) {
                  message.warning('请先在「设置 → 告警通知配置」中填写 notification_channels（JSON 数组）');
                  return;
                }
                Modal.confirm({
                  title: `向以下通道触发一次手动通知？`,
                  content: `${chans.join(', ')}。仍走后端去重与 alert_notify 通道开关。`,
                  okText: '发送',
                  onOk: async () => {
                    try {
                      await notifyTaskAlert(r.id, chans);
                      message.success('已触发');
                      actionRef.current?.reload?.();
                    } catch (e) {
                      message.error((e as Error).message);
                    }
                  },
                });
              }}
            >
              发送通知
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
    [configuredNotifyChannels],
  );

  return (
    <PageContainer
      header={{
        title: '告警中心',
        subTitle: '站内告警与可选外部通知（邮件 / Webhook；飞书与企业微信预留）',
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
        scroll={{ x: 1400 }}
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
      <Drawer
        title={drawerAlert ? `通知记录 · ${drawerAlert.title}` : '通知记录'}
        width={560}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        destroyOnClose
      >
        <Table<TaskAlertNotificationDTO>
          loading={notifLoading}
          size="small"
          rowKey="id"
          dataSource={notifRows}
          pagination={false}
          columns={[
            { title: '通道', dataIndex: 'channel', width: 88 },
            { title: '状态', dataIndex: 'status', width: 88 },
            { title: '目标', dataIndex: 'target', ellipsis: true },
            {
              title: '时间',
              dataIndex: 'createdAt',
              width: 168,
              render: (t: string) => (t ? dayjs(t).format('MM-DD HH:mm:ss') : '—'),
            },
            { title: '摘要', dataIndex: 'errorMessage', ellipsis: true },
          ]}
        />
      </Drawer>
    </PageContainer>
  );
}
