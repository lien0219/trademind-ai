import { Link } from '@umijs/renderer-react';
import {
  ApiOutlined,
  DeleteOutlined,
  EditOutlined,
  EyeOutlined,
  MoreOutlined,
  SafetyCertificateOutlined,
  ShopOutlined,
} from '@ant-design/icons';
import { formatDateTime } from '@/utils/formatTime';
import { ModalForm, ProFormDigit, ProFormRadio, ProFormSelect, ProFormText, ProFormTextArea, ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails } from '@/components/ui';
import {
  Alert,
  Button,
  Descriptions,
  Divider,
  Drawer,
  Dropdown,
  Form,
  Input,
  Modal,
  Popconfirm,
  Space,
  Tag,
  Typography,
  message,
  type MenuProps,
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
  getDouyinOAuthAuthorizeUrl,
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
  refreshDouyinOAuth,
  revokeDouyinOAuth,
  syncDouyinShopInfo,
  testDouyinOAuth,
  testShopConnection,
  updateShop,
  updateShopAuth,
  type PlatformProviderMeta,
  type ShopDetail,
  type ShopListRow,
} from '@/services/shops';
import { syncShopOrders } from '@/services/orderSync';
import { syncCustomerMessages } from '@/services/customer';
import { getPlatformAppSettings } from '@/services/platformOpen';
import { isDeployAppConfigComplete } from '@/utils/platformAppConfig';
import { shopCapabilityLabel } from '@/constants/shopCapabilities';

const PLATFORM_TAG_COLORS: Record<string, string> = {
  manual: 'default',
  tiktok: 'magenta',
  douyin_shop: 'volcano',
  shopee: 'orange',
  lazada: 'blue',
  amazon: 'gold',
};

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

function cellText(val?: string | null) {
  const t = (val ?? '').trim();
  return t ? t : <Typography.Text type="secondary">—</Typography.Text>;
}

function renderPlatformCell(platform: string, providers: PlatformProviderMeta[]) {
  const meta = providers.find((x) => x.platform === platform);
  const label = meta?.name ?? platform;
  const color = PLATFORM_TAG_COLORS[platform] ?? 'processing';
  return (
    <Space size={4} wrap>
      <Tag color={color as never} style={{ margin: 0 }}>
        {label}
      </Tag>
      {meta?.status === 'beta' ? (
        <Tag color="processing" style={{ margin: 0 }}>
          Beta
        </Tag>
      ) : null}
      {meta?.status === 'planned' ? (
        <Tag style={{ margin: 0 }}>规划中</Tag>
      ) : null}
    </Space>
  );
}

function renderCapabilityTags(raw: unknown) {
  const list = Array.isArray(raw) ? raw.map(String).filter(Boolean) : [];
  if (!list.length) return <Typography.Text type="secondary">—</Typography.Text>;
  const visible = list.slice(0, 2);
  const rest = list.length - visible.length;
  return (
    <Space size={[4, 4]} wrap>
      {visible.map((c) => (
        <Tag key={c} style={{ margin: 0 }}>
          {shopCapabilityLabel(c)}
        </Tag>
      ))}
      {rest > 0 ? <Tag style={{ margin: 0 }}>+{rest}</Tag> : null}
    </Space>
  );
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
    res.externalShopId ? `平台店铺编号 ${res.externalShopId}` : '',
  ].filter(Boolean);
  return parts.join(' · ') || '连接成功';
}

