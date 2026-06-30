import { Alert, Button, Form, Input, Modal, Space, Switch, Typography, message } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { PinduoduoLoginPanel } from '@/pages/Collect/components/PinduoduoLoginPanel';
import type { ProviderPinduoduoAuthStatus } from '@/services/collectAuth';
import { createCollectTask } from '@/services/collectTasks';
import { COLLECT_SUCCESS_SHOP_HINT } from '@/constants/copywriting';
import { mapCollectErrorMessage } from '@/constants/collectErrors';
import {
  classifyPinduoduoUrl,
  hasPinduoduoLoginContext,
  pinduoduoUrlHint,
} from '@/utils/pinduoduoUrl';

type Props = {
  open: boolean;
  onClose: () => void;
  onSubmitted?: () => void;
};

export function PinduoduoCollectModal({ open, onClose, onSubmitted }: Props) {
  const [form] = Form.useForm<{ url: string }>();
  const url = Form.useWatch('url', form);
  const [submitting, setSubmitting] = useState(false);
  const [useBrowserProfile, setUseBrowserProfile] = useState(false);
  const [authStatus, setAuthStatus] = useState<ProviderPinduoduoAuthStatus | null>(null);

  useEffect(() => {
    if (!open) {
      form.resetFields();
      setUseBrowserProfile(false);
      setAuthStatus(null);
    }
  }, [open, form]);

  const urlHint = useMemo(() => {
    const u = url?.trim();
    if (!u) return null;
    return pinduoduoUrlHint(u);
  }, [url]);

  const urlType = useMemo(() => classifyPinduoduoUrl(url?.trim() ?? ''), [url]);
  const canSubmit =
    urlType === 'goods_detail' || urlType === 'wholesale_detail';

  useEffect(() => {
    if (urlType === 'wholesale_detail') {
      setUseBrowserProfile(true);
    }
  }, [urlType]);

  const loginNotReady =
    (useBrowserProfile || urlType === 'wholesale_detail') &&
    authStatus != null &&
    !authStatus.loggedIn;

  const handleSubmit = async () => {
    const vals = await form.validateFields();
    const raw = vals.url?.trim();
    if (!raw) {
      message.warning('请填写商品链接');
      return;
    }
    if (!canSubmit) {
      message.warning(pinduoduoUrlHint(raw) ?? '请输入拼多多商品详情页链接');
      return;
    }
    if (urlType === 'wholesale_detail' && authStatus && !authStatus.loggedIn) {
      message.warning('批发页通常需要登录，请先打开拼多多采集浏览器登录后再采集');
    }
    setSubmitting(true);
    try {
      await createCollectTask({
        source: 'pinduoduo',
        url: raw,
        useBrowserProfile: useBrowserProfile || urlType === 'wholesale_detail',
      });
      message.success(COLLECT_SUCCESS_SHOP_HINT, 6);
      onSubmitted?.();
      onClose();
    } catch (e) {
      message.error(mapCollectErrorMessage(e, 'pinduoduo'));
    } finally {
      setSubmitting(false);
    }
  };

  const loginUrl = hasPinduoduoLoginContext(url?.trim()) ? url!.trim() : undefined;

  return (
    <Modal
      title="拼多多采集"
      open={open}
      onCancel={onClose}
      width={680}
      destroyOnHidden
      footer={
        <Space>
          <Button onClick={onClose}>取消</Button>
          <Button type="primary" loading={submitting} disabled={!canSubmit} onClick={() => void handleSubmit()}>
            开始采集
          </Button>
        </Space>
      }
    >
      <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
        第一版优先支持普通商品详情页。拼多多批发页（pifa.pinduoduo.com）通常需要登录后才能采集。
      </Typography.Paragraph>
      <Form form={form} layout="vertical">
        <Form.Item
          label="商品链接"
          name="url"
          rules={[{ required: true, message: '请填写拼多多商品链接' }]}
        >
          <Input placeholder="https://mobile.yangkeduo.com/goods.html?goods_id=..." />
        </Form.Item>
      </Form>
      {urlHint ? (
        <Alert
          type={urlType === 'goods_detail' ? 'success' : urlType === 'wholesale_detail' ? 'warning' : 'info'}
          showIcon
          message={urlHint}
          style={{ marginBottom: 12 }}
        />
      ) : null}
      <Space direction="vertical" style={{ width: '100%' }} size="middle">
        <PinduoduoLoginPanel loginUrl={loginUrl} onAuthChange={setAuthStatus} />
        <Space>
          <Switch
            checked={useBrowserProfile || urlType === 'wholesale_detail'}
            disabled={urlType === 'wholesale_detail'}
            onChange={setUseBrowserProfile}
          />
          <Typography.Text>使用已登录的采集浏览器</Typography.Text>
        </Space>
        {loginNotReady ? (
          <Typography.Text type="warning">
            当前未检测到登录态，任务可能因需要登录而失败，建议先完成登录后再采集。
          </Typography.Text>
        ) : null}
      </Space>
    </Modal>
  );
}
