import SettingsMessage from '../SettingsMessage';
import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { ApiOutlined, LinkOutlined, ReloadOutlined, SafetyCertificateOutlined, SaveOutlined, SyncOutlined } from '@ant-design/icons';
import {
  Alert,
  Button,
  Collapse,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Spin,
  Switch,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import { Link } from '@umijs/renderer-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import DouyinE2EPrecheckBanner from '@/components/platform/DouyinE2EPrecheckBanner';
import StoragePublicUrlBanner from '@/components/platform/StoragePublicUrlBanner';
import { ActionBar, FormGrid, FormGridFull, FormGridItem, SectionCard, TechnicalDetails, TmPageContainer } from '@/components/ui';
import { ACTION_COPY, PAGE_COPY } from '@/constants/copywriting';
import { confirmPlatformConfigSave } from '@/constants/sensitiveActions';
import { platformRuntimeHref } from '@/constants/platformRuntime';
import { formatUserErrorMessage } from '@/constants/errorMessages';
import {
  PLATFORM_DEV_PORTALS,
  PLATFORM_STATUS_META,
  isPlatformSwitchField,
  platformAppFieldHelp,
  platformAppFieldLabel,
  platformAppFieldPlaceholder,
} from '@/constants/platformAppConfig';
import type { AppConfigFieldDTO, PlatformProviderMeta } from '@/services/shops';
import {
  externalDocUrlFor,
  getPlatformAppSettings,
  preferredPlatformTabOrder,
  putPlatformAppSettings,
  startDouyinOAuth,
  testPlatformAppSettings,
} from '@/services/platformOpen';
import { queryPlatformProviders, queryShops, type ShopListRow } from '@/services/shops';
import { getDouyinCategoryStats, syncDouyinCategories } from '@/services/douyinCategories';
import {
  getDouyinProductionPreflightLatest,
  runDouyinProductionPreflight,
  getDouyinRuntimeStatus,
  pauseDouyinRuntime,
  resumeDouyinRuntime,
  emergencyDisableDouyinRuntime,
  type DouyinPreflightResult,
  type DouyinRuntimeStatus,
  type PreflightCheckItem,
} from '@/services/douyinProduction';

const { Paragraph, Text } = Typography;

function valuesToFormFields(fields: AppConfigFieldDTO[], values: Record<string, string>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const f of fields) {
    const raw = values[f.name] ?? '';
    switch (f.type) {
      case 'switch':
        out[f.name] = raw === 'true';
        break;
      case 'number':
        out[f.name] = raw === '' ? undefined : Number(raw);
        break;
      default:
        out[f.name] = raw;
    }
  }
  return out;
}

function buildPutValues(fields: AppConfigFieldDTO[], formVals: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const f of fields) {
    const v = formVals[f.name];
    if (v === undefined) continue;
    if (f.type === 'switch') {
      out[f.name] = !!v;
      continue;
    }
    if (f.type === 'number') {
      if (typeof v === 'number' && Number.isFinite(v)) out[f.name] = v;
      continue;
    }
    if (typeof v === 'string') {
      out[f.name] = v;
      continue;
    }
    out[f.name] = v;
  }
  return out;
}

function isFullWidthField(f: AppConfigFieldDTO): boolean {
  return f.type === 'textarea' || f.name === 'oauth_scopes' || f.name === 'scopes' || isPlatformSwitchField(f);
}

function renderFieldControl(f: AppConfigFieldDTO) {
  const ph = tr(platformAppFieldPlaceholder(f) ?? '');
  switch (f.type) {
    case 'password':
      return <Input.Password placeholder={ph} autoComplete={f.sensitive ? 'new-password' : 'off'} />;
    case 'textarea':
      return <Input.TextArea rows={4} placeholder={ph} />;
    case 'number':
      return (
        <InputNumber
          min={f.name === 'timeout_sec' ? 5 : undefined}
          style={{ width: '100%' }}
          placeholder={ph || undefined}
        />
      );
    case 'switch':
      return <Switch />;
    case 'select':
      return (
        <Select
          allowClear={!f.required}
          options={(f.options ?? []).map((o) => ({ label: tr(o.label), value: o.value }))}
          placeholder={ph || undefined}
        />
      );
    case 'text':
    default:
      return <Input placeholder={ph} autoComplete="off" />;
  }
}