/** Map incomplete platform Partner Open settings errors → CN hints (no secrets). */
function formatPlatformPartnerErr(err: unknown): string {
  const msg = err instanceof Error ? err.message : String(err);
  const low = msg.toLowerCase();

  if (msg.includes('required setting missing:')) {
    return `${msg}\n请先到「设置 → 平台接入设置」补齐该平台应用信息的必填项。`;
  }

  if (msg.includes('platform config incomplete: please configure platform_tiktok')) {
    return `${msg}\n请先到「设置 → 平台接入设置 → TikTok Shop」填写 App Key、App Secret 和 Redirect URI。`;
  }
  if (msg.includes('platform config incomplete: please configure platform_shopee')) {
    return `${msg}\n请先到「设置 → 平台接入设置 → Shopee」填写 Partner ID、Partner Key 和 Redirect URI。`;
  }
  if (msg.includes('platform config incomplete: please configure platform_lazada')) {
    return `${msg}\n请先到「设置 → 平台接入设置 → Lazada」填写 App Key、App Secret 和 Redirect URI。`;
  }
  if (msg.includes('platform customer message permission denied') || msg.includes('platform customer message permission')) {
    return `${msg}\n平台客服权限不足，请确认已在 TikTok / Shopee / Lazada 等平台开放后台申请客服消息权限并重新授权；Amazon 请在 Seller Central / SP-API Developer Console 申请 Buyer-Seller Messaging（Messaging API）相关权限并重新授权店铺。`;
  }
  if (msg.includes('platform customer message provider not implemented')) {
    return `${msg}\n当前平台客服消息 API 尚未接入，可使用 mock 店铺验证拉取与发送联调。`;
  }
  if (msg.includes('manual shop does not support platform customer messages')) {
    return `${msg}\n手工店铺仅支持会话手工录入，不支持平台客服消息同步。`;
  }
  if (msg.includes('platform config incomplete: please configure platform_amazon')) {
    return `${msg}\n请先到「设置 → 平台接入设置 → Amazon SP-API」填写 Client ID、Client Secret、Redirect URI、Marketplace ID 和 SP-API Base URL。`;
  }
  if (msg.includes('platform_amazon.lwa_auth_base_url and lwa_token_url')) {
    return `${msg}\n请在「Amazon SP-API」配置中补齐 LWA Auth Base URL 与 LWA Token URL。`;
  }
  if (
    msg.includes(
      'TikTok platform config is incomplete. Please configure App Key, App Secret and Redirect URI first.',
    )
  ) {
    return `${msg}\n请先前往「设置 → 平台接入设置」填写 TikTok Shop（分组 platform_tiktok）必填项后再试。`;
  }
  if (low.includes('tiktok platform config is incomplete') || low.includes('platform_tiktok')) {
    return `${msg}\n请到「设置 → 平台接入设置」完成 TikTok Shop 必填项后再试。`;
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
  const [cmSyncOpen, setCmSyncOpen] = useState(false);
  const [cmSyncTarget, setCmSyncTarget] = useState<{ id: string; platform: string } | null>(null);
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
            `请先到「设置 → 平台接入设置」填写「${p.name}」平台应用信息（分组 ${p.settingsGroupKey}），再完成店铺授权。`,
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

  const redirectDouyinOAuth = async (shopId: string) => {
    const res = await getDouyinOAuthAuthorizeUrl(shopId);
    const target = res.redirectUrl || res.authorizeUrl;
    if (!target) {
      message.error('缺少抖店授权链接');
      return;
    }
    window.location.href = target;
  };

  const refreshDouyinFor = async (shopId: string) => {
    await refreshDouyinOAuth(shopId);
    message.success('抖店授权已刷新');
    actionRef.current?.reload();
    if (detail?.id === shopId) await refreshDetail(shopId);
  };

  const revokeDouyinFor = async (shopId: string) => {
    await revokeDouyinOAuth(shopId);
    message.success('抖店店铺已解除授权，历史数据不会删除');
    actionRef.current?.reload();
    if (detail?.id === shopId) await refreshDetail(shopId);
  };

  const syncDouyinInfoFor = async (shopId: string) => {
    await syncDouyinShopInfo(shopId);
    message.success('抖店店铺信息已同步');
    actionRef.current?.reload();
    if (detail?.id === shopId) await refreshDetail(shopId);
  };

  const openCustomerMessageSyncModal = (platform: string, shopId: string) => {
    const p = providers.find((x) => x.platform === platform);
    if (platform === 'manual') {
      message.warning('手工店铺不支持平台客服消息同步');
      return;
    }
    const cm = p?.capabilityStatus?.customer_message;
    if (cm === 'planned' || cm === 'disabled') {
      message.warning('当前平台客服消息接口尚未接入；请使用 mock 店铺验证联调，或等待后续版本。');
      return;
    }
    if (cm !== 'available' && cm !== 'beta') {
      message.warning('当前平台不支持客服消息同步');
      return;
    }
    setCmSyncTarget({ id: shopId, platform });
    setCmSyncOpen(true);
  };

  const openOrderSyncModal = (platform: string, shopId: string) => {
    const p = providers.find((x) => x.platform === platform);
    if (platform === 'manual') {
      message.warning('手工店铺不支持订单同步');
      return;
    }
    const os = p?.capabilityStatus?.order_sync;
    if (os === 'planned' || os === 'disabled') {
      message.warning('当前平台订单同步尚未接入');
      return;
    }
    if (p?.status === 'planned') {
      message.warning('平台订单同步暂未实现');
      return;
    }
    if (platform === 'douyin_shop' && p?.status === 'beta') {
      message.info('请先在「设置 → 平台接入设置 → 抖店」开启订单同步，并完成店铺授权。');
    }
    setSyncTarget({ id: shopId, platform });
    setSyncOpen(true);
  };

  const columns: ProColumns<ShopListRow>[] = useMemo(
    () => [
      {
        title: '平台',
        dataIndex: 'platform',
        width: 148,
        valueType: 'select',
        valueEnum: Object.fromEntries(providers.map((p) => [p.platform, { text: p.name }])),
        fieldProps: { showSearch: true, optionFilterProp: 'label' },
        render: (_, r) => renderPlatformCell(r.platform, providers),
      },
      {
        title: '店铺名',
        dataIndex: 'shopName',
        width: 160,
        ellipsis: true,
        render: (_, r) => (
          <Button
            type="link"
            size="small"
            style={{ padding: 0, height: 'auto', maxWidth: '100%', textAlign: 'left' }}
            onClick={() => void openDetail(r)}
          >
            <Space size={6} style={{ maxWidth: '100%' }}>
              <ShopOutlined style={{ color: 'var(--ant-color-primary)', flexShrink: 0 }} />
              <Typography.Text ellipsis style={{ maxWidth: 140 }}>
                {r.shopName}
              </Typography.Text>
            </Space>
          </Button>
        ),
      },
      {
        title: '编码',
        dataIndex: 'shopCode',
        width: 108,
        search: false,
        ellipsis: true,
        render: (_, r) => cellText(r.shopCode),
      },
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
        width: 104,
        valueType: 'select',
        valueEnum: Object.fromEntries(Object.entries(SHOP_AUTH_STATUS).map(([k, v]) => [k, { text: v.text }])),
        render: (_, r) => tagFromMap(r.authStatus, SHOP_AUTH_STATUS),
      },
      {
        title: '地区',
        dataIndex: 'region',
        width: 80,
        search: false,
        render: (_, r) => cellText(r.region),
      },
      {
        title: '币种',
        dataIndex: 'currency',
        width: 72,
        search: false,
        render: (_, r) => cellText(r.currency),
      },
      {
        title: '能力',
        dataIndex: 'capabilities',
        width: 168,
        search: false,
        render: (_, r) => renderCapabilityTags(r.capabilities),
      },
      {
        title: '更新时间',
        dataIndex: 'updatedAt',
        width: 168,
        search: false,
        render: (_, r) => (
          <Typography.Text type="secondary" style={{ fontSize: 13 }}>
            {formatDateTime(r.updatedAt)}
          </Typography.Text>
        ),
      },
      {
        title: '操作',
        valueType: 'option',
        width: 280,
        onCell: () => ({ style: { whiteSpace: 'nowrap' } }),
        render: (_, r) => {
          const moreItems: MenuProps['items'] = [];

          if (r.platform === 'douyin_shop') {
            moreItems.push({
              type: 'group',
              label: '抖店授权',
              children: [
                {
                  key: 'dy-oauth',
                  label: '重新授权',
                  onClick: () => void redirectDouyinOAuth(r.id),
                },
                {
                  key: 'dy-refresh',
                  label: '刷新授权',
                  onClick: () => void refreshDouyinFor(r.id),
                },
                {
                  key: 'dy-sync-shop',
                  label: '同步店铺信息',
                  onClick: () => void syncDouyinInfoFor(r.id),
                },
                {
                  key: 'dy-revoke',
                  label: '解除授权',
                  danger: true,
                  onClick: () => {
                    Modal.confirm({
                      title: '解除抖店授权？',
                      content: '解除授权后无法继续同步该店铺，历史数据不会删除。',
                      okType: 'danger',
                      onOk: async () => {
                        await revokeDouyinFor(r.id);
                      },
                    });
                  },
                },
              ],
            });
          }

          moreItems.push({
            type: 'group',
            label: '数据同步',
            children: [
              {
                key: 'osync',
                label: '同步订单',
                onClick: () => openOrderSyncModal(r.platform, r.id),
              },
              {
                key: 'olog',
                label: (
                  <Link to={`/orders/sync-tasks?shopId=${encodeURIComponent(r.id)}`}>订单同步记录</Link>
                ),
              },
              {
                key: 'cmsync',
                label: '拉取客服消息',
                onClick: () => openCustomerMessageSyncModal(r.platform, r.id),
              },
              {
                key: 'cmlog',
                label: (
                  <Link to={`/customer/message-sync-tasks?shopId=${encodeURIComponent(r.id)}`}>
                    客服同步记录
                  </Link>
                ),
              },
            ],
          });

          moreItems.push({
            key: 'test',
            label: '测试连接',
            icon: <ApiOutlined />,
            onClick: async () => {
              try {
                const res =
                  r.platform === 'douyin_shop' ? await testDouyinOAuth(r.id) : await testShopConnection(r.id);
                message.success(summarizeShopTest(res));
              } catch (e: unknown) {
                message.error(formatPlatformPartnerErr(e));
              }
            },
          });
          moreItems.push({ type: 'divider' });
          moreItems.push({
            key: 'delete',
            label: '删除店铺',
            icon: <DeleteOutlined />,
            danger: true,
            onClick: () => {
              Modal.confirm({
                title: '删除店铺？',
                content: '删除后不可恢复，请确认是否继续。',
                okType: 'danger',
                onOk: async () => {
                  await deleteShop(r.id);
                  message.success('已删除');
                  actionRef.current?.reload();
                },
              });
            },
          });

          return (
            <Space size={0} wrap={false}>
              <Button
                type="link"
                size="small"
                style={{ paddingInline: 4 }}
                icon={<EyeOutlined />}
                onClick={() => void openDetail(r)}
              >
                查看
              </Button>
              <Button
                type="link"
                size="small"
                style={{ paddingInline: 4 }}
                icon={<EditOutlined />}
                onClick={async () => {
                  const d = await getShop(r.id);
                  setDetail(d);
                  setEditOpen(true);
                }}
              >
                编辑
              </Button>
              <Button
                type="link"
                size="small"
                style={{ paddingInline: 4 }}
                icon={<SafetyCertificateOutlined />}
                onClick={() => void openAuthFor(r.id)}
              >
                授权
              </Button>
              <Dropdown menu={{ items: moreItems }} trigger={['click']}>
                <Button type="link" size="small" style={{ paddingInline: 4 }} icon={<MoreOutlined />}>
                  更多
                </Button>
              </Dropdown>
            </Space>
          );
        },
      },
    ],
    [providers],
  );

  return (
    <TmPageContainer
      title="店铺管理"
      subTitle="授权并管理已连接的电商平台店铺，可在此同步订单与更新店铺信息。"
    >
      <ProTable<ShopListRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        tableStyle={{ tableLayout: 'fixed' }}
        scroll={{ x: 1376 }}
        search={{
          labelWidth: 'auto',
          defaultCollapsed: false,
          span: { xs: 24, sm: 12, md: 8, lg: 6, xl: 6, xxl: 6 },
        }}
        options={{ reload: true, density: true, setting: true }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true, showQuickJumper: true }}
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
        modalProps={{ destroyOnHidden: true, onCancel: () => setCreateOpen(false) }}
        onFinish={async (vals) => {
          const plat = vals.platform as string;
          const meta = providers.find((x) => x.platform === plat);
          if (meta?.settingsGroupKey && plat !== 'manual') {
            try {
              const row = await getPlatformAppSettings(meta.platform);
              if (!isDeployAppConfigComplete(row.schema ?? meta.appConfigSchema, row.values)) {
                message.error(
                  `请先到「设置 → 平台接入设置」填写「${meta.name}」平台应用信息后再创建店铺。`,
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
        <ProFormText name="defaultLanguage" label="默认语言" placeholder="例如 en" />
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
        modalProps={{ destroyOnHidden: true, onCancel: () => setEditOpen(false) }}
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
          destroyOnHidden: true,
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
        <ProFormText name="start" label="开始时间（可选）" placeholder="2026-05-01T00:00:00Z" extra="ISO 8601 格式" />
        <ProFormText name="end" label="结束时间（可选）" placeholder="2026-05-16T23:59:59Z" extra="ISO 8601 格式" />
        <ProFormText name="cursor" label="游标（可选）" />
        <ProFormDigit name="limit" label="每页条数" min={1} max={200} fieldProps={{ precision: 0 }} />
      </ModalForm>

      <ModalForm
        title="拉取平台客服消息"
        open={cmSyncOpen}
        modalProps={{
          destroyOnHidden: true,
          onCancel: () => {
            setCmSyncOpen(false);
            setCmSyncTarget(null);
          },
        }}
        initialValues={{ mode: 'incremental', limit: 50, cursor: '', start: '', end: '' }}
        onFinish={async (vals) => {
          if (!cmSyncTarget) return false;
          try {
            await syncCustomerMessages(cmSyncTarget.id, {
              mode: vals.mode as string,
              start: (vals.start as string | undefined) || undefined,
              end: (vals.end as string | undefined) || undefined,
              cursor: (vals.cursor as string | undefined) || undefined,
              limit: vals.limit as number | undefined,
            });
          } catch (e: unknown) {
            message.error(formatPlatformPartnerErr(e));
            return false;
          }
          message.success('客服消息同步任务已提交');
          setCmSyncOpen(false);
          setCmSyncTarget(null);
          return true;
        }}
      >
        <ProFormRadio.Group
          name="mode"
          label="同步模式"
          options={[
            { label: '增量 incremental', value: 'incremental' },
            { label: '全量 full', value: 'full' },
            { label: '手动 manual', value: 'manual' },
          ]}
          rules={[{ required: true }]}
        />
        <ProFormText name="start" label="开始时间（可选）" placeholder="2026-05-01T00:00:00Z" extra="ISO 8601 格式" />
        <ProFormText name="end" label="结束时间（可选）" placeholder="2026-05-16T23:59:59Z" extra="ISO 8601 格式" />
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
        destroyOnHidden
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
              {detail.platform === 'douyin_shop' ? (
                <Button onClick={() => void redirectDouyinOAuth(detail.id)}>重新授权</Button>
              ) : null}
              {detail.platform === 'douyin_shop' ? (
                <Button onClick={() => void refreshDouyinFor(detail.id)}>刷新授权</Button>
              ) : null}
              {detail.platform === 'douyin_shop' ? (
                <Button onClick={() => void syncDouyinInfoFor(detail.id)}>同步店铺信息</Button>
              ) : null}
              {detail.platform === 'douyin_shop' ? (
                <Popconfirm
                  title="解除抖店授权？"
                  description="解除授权后无法继续同步该店铺，历史数据不会删除。"
                  onConfirm={() => void revokeDouyinFor(detail.id)}
                >
                  <Button danger>解除授权</Button>
                </Popconfirm>
              ) : null}
              <Button
                onClick={() => {
                  if (!detail) return;
                  openCustomerMessageSyncModal(detail.platform, detail.id);
                }}
              >
                拉取客服消息
              </Button>
              <Link to={`/customer/message-sync-tasks?shopId=${encodeURIComponent(detail?.id ?? '')}`}>
                <Button disabled={!detail}>客服同步记录</Button>
              </Link>
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
                    const res = detail.platform === 'douyin_shop' ? await testDouyinOAuth(detail.id) : await testShopConnection(detail.id);
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
                description="可先创建店铺占位；店铺授权与订单同步能力将陆续开放。"
              />
            )}
            {detail.platform === 'tiktok' && provForShop.status === 'beta' && (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 12 }}
                message="TikTok Shop（Beta）"
                description="支持店铺授权、连接测试与订单同步。请先在「平台接入设置」填写平台应用信息，再在此生成授权链接并完成授权。"
              />
            )}
            {detail.platform === 'douyin_shop' && provForShop.status === 'beta' && (
              <Alert
                type={detail.authStatus === 'authorized' ? 'success' : detail.authStatus === 'expired' ? 'warning' : 'info'}
                showIcon
                style={{ marginBottom: 12 }}
                message={
                  detail.authStatus === 'authorized'
                    ? '店铺连接正常'
                    : detail.authStatus === 'expired'
                      ? '授权已过期，请重新授权'
                      : detail.authStatus === 'invalid' || detail.authStatus === 'need_check'
                        ? '店铺连接异常，请检查应用权限或重新授权'
                        : '请先连接抖店店铺'
                }
                description="支持抖店店铺授权、连接测试、手动订单同步与店铺信息校准；订单同步需在平台接入设置中开启订单同步。不会在前端返回授权凭证明文。"
              />
            )}
            {detail.platform === 'shopee' && provForShop.status === 'beta' && (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 12 }}
                message="Shopee（Beta）"
                description="支持店铺授权、连接测试与订单同步。请先在「平台接入设置 → Shopee」填写平台应用信息，再完成授权。"
              />
            )}
            {detail.platform === 'lazada' && provForShop.status === 'beta' && (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 12 }}
                message="Lazada（测试中）"
                description="支持店铺授权、连接测试与订单同步。请先在「平台接入设置 → Lazada」填写平台应用信息，再完成授权。"
              />
            )}
            {detail.platform === 'amazon' && provForShop.status === 'beta' && (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 12 }}
                message="Amazon SP-API（测试中）"
                description="支持店铺授权、连接测试与订单同步。请先在「平台接入设置 → Amazon」填写完整平台应用信息；服务器需按文档配置亚马逊访问凭证。"
              />
            )}
            <Descriptions bordered size="small" column={2}>
              <Descriptions.Item label="平台">{providerLabel(providers, detail.platform)}</Descriptions.Item>
              <Descriptions.Item label="平台标识">{detail.platform}</Descriptions.Item>
              <Descriptions.Item label="状态">{tagFromMap(detail.status, SHOP_STATUS)}</Descriptions.Item>
              <Descriptions.Item label="授权">{tagFromMap(detail.authStatus, SHOP_AUTH_STATUS)}</Descriptions.Item>
              <Descriptions.Item label="店铺编码">{detail.shopCode || '—'}</Descriptions.Item>
              <Descriptions.Item label="地区">{detail.region || '—'}</Descriptions.Item>
              <Descriptions.Item label="币种">{detail.currency || '—'}</Descriptions.Item>
              <Descriptions.Item label="时区">{detail.timezone || '—'}</Descriptions.Item>
              <Descriptions.Item label="语言">{detail.defaultLanguage || '—'}</Descriptions.Item>
              <Descriptions.Item label="平台店铺编号">{detail.externalShopId || '—'}</Descriptions.Item>
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
        destroyOnHidden
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
                message="平台应用信息可能不完整"
                description={authPartnerWarn}
              />
            )}
            {detail.platform === 'manual' && (
              <Alert type="success" showIcon message="手工店铺无需授权" description="无需配置授权凭证或密钥。" />
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
                message="TikTok 授权提示"
                description={
                  <>
                    请先在 <Link to="/settings/platforms">平台接入设置</Link> 填写 TikTok 平台应用信息，再生成授权链接。
                    本地开发需启动任务队列服务。仅在展开「可选覆盖」时才使用本页单独填写的密钥。
                  </>
                }
              />
            )}
            {detail.platform === 'shopee' && provForShop?.status === 'beta' && (
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message="Shopee 授权提示"
                description={
                  <>
                    请先在 <Link to="/settings/platforms">平台接入设置</Link> 填写 Shopee 平台应用信息。
                    授权回调中如有店铺编号，请一并填写到提交表单。
                  </>
                }
              />
            )}
            {detail.platform === 'lazada' && provForShop?.status === 'beta' && (
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message="Lazada 授权提示"
                description={
                  <>
                    请先在 <Link to="/settings/platforms">平台接入设置</Link> 填写 Lazada 平台应用信息。
                    授权完成后从回调页面复制授权码并提交即可。
                  </>
                }
              />
            )}
            {detail.platform === 'amazon' && provForShop?.status === 'beta' && (
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message="Amazon 授权提示"
                description={
                  <>
                    请先在 <Link to="/settings/platforms">平台接入设置</Link> 填写 Amazon 平台应用信息。
                    授权完成后按页面提示填写卖家编号与授权码。管理端不会直接访问亚马逊接口。
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
                  <TechnicalDetails label="可选：覆盖应用密钥与回调地址" className="tm-shop-auth-override">
                    <Form.Item
                      name="appKey"
                      label="覆盖 App Key"
                      tooltip="多数情况留空即可，使用「平台接入设置」中的默认值"
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
                  </TechnicalDetails>
                ) : detail.platform === 'shopee' ? (
                  <TechnicalDetails label="可选：覆盖 Partner 凭证与回调地址" className="tm-shop-auth-override">
                    <Form.Item
                      name="appKey"
                      label="覆盖 Partner ID"
                      tooltip="留空则使用「平台接入设置」中的 partner_id"
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
                  </TechnicalDetails>
                ) : detail.platform === 'lazada' ? (
                  <TechnicalDetails label="可选：覆盖应用密钥与回调地址" className="tm-shop-auth-override">
                    <Form.Item
                      name="appKey"
                      label="覆盖 App Key"
                      tooltip="留空则使用「平台接入设置」中的 app_key"
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
                  </TechnicalDetails>
                ) : detail.platform === 'amazon' ? (
                  <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
                    LWA Client ID / Secret、Redirect URI、SP-API 与 LWA 端点均在「
                    <Link to="/settings/platforms">平台接入设置</Link>」维护；本页保存店铺授权凭证与 Selling Partner 标识。
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
                <TechnicalDetails label="手工填入 / 高级字段" className="tm-shop-auth-override">
                  <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 12 }}>
                    一般通过上方「生成授权链接」完成授权即可。仅在调试或特殊场景下手工填写 Token、卖家编号等字段。
                  </Typography.Paragraph>
                  <Form.Item name="accessToken" label="授权凭证">
                    <Input.Password autoComplete="new-password" />
                  </Form.Item>
                  <Form.Item name="refreshToken" label="刷新授权凭证">
                    <Input.Password autoComplete="new-password" />
                  </Form.Item>
                  <Form.Item
                    name="expiresAt"
                    label="授权凭证过期时间"
                    tooltip="ISO 8601 格式，例如 2026-06-01T00:00:00Z"
                  >
                    <Input placeholder="2026-06-01T00:00:00Z" />
                  </Form.Item>
                  <Form.Item name="refreshExpiresAt" label="刷新凭证过期时间" tooltip="ISO 8601 格式">
                    <Input placeholder="2026-06-01T00:00:00Z" />
                  </Form.Item>
                  <Form.Item
                    name="sellerId"
                    label={
                      detail.platform === 'tiktok'
                        ? 'Seller Id（TikTok 通常不用填）'
                        : detail.platform === 'shopee'
                          ? 'Shopee 店铺编号'
                          : detail.platform === 'lazada'
                            ? '卖家标识'
                            : detail.platform === 'amazon'
                              ? 'Selling Partner Id'
                              : 'Seller Id'
                    }
                    tooltip={
                      detail.platform === 'shopee'
                        ? '与 Shopee 回调 URL 参数 shop_id 一致，用于 OpenAPI 签名。'
                        : detail.platform === 'lazada'
                          ? 'Lazada 店铺标识；店铺授权成功后通常会自动写入。'
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
                        ? 'Shop cipher'
                        : detail.platform === 'shopee'
                          ? '主账号 ID（可选）'
                          : detail.platform === 'lazada'
                            ? '扩展信息（可选）'
                            : detail.platform === 'amazon'
                              ? '扩展（可选）'
                              : 'Merchant Id'
                    }
                    tooltip={
                      detail.platform === 'tiktok'
                        ? '店铺授权成功后通常会自动写入；仅在手工调试时粘贴。'
                        : detail.platform === 'shopee'
                          ? '跨境/主帐号场景可选；店铺授权回调可一并提交。'
                          : detail.platform === 'lazada'
                            ? '店铺授权回调 country / account 摘要可写入 auth_config。'
                            : undefined
                    }
                  >
                    <Input />
                  </Form.Item>
                  <Form.Item name="marketplaceId" label="站点 Marketplace ID（可选）">
                    <Input placeholder={detail.platform === 'amazon' ? '可覆盖平台接入设置中的默认站点 ID' : undefined} />
                  </Form.Item>
                </TechnicalDetails>
                {detail.platform === 'tiktok' && (
                  <>
                    <Divider>TikTok 店铺授权</Divider>
                    <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
                      若填写「可选覆盖」中的字段，会先同步到服务端再生成链接（留空则不发送覆盖项）。默认直接使用「平台接入设置」。
                    </Typography.Paragraph>
                    <Space wrap>
                      <Button
                        type="primary"
                        onClick={async () => {
                          if (authPartnerWarn) {
                            message.warning('请先完成「平台接入设置」中的必填项。');
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
                    <Divider>Shopee 店铺授权</Divider>
                    <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
                      若填写「可选覆盖」中的字段，会先同步到服务端再生成链接。默认使用「平台接入设置」中的平台应用信息。
                    </Typography.Paragraph>
                    <Space wrap>
                      <Button
                        type="primary"
                        onClick={async () => {
                          if (authPartnerWarn) {
                            message.warning('请先完成「平台接入设置」中的必填项。');
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
                    <Form.Item name="shopeeAuthCode" label="授权码">
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
                    <Divider>Lazada 店铺授权</Divider>
                    <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
                      若填写「可选覆盖」中的字段，会先同步到服务端再生成链接。默认使用「平台接入设置」中的 Lazada 平台应用信息。
                    </Typography.Paragraph>
                    <Space wrap>
                      <Button
                        type="primary"
                        onClick={async () => {
                          if (authPartnerWarn) {
                            message.warning('请先完成「平台接入设置」中的必填项。');
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
                    <Form.Item name="lazadaAuthCode" label="授权码">
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
                    <Divider>Amazon 店铺授权</Divider>
                    <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
                      生成链接使用「平台接入设置」中的 Client ID、Redirect URI 与 Seller Central 基址；无需在下方手工填写 Secret。
                    </Typography.Paragraph>
                    <Space wrap>
                      <Button
                        type="primary"
                        onClick={async () => {
                          if (authPartnerWarn) {
                            message.warning('请先完成「平台接入设置」中的必填项。');
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
    </TmPageContainer>
  );
}
