import { PlusOutlined, ReloadOutlined } from "@ant-design/icons";
import type { ProColumns } from "@ant-design/pro-components";
import {
  Alert,
  Button,
  Form,
  Input,
  Modal,
  Radio,
  Select,
  Space,
  Tag,
  message,
} from "antd";
import { useCallback, useEffect, useMemo, useState } from "react";
import PermissionGuard from "@/components/PermissionGuard";
import { ErrorAlert, TmPageContainer, TmProTable } from "@/components/ui";
import { usePermission } from "@/hooks/usePermission";
import {
  createWarehouseSKUPlacement,
  createWarehouseLocation,
  listInventoryWarehouses,
  listWarehouseLocations,
  listWarehouseSKUPlacements,
  queryInventoryCenter,
  updateWarehouseSKUPlacement,
  type InventoryCenterRow,
  type InventoryWarehouse,
  type WarehouseLocation,
  type WarehouseSKUPlacement,
} from "@/services/inventory";
import { PERMISSIONS } from "@/utils/permission";

type PlacementFormValues = {
  productSkuId: string;
  locationId?: string;
  barcode?: string;
  status: string;
};

export default function WarehousePlacementsPage() {
  const { can, readonly } = usePermission();
  const canManage = !readonly && can(PERMISSIONS.INVENTORY_OPERATE);
  const [form] = Form.useForm<PlacementFormValues>();
  const [warehouses, setWarehouses] = useState<InventoryWarehouse[]>([]);
  const [warehouseId, setWarehouseId] = useState("");
  const [locations, setLocations] = useState<WarehouseLocation[]>([]);
  const [placements, setPlacements] = useState<WarehouseSKUPlacement[]>([]);
  const [skus, setSkus] = useState<InventoryCenterRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [modalOpen, setModalOpen] = useState(false);
  const [locationModalOpen, setLocationModalOpen] = useState(false);
  const [editing, setEditing] = useState<WarehouseSKUPlacement>();
  const [submitting, setSubmitting] = useState(false);
  const [locationSubmitting, setLocationSubmitting] = useState(false);
  const [locationForm] = Form.useForm<{ code: string; name: string; zone?: string }>();

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const warehouseResult = await listInventoryWarehouses();
      const active = (warehouseResult.list ?? []).filter((row) => row.status === "active");
      setWarehouses(active);
      const selected = active.find((row) => row.id === warehouseId) ?? active[0];
      if (!selected) {
        setWarehouseId("");
        setLocations([]);
        setPlacements([]);
        setSkus([]);
        return;
      }
      if (selected.id !== warehouseId) setWarehouseId(selected.id);
      const [locationResult, placementResult, skuResult] = await Promise.all([
        listWarehouseLocations(selected.id, true),
        listWarehouseSKUPlacements({ warehouseId: selected.id, includeInactive: true }),
        queryInventoryCenter({ warehouseId: selected.id, page: 1, pageSize: 100 }),
      ]);
      setLocations(locationResult.list ?? []);
      setPlacements(placementResult.list ?? []);
      setSkus(skuResult.list ?? []);
    } catch (nextError) {
      setError((nextError as Error)?.message || "库位与条码数据加载失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  }, [warehouseId]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!modalOpen) return;
    form.setFieldsValue({
      productSkuId: editing?.productSkuId,
      locationId: editing?.locationId,
      barcode: editing?.barcode ?? "",
      status: editing?.status ?? "active",
    });
  }, [editing, form, modalOpen]);

  useEffect(() => {
    if (!locationModalOpen) return;
    locationForm.resetFields();
  }, [locationForm, locationModalOpen]);

  const skuOptions = useMemo(
    () =>
      skus.map((row) => ({
        value: row.productSkuId,
        label: `${row.skuCode || row.productSkuId} · ${row.skuName || row.productTitle || "未命名 SKU"}`,
      })),
    [skus],
  );

  const openCreate = () => {
    setEditing(undefined);
    setModalOpen(true);
  };

  const openEdit = (row: WarehouseSKUPlacement) => {
    setEditing(row);
    setModalOpen(true);
  };

  const save = async (values: PlacementFormValues) => {
    if (!warehouseId) return;
    setSubmitting(true);
    try {
      if (editing) {
        await updateWarehouseSKUPlacement(editing.id, {
          locationId: values.locationId || undefined,
          barcode: values.barcode?.trim().toUpperCase() || undefined,
          status: values.status,
        });
      } else {
        await createWarehouseSKUPlacement({
          warehouseId,
          productSkuId: values.productSkuId,
          locationId: values.locationId || undefined,
          barcode: values.barcode?.trim().toUpperCase() || undefined,
          status: values.status,
        });
      }
      message.success(editing ? "绑定已更新" : "绑定已创建");
      setModalOpen(false);
      await load();
    } catch (nextError) {
      message.error((nextError as Error)?.message || "绑定保存失败，请检查条码和库位");
    } finally {
      setSubmitting(false);
    }
  };

  const saveLocation = async (values: { code: string; name: string; zone?: string }) => {
    if (!warehouseId) return;
    setLocationSubmitting(true);
    try {
      await createWarehouseLocation(warehouseId, {
        code: values.code.trim().toUpperCase(),
        name: values.name.trim(),
        zone: values.zone?.trim() || undefined,
      });
      message.success("库位已创建");
      setLocationModalOpen(false);
      await load();
    } catch (nextError) {
      message.error((nextError as Error)?.message || "库位创建失败，请检查编码是否重复");
    } finally {
      setLocationSubmitting(false);
    }
  };

  const columns: ProColumns<WarehouseSKUPlacement>[] = [
    { title: "SKU", dataIndex: "skuCode", width: 160, render: (_, row) => row.skuCode || row.productSkuId },
    { title: "商品", dataIndex: "productTitle", width: 200, ellipsis: true },
    { title: "条码", dataIndex: "barcode", width: 150, render: (value) => value || "未配置" },
    { title: "库位", dataIndex: "locationCode", width: 160, render: (_, row) => row.locationCode ? `${row.locationCode} · ${row.locationName || ""}` : "未配置" },
    { title: "状态", dataIndex: "status", width: 90, render: (_, row) => row.status === "active" ? <Tag color="success">启用</Tag> : <Tag>停用</Tag> },
    { title: "操作", valueType: "option", width: 90, render: (_, row) => canManage ? <Button type="link" size="small" onClick={() => openEdit(row)}>编辑</Button> : "—" },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.INVENTORY_VIEW} showForbiddenPage>
      <TmPageContainer
        title="库位与条码"
        subTitle="维护单仓 SKU 的拣货条码和物理库位；波次创建时会冻结当前绑定。"
        extra={
          <Space wrap>
            <Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>刷新</Button>
            <Button icon={<PlusOutlined />} disabled={!canManage || !warehouseId} onClick={() => setLocationModalOpen(true)}>新增库位</Button>
            <Button type="primary" icon={<PlusOutlined />} disabled={!canManage || !warehouseId} onClick={openCreate}>新增绑定</Button>
          </Space>
        }
      >
        {!canManage ? <Alert type="info" showIcon message="当前账号为只读模式，只能查看绑定，不能修改。" /> : null}
        {error ? <ErrorAlert title={error} actionHint={<Button onClick={() => void load()}>重试</Button>} /> : null}
        <TmProTable<WarehouseSKUPlacement>
          rowKey="id"
          columns={columns}
          dataSource={placements}
          loading={loading}
          filterBar={
            <Select
              aria-label="选择仓库"
              value={warehouseId || undefined}
              placeholder="选择仓库"
              style={{ width: 220, maxWidth: '100%' }}
              options={warehouses.map((row) => ({ value: row.id, label: `${row.code} · ${row.name}` }))}
              onChange={setWarehouseId}
            />
          }
          search={false}
          options={false}
          pagination={false}
          scroll={{ x: 900 }}
          cardBordered
          locale={{ emptyText: error ? "数据暂不可用" : "当前仓库尚无 SKU 绑定" }}
        />
        <Modal
          title={editing ? "编辑 SKU 绑定" : "新增 SKU 绑定"}
          open={modalOpen}
          confirmLoading={submitting}
          destroyOnHidden
          onCancel={() => !submitting && setModalOpen(false)}
          onOk={() => form.submit()}
          okText="保存"
          cancelText="取消"
        >
          <Form form={form} layout="vertical" onFinish={(values) => void save(values)} preserve={false}>
            <Form.Item label="SKU" name="productSkuId" rules={[{ required: true, message: "请选择 SKU" }]}>
              <Select showSearch optionFilterProp="label" disabled={Boolean(editing)} options={skuOptions} placeholder="选择当前仓库 SKU" />
            </Form.Item>
            <Form.Item label="条码" name="barcode" rules={[{ max: 128, message: "条码不能超过 128 个字符" }]}>
              <Input maxLength={128} placeholder="可选；启用后拣货必须匹配" />
            </Form.Item>
            <Form.Item label="库位" name="locationId">
              <Select
                allowClear
                options={locations.map((row) => ({
                  value: row.id,
                  label: `${row.code} · ${row.name}`,
                  disabled: row.status !== "active",
                }))}
                placeholder="选择物理库位"
              />
            </Form.Item>
            <Form.Item label="状态" name="status" rules={[{ required: true }]}>
              <Radio.Group options={[{ label: "启用", value: "active" }, { label: "停用", value: "inactive" }]} />
            </Form.Item>
          </Form>
        </Modal>
        <Modal
          title="新增库位"
          open={locationModalOpen}
          confirmLoading={locationSubmitting}
          destroyOnHidden
          onCancel={() => !locationSubmitting && setLocationModalOpen(false)}
          onOk={() => locationForm.submit()}
          okText="创建"
          cancelText="取消"
        >
          <Form form={locationForm} layout="vertical" onFinish={(values) => void saveLocation(values)} preserve={false}>
            <Form.Item label="库位编码" name="code" rules={[{ required: true, message: "请输入库位编码" }, { pattern: /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/, message: "仅支持字母、数字、点、下划线和短横线" }]}>
              <Input maxLength={64} placeholder="例如 A-01-01" />
            </Form.Item>
            <Form.Item label="库位名称" name="name" rules={[{ required: true, whitespace: true, message: "请输入库位名称" }]}>
              <Input maxLength={160} placeholder="例如 A 区第一货架" />
            </Form.Item>
            <Form.Item label="分区" name="zone">
              <Input maxLength={64} placeholder="可选，例如 A 区" />
            </Form.Item>
          </Form>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