function SwitchField({
  label,
  help,
  checked,
  onChange,
}: {
  label: string;
  help?: string;
  checked?: boolean;
  onChange?: (v: boolean) => void;
}) {
  useLanguage();
  return (
    <div className="tm-switch-field">
      <div className="tm-switch-field__row">
        <span className="tm-switch-field__label">{label}</span>
        <Switch checked={checked} onChange={onChange} />
      </div>
      {help ? <div className="tm-switch-field__help">{help}</div> : null}
    </div>
  );
}

function renderFormField(f: AppConfigFieldDTO) {
  const label = tr(platformAppFieldLabel(f) ?? '');
  const help = tr(platformAppFieldHelp(f) ?? '');

  if (isPlatformSwitchField(f)) {
    return (
      <Form.Item
        key={f.name}
        name={f.name}
        valuePropName="checked"
        rules={[{ required: f.required, message: trFormat("请设置{value0}", { value0: label }) }]}
      >
        <SwitchField label={label} help={help} />
      </Form.Item>
    );
  }

  return (
    <Form.Item
      key={f.name}
      name={f.name}
      label={label}
      valuePropName={f.type === 'switch' ? 'checked' : 'value'}
      rules={[{ required: f.required, message: trFormat("请填写{value0}", { value0: label }) }]}
      extra={help}
    >
      {renderFieldControl(f)}
    </Form.Item>
  );
}

const PREFLIGHT_STATUS: Record<string, { label: string; color: string }> = {
  passed: { get label() { return tr("通过"); }, color: 'success' },
  warning: { get label() { return tr("警告"); }, color: 'warning' },
  failed: { get label() { return tr("未通过"); }, color: 'error' },
};

function preflightOverallTag(status?: string) {
  const st = PREFLIGHT_STATUS[status || ''] ?? { label: status || tr("未知"), color: 'default' };
  return (
    <Tag color={st.color} style={{ margin: 0 }}>
      {st.label}
    </Tag>
  );
}

function runtimeStatusTag(status?: string) {
  switch (status) {
    case 'paused':
      return <Tag color="warning">{tr("已暂停")}</Tag>;
    case 'emergency_disabled':
      return <Tag color="error">{tr("紧急停用")}</Tag>;
    default:
      return <Tag color="success">{tr("正常运行")}</Tag>;
  }
}

