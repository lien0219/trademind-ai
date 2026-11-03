import { useLanguage } from '@/i18n/useLanguage';
import { tr } from '@/i18n';
import { DownloadOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { Alert, Button, DatePicker, Form, Input, Modal, Select, Space, Table, Tag, Typography, message } from 'antd';
import type { Dayjs } from 'dayjs';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import PermissionGuard from '@/components/PermissionGuard';
import { TmPageContainer } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import { createFXRate, downloadFXReport, FX_CURRENCIES, getFXReport, listFXRates } from '@/services/fxReport';
import type { FXCurrency, FXRate, FXReport, FXReportFilters, FXReportRow } from '@/services/fxReport';
import { formatProfitAmount } from '@/services/profitability';
import { createTenantReadGuard } from '@/utils/tenantAsync';
import { captureTenantContext, isTenantContextCurrent } from '@/utils/tenantContext';
import { PERMISSIONS } from '@/utils/permission';
import { formatDateTime } from '@/utils/formatTime';

const options = FX_CURRENCIES.map((currency) => ({ value: currency, label: currency }));
const conversionLabels: Record<FXReportRow['conversionStatus'], string> = {
  complete: '可汇总',
  source_incomplete: '来源数据不完整',
  rate_missing: '缺少历史汇率',
  order_date_missing: '缺少订单日期',
};

type RateForm = {
  fromCurrency: FXCurrency;
  rate: string;
  effectiveAt: Dayjs;
  evidence: string;
};

export default function CurrencyReportPage() {
  useLanguage();
  const { can } = usePermission();
  const canManage = can(PERMISSIONS.SETTINGS_MANAGE);
  const canExport = can(PERMISSIONS.ORDER_PROFIT_EXPORT);
  const [baseCurrency, setBaseCurrency] = useState<FXCurrency>('CNY');
  const [dateRange, setDateRange] = useState<[Dayjs | null, Dayjs | null] | null>(null);
  const [report, setReport] = useState<FXReport>();
  const [rates, setRates] = useState<FXRate[]>([]);
  const [loading, setLoading] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [saving, setSaving] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [error, setError] = useState('');
  const [form] = Form.useForm<RateForm>();
  const readGuard = useMemo(createTenantReadGuard, []);
  const exportGuard = useMemo(createTenantReadGuard, []);
  const saveLocked = useRef(false);
  const exportLocked = useRef(false);

  const filters = useMemo<FXReportFilters>(() => ({
    baseCurrency,
    start: dateRange?.[0]?.startOf('day').toISOString(),
    end: dateRange?.[1]?.endOf('day').toISOString(),
  }), [baseCurrency, dateRange]);

  const load = useCallback(async () => {
    const isCurrent = readGuard.begin();
    setLoading(true);
    setError('');
    setReport(undefined);
    setRates([]);
    try {
      const [nextReport, nextRates] = await Promise.all([getFXReport(filters), listFXRates(filters.baseCurrency)]);
      if (!isCurrent()) return;
      setReport(nextReport);
      setRates(nextRates.items || []);
    } catch (cause) {
      if (isCurrent()) setError((cause as Error)?.message || tr('汇率报表加载失败'));
    } finally {
      if (isCurrent()) setLoading(false);
    }
  }, [filters, readGuard]);

  useEffect(() => {
    void load();
    return () => readGuard.invalidate();
  }, [load, readGuard]);

  useEffect(() => () => exportGuard.invalidate(), [exportGuard]);

  const save = async (values: RateForm) => {
    if (!canManage || saveLocked.current) return;
    const context = captureTenantContext();
    saveLocked.current = true;
    setSaving(true);
    try {
      await createFXRate({
        fromCurrency: values.fromCurrency,
        toCurrency: baseCurrency,
        rate: values.rate,
        effectiveAt: values.effectiveAt.toISOString(),
        evidence: values.evidence,
      });
      if (!isTenantContextCurrent(context)) return;
      setModalOpen(false);
      form.resetFields();
      message.success(tr('汇率记录已保存'));
      await load();
    } catch (cause) {
      if (isTenantContextCurrent(context)) {
        message.error((cause as Error)?.message || tr('汇率保存失败'));
      }
    } finally {
      saveLocked.current = false;
      if (isTenantContextCurrent(context)) setSaving(false);
    }
  };

  const exportReport = async () => {
    if (!canExport || exportLocked.current) return;
    const isCurrent = exportGuard.begin();
    exportLocked.current = true;
    setExporting(true);
    try {
      const success = await downloadFXReport(filters, isCurrent);
      if (success && isCurrent()) message.success(tr('跨币种报表已导出'));
    } catch (cause) {
      if (isCurrent()) message.error((cause as Error)?.message || tr('跨币种报表导出失败'));
    } finally {
      exportLocked.current = false;
      if (isCurrent()) setExporting(false);
    }
  };

  return (
    <PermissionGuard require={PERMISSIONS.ORDER_PROFIT_VIEW} showForbiddenPage>
      <TmPageContainer
        title={tr('跨币种利润报表')}
        subTitle={tr('仅汇总来源完整且有有效历史汇率的订单，原始金额不变')}
        extra={
          <Space wrap>
            <Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>{tr('重新加载')}</Button>
            <Button icon={<DownloadOutlined />} disabled={!canExport || !report} loading={exporting} onClick={() => void exportReport()}>
              {tr('导出换汇报表')}
            </Button>
            {canManage ? (
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>{tr('登记历史汇率')}</Button>
            ) : null}
          </Space>
        }
      >
        <Alert
          type="warning"
          showIcon
          message={tr('汇率由企业管理员登记，仅供预估报表使用')}
          description={tr('按照订单下单时间选取已登记的最新有效汇率；缺失汇率或原始费用不完整的订单不进入合计。此报表不是会计利润。')}
        />
        <Space wrap style={{ marginBlock: 16 }}>
          <Typography.Text>{tr('报表基准币种')}</Typography.Text>
          <Select aria-label={tr('报表基准币种')} value={baseCurrency} options={options} onChange={setBaseCurrency} style={{ width: 120 }} />
          <DatePicker.RangePicker aria-label={tr('下单日期范围')} value={dateRange} onChange={(values) => setDateRange(values)} />
        </Space>
        {error ? <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} /> : null}
        {report ? (
          <>
            <Space wrap size="large" style={{ marginBottom: 16 }}>
              <Typography.Text strong>{tr('订单数量')}: {report.totalOrders}</Typography.Text>
              <Typography.Text>{tr('可汇总')}: {report.includedOrders}</Typography.Text>
              <Typography.Text type={report.excludedOrders ? 'warning' : 'secondary'}>{tr('未计入汇总')}: {report.excludedOrders}</Typography.Text>
              <Typography.Text strong>{tr('可汇总订单收入')}: {formatProfitAmount(report.revenueTotalMinor, baseCurrency)}</Typography.Text>
              <Typography.Text strong>{tr('可汇总订单预估利润')}: {formatProfitAmount(report.estimatedProfitTotalMinor, baseCurrency)}</Typography.Text>
            </Space>
            <Table<FXReportRow>
              rowKey="orderId"
              loading={loading}
              size="middle"
              pagination={{ pageSize: 20, showSizeChanger: true }}
              scroll={{ x: 1150 }}
              dataSource={report.rows}
              columns={[
                { title: tr('订单号'), dataIndex: 'orderNo', width: 180, ellipsis: true },
                { title: tr('原始币种'), dataIndex: 'currency', width: 90 },
                { title: tr('原始收入'), width: 150, render: (_, row) => formatProfitAmount(row.components.revenue.amountMinor, row.currency) },
                { title: tr('原始预估利润'), width: 150, render: (_, row) => formatProfitAmount(row.estimatedProfitMinor, row.currency) },
                { title: tr('换算收入'), width: 150, render: (_, row) => formatProfitAmount(row.converted.revenueMinor, baseCurrency) },
                { title: tr('换算预估利润'), width: 150, render: (_, row) => formatProfitAmount(row.converted.estimatedProfitMinor, baseCurrency) },
                { title: tr('汇率'), dataIndex: 'rate', width: 110, render: (value: string) => value || '—' },
                { title: tr('汇率生效时间'), dataIndex: 'rateEffectiveAt', width: 190, render: (value: string) => value ? formatDateTime(value) : '—' },
                { title: tr('完整性'), dataIndex: 'conversionStatus', width: 160, render: (value: FXReportRow['conversionStatus']) => <Tag color={value === 'complete' ? 'green' : 'orange'}>{tr(conversionLabels[value] || value)}</Tag> },
              ]}
            />
          </>
        ) : null}
        <Typography.Title level={5}>{tr('已登记汇率')}</Typography.Title>
        <Table<FXRate>
          rowKey="id"
          size="small"
          loading={loading}
          pagination={{ pageSize: 10 }}
          dataSource={rates}
          columns={[
            { title: tr('原始币种'), dataIndex: 'fromCurrency' },
            { title: tr('目标币种'), dataIndex: 'toCurrency' },
            { title: tr('汇率'), dataIndex: 'rate' },
            { title: tr('汇率生效时间'), dataIndex: 'effectiveAt', render: (value: string) => formatDateTime(value) },
            { title: tr('汇率来源依据'), dataIndex: 'evidence', ellipsis: true },
          ]}
        />
        <Modal
          title={tr('登记历史汇率')}
          open={modalOpen}
          okText={tr('确认登记')}
          okButtonProps={{ loading: saving }}
          onOk={() => void form.submit()}
          onCancel={() => { if (!saving) setModalOpen(false); }}
          destroyOnHidden
        >
          <Form form={form} layout="vertical" onFinish={(values) => void save(values)}>
            <Form.Item name="fromCurrency" label={tr('原始币种')} rules={[{ required: true }]}>
              <Select options={options.filter((option) => option.value !== baseCurrency)} />
            </Form.Item>
            <Form.Item label={tr('目标币种')}>
              <Typography.Text strong>{baseCurrency}</Typography.Text>
            </Form.Item>
            <Form.Item name="rate" label={tr('汇率')} rules={[{ required: true }, { pattern: /^[0-9]{1,12}(\.[0-9]{1,10})?$/, message: tr('请输入正数汇率，最多十位小数') }]}>
              <Input placeholder="1.000000" />
            </Form.Item>
            <Form.Item name="effectiveAt" label={tr('汇率生效时间')} rules={[{ required: true }]}>
              <DatePicker showTime style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="evidence" label={tr('汇率来源依据')} rules={[{ required: true, min: 8, max: 250 }]}>
              <Input.TextArea rows={3} maxLength={250} />
            </Form.Item>
          </Form>
        </Modal>
      </TmPageContainer>
    </PermissionGuard>
  );
}
