import { PageContainer, ProCard } from '@ant-design/pro-components';
import { history, useLocation } from '@umijs/max';
import {
  Alert,
  Badge,
  Button,
  Col,
  Divider,
  Empty,
  Form,
  Input,
  InputNumber,
  Row,
  Space,
  Switch,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import type { CollectProviderRow } from '@/services/collectProviders';
import { queryCollectProviders } from '@/services/collectProviders';
import { collectProviderStatusPresentation } from '@/utils/collectProviderStatus';
import {
  checkPinduoduoLogin,
  fetch1688AuthStatus,
  open1688LoginBrowser,
  openPinduoduoLoginBrowser,
  type Provider1688AuthStatus,
  type Provider1688AuthStatusValue,
  type ProviderPinduoduoAuthStatus,
  type ProviderPinduoduoAuthStatusValue,
} from '@/services/collectAuth';
import { fetchSettingsList, saveSettingsItems } from '@/services/settings';
import {
  COLLECT_SETTINGS_PROVIDER_OPTIONS,
  findCollectSettingsOption,
  isPlannedCollectProvider,
  resolveCollectSettingsProvider,
  type CollectSettingsProviderKey,
} from '@/utils/collectSettingsProvider';
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
  collect_custom_access_check_enabled: {},
  collect_custom_profile_enabled: {},
  collect_custom_batch_enabled: {},
  collect_rule_ai_enabled: {},
  collect_aliexpress_timeout_ms: {},
  collect_aliexpress_retry_on_timeout: {},
  collect_aliexpress_batch_enabled: {},
  collect_pinduoduo_timeout_ms: {},
  collect_pinduoduo_auth_check_url: {},
  collect_pinduoduo_access_check_enabled: {},
  collect_pinduoduo_retry_on_timeout: {},
  collect_pinduoduo_batch_enabled: {},
};

const PROVIDER_CARD_DESC: Record<CollectSettingsProviderKey, string> = {
  '1688': '登录态检测与批量采集节流',
  aliexpress: 'Beta 单条采集与重试策略',
  pinduoduo: 'Beta 单链接采集与访问检测',
  taobao: '暂未开放，预留配置入口',
  shein_temu: '暂未开放，预留配置入口',
  custom: '登录状态、采集规则与页面访问检测',
};

type AuthDisplayStatus = 'unchecked' | 'checking' | Provider1688AuthStatusValue;
type PddAuthDisplayStatus = 'unchecked' | 'checking' | ProviderPinduoduoAuthStatusValue;

function resolveDisplayStatus(
  status: Provider1688AuthStatus | null,
  checking: boolean,
  loaded = true,
): AuthDisplayStatus {
  if (checking) return 'checking';
  if (!loaded) return 'unchecked';
  if (!status) return 'unknown';
  if (status.status) return status.status;
  if (status.needVerification) return 'verification_required';
  if (status.loggedIn) return 'ok';
  if (status.message?.includes('异常')) return 'unknown';
  return 'not_logged_in';
}

function resolvePddDisplayStatus(
  status: ProviderPinduoduoAuthStatus | null,
  checking: boolean,
  loaded: boolean,
): PddAuthDisplayStatus {
  if (checking) return 'checking';
  if (!loaded) return 'unchecked';
  if (!status) return 'unknown';
  if (status.status) return status.status;
  if (status.needVerification) return 'verification_required';
  if (status.loggedIn) return 'ok';
  return 'not_logged_in';
}

const AUTH_STATUS_LABEL: Record<
  AuthDisplayStatus,
  { text: string; badge: 'processing' | 'success' | 'error' | 'warning' | 'default' }
> = {
  unchecked: { text: '未检测', badge: 'default' },
  checking: { text: '检测中…', badge: 'processing' },
  ok: { text: '已登录', badge: 'success' },
  not_logged_in: { text: '未登录', badge: 'error' },
  verification_required: { text: '需要验证', badge: 'warning' },
  unknown: { text: '检测异常', badge: 'default' },
};

