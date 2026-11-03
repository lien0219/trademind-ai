import { useLanguage } from '@/i18n/useLanguage';
import {
  ArrowRightOutlined,
  LockOutlined,
  MailOutlined,
} from '@ant-design/icons';
import { Alert, Form, Input, Checkbox, Button, Tabs, Row, Col, Modal } from 'antd';
import { history, Link, useLocation } from '@umijs/max';
import { useInitialStateModel } from '@/hooks/useInitialStateModel';
import { message } from 'antd';
import { useEffect, useState, useRef } from 'react';
import BrandLogo from '@/components/BrandLogo';
import { setLanguage, tr } from '@/i18n';
import { languageNames, supportedLanguages, type AppLanguage } from '@/i18n/dictionaries';
import { AUTH_TOKEN_KEY, AUTH_SESSION_MODE_KEY } from '@/constants/auth';
import { formatUserErrorMessage } from '@/constants/errorMessages';
import { PASSWORD_HINT, passwordError } from '@/utils/password';
import { changeRequiredPassword, getProfile, isPasswordChangeRequired, login, register, sendEmailCode, getPublicRegistrationPolicy, resetPasswordByEmail, type LoginResult, type RegistrationMode } from '@/services/auth';
import './index.less';

type AuthTabKey = 'login' | 'register';

function authTabFromPathname(pathname: string): AuthTabKey {
  return pathname.replace(/\/+$/, '') === '/user/register' ? 'register' : 'login';
}

type ApiErrorLike = {
  response?: { data?: { message?: string } };
  message?: string;
};

function getAuthErrorMessage(error: unknown, fallback: string) {
  const ax = error as ApiErrorLike;
  const raw = ax?.response?.data?.message || ax?.message;
  return formatUserErrorMessage(raw, fallback);
}

