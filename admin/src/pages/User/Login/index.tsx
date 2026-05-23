import { Form, Input, Checkbox, Button, Tabs, Row, Col } from 'antd';
import { history, useModel } from '@umijs/max';
import { message } from 'antd';
import { useEffect, useState, useRef } from 'react';
import BrandLogo from '@/components/BrandLogo';
import { AUTH_TOKEN_KEY } from '@/constants/auth';
import { login, register, sendEmailCode } from '@/services/auth';
import './index.less';

const CollectIcon = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <rect x="2" y="4" width="20" height="16" rx="2" stroke="currentColor" strokeWidth="2" />
    <path d="M2 10H22" stroke="currentColor" strokeWidth="2" />
    <path d="M6 14H10" stroke="currentColor" strokeWidth="2" />
  </svg>
);

const AiIcon = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path
      d="M12 2L14.4 9.6L22 12L14.4 14.4L12 22L9.6 14.4L2 12L9.6 9.6L12 2Z"
      fill="currentColor"
    />
  </svg>
);

const ImageIcon = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <rect x="3" y="3" width="18" height="18" rx="2" stroke="currentColor" strokeWidth="2" />
    <circle cx="8.5" cy="8.5" r="1.5" fill="currentColor" />
    <path d="M21 15L16 10L5 21" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
  </svg>
);

const PublishIcon = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path
      d="M12 3L20 7V17L12 21L4 17V7L12 3Z"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinejoin="round"
    />
    <path d="M12 12L20 7" stroke="currentColor" strokeWidth="2" />
    <path d="M12 12V21" stroke="currentColor" strokeWidth="2" />
    <path d="M12 12L4 7" stroke="currentColor" strokeWidth="2" />
  </svg>
);

const InventoryIcon = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path
      d="M21 8L12 13L3 8"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
    <path
      d="M21 16L12 21L3 16"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
    <path
      d="M12 3L21 8L12 13L3 8L12 3Z"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </svg>
);

const DashboardIcon = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <rect x="3" y="3" width="8" height="8" rx="1.5" stroke="currentColor" strokeWidth="2" />
    <rect x="13" y="3" width="8" height="5" rx="1.5" stroke="currentColor" strokeWidth="2" />
    <rect x="13" y="10" width="8" height="11" rx="1.5" stroke="currentColor" strokeWidth="2" />
    <rect x="3" y="13" width="8" height="8" rx="1.5" stroke="currentColor" strokeWidth="2" />
  </svg>
);

const MailIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path
      d="M4 7.00005L10.2 11.65C11.2667 12.45 12.7333 12.45 13.8 11.65L20 7"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
    <rect
      x="3"
      y="5"
      width="18"
      height="14"
      rx="2"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
    />
  </svg>
);

const LockIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <rect
      x="5"
      y="11"
      width="14"
      height="10"
      rx="2"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
    <path
      d="M8 11V7C8 4.79086 9.79086 3 12 3C14.2091 3 16 4.79086 16 7V11"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </svg>
);

const FEATURE_TAGS = [
  { icon: <CollectIcon />, label: '多平台商品采集', className: 'tag-blue' },
  { icon: <AiIcon />, label: 'AI 商品运营', className: 'tag-violet' },
  { icon: <ImageIcon />, label: '图片智能处理', className: 'tag-teal' },
  { icon: <PublishIcon />, label: '多平台刊登', className: 'tag-indigo' },
  { icon: <InventoryIcon />, label: '库存同步', className: 'tag-green' },
  { icon: <DashboardIcon />, label: '运营看板', className: 'tag-slate' },
] as const;