const PDD_AUTH_STATUS_LABEL: Record<
  PddAuthDisplayStatus,
  { text: string; badge: 'processing' | 'success' | 'error' | 'warning' | 'default' }
> = {
  unchecked: { text: '未检测', badge: 'default' },
  checking: { text: '检测中…', badge: 'processing' },
  ok: { text: '已登录', badge: 'success' },
  not_logged_in: { text: '需要登录', badge: 'error' },
  wechat_auth_required: { text: '需要微信扫码授权', badge: 'warning' },
  app_redirect: { text: 'App 引导页', badge: 'warning' },
  verification_required: { text: '需要验证', badge: 'warning' },
  homepage_only: { text: '只能访问首页，无法确认是否已登录', badge: 'warning' },
  unknown: { text: '暂时无法确认登录状态', badge: 'default' },
};

function authStatusBadge(
  status: Provider1688AuthStatus | null,
  checking: boolean,
  loaded = true,
) {
  const key = resolveDisplayStatus(status, checking, loaded);
  const meta = AUTH_STATUS_LABEL[key];
  return <Badge status={meta.badge} text={meta.text} />;
}

function pddAuthStatusBadge(
  status: ProviderPinduoduoAuthStatus | null,
  checking: boolean,
  loaded: boolean,
) {
  const key = resolvePddDisplayStatus(status, checking, loaded);
  const meta = PDD_AUTH_STATUS_LABEL[key];
  return <Badge status={meta.badge} text={meta.text} />;
}

function parseBoolSetting(v: string | undefined, defaultTrue = true): boolean {
  if (v === undefined || v === '') return defaultTrue;
  return v === '1' || v === 'true';
}

function providerStatusTag(row?: CollectProviderRow) {
  if (!row?.status) return null;
  const tag = collectProviderStatusPresentation(row.source, row.status);
  return <Tag color={tag.color}>{tag.text}</Tag>;
}

function CollectorProviderSelector({
  activeKey,
  providers,
  onChange,
}: {
  activeKey: CollectSettingsProviderKey;
  providers: CollectProviderRow[];
  onChange: (key: CollectSettingsProviderKey) => void;
}) {
  return (
    <div className="tm-collector-provider-grid">
      {COLLECT_SETTINGS_PROVIDER_OPTIONS.map((option) => {
        const row = providers.find((p) => p.source.toLowerCase() === option.source.toLowerCase());
        const planned = row ? row.status === 'planned' : !!option.planned;
        return (
          <div
            key={option.key}
            role="button"
            tabIndex={0}
            className={[
              'tm-collector-provider-card',
              activeKey === option.key ? 'is-active' : '',
              planned ? 'is-planned' : '',
            ]
              .filter(Boolean)
              .join(' ')}
            onClick={() => onChange(option.key)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                onChange(option.key);
              }
            }}
          >
            <div className="tm-collector-provider-card__head">
              <span className="tm-collector-provider-card__title">{option.label}</span>
              {providerStatusTag(row) ?? (planned ? <Tag>规划中</Tag> : null)}
            </div>
            <span className="tm-collector-provider-card__desc">{PROVIDER_CARD_DESC[option.key]}</span>
          </div>
        );
      })}
    </div>
  );
}