export default function LoginPage() {
  const language = useLanguage();
  const { setInitialState, initialState } = useInitialStateModel();
  const location = useLocation();
  const [loading, setLoading] = useState(false);
  const [passwordChangeAccount, setPasswordChangeAccount] = useState<{ account: string; currentPassword: string } | null>(null);
  const [passwordChangeForm] = Form.useForm();
  const activeTab = authTabFromPathname(location.pathname);

  const [loginForm] = Form.useForm();
  const [registerForm] = Form.useForm();
  const [resetForm] = Form.useForm();
  const [resetVisible, setResetVisible] = useState(false);
  const [registrationMode, setRegistrationMode] = useState<RegistrationMode>('closed');
  const [resetSubmitting, setResetSubmitting] = useState(false);
  const [resetSending, setResetSending] = useState(false);

  useEffect(() => {
    let current = true;
    void getPublicRegistrationPolicy().then((policy) => {
      if (current) setRegistrationMode(policy.mode);
    }).catch(() => {
      if (current) setRegistrationMode('closed');
    });
    return () => { current = false; };
  }, []);

  useEffect(() => {
    if (activeTab !== 'register') return;
    const raw = sessionStorage.getItem('trademind_invitation_pending');
    if (!raw) return;
    try {
      const pending = JSON.parse(raw) as { token?: string; expiresAt?: number };
      if ((pending.expiresAt || 0) > Date.now() && pending.token) {
        registerForm.setFieldValue('invitationToken', pending.token);
      }
    } catch {
      sessionStorage.removeItem('trademind_invitation_pending');
    }
  }, [activeTab, registerForm]);

  useEffect(() => {
    if (activeTab === 'login' && !passwordChangeAccount) loginForm.setFieldValue('password', undefined);
  }, [activeTab, passwordChangeAccount, loginForm]);

  useEffect(() => {
    if (passwordChangeAccount) passwordChangeForm.setFieldsValue({ currentPassword: passwordChangeAccount.currentPassword });
  }, [passwordChangeAccount, passwordChangeForm]);

  const [countdown, setCountdown] = useState(0);
  const countdownTimer = useRef<NodeJS.Timeout | null>(null);

  const loggedIn = Boolean(initialState?.currentUser);
  useEffect(() => {
    if (!loggedIn) return;
    const q = new URLSearchParams(history.location.search);
    const user = initialState?.currentUser;
    if (user?.platformOperator === true && user.tenantId === 0 && user.isRoot !== true) {
      const redirect = q.get('redirect');
      history.replace(redirect?.startsWith('/tenant/invitations/accept')
        ? redirect : '/platform/tenants');
      return;
    }
    history.replace(q.get('redirect') || '/dashboard');
  }, [loggedIn]);

  useEffect(() => {
    return () => {
      if (countdownTimer.current) clearInterval(countdownTimer.current);
    };
  }, []);

  const changeAuthTab = (key: string) => {
    const nextTab: AuthTabKey = key === 'register' ? 'register' : 'login';
    history.replace(
      `${nextTab === 'register' ? '/user/register' : '/user/login'}${location.search}`,
    );
  };

  const completeLogin = async (data: LoginResult) => {
    localStorage.setItem(AUTH_TOKEN_KEY, data.token);
    if (data.sessionMode) {
      localStorage.setItem(AUTH_SESSION_MODE_KEY, data.sessionMode);
    } else {
      localStorage.removeItem(AUTH_SESSION_MODE_KEY);
    }
    try {
      const currentUser = await getProfile();
      await setInitialState((s) => ({ ...s, currentUser }));
    } catch (error) {
      localStorage.removeItem(AUTH_TOKEN_KEY);
      localStorage.removeItem(AUTH_SESSION_MODE_KEY);
      throw error;
    }
  };

  const onLogin = async (values: any) => {
    setLoading(true);
    try {
      const data = await login(values.account as string, values.password as string);
      await completeLogin(data);
      message.success(tr('登录成功'));
    } catch (e: unknown) {
      if (isPasswordChangeRequired(e)) {
        setPasswordChangeAccount({ account: values.account as string, currentPassword: values.password as string });
        message.warning(tr("临时密码需要先修改后才能登录"));
      } else {
        message.error(getAuthErrorMessage(e, tr("登录失败")));
      }
    } finally {
      setLoading(false);
    }
  };

  const onRegister = async (values: any) => {
    setLoading(true);
    try {
      await register({
        email: values.email,
        code: values.code,
        password: values.password,
        confirmPassword: values.confirmPassword,
        invitationToken: values.invitationToken,
        locale: language,
      });
      sessionStorage.removeItem('trademind_invitation_pending');
      message.success(tr('注册成功，请登录'));
      changeAuthTab('login');
    } catch (e: unknown) {
      message.error(getAuthErrorMessage(e, tr("注册失败")));
    } finally {
      setLoading(false);
    }
  };

  const handleSendCode = async () => {
    try {
      await registerForm.validateFields(['email']);
    } catch {
      return;
    }
    const email = registerForm.getFieldValue('email');
    try {
      await sendEmailCode(email, 'register', language, registerForm.getFieldValue('invitationToken'));
      message.success(tr('验证码已发送'));
      setCountdown(60);
      countdownTimer.current = setInterval(() => {
        setCountdown((c) => {
          if (c <= 1) {
            if (countdownTimer.current) clearInterval(countdownTimer.current);
            return 0;
          }
          return c - 1;
        });
      }, 1000);
    } catch (e: unknown) {
      message.error(getAuthErrorMessage(e, tr("发送失败")));
    }
  };

  const sendPasswordResetCode = async () => {
    const email = resetForm.getFieldValue('email');
    if (!email) {
      message.warning(tr('请输入邮箱'));
      return;
    }
    setResetSending(true);
    try {
      await resetForm.validateFields(['email']);
      await sendEmailCode(email, 'password_reset', language);
      message.success(tr('如邮箱已注册，验证码将发送至该邮箱'));
    } catch (error) {
      if (!(error as { errorFields?: unknown })?.errorFields) {
        message.error(getAuthErrorMessage(error, tr('发送失败')));
      }
    } finally { setResetSending(false); }
  };

  const performPasswordReset = async (values: any) => {
    setResetSubmitting(true);
    try {
      await resetPasswordByEmail({
        email: values.email, code: values.code,
        newPassword: values.newPassword, confirmPassword: values.confirmPassword,
      });
      message.success(tr('密码重置成功，请重新登录'));
      resetForm.resetFields();
      setResetVisible(false);
    } catch (error) {
      message.error(getAuthErrorMessage(error, tr('密码重置失败')));
    } finally { setResetSubmitting(false); }
  };

  return (
    <div className="login-shell">
      <div style={{position:'absolute',top:16,right:24,zIndex:20}}>
        <select aria-label={tr('语言')} value={language} onChange={e=>{
          const next=e.target.value as AppLanguage;
          setLanguage(next);
        }} style={{padding:'7px 10px',borderRadius:8,border:'1px solid #b0b0b0',background:'var(--ant-color-bg-container, #fff)',color:'var(--ant-color-text, #222)'}}>
          {supportedLanguages.map(locale=><option value={locale} key={locale}>{languageNames[locale]}</option>)}
        </select>
      </div>
      <div className="login-container">

        <div className="login-right">
          <div className="login-right-inner">
            <div className="auth-brand">
              <BrandLogo height={28} />
              <div>
                <h1 className="brand-text">{tr("贸灵 TradeMind")}</h1>
              </div>
            </div>

            <div className={`auth-card auth-card-${activeTab}`}>
              <Tabs
                className="auth-tabs"
                activeKey={activeTab}
                centered
                onChange={changeAuthTab}
                items={[
                  { key: 'login', label: tr('登录') },
                  { key: 'register', label: tr('注册') },
                ]}
              />

            {activeTab === 'login' && passwordChangeAccount ? (
              <Form
                key={passwordChangeAccount.account}
                form={passwordChangeForm}
                initialValues={{ currentPassword: passwordChangeAccount.currentPassword }}
                preserve={false}
                layout="vertical"
                requiredMark={false}
                onFinish={async (values) => {
                  setLoading(true);
                  try {
                    await changeRequiredPassword({
                      account: passwordChangeAccount.account,
                      currentPassword: values.currentPassword,
                      newPassword: values.newPassword,
                    });
                    passwordChangeForm.resetFields();
                    setPasswordChangeAccount(null);
                    message.success(tr("密码修改成功，请使用新密码登录"));
                  } catch (error) {
                    message.error(getAuthErrorMessage(error, tr("密码修改失败")));
                  } finally {
                    setLoading(false);
                  }
                }}
              >
                <p>{tr("你的账号使用初始或重置后的临时密码，请先设置新的密码。")}</p>
                <Form.Item name="currentPassword" label={tr("临时密码")} rules={[{ required: true }]}>
                  <Input.Password autoComplete="current-password" />
                </Form.Item>
                <Form.Item name="newPassword" label={tr("新密码")} extra={tr(PASSWORD_HINT)} rules={[
                  { required: true, message: tr("请输入新密码") },
                  { validator: (_, value) => {
                    const error = passwordError(value || '');
                    return error ? Promise.reject(new Error(tr(error))) : Promise.resolve();
                  } },
                ]}>
                  <Input.Password autoComplete="new-password" />
                </Form.Item>
                <Form.Item name="confirmPassword" label={tr("确认新密码")} dependencies={['newPassword']} rules={[
                  { required: true },
                  ({ getFieldValue }) => ({ validator(_, value) {
                    return value === getFieldValue('newPassword')
                      ? Promise.resolve()
                      : Promise.reject(new Error(tr("两次输入的密码不一致")));
                  } }),
                ]}>
                  <Input.Password autoComplete="new-password" />
                </Form.Item>
                <div className="password-change-actions">
                  <Button type="primary" htmlType="submit" className="submit-btn" loading={loading}>
                    {tr("修改密码")}</Button>
                  <Button
                    type="text"
                    className="password-change-back"
                    onClick={() => { setPasswordChangeAccount(null); passwordChangeForm.resetFields(); }}
                  >
                    {tr("返回登录")}</Button>
                </div>
              </Form>
            ) : activeTab === 'login' ? (
              <Form
                form={loginForm}
                layout="vertical"
                onFinish={onLogin}
                requiredMark={false}
                autoComplete="off"
              >
                <Form.Item
                  name="account"
                  label={tr("邮箱 / 手机号")}
                  rules={[{ required: true, message: tr("请输入邮箱或手机号") }]}
                  validateTrigger="onBlur"
                >
                  <Input
                    placeholder={tr("请输入邮箱或手机号")}
                    prefix={<MailOutlined />}
                    autoComplete="off"
                    data-lpignore="true"
                    data-1p-ignore="true"
                  />
                </Form.Item>

                <Form.Item
                  name="password"
                  label={tr("密码")}
                  rules={[{ required: true, message: tr("请输入登录密码") }]}
                  validateTrigger="onBlur"
                >
                  <Input.Password
                    placeholder={tr("请输入登录密码")}
                    prefix={<LockOutlined />}
                    autoComplete="new-password"
                    data-lpignore="true"
                    data-1p-ignore="true"
                  />
                </Form.Item>

                <div className="form-actions">
                  <Form.Item name="remember" valuePropName="checked" noStyle initialValue={true}>
                    <Checkbox>{tr("记住我")}</Checkbox>
                  </Form.Item>
                  <a href="#" className="forgot-link" onClick={(e) => { e.preventDefault(); setResetVisible(true); }}>
                    {tr("忘记密码？")}</a>
                </div>

                <Form.Item>
                  <Button
                    type="primary"
                    htmlType="submit"
                    className="submit-btn"
                    loading={loading}
                    disabled={loading}
                  >
                    {tr("登录工作台")}<ArrowRightOutlined />
                  </Button>
                </Form.Item>

                <div className="register-link">
                  {tr("还没有账号？")}<Link
                    to={`/user/register${location.search}`}
                  >
                    {tr("立即注册")}</Link>
                </div>
              </Form>
            ) : (
              <Form
                form={registerForm}
                layout="vertical"
                onFinish={onRegister}
                requiredMark={false}
                disabled={registrationMode === 'closed'}
              >
                {registrationMode === 'closed' ? <Alert type="info" showIcon message={tr("请联系管理员创建账号，当前不开放自助注册")} /> : null}
                {registrationMode === 'invite_only' ? <Alert type="info" showIcon message={tr('当前仅允许受邀用户注册，请填写管理员提供的邀请令牌')} /> : null}
                {registrationMode === 'invite_only' ? (
                  <Form.Item name="invitationToken" label={tr('企业邀请令牌')} rules={[{ required: true, message: tr('请输入邀请令牌') }]}>
                    <Input.Password autoComplete="off" />
                  </Form.Item>
                ) : null}
                <Form.Item
                  name="email"
                  label={tr("邮箱")}
                  rules={[
                    { required: true, message: tr("请输入邮箱") },
                    { type: 'email', message: tr("请输入有效的邮箱地址") },
                  ]}
                  validateTrigger="onBlur"
                >
                  <Input placeholder={tr("请输入邮箱")} prefix={<MailOutlined />} autoComplete="email" />
                </Form.Item>

                <Form.Item label={tr("邮箱验证码")} required>
                  <Row gutter={8}>
                    <Col span={15}>
                      <Form.Item
                        name="code"
                        noStyle
                        rules={[{ required: true, message: tr("请输入验证码") }]}
                      >
                        <Input placeholder={tr("6位验证码")} />
                      </Form.Item>
                    </Col>
                    <Col span={9}>
                      <Button
                        className="code-btn"
                        onClick={handleSendCode}
                        disabled={registrationMode === 'closed' || countdown > 0}
                      >
                        {countdown > 0 ? tr('{seconds} 秒后重发').replace('{seconds}', String(countdown)) : tr("获取验证码")}
                      </Button>
                    </Col>
                  </Row>
                </Form.Item>

                <Form.Item
                  name="password"
                  label={tr("密码")}
                  rules={[
                    { required: true, message: tr("请输入密码") },
                    { min: 12, message: tr('密码至少12位') },
                  ]}
                  validateTrigger="onBlur"
                >
                  <Input.Password
                    placeholder={tr('请输入至少12位密码')}
                    prefix={<LockOutlined />}
                    autoComplete="new-password"
                  />
                </Form.Item>

                <Form.Item
                  name="confirmPassword"
                  label={tr("确认密码")}
                  dependencies={['password']}
                  validateTrigger="onBlur"
                  rules={[
                    { required: true, message: tr("请确认密码") },
                    ({ getFieldValue }) => ({
                      validator(_, value) {
                        if (!value || getFieldValue('password') === value) {
                          return Promise.resolve();
                        }
                        return Promise.reject(new Error(tr("两次输入的密码不一致!")));
                      },
                    }),
                  ]}
                >
                  <Input.Password
                    placeholder={tr("请再次输入密码")}
                    prefix={<LockOutlined />}
                    autoComplete="new-password"
                  />
                </Form.Item>

                <Form.Item>
                  <Button
                    type="primary"
                    htmlType="submit"
                    className="submit-btn"
                    loading={loading}
                    disabled={registrationMode === 'closed'}
                  >
                    {tr("注册")}<ArrowRightOutlined />
                  </Button>
                </Form.Item>

                <div className="register-link">
                  {tr("已有账号？")}<Link
                    to={`/user/login${location.search}`}
                  >
                    {tr("去登录")}</Link>
                </div>
              </Form>
            )}

            <Modal
              title={tr('邮箱找回密码')}
              open={resetVisible}
              onCancel={() => setResetVisible(false)}
              onOk={() => void resetForm.submit()}
              confirmLoading={resetSubmitting}
              okText={tr('重置密码')}
              destroyOnHidden
            >
              <Form form={resetForm} layout="vertical" onFinish={(values) => void performPasswordReset(values)}>
                <Form.Item name="email" label={tr('邮箱')} rules={[{ required: true, type: 'email' }]}>
                  <Input autoComplete="email" />
                </Form.Item>
                <Form.Item label={tr('邮箱验证码')} required>
                  <Row gutter={8}>
                    <Col span={15}><Form.Item noStyle name="code" rules={[{ required: true, len: 6 }]}><Input maxLength={6} /></Form.Item></Col>
                    <Col span={9}><Button block loading={resetSending} onClick={() => void sendPasswordResetCode()}>{tr('获取验证码')}</Button></Col>
                  </Row>
                </Form.Item>
                <Form.Item name="newPassword" label={tr('新密码')} rules={[{ required: true, min: 12, message: tr('密码至少12位') }]}>
                  <Input.Password autoComplete="new-password" />
                </Form.Item>
                <Form.Item name="confirmPassword" dependencies={['newPassword']} label={tr('确认新密码')}
                  rules={[{ required: true }, ({ getFieldValue }) => ({
                    validator(_, value) {
                      return value && value === getFieldValue('newPassword') ? Promise.resolve() : Promise.reject(new Error(tr('两次输入的密码不一致!')));
                    },
                  })]}>
                  <Input.Password autoComplete="new-password" />
                </Form.Item>
              </Form>
            </Modal>
            <div className="agreement">
              {activeTab === 'login' ? tr("登录") : tr("注册")}{tr("即表示你已阅读并同意用户协议与隐私政策")}</div>
          </div>
        </div>
      </div>
    </div>
    </div>
  );
}
