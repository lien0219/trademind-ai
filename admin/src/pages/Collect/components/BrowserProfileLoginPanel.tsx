import { Alert, Button, Form, Input, Modal, Select, Space, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  checkBrowserProfile,
  createBrowserProfile,
  openBrowserProfileLogin,
  queryBrowserProfiles,
  type BrowserProfileRow,
  type ProfileCheckResult,
} from '@/services/collectBrowserProfiles';
import { suggestRuleDomainForHost } from '@/utils/collectRuleMatch';
import { accessStatusLabel } from '@/constants/collectAccess';
import { mapCollectorErrorCodeDetail } from '@/constants/collectErrors';

type Props = {
  url: string;
  domain?: string;
  /** 新建 Profile 时的 provider，默认 custom */
  profileProvider?: string;
  profileId?: string;
  onProfileIdChange?: (id: string | undefined) => void;
  onUseProfileChange?: (use: boolean) => void;
  useBrowserProfile?: boolean;
  onRecheckDone?: (result: ProfileCheckResult) => void;
  /** login_required = 规则测试命中登录页；optional = 用户主动启用登录态 */
  tone?: 'login_required' | 'optional';
};

function hostDomain(url: string): string {
  try {
    const host = new URL(url.trim()).hostname.toLowerCase();
    return suggestRuleDomainForHost(host);
  } catch {
    return '';
  }
}

export function BrowserProfileLoginPanel({
  url,
  domain: domainProp,
  profileProvider = 'custom',
  profileId,
  onProfileIdChange,
  onUseProfileChange,
  useBrowserProfile = false,
  onRecheckDone,
  tone = 'login_required',
}: Props) {
  const domain = domainProp?.trim() || hostDomain(url);
  const [profiles, setProfiles] = useState<BrowserProfileRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm] = Form.useForm<{ name: string }>();
  const [checkResult, setCheckResult] = useState<ProfileCheckResult | null>(null);
  const [busy, setBusy] = useState<'open' | 'check' | null>(null);

  const loadProfiles = useCallback(async () => {
    if (!domain) return;
    setLoading(true);
    try {
      const res = await queryBrowserProfiles({
        page: 1,
        pageSize: 100,
        domain,
        status: 'active',
      });
      setProfiles(res.list ?? []);
    } catch {
      setProfiles([]);
    } finally {
      setLoading(false);
    }
  }, [domain]);

  useEffect(() => {
    void loadProfiles();
  }, [loadProfiles]);

  const options = useMemo(
    () =>
      profiles.map((p) => ({
        label: `${p.name}（${p.domain} · ${p.lastCheckStatus || '未检测'}）`,
        value: p.id,
      })),
    [profiles],
  );

  const handleCreate = async () => {
    const vals = await createForm.validateFields();
    if (!domain) {
      message.warning('无法识别适用网站，请填写有效商品链接');
      return;
    }
    try {
      const res = await createBrowserProfile({
        name: vals.name.trim(),
        domain,
        provider: profileProvider,
      });
      message.success('登录状态已创建');
      setCreateOpen(false);
      createForm.resetFields();
      await loadProfiles();
      onProfileIdChange?.(res.profileId);
      onUseProfileChange?.(true);
    } catch (e) {
      message.error(e instanceof Error ? e.message : '创建失败');
    }
  };

  const needUrl = () => {
    const u = url.trim();
    if (!u) {
      message.warning('请先填写商品链接');
      return '';
    }
    return u;
  };

  const handleOpenLogin = async () => {
    const u = needUrl();
    if (!u || !profileId) return;
    setBusy('open');
    try {
      const res = await openBrowserProfileLogin(profileId, { url: u });
      message.success(res.message || '已打开采集浏览器，请在窗口中完成登录');
    } catch (e) {
      const msg = e instanceof Error ? e.message : '打开失败';
      if (msg.includes('HEADED_BROWSER_REQUIRED')) {
        message.error(mapCollectorErrorCodeDetail('HEADED_BROWSER_REQUIRED'));
      } else {
        message.error(msg);
      }
    } finally {
      setBusy(null);
    }
  };

  const handleCheck = async () => {
    const u = needUrl();
    if (!u || !profileId) return;
    setBusy('check');
    setCheckResult(null);
    try {
      const res = await checkBrowserProfile(profileId, { url: u });
      setCheckResult(res);
      onRecheckDone?.(res);
      if (res.accessStatus === 'public') {
        message.success('登录状态检测通过，可重新测试采集效果');
      } else {
        message.warning(res.message || '仍未通过登录检测');
      }
    } catch (e) {
      message.error(e instanceof Error ? e.message : '检测失败');
    } finally {
      setBusy(null);
    }
  };

  return (
    <Alert
      type={tone === 'optional' ? 'info' : 'warning'}
      showIcon
      style={{ marginTop: tone === 'optional' ? 0 : 12 }}
      message={tone === 'optional' ? '使用已登录的采集浏览器（可选）' : '页面需要登录'}
      description={
        <div>
          <Typography.Paragraph style={{ marginBottom: 8 }}>
            {tone === 'optional'
              ? '若商品页需登录才可查看，请选择或新建一条登录状态，在采集浏览器中手动登录后再生成或测试规则（系统不保存账号密码）。'
              : '当前商品页跳转到了登录页面。请创建或选择登录状态，打开采集浏览器手动登录后再重新测试（系统不保存账号密码）。'}
          </Typography.Paragraph>
          <Space direction="vertical" style={{ width: '100%' }} size="small">
            <Space wrap>
              <Select
                style={{ minWidth: 260 }}
                placeholder={domain ? `选择 ${domain} 的登录状态` : '选择登录状态'}
                loading={loading}
                allowClear
                value={profileId}
                options={options}
                onChange={(v) => {
                  onProfileIdChange?.(v);
                  onUseProfileChange?.(Boolean(v));
                  setCheckResult(null);
                }}
              />
              <Button onClick={() => setCreateOpen(true)}>新建登录状态</Button>
            </Space>
            <Space wrap>
              <Button
                disabled={!profileId}
                loading={busy === 'open'}
                onClick={() => void handleOpenLogin()}
              >
                打开浏览器去登录
              </Button>
              <Button
                disabled={!profileId}
                loading={busy === 'check'}
                onClick={() => void handleCheck()}
              >
                重新检测登录状态
              </Button>
              {useBrowserProfile ? (
                <Typography.Text type="success">将使用所选登录状态进行测试或采集</Typography.Text>
              ) : null}
            </Space>
            {checkResult ? (
              <Typography.Text type="secondary">
                检测结果：{accessStatusLabel(checkResult.accessStatus).text}
                {checkResult.message ? ` — ${checkResult.message}` : ''}
              </Typography.Text>
            ) : null}
          </Space>
          <Modal
            title="新建采集浏览器登录状态"
            open={createOpen}
            onCancel={() => setCreateOpen(false)}
            onOk={() => void handleCreate()}
            destroyOnHidden
          >
            <Form form={createForm} component={false} layout="vertical">
              <Form.Item label="适用网站">{domain || '—'}</Form.Item>
              <Form.Item
                name="name"
                label="名称"
                rules={[{ required: true, message: '必填' }]}
              >
                <Input placeholder="例如：京东-采集登录" />
              </Form.Item>
            </Form>
          </Modal>
        </div>
      }
    />
  );
}