function Collector1688Section({
  authStatus,
  authChecking,
  loginOpening,
  onRecheck,
  onOpenLogin,
}: {
  authStatus: Provider1688AuthStatus | null;
  authChecking: boolean;
  loginOpening: boolean;
  onRecheck: () => void;
  onOpenLogin: () => void;
}) {
  const authKey = resolveDisplayStatus(authStatus, authChecking);

  return (
    <ProCard
      title="1688 专属配置"
      bordered
      className="tm-collector-settings__panel"
      extra={
        <Space wrap size="small" className="tm-action-space">
          <Button size="small" onClick={onRecheck} loading={authChecking}>
            重新检测
          </Button>
          <Button size="small" type="primary" onClick={onOpenLogin} loading={loginOpening}>
            打开采集浏览器登录
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <div className={`tm-collector-auth-panel tm-collector-auth-panel--${authKey}`}>
          <Space direction="vertical" size={4} style={{ width: '100%' }}>
            <Space wrap>
              <Typography.Text strong>1688 登录状态</Typography.Text>
              {authStatusBadge(authStatus, authChecking)}
            </Space>
            {authChecking ? (
              <Typography.Text type="secondary">正在检测登录态…</Typography.Text>
            ) : authStatus?.message ? (
              <Typography.Text type="secondary">{authStatus.message}</Typography.Text>
            ) : (
              <Typography.Text type="secondary">
                请在普通浏览器外，使用采集专用浏览器完成 1688 登录。
              </Typography.Text>
            )}
            {authStatus?.lastCheckedAt ? (
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                上次检测：{authStatus.lastCheckedAt}
              </Typography.Text>
            ) : null}
          </Space>
        </div>

        <Alert
          type="info"
          showIcon
          message="在 Chrome / Edge 中登录 1688 不会被采集器识别，请使用上方按钮打开专用浏览器。"
        />

        <Typography.Title level={5} className="tm-collector-settings__section-title">
          批量采集节流
        </Typography.Title>
        <Row gutter={16}>
          <Col xs={24} sm={12}>
            <Form.Item
              label="并发上限"
              name="collect_batch_concurrency_1688"
              tooltip="仅 1688 批量采集生效；建议 1–2，过高易触发风控。"
            >
              <InputNumber min={1} max={2} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12}>
            <Form.Item label="最大自动重试次数" name="collect_batch_max_retries_1688">
              <InputNumber min={0} max={5} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12}>
            <Form.Item label="随机间隔最小（毫秒）" name="collect_batch_delay_min_ms_1688">
              <InputNumber min={0} max={120000} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12}>
            <Form.Item label="随机间隔最大（毫秒）" name="collect_batch_delay_max_ms_1688">
              <InputNumber min={0} max={120000} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
        </Row>

        <Divider style={{ margin: '4px 0 12px' }} />

        <Typography.Title level={5} className="tm-collector-settings__section-title">
          失败重试
        </Typography.Title>
        <Row gutter={16}>
          <Col xs={24} sm={12}>
            <Form.Item
              label="遇风控 / 验证页自动重试"
              name="collect_batch_retry_on_blocked"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12}>
            <Form.Item
              label="遇超时 / 导航失败自动重试"
              name="collect_batch_retry_on_timeout"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
        </Row>
      </Space>
    </ProCard>
  );
}

function CollectorCustomSection({ providerRow }: { providerRow?: CollectProviderRow }) {
  const statusTag = providerRow ? collectProviderStatusPresentation(providerRow.source, providerRow.status) : null;

  return (
    <ProCard title="自定义链接专属配置" bordered className="tm-collector-settings__panel">
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        {statusTag ? (
          <div className="tm-collector-auth-panel">
            <Space wrap>
              <Typography.Text strong>自定义链接采集状态</Typography.Text>
              <Tag color={statusTag.color}>{statusTag.text}</Tag>
            </Space>
          </div>
        ) : null}
        <Alert
          type="info"
          showIcon
          message="用于采集没有专用采集器的网站商品页"
          description="请先创建采集规则，再开始采集。需要登录的网站，可管理登录状态并在采集浏览器中自行登录后再测试与采集。"
        />
        <Space wrap className="tm-action-space">
          <Button type="primary" onClick={() => history.push('/collect/browser-profiles')}>
            管理登录状态
          </Button>
          <Button onClick={() => history.push('/collect/rules')}>采集规则</Button>
          <Button onClick={() => history.push('/collect/rules')}>测试采集效果</Button>
        </Space>
        <Row gutter={16}>
          <Col xs={24} sm={12}>
            <Form.Item
              label="启用页面访问检测"
              name="collect_custom_access_check_enabled"
              valuePropName="checked"
              tooltip="提交前检测商品页能否打开、是否需要登录或验证。"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12}>
            <Form.Item
              label="允许使用已登录的采集浏览器"
              name="collect_custom_profile_enabled"
              valuePropName="checked"
              tooltip="开启后，采集与规则测试可使用你在采集浏览器中保存的登录状态。"
            >
              <Switch />
            </Form.Item>
          </Col>
        </Row>
        <Alert type="warning" showIcon message="自定义链接批量采集：暂未开放" />
        <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
          在「采集规则」页创建并启用规则后，先「测试采集效果」确认标题、价格、图片能识别，再提交采集任务。不会写规则时可使用「AI 帮我生成规则」。
        </Typography.Paragraph>
        <Row gutter={16}>
          <Col xs={24} sm={12}>
            <Form.Item
              label="启用 AI 帮我生成规则"
              name="collect_rule_ai_enabled"
              valuePropName="checked"
              tooltip="关闭后管理端隐藏 AI 生成入口（需已在 AI 设置中配置大模型）。"
            >
              <Switch />
            </Form.Item>
          </Col>
        </Row>
      </Space>
    </ProCard>
  );
}

