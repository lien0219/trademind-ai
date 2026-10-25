import { CheckCircleOutlined, LoginOutlined } from '@ant-design/icons';
import { Alert, Button, Form, Input, Result, Space, message } from 'antd';
import { history } from '@umijs/max';
import { useEffect, useRef, useState } from 'react';
import { SectionCard, TmPageContainer } from '@/components/ui';
import { AUTH_SESSION_MODE_KEY, AUTH_TOKEN_KEY } from '@/constants/auth';
import { acceptTenantInvitation } from '@/services/tenants';

const INVITE_SESSION_KEY = 'trademind_invitation_pending';

function invitationTokenFromLocation() {
  if (typeof window === 'undefined') return '';
  const hash = window.location.hash.replace(/^#/, '');
  const hashParams = new URLSearchParams(hash);
  return hashParams.get('token') || hash || '';
}

export default function AcceptInvitationPage() {
  const [form] = Form.useForm<{ token?: string }>();
  const [submitting, setSubmitting] = useState(false);
  const [success, setSuccess] = useState(false);
  const [hasSession, setHasSession] = useState(false);
  const submittedRef = useRef(false);

  useEffect(() => {
    const token = invitationTokenFromLocation();
    const stored = sessionStorage.getItem(INVITE_SESSION_KEY);
    let restoredToken = '';
    if (stored) {
      try {
        const pending = JSON.parse(stored) as { token?: string; expiresAt?: number };
        if ((pending.expiresAt || 0) > Date.now()) restoredToken = pending.token || '';
        else sessionStorage.removeItem(INVITE_SESSION_KEY);
      } catch {
        sessionStorage.removeItem(INVITE_SESSION_KEY);
      }
    }
    const value = token || restoredToken;
    setHasSession(Boolean(localStorage.getItem(AUTH_TOKEN_KEY)));
    if (value) {
      if (!localStorage.getItem(AUTH_TOKEN_KEY)) {
        sessionStorage.setItem(INVITE_SESSION_KEY, JSON.stringify({ token: value, expiresAt: Date.now() + 60 * 60 * 1000 }));
      }
      form.setFieldValue('token', value);
      // Remove the secret from the address bar before any user interaction.
      if (window.location.hash) window.history.replaceState(null, document.title, '/tenant/invitations/accept');
    }
  }, [form]);

  const submit = async ({ token }: { token?: string }) => {
    if (!token?.trim()) return;
    if (submittedRef.current) return;
    submittedRef.current = true;
    setSubmitting(true);
    try {
      await acceptTenantInvitation(token.trim());
      localStorage.removeItem(AUTH_TOKEN_KEY);
      localStorage.removeItem(AUTH_SESSION_MODE_KEY);
      setSuccess(true);
      form.resetFields();
      sessionStorage.removeItem(INVITE_SESSION_KEY);
      message.success('已接受企业邀请，请重新登录');
    } catch (error) {
      message.error((error as Error)?.message || '接受邀请失败，请确认令牌仍在有效期内');
    } finally {
      submittedRef.current = false;
      setSubmitting(false);
    }
  };

  return (
    <TmPageContainer title="接受企业邀请" subTitle="使用企业管理员提供的一次性邀请令牌加入企业。">
      <SectionCard>
        {success ? (
          <Result
            status="success"
            icon={<CheckCircleOutlined />}
            title="邀请已接受"
            subTitle="当前会话已失效，请重新登录后手动切换到受邀企业。"
            extra={<Button type="primary" icon={<LoginOutlined />} onClick={() => history.push('/user/login')}>重新登录</Button>}
          />
        ) : (
          <Form
            form={form}
            layout="vertical"
            onFinish={submit}
            onValuesChange={(_, values: { token?: string }) => {
              if (!hasSession && values.token?.trim()) {
                sessionStorage.setItem(INVITE_SESSION_KEY, JSON.stringify({ token: values.token.trim(), expiresAt: Date.now() + 60 * 60 * 1000 }));
              }
            }}
            style={{ maxWidth: 560 }}
          >
            <Alert type="info" showIcon message="仅已登录且邮箱与邀请绑定的账号可以接受邀请。" description="邀请令牌不会用于匿名注册，也不会在页面地址中长期保留。" style={{ marginBottom: 20 }} />
            <Form.Item name="token" label="一次性邀请令牌" rules={[{ required: true, message: '请输入邀请令牌' }]}>
              <Input.Password autoComplete="off" placeholder="粘贴企业管理员提供的邀请令牌" />
            </Form.Item>
            {hasSession ? (
              <Button type="primary" htmlType="submit" loading={submitting} disabled={submitting} icon={<LoginOutlined />}>接受邀请</Button>
            ) : (
              <Space direction="vertical" size={8}>
                <Alert type="warning" showIcon message="请先登录受邀邮箱对应的账号。" />
                <Button type="primary" icon={<LoginOutlined />} onClick={() => history.push('/user/login?redirect=%2Ftenant%2Finvitations%2Faccept')}>登录并继续</Button>
              </Space>
            )}
          </Form>
        )}
      </SectionCard>
    </TmPageContainer>
  );
}
