import { Alert, Button, Form, Input, Modal, Space, Switch, Typography, message } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { BrowserProfileLoginPanel } from '@/pages/Collect/components/BrowserProfileLoginPanel';
import { createCollectTask } from '@/services/collectTasks';
import { mapCollectErrorMessage } from '@/constants/collectErrors';
import { classifyPinduoduoUrl, pinduoduoProfileDomain, pinduoduoUrlHint } from '@/utils/pinduoduoUrl';

type Props = {
  open: boolean;
  onClose: () => void;
  onSubmitted?: () => void;
};

export function PinduoduoCollectModal({ open, onClose, onSubmitted }: Props) {
  const [form] = Form.useForm<{ url: string }>();
  const url = Form.useWatch('url', form);
  const [submitting, setSubmitting] = useState(false);
  const [profileId, setProfileId] = useState<string | undefined>();
  const [useBrowserProfile, setUseBrowserProfile] = useState(false);

  useEffect(() => {
    if (!open) {
      form.resetFields();
      setProfileId(undefined);
      setUseBrowserProfile(false);
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
    setSubmitting(true);
    try {
      await createCollectTask({
        source: 'pinduoduo',
        url: raw,
        profileId: useBrowserProfile ? profileId : undefined,
        useBrowserProfile: useBrowserProfile && Boolean(profileId),
      });
      message.success('采集任务已提交，正在后台处理');
      onSubmitted?.();
      onClose();
    } catch (e) {
      message.error(mapCollectErrorMessage(e, 'pinduoduo'));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="拼多多采集"
      open={open}
      onCancel={onClose}
      width={640}
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
        第一版优先支持普通商品详情页（mobile.yangkeduo.com / pinduoduo.com）。拼多多批发页（pifa.pinduoduo.com）可能需要登录态。
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
        <Space>
          <Switch checked={useBrowserProfile} onChange={setUseBrowserProfile} />
          <Typography.Text>使用已登录的采集浏览器</Typography.Text>
        </Space>
        {useBrowserProfile ? (
          <BrowserProfileLoginPanel
            url={url?.trim() ?? ''}
            domain={pinduoduoProfileDomain()}
            profileProvider="pinduoduo"
            profileId={profileId}
            useBrowserProfile={useBrowserProfile}
            tone="optional"
            onProfileIdChange={setProfileId}
            onUseProfileChange={setUseBrowserProfile}
          />
        ) : null}
      </Space>
    </Modal>
  );
}