function CollectorAliExpressSection({ providerRow }: { providerRow?: CollectProviderRow }) {
  const isBeta = providerRow?.status === 'beta';

  return (
    <ProCard title="速卖通专属配置" bordered className="tm-collector-settings__panel">
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <div className="tm-collector-auth-panel">
          <Space wrap>
            <Typography.Text strong>AliExpress 采集状态</Typography.Text>
            <Badge
              status={providerRow?.status === 'available' ? 'success' : 'processing'}
              text={
                providerRow?.status === 'available'
                  ? '已可用'
                  : providerRow?.status === 'beta'
                    ? '测试中（beta）'
                    : '状态未知'
              }
            />
          </Space>
        </div>

        {isBeta ? (
          <Alert
            type="warning"
            showIcon
            message="速卖通采集器当前为 beta，部分页面可能因页面结构或网络原因采集不完整。"
          />
        ) : null}

        <Alert
          type="info"
          showIcon
          message="商品信息提取"
          description="专用采集器会尝试提取标题、主图、详情图、商品参数与商品规格。请先单条采集验证是否完整。"
        />

        <Row gutter={16}>
          <Col xs={24} sm={12}>
            <Form.Item
              label="页面超时覆盖（毫秒）"
              name="collect_aliexpress_timeout_ms"
              tooltip="留空或 0 时使用通用「页面打开超时」。"
            >
              <InputNumber min={0} max={300000} style={{ width: '100%' }} placeholder="使用通用超时" />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12}>
            <Form.Item
              label="超时 / 导航失败自动重试"
              name="collect_aliexpress_retry_on_timeout"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
        </Row>

        <Alert
          type="warning"
          showIcon
          message={
            providerRow?.batchSupported
              ? '速卖通批量采集已开放'
              : '速卖通批量采集：暂未开放（测试阶段仅支持单条采集）'
          }
        />
        <Form.Item
          label="启用速卖通批量采集（预留）"
          name="collect_aliexpress_batch_enabled"
          valuePropName="checked"
          hidden={!providerRow?.batchSupported}
        >
          <Switch disabled={!providerRow?.batchSupported} />
        </Form.Item>
      </Space>
    </ProCard>
  );
}

