import { useLanguage } from '@/i18n/useLanguage';
import {
  ArrowRightOutlined,
  CheckCircleOutlined,
  CloudDownloadOutlined,
  CloudUploadOutlined,
  DashboardOutlined,
  FileImageOutlined,
  InboxOutlined,
  LockOutlined,
  MailOutlined,
  RobotOutlined,
  SafetyCertificateOutlined,
} from '@ant-design/icons';
import { Alert, Form, Input, Checkbox, Button, Tabs, Row, Col } from 'antd';
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
import { changeRequiredPassword, getProfile, isPasswordChangeRequired, login, register, sendEmailCode, type LoginResult } from '@/services/auth';
import './index.less';

const FEATURE_TAGS = [
  { icon: <CloudDownloadOutlined />, label: '多平台商品采集', className: 'tag-blue' },
  { icon: <RobotOutlined />, label: 'AI 商品运营', className: 'tag-violet' },
  { icon: <FileImageOutlined />, label: '图片智能处理', className: 'tag-teal' },
  { icon: <CloudUploadOutlined />, label: '多平台刊登', className: 'tag-indigo' },
  { icon: <InboxOutlined />, label: '库存同步', className: 'tag-green' },
  { icon: <DashboardOutlined />, label: '运营看板', className: 'tag-amber' },
] as const;

const PLATFORM_ITEMS = ['1688', 'Shopee', 'Lazada', 'Temu'];

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
      const data = await register({
        email: values.email,
        code: values.code,
        password: values.password,
        confirmPassword: values.confirmPassword,
      });
      await completeLogin(data);
      message.success(tr('注册并登录成功'));
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
      await sendEmailCode(email, 'register');
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

  const [heroBeforeAI, heroAfterAI] = tr('用 {ai} 串联商品、刊登与库存增长').split('{ai}');
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
        <div className="login-left">
          <div className="login-left-decor" aria-hidden="true">
            <div className="decor-line decor-line-1" />
            <div className="decor-line decor-line-2" />
            <div className="decor-card decor-card-1" />
            <div className="decor-card decor-card-2" />
            <div className="decor-card decor-card-3" />
          </div>

          <div className="login-left-content">
            <div className="brand">
              <BrandLogo height={32} />
              <div>
                <div className="brand-text">{tr("贸灵 TradeMind")}</div>
                <div className="brand-sub">AI-Powered Cross-Border ERP</div>
              </div>
            </div>

            <div className="slogan">
              <div className="eyebrow">
                <SafetyCertificateOutlined />
                <span>{tr("面向跨境团队的 AI 运营工作台")}</span>
              </div>
              <h1>
                {heroBeforeAI}<span className="highlight">AI</span>{heroAfterAI}
              </h1>
              <p>
                {tr('从商品采集、图片处理、AI 优化到多平台刊登，TradeMind 把高频运营动作收进一个更轻、更快的工作台。')}
              </p>
            </div>

            <div className="features">
              {FEATURE_TAGS.map((tag, index) => (
                <div
                  key={tag.label}
                  className={`feature-tag ${tag.className}`}
                  style={{ animationDelay: `${index * 80 + 180}ms` }}
                >
                  {tag.icon} {tr(tag.label)}
                </div>
              ))}
            </div>

            <div className="hero-board" aria-hidden="true">
              <div className="hero-board__top">
                <div>
                  <span className="hero-board__label">Today GMV</span>
                  <strong>¥128,430</strong>
                </div>
                <span className="hero-board__badge">+18.6%</span>
              </div>
              <div className="hero-board__chart">
                <span className="chart-bar chart-bar-1" />
                <span className="chart-bar chart-bar-2" />
                <span className="chart-bar chart-bar-3" />
                <span className="chart-bar chart-bar-4" />
                <span className="chart-bar chart-bar-5" />
              </div>
              <div className="hero-board__flow">
                {PLATFORM_ITEMS.map((platform) => (
                  <span key={platform}>{platform}</span>
                ))}
              </div>
              <div className="hero-board__task">
                <CheckCircleOutlined />
                <span>{tr("AI 已生成 36 条商品优化建议")}</span>
              </div>
            </div>
          </div>
        </div>

        <div className="login-right">
          <div className="login-right-inner">
            <div className="mobile-brand">
              <BrandLogo height={28} />
              <div>
                <div className="brand-text">{tr("贸灵 TradeMind")}</div>
                <div className="brand-sub">AI-Powered Cross-Border ERP</div>
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

              <div className="welcome-text" key={`welcome-${activeTab}`}>
                <h2>{activeTab === 'login' ? tr('欢迎回来') : tr('注册账号')}</h2>
                <p>
                  {activeTab === 'login'
                    ? tr('登录你的 TradeMind 工作台')
                    : tr('开启你的 AI 跨境之旅')}
                </p>
              </div>

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
                  <a href="#" className="forgot-link" onClick={(e) => e.preventDefault()}>
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
                disabled
              >
                <Alert type="info" showIcon message={tr("请联系管理员创建账号，当前不开放自助注册")} />
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
                        disabled
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
                    { min: 6, message: tr("密码至少6位") },
                  ]}
                  validateTrigger="onBlur"
                >
                  <Input.Password
                    placeholder={tr("请输入至少6位密码")}
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
                    disabled
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

            <div className="agreement">
              {activeTab === 'login' ? tr("登录") : tr("注册")}{tr("即表示你已阅读并同意用户协议与隐私政策")}</div>
          </div>
        </div>
      </div>
    </div>
    </div>
  );
}