function DouyinRuntimePanel() {
  useLanguage();
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<DouyinRuntimeStatus | null>(null);
  const [reason, setReason] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await getDouyinRuntimeStatus();
      setStatus(res);
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("加载运行状态失败"));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const confirmChange = (title: string, onOk: () => Promise<void>) => {
    if (!reason.trim()) {
      message.warning(tr("请填写变更原因"));
      return;
    }
    Modal.confirm({
      title,
      content: (
        <div>
          <Paragraph type="secondary">{tr("此操作将影响抖店任务执行，请确认原因已记录。")}</Paragraph>
          <Text>{trFormat("原因：{value0}", { value0: reason })}</Text>
        </div>
      ),
      okText: <SettingsMessage source={"确认"} />,
      cancelText: <SettingsMessage source={"取消"} />,
      onOk,
    });
  };

  return (
    <SectionCard
      title={tr("抖店运行状态")}
      description={tr("暂停或紧急停用后，后台任务进程将不再调用抖店写接口；历史数据仍可查看。")}
      headerExtra={
        <Link to={platformRuntimeHref('douyin_shop')}>
          <Button type="link" size="small">{tr("查看完整运行状态")}</Button>
        </Link>
      }
    >
      <Spin spinning={loading}>
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Space wrap>
            <Text strong>{tr("当前状态")}</Text>
            {runtimeStatusTag(status?.status)}
            {status?.message ? <Text type="secondary">{status.message}</Text> : null}
          </Space>
          {status?.reason ? <Text type="secondary">{trFormat("最近变更原因：{value0}", { value0: status.reason })}</Text> : null}
          {status?.changedAt ? <Text type="secondary">{trFormat("变更时间：{value0}", { value0: status.changedAt })}</Text> : null}
          <Input.TextArea
            rows={2}
            placeholder={tr("填写本次操作原因（必填）")}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
          <ActionBar>
            <Button
              onClick={() =>
                confirmChange(tr("暂停抖店任务？"), async () => {
                  const res = await pauseDouyinRuntime(reason.trim());
                  setStatus(res);
                  setReason('');
                  message.success(tr("已暂停"));
                })
              }
            >{tr("暂停任务")}</Button>
            <Button
              type="primary"
              onClick={() =>
                confirmChange(tr("恢复抖店运行？"), async () => {
                  const res = await resumeDouyinRuntime(reason.trim());
                  setStatus(res);
                  setReason('');
                  message.success(tr("已恢复运行"));
                })
              }
            >{tr("恢复运行")}</Button>
            <Button
              danger
              onClick={() =>
                confirmChange(tr("紧急停用抖店？"), async () => {
                  const res = await emergencyDisableDouyinRuntime(reason.trim());
                  setStatus(res);
                  setReason('');
                  message.warning(tr("已紧急停用"));
                })
              }
            >{tr("紧急停用")}</Button>
            <Button icon={<ReloadOutlined />} onClick={() => void load()}>{tr("刷新")}</Button>
          </ActionBar>
        </Space>
      </Spin>
    </SectionCard>
  );
}

function DouyinPreflightPanel() {
  useLanguage();
  const [running, setRunning] = useState(false);
  const [liveTest, setLiveTest] = useState(false);
  const [result, setResult] = useState<DouyinPreflightResult | null>(null);

  const loadLatest = useCallback(async () => {
    try {
      const res = await getDouyinProductionPreflightLatest();
      if (res && 'checks' in res && Array.isArray(res.checks)) {
        setResult(res);
      }
    } catch {
      /* ignore */
    }
  }, []);

  useEffect(() => {
    void loadLatest();
  }, [loadLatest]);

  const run = useCallback(async () => {
    setRunning(true);
    try {
      const res = await runDouyinProductionPreflight(liveTest);
      setResult(res);
      if (res.status === 'passed') {
        message.success(tr("生产预检全部通过"));
      } else if (res.status === 'warning') {
        message.warning(trFormat("预检完成：{value0} 项警告，{value1} 项未通过", { value0: res.warningCount, value1: res.failedCount }));
      } else {
        message.error(trFormat("预检未通过：{value0} 项需处理", { value0: res.failedCount }));
      }
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("运行预检失败"));
    } finally {
      setRunning(false);
    }
  }, [liveTest]);

  return (
    <SectionCard title={tr("抖店生产预检")} extra={result?.checkedAt ? trFormat("最近：{value0}", { value0: result.checkedAt }) : undefined}>
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Paragraph type="secondary" style={{ marginBottom: 0 }}>{tr("上线前检查应用配置、店铺授权、功能开关、存储公网访问与基础数据状态；不包含自动写操作。")}</Paragraph>
        <DouyinE2EPrecheckBanner blockedByCredentials={result?.blockedByRealCredentials ?? true} compact />
        <StoragePublicUrlBanner
          missing={
            result?.checks?.some(
              (c) => c.key === 'storage.public_base' && (c.status === 'failed' || c.status === 'warning'),
            ) ?? false
          }
          compact
        />
        <Space wrap>
          <Button type="primary" icon={<SafetyCertificateOutlined />} loading={running} onClick={() => void run()}>{tr("运行生产预检")}</Button>
          <Button onClick={() => window.open('/docs/DOUYIN_E2E_PRECHECK_GUIDE.md', '_blank')}>{tr("查看上线准备清单")}</Button>
          <Switch checked={liveTest} onChange={setLiveTest} /> <Text type="secondary">{tr("包含真实访问令牌刷新联调（需已授权店铺）")}</Text>
        </Space>
        {result?.blockedByRealCredentials ? (
          <Alert
            showIcon
            type="info"
            message={tr("当前未配置抖店真实凭证，只能完成本地配置检查，不能调用真实平台。")}
            description={tr("凭证缺失不是系统失败，属于需真实环境凭证的前置检查项。抖店仍为发布候选。")}
          />
        ) : null}
        {result ? (
          <>
            <Space>
              <Text strong>{tr("总体结果")}</Text>
              {preflightOverallTag(result.status)}
              <Text type="secondary">{trFormat("通过 {value0} · 警告 {value1} · 未通过 {value2}", { value0: result.passedCount, value1: result.warningCount, value2: result.failedCount })}</Text>
            </Space>
            <Collapse
              accordion
              items={result.checks.map((c: PreflightCheckItem) => ({
                key: c.key,
                label: (
                  <Space>
                    {preflightOverallTag(c.status)}
                    <span>{c.title}</span>
                  </Space>
                ),
                children: (
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    <Text>{c.message}</Text>
                    {c.suggestion ? <Text type="secondary">{trFormat("建议：{value0}", { value0: c.suggestion })}</Text> : null}
                    {c.technicalDetails && Object.keys(c.technicalDetails).length > 0 ? (
                      <TechnicalDetails label={tr("技术详情")}>
                        <pre style={{ margin: 0, fontSize: 12 }}>{JSON.stringify(c.technicalDetails, null, 2)}</pre>
                      </TechnicalDetails>
                    ) : null}
                  </Space>
                ),
              }))}
            />
          </>
        ) : (
          <Alert showIcon type="info" message={tr("尚未运行预检，点击上方按钮开始检查")} />
        )}
      </Space>
    </SectionCard>
  );
}