function CollectorPinduoduoSection({
  providerRow,
  authStatus,
  authChecking,
  authLoaded,
  loginOpening,
  onRecheck,
  onOpenLogin,
}: {
  providerRow?: CollectProviderRow;
  authStatus: ProviderPinduoduoAuthStatus | null;
  authChecking: boolean;
  authLoaded: boolean;
  loginOpening: boolean;
  onRecheck: () => void;
  onOpenLogin: () => void;
}) {
  const isBeta = providerRow?.status === 'beta';
  const authKey = resolvePddDisplayStatus(authStatus, authChecking, authLoaded);

  return (
    <ProCard
      title="拼多多专属配置"
      bordered
      className="tm-collector-settings__panel"
      extra={
        <Space wrap size="small" className="tm-action-space">
          <Button size="small" onClick={onRecheck} loading={authChecking}>
            重新检测
          </Button>
          <Button size="small" type="primary" onClick={onOpenLogin} loading={loginOpening}>
            打开拼多多采集浏览器登录
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <div className="tm-collector-auth-panel">
          <Space wrap>
            <Typography.Text strong>拼多多采集器状态</Typography.Text>
            <Badge
              status={providerRow?.status === 'available' ? 'success' : 'processing'}
              text={
                providerRow?.status === 'available'
                  ? '已可用'
                  : providerRow?.status === 'beta'
                    ? '测试中（beta）'
                    : '状态未知'
              }
            />
          </Space>
        </div>

        <div className={`tm-collector-auth-panel tm-collector-auth-panel--${authKey}`}>
          <Space direction="vertical" size={4} style={{ width: '100%' }}>
            <Space wrap>
              <Typography.Text strong>拼多多登录状态</Typography.Text>
              {pddAuthStatusBadge(authStatus, authChecking, authLoaded)}
            </Space>
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
              拼多多部分商品页和批发页需要登录或微信扫码授权。建议从失败任务或采集弹窗中打开登录，系统会直接打开需要采集的商品页；若无上下文则打开批发入口（不会打开移动端 App 首页）。系统不会保存账号密码。
            </Typography.Paragraph>
            {authChecking ? (
              <Typography.Text type="secondary">正在检测登录态…</Typography.Text>
            ) : authStatus?.message ? (
              <Typography.Text type="secondary">{authStatus.message}</Typography.Text>
            ) : authKey === 'wechat_auth_required' ? (
              <Typography.Text type="secondary">
                拼多多登录需要微信扫码授权，请在弹出的采集浏览器中完成扫码登录。
              </Typography.Text>
            ) : authKey === 'not_logged_in' ? (
              <Typography.Text type="secondary">
                请先打开采集浏览器登录拼多多，然后重新检测。
              </Typography.Text>
            ) : authKey === 'app_redirect' ? (
              <Typography.Text type="secondary">
                当前为 App 引导页，无法确认登录态。请从具体商品链接或失败任务中打开登录后再检测。
              </Typography.Text>
            ) : authKey === 'verification_required' ? (
              <Typography.Text type="secondary">
                拼多多页面可能出现验证码或安全验证，请在采集浏览器中手动完成验证后重试。
              </Typography.Text>
            ) : authKey === 'homepage_only' ? (
              <Typography.Text type="secondary">
                拼多多首页可能游客也能访问。请从失败任务中点击「打开拼多多采集浏览器登录」，或在采集弹窗输入具体商品链接后重新检测。
              </Typography.Text>
            ) : authKey === 'unknown' ? (
              <Typography.Text type="secondary">
                请确认采集浏览器中是否已完成登录或微信授权，然后使用具体商品链接重新检测。
              </Typography.Text>
            ) : null}
            <Typography.Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 4 }}>
              如跳转到微信页面，请用微信扫码完成授权。
            </Typography.Text>
            {authStatus?.lastCheckedAt ? (
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                上次检测：{authStatus.lastCheckedAt}
              </Typography.Text>
            ) : null}
          </Space>
        </div>

        {isBeta ? (
          <Alert
            type="warning"
            showIcon
            message="拼多多采集器当前为测试中，适合单链接采集商品基础信息。商品规格、库存和动态价格可能不完整，请采集后人工检查。"
          />
        ) : null}

        <Form.Item
          label="用于检测的商品链接"
          name="collect_pinduoduo_auth_check_url"
          tooltip="填写拼多多商品详情页（含批发 goods/detail），重新检测时将打开该页验证登录态。"
        >
          <Input placeholder="建议填写一个需要采集的拼多多商品详情链接，用于确认登录状态" allowClear />
        </Form.Item>

        <Row gutter={16}>
          <Col xs={24} sm={12}>
            <Form.Item
              label="页面打开超时覆盖（毫秒）"
              name="collect_pinduoduo_timeout_ms"
              tooltip="留空或 0 时使用通用「页面打开超时」。"
            >
              <InputNumber min={0} max={300000} style={{ width: '100%' }} placeholder="使用通用超时" />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12}>
            <Form.Item
              label="启用访问状态检测"
              name="collect_pinduoduo_access_check_enabled"
              valuePropName="checked"
              tooltip="检测登录页、验证页、App 引导页等访问状态。"
            >
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} sm={12}>
            <Form.Item
              label="超时 / 导航失败自动重试"
              name="collect_pinduoduo_retry_on_timeout"
              valuePropName="checked"
            >
              <Switch />
            </Form.Item>
          </Col>
        </Row>

        <Alert type="warning" showIcon message="拼多多批量采集：暂未开放" />
        <Form.Item
          label="启用拼多多批量采集（预留）"
          name="collect_pinduoduo_batch_enabled"
          valuePropName="checked"
          hidden
        >
          <Switch disabled />
        </Form.Item>
      </Space>
    </ProCard>
  );
}

