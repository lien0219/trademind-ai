import { Link } from '@umijs/renderer-react';
import {
  ModalForm,
  PageContainer,
  ProFormDigit,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import {
  Alert,
  Button,
  Collapse,
  Descriptions,
  Divider,
  Drawer,
  Form,
  Input,
  Popconfirm,
  Space,
  Tag,
  Typography,
  message,
} from 'antd';
import dayjs from 'dayjs';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  PLATFORM_PROVIDER_STATUS,
  SHOP_AUTH_STATUS,
  SHOP_STATUS,
} from '@/constants/status';
import {
  createShop,
  deleteShop,
  getShop,
  getTikTokOAuthAuthorizeUrl,
  postTikTokOAuthCallback,
  getShopeeOAuthAuthorizeUrl,
  postShopeeOAuthCallback,
  getLazadaOAuthAuthorizeUrl,
  postLazadaOAuthCallback,
  getAmazonOAuthAuthorizeUrl,
  postAmazonOAuthCallback,
  queryPlatformProviders,
  queryShops,
  testShopConnection,
  updateShop,
  updateShopAuth,
  type PlatformProviderMeta,
  type ShopDetail,
  type ShopListRow,
} from '@/services/shops';
import { syncShopOrders } from '@/services/orderSync';
import { getPlatformAppSettings } from '@/services/platformOpen';
import { isDeployAppConfigComplete } from '@/utils/platformAppConfig';

const STD_AUTH_KEYS = new Set([
  'appKey',
  'appSecret',
  'accessToken',
  'refreshToken',
  'sellerId',
  'merchantId',
  'marketplaceId',
  'redirectUri',
]);

function providerLabel(list: PlatformProviderMeta[], platform: string) {
  const p = list.find((x) => x.platform === platform);
  return p ? `${p.name} (${p.platform})` : platform;
}

function tagFromMap(raw: string, map: Record<string, { text: string; color: string }>) {
  const c = map[raw as keyof typeof map];
  if (!c) return <Tag>{raw}</Tag>;
  return <Tag color={c.color as never}>{c.text}</Tag>;
}

function summarizeShopTest(res: {
  ok: boolean;
  message?: string;
  shopName?: string;
  externalShopId?: string;
  region?: string;
}) {
  const parts = [
    res.message,
    res.shopName ? `店铺 ${res.shopName}` : '',
    res.region ? `地区 ${res.region}` : '',
    res.externalShopId ? `外部ID ${res.externalShopId}` : '',
  ].filter(Boolean);
  return parts.join(' · ') || '连接成功';
}

/** Map incomplete platform Partner Open settings errors → CN hints (no secrets). */
function formatPlatformPartnerErr(err: unknown): string {
  const msg = err instanceof Error ? err.message : String(err);
  const low = msg.toLowerCase();

  if (msg.includes('required setting missing:')) {
    return `${msg}\n请先到「设置 → 平台开放配置」补齐该平台应用参数的必填项。`;
  }

  if (msg.includes('platform config incomplete: please configure platform_tiktok')) {
    return `${msg}\n请先到「设置 → 平台开放配置 → TikTok Shop」填写 App Key、App Secret 和 Redirect URI。`;
  }
  if (msg.includes('platform config incomplete: please configure platform_shopee')) {
    return `${msg}\n请先到「设置 → 平台开放配置 → Shopee」填写 Partner ID、Partner Key 和 Redirect URI。`;
  }
  if (msg.includes('platform config incomplete: please configure platform_lazada')) {
    return `${msg}\n请先到「设置 → 平台开放配置 → Lazada」填写 App Key、App Secret 和 Redirect URI。`;
  }
  if (msg.includes('platform config incomplete: please configure platform_amazon')) {
    return `${msg}\n请先到「设置 → 平台开放配置 → Amazon SP-API」填写 Client ID、Client Secret、Redirect URI、Marketplace ID 和 SP-API Base URL。`;
  }
  if (msg.includes('platform_amazon.lwa_auth_base_url and lwa_token_url')) {
    return `${msg}\n请在「Amazon SP-API」配置中补齐 LWA Auth Base URL 与 LWA Token URL。`;
  }
  if (
    msg.includes(
      'TikTok platform config is incomplete. Please configure App Key, App Secret and Redirect URI first.',
    )
  ) {
    return `${msg}\n请先前往「设置 → 平台开放配置」填写 TikTok Shop（分组 platform_tiktok）必填项后再试。`;
  }
  if (low.includes('tiktok platform config is incomplete') || low.includes('platform_tiktok')) {
    return `${msg}\n请到「设置 → 平台开放配置」完成 TikTok Shop 必填项后再试。`;
  }
  return msg;
}