export default function LoginPage() {
  const { setInitialState, initialState } = useModel('@@initialState');
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('login');

  const [loginForm] = Form.useForm();
  const [registerForm] = Form.useForm();

  const [countdown, setCountdown] = useState(0);
  const countdownTimer = useRef<NodeJS.Timeout | null>(null);

  const loggedIn = Boolean(initialState?.currentUser);
  useEffect(() => {
    if (!loggedIn) return;
    const q = new URLSearchParams(history.location.search);
    history.replace(q.get('redirect') || '/dashboard');
  }, [loggedIn]);

  useEffect(() => {
    return () => {
      if (countdownTimer.current) clearInterval(countdownTimer.current);
    };
  }, []);

  const onLogin = async (values: any) => {
    setLoading(true);
    try {
      const data = await login(values.account as string, values.password as string);
      localStorage.setItem(AUTH_TOKEN_KEY, data.token);
      await setInitialState((s) => ({ ...s, currentUser: data.user }));
      message.success('登录成功');
    } catch (e: unknown) {
      const ax = e as { response?: { data?: { message?: string } }; message?: string };
      message.error(ax?.response?.data?.message || ax?.message || '登录失败');
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
      localStorage.setItem(AUTH_TOKEN_KEY, data.token);
      await setInitialState((s) => ({ ...s, currentUser: data.user }));
      message.success('注册并登录成功');
    } catch (e: unknown) {
      const ax = e as { response?: { data?: { message?: string } }; message?: string };
      message.error(ax?.response?.data?.message || ax?.message || '注册失败');
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
      message.success('验证码已发送');
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
      const ax = e as { response?: { data?: { message?: string } }; message?: string };
      message.error(ax?.response?.data?.message || ax?.message || '发送失败');
    }
  };

  return (
    <div className="login-shell">
    <div className="login-container">
      <div className="login-left">
        <div className="login-left-decor" aria-hidden="true">
          <div className="decor-glow" />
          <div className="decor-card decor-card-1" />
          <div className="decor-card decor-card-2" />
          <div className="decor-card decor-card-3" />
        </div>

        <div className="login-left-content">
          <div className="brand">
            <BrandLogo height={32} />
            <div>
              <div className="brand-text">贸灵 TradeMind</div>
              <div className="brand-sub">AI-Powered Cross-Border ERP</div>
            </div>
          </div>

          <div className="slogan">
            <h1>
              用 <span className="highlight">AI</span> 驱动你的
              <br />
              跨境运营增长
            </h1>
            <p>
              贸灵 TradeMind 帮助跨境卖家统一管理商品、采集、AI 优化、刊登、库存与数据分析，让运营更高效。
            </p>
          </div>

          <div className="features">
            {FEATURE_TAGS.map((tag) => (
              <div key={tag.label} className={`feature-tag ${tag.className}`}>
                {tag.icon} {tag.label}
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="login-right">
        <div className="login-right-inner">
          <div className="mobile-brand">
            <BrandLogo height={28} />
            <div>
              <div className="brand-text">贸灵 TradeMind</div>
              <div className="brand-sub">AI-Powered Cross-Border ERP</div>
            </div>
          </div>

          <div className="auth-card">
            <Tabs
              className="auth-tabs"
              activeKey={activeTab}
              centered
              onChange={setActiveTab}
              items={[
                { key: 'login', label: '登录' },
                { key: 'register', label: '注册' },
              ]}
            />

            <div className="welcome-text">
              <h2>{activeTab === 'login' ? '欢迎回来' : '注册账号'}</h2>
              <p>
                {activeTab === 'login'
                  ? '登录你的 TradeMind 工作台'
                  : '开启你的 AI 跨境之旅'}
              </p>
            </div>

            {activeTab === 'login' ? (
              <Form
                form={loginForm}
                layout="vertical"
                onFinish={onLogin}
                requiredMark={false}
                autoComplete="off"
              >
                <Form.Item
                  name="account"
                  label="邮箱 / 手机号"
                  rules={[{ required: true, message: '请输入邮箱或手机号' }]}
                  validateTrigger="onBlur"
                >
                  <Input
                    placeholder="请输入邮箱或手机号"
                    prefix={<MailIcon />}
                    autoComplete="off"
                    data-lpignore="true"
                    data-1p-ignore="true"
                  />
                </Form.Item>

                <Form.Item
                  name="password"
                  label="密码"
                  rules={[{ required: true, message: '请输入登录密码' }]}
                  validateTrigger="onBlur"
                >
                  <Input.Password
                    placeholder="请输入登录密码"
                    prefix={<LockIcon />}
                    autoComplete="new-password"
                    data-lpignore="true"
                    data-1p-ignore="true"
                  />
                </Form.Item>

                <div className="form-actions">
                  <Form.Item name="remember" valuePropName="checked" noStyle initialValue={true}>
                    <Checkbox>记住我</Checkbox>
                  </Form.Item>
                  <a href="#" className="forgot-link" onClick={(e) => e.preventDefault()}>
                    忘记密码？
                  </a>
                </div>

                <Form.Item>
                  <Button
                    type="primary"
                    htmlType="submit"
                    className="submit-btn"
                    loading={loading}
                    disabled={loading}
                  >
                    登录工作台
                  </Button>
                </Form.Item>

                <div className="register-link">
                  还没有账号？
                  <a
                    onClick={(e) => {
                      e.preventDefault();
                      setActiveTab('register');
                    }}
                  >
                    立即注册
                  </a>
                </div>
              </Form>
            ) : (
              <Form
                form={registerForm}
                layout="vertical"
                onFinish={onRegister}
                requiredMark={false}
              >
                <Form.Item
                  name="email"
                  label="邮箱"
                  rules={[
                    { required: true, message: '请输入邮箱' },
                    { type: 'email', message: '请输入有效的邮箱地址' },
                  ]}
                  validateTrigger="onBlur"
                >
                  <Input placeholder="请输入邮箱" prefix={<MailIcon />} autoComplete="email" />
                </Form.Item>

                <Form.Item label="邮箱验证码" required>
                  <Row gutter={8}>
                    <Col span={15}>
                      <Form.Item
                        name="code"
                        noStyle
                        rules={[{ required: true, message: '请输入验证码' }]}
                      >
                        <Input placeholder="6位验证码" />
                      </Form.Item>
                    </Col>
                    <Col span={9}>
                      <Button
                        style={{ width: '100%', height: 50, borderRadius: 11 }}
                        onClick={handleSendCode}
                        disabled={countdown > 0}
                      >
                        {countdown > 0 ? `${countdown}s 后重发` : '获取验证码'}
                      </Button>
                    </Col>
                  </Row>
                </Form.Item>

                <Form.Item
                  name="password"
                  label="密码"
                  rules={[
                    { required: true, message: '请输入密码' },
                    { min: 6, message: '密码至少6位' },
                  ]}
                  validateTrigger="onBlur"
                >
                  <Input.Password
                    placeholder="请输入至少6位密码"
                    prefix={<LockIcon />}
                    autoComplete="new-password"
                  />
                </Form.Item>

                <Form.Item
                  name="confirmPassword"
                  label="确认密码"
                  dependencies={['password']}
                  validateTrigger="onBlur"
                  rules={[
                    { required: true, message: '请确认密码' },
                    ({ getFieldValue }) => ({
                      validator(_, value) {
                        if (!value || getFieldValue('password') === value) {
                          return Promise.resolve();
                        }
                        return Promise.reject(new Error('两次输入的密码不一致!'));
                      },
                    }),
                  ]}
                >
                  <Input.Password
                    placeholder="请再次输入密码"
                    prefix={<LockIcon />}
                    autoComplete="new-password"
                  />
                </Form.Item>

                <Form.Item>
                  <Button
                    type="primary"
                    htmlType="submit"
                    className="submit-btn"
                    loading={loading}
                    disabled={loading}
                  >
                    注册
                  </Button>
                </Form.Item>

                <div className="register-link">
                  已有账号？
                  <a
                    onClick={(e) => {
                      e.preventDefault();
                      setActiveTab('login');
                    }}
                  >
                    去登录
                  </a>
                </div>
              </Form>
            )}

            <div className="agreement">
              登录即表示你同意 <a href="#">《用户协议》</a> 和 <a href="#">《隐私政策》</a>
            </div>
          </div>
        </div>
      </div>
    </div>
    </div>
  );
}
