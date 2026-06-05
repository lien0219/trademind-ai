import { ProCard } from '@ant-design/pro-components';
import { formatDateTime } from '@/utils/formatTime';
import {
  Alert,
  Badge,
  Button,
  Col,
  Form,
  Input,
  InputNumber,
  Row,
  Space,
  Switch,
  Typography,
} from 'antd';
import type { CollectProviderRow } from '@/services/collectProviders';
import type { ProviderTaobaoTmallAuthStatus } from '@/services/collectAuth';

type AuthDisplayStatus = 'unchecked' | 'checking' | ProviderTaobaoTmallAuthStatus['status'];

const AUTH_STATUS_LABEL: Record<
  AuthDisplayStatus,
  { text: string; badge: 'processing' | 'success' | 'error' | 'warning' | 'default' }
> = {
  unchecked: { text: '未检测', badge: 'default' },
  checking: { text: '检测中…', badge: 'processing' },
  logged_in: { text: '已登录', badge: 'success' },
  login_required: { text: '需要登录', badge: 'error' },
  verify_required: { text: '需要验证', badge: 'warning' },
  unknown: { text: '检测异常', badge: 'default' },
};

function resolveDisplayStatus(
  status: ProviderTaobaoTmallAuthStatus | null,
  checking: boolean,
  loaded: boolean,
): AuthDisplayStatus {
  if (checking) return 'checking';
  if (!loaded) return 'unchecked';
  if (!status) return 'unknown';
  if (status.status) return status.status;
  if (status.needVerification) return 'verify_required';
  if (status.loggedIn) return 'logged_in';
  return 'login_required';
}

function authStatusBadge(
  status: ProviderTaobaoTmallAuthStatus | null,
  checking: boolean,
  loaded: boolean,
) {
  const key = resolveDisplayStatus(status, checking, loaded);
  const meta = AUTH_STATUS_LABEL[key];
  return <Badge status={meta.badge} text={meta.text} />;
}

type Props = {
  providerRow?: CollectProviderRow;
  authStatus: ProviderTaobaoTmallAuthStatus | null;
  authChecking: boolean;
  authLoaded: boolean;
  loginOpening: boolean;
  onRecheck: () => void;
  onOpenLogin: () => void;
};

export function CollectorTaobaoTmallSection({
  providerRow,
  authStatus,
  authChecking,
  authLoaded,
  loginOpening,
  onRecheck,
  onOpenLogin,
}: Props) {
  const authKey = resolveDisplayStatus(authStatus, authChecking, authLoaded);

  return (
    <ProCard
      title="淘宝/天猫专属配置"
      bordered
      className="tm-collector-settings__panel"
      extra={
        <Space wrap size="small" className="tm-action-space">
          <Button size="small" onClick={onRecheck} loading={authChecking}>
            重新检测
          </Button>
          <Button size="small" type="primary" onClick={onOpenLogin} loading={loginOpening}>
            打开淘宝/天猫采集浏览器
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <div className="tm-collector-auth-panel">
          <Space wrap>
            <Typography.Text strong>淘宝/天猫采集器状态</Typography.Text>
            <Badge
              status={providerRow?.status === 'beta' ? 'processing' : 'success'}
              text={providerRow?.status === 'beta' ? '测试中（Beta）' : '已可用'}
            />
          </Space>
        </div>

        <div className={`tm-collector-auth-panel tm-collector-auth-panel--${authKey}`}>
          <Space direction="vertical" size={4} style={{ width: '100%' }}>
            <Space wrap>
              <Typography.Text strong>登录状态</Typography.Text>
              {authStatusBadge(authStatus, authChecking, authLoaded)}
            </Space>
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
              部分淘宝/天猫商品需要登录后才能采集。如遇安全验证或滑块，请在采集浏览器中手动完成后再重试。
            </Typography.Paragraph>
            {authStatus?.message ? (
              <Typography.Text type="secondary">{authStatus.message}</Typography.Text>
            ) : null}
            {authStatus?.lastCheckedAt ? (
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                上次检测：{formatDateTime(authStatus.lastCheckedAt)}
              </Typography.Text>
            ) : null}
          </Space>
        </div>

        <Alert type="warning" showIcon message="批量采集暂未开放，当前仅支持单品采集（Beta）。" />

        <Form.Item
          label="用于检测的商品链接（可选）"
          name="collect_taobao_tmall_auth_check_url"
          tooltip="填写淘宝/天猫商品详情页链接后，「重新检测」将打开该页面判断登录态。"
        >
          <Input placeholder="https://item.taobao.com/item.htm?id=..." />
        </Form.Item>

        <Row gutter={16}>
          <Col xs={24} sm={12}>
            <Form.Item
              label="页面打开超时（毫秒）"
              name="collect_taobao_tmall_timeout_ms"
              tooltip="留空时使用通用「页面打开超时」，默认 45000ms。"
            >
              <InputNumber min={0} max={300000} style={{ width: '100%' }} placeholder="45000" />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12}>
            <Form.Item
              label="启用可访问状态检测"
              name="collect_taobao_tmall_access_check_enabled"
              valuePropName="checked"
            >
              <Switch defaultChecked />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} sm={12}>
            <Form.Item
              label="失败自动重试"
              name="collect_taobao_tmall_retry_on_failure"
              valuePropName="checked"
            >
              <Switch defaultChecked />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12}>
            <Form.Item label="最大重试次数" name="collect_taobao_tmall_max_retries">
              <InputNumber min={0} max={10} style={{ width: '100%' }} placeholder="2" />
            </Form.Item>
          </Col>
        </Row>
      </Space>
    </ProCard>
  );
}
