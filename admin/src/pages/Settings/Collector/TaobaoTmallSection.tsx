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
  unchecked: { text: '未确认', badge: 'default' },
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
  const batchOpen = providerRow?.batchSupported && providerRow?.status === 'available';

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
              status={providerRow?.status === 'available' ? 'success' : 'processing'}
              text={providerRow?.status === 'available' ? '已可用' : '测试中（Beta）'}
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

        {batchOpen ? (
          <Alert
            type="info"
            showIcon
            message="批量采集说明"
            description="批量采集会逐条打开商品页面，默认每批最多 20 条。遇到登录或安全验证时，可先暂停批次、完成验证后再重试失败链接。"
          />
        ) : null}

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
        <Row gutter={16}>
          <Col xs={24} sm={12}>
            <Form.Item
              label="页面滚动等待"
              name="collect_taobao_tmall_scroll_wait_enabled"
              valuePropName="checked"
              tooltip="采集前轻微滚动页面，帮助主图与详情区域加载。"
            >
              <Switch defaultChecked />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12}>
            <Form.Item
              label="详情图加载等待（毫秒）"
              name="collect_taobao_tmall_detail_image_wait_ms"
              tooltip="滚动详情区域并等待懒加载图片，默认 3000ms。"
            >
              <InputNumber min={0} max={30000} style={{ width: '100%' }} placeholder="3000" />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} sm={12}>
            <Form.Item
              label="SKU 点击采集"
              name="collect_taobao_tmall_sku_click_enabled"
              valuePropName="checked"
              tooltip="规格组合较少时点击采集价格；组合过多时自动限制点击次数。"
            >
              <Switch defaultChecked />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12}>
            <Form.Item
              label="SKU 最大点击次数"
              name="collect_taobao_tmall_sku_click_max"
              tooltip="避免规格组合过多导致采集卡死，默认 24。"
            >
              <InputNumber min={1} max={48} style={{ width: '100%' }} placeholder="24" />
            </Form.Item>
          </Col>
        </Row>

        {batchOpen ? (
          <>
            <Typography.Title level={5} style={{ marginTop: 8, marginBottom: 0 }}>
              批量采集设置
            </Typography.Title>
            <Row gutter={16}>
              <Col xs={24} sm={12}>
                <Form.Item
                  label="每批最多链接数"
                  name="collect_taobao_tmall_batch_max_items"
                  tooltip="建议不超过 20 条，过多请分批提交。"
                >
                  <InputNumber min={1} max={50} style={{ width: '100%' }} placeholder="20" />
                </Form.Item>
              </Col>
              <Col xs={24} sm={12}>
                <Form.Item
                  label="同时采集条数"
                  name="collect_taobao_tmall_batch_concurrency"
                  tooltip="建议保持 1，最多 2，避免触发平台风控。"
                >
                  <InputNumber min={1} max={2} style={{ width: '100%' }} placeholder="1" />
                </Form.Item>
              </Col>
              <Col xs={24} sm={12}>
                <Form.Item
                  label="每条间隔最小值（毫秒）"
                  name="collect_taobao_tmall_batch_delay_min_ms"
                >
                  <InputNumber min={0} max={120000} style={{ width: '100%' }} placeholder="3500" />
                </Form.Item>
              </Col>
              <Col xs={24} sm={12}>
                <Form.Item
                  label="每条间隔最大值（毫秒）"
                  name="collect_taobao_tmall_batch_delay_max_ms"
                >
                  <InputNumber min={0} max={120000} style={{ width: '100%' }} placeholder="6000" />
                </Form.Item>
              </Col>
              <Col xs={24} sm={12}>
                <Form.Item label="批量失败重试次数" name="collect_taobao_tmall_batch_max_retries">
                  <InputNumber min={0} max={5} style={{ width: '100%' }} placeholder="2" />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item
              label="启用淘宝/天猫批量采集"
              name="collect_taobao_tmall_batch_enabled"
              valuePropName="checked"
            >
              <Switch defaultChecked />
            </Form.Item>
            <Form.Item
              label="遇到需要登录时暂停本批剩余任务"
              name="collect_taobao_tmall_batch_pause_on_login"
              valuePropName="checked"
            >
              <Switch defaultChecked />
            </Form.Item>
            <Form.Item
              label="遇到安全验证时暂停本批剩余任务"
              name="collect_taobao_tmall_batch_pause_on_verify"
              valuePropName="checked"
            >
              <Switch defaultChecked />
            </Form.Item>
          </>
        ) : null}
      </Space>
    </ProCard>
  );
}
