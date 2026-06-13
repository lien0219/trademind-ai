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
import { ActionBar, FormGrid, FormGridFull, FormGridItem, SectionCard, TechnicalDetails, TmPageContainer } from '@/components/ui';
import { ACTION_COPY, PAGE_COPY } from '@/constants/copywriting';
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
  const ph = platformAppFieldPlaceholder(f);
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
          options={(f.options ?? []).map((o) => ({ label: o.label, value: o.value }))}
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
  const label = platformAppFieldLabel(f);
  const help = platformAppFieldHelp(f);

  if (isPlatformSwitchField(f)) {
    return (
      <Form.Item
        key={f.name}
        name={f.name}
        valuePropName="checked"
        rules={[{ required: f.required, message: `请设置${label}` }]}
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
      rules={[{ required: f.required, message: `请填写${label}` }]}
      extra={help}
    >
      {renderFieldControl(f)}
    </Form.Item>
  );
}

const PREFLIGHT_STATUS: Record<string, { label: string; color: string }> = {
  passed: { label: '通过', color: 'success' },
  warning: { label: '警告', color: 'warning' },
  failed: { label: '未通过', color: 'error' },
};

function preflightOverallTag(status?: string) {
  const st = PREFLIGHT_STATUS[status || ''] ?? { label: status || '未知', color: 'default' };
  return (
    <Tag color={st.color} style={{ margin: 0 }}>
      {st.label}
    </Tag>
  );
}

function runtimeStatusTag(status?: string) {
  switch (status) {
    case 'paused':
      return <Tag color="warning">已暂停</Tag>;
    case 'emergency_disabled':
      return <Tag color="error">紧急停用</Tag>;
    default:
      return <Tag color="success">正常运行</Tag>;
  }
}

function DouyinRuntimePanel() {
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<DouyinRuntimeStatus | null>(null);
  const [reason, setReason] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await getDouyinRuntimeStatus();
      setStatus(res);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载运行状态失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const confirmChange = (title: string, onOk: () => Promise<void>) => {
    if (!reason.trim()) {
      message.warning('请填写变更原因');
      return;
    }
    Modal.confirm({
      title,
      content: (
        <div>
          <Paragraph type="secondary">此操作将影响抖店任务执行，请确认原因已记录。</Paragraph>
          <Text>原因：{reason}</Text>
        </div>
      ),
      okText: '确认',
      cancelText: '取消',
      onOk,
    });
  };

  return (
    <SectionCard title="抖店运行状态" description="暂停或紧急停用后，Worker 将不再调用抖店写接口；历史数据仍可查看。">
      <Spin spinning={loading}>
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Space wrap>
            <Text strong>当前状态</Text>
            {runtimeStatusTag(status?.status)}
            {status?.message ? <Text type="secondary">{status.message}</Text> : null}
          </Space>
          {status?.reason ? <Text type="secondary">最近变更原因：{status.reason}</Text> : null}
          {status?.changedAt ? <Text type="secondary">变更时间：{status.changedAt}</Text> : null}
          <Input.TextArea
            rows={2}
            placeholder="填写本次操作原因（必填）"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
          <ActionBar>
            <Button
              onClick={() =>
                confirmChange('暂停抖店任务？', async () => {
                  const res = await pauseDouyinRuntime(reason.trim());
                  setStatus(res);
                  setReason('');
                  message.success('已暂停');
                })
              }
            >
              暂停任务
            </Button>
            <Button
              type="primary"
              onClick={() =>
                confirmChange('恢复抖店运行？', async () => {
                  const res = await resumeDouyinRuntime(reason.trim());
                  setStatus(res);
                  setReason('');
                  message.success('已恢复运行');
                })
              }
            >
              恢复运行
            </Button>
            <Button
              danger
              onClick={() =>
                confirmChange('紧急停用抖店？', async () => {
                  const res = await emergencyDisableDouyinRuntime(reason.trim());
                  setStatus(res);
                  setReason('');
                  message.warning('已紧急停用');
                })
              }
            >
              紧急停用
            </Button>
            <Button icon={<ReloadOutlined />} onClick={() => void load()}>
              刷新
            </Button>
          </ActionBar>
        </Space>
      </Spin>
    </SectionCard>
  );
}

