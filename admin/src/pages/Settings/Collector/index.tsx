import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Alert, Badge, Button, Form, Input, InputNumber, message, Space, Switch, Typography } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import {
  fetch1688AuthStatus,
  open1688LoginBrowser,
  type Provider1688AuthStatus,
  type Provider1688AuthStatusValue,
} from '@/services/collectAuth';
import { fetchSettingsList, saveSettingsItems } from '@/services/settings';
import { pickGroup, toPutItems, type FieldSpec } from '@/utils/settingsForm';

const GROUP = 'collector';

const FIELDS: Record<string, FieldSpec> = {
  main_service_url: {},
  collector_http_addr: {},
  goto_timeout_ms: {},
  headless: {},
  collect_batch_concurrency_1688: {},
  collect_batch_delay_min_ms_1688: {},
  collect_batch_delay_max_ms_1688: {},
  collect_batch_retry_on_blocked: {},
  collect_batch_retry_on_timeout: {},
  collect_batch_max_retries_1688: {},
};

type AuthDisplayStatus = 'checking' | Provider1688AuthStatusValue;

function resolveDisplayStatus(
  status: Provider1688AuthStatus | null,
  checking: boolean,
): AuthDisplayStatus {
  if (checking) return 'checking';
  if (!status) return 'unknown';
  if (status.status) return status.status;
  if (status.needVerification) return 'verification_required';
  if (status.loggedIn) return 'ok';
  if (status.message?.includes('异常')) return 'unknown';
  return 'not_logged_in';
}

const AUTH_STATUS_LABEL: Record<AuthDisplayStatus, { text: string; badge: 'processing' | 'success' | 'error' | 'warning' | 'default' }> = {
  checking: { text: '检测中…', badge: 'processing' },
  ok: { text: '已登录', badge: 'success' },
  not_logged_in: { text: '未登录', badge: 'error' },
  verification_required: { text: '需要安全验证', badge: 'warning' },
  unknown: { text: '检测异常', badge: 'default' },
};

function authStatusBadge(status: Provider1688AuthStatus | null, checking: boolean) {
  const key = resolveDisplayStatus(status, checking);
  const meta = AUTH_STATUS_LABEL[key];
  return <Badge status={meta.badge} text={meta.text} />;
}