function CollectorPlannedSection({ providerLabel }: { providerLabel: string }) {
  return (
    <ProCard bordered className="tm-collector-settings__panel">
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description={
          <Space direction="vertical" size="small">
            <Typography.Text>{providerLabel}暂未开放，当前仅保留配置入口。</Typography.Text>
            <Typography.Text type="secondary">
              后续支持后，可在这里配置登录态、并发、重试和字段提取策略。
            </Typography.Text>
          </Space>
        }
      >
        <Button type="primary" onClick={() => history.push('/collect/hub')}>
          返回采集中心
        </Button>
      </Empty>
    </ProCard>
  );
}

export default function CollectorSettingsPage() {
  const location = useLocation();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [providers, setProviders] = useState<CollectProviderRow[]>([]);
  const [authStatus, setAuthStatus] = useState<Provider1688AuthStatus | null>(null);
  const [authChecking, setAuthChecking] = useState(false);
  const [authLoaded, setAuthLoaded] = useState(false);
  const [loginOpening, setLoginOpening] = useState(false);
  const [pddAuthStatus, setPddAuthStatus] = useState<ProviderPinduoduoAuthStatus | null>(null);
  const [pddAuthChecking, setPddAuthChecking] = useState(false);
  const [pddAuthLoaded, setPddAuthLoaded] = useState(false);
  const [pddLoginOpening, setPddLoginOpening] = useState(false);

  const providerKey = useMemo(
    () => resolveCollectSettingsProvider(new URLSearchParams(location.search || '').get('provider')),
    [location.search],
  );
  const providerOption = useMemo(() => findCollectSettingsOption(providerKey), [providerKey]);
  const planned = useMemo(
    () => isPlannedCollectProvider(providers, providerOption),
    [providers, providerOption],
  );
  const providerRow = useMemo(
    () => providers.find((p) => p.source.toLowerCase() === providerOption.source.toLowerCase()),
    [providers, providerOption.source],
  );

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
      setAuthLoaded(true);
    }
  }, []);

  const loadPddAuthStatus = useCallback(async () => {
    const testUrl = String(form.getFieldValue('collect_pinduoduo_auth_check_url') ?? '').trim();
    if (!testUrl) {
      message.warning(
        '未提供商品详情链接，本次只能检测拼多多首页是否可访问，不能准确判断是否已登录。',
        6,
      );
    }
    setPddAuthChecking(true);
    try {
      const data = await checkPinduoduoLogin({ testUrl: testUrl || undefined });
      setPddAuthStatus(data);
    } catch (e: unknown) {
      setPddAuthStatus(null);
      message.error((e as Error)?.message || '拼多多登录态检测失败');
    } finally {
      setPddAuthChecking(false);
      setPddAuthLoaded(true);
    }
  }, [form]);

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
        collect_batch_retry_on_blocked: parseBoolSetting(g.collect_batch_retry_on_blocked),
        collect_batch_retry_on_timeout: parseBoolSetting(g.collect_batch_retry_on_timeout),
        collect_batch_max_retries_1688: g.collect_batch_max_retries_1688
          ? Number(g.collect_batch_max_retries_1688)
          : 2,
        collect_custom_access_check_enabled: parseBoolSetting(g.collect_custom_access_check_enabled),
        collect_custom_profile_enabled: parseBoolSetting(g.collect_custom_profile_enabled),
        collect_custom_batch_enabled: g.collect_custom_batch_enabled === '1' || g.collect_custom_batch_enabled === 'true',
        collect_rule_ai_enabled: g.collect_rule_ai_enabled !== '0' && g.collect_rule_ai_enabled !== 'false',
        collect_aliexpress_timeout_ms: g.collect_aliexpress_timeout_ms
          ? Number(g.collect_aliexpress_timeout_ms)
          : undefined,
        collect_aliexpress_retry_on_timeout: parseBoolSetting(g.collect_aliexpress_retry_on_timeout),
        collect_aliexpress_batch_enabled:
          g.collect_aliexpress_batch_enabled === '1' || g.collect_aliexpress_batch_enabled === 'true',
        collect_pinduoduo_timeout_ms: g.collect_pinduoduo_timeout_ms
          ? Number(g.collect_pinduoduo_timeout_ms)
          : undefined,
        collect_pinduoduo_auth_check_url: g.collect_pinduoduo_auth_check_url || '',
        collect_pinduoduo_access_check_enabled: parseBoolSetting(g.collect_pinduoduo_access_check_enabled),
        collect_pinduoduo_retry_on_timeout: parseBoolSetting(g.collect_pinduoduo_retry_on_timeout),
        collect_pinduoduo_batch_enabled:
          g.collect_pinduoduo_batch_enabled === '1' || g.collect_pinduoduo_batch_enabled === 'true',
      });
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => {
    void load();
    void queryCollectProviders()
      .then((rows) => setProviders(Array.isArray(rows) ? rows : []))
      .catch(() => setProviders([]));
  }, [load]);

  useEffect(() => {
    if (providerKey === '1688') {
      void loadAuthStatus();
    }
    if (providerKey === 'pinduoduo') {
      void loadPddAuthStatus();
    }
  }, [providerKey, loadAuthStatus, loadPddAuthStatus]);

  const handleProviderChange = (key: CollectSettingsProviderKey) => {
    history.replace(`/settings/collector?provider=${encodeURIComponent(key)}`);
  };

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

  const handleOpenPddLoginBrowser = async () => {
    setPddLoginOpening(true);
    try {
      const testUrl = String(form.getFieldValue('collect_pinduoduo_auth_check_url') ?? '').trim();
      const loginTarget = testUrl || 'https://pifa.pinduoduo.com/';
      const result = await openPinduoduoLoginBrowser(loginTarget);
      message.success(result.message || '已打开拼多多采集浏览器');
      await loadPddAuthStatus();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '打开拼多多采集浏览器失败');
    } finally {
      setPddLoginOpening(false);
    }
  };

  const handleSave = async (values: Record<string, unknown>) => {
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
        collect_custom_access_check_enabled: values.collect_custom_access_check_enabled ? '1' : '0',
        collect_custom_profile_enabled: values.collect_custom_profile_enabled ? '1' : '0',
        collect_custom_batch_enabled: values.collect_custom_batch_enabled ? '1' : '0',
        collect_rule_ai_enabled: values.collect_rule_ai_enabled ? '1' : '0',
        collect_aliexpress_timeout_ms:
          values.collect_aliexpress_timeout_ms != null && values.collect_aliexpress_timeout_ms !== ''
            ? String(values.collect_aliexpress_timeout_ms)
            : '',
        collect_aliexpress_retry_on_timeout: values.collect_aliexpress_retry_on_timeout ? '1' : '0',
        collect_aliexpress_batch_enabled: values.collect_aliexpress_batch_enabled ? '1' : '0',
        collect_pinduoduo_timeout_ms:
          values.collect_pinduoduo_timeout_ms != null && values.collect_pinduoduo_timeout_ms !== ''
            ? String(values.collect_pinduoduo_timeout_ms)
            : '',
        collect_pinduoduo_auth_check_url: String(values.collect_pinduoduo_auth_check_url ?? '').trim(),
        collect_pinduoduo_access_check_enabled: values.collect_pinduoduo_access_check_enabled ? '1' : '0',
        collect_pinduoduo_retry_on_timeout: values.collect_pinduoduo_retry_on_timeout ? '1' : '0',
        collect_pinduoduo_batch_enabled: values.collect_pinduoduo_batch_enabled ? '1' : '0',
      };
      await saveSettingsItems(toPutItems(GROUP, FIELDS, payload));
      message.success('已保存');
      await load();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '保存失败');
    }
  };

  const providerSpecificSection = planned ? (
    <CollectorPlannedSection providerLabel={providerOption.label} />
  ) : providerKey === '1688' ? (
    <Collector1688Section
      authStatus={authStatus}
      authChecking={authChecking}
      loginOpening={loginOpening}
      onRecheck={loadAuthStatus}
      onOpenLogin={handleOpenLoginBrowser}
    />
  ) : providerKey === 'custom' ? (
    <CollectorCustomSection providerRow={providerRow} />
  ) : providerKey === 'aliexpress' ? (
    <CollectorAliExpressSection providerRow={providerRow} />
  ) : providerKey === 'pinduoduo' ? (
    <CollectorPinduoduoSection
      providerRow={providerRow}
      authStatus={pddAuthStatus}
      authChecking={pddAuthChecking}
      authLoaded={pddAuthLoaded}
      loginOpening={pddLoginOpening}
      onRecheck={loadPddAuthStatus}
      onOpenLogin={handleOpenPddLoginBrowser}
    />
  ) : null;

  return (
    <PageContainer title="采集设置" subTitle={`当前：${providerOption.label}`}>
      <div className="tm-collector-settings">
        <ProCard bordered className="tm-collector-settings__selector" title="采集器类型">
          <CollectorProviderSelector
            activeKey={providerKey}
            providers={providers}
            onChange={handleProviderChange}
          />
        </ProCard>

        <Form form={form} layout="vertical" onFinish={handleSave}>
          {planned ? (
            providerSpecificSection
          ) : (
            <Row gutter={[16, 16]} align="stretch">
              <Col xs={24} xl={10}>
                <ProCard
                  title="通用采集设置"
                  bordered
                  className="tm-collector-settings__panel"
                  extra={
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      影响所有采集器
                    </Typography.Text>
                  }
                >
                  <Form.Item label="主服务 URL" name="main_service_url" rules={[{ required: true }]}>
                    <Input placeholder="http://127.0.0.1:8080" />
                  </Form.Item>
                  <Form.Item
                    label="采集服务监听地址"
                    name="collector_http_addr"
                    rules={[{ required: true }]}
                  >
                    <Input placeholder=":3100" />
                  </Form.Item>
                  <Row gutter={16}>
                    <Col xs={24} sm={12}>
                      <Form.Item
                        label="页面打开超时（毫秒）"
                        name="goto_timeout_ms"
                        rules={[{ required: true }]}
                      >
                        <InputNumber min={1000} max={300000} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                    <Col xs={24} sm={12}>
                      <Form.Item label="无头模式" name="headless" valuePropName="checked">
                        <Switch />
                      </Form.Item>
                    </Col>
                  </Row>
                </ProCard>
              </Col>
              <Col xs={24} xl={14}>
                {providerSpecificSection}
              </Col>
            </Row>
          )}

          {!planned ? (
            <ProCard bordered className="tm-collector-settings__footer">
              <Space wrap className="tm-action-space">
                <Button type="primary" htmlType="submit" loading={loading}>
                  保存配置
                </Button>
                <Button onClick={load} disabled={loading}>
                  重新加载
                </Button>
                <Button type="link" onClick={() => history.push('/collect/hub')} style={{ paddingInline: 0 }}>
                  返回采集中心
                </Button>
              </Space>
            </ProCard>
          ) : null}
        </Form>
      </div>
    </PageContainer>
  );
}
