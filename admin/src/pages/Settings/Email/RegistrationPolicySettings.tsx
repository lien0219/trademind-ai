import { tr } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { ProCard } from '@ant-design/pro-components';
import { Alert, Button, Radio, Space, Typography, message } from 'antd';
import { useEffect, useState } from 'react';
import { getRegistrationPolicy, updateRegistrationPolicy, type RegistrationMode } from '@/services/mailCenter';

export default function RegistrationPolicySettings() {
  useLanguage();
  const [mode, setMode] = useState<RegistrationMode>('closed');
  const [saved, setSaved] = useState<RegistrationMode>('closed');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let active = true;
    getRegistrationPolicy().then((result) => {
      if (active) { setMode(result.mode); setSaved(result.mode); }
    }).catch(() => {
      if (active) message.error(tr('注册策略加载失败'));
    }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, []);

  const save = async () => {
    setSaving(true);
    try {
      const result = await updateRegistrationPolicy(mode);
      setMode(result.mode);
      setSaved(result.mode);
      message.success(tr('注册策略已保存'));
    } catch (e) {
      message.error((e as Error)?.message || tr('注册策略保存失败'));
    } finally { setSaving(false); }
  };
  return (
    <ProCard title={tr('账号注册策略')} variant="outlined" className="tm-system-settings__panel">
      <Alert type="info" showIcon style={{ marginBottom: 16 }}
        message={tr('注册策略由平台管理员统一管理，企业管理员不能修改。普通注册账号不具备 Root 或平台运营权限。')}
      />
      <Radio.Group disabled={loading} value={mode} onChange={(e) => setMode(e.target.value as RegistrationMode)}>
        <Space direction="vertical">
          <Radio value="closed">{tr('关闭自助注册')}</Radio>
          <Radio value="invite_only">{tr('仅邀请注册')}</Radio>
          <Radio value="open">{tr('开放邮箱注册')}</Radio>
        </Space>
      </Radio.Group>
      <Typography.Paragraph type="secondary" style={{ marginTop: 12 }}>
        {tr('默认关闭公开注册。开启后还必须正确配置 SMTP，未完成邮箱验证的用户不能登录。')}
      </Typography.Paragraph>
      <Button type="primary" disabled={loading || mode === saved} loading={saving} onClick={() => void save()}>
        {tr('保存注册策略')}
      </Button>
    </ProCard>
  );
}
