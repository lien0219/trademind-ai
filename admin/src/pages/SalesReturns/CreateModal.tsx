import type { ColumnsType } from 'antd/es/table';
import {
  Alert,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Table,
  Typography,
  message,
} from 'antd';
import { useEffect, useRef, useState } from 'react';
import { ErrorAlert } from '@/components/ui';
import {
  createSalesReturn,
  createSalesReturnIdempotencyKey,
  extractSalesReturnAPIError,
  listReturnableSalesItems,
  salesReturnErrorMessage,
  type ReturnableSalesItem,
  type SalesReturnType,
} from '@/services/salesReturns';

type Values = {
  type: SalesReturnType;
  reason: string;
  remark: string;
  quantities: Record<string, number>;
  dispositions: Record<string, 'sellable' | 'damaged'>;
  refundAmounts: Record<string, number>;
};

type Props = {
  orderId: string;
  open: boolean;
  onClose: () => void;
  onCreated: (id: string) => void;
};

export default function SalesReturnCreateModal({
  orderId,
  open,
  onClose,
  onCreated,
}: Props) {
  const [form] = Form.useForm<Values>();
  const returnType = Form.useWatch('type', form) || 'return_refund';
  const [items, setItems] = useState<ReturnableSalesItem[]>([]);
  const [currency, setCurrency] = useState('CNY');
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const [requestKey, setRequestKey] = useState('');
  const sequenceRef = useRef(0);
  const submittingRef = useRef(false);

  useEffect(() => {
    if (!open || !orderId) return;
    const sequence = ++sequenceRef.current;
    setLoading(true);
    setError('');
    setRequestKey(createSalesReturnIdempotencyKey('sales-return-create'));
    form.resetFields();
    form.setFieldsValue({
      type: 'return_refund',
      reason: '',
      remark: '',
      quantities: {},
      dispositions: {},
      refundAmounts: {},
    });
    void listReturnableSalesItems(orderId)
      .then((result) => {
        if (sequence !== sequenceRef.current) return;
        setItems(result.list || []);
        setCurrency(result.currency || 'CNY');
        form.setFieldsValue({
          quantities: Object.fromEntries(
            (result.list || []).map((item) => [item.orderItemId, 0]),
          ),
          dispositions: Object.fromEntries(
            (result.list || []).map((item) => [item.orderItemId, 'sellable']),
          ),
          refundAmounts: Object.fromEntries(
            (result.list || []).map((item) => [item.orderItemId, 0]),
          ),
        });
      })
      .catch((nextError) => {
        if (sequence !== sequenceRef.current) return;
        setItems([]);
        setError(
          salesReturnErrorMessage(
            extractSalesReturnAPIError(nextError),
            '可售后明细加载失败，请稍后重试。',
          ),
        );
      })
      .finally(() => {
        if (sequence === sequenceRef.current) setLoading(false);
      });
    return () => {
      sequenceRef.current += 1;
    };
  }, [form, open, orderId]);

  const submit = async (values: Values) => {
    if (submittingRef.current) return;
    const selected = items.filter(
      (item) => Number(values.quantities?.[item.orderItemId] || 0) > 0,
    );
    if (selected.length === 0) {
      message.warning('请至少填写一条售后数量。');
      return;
    }
    const payloadItems = selected.map((item) => ({
      orderItemId: item.orderItemId,
      quantity: Number(values.quantities[item.orderItemId]),
      disposition:
        values.type === 'return_refund'
          ? values.dispositions[item.orderItemId] || 'sellable'
          : ('' as const),
      refundAmountMinor: Math.round(
        Number(values.refundAmounts?.[item.orderItemId] || 0) * 100,
      ),
    }));
    if (
      payloadItems.some((item) => item.refundAmountMinor < 0) ||
      payloadItems.reduce((sum, item) => sum + item.refundAmountMinor, 0) < 1
    ) {
      message.warning('退款金额必须大于 0。');
      return;
    }
    submittingRef.current = true;
    setSubmitting(true);
    try {
      const result = await createSalesReturn({
        idempotencyKey: requestKey,
        orderId,
        type: values.type,
        reason: values.reason.trim(),
        remark: values.remark?.trim() || '',
        items: payloadItems,
      });
      message.success(`售后草稿已创建：${result.returnNo}`);
      onCreated(result.id);
    } catch (nextError) {
      message.error(
        salesReturnErrorMessage(
          extractSalesReturnAPIError(nextError),
          '售后草稿创建失败，请核对后重试。',
        ),
      );
    } finally {
      submittingRef.current = false;
      setSubmitting(false);
    }
  };

  const columns: ColumnsType<ReturnableSalesItem> = [
    { title: '商品', dataIndex: 'productTitle', width: 190, ellipsis: true },
    {
      title: '规格',
      width: 150,
      ellipsis: true,
      render: (_, item) => item.skuName || item.skuCode,
    },
    {
      title: '可退',
      dataIndex: 'remainingQuantity',
      width: 72,
      align: 'right',
    },
    {
      title: '本次数量',
      width: 130,
      render: (_, item) => (
        <Form.Item
          name={['quantities', item.orderItemId]}
          noStyle
          rules={[
            {
              type: 'number',
              min: 0,
              max: item.remainingQuantity,
              message: `不能超过 ${item.remainingQuantity}`,
            },
          ]}
        >
          <InputNumber
            min={0}
            max={item.remainingQuantity}
            precision={0}
            controls
            style={{ width: 104 }}
          />
        </Form.Item>
      ),
    },
    {
      title: '入库处置',
      width: 130,
      render: (_, item) =>
        returnType === 'refund_only' ? (
          <Typography.Text type="secondary">不入库</Typography.Text>
        ) : (
          <Form.Item name={['dispositions', item.orderItemId]} noStyle>
            <Select
              style={{ width: 108 }}
              options={[
                { value: 'sellable', label: '良品' },
                { value: 'damaged', label: '残次品' },
              ]}
            />
          </Form.Item>
        ),
    },
    {
      title: `退款金额 (${currency})`,
      width: 170,
      render: (_, item) => (
        <Form.Item
          name={['refundAmounts', item.orderItemId]}
          noStyle
          rules={[{ type: 'number', min: 0, message: '金额不能为负数' }]}
        >
          <InputNumber
            min={0}
            precision={2}
            controls={false}
            style={{ width: 140 }}
          />
        </Form.Item>
      ),
    },
  ];

  return (
    <Modal
      title="发起退货退款"
      open={open}
      width={920}
      confirmLoading={submitting}
      okText="创建草稿"
      cancelText="取消"
      okButtonProps={{
        disabled: loading || Boolean(error) || items.length === 0,
      }}
      onCancel={() => {
        if (!submitting) onClose();
      }}
      onOk={() => form.submit()}
      forceRender
    >
      <Form
        form={form}
        layout="vertical"
        preserve={false}
        onFinish={(values) => void submit(values)}
      >
        {error ? <ErrorAlert title={error} /> : null}
        {!loading && !error && items.length === 0 ? (
          <Alert type="info" showIcon message="该订单没有剩余可售后数量。" />
        ) : null}
        <div className="tm-sales-return-form-grid">
          <Form.Item label="售后类型" name="type" rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'return_refund', label: '退货退款' },
                { value: 'refund_only', label: '仅退款' },
              ]}
            />
          </Form.Item>
          <Form.Item
            label="售后原因"
            name="reason"
            rules={[
              { required: true, whitespace: true, message: '请输入售后原因' },
              { max: 128 },
            ]}
          >
            <Input maxLength={128} />
          </Form.Item>
        </div>
        <Form.Item label="备注" name="remark" rules={[{ max: 520 }]}>
          <Input.TextArea rows={2} maxLength={520} showCount />
        </Form.Item>
        <Table<ReturnableSalesItem>
          rowKey="orderItemId"
          size="small"
          loading={loading}
          pagination={false}
          dataSource={items}
          columns={columns}
          scroll={{ x: 850 }}
          locale={{ emptyText: '暂无可售后明细' }}
        />
      </Form>
    </Modal>
  );
}