function PlatformPanel({ meta }: { meta: PlatformProviderMeta }) {
  useLanguage();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState(false);
  const [connecting, setConnecting] = useState(false);
  const [categorySyncing, setCategorySyncing] = useState(false);
  const [douyinStats, setDouyinStats] = useState<{ count: number; leafCount: number; lastSyncedAt?: string }>();
  const [douyinShops, setDouyinShops] = useState<ShopListRow[]>([]);

  const schema = meta.appConfigSchema;
  const fields = schema?.fields ?? [];
  const st = PLATFORM_STATUS_META[meta.status] ?? { label: meta.status, color: 'default' };
  const panelTitle = meta.platform === 'douyin_shop' ? tr("抖店接入设置") : trFormat("{value0} 接入设置", { value0: meta.name });

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const row = await getPlatformAppSettings(meta.platform);
      const flds = row.schema?.fields?.length ? row.schema.fields : meta.appConfigSchema?.fields ?? [];
      form.resetFields();
      form.setFieldsValue(valuesToFormFields(flds, row.values ?? {}));
      if (meta.platform === 'douyin_shop') {
        const [stats, shops] = await Promise.all([
          getDouyinCategoryStats().catch(() => undefined),
          queryShops({ page: 1, pageSize: 100, platform: 'douyin_shop', authStatus: 'authorized' }).catch(() => ({ list: [] })),
        ]);
        if (stats) setDouyinStats(stats);
        setDouyinShops(Array.isArray(shops.list) ? shops.list : []);
      }
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("加载失败"));
    } finally {
      setLoading(false);
    }
  }, [form, meta]);

  useEffect(() => {
    void load();
  }, [load]);

  const docUrl = externalDocUrlFor(meta.platform);

  const testConnection = useCallback(async () => {
    setTesting(true);
    try {
      const res = await testPlatformAppSettings(meta.platform);
      if (res.ok) {
        message.success(res.message || tr("连接测试通过"));
      } else {
        message.warning(res.message || tr("连接测试未通过，请检查应用信息"));
      }
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("连接测试失败"));
    } finally {
      setTesting(false);
    }
  }, [meta.platform]);

  const connectDouyinShop = useCallback(async () => {
    setConnecting(true);
    try {
      const res = await startDouyinOAuth();
      const target = res.redirectUrl || res.authorizeUrl;
      if (!target) throw new Error(tr("无法获取授权地址"));
      window.location.href = target;
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("发起店铺授权失败"));
      setConnecting(false);
    }
  }, []);

  const syncDouyinCategoryCache = useCallback(async () => {
    const shop = douyinShops[0];
    if (!shop?.id) {
      message.warning(tr("请先完成抖店店铺授权，再同步类目"));
      return;
    }
    setCategorySyncing(true);
    try {
      const stats = await syncDouyinCategories(shop.id);
      setDouyinStats(stats);
      message.success(trFormat("已同步 {value0} 个抖店类目", { value0: stats.count ?? 0 }));
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("同步类目失败"));
    } finally {
      setCategorySyncing(false);
    }
  }, [douyinShops]);

  return (
    <Spin spinning={loading}>
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        {meta.status === 'planned' && (
          <Alert
            showIcon
            type="warning"
            message={tr("该平台能力尚未完全开放")}
            description={tr("可先填写平台应用信息，供后续店铺授权使用。")}
          />
        )}

        <SectionCard title={panelTitle} description={schema?.description || undefined}>
          <div style={{ marginBottom: 12 }}>
            <Text type="secondary">{tr("接入状态")}</Text>
            <Tag color={st.color}>{st.label}</Tag>
          </div>

          <Form
            layout="vertical"
            form={form}
            onFinish={(vals: Record<string, unknown>) =>
              new Promise<void>((resolve, reject) => {
                confirmPlatformConfigSave(async () => {
                  try {
                    await putPlatformAppSettings(meta.platform, buildPutValues(fields, vals));
                    message.success(tr("设置已保存"));
                    await load();
                    resolve();
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || tr("保存失败"));
                    reject(e);
                  }
                });
              })
            }
          >
            <FormGrid>
              {fields.map((f) =>
                isFullWidthField(f) ? (
                  <FormGridFull key={f.name}>{renderFormField(f)}</FormGridFull>
                ) : (
                  <FormGridItem key={f.name}>{renderFormField(f)}</FormGridItem>
                ),
              )}
            </FormGrid>

            <ActionBar>
              <Button type="primary" htmlType="submit" loading={loading} icon={<SaveOutlined />}>
                {ACTION_COPY.saveSettings}
              </Button>
              <Button icon={<ReloadOutlined />} onClick={() => void load()} disabled={loading}>
                {ACTION_COPY.reload}
              </Button>
              <Button icon={<ApiOutlined />} onClick={() => void testConnection()} loading={testing} disabled={loading}>
                {ACTION_COPY.testConnection}
              </Button>
              {meta.platform === 'douyin_shop' ? (
                <Button icon={<LinkOutlined />} onClick={() => void connectDouyinShop()} loading={connecting} disabled={loading}>
                  {ACTION_COPY.authorizeShop}
                </Button>
              ) : null}
              {meta.platform === 'douyin_shop' ? (
                <Button icon={<SyncOutlined />} onClick={() => void syncDouyinCategoryCache()} loading={categorySyncing} disabled={loading}>{tr("同步类目")}</Button>
              ) : null}
              {docUrl ? (
                <Typography.Link href={docUrl} target="_blank" rel="noreferrer">{tr("查看平台文档")}</Typography.Link>
              ) : null}
            </ActionBar>
          </Form>
        </SectionCard>

        {meta.platform === 'douyin_shop' ? (
          <Alert
            showIcon
            type="info"
            message={tr("使用前请确认")}
            description={
              <>{tr("在「存储设置」中配置抖店可访问的公网图片地址。同步订单、库存或创建商品草稿前，请在本页开启对应开关并完成店铺授权。")}</>
            }
          />
        ) : null}

        {meta.platform === 'douyin_shop' ? (
          <Alert
            showIcon
            type={douyinStats?.count ? 'success' : 'warning'}
            message={tr("抖店类目")}
            description={
              douyinStats?.count
                ? trFormat("已缓存 {value0} 个类目（叶子类目 {value1} 个），最近同步：{value2}", { value0: douyinStats.count, value1: douyinStats.leafCount ?? 0, value2: douyinStats.lastSyncedAt || tr('未知') })
                : tr("暂无类目数据，请先完成店铺授权，再点击「同步类目」。")
            }
          />
        ) : null}

        {meta.platform === 'douyin_shop' ? <DouyinRuntimePanel /> : null}
        {meta.platform === 'douyin_shop' ? <DouyinPreflightPanel /> : null}
      </Space>
    </Spin>
  );
}

