import { useLanguage } from '@/i18n/useLanguage';
import { tr, trFormat } from '@/i18n';
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import {
  Alert,
  Button,
  Form,
  Input,
  InputNumber,
  Modal,
  Radio,
  Select,
  Space,
  Tabs,
  Tag,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import PermissionGuard from '@/components/PermissionGuard';
import {
  ErrorAlert,
  TmPageContainer,
  TmPageHeaderExtra,
  TmProTable,
} from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import {
  createShippingChannel,
  createShippingRateTemplate,
  listShippingChannels,
  listShippingRateTemplates,
  updateShippingChannel,
  updateShippingRateTemplate,
  type RatePayload,
  type ShippingChannel,
  type ShippingRateTemplate,
} from '@/services/logistics';
import { listWarehouses, type Warehouse } from '@/services/procurement';
import { PERMISSIONS } from '@/utils/permission';
import './index.less';

type ChannelForm = {
  code: string;
  name: string;
  carrier: string;
  status: 'active' | 'inactive';
};
type RateForm = RatePayload & { status: 'active' | 'inactive' };

const money = (minor: number, currency: string) =>
  `${currency} ${(minor / 100).toFixed(2)}`;
const statusTag = (status: string) =>
  status === 'active' ? <Tag color="success">{tr("启用")}</Tag> : <Tag>{tr("停用")}</Tag>;
const errorText = (error: unknown, fallback: string) => {
  const value = (error as Error)?.message?.trim();
  return value && /[\u3400-\u9fff]/.test(value) ? value : fallback;
};

export default function LogisticsChannelsPage() {
  useLanguage();
  const { can, readonly } = usePermission();
  const canManage = !readonly && can(PERMISSIONS.LOGISTICS_MANAGE);
  const [channelForm] = Form.useForm<ChannelForm>();
  const [rateForm] = Form.useForm<RateForm>();
  const [channels, setChannels] = useState<ShippingChannel[]>([]);
  const [rates, setRates] = useState<ShippingRateTemplate[]>([]);
  const [warehouses, setWarehouses] = useState<Warehouse[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [channelModal, setChannelModal] = useState(false);
  const [rateModal, setRateModal] = useState(false);
  const [editingChannel, setEditingChannel] = useState<ShippingChannel>();
  const [editingRate, setEditingRate] = useState<ShippingRateTemplate>();
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [channelResult, rateResult, warehouseResult] = await Promise.all([
        listShippingChannels(),
        listShippingRateTemplates(),
        listWarehouses(),
      ]);
      setChannels(channelResult.list ?? []);
      setRates(rateResult.list ?? []);
      setWarehouses(warehouseResult.list ?? []);
    } catch (nextError) {
      setError(errorText(nextError, '物流配置加载失败，请稍后重试。'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const activeChannels = useMemo(
    () => channels.filter((item) => item.status === 'active'),
    [channels],
  );

  const openChannel = (row?: ShippingChannel) => {
    setEditingChannel(row);
    channelForm.setFieldsValue(
      row
        ? {
            code: row.code,
            name: row.name,
            carrier: row.carrier,
            status: row.status,
          }
        : { code: '', name: '', carrier: '', status: 'active' },
    );
    setChannelModal(true);
  };

  const openRate = (row?: ShippingRateTemplate) => {
    setEditingRate(row);
    rateForm.setFieldsValue(
      row
        ? {
            channelId: row.channelId,
            warehouseId: row.warehouseId,
            code: row.code,
            name: row.name,
            countryCode: row.countryCode,
            region: row.region ?? '',
            postalCodePrefix: row.postalCodePrefix ?? '',
            minWeightGrams: row.minWeightGrams,
            maxWeightGrams: row.maxWeightGrams,
            baseFeeMinor: row.baseFeeMinor,
            perKilogramFeeMinor: row.perKilogramFeeMinor,
            currency: row.currency,
            priority: row.priority,
            status: row.status,
          }
        : {
            channelId: activeChannels[0]?.id,
            code: '',
            name: '',
            countryCode: 'CN',
            region: '',
            postalCodePrefix: '',
            minWeightGrams: 0,
            maxWeightGrams: 1000,
            baseFeeMinor: 0,
            perKilogramFeeMinor: 0,
            currency: 'CNY',
            priority: 0,
            status: 'active',
          },
    );
    setRateModal(true);
  };

  const saveChannel = async (values: ChannelForm) => {
    setSubmitting(true);
    try {
      if (editingChannel) {
        await updateShippingChannel(editingChannel.id, {
          expectedRevision: editingChannel.revision,
          name: values.name.trim(),
          carrier: values.carrier.trim(),
          status: values.status,
        });
        message.success(tr("物流渠道已更新"));
      } else {
        await createShippingChannel({
          code: values.code.trim().toUpperCase(),
          name: values.name.trim(),
          carrier: values.carrier.trim(),
        });
        message.success(tr("物流渠道已创建"));
      }
      setChannelModal(false);
      await load();
    } catch (nextError) {
      message.error(tr(errorText(nextError, tr('物流渠道保存失败，请刷新后重试。'))));
    } finally {
      setSubmitting(false);
    }
  };

  const saveRate = async (values: RateForm) => {
    const payload: RatePayload = {
      channelId: values.channelId,
      warehouseId: values.warehouseId || undefined,
      code: values.code.trim().toUpperCase(),
      name: values.name.trim(),
      countryCode: values.countryCode.trim().toUpperCase(),
      region: values.region?.trim() || undefined,
      postalCodePrefix:
        values.postalCodePrefix?.trim().toUpperCase() || undefined,
      minWeightGrams: values.minWeightGrams,
      maxWeightGrams: values.maxWeightGrams,
      baseFeeMinor: values.baseFeeMinor,
      perKilogramFeeMinor: values.perKilogramFeeMinor,
      currency: values.currency.trim().toUpperCase(),
      priority: values.priority,
    };
    setSubmitting(true);
    try {
      if (editingRate) {
        await updateShippingRateTemplate(editingRate.id, {
          ...payload,
          expectedRevision: editingRate.revision,
          status: values.status,
        });
        message.success(tr("运费模板已更新"));
      } else {
        await createShippingRateTemplate(payload);
        message.success(tr("运费模板已创建"));
      }
      setRateModal(false);
      await load();
    } catch (nextError) {
      message.error(
        tr(errorText(nextError, tr('运费模板保存失败，请检查重量区间和引用配置。'))),
      );
    } finally {
      setSubmitting(false);
    }
  };

  const channelColumns: ProColumns<ShippingChannel>[] = [
    { title: tr("渠道编码"), dataIndex: 'code', width: 140, copyable: true },
    { title: tr("渠道名称"), dataIndex: 'name', minWidth: 180, ellipsis: true },
    { title: tr("承运商"), dataIndex: 'carrier', minWidth: 160, ellipsis: true },
    {
      title: tr("状态"),
      dataIndex: 'status',
      width: 90,
      render: (_, row) => statusTag(row.status),
    },
    { title: tr("版本"), dataIndex: 'revision', width: 80 },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 90,
      render: (_, row) =>
        canManage ? (
          <Button type="link" size="small" onClick={() => openChannel(row)}>
            {tr("编辑")}
          </Button>
        ) : (
          '—'
        ),
    },
  ];
  const rateColumns: ProColumns<ShippingRateTemplate>[] = [
    { title: tr("模板编码"), dataIndex: 'code', width: 140, copyable: true },
    { title: tr("模板名称"), dataIndex: 'name', minWidth: 180, ellipsis: true },
    {
      title: tr("渠道"),
      width: 180,
      render: (_, row) =>
        [row.channelCode, row.channelName].filter(Boolean).join(' · '),
    },
    {
      title: tr("仓库"),
      width: 150,
      render: (_, row) =>
        row.warehouseId
          ? [row.warehouseCode, row.warehouseName]
              .filter(Boolean)
              .join(' · ') || row.warehouseId
          : tr("全部仓库"),
    },
    {
      title: tr("目的地"),
      width: 180,
      render: (_, row) =>
        [
          row.countryCode,
          row.region,
          row.postalCodePrefix && trFormat("邮编 {prefix}*", { prefix: row.postalCodePrefix }),
        ]
          .filter(Boolean)
          .join(' · '),
    },
    {
      title: tr("重量（克）"),
      width: 150,
      render: (_, row) => `${row.minWeightGrams}–${row.maxWeightGrams}`,
    },
    {
      title: tr("计费"),
      width: 230,
      render: (_, row) =>
        trFormat("{value0} + 每起始千克 {value1}", { value0: String(money(row.baseFeeMinor, row.currency)), value1: String(money(row.perKilogramFeeMinor, row.currency)) }),
    },
    {
      title: tr("状态"),
      dataIndex: 'status',
      width: 90,
      render: (_, row) => statusTag(row.status),
    },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 90,
      render: (_, row) =>
        canManage ? (
          <Button type="link" size="small" onClick={() => openRate(row)}>
            {tr("编辑")}
          </Button>
        ) : (
          '—'
        ),
    },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.LOGISTICS_VIEW} showForbiddenPage>
      <TmPageContainer
        title={tr("物流渠道与运费模板")}
        subTitle={tr("维护本地渠道和确定性计费规则。试算结果仅供人工确认，不连接物流商、不申请运单或面单。")}
        extra={
          <TmPageHeaderExtra>
            <Button
              icon={<ReloadOutlined />}
              loading={loading}
              onClick={() => void load()}
            >
              {tr("刷新")}
            </Button>
          </TmPageHeaderExtra>
        }
      >
        {!canManage ? (
          <Alert
            showIcon
            type="info"
            message={tr("当前账号只能查看物流配置，不能创建或修改。")}
          />
        ) : null}
        {error ? (
          <ErrorAlert
            title={tr(error)}
            actionHint={<Button onClick={() => void load()}>{tr("重新加载")}</Button>}
          />
        ) : null}
        <Tabs
          items={[
            {
              key: 'channels',
              label: trFormat("物流渠道（{value0}）", { value0: channels.length }),
              children: (
                <TmProTable<ShippingChannel>
                  rowKey="id"
                  columns={channelColumns}
                  dataSource={channels}
                  loading={loading}
                  search={false}
                  options={false}
                  pagination={false}
                  cardBordered
                  scroll={{ x: 820 }}
                  toolBarRender={() => [
                    <Button
                      key="create"
                      type="primary"
                      icon={<PlusOutlined />}
                      disabled={!canManage}
                      onClick={() => openChannel()}
                    >
                      {tr("新建渠道")}
                    </Button>,
                  ]}
                  locale={{
                    emptyText: error ? tr('渠道列表暂不可用') : tr('暂无物流渠道'),
                  }}
                />
              ),
            },
            {
              key: 'rates',
              label: trFormat("运费模板（{value0}）", { value0: rates.length }),
              children: (
                <TmProTable<ShippingRateTemplate>
                  rowKey="id"
                  columns={rateColumns}
                  dataSource={rates}
                  loading={loading}
                  search={false}
                  options={false}
                  pagination={false}
                  cardBordered
                  scroll={{ x: 1350 }}
                  toolBarRender={() => [
                    <Button
                      key="create"
                      type="primary"
                      icon={<PlusOutlined />}
                      disabled={!canManage || activeChannels.length === 0}
                      onClick={() => openRate()}
                    >
                      {tr("新建模板")}
                    </Button>,
                  ]}
                  locale={{
                    emptyText: error
                      ? tr('模板列表暂不可用')
                      : tr('暂无运费模板，请先创建启用渠道。'),
                  }}
                />
              ),
            },
          ]}
        />

        <Modal
          title={
            editingChannel
              ? trFormat("编辑渠道 · {value0}", { value0: String(editingChannel.code) })
              : tr('新建物流渠道')
          }
          open={channelModal}
          confirmLoading={submitting}
          onCancel={() => !submitting && setChannelModal(false)}
          onOk={() => channelForm.submit()}
          forceRender
        >
          <Form
            form={channelForm}
            layout="vertical"
            preserve={false}
            onFinish={(values) => void saveChannel(values)}
          >
            <Form.Item
              label={tr("渠道编码")}
              name="code"
              rules={[
                { required: true },
                {
                  pattern: /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/,
                  message: tr("仅支持字母、数字、下划线和短横线"),
                },
              ]}
            >
              <Input
                disabled={Boolean(editingChannel)}
                maxLength={64}
                placeholder={tr("例如 SF_STANDARD")}
              />
            </Form.Item>
            <Form.Item
              label={tr("渠道名称")}
              name="name"
              rules={[{ required: true, whitespace: true }]}
            >
              <Input maxLength={160} />
            </Form.Item>
            <Form.Item
              label={tr("承运商")}
              name="carrier"
              rules={[{ required: true, whitespace: true }]}
            >
              <Input
                maxLength={128}
                placeholder={tr("提交出库复核时使用的承运商快照")}
              />
            </Form.Item>
            {editingChannel ? (
              <Form.Item label={tr("状态")} name="status">
                <Radio.Group
                  options={[
                    { label: tr("启用"), value: 'active' },
                    { label: tr("停用"), value: 'inactive' },
                  ]}
                />
              </Form.Item>
            ) : null}
          </Form>
        </Modal>

        <Modal
          width={760}
          title={
            editingRate ? trFormat("编辑运费模板 · {value0}", { value0: String(editingRate.code) }) : tr('新建运费模板')
          }
          open={rateModal}
          confirmLoading={submitting}
          onCancel={() => !submitting && setRateModal(false)}
          onOk={() => rateForm.submit()}
          forceRender
        >
          <Form
            form={rateForm}
            layout="vertical"
            preserve={false}
            onFinish={(values) => void saveRate(values)}
          >
            <Space wrap size="middle" className="tm-logistics-rate-fields">
              <Form.Item
                label={tr("物流渠道")}
                name="channelId"
                rules={[{ required: true }]}
              >
                <Select
                  className="tm-logistics-field-lg"
                  options={activeChannels.map((row) => ({
                    label: `${row.code} · ${row.name}`,
                    value: row.id,
                  }))}
                />
              </Form.Item>
              <Form.Item label={tr("适用仓库")} name="warehouseId">
                <Select
                  allowClear
                  className="tm-logistics-field-lg"
                  placeholder={tr("全部仓库")}
                  options={warehouses
                    .filter((row) => row.status === 'active')
                    .map((row) => ({
                      label: `${row.code} · ${row.name}`,
                      value: row.id,
                    }))}
                />
              </Form.Item>
              <Form.Item
                label={tr("模板编码")}
                name="code"
                rules={[
                  { required: true },
                  { pattern: /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/ },
                ]}
              >
                <Input
                  disabled={Boolean(editingRate)}
                  className="tm-logistics-field-lg"
                />
              </Form.Item>
              <Form.Item
                label={tr("模板名称")}
                name="name"
                rules={[{ required: true, whitespace: true }]}
              >
                <Input className="tm-logistics-field-lg" maxLength={160} />
              </Form.Item>
              <Form.Item
                label={tr("国家/地区代码")}
                name="countryCode"
                rules={[
                  { required: true },
                  { pattern: /^[A-Za-z]{2}$/, message: tr("请输入 2 位代码") },
                ]}
              >
                <Input className="tm-logistics-field-sm" maxLength={2} />
              </Form.Item>
              <Form.Item label={tr("省/州（可选）")} name="region">
                <Input className="tm-logistics-field-md" maxLength={120} />
              </Form.Item>
              <Form.Item label={tr("邮编前缀（可选）")} name="postalCodePrefix">
                <Input className="tm-logistics-field-md" maxLength={32} />
              </Form.Item>
              <Form.Item
                label={tr("起始重量（克）")}
                name="minWeightGrams"
                rules={[{ required: true }]}
              >
                <InputNumber min={0} max={4_999_999} precision={0} />
              </Form.Item>
              <Form.Item
                label={tr("最大重量（克）")}
                name="maxWeightGrams"
                rules={[{ required: true }]}
              >
                <InputNumber min={1} max={5_000_000} precision={0} />
              </Form.Item>
              <Form.Item
                label={tr("基础费（分）")}
                name="baseFeeMinor"
                rules={[{ required: true }]}
              >
                <InputNumber min={0} precision={0} />
              </Form.Item>
              <Form.Item
                label={tr("续重每千克（分）")}
                name="perKilogramFeeMinor"
                rules={[{ required: true }]}
              >
                <InputNumber min={0} precision={0} />
              </Form.Item>
              <Form.Item
                label={tr("币种")}
                name="currency"
                rules={[{ required: true }, { pattern: /^[A-Za-z]{3}$/ }]}
              >
                <Input className="tm-logistics-field-xs" maxLength={3} />
              </Form.Item>
              <Form.Item
                label={tr("优先级")}
                name="priority"
                rules={[{ required: true }]}
              >
                <InputNumber min={0} max={999} precision={0} />
              </Form.Item>
              {editingRate ? (
                <Form.Item label={tr("状态")} name="status">
                  <Radio.Group
                    options={[
                      { label: tr("启用"), value: 'active' },
                      { label: tr("停用"), value: 'inactive' },
                    ]}
                  />
                </Form.Item>
              ) : null}
            </Space>
            <Alert
              showIcon
              type="info"
              message={tr("基础费覆盖起始重量；超出部分按每个起始千克计费。国家、区域、邮编和重量必须同时匹配才会产生候选报价。")}
            />
          </Form>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
