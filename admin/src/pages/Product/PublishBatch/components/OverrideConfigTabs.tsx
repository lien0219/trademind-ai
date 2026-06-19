import type { PublishConfigLayer } from '@/constants/publishConfig';
import { countConfigFields } from '@/constants/publishConfig';
import PublishConfigEditor from '@/pages/Product/PublishBatch/components/PublishConfigEditor';
import type { PublishConfigOverrides } from '@/services/productPublish';
import { productTargetKey } from '@/utils/publishConfigMerge';
import type { ProductListRow } from '@/services/products';
import { Button, Modal, Select, Space, Table, Tabs, Typography, message } from 'antd';
import { useMemo, useState } from 'react';

type SelectedTarget = { platform: string; shopId?: string | null; shopName?: string; platformLabel?: string };

type Props = {
  products: ProductListRow[];
  targets: SelectedTarget[];
  overrides: PublishConfigOverrides;
  onChange: (next: PublishConfigOverrides) => void;
  commonConfig: PublishConfigLayer;
};

type EditState = {
  scope: 'product' | 'platform' | 'shop' | 'productTarget';
  key: string;
  label: string;
  productId?: string;
  platform?: string;
  shopId?: string;
  config: PublishConfigLayer;
};
export default function OverrideConfigTabs({ products, targets, overrides, onChange, commonConfig }: Props) {
  const [edit, setEdit] = useState<EditState | null>(null);
  const [copyFrom, setCopyFrom] = useState<PublishConfigLayer | null>(null);
  const [pickProductId, setPickProductId] = useState<string>();
  const [addProductOpen, setAddProductOpen] = useState(false);

  const productRows = useMemo(
    () =>
      Object.entries(overrides.products ?? {}).map(([id, cfg]) => {
        const p = products.find((x) => x.id === id);
        return {
          key: id,
          productId: id,
          title: p?.title || id,
          count: countConfigFields(cfg as Record<string, unknown>),
          config: cfg as PublishConfigLayer,
        };
      }),
    [overrides.products, products],
  );

  const platformSet = useMemo(() => {
    const s = new Set<string>();
    targets.forEach((t) => s.add(t.platform));
    return Array.from(s);
  }, [targets]);

  const platformRows = useMemo(
    () =>
      Object.entries(overrides.platforms ?? {}).map(([plat, cfg]) => ({
        key: plat,
        platform: plat,
        count: countConfigFields(cfg as Record<string, unknown>),
        config: cfg as PublishConfigLayer,
      })),
    [overrides.platforms],
  );

  const shopRows = useMemo(
    () =>
      Object.entries(overrides.shops ?? {}).map(([sid, cfg]) => {
        const t = targets.find((x) => x.shopId === sid);
        return {
          key: sid,
          shopId: sid,
          shopName: t?.shopName || sid,
          platform: t?.platform || '—',
          count: countConfigFields(cfg as Record<string, unknown>),
          config: cfg as PublishConfigLayer,
        };
      }),
    [overrides.shops, targets],
  );

  const targetRows = useMemo(
    () =>
      Object.entries(overrides.productTargets ?? {}).map(([key, cfg]) => {
        const [pid, plat, sid] = key.split(':');
        const p = products.find((x) => x.id === pid);
        const t = targets.find((x) => x.platform === plat && (!sid || x.shopId === sid));
        return {
          key,
          productId: pid,
          productTitle: p?.title || pid,
          platform: plat,
          platformLabel: t?.platformLabel || plat,
          shopId: sid,
          shopName: t?.shopName,
          count: countConfigFields(cfg as Record<string, unknown>),
          config: cfg as PublishConfigLayer,
        };
      }),
    [overrides.productTargets, products, targets],
  );

  const saveEdit = () => {
    if (!edit) return;
    const next = { ...overrides };
    const layer = edit.config;
    if (countConfigFields(layer as Record<string, unknown>) === 0) {
      message.warning('请至少配置一项覆盖字段');
      return;
    }
    if (edit.scope === 'product') {
      next.products = { ...(next.products ?? {}), [edit.key]: layer };
    } else if (edit.scope === 'platform') {
      next.platforms = { ...(next.platforms ?? {}), [edit.key]: layer };
    } else if (edit.scope === 'shop') {
      next.shops = { ...(next.shops ?? {}), [edit.key]: layer };
    } else {
      next.productTargets = { ...(next.productTargets ?? {}), [edit.key]: layer };
    }
    onChange(next);
    setEdit(null);
    setCopyFrom(null);
    message.success('覆盖配置已保存');
  };

  const removeOverride = (scope: EditState['scope'], key: string) => {
    const next = { ...overrides };
    if (scope === 'product' && next.products) {
      const { [key]: _, ...rest } = next.products;
      next.products = Object.keys(rest).length ? rest : undefined;
    } else if (scope === 'platform' && next.platforms) {
      const { [key]: _, ...rest } = next.platforms;
      next.platforms = Object.keys(rest).length ? rest : undefined;
    } else if (scope === 'shop' && next.shops) {
      const { [key]: _, ...rest } = next.shops;
      next.shops = Object.keys(rest).length ? rest : undefined;
    } else if (scope === 'productTarget' && next.productTargets) {
      const { [key]: _, ...rest } = next.productTargets;
      next.productTargets = Object.keys(rest).length ? rest : undefined;
    }
    onChange(next);
  };

  const actionCol = (scope: EditState['scope'], key: string, label: string, cfg: PublishConfigLayer, extra?: Partial<EditState>) => ({
    title: '操作',
    key: 'ops',
    width: 200,
    render: () => (
      <Space size="small" wrap>
        <Typography.Link
          onClick={() => setEdit({ scope, key, label, config: { ...cfg }, ...extra })}
        >
          编辑
        </Typography.Link>
        <Typography.Link
          onClick={() => {
            setCopyFrom({ ...cfg });
            message.info('已复制配置，请选择目标后添加');
          }}
        >
          复制
        </Typography.Link>
        <Typography.Link type="danger" onClick={() => removeOverride(scope, key)}>
          删除
        </Typography.Link>
      </Space>
    ),
  });

  const openAddProduct = () => {
    const unused = products.filter((p) => !overrides.products?.[p.id]);
    if (!unused.length) {
      message.warning('所有商品均已添加覆盖');
      return;
    }
    setPickProductId(unused[0].id);
    setAddProductOpen(true);
  };

  const openAddPlatform = () => {
    const unused = platformSet.filter((p) => !overrides.platforms?.[p]);
    if (!unused.length) {
      message.warning('所有平台均已添加覆盖');
      return;
    }
    setEdit({
      scope: 'platform',
      key: unused[0],
      label: unused[0],
      platform: unused[0],
      config: copyFrom ? { ...copyFrom } : {},
    });
    setCopyFrom(null);
  };

  const openAddShop = () => {
    const unused = targets.filter((t) => t.shopId && !overrides.shops?.[t.shopId]);
    if (!unused.length) {
      message.warning('所有店铺均已添加覆盖');
      return;
    }
    const t = unused[0];
    setEdit({
      scope: 'shop',
      key: t.shopId!,
      label: t.shopName || t.shopId!,
      shopId: t.shopId!,
      config: copyFrom ? { ...copyFrom } : {},
    });
    setCopyFrom(null);
  };

  const openAddProductTarget = () => {
    const combos: { pid: string; plat: string; sid?: string; label: string }[] = [];
    products.forEach((p) => {
      targets.forEach((t) => {
        const key = productTargetKey(p.id, t.platform, t.shopId || undefined);
        if (!overrides.productTargets?.[key]) {
          combos.push({
            pid: p.id,
            plat: t.platform,
            sid: t.shopId || undefined,
            label: `${p.title} / ${t.platformLabel || t.platform}${t.shopName ? ` / ${t.shopName}` : ''}`,
          });
        }
      });
    });
    if (!combos.length) {
      message.warning('所有商品×目标均已添加覆盖');
      return;
    }
    const c = combos[0];
    setEdit({
      scope: 'productTarget',
      key: productTargetKey(c.pid, c.plat, c.sid),
      label: c.label,
      productId: c.pid,
      platform: c.plat,
      shopId: c.sid,
      config: copyFrom ? { ...copyFrom } : {},
    });
    setCopyFrom(null);
  };

  const tabItems = [
    {
      key: 'product',
      label: `按商品覆盖 (${productRows.length})`,
      children: (
        <>
          <Button type="primary" size="small" onClick={openAddProduct} style={{ marginBottom: 12 }}>
            添加商品覆盖
          </Button>
          <Table
            rowKey="key"
            size="small"
            pagination={false}
            scroll={{ x: 640 }}
            dataSource={productRows}
            columns={[
              { title: '商品', dataIndex: 'title', ellipsis: true },
              { title: '覆盖项数量', dataIndex: 'count', width: 100 },
              {
                title: '操作',
                key: 'ops',
                width: 200,
                render: (_: unknown, row: (typeof productRows)[0]) =>
                  actionCol('product', row.key, row.title, row.config, { productId: row.productId }).render(),
              },
            ]}
          />
        </>
      ),
    },
    {
      key: 'platform',
      label: `按平台覆盖 (${platformRows.length})`,
      children: (
        <>
          <Button type="primary" size="small" onClick={openAddPlatform} style={{ marginBottom: 12 }}>
            添加平台覆盖
          </Button>
          <Table
            rowKey="key"
            size="small"
            pagination={false}
            scroll={{ x: 480 }}
            dataSource={platformRows}
            columns={[
              { title: '平台', dataIndex: 'platform' },
              { title: '覆盖项数量', dataIndex: 'count', width: 100 },
              {
                title: '操作',
                key: 'ops',
                width: 200,
                render: (_: unknown, row: (typeof platformRows)[0]) =>
                  actionCol('platform', row.key, row.platform, row.config, { platform: row.platform }).render(),
              },
            ]}
          />
        </>
      ),
    },
    {
      key: 'shop',
      label: `按店铺覆盖 (${shopRows.length})`,
      children: (
        <>
          <Button type="primary" size="small" onClick={openAddShop} style={{ marginBottom: 12 }}>
            添加店铺覆盖
          </Button>
          <Table
            rowKey="key"
            size="small"
            pagination={false}
            scroll={{ x: 560 }}
            dataSource={shopRows}
            columns={[
              { title: '平台', dataIndex: 'platform', width: 120 },
              { title: '店铺', dataIndex: 'shopName', ellipsis: true },
              { title: '覆盖项数量', dataIndex: 'count', width: 100 },
              {
                title: '操作',
                key: 'ops',
                width: 200,
                render: (_: unknown, row: (typeof shopRows)[0]) =>
                  actionCol('shop', row.key, row.shopName, row.config, { shopId: row.shopId }).render(),
              },
            ]}
          />
        </>
      ),
    },
    {
      key: 'productTarget',
      label: `按商品 + 目标覆盖 (${targetRows.length})`,
      children: (
        <>
          <Button type="primary" size="small" onClick={openAddProductTarget} style={{ marginBottom: 12 }}>
            添加商品目标覆盖
          </Button>
          <Table
            rowKey="key"
            size="small"
            pagination={false}
            scroll={{ x: 720 }}
            dataSource={targetRows}
            columns={[
              { title: '商品', dataIndex: 'productTitle', ellipsis: true },
              { title: '平台', dataIndex: 'platformLabel', width: 100 },
              { title: '店铺', dataIndex: 'shopName', width: 120, render: (v: string) => v || '—' },
              { title: '覆盖项数量', dataIndex: 'count', width: 100 },
              {
                title: '操作',
                key: 'ops',
                width: 200,
                render: (_: unknown, row: (typeof targetRows)[0]) =>
                  actionCol('productTarget', row.key, row.productTitle, row.config, {
                    productId: row.productId,
                    platform: row.platform,
                    shopId: row.shopId,
                  }).render(),
              },
            ]}
          />
        </>
      ),
    },
  ];

  return (
    <>
      <Tabs items={tabItems} />
      <Modal
        title="添加商品覆盖"
        open={addProductOpen}
        onCancel={() => setAddProductOpen(false)}
        onOk={() => {
          if (!pickProductId) return;
          const p = products.find((x) => x.id === pickProductId);
          setEdit({
            scope: 'product',
            key: pickProductId,
            label: p?.title || pickProductId,
            productId: pickProductId,
            config: copyFrom ? { ...copyFrom } : {},
          });
          setCopyFrom(null);
          setAddProductOpen(false);
        }}
      >
        <Select
          style={{ width: '100%' }}
          value={pickProductId}
          onChange={setPickProductId}
          options={products
            .filter((p) => !overrides.products?.[p.id])
            .map((p) => ({ label: p.title, value: p.id }))}
        />
      </Modal>
      <Modal
        title={edit ? `编辑覆盖：${edit.label}` : '编辑覆盖'}
        open={!!edit}
        onCancel={() => setEdit(null)}
        onOk={saveEdit}
        width={720}
        destroyOnClose
        okText="保存"
      >
        {edit ? (
          <PublishConfigEditor
            value={edit.config}
            onChange={(cfg) => setEdit({ ...edit, config: cfg })}
            showPreview
            previewContext={{
              common: commonConfig,
              overrides,
              productId: edit.productId,
              platform: edit.platform,
              shopId: edit.shopId,
            }}
          />
        ) : null}
      </Modal>
    </>
  );
}
