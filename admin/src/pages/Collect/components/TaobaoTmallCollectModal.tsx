import { useLanguage } from '@/i18n/useLanguage';
import { tr } from '@/i18n';
import { Alert, Button, Form, Input, Modal, Space, Typography, message } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { TaobaoTmallLoginPanel } from '@/pages/Collect/components/TaobaoTmallLoginPanel';
import type { ProviderTaobaoTmallAuthStatus } from '@/services/collectAuth';
import { createCollectTask } from '@/services/collectTasks';
import { COLLECT_SUCCESS_SHOP_HINT } from '@/constants/copywriting';
import { mapCollectErrorMessage } from '@/constants/collectErrors';
import { classifyTaobaoTmallUrl, taobaoTmallUrlHint, validateTaobaoTmallUrl } from '@/utils/taobaoTmallUrl';

type Props = {
  open: boolean;
  onClose: () => void;
  onSubmitted?: () => void;
};

export function TaobaoTmallCollectModal({ open, onClose, onSubmitted }: Props) {
  useLanguage();
  const [form] = Form.useForm<{ url: string }>();
  const url = Form.useWatch('url', form);
  const [submitting, setSubmitting] = useState(false);
  const [authStatus, setAuthStatus] = useState<ProviderTaobaoTmallAuthStatus | null>(null);

  useEffect(() => {
    if (!open) {
      form.resetFields();
      setAuthStatus(null);
    }
  }, [open, form]);

  const urlHint = useMemo(() => {
    const u = url?.trim();
    if (!u) return null;
    return taobaoTmallUrlHint(u);
  }, [url]);

  const urlType = url?.trim() ? classifyTaobaoTmallUrl(url.trim()) : null;
  const canSubmit = urlType === 'product_detail';

  const handleSubmit = async () => {
    const vals = await form.validateFields();
    const raw = vals.url?.trim();
    if (!raw || !validateTaobaoTmallUrl(raw)) {
      message.warning(tr("请输入有效的淘宝/天猫商品详情页链接"));
      return;
    }
    setSubmitting(true);
    try {
      await createCollectTask({
        source: 'taobao_tmall',
        url: raw,
        useBrowserProfile: true,
      });
      message.success(COLLECT_SUCCESS_SHOP_HINT, 6);
      onSubmitted?.();
      onClose();
    } catch (e) {
      message.error(tr(mapCollectErrorMessage(e, 'taobao_tmall')));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title={tr("淘宝/天猫采集")}
      open={open}
      onCancel={onClose}
      width={680}
      destroyOnHidden
      footer={
        <Space>
          <Button onClick={onClose}>{tr("取消")}</Button>
          <Button type="primary" loading={submitting} disabled={!canSubmit} onClick={() => void handleSubmit()}>
            {tr("开始采集")}</Button>
        </Space>
      }
    >
      <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
        {tr("部分淘宝/天猫商品需要登录后才能采集。如遇安全验证或滑块，请在采集浏览器中手动完成后再重试。批量采集已开放，建议每批不超过 20 条。")}</Typography.Paragraph>
      <Form form={form} layout="vertical">
        <Form.Item
          label={tr("商品链接")}
          name="url"
          rules={[{ required: true, message: tr("请填写淘宝/天猫商品链接") }]}
        >
          <Input placeholder="https://item.taobao.com/item.htm?id=..." />
        </Form.Item>
      </Form>
      {urlHint ? (
        <Alert
          type={urlType === 'product_detail' ? 'success' : urlType === 'unsupported_taobao' ? 'error' : 'warning'}
          showIcon
          message={urlHint}
          style={{ marginBottom: 12 }}
        />
      ) : null}
      <TaobaoTmallLoginPanel loginUrl={url?.trim()} onAuthChange={setAuthStatus} />
      {authStatus && !authStatus.loggedIn ? (
        <Typography.Text type="warning" style={{ display: 'block', marginTop: 12 }}>
          {tr("当前未检测到登录态，部分商品可能采集失败，建议先完成登录后再采集。")}</Typography.Text>
      ) : null}
    </Modal>
  );
}
