import { Link } from '@umijs/max';
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
  Descriptions,
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

const STD_AUTH_KEYS = new Set([
  'appKey',
  'appSecret',
  'accessToken',
  'refreshToken',
  'sellerId',
  'merchantId',
  'marketplaceId',
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
    const base: Record<string, unknown> = {
      authType: d.auth?.authType || p?.authType || 'token',
      appKey: d.auth?.appKey || '',
      appSecret: d.auth?.appSecret || '',
      accessToken: d.auth?.accessToken || '',
      refreshToken: d.auth?.refreshToken || '',
      sellerId: d.auth?.sellerId || '',
      merchantId: d.auth?.merchantId || '',
      marketplaceId: d.auth?.marketplaceId || '',
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
                message.success(res.message || '连接成功');
              } catch (e: unknown) {
                message.error(e instanceof Error ? e.message : '测试失败');
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
          await createShop({
            platform: vals.platform as string,
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
                    message.success(res.message || '连接成功');
                  } catch (e: unknown) {
                    message.error(e instanceof Error ? e.message : '测试失败');
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
        width={520}
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
            {detail.platform !== 'manual' && (
              <Form form={authForm} layout="vertical">
                <Form.Item name="authType" label="授权类型" hidden>
                  <Input />
                </Form.Item>
                <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
                  密钥字段已脱敏展示为 ****；不修改请留空或保持原样，保存时不覆盖。
                </Typography.Text>
                <Form.Item name="appKey" label="App Key">
                  <Input />
                </Form.Item>
                <Form.Item name="appSecret" label="App Secret">
                  <Input.Password autoComplete="new-password" placeholder="敏感，留空不更新" />
                </Form.Item>
                <Form.Item name="accessToken" label="Access Token">
                  <Input.Password autoComplete="new-password" />
                </Form.Item>
                <Form.Item name="refreshToken" label="Refresh Token">
                  <Input.Password autoComplete="new-password" />
                </Form.Item>
                <Form.Item name="sellerId" label="Seller Id">
                  <Input />
                </Form.Item>
                <Form.Item name="merchantId" label="Merchant Id">
                  <Input />
                </Form.Item>
                <Form.Item name="marketplaceId" label="Marketplace Id">
                  <Input />
                </Form.Item>
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
