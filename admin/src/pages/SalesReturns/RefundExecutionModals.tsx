import {
  Alert,
  DatePicker,
  Form,
  Input,
  Modal,
  Select,
  type FormInstance,
} from 'antd';
import type { Dayjs } from 'dayjs';

export type ManualRefundValues = {
  result: 'succeeded' | 'failed' | 'unknown';
  externalRefundId?: string;
  executedAt: Dayjs;
  reason?: string;
};

export type RefundLocalAction = 'confirm_platform' | 'cancel';
export type RefundActionValues = { reason?: string };

type Props = {
  manualForm: FormInstance<ManualRefundValues>;
  actionForm: FormInstance<RefundActionValues>;
  manualOpen: boolean;
  localAction?: RefundLocalAction;
  submitting: boolean;
  onManualCancel: () => void;
  onManualSubmit: (values: ManualRefundValues) => void;
  onActionCancel: () => void;
  onActionSubmit: (values: RefundActionValues) => void;
};

export default function RefundExecutionModals({
  manualForm,
  actionForm,
  manualOpen,
  localAction,
  submitting,
  onManualCancel,
  onManualSubmit,
  onActionCancel,
  onActionSubmit,
}: Props) {
  const manualResult = Form.useWatch('result', manualForm);

  return (
    <>
      <Modal
        title="人工登记退款结果"
        open={manualOpen}
        confirmLoading={submitting}
        okText="确认登记"
        cancelText="取消"
        onCancel={onManualCancel}
        onOk={() => manualForm.submit()}
        forceRender
      >
        <Form
          form={manualForm}
          layout="vertical"
          preserve={false}
          onFinish={onManualSubmit}
        >
          <Alert
            type="warning"
            showIcon
            message="请先在外部平台或支付渠道完成操作，再如实登记结果。本系统不会发起外部退款。"
          />
          <Form.Item
            label="退款结果"
            name="result"
            rules={[{ required: true, message: '请选择退款结果' }]}
          >
            <Select
              options={[
                { value: 'succeeded', label: '退款成功' },
                { value: 'failed', label: '退款失败' },
                { value: 'unknown', label: '结果未知，待平台事实确认' },
              ]}
            />
          </Form.Item>
          <Form.Item
            label="外部退款编号"
            name="externalRefundId"
            rules={[
              {
                validator: (_, value) =>
                  manualResult !== 'succeeded' || String(value || '').trim()
                    ? Promise.resolve()
                    : Promise.reject(new Error('退款成功时必须填写外部退款编号')),
              },
              { max: 255, message: '外部退款编号不能超过 255 个字符' },
            ]}
          >
            <Input maxLength={255} placeholder="支付渠道或平台返回的退款编号" />
          </Form.Item>
          <Form.Item
            label="外部执行时间"
            name="executedAt"
            rules={[{ required: true, message: '请选择外部执行时间' }]}
          >
            <DatePicker showTime style={{ width: '100%' }} allowClear={false} />
          </Form.Item>
          <Form.Item
            label="结果说明"
            name="reason"
            rules={[
              {
                validator: (_, value) =>
                  manualResult === 'succeeded' || String(value || '').trim()
                    ? Promise.resolve()
                    : Promise.reject(new Error('失败或未知结果必须填写说明')),
              },
              { max: 255, message: '结果说明不能超过 255 个字符' },
            ]}
          >
            <Input.TextArea rows={3} maxLength={255} showCount />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={localAction === 'confirm_platform' ? '按平台事实确认' : '取消退款执行单'}
        open={Boolean(localAction)}
        confirmLoading={submitting}
        okText={localAction === 'confirm_platform' ? '确认平台结果' : '确认取消'}
        okButtonProps={{ danger: localAction === 'cancel' }}
        cancelText="返回"
        onCancel={onActionCancel}
        onOk={() => actionForm.submit()}
        forceRender
      >
        <Form
          form={actionForm}
          layout="vertical"
          preserve={false}
          onFinish={onActionSubmit}
        >
          <Alert
            type={localAction === 'cancel' ? 'warning' : 'info'}
            showIcon
            message={
              localAction === 'confirm_platform'
                ? '系统将校验平台事实与售后单的关联、金额、币种和终态后更新本地结果，不会调用平台写接口。'
                : '取消后不能再登记结果；如外部退款已经发生，请不要取消。'
            }
          />
          <Form.Item
            label="操作说明"
            name="reason"
            rules={[
              {
                validator: (_, value) =>
                  localAction !== 'cancel' || String(value || '').trim()
                    ? Promise.resolve()
                    : Promise.reject(new Error('取消时必须填写原因')),
              },
              { max: 128, message: '操作说明不能超过 128 个字符' },
            ]}
          >
            <Input.TextArea rows={3} maxLength={128} showCount />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
