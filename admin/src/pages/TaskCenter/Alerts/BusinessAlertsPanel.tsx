import {
  CheckOutlined,
  HistoryOutlined,
  LinkOutlined,
  ScanOutlined,
  SendOutlined,
  StopOutlined,
  UndoOutlined,
} from '@ant-design/icons';
import { type ActionType, type ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Alert, Button, Drawer, message, Modal, Space, Table, Tag } from 'antd';
import dayjs from 'dayjs';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { EmptyState, ErrorAlert, TmProTable as ProTable } from '@/components/ui';
import { notificationChannelLabel, parseNotificationChannels } from '@/constants/alertNotify';
import {
  failureCategoryLabel,
  resolveAlertRelatedLink,
  taskCenterTaskTypeLabel,
} from '@/constants/taskCenter';
import { fetchSettingsList } from '@/services/settings';
import { usePermission } from '@/hooks/usePermission';
import { mayReadGlobalSettings } from '@/utils/globalSettingsAccess';
import {
  markTaskAlertHandled,
  markTaskAlertIgnored,
  notifyTaskAlert,
  queryAlertNotifications,
  queryTaskAlerts,
  scanTaskAlerts,
  unmarkTaskAlertRecord,
  type TaskAlertDTO,
  type TaskAlertNotificationDTO,
} from '@/services/taskCenter';
import { formatDateTime } from '@/utils/formatTime';
import { pickGroup } from '@/utils/settingsForm';
import { platformLabel } from '@/constants/userFriendly';
import { createActionGuard } from './actionGuard';

const SEVERITY_META: Record<string, { color: string; text: string }> = {
  low: { color: 'default', text: '低' },
  medium: { color: 'blue', text: '中' },
  high: { color: 'orange', text: '高' },
  critical: { color: 'red', text: '严重' },
};

const STATUS_META: Record<string, { color: string; text: string }> = {
  open: { color: 'gold', text: '待处理' },
  handled: { color: 'green', text: '已处理' },
  ignored: { color: 'default', text: '已忽略' },
  resolved: { color: 'green', text: '已恢复' },
};

const NOTIFY_META: Record<string, { color: string; text: string }> = {
  none: { color: 'default', text: '未通知' },
  ok: { color: 'green', text: '已通知' },
  failed: { color: 'red', text: '通知失败' },
};

function severityTag(severity: string) {
  const key = (severity || '').trim().toLowerCase();
  const meta = SEVERITY_META[key];
  return <Tag color={meta?.color}>{meta?.text ?? (key || '-')}</Tag>;
}