function DouyinPreflightPanel() {
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
        message.success('生产预检全部通过');
      } else if (res.status === 'warning') {
        message.warning(`预检完成：${res.warningCount} 项警告，${res.failedCount} 项未通过`);
      } else {
        message.error(`预检未通过：${res.failedCount} 项需处理`);
      }
    } catch (e: unknown) {
      message.error((e as Error)?.message || '运行预检失败');
    } finally {
      setRunning(false);
    }
  }, [liveTest]);

  return (
    <SectionCard title="抖店生产预检" extra={result?.checkedAt ? `最近：${result.checkedAt}` : undefined}>
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Paragraph type="secondary" style={{ marginBottom: 0 }}>
          上线前一键检查应用配置、店铺授权、功能开关、Storage 公网访问与基础数据状态。不含自动写操作。
        </Paragraph>
        <Space wrap>
          <Button type="primary" icon={<SafetyCertificateOutlined />} loading={running} onClick={() => void run()}>
            运行生产预检
          </Button>
          <Switch checked={liveTest} onChange={setLiveTest} /> <Text type="secondary">包含真实 Token 刷新联调（需已授权店铺）</Text>
        </Space>
        {result?.blockedByRealCredentials ? (
          <Alert showIcon type="info" message="部分真实联调项需 App Key / Secret 与已授权店铺，当前以结构检查为主" />
        ) : null}
        {result ? (
          <>
            <Space>
              <Text strong>总体结果</Text>
              {preflightOverallTag(result.status)}
              <Text type="secondary">
                通过 {result.passedCount} · 警告 {result.warningCount} · 未通过 {result.failedCount}
              </Text>
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
                    {c.suggestion ? <Text type="secondary">建议：{c.suggestion}</Text> : null}
                    {c.technicalDetails && Object.keys(c.technicalDetails).length > 0 ? (
                      <TechnicalDetails label="技术详情">
                        <pre style={{ margin: 0, fontSize: 12 }}>{JSON.stringify(c.technicalDetails, null, 2)}</pre>
                      </TechnicalDetails>
                    ) : null}
                  </Space>
                ),
              }))}
            />
          </>
        ) : (
          <Alert showIcon type="info" message="尚未运行预检，点击上方按钮开始检查" />
        )}
      </Space>
    </SectionCard>
  );
}

