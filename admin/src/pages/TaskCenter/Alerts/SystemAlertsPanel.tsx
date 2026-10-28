import { tr } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { CheckOutlined, NotificationOutlined } from '@ant-design/icons';
import { type ActionType, type ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Form, Input, InputNumber, message, Modal, Space, Tag } from 'antd';
import { useCallback, useMemo, useRef, useState } from 'react';
import { EmptyState, ErrorAlert, TmProTable as ProTable } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import {
  acknowledgeAlert,
  fetchObservabilityAlerts,
  silenceAlert,
  type AlertEvent,
} from '@/services/observability';
import { formatDateTime } from '@/utils/formatTime';
import { PERMISSIONS } from '@/utils/permission';
import { createActionGuard } from './actionGuard';
import {
  SYSTEM_ALERT_SEVERITY_META,
  SYSTEM_ALERT_STATUS_META,
  systemAlertSeverityMeta,
  systemAlertStatusMeta,
} from './alertMeta';

type SilenceForm = {
  reason: string;
  durationHours: number;
};

export default function SystemAlertsPanel() {
  const language = useLanguage();
  const actionRef = useRef<ActionType>();
  const guardRef = useRef(createActionGuard());
  const { can } = usePermission();
  const [form] = Form.useForm<SilenceForm>();
  const [loadError, setLoadError] = useState('');
  const [pendingKeys, setPendingKeys] = useState<Set<string>>(() => new Set());
  const [silenceTarget, setSilenceTarget] = useState<AlertEvent | null>(null);

  const canAcknowledge = can(PERMISSIONS.ALERTS_ACK);
  const canSilence = can(PERMISSIONS.ALERTS_SILENCE);

  const setPending = useCallback((key: string, pending: boolean) => {
    setPendingKeys((current) => {
      const next = new Set(current);
      if (pending) next.add(key);
      else next.delete(key);
      return next;
    });
  }, []);

  const runGuarded = useCallback(
    async (key: string, action: () => Promise<unknown>) => {
      if (!guardRef.current.tryLock(key)) return false;
      setPending(key, true);
      try {
        await action();
        actionRef.current?.reload?.();
        return true;
      } catch (error) {
        message.error((error as Error).message || tr('操作失败'));
        throw error;
      } finally {
        guardRef.current.unlock(key);
        setPending(key, false);
      }
    },
    [setPending],
  );

  const confirmAcknowledge = useCallback(
    (alert: AlertEvent) => {
      const key = `ack:${alert.id}`;
      if (!guardRef.current.tryLock(key)) return;
      setPending(key, true);
      let submitted = false;
      Modal.confirm({
        title: tr("确认系统告警"),
        content: tr('确认仅表示运维人员已关注，不会自动恢复故障。'),
        okText: tr("确认告警"),
        cancelText: tr("取消"),
        onCancel: () => {
          guardRef.current.unlock(key);
          setPending(key, false);
        },
        onOk: async () => {
          if (submitted) return;
          submitted = true;
          try {
            await acknowledgeAlert(alert.id);
            message.success(tr("告警已确认"));
            actionRef.current?.reload?.();
            guardRef.current.unlock(key);
            setPending(key, false);
          } catch (error) {
            submitted = false;
            setPending(key, false);
            message.error((error as Error).message || tr('操作失败'));
            throw error;
          }
        },
      });
    },
    [setPending],
  );

  const columns: ProColumns<AlertEvent>[] = useMemo(
    () => [
      {
        title: tr("最后出现"),
        dataIndex: 'lastSeenAt',
        width: 168,
        search: false,
        render: (_, row) => formatDateTime(row.lastSeenAt),
      },
      {
        title: tr("级别"),
        dataIndex: 'severity',
        width: 104,
        valueType: 'select',
        valueEnum: Object.fromEntries(
          Object.entries(SYSTEM_ALERT_SEVERITY_META).map(([key, meta]) => [key, { text: meta.text }]),
        ),
        render: (_, row) => {
          const meta = systemAlertSeverityMeta(row.severity);
          return <Tag color={meta.color}>{meta.text}</Tag>;
        },
      },
      {
        title: tr("状态"),
        dataIndex: 'status',
        width: 112,
        valueType: 'select',
        valueEnum: Object.fromEntries(
          Object.entries(SYSTEM_ALERT_STATUS_META).map(([key, meta]) => [key, { text: meta.text }]),
        ),
        render: (_, row) => {
          const meta = systemAlertStatusMeta(row.status);
          return <Tag color={meta.color}>{meta.text}</Tag>;
        },
      },
      { title: tr("模块"), dataIndex: 'module', width: 136, ellipsis: true },
      { title: tr("规则"), dataIndex: 'ruleId', width: 208, ellipsis: true, search: false },
      { title: tr("摘要"), dataIndex: 'summary', ellipsis: true, search: false },
      { title: tr("次数"), dataIndex: 'occurrenceCount', width: 72, search: false },
      {
        title: tr("操作"),
        valueType: 'option',
        width: 176,
        fixed: 'right',
        hideInTable: !canAcknowledge && !canSilence,
        render: (_, row) => {
          const rowPending = [...pendingKeys].some((key) => key.endsWith(`:${row.id}`));
          const actionable = row.status === 'firing' || row.status === 'acknowledged';
          return (
            <Space size={0} wrap>
              {canAcknowledge ? (
                <Button
                  size="small"
                  type="link"
                  icon={<CheckOutlined />}
                  loading={pendingKeys.has(`ack:${row.id}`)}
                  disabled={rowPending || row.status !== 'firing'}
                  onClick={() => confirmAcknowledge(row)}
                >
                  {tr("确认")}
                </Button>
              ) : null}
              {canSilence ? (
                <Button
                  size="small"
                  type="link"
                  icon={<NotificationOutlined />}
                  disabled={rowPending || !actionable}
                  onClick={() => {
                    form.setFieldsValue({ reason: '', durationHours: 4 });
                    setSilenceTarget(row);
                  }}
                >
                  {tr("静默")}
                </Button>
              ) : null}
            </Space>
          );
        },
      },
    ],
    [canAcknowledge, canSilence, confirmAcknowledge, form, language, pendingKeys],
  );

  const silenceKey = silenceTarget ? `silence:${silenceTarget.id}` : '';

  return (
    <>
      {!canAcknowledge && !canSilence ? (
        <Alert
          type="info"
          showIcon
          message={tr("当前账号可查看系统告警，处置操作需要单独授权")}
          style={{ marginBottom: 16 }}
        />
      ) : null}
      {loadError ? (
        <ErrorAlert
          title={tr("系统告警加载失败")}
          actionHint={tr("请检查可观测性服务状态后重试。")}
          style={{ marginBottom: 16 }}
        />
      ) : null}
      <ProTable<AlertEvent>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ layout: 'vertical' }}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        scroll={{ x: 1080 }}
        locale={{ emptyText: <EmptyState compact title={tr("暂无系统告警")} /> }}
        request={async (params) => {
          try {
            const result = await fetchObservabilityAlerts({
              page: params.current ?? 1,
              pageSize: params.pageSize ?? 20,
              status: (params.status as string | undefined)?.trim(),
              severity: (params.severity as string | undefined)?.trim(),
              module: (params.module as string | undefined)?.trim(),
            });
            setLoadError('');
            return { data: result.items, total: result.pagination.total, success: true };
          } catch (error) {
            setLoadError((error as Error).message || tr('系统告警加载失败'));
            return { data: [], total: 0, success: false };
          }
        }}
      />

      <Modal
        title={tr("静默系统告警")}
        open={!!silenceTarget}
        okText={tr("确认静默")}
        cancelText={tr("取消")}
        confirmLoading={!!silenceKey && pendingKeys.has(silenceKey)}
        maskClosable={false}
        destroyOnHidden
        onCancel={() => {
          if (!silenceKey || !pendingKeys.has(silenceKey)) setSilenceTarget(null);
        }}
        onOk={async () => {
          if (!silenceTarget) return;
          const values = await form.validateFields();
          const completed = await runGuarded(`silence:${silenceTarget.id}`, () =>
            silenceAlert(silenceTarget.id, values),
          );
          if (completed) {
            message.success(tr("告警已静默"));
            setSilenceTarget(null);
            form.resetFields();
          }
        }}
      >
        <Alert
          type="warning"
          showIcon
          message={tr("静默期间同一规则的新告警会被抑制")}
          style={{ marginBottom: 16 }}
        />
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item
            name="reason"
            label={tr("静默原因")}
            rules={[
              { required: true, whitespace: true, message: tr("请输入静默原因") },
              { max: 256, message: tr("静默原因不能超过 256 个字符") },
            ]}
          >
            <Input.TextArea rows={3} maxLength={256} showCount />
          </Form.Item>
          <Form.Item
            name="durationHours"
            label={tr("静默时长（小时）")}
            rules={[{ required: true, message: tr("请输入静默时长") }]}
          >
            <InputNumber min={1} max={720} precision={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