export default function PlatformSettingsPage() {
  const language = useLanguage();
  const [loadingProviders, setLoadingProviders] = useState(true);
  const [providers, setProviders] = useState<PlatformProviderMeta[]>([]);
  const [tab, setTab] = useState<string>();

  const withSchema = useMemo(() => {
    return [...providers]
      .filter((p) => p.settingsGroupKey && p.settingsGroupKey.trim())
      .sort((a, b) => preferredPlatformTabOrder(a.platform) - preferredPlatformTabOrder(b.platform));
  }, [providers, language]);

  const loadProviders = useCallback(async () => {
    setLoadingProviders(true);
    try {
      const { list } = await queryPlatformProviders();
      setProviders(Array.isArray(list) ? list : []);
    } catch (e: unknown) {
      message.error((e as Error)?.message || tr("加载平台列表失败"));
    } finally {
      setLoadingProviders(false);
    }
  }, []);

  useEffect(() => {
    void loadProviders();
  }, [loadProviders]);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    if (params.get('platform') !== 'douyin_shop') return;
    const auth = params.get('auth');
    if (auth === 'success') {
      message.success(tr("抖店店铺授权成功"));
    } else if (auth === 'failed') {
      const reason = params.get('reason');
      message.error(formatUserErrorMessage(reason, tr("抖店店铺授权失败")));
    }
  }, []);

  useEffect(() => {
    if (!tab && withSchema.length > 0) {
      const douyin = withSchema.find((p) => p.platform === 'douyin_shop');
      setTab((douyin ?? withSchema[0]).platform);
    }
  }, [tab, withSchema]);

  const items = withSchema.map((p) => {
    const st = PLATFORM_STATUS_META[p.status];
    return {
      key: p.platform,
      label: (
        <Space size={6}>
          <span>{p.name}</span>
          {st && p.status !== 'available' ? (
            <Tag color={st.color} style={{ margin: 0 }}>
              {st.label}
            </Tag>
          ) : null}
        </Space>
      ),
      children: <PlatformPanel meta={p} />,
    };
  });

  return (
    <TmPageContainer title={PAGE_COPY.platformSettings.title} subTitle={PAGE_COPY.platformSettings.description}>
      <div className="tm-settings-stack">
        <SectionCard
          title={PAGE_COPY.platformSettings.heroTitle}
          description={PAGE_COPY.platformSettings.heroDescription}
        >
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>{tr("开发者门户：")}{PLATFORM_DEV_PORTALS.map((p, i) => (
              <span key={p.url}>
                {i > 0 ? ' · ' : ' '}
                <Typography.Link href={p.url} target="_blank" rel="noreferrer">
                  {p.name}
                </Typography.Link>
              </span>
            ))}</Paragraph>
        </SectionCard>

        <SectionCard title={tr("选择平台")}>
          <Spin spinning={loadingProviders}>
            {items.length === 0 ? (
              <Paragraph type="secondary">{tr("暂无可配置的平台，请刷新页面后重试。")}</Paragraph>
            ) : (
              <Tabs activeKey={tab} onChange={setTab} items={items} />
            )}
          </Spin>
        </SectionCard>
      </div>
    </TmPageContainer>
  );
}