function PlatformPanel({ meta }: { meta: PlatformProviderMeta }) {
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
  const panelTitle = meta.platform === 'douyin_shop' ? '抖店接入设置' : `${meta.name} 接入设置`;

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
      message.error((e as Error)?.message || '加载失败');
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
        message.success(res.message || '连接测试通过');
      } else {
        message.warning(res.message || '连接测试未通过，请检查应用信息');
      }
    } catch (e: unknown) {
      message.error((e as Error)?.message || '连接测试失败');
    } finally {
      setTesting(false);
    }
  }, [meta.platform]);

  const connectDouyinShop = useCallback(async () => {
    setConnecting(true);
    try {
      const res = await startDouyinOAuth();
      const target = res.redirectUrl || res.authorizeUrl;
      if (!target) throw new Error('无法获取授权地址');
      window.location.href = target;
    } catch (e: unknown) {
      message.error((e as Error)?.message || '发起店铺授权失败');
      setConnecting(false);
    }
  }, []);

  const syncDouyinCategoryCache = useCallback(async () => {
    const shop = douyinShops[0];
    if (!shop?.id) {
      message.warning('请先完成抖店店铺授权，再同步类目');
      return;
    }
    setCategorySyncing(true);
    try {
      const stats = await syncDouyinCategories(shop.id);
      setDouyinStats(stats);
      message.success(`已同步 ${stats.count ?? 0} 个抖店类目`);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '同步类目失败');
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
            message="该平台能力尚未完全开放"
            description="可先填写平台应用信息，供后续店铺授权使用。"
          />
        )}

        <SectionCard title={panelTitle} description={schema?.description || undefined}>
          <div style={{ marginBottom: 12 }}>
            <Text type="secondary">接入状态 </Text>
            <Tag color={st.color}>{st.label}</Tag>
          </div>

          <Form
            layout="vertical"
            form={form}
            onFinish={async (vals: Record<string, unknown>) => {
              try {
                await putPlatformAppSettings(meta.platform, buildPutValues(fields, vals));
                message.success('设置已保存');
                await load();
              } catch (e: unknown) {
                message.error((e as Error)?.message || '保存失败');
              }
            }}
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
                <Button icon={<SyncOutlined />} onClick={() => void syncDouyinCategoryCache()} loading={categorySyncing} disabled={loading}>
                  同步类目
                </Button>
              ) : null}
              {docUrl ? (
                <Typography.Link href={docUrl} target="_blank" rel="noreferrer">
                  查看平台文档
                </Typography.Link>
              ) : null}
            </ActionBar>
          </Form>
        </SectionCard>

        {meta.platform === 'douyin_shop' ? (
          <Alert
            showIcon
            type="info"
            message="使用前请确认"
            description={
              <>
                在「存储设置」中配置抖店可访问的公网图片地址。同步订单、库存或创建商品草稿前，请在本页开启对应开关并完成店铺授权。
              </>
            }
          />
        ) : null}

        {meta.platform === 'douyin_shop' ? (
          <Alert
            showIcon
            type={douyinStats?.count ? 'success' : 'warning'}
            message="抖店类目"
            description={
              douyinStats?.count
                ? `已缓存 ${douyinStats.count} 个类目（叶子类目 ${douyinStats.leafCount ?? 0} 个），最近同步：${douyinStats.lastSyncedAt || '未知'}`
                : '暂无类目数据，请先完成店铺授权，再点击「同步类目」。'
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
  const [loadingProviders, setLoadingProviders] = useState(true);
  const [providers, setProviders] = useState<PlatformProviderMeta[]>([]);
  const [tab, setTab] = useState<string>();

  const withSchema = useMemo(() => {
    return [...providers]
      .filter((p) => p.settingsGroupKey && p.settingsGroupKey.trim())
      .sort((a, b) => preferredPlatformTabOrder(a.platform) - preferredPlatformTabOrder(b.platform));
  }, [providers]);

  const loadProviders = useCallback(async () => {
    setLoadingProviders(true);
    try {
      const { list } = await queryPlatformProviders();
      setProviders(Array.isArray(list) ? list : []);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载平台列表失败');
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
      message.success('抖店店铺授权成功');
    } else if (auth === 'failed') {
      const reason = params.get('reason');
      message.error(formatUserErrorMessage(reason, '抖店店铺授权失败'));
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
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            开发者门户：
            {PLATFORM_DEV_PORTALS.map((p, i) => (
              <span key={p.url}>
                {i > 0 ? ' · ' : ' '}
                <Typography.Link href={p.url} target="_blank" rel="noreferrer">
                  {p.name}
                </Typography.Link>
              </span>
            ))}
          </Paragraph>
        </SectionCard>

        <SectionCard title="选择平台">
          <Spin spinning={loadingProviders}>
            {items.length === 0 ? (
              <Paragraph type="secondary">暂无可配置的平台，请刷新页面后重试。</Paragraph>
            ) : (
              <Tabs activeKey={tab} onChange={setTab} items={items} />
            )}
          </Spin>
        </SectionCard>
      </div>
    </TmPageContainer>
  );
}