export default function BusinessAlertsPanel() {
  const { user } = usePermission();
  const mayReadSettings = mayReadGlobalSettings(user);
  const actionRef = useRef<ActionType>();
  const guardRef = useRef(createActionGuard());
  const [pendingKeys, setPendingKeys] = useState<Set<string>>(() => new Set());
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerAlert, setDrawerAlert] = useState<TaskAlertDTO | null>(null);
  const [notificationLoading, setNotificationLoading] = useState(false);
  const [notificationError, setNotificationError] = useState('');
  const [notificationRows, setNotificationRows] = useState<TaskAlertNotificationDTO[]>([]);
  const [configuredChannels, setConfiguredChannels] = useState<string[]>([]);

  const setPending = useCallback((key: string, pending: boolean) => {
    setPendingKeys((current) => {
      const next = new Set(current);
      if (pending) next.add(key);
      else next.delete(key);
      return next;
    });
  }, []);

  const release = useCallback(
    (key: string) => {
      guardRef.current.unlock(key);
      setPending(key, false);
    },
    [setPending],
  );

  const confirmAction = useCallback(
    (options: {
      key: string;
      title: string;
      content?: string;
      okText?: string;
      danger?: boolean;
      action: () => Promise<unknown>;
      success?: string;
    }) => {
      if (!guardRef.current.tryLock(options.key)) return;
      setPending(options.key, true);
      let submitted = false;
      Modal.confirm({
        title: options.title,
        content: options.content,
        okText: options.okText ?? '确认',
        cancelText: '取消',
        okButtonProps: { danger: options.danger },
        onCancel: () => release(options.key),
        onOk: async () => {
          if (submitted) return;
          submitted = true;
          try {
            await options.action();
            if (options.success) message.success(options.success);
            actionRef.current?.reload?.();
            release(options.key);
          } catch (error) {
            submitted = false;
            setPending(options.key, false);
            message.error((error as Error).message || '操作失败');
            throw error;
          }
        },
      });
    },
    [release, setPending],
  );

  useEffect(() => {
    if (!mayReadSettings) {
      setConfiguredChannels([]);
      return;
    }
    void fetchSettingsList()
      .then(({ items }) => {
        const taskCenter = pickGroup(items, 'taskcenter');
        setConfiguredChannels(parseNotificationChannels(taskCenter.notification_channels));
      })
      .catch(() => setConfiguredChannels([]));
  }, [mayReadSettings]);

  const openNotificationHistory = useCallback(async (alert: TaskAlertDTO) => {
    setDrawerAlert(alert);
    setDrawerOpen(true);
    setNotificationLoading(true);
    setNotificationError('');
    try {
      const result = await queryAlertNotifications({ alertId: alert.id, pageSize: 50, page: 1 });
      setNotificationRows(result.list ?? []);
    } catch (error) {
      setNotificationRows([]);
      setNotificationError((error as Error).message || '通知记录加载失败');
    } finally {
      setNotificationLoading(false);
    }
  }, []);

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
        width: 168,
        search: false,
        render: (_, row) => formatDateTime(row.lastSeenAt),
      },
      {
        title: '严重等级',
        dataIndex: 'severity',
        width: 104,
        valueType: 'select',
        valueEnum: Object.fromEntries(
          Object.entries(SEVERITY_META).map(([key, meta]) => [key, { text: meta.text }]),
        ),
        render: (_, row) => severityTag(row.severity),
      },
      {
        title: '类别',
        dataIndex: 'failureCategory',
        width: 168,
        ellipsis: true,
        render: (_, row) => failureCategoryLabel(row.failureCategory),
      },
      {
        title: '任务类型',
        dataIndex: 'taskType',
        width: 120,
        render: (_, row) => taskCenterTaskTypeLabel(row.taskType),
      },
      {
        title: '平台',
        dataIndex: 'platform',
        width: 96,
        render: (_, row) => platformLabel(row.platform),
      },
      { title: '摘要', dataIndex: 'title', ellipsis: true, search: false },
      { title: '次数', dataIndex: 'alertCount', width: 72, search: false },
      {
        title: '状态',
        dataIndex: 'status',
        width: 96,
        valueType: 'select',
        valueEnum: Object.fromEntries(
          Object.entries(STATUS_META).map(([key, meta]) => [key, { text: meta.text }]),
        ),
        render: (_, row) => {
          const meta = STATUS_META[row.status];
          return <Tag color={meta?.color}>{meta?.text ?? row.status}</Tag>;
        },
      },
      {
        title: '外部通知',
        dataIndex: 'notificationStatus',
        width: 104,
        search: false,
        render: (_, row) => {
          const key = row.notificationStatus || 'none';
          const meta = NOTIFY_META[key] ?? { color: 'default', text: key };
          return <Tag color={meta.color}>{meta.text}</Tag>;
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
        width: 410,
        fixed: 'right',
        render: (_, row) => {
          const related = resolveAlertRelatedLink(row);
          const rowPending = [...pendingKeys].some((key) => key.endsWith(`:${row.id}`));
          return (
            <Space size={0} wrap>
              <Button
                size="small"
                type="link"
                icon={<LinkOutlined />}
                onClick={() => history.push(related.href)}
              >
                {related.label}
              </Button>
              <Button
                size="small"
                type="link"
                icon={<HistoryOutlined />}
                onClick={() => void openNotificationHistory(row)}
              >
                通知记录
              </Button>
              <Button
                size="small"
                type="link"
                icon={<SendOutlined />}
                disabled={rowPending || row.status !== 'open'}
                onClick={() => {
                  if (!configuredChannels.length) {
                    message.warning('请先在告警通知配置中启用通知渠道');
                    return;
                  }
                  confirmAction({
                    key: `notify:${row.id}`,
                    title: '发送告警通知',
                    content: `渠道：${configuredChannels.map(notificationChannelLabel).join('、')}`,
                    okText: '发送',
                    action: () => notifyTaskAlert(row.id, configuredChannels),
                    success: '通知已触发',
                  });
                }}
              >
                发送通知
              </Button>
              <Button
                size="small"
                type="link"
                icon={<CheckOutlined />}
                disabled={rowPending || row.status === 'handled'}
                onClick={() =>
                  confirmAction({
                    key: `handle:${row.id}`,
                    title: '标记为已处理',
                    content: '此操作不会修改原始任务状态。',
                    action: () => markTaskAlertHandled(row.id),
                    success: '已标记为已处理',
                  })
                }
              >
                已处理
              </Button>
              <Button
                size="small"
                type="link"
                icon={<StopOutlined />}
                disabled={rowPending || row.status === 'ignored'}
                onClick={() =>
                  confirmAction({
                    key: `ignore:${row.id}`,
                    title: '忽略此告警',
                    content: '原始失败任务仍会保留。',
                    okText: '忽略',
                    danger: true,
                    action: () => markTaskAlertIgnored(row.id),
                    success: '已忽略告警',
                  })
                }
              >
                忽略
              </Button>
              {row.status === 'handled' || row.status === 'ignored' ? (
                <Button
                  size="small"
                  type="link"
                  icon={<UndoOutlined />}
                  disabled={rowPending}
                  onClick={() =>
                    confirmAction({
                      key: `reopen:${row.id}`,
                      title: '恢复为待处理',
                      action: () => unmarkTaskAlertRecord(row.id),
                      success: '已恢复为待处理',
                    })
                  }
                >
                  恢复
                </Button>
              ) : null}
            </Space>
          );
        },
      },
    ],
    [configuredChannels, confirmAction, openNotificationHistory, pendingKeys],
  );

  const scanKey = 'scan:business';

  return (
    <>
      <Alert
        type="info"
        showIcon
        message="业务告警状态与原始任务状态相互独立"
        style={{ marginBottom: 16 }}
      />
      <ProTable<TaskAlertDTO>
        rowKey="id"
        columns={columns}
        actionRef={actionRef}
        search={{ layout: 'vertical' }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        scroll={{ x: 1600 }}
        locale={{ emptyText: <EmptyState compact title="暂无业务告警" /> }}
        toolBarRender={() => [
          <Button
            key="scan"
            type="primary"
            icon={<ScanOutlined />}
            loading={pendingKeys.has(scanKey)}
            onClick={() =>
              confirmAction({
                key: scanKey,
                title: '按当前规则扫描近期失败任务',
                okText: '扫描',
                action: async () => {
                  const summary = await scanTaskAlerts();
                  message.success(
                    `扫描 ${summary.scannedCount} 条，新建 ${summary.generatedCount}，更新 ${summary.updatedCount}，跳过 ${summary.ignoredCount}`,
                  );
                },
              })
            }
          >
            扫描业务告警
          </Button>,
        ]}
        request={async (params) => {
          try {
            const result = await queryTaskAlerts({
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
            return { data: result.list, total: result.total, success: true };
          } catch (error) {
            message.error((error as Error).message || '业务告警加载失败');
            return { data: [], total: 0, success: false };
          }
        }}
      />

      <Drawer
        title={drawerAlert ? `通知记录 · ${drawerAlert.title}` : '通知记录'}
        width={560}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        destroyOnHidden
      >
        {notificationError ? (
          <ErrorAlert
            title="通知记录加载失败"
            actionHint={notificationError}
            style={{ marginBottom: 16 }}
          />
        ) : null}
        <Table<TaskAlertNotificationDTO>
          loading={notificationLoading}
          size="small"
          rowKey="id"
          dataSource={notificationRows}
          pagination={false}
          locale={{ emptyText: <EmptyState compact title="暂无通知记录" /> }}
          scroll={{ x: 620 }}
          columns={[
            {
              title: '渠道',
              dataIndex: 'channel',
              width: 96,
              render: (channel: string) => notificationChannelLabel(channel),
            },
            { title: '状态', dataIndex: 'status', width: 88 },
            { title: '目标', dataIndex: 'target', ellipsis: true },
            {
              title: '时间',
              dataIndex: 'createdAt',
              width: 168,
              render: (value: string) => (value ? formatDateTime(value) : '-'),
            },
            { title: '结果摘要', dataIndex: 'errorMessage', ellipsis: true },
          ]}
        />
      </Drawer>
    </>
  );
}