export default function ShopsPage() {
  const actionRef = useRef<ActionType>();
  const [providers, setProviders] = useState<PlatformProviderMeta[]>([]);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<ShopDetail | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [authOpen, setAuthOpen] = useState(false);
  const [authForm] = Form.useForm();
  const [syncOpen, setSyncOpen] = useState(false);
  const [syncTarget, setSyncTarget] = useState<{ id: string; platform: string } | null>(null);
  const [tiktokOAuthAuthorizeUrl, setTikTokOAuthAuthorizeUrl] = useState('');
  const [tiktokOAuthState, setTikTokOAuthState] = useState('');
  const [shopeeOAuthAuthorizeUrl, setShopeeOAuthAuthorizeUrl] = useState('');
  const [shopeeOAuthState, setShopeeOAuthState] = useState('');
  const [lazadaOAuthAuthorizeUrl, setLazadaOAuthAuthorizeUrl] = useState('');
  const [lazadaOAuthState, setLazadaOAuthState] = useState('');
  const [amazonOAuthAuthorizeUrl, setAmazonOAuthAuthorizeUrl] = useState('');
  const [amazonOAuthState, setAmazonOAuthState] = useState('');
  const [authPartnerWarn, setAuthPartnerWarn] = useState<string | null>(null);

  const loadProviders = useCallback(async () => {
    const res = await queryPlatformProviders();
    setProviders(res.list || []);
  }, []);

  useEffect(() => {
    void loadProviders();
  }, [loadProviders]);

  const refreshDetail = async (id: string) => {
    const d = await getShop(id);
    setDetail(d);
  };

  const openDetail = async (row: ShopListRow) => {
    setDetailOpen(true);
    await refreshDetail(row.id);
  };

  const provForShop = useMemo(() => {
    if (!detail) return undefined;
    return providers.find((p) => p.platform === detail.platform);
  }, [detail, providers]);

  const openAuthFor = async (shopId: string) => {
    const d = await getShop(shopId);
    setDetail(d);
    const p = providers.find((x) => x.platform === d.platform);
    setAuthPartnerWarn(null);
    if (p?.settingsGroupKey && d.platform !== 'manual') {
      try {
        const row = await getPlatformAppSettings(p.platform);
        if (!isDeployAppConfigComplete(row.schema ?? p.appConfigSchema, row.values)) {
          setAuthPartnerWarn(
            `请先到「设置 → 平台开放配置」填写「${p.name}」开放平台应用参数（分组 ${p.settingsGroupKey}），再完成店铺授权。`,
          );
        }
      } catch {
        setAuthPartnerWarn('无法校验平台应用配置，请检查网络或稍后重试。');
      }
    }
    const base: Record<string, unknown> = {
      authType: d.auth?.authType || p?.authType || 'token',
      appKey: d.auth?.appKey || '',
      appSecret: d.auth?.appSecret || '',
      accessToken: d.auth?.accessToken || '',
      refreshToken: d.auth?.refreshToken || '',
      sellerId: d.auth?.sellerId || '',
      merchantId: d.auth?.merchantId || '',
      marketplaceId: d.auth?.marketplaceId || '',
      expiresAt: d.auth?.expiresAt ? String(d.auth.expiresAt) : '',
      refreshExpiresAt: d.auth?.refreshExpiresAt ? String(d.auth.refreshExpiresAt) : '',
    };
    if (d.auth?.authConfig && typeof d.auth.authConfig === 'object') {
      Object.assign(base, d.auth.authConfig);
    }
    if (p?.authSchema?.length) {
      for (const f of p.authSchema) {
        if (base[f.name] === undefined) base[f.name] = '';
      }
    }
    authForm.resetFields();
    authForm.setFieldsValue(base);
    setTikTokOAuthAuthorizeUrl('');
    setTikTokOAuthState('');
    setShopeeOAuthAuthorizeUrl('');
    setShopeeOAuthState('');
    setLazadaOAuthAuthorizeUrl('');
    setLazadaOAuthState('');
    setAmazonOAuthAuthorizeUrl('');
    setAmazonOAuthState('');
    setAuthOpen(true);
  };

  const buildAuthPayload = (values: Record<string, unknown>) => {
    const authType = String(values.authType || provForShop?.authType || 'token');
    const payload: Record<string, unknown> = { authType };
    const authConfig: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(values)) {
      if (k === 'authType') continue;
      if (v === undefined || v === null) continue;
      const out = v;
      if (out === '') continue;
      if (STD_AUTH_KEYS.has(k)) {
        payload[k] = out;
      } else if (k === 'expiresAt' || k === 'refreshExpiresAt') {
        payload[k] = out;
      } else {
        authConfig[k] = out;
      }
    }
    if (Object.keys(authConfig).length) payload.authConfig = authConfig;
    return payload;
  };

  const openOrderSyncModal = (platform: string, shopId: string) => {
    const p = providers.find((x) => x.platform === platform);
    if (platform === 'manual') {
      message.warning('手工店铺不支持订单同步');
      return;
    }
    if (p?.status === 'planned') {
      message.warning('平台订单同步暂未实现');
      return;
    }
    setSyncTarget({ id: shopId, platform });
    setSyncOpen(true);
  };

  const columns: ProColumns<ShopListRow>[] = useMemo(
    () => [
      { title: '平台', dataIndex: 'platform', width: 120, copyable: true },
      { title: '店铺名', dataIndex: 'shopName', ellipsis: true, width: 160 },
      { title: '编码', dataIndex: 'shopCode', width: 120, search: false },
      {
        title: '状态',
        dataIndex: 'status',
        width: 88,
        valueType: 'select',
        valueEnum: Object.fromEntries(Object.entries(SHOP_STATUS).map(([k, v]) => [k, { text: v.text }])),
        render: (_, r) => tagFromMap(r.status, SHOP_STATUS),
      },
      {
        title: '授权',
        dataIndex: 'authStatus',
        width: 96,
        valueType: 'select',
        valueEnum: Object.fromEntries(Object.entries(SHOP_AUTH_STATUS).map(([k, v]) => [k, { text: v.text }])),
        render: (_, r) => tagFromMap(r.authStatus, SHOP_AUTH_STATUS),
      },
      { title: '地区', dataIndex: 'region', width: 72, search: false },
      { title: '币种', dataIndex: 'currency', width: 72, search: false },
      {
        title: '能力',
        dataIndex: 'capabilities',
        search: false,
        ellipsis: true,
        render: (_, r) => {
          const c = r.capabilities;
          if (Array.isArray(c)) return <Typography.Text ellipsis>{c.join(', ')}</Typography.Text>;
          return '—';
        },
      },
      {
        title: '更新时间',
        dataIndex: 'updatedAt',
        width: 168,
        search: false,
        render: (_, r) => dayjs(r.updatedAt).format('YYYY-MM-DD HH:mm'),
      },
      {
        title: '操作',
        valueType: 'option',
        width: 340,
        render: (_, r) => [
          <a key="v" onClick={() => void openDetail(r)}>
            查看
          </a>,
          <a
            key="e"
            onClick={async () => {
              const d = await getShop(r.id);
              setDetail(d);
              setEditOpen(true);
            }}
          >
            编辑
          </a>,
          <a key="a" onClick={() => void openAuthFor(r.id)}>
            授权配置
          </a>,
          <a
            key="osync"
            onClick={() => {
              openOrderSyncModal(r.platform, r.id);
            }}
          >
            同步订单
          </a>,
          <Link key="olog" to={`/orders/sync-tasks?shopId=${encodeURIComponent(r.id)}`}>
            同步记录
          </Link>,
          <a
            key="t"
            onClick={async () => {
              try {
                const res = await testShopConnection(r.id);
                message.success(summarizeShopTest(res));
              } catch (e: unknown) {
                message.error(formatPlatformPartnerErr(e));
              }
            }}
          >
            测试连接
          </a>,
          <Popconfirm
            key="d"
            title="删除店铺？"
            onConfirm={async () => {
              await deleteShop(r.id);
              message.success('已删除');
              actionRef.current?.reload();
            }}
          >
            <a style={{ color: 'var(--ant-color-error)' }}>删除</a>
          </Popconfirm>,
        ],
      },
    ],
    [providers],
  );

  return (
    <PageContainer title="店铺管理">
      <ProTable<ShopListRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        headerTitle="店铺列表"
        toolBarRender={() => [
          <Button key="n" type="primary" onClick={() => setCreateOpen(true)}>
            新建店铺
          </Button>,
        ]}
        request={async (params) => {
          const res = await queryShops({
            page: params.current,
            pageSize: params.pageSize,
            platform: params.platform as string | undefined,
            status: params.status as string | undefined,
            authStatus: params.authStatus as string | undefined,
            shopName: params.shopName as string | undefined,
          });
          return { data: res.list, success: true, total: res.pagination.total };
        }}
      />

      <ModalForm
        title="新建店铺"
        open={createOpen}
        modalProps={{ destroyOnClose: true, onCancel: () => setCreateOpen(false) }}
        onFinish={async (vals) => {
          const plat = vals.platform as string;
          const meta = providers.find((x) => x.platform === plat);
          if (meta?.settingsGroupKey && plat !== 'manual') {
            try {
              const row = await getPlatformAppSettings(meta.platform);
              if (!isDeployAppConfigComplete(row.schema ?? meta.appConfigSchema, row.values)) {
                message.error(
                  `请先到「设置 → 平台开放配置」填写「${meta.name}」应用参数后再创建店铺。`,
                );
                return false;
              }
            } catch (e: unknown) {
              message.error(e instanceof Error ? e.message : '加载平台配置失败');
              return false;
            }
          }
          await createShop({
            platform: plat,
            shopName: vals.shopName as string,
            shopCode: vals.shopCode as string | undefined,
            region: vals.region as string | undefined,
            currency: vals.currency as string | undefined,
            timezone: vals.timezone as string | undefined,
            defaultLanguage: vals.defaultLanguage as string | undefined,
            remark: vals.remark as string | undefined,
          });
          message.success('已创建');
          setCreateOpen(false);
          actionRef.current?.reload();
          return true;
        }}
      >
        <ProFormSelect
          name="platform"
          label="销售平台"
          rules={[{ required: true }]}
          options={providers.map((p) => ({
            label: `${p.name} (${p.platform}) — ${PLATFORM_PROVIDER_STATUS[p.status as keyof typeof PLATFORM_PROVIDER_STATUS]?.text ?? p.status}`,
            value: p.platform,
          }))}
          fieldProps={{ showSearch: true, optionFilterProp: 'label' }}
        />
        <ProFormText name="shopName" label="店铺名称" rules={[{ required: true }]} />
        <ProFormText name="shopCode" label="店铺编码" />
        <ProFormText name="region" label="地区" />
        <ProFormText name="currency" label="币种" placeholder="USD" />
        <ProFormText name="timezone" label="时区" placeholder="America/Los_Angeles" />
        <ProFormText name="defaultLanguage" label="默认语言" placeholder="en" />
        <ProFormTextArea name="remark" label="备注" fieldProps={{ rows: 2 }} />
      </ModalForm>

      <ModalForm
        title="编辑店铺"
        open={editOpen}
        initialValues={
          detail
            ? {
                shopName: detail.shopName,
                shopCode: detail.shopCode,
                region: detail.region,
                currency: detail.currency,
                timezone: detail.timezone,
                defaultLanguage: detail.defaultLanguage,
                remark: detail.remark,
                status: detail.status,
              }
            : undefined
        }
        key={detail?.id ?? 'edit'}
        modalProps={{ destroyOnClose: true, onCancel: () => setEditOpen(false) }}
        onFinish={async (vals) => {
          if (!detail) return false;
          await updateShop(detail.id, {
            shopName: vals.shopName as string,
            shopCode: vals.shopCode as string | undefined,
            region: vals.region as string | undefined,
            currency: vals.currency as string | undefined,
            timezone: vals.timezone as string | undefined,
            defaultLanguage: vals.defaultLanguage as string | undefined,
            remark: vals.remark as string | undefined,
            status: vals.status as string,
          });
          message.success('已保存');
          setEditOpen(false);
          actionRef.current?.reload();
          if (detailOpen) void refreshDetail(detail.id);
          return true;
        }}
      >
        <ProFormText name="shopName" label="店铺名称" rules={[{ required: true }]} />
        <ProFormText name="shopCode" label="店铺编码" />
        <ProFormText name="region" label="地区" />
        <ProFormText name="currency" label="币种" />
        <ProFormText name="timezone" label="时区" />
        <ProFormText name="defaultLanguage" label="默认语言" />
        <ProFormSelect
          name="status"
          label="状态"
          options={[
            { label: '启用', value: 'active' },
            { label: '停用', value: 'disabled' },
          ]}
          rules={[{ required: true }]}
        />
        <ProFormTextArea name="remark" label="备注" fieldProps={{ rows: 2 }} />
      </ModalForm>

      <ModalForm
        title="同步店铺订单"
        open={syncOpen}
        modalProps={{
          destroyOnClose: true,
          onCancel: () => {
            setSyncOpen(false);
            setSyncTarget(null);
          },
        }}
        initialValues={{ mode: 'incremental', limit: 50, cursor: '', start: '', end: '' }}
        onFinish={async (vals) => {
          if (!syncTarget) return false;
          try {
            await syncShopOrders(syncTarget.id, {
              mode: vals.mode as string,
              start: (vals.start as string | undefined) || undefined,
              end: (vals.end as string | undefined) || undefined,
              cursor: (vals.cursor as string | undefined) || undefined,
              limit: vals.limit as number | undefined,
            });
          } catch (e: unknown) {
            message.error(e instanceof Error ? e.message : '同步失败');
            return false;
          }
          message.success('订单同步任务已提交');
          setSyncOpen(false);
          setSyncTarget(null);
          return true;
        }}
      >
        <ProFormRadio.Group
          name="mode"
          label="同步模式"
          options={[
            { label: '增量 incremental', value: 'incremental' },
            { label: '全量 full', value: 'full' },
          ]}
          rules={[{ required: true }]}
        />
        <ProFormText name="start" label="开始时间（可选 RFC3339）" placeholder="2026-05-01T00:00:00Z" />
        <ProFormText name="end" label="结束时间（可选 RFC3339）" placeholder="2026-05-16T23:59:59Z" />
        <ProFormText name="cursor" label="游标（可选）" />
        <ProFormDigit name="limit" label="每页条数" min={1} max={200} fieldProps={{ precision: 0 }} />
      </ModalForm>

      <Drawer
        title={detail ? `店铺：${detail.shopName}` : '店铺详情'}
        width={640}
        open={detailOpen}
        onClose={() => {
          setDetailOpen(false);
          setDetail(null);
        }}
        destroyOnClose
        extra={
          detail ? (
            <Space>
              <Button
                onClick={() => {
                  setEditOpen(true);
                }}
              >
                编辑
              </Button>
              <Button type="primary" onClick={() => detail && void openAuthFor(detail.id)}>
                授权配置
              </Button>
              <Button
                onClick={() => {
                  if (!detail) return;
                  openOrderSyncModal(detail.platform, detail.id);
                }}
              >
                同步订单
              </Button>
              <Link to={`/orders/sync-tasks?shopId=${encodeURIComponent(detail?.id ?? '')}`}>
                <Button disabled={!detail}>同步记录</Button>
              </Link>
              <Button
                onClick={async () => {
                  try {
                    const res = await testShopConnection(detail.id);
                    message.success(summarizeShopTest(res));
                  } catch (e: unknown) {
                    message.error(formatPlatformPartnerErr(e));
                  }
                }}
              >
                测试连接
              </Button>
            </Space>
          ) : null
        }
      >
        {detail && provForShop && (
          <>
            {provForShop.status === 'planned' && (
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message="平台暂未接入"
                description="可先创建店铺占位；真实 OAuth / 订单同步需后续实现对应 Platform Provider。"
              />
            )}
            {detail.platform === 'tiktok' && provForShop.status === 'beta' && (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 12 }}
                message="TikTok Shop（Beta）"
                description="支持 OAuth、测试连接与订单同步。请先在「授权配置」保存 App Key / App Secret 与 Redirect URI，再生成授权链接并完成授权。"
              />
            )}
            {detail.platform === 'shopee' && provForShop.status === 'beta' && (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 12 }}
                message="Shopee（Beta）"
                description="支持 OAuth、测试连接与订单同步。请先在「设置 → 平台开放配置 → Shopee」填写 Partner ID、Partner Key 与 Redirect URI，再在「授权配置」完成授权。"
              />
            )}
            {detail.platform === 'lazada' && provForShop.status === 'beta' && (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 12 }}
                message="Lazada（测试中）"
                description="支持 OAuth、测试连接与订单同步。请先在「设置 → 平台开放配置 → Lazada」填写 App Key、App Secret、Auth/API Base URL 与 Redirect URI，再在「授权配置」完成授权。"
              />
            )}
            {detail.platform === 'amazon' && provForShop.status === 'beta' && (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 12 }}
                message="Amazon SP-API（测试中）"
                description="支持 LWA OAuth、SigV4 调用 SP-API、测试连接与订单同步。请先在「设置 → 平台开放配置 → Amazon SP-API」填写完整应用参数；服务器需具备 AWS IAM 凭证以对 SP-API 请求做 SigV4 签名（环境/实例/Task 角色或 role_arn）。"
              />
            )}
            <Descriptions bordered size="small" column={2}>
              <Descriptions.Item label="平台">{providerLabel(providers, detail.platform)}</Descriptions.Item>
              <Descriptions.Item label="platform id">{detail.platform}</Descriptions.Item>
              <Descriptions.Item label="状态">{tagFromMap(detail.status, SHOP_STATUS)}</Descriptions.Item>
              <Descriptions.Item label="授权">{tagFromMap(detail.authStatus, SHOP_AUTH_STATUS)}</Descriptions.Item>
              <Descriptions.Item label="店铺编码">{detail.shopCode || '—'}</Descriptions.Item>
              <Descriptions.Item label="地区">{detail.region || '—'}</Descriptions.Item>
              <Descriptions.Item label="币种">{detail.currency || '—'}</Descriptions.Item>
              <Descriptions.Item label="时区">{detail.timezone || '—'}</Descriptions.Item>
              <Descriptions.Item label="语言">{detail.defaultLanguage || '—'}</Descriptions.Item>
              <Descriptions.Item label="外部店铺 ID">{detail.externalShopId || '—'}</Descriptions.Item>
              <Descriptions.Item label="备注" span={2}>
                {detail.remark || '—'}
              </Descriptions.Item>
            </Descriptions>
          </>
        )}
      </Drawer>

      <Drawer
        title="授权配置"
        width={640}
        open={authOpen}
        onClose={() => setAuthOpen(false)}
        destroyOnClose
        extra={
          detail ? (
            <Button
              type="primary"
              onClick={async () => {
                try {
                  const v = await authForm.validateFields();
                  const payload = buildAuthPayload(v as Record<string, unknown>);
                  await updateShopAuth(detail.id, payload);
                  message.success('授权已保存');
                  setAuthOpen(false);
                  actionRef.current?.reload();
                  if (detailOpen) void refreshDetail(detail.id);
                } catch (e: unknown) {
                  if (e instanceof Error) message.error(e.message);
                }
              }}
            >
              保存
            </Button>
          ) : null
        }
      >
        {detail && (
          <>
            {authPartnerWarn && (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 12 }}
                message="平台开放配置可能不完整"
                description={authPartnerWarn}
              />
            )}
            {detail.platform === 'manual' && (
              <Alert type="success" showIcon message="手工店铺无需授权" description="无需配置 Token / Secret。" />
            )}
            {detail.platform !== 'manual' && provForShop?.status === 'planned' && (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 12 }}
                message="平台接入规划中"
                description="可预先填写占位字段；测试连接将返回「未实现」。不真实访问平台 API。"
              />
            )}
            {detail.platform === 'tiktok' && provForShop?.status === 'beta' && (
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message="TikTok OAuth 提示"
                description={
                  <>
                    Partner 应用在{' '}
                    <Link to="/settings/platforms">平台开放配置（platform_tiktok）</Link>{' '}
                    填写；OAuth <code style={{ padding: '0 4px' }}>state</code> 存 Redis，
                    请确保本地已启动 Redis。生成链接默认使用该配置；仅在展开「可选覆盖」时才会使用本页的 App Key /
                    Secret。
                  </>
                }
              />
            )}
            {detail.platform === 'shopee' && provForShop?.status === 'beta' && (
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message="Shopee OAuth 提示"
                description={
                  <>
                    Partner 参数在{' '}
                    <Link to="/settings/platforms">平台开放配置（platform_shopee）</Link>{' '}
                    填写；<code style={{ padding: '0 4px' }}>state</code> 存 Redis；
                    Shopee 跳转回调一般会带 <code style={{ padding: '0 4px' }}>shop_id</code>
                    ，请一并粘贴到提交表单。
                  </>
                }
              />
            )}
            {detail.platform === 'lazada' && provForShop?.status === 'beta' && (
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message="Lazada OAuth 提示"
                description={
                  <>
                    应用在{' '}
                    <Link to="/settings/platforms">平台开放配置（platform_lazada）</Link>{' '}
                    填写；<code style={{ padding: '0 4px' }}>state</code> 存 Redis；
                    授权完成后从回调 URL 复制 <code style={{ padding: '0 4px' }}>code</code> 与签发的 state 提交即可。
                  </>
                }
              />
            )}
            {detail.platform === 'amazon' && provForShop?.status === 'beta' && (
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message="Amazon OAuth 提示"
                description={
                  <>
                    LWA 应用在{' '}
                    <Link to="/settings/platforms">平台开放配置（platform_amazon）</Link>{' '}
                    填写；<code style={{ padding: '0 4px' }}>state</code> 存 Redis。
                    授权完成后从回调拷贝 <code style={{ padding: '0 4px' }}>spapi_oauth_code</code>（作为 code）以及{' '}
                    <code style={{ padding: '0 4px' }}>selling_partner_id</code> 等参数提交。前端不直连 Amazon API。
                  </>
                }
              />
            )}
            {detail.platform !== 'manual' && (
              <Form form={authForm} layout="vertical">
                <Form.Item name="authType" label="授权类型" hidden>
                  <Input />
                </Form.Item>
                <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
                  密钥字段已脱敏展示为 ****；不修改请留空或保持原样，保存时不覆盖。
                </Typography.Text>
                {detail.platform === 'tiktok' ? (
                  <Collapse
                    bordered={false}
                    style={{ marginBottom: 12 }}
                    items={[
                      {
                        key: 'tiktok_ov',
                        label: '可选：覆盖 Partner App Key / Secret / Redirect URI',
                        children: (
                          <>
                            <Form.Item
                              name="appKey"
                              label="覆盖 App Key"
                              tooltip="多数情况留空即可，使用「平台开放配置」中的默认值"
                            >
                              <Input autoComplete="off" />
                            </Form.Item>
                            <Form.Item
                              name="appSecret"
                              label="覆盖 App Secret"
                              tooltip="留空以保持平台配置的 Secret；仅在多应用并行调试时使用"
                            >
                              <Input.Password autoComplete="new-password" />
                            </Form.Item>
                            <Form.Item
                              name="redirectUri"
                              label="覆盖 Redirect URI"
                              tooltip="留空则用平台配置的 redirect_uri；必须与 Partner Center 登记的回调完全一致"
                            >
                              <Input placeholder="https://…" />
                            </Form.Item>
                          </>
                        ),
                      },
                    ]}
                  />
                ) : detail.platform === 'shopee' ? (
                  <Collapse
                    bordered={false}
                    style={{ marginBottom: 12 }}
                    items={[
                      {
                        key: 'shopee_ov',
                        label: '可选：覆盖 Partner ID / Partner Key / Redirect URI',
                        children: (
                          <>
                            <Form.Item
                              name="appKey"
                              label="覆盖 Partner ID"
                              tooltip="留空则使用「平台开放配置」中的 partner_id"
                            >
                              <Input autoComplete="off" />
                            </Form.Item>
                            <Form.Item
                              name="appSecret"
                              label="覆盖 Partner Key"
                              tooltip="留空则使用平台配置中的 partner_key（加密存储）"
                            >
                              <Input.Password autoComplete="new-password" />
                            </Form.Item>
                            <Form.Item
                              name="redirectUri"
                              label="覆盖 Redirect URI"
                              tooltip="留空则用平台配置的 redirect_uri；须与 Shopee Open Platform 登记一致"
                            >
                              <Input placeholder="https://…" />
                            </Form.Item>
                          </>
                        ),
                      },
                    ]}
                  />
                ) : detail.platform === 'lazada' ? (
                  <Collapse
                    bordered={false}
                    style={{ marginBottom: 12 }}
                    items={[
                      {
                        key: 'lazada_ov',
                        label: '可选：覆盖 App Key / App Secret / Redirect URI',
                        children: (
                          <>
                            <Form.Item
                              name="appKey"
                              label="覆盖 App Key"
                              tooltip="留空则使用「平台开放配置」中的 app_key"
                            >
                              <Input autoComplete="off" />
                            </Form.Item>
                            <Form.Item
                              name="appSecret"
                              label="覆盖 App Secret"
                              tooltip="留空以保持平台配置的 app_secret（加密存储）"
                            >
                              <Input.Password autoComplete="new-password" />
                            </Form.Item>
                            <Form.Item
                              name="redirectUri"
                              label="覆盖 Redirect URI"
                              tooltip="留空则用平台配置的 redirect_uri；须与 Lazada App Console 登记一致"
                            >
                              <Input placeholder="https://…" />
                            </Form.Item>
                          </>
                        ),
                      },
                    ]}
                  />
                ) : detail.platform === 'amazon' ? (
                  <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
                    LWA Client ID / Secret、Redirect URI、SP-API 与 LWA 端点均在「
                    <Link to="/settings/platforms">平台开放配置</Link>」维护；本页保存店铺 Token 与 Selling Partner 标识。
                  </Typography.Paragraph>
                ) : (
                  <>
                    <Form.Item name="appKey" label="App Key">
                      <Input />
                    </Form.Item>
                    <Form.Item name="appSecret" label="App Secret">
                      <Input.Password autoComplete="new-password" placeholder="敏感，留空不更新" />
                    </Form.Item>
                  </>
                )}
                <Form.Item name="accessToken" label="Access Token（可手工填入调试）">
                  <Input.Password autoComplete="new-password" />
                </Form.Item>
                <Form.Item name="refreshToken" label="Refresh Token（可手工填入）">
                  <Input.Password autoComplete="new-password" />
                </Form.Item>
                <Form.Item
                  name="expiresAt"
                  label="Access Token 过期时间（RFC3339，可选）"
                  tooltip="示例 2026-06-01T00:00:00Z"
                >
                  <Input placeholder="2026-06-01T00:00:00Z" />
                </Form.Item>
                <Form.Item name="refreshExpiresAt" label="Refresh Token 过期（RFC3339，可选）">
                  <Input />
                </Form.Item>
                <Form.Item
                  name="sellerId"
                  label={
                    detail.platform === 'tiktok'
                      ? 'Seller Id（TikTok 通常不用填）'
                      : detail.platform === 'shopee'
                        ? 'Shopee shop_id（OAuth 成功后写入；可手工改）'
                        : detail.platform === 'lazada'
                          ? 'Seller / short_code（OAuth 后可写入；可手工改）'
                          : detail.platform === 'amazon'
                            ? 'Selling Partner Id（OAuth 回调 selling_partner_id）'
                            : 'Seller Id'
                  }
                  tooltip={
                    detail.platform === 'shopee'
                      ? '与 Shopee 回调 URL 参数 shop_id 一致，用于 OpenAPI 签名。'
                      : detail.platform === 'lazada'
                        ? 'Lazada 店铺标识；OAuth 成功后通常会自动写入。'
                        : detail.platform === 'amazon'
                          ? 'Amazon SP-API 卖家编号；授权回调必填。'
                          : undefined
                  }
                >
                  <Input />
                </Form.Item>
                <Form.Item
                  name="merchantId"
                  label={
                    detail.platform === 'tiktok'
                      ? 'Shop cipher（merchant_id）'
                      : detail.platform === 'shopee'
                        ? 'Main account id（可选）'
                        : detail.platform === 'lazada'
                          ? '扩展信息（可选）'
                          : detail.platform === 'amazon'
                            ? '扩展（可选）'
                            : 'Merchant Id'
                  }
                  tooltip={
                    detail.platform === 'tiktok'
                      ? 'OAuth 成功后通常会自动写入；仅在手工调试时粘贴。'
                      : detail.platform === 'shopee'
                        ? '跨境/主帐号场景可选；OAuth 回调可一并提交。'
                        : detail.platform === 'lazada'
                          ? 'OAuth 回调 country / account 摘要可写入 auth_config。'
                          : undefined
                  }
                >
                  <Input />
                </Form.Item>
                <Form.Item name="marketplaceId" label="Marketplace Id">
                  <Input
                    placeholder={
                      detail.platform === 'amazon' ? '可覆盖 platform_amazon 默认 Marketplace ID' : undefined
                    }
                  />
                </Form.Item>
                {detail.platform === 'tiktok' && (
                  <>
                    <Divider>TikTok OAuth</Divider>
                    <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
                      若填写「可选覆盖」中的字段，会先同步到服务端再生成链接（留空则不发送覆盖项）。默认直接使用「平台开放配置」。
                    </Typography.Paragraph>
                    <Space wrap>
                      <Button
                        type="primary"
                        onClick={async () => {
                          if (authPartnerWarn) {
                            message.warning('请先完成「平台开放配置」中的必填项。');
                            return;
                          }
                          try {
                            const vals = authForm.getFieldsValue(true) as Record<string, unknown>;
                            const rd = String(vals.redirectUri || '').trim();
                            const pk = String(vals.appKey || '').trim();
                            const ps = String(vals.appSecret || '').trim();
                            if (pk || ps || rd) {
                              await updateShopAuth(detail.id, buildAuthPayload(vals));
                            }
                            const r = await getTikTokOAuthAuthorizeUrl(detail.id, rd || undefined);
                            setTikTokOAuthAuthorizeUrl(r.authorizeUrl);
                            setTikTokOAuthState(r.state);
                            message.success('已生成授权链接');
                          } catch (e: unknown) {
                            message.error(formatPlatformPartnerErr(e));
                          }
                        }}
                      >
                        生成授权链接
                      </Button>
                      <Button
                        onClick={() => {
                          if (!tiktokOAuthAuthorizeUrl) {
                            message.warning('请先生成授权链接');
                            return;
                          }
                          window.open(tiktokOAuthAuthorizeUrl, '_blank', 'noopener,noreferrer');
                        }}
                      >
                        新窗口打开
                      </Button>
                    </Space>
                    <Form.Item label="authorizeUrl">
                      <Space align="start">
                        <Input.TextArea
                          style={{ width: 460 }}
                          autoSize={{ minRows: 2, maxRows: 6 }}
                          readOnly
                          value={tiktokOAuthAuthorizeUrl}
                          placeholder='点击上方「生成授权链接」后出现'
                        />
                        <Button
                          disabled={!tiktokOAuthAuthorizeUrl}
                          onClick={() => {
                            void navigator.clipboard?.writeText?.(tiktokOAuthAuthorizeUrl);
                            message.success('已复制链接');
                          }}
                        >
                          复制
                        </Button>
                      </Space>
                    </Form.Item>
                    <Form.Item label="state（服务端签发）">
                      <Input readOnly value={tiktokOAuthState} placeholder='生成后出现' />
                    </Form.Item>
                    <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
                      用户在 TikTok 授权后复制返回的 code，填入下方并提交：
                    </Typography.Text>
                    <Form.Item name="tiktokAuthCode">
                      <Input placeholder="Paste authorization code" />
                    </Form.Item>
                    <Button
                      type="default"
                      onClick={async () => {
                        const vals = authForm.getFieldsValue();
                        const code = String(vals.tiktokAuthCode || '').trim();
                        if (!detail?.id) return;
                        if (!code || !tiktokOAuthState) {
                          message.warning('需要 code 与已生成的 state');
                          return;
                        }
                        try {
                          await postTikTokOAuthCallback(detail.id, {
                            code,
                            state: tiktokOAuthState,
                          });
                          message.success('TikTok 授权已写入');
                          setAuthOpen(false);
                          actionRef.current?.reload();
                          if (detailOpen) void refreshDetail(detail.id);
                        } catch (e: unknown) {
                          message.error(formatPlatformPartnerErr(e));
                        }
                      }}
                    >
                      提交授权 code + state
                    </Button>
                    <Divider />
                  </>
                )}
                {detail.platform === 'shopee' && provForShop?.status === 'beta' && (
                  <>
                    <Divider>Shopee OAuth</Divider>
                    <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
                      若填写「可选覆盖」中的字段，会先同步到服务端再生成链接。默认使用「平台开放配置」中的 partner 参数。
                    </Typography.Paragraph>
                    <Space wrap>
                      <Button
                        type="primary"
                        onClick={async () => {
                          if (authPartnerWarn) {
                            message.warning('请先完成「平台开放配置」中的必填项。');
                            return;
                          }
                          try {
                            const vals = authForm.getFieldsValue(true) as Record<string, unknown>;
                            const rd = String(vals.redirectUri || '').trim();
                            const pk = String(vals.appKey || '').trim();
                            const ps = String(vals.appSecret || '').trim();
                            if (pk || ps || rd) {
                              await updateShopAuth(detail.id, buildAuthPayload(vals));
                            }
                            const r = await getShopeeOAuthAuthorizeUrl(detail.id, rd || undefined);
                            setShopeeOAuthAuthorizeUrl(r.authorizeUrl);
                            setShopeeOAuthState(r.state);
                            message.success('已生成 Shopee 授权链接');
                          } catch (e: unknown) {
                            message.error(formatPlatformPartnerErr(e));
                          }
                        }}
                      >
                        生成授权链接
                      </Button>
                      <Button
                        onClick={() => {
                          if (!shopeeOAuthAuthorizeUrl) {
                            message.warning('请先生成授权链接');
                            return;
                          }
                          window.open(shopeeOAuthAuthorizeUrl, '_blank', 'noopener,noreferrer');
                        }}
                      >
                        新窗口打开
                      </Button>
                    </Space>
                    <Form.Item label="authorizeUrl">
                      <Space align="start">
                        <Input.TextArea
                          style={{ width: 460 }}
                          autoSize={{ minRows: 2, maxRows: 6 }}
                          readOnly
                          value={shopeeOAuthAuthorizeUrl}
                          placeholder='点击上方「生成授权链接」后出现'
                        />
                        <Button
                          disabled={!shopeeOAuthAuthorizeUrl}
                          onClick={() => {
                            void navigator.clipboard?.writeText?.(shopeeOAuthAuthorizeUrl);
                            message.success('已复制链接');
                          }}
                        >
                          复制
                        </Button>
                      </Space>
                    </Form.Item>
                    <Form.Item label="state（服务端签发）">
                      <Input readOnly value={shopeeOAuthState} placeholder='生成后出现' />
                    </Form.Item>
                    <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
                      授权成功后，从回调 URL 读取 <code style={{ padding: '0 4px' }}>code</code> 与{' '}
                      <code style={{ padding: '0 4px' }}>shop_id</code>，填入下方：
                    </Typography.Text>
                    <Form.Item name="shopeeAuthCode" label="code">
                      <Input placeholder="Paste authorization code" />
                    </Form.Item>
                    <Form.Item name="shopeeCallbackShopId" label="shopId（回调 shop_id）" rules={[{ required: false }]}>
                      <Input placeholder="例如 92348765" />
                    </Form.Item>
                    <Form.Item name="shopeeMainAccountId" label="main_account_id（可选）">
                      <Input placeholder="如需可填" />
                    </Form.Item>
                    <Button
                      type="default"
                      onClick={async () => {
                        const vals = authForm.getFieldsValue();
                        const code = String(vals.shopeeAuthCode || '').trim();
                        const extShopId = String(vals.shopeeCallbackShopId || '').trim();
                        if (!detail?.id) return;
                        if (!code || !shopeeOAuthState) {
                          message.warning('需要 code 与已生成的 state');
                          return;
                        }
                        if (!extShopId) {
                          message.warning('需要 shopId（Shopee 回调参数 shop_id）');
                          return;
                        }
                        try {
                          await postShopeeOAuthCallback(detail.id, {
                            code,
                            state: shopeeOAuthState,
                            shopId: extShopId,
                            mainAccountId: String(vals.shopeeMainAccountId || '').trim() || undefined,
                          });
                          message.success('Shopee 授权已写入');
                          setAuthOpen(false);
                          actionRef.current?.reload();
                          if (detailOpen) void refreshDetail(detail.id);
                        } catch (e: unknown) {
                          message.error(formatPlatformPartnerErr(e));
                        }
                      }}
                    >
                      提交授权（code + state + shopId）
                    </Button>
                    <Divider />
                  </>
                )}
                {detail.platform === 'lazada' && provForShop?.status === 'beta' && (
                  <>
                    <Divider>Lazada OAuth</Divider>
                    <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
                      若填写「可选覆盖」中的字段，会先同步到服务端再生成链接。默认使用「平台开放配置」中的 Lazada 应用参数。
                    </Typography.Paragraph>
                    <Space wrap>
                      <Button
                        type="primary"
                        onClick={async () => {
                          if (authPartnerWarn) {
                            message.warning('请先完成「平台开放配置」中的必填项。');
                            return;
                          }
                          try {
                            const vals = authForm.getFieldsValue(true) as Record<string, unknown>;
                            const rd = String(vals.redirectUri || '').trim();
                            const pk = String(vals.appKey || '').trim();
                            const ps = String(vals.appSecret || '').trim();
                            if (pk || ps || rd) {
                              await updateShopAuth(detail.id, buildAuthPayload(vals));
                            }
                            const r = await getLazadaOAuthAuthorizeUrl(detail.id, rd || undefined);
                            setLazadaOAuthAuthorizeUrl(r.authorizeUrl);
                            setLazadaOAuthState(r.state);
                            message.success('已生成 Lazada 授权链接');
                          } catch (e: unknown) {
                            message.error(formatPlatformPartnerErr(e));
                          }
                        }}
                      >
                        生成授权链接
                      </Button>
                      <Button
                        onClick={() => {
                          if (!lazadaOAuthAuthorizeUrl) {
                            message.warning('请先生成授权链接');
                            return;
                          }
                          window.open(lazadaOAuthAuthorizeUrl, '_blank', 'noopener,noreferrer');
                        }}
                      >
                        新窗口打开
                      </Button>
                    </Space>
                    <Form.Item label="authorizeUrl">
                      <Space align="start">
                        <Input.TextArea
                          style={{ width: 460 }}
                          autoSize={{ minRows: 2, maxRows: 6 }}
                          readOnly
                          value={lazadaOAuthAuthorizeUrl}
                          placeholder='点击上方「生成授权链接」后出现'
                        />
                        <Button
                          disabled={!lazadaOAuthAuthorizeUrl}
                          onClick={() => {
                            void navigator.clipboard?.writeText?.(lazadaOAuthAuthorizeUrl);
                            message.success('已复制链接');
                          }}
                        >
                          复制
                        </Button>
                      </Space>
                    </Form.Item>
                    <Form.Item label="state（服务端签发）">
                      <Input readOnly value={lazadaOAuthState} placeholder='生成后出现' />
                    </Form.Item>
                    <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
                      从 Lazada 授权回调 URL 复制 <code style={{ padding: '0 4px' }}>code</code>，与上方 state 一并提交：
                    </Typography.Text>
                    <Form.Item name="lazadaAuthCode" label="code">
                      <Input placeholder="Paste authorization code" />
                    </Form.Item>
                    <Button
                      type="default"
                      onClick={async () => {
                        const vals = authForm.getFieldsValue();
                        const code = String(vals.lazadaAuthCode || '').trim();
                        if (!detail?.id) return;
                        if (!code || !lazadaOAuthState) {
                          message.warning('需要 code 与已生成的 state');
                          return;
                        }
                        try {
                          await postLazadaOAuthCallback(detail.id, {
                            code,
                            state: lazadaOAuthState,
                          });
                          message.success('Lazada 授权已写入');
                          setAuthOpen(false);
                          actionRef.current?.reload();
                          if (detailOpen) void refreshDetail(detail.id);
                        } catch (e: unknown) {
                          message.error(formatPlatformPartnerErr(e));
                        }
                      }}
                    >
                      提交授权（code + state）
                    </Button>
                    <Divider />
                  </>
                )}
                {detail.platform === 'amazon' && provForShop?.status === 'beta' && (
                  <>
                    <Divider>Amazon LWA OAuth</Divider>
                    <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
                      生成链接使用「平台开放配置」中的 Client ID、Redirect URI 与 Seller Central 基址；无需在下方手工填写 Secret。
                    </Typography.Paragraph>
                    <Space wrap>
                      <Button
                        type="primary"
                        onClick={async () => {
                          if (authPartnerWarn) {
                            message.warning('请先完成「平台开放配置」中的必填项。');
                            return;
                          }
                          try {
                            const r = await getAmazonOAuthAuthorizeUrl(detail.id);
                            setAmazonOAuthAuthorizeUrl(r.authorizeUrl);
                            setAmazonOAuthState(r.state);
                            message.success('已生成 Amazon 授权链接');
                          } catch (e: unknown) {
                            message.error(formatPlatformPartnerErr(e));
                          }
                        }}
                      >
                        生成授权链接
                      </Button>
                      <Button
                        onClick={() => {
                          if (!amazonOAuthAuthorizeUrl) {
                            message.warning('请先生成授权链接');
                            return;
                          }
                          window.open(amazonOAuthAuthorizeUrl, '_blank', 'noopener,noreferrer');
                        }}
                      >
                        新窗口打开
                      </Button>
                    </Space>
                    <Form.Item label="authorizeUrl">
                      <Space align="start">
                        <Input.TextArea
                          style={{ width: 460 }}
                          autoSize={{ minRows: 2, maxRows: 6 }}
                          readOnly
                          value={amazonOAuthAuthorizeUrl}
                          placeholder='点击上方「生成授权链接」后出现'
                        />
                        <Button
                          disabled={!amazonOAuthAuthorizeUrl}
                          onClick={() => {
                            void navigator.clipboard?.writeText?.(amazonOAuthAuthorizeUrl);
                            message.success('已复制链接');
                          }}
                        >
                          复制
                        </Button>
                      </Space>
                    </Form.Item>
                    <Form.Item label="state（服务端签发）">
                      <Input readOnly value={amazonOAuthState} placeholder='生成后出现' />
                    </Form.Item>
                    <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
                      提交授权 code（Seller Central 回调中的 spapi_oauth_code）、selling_partner_id，以及可选 marketplaceId：
                    </Typography.Text>
                    <Form.Item name="amazonAuthCode" label="code（spapi_oauth_code）">
                      <Input placeholder="Paste authorization code" />
                    </Form.Item>
                    <Form.Item
                      name="amazonSellingPartnerId"
                      label="sellingPartnerId"
                    >
                      <Input placeholder="来自回调 selling_partner_id" />
                    </Form.Item>
                    <Form.Item name="amazonMarketplaceId" label="marketplaceId（可选）">
                      <Input placeholder="留空则使用平台配置默认 Marketplace ID" />
                    </Form.Item>
                    <Button
                      type="default"
                      onClick={async () => {
                        const vals = authForm.getFieldsValue();
                        const code = String(vals.amazonAuthCode || '').trim();
                        const sp = String(vals.amazonSellingPartnerId || '').trim();
                        const mp = String(vals.amazonMarketplaceId || '').trim();
                        if (!detail?.id) return;
                        if (!code || !amazonOAuthState) {
                          message.warning('需要 code 与已生成的 state');
                          return;
                        }
                        if (!sp) {
                          message.warning('需要 sellingPartnerId');
                          return;
                        }
                        try {
                          await postAmazonOAuthCallback(detail.id, {
                            code,
                            state: amazonOAuthState,
                            sellingPartnerId: sp,
                            marketplaceId: mp || undefined,
                          });
                          message.success('Amazon 授权已写入');
                          setAuthOpen(false);
                          actionRef.current?.reload();
                          if (detailOpen) void refreshDetail(detail.id);
                        } catch (e: unknown) {
                          message.error(formatPlatformPartnerErr(e));
                        }
                      }}
                    >
                      提交授权
                    </Button>
                    <Divider />
                  </>
                )}
                {provForShop?.authSchema?.map((f) =>
                  STD_AUTH_KEYS.has(f.name) ? null : (
                    <Form.Item
                      key={f.name}
                      name={f.name}
                      label={f.label}
                      tooltip={f.hint}
                      rules={f.required ? [{ required: true, message: `请填写 ${f.label}` }] : undefined}
                    >
                      {f.sensitive || f.type === 'password' ? (
                        <Input.Password autoComplete="new-password" />
                      ) : (
                        <Input />
                      )}
                    </Form.Item>
                  ),
                )}
              </Form>
            )}
          </>
        )}
      </Drawer>
    </PageContainer>
  );
}