export default function CollectorSettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [authStatus, setAuthStatus] = useState<Provider1688AuthStatus | null>(null);
  const [authChecking, setAuthChecking] = useState(false);
  const [loginOpening, setLoginOpening] = useState(false);

  const loadAuthStatus = useCallback(async () => {
    setAuthChecking(true);
    try {
      const data = await fetch1688AuthStatus();
      setAuthStatus(data);
    } catch (e: unknown) {
      setAuthStatus(null);
      message.error((e as Error)?.message || '1688 登录态检测失败');
    } finally {
      setAuthChecking(false);
    }
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const g = pickGroup(items, GROUP);
      form.setFieldsValue({
        main_service_url: g.main_service_url || 'http://127.0.0.1:8080',
        collector_http_addr: g.collector_http_addr || ':3100',
        goto_timeout_ms: g.goto_timeout_ms ? Number(g.goto_timeout_ms) : 45000,
        headless: g.headless === '0' || g.headless === 'false' ? false : true,
        collect_batch_concurrency_1688: g.collect_batch_concurrency_1688
          ? Number(g.collect_batch_concurrency_1688)
          : 1,
        collect_batch_delay_min_ms_1688: g.collect_batch_delay_min_ms_1688
          ? Number(g.collect_batch_delay_min_ms_1688)
          : 1500,
        collect_batch_delay_max_ms_1688: g.collect_batch_delay_max_ms_1688
          ? Number(g.collect_batch_delay_max_ms_1688)
          : 5000,
        collect_batch_retry_on_blocked:
          g.collect_batch_retry_on_blocked === undefined ||
          g.collect_batch_retry_on_blocked === '' ||
          g.collect_batch_retry_on_blocked === '1' ||
          g.collect_batch_retry_on_blocked === 'true',
        collect_batch_retry_on_timeout:
          g.collect_batch_retry_on_timeout === undefined ||
          g.collect_batch_retry_on_timeout === '' ||
          g.collect_batch_retry_on_timeout === '1' ||
          g.collect_batch_retry_on_timeout === 'true',
        collect_batch_max_retries_1688: g.collect_batch_max_retries_1688
          ? Number(g.collect_batch_max_retries_1688)
          : 2,
      });
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => {
    load();
    loadAuthStatus();
  }, [load, loadAuthStatus]);

  const handleOpenLoginBrowser = async () => {
    setLoginOpening(true);
    try {
      const result = await open1688LoginBrowser();
      message.success(result.message || '已打开采集浏览器');
      await loadAuthStatus();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '打开采集浏览器失败');
    } finally {
      setLoginOpening(false);
    }
  };

  return (
    <PageContainer title="采集服务">
      <ProCard
        title="1688 采集浏览器登录态"
        bordered
        style={{ marginBottom: 16 }}
        extra={
          <Space>
            <Button onClick={loadAuthStatus} loading={authChecking}>
              重新检测
            </Button>
            <Button type="primary" onClick={handleOpenLoginBrowser} loading={loginOpening}>
              打开采集浏览器登录 1688
            </Button>
          </Space>
        }
      >
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message="请注意：在普通 Chrome/Edge 中登录 1688 不会被采集器识别。请使用这里打开的采集浏览器完成登录。"
          />
          <div>
            <Typography.Text type="secondary">当前状态：</Typography.Text>{' '}
            {authStatusBadge(authStatus, authChecking)}
            {authChecking ? (
              <Typography.Text style={{ marginLeft: 12 }}>正在检测登录态…</Typography.Text>
            ) : authStatus?.message ? (
              <Typography.Text style={{ marginLeft: 12 }}>{authStatus.message}</Typography.Text>
            ) : null}
          </div>
          {authStatus?.lastCheckedAt ? (
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              上次检测：{authStatus.lastCheckedAt}
            </Typography.Text>
          ) : null}
        </Space>
      </ProCard>

      <ProCard
        title="通用采集浏览器 Profile"
        bordered
        style={{ marginBottom: 16 }}
        extra={
          <Button type="link" onClick={() => window.location.assign('/collect/browser-profiles')}>
            管理采集浏览器 Profile
          </Button>
        }
      >
        <Alert
          type="info"
          showIcon
          message="适用于自定义链接采集器"
          description={
            <ul style={{ margin: '8px 0 0', paddingLeft: 20 }}>
              <li>商品页需登录时，可创建 Profile，在可视化采集浏览器中手动登录后保存状态。</li>
              <li>系统不保存账号密码；Cookie 仅存于 Collector 本地目录，请勿在公共电脑使用。</li>
              <li>验证码 / 风控需用户自行完成，系统不提供破解能力。</li>
              <li>Docker 无头环境无法弹出登录窗口，本地开发请设置 COLLECTOR_HEADLESS=0。</li>
            </ul>
          }
        />
      </ProCard>

      <ProCard
        bordered
        extra={
          <Button type="link" onClick={load} disabled={loading}>
            重新加载
          </Button>
        }
      >
        <Form
          form={form}
          layout="vertical"
          style={{ maxWidth: 560 }}
          onFinish={async (values) => {
            try {
              const payload = {
                ...values,
                goto_timeout_ms: String(values.goto_timeout_ms ?? ''),
                headless: values.headless ? '1' : '0',
                collect_batch_concurrency_1688: String(values.collect_batch_concurrency_1688 ?? 1),
                collect_batch_delay_min_ms_1688: String(values.collect_batch_delay_min_ms_1688 ?? 1500),
                collect_batch_delay_max_ms_1688: String(values.collect_batch_delay_max_ms_1688 ?? 5000),
                collect_batch_retry_on_blocked: values.collect_batch_retry_on_blocked ? '1' : '0',
                collect_batch_retry_on_timeout: values.collect_batch_retry_on_timeout ? '1' : '0',
                collect_batch_max_retries_1688: String(values.collect_batch_max_retries_1688 ?? 2),
              };
              await saveSettingsItems(toPutItems(GROUP, FIELDS, payload));
              message.success('已保存');
              await load();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '保存失败');
            }
          }}
        >
          <Form.Item
            label="主服务 URL"
            name="main_service_url"
            rules={[{ required: true }]}
          >
            <Input placeholder="http://127.0.0.1:8080" />
          </Form.Item>
          <Form.Item
            label="采集服务监听地址"
            name="collector_http_addr"
            rules={[{ required: true }]}
          >
            <Input placeholder=":3100" />
          </Form.Item>
          <Form.Item label="页面打开超时（毫秒）" name="goto_timeout_ms" rules={[{ required: true }]}>
            <InputNumber min={1000} max={300000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="无头模式" name="headless" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item
            label="1688 批量并发上限"
            name="collect_batch_concurrency_1688"
            tooltip="仅批量采集生效；建议 1–2，过高易触发 1688 风控导致整批失败。"
          >
            <InputNumber min={1} max={2} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="1688 批量随机间隔最小（毫秒）" name="collect_batch_delay_min_ms_1688">
            <InputNumber min={0} max={120000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="1688 批量随机间隔最大（毫秒）" name="collect_batch_delay_max_ms_1688">
            <InputNumber min={0} max={120000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="批量遇风控/验证页时自动重试" name="collect_batch_retry_on_blocked" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item label="批量遇超时/导航失败时自动重试" name="collect_batch_retry_on_timeout" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item label="1688 批量任务最大自动重试次数" name="collect_batch_max_retries_1688">
            <InputNumber min={0} max={5} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading}>
              保存
            </Button>
          </Form.Item>
        </Form>
      </ProCard>
    </PageContainer>
  );
}
