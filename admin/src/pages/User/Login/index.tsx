import { Form, Input, Checkbox, Button, Tabs, Row, Col } from 'antd';
import { history, useModel } from '@umijs/max';
import { message } from 'antd';
import { useEffect, useState, useRef } from 'react';
import BrandLogo from '@/components/BrandLogo';
import { AUTH_TOKEN_KEY } from '@/constants/auth';
import { login, register, sendEmailCode } from '@/services/auth';
import './index.less';

const PlatformIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" className="icon-blue">
    <rect x="2" y="4" width="20" height="16" rx="2" stroke="currentColor" strokeWidth="2" />
    <path d="M2 10H22" stroke="currentColor" strokeWidth="2" />
    <path d="M6 14H10" stroke="currentColor" strokeWidth="2" />
  </svg>
);

const RestockIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" className="icon-green">
    <path d="M21 8L12 13L3 8" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    <path d="M21 16L12 21L3 16" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    <path d="M12 3L21 8L12 13L3 8L12 3Z" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);

const ProfitIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" className="icon-purple">
    <path d="M18 20V10" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    <path d="M12 20V4" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    <path d="M6 20V14" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    <path d="M3 20H21" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);

const AiIcon = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" className="icon-green">
    <path d="M12 2L14.4 9.6L22 12L14.4 14.4L12 22L9.6 14.4L2 12L9.6 9.6L12 2Z" fill="currentColor" />
  </svg>
);

const MailIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path d="M4 7.00005L10.2 11.65C11.2667 12.45 12.7333 12.45 13.8 11.65L20 7" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    <rect x="3" y="5" width="18" height="14" rx="2" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
  </svg>
);

const LockIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <rect x="5" y="11" width="14" height="10" rx="2" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    <path d="M8 11V7C8 4.79086 9.79086 3 12 3C14.2091 3 16 4.79086 16 7V11" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
);

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
    <div className="login-container">
      <div className="login-left">
        <div className="brand">
          <BrandLogo height={32} />
          <div>
            <div className="brand-text">贸灵 TradeMind</div>
            <div className="brand-sub">AI-Powered Cross-Border ERP</div>
          </div>
        </div>

        <div className="slogan">
          <h1>
            用 <span className="highlight">AI</span> 驱动你的<br />跨境运营增长
          </h1>
          <p>
            贸灵 TradeMind 帮助跨境卖家统一管理订单、商品、<br />
            库存、采购、物流与利润分析，并通过 AI 提供经营洞察<br />
            与自动化建议。
          </p>
        </div>

        <div className="features">
          <div className="feature-tag tag-blue">
            <PlatformIcon /> 多平台订单同步
          </div>
          <div className="feature-tag tag-green">
            <RestockIcon /> 智能补货建议
          </div>
          <div className="feature-tag tag-purple">
            <ProfitIcon /> 跨境利润核算
          </div>
          <div className="feature-tag tag-teal">
            <AiIcon /> AI 经营助手
          </div>
        </div>

        <div className="illustration">
          <div className="sidebar">
            <div className="sidebar-item active"></div>
            <div className="sidebar-item"></div>
            <div className="sidebar-item"></div>
            <div className="sidebar-item"></div>
          </div>
          <div className="main-content">
            <div className="top-cards">
              <div className="card">
                <div className="title"></div>
                <div className="value"></div>
              </div>
              <div className="card">
                <div className="title"></div>
                <div className="value"></div>
              </div>
              <div className="card">
                <div className="title"></div>
                <div className="value"></div>
              </div>
              <div className="card">
                <div className="title"></div>
                <div className="value"></div>
              </div>
            </div>
            <div className="charts">
              <div className="chart"></div>
              <div className="chart" style={{ flex: 1.5 }}></div>
            </div>
          </div>
        </div>
      </div>

      <div className="login-right">
        <div className="login-card">
          <Tabs
            activeKey={activeTab}
            onChange={setActiveTab}
            items={[
              { key: 'login', label: '登录' },
              { key: 'register', label: '注册' },
            ]}
          />
          
          <div className="welcome-text">
            <h2>{activeTab === 'login' ? '欢迎回来' : '注册账号'}</h2>
            <p>{activeTab === 'login' ? '登录你的 TradeMind 工作台' : '开启你的 AI 跨境之旅'}</p>
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
              >
                <Input
                  size="large"
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
              >
                <Input.Password
                  size="large"
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
                <a href="#" className="forgot-link">忘记密码？</a>
              </div>

              <Form.Item>
                <Button
                  type="primary"
                  htmlType="submit"
                  className="submit-btn"
                  loading={loading}
                >
                  登录工作台
                </Button>
              </Form.Item>

              <div className="register-link">
                还没有账号？ <a onClick={(e) => { e.preventDefault(); setActiveTab('register'); }}>立即注册</a>
              </div>
            </Form>
          ) : (
            <Form form={registerForm} layout="vertical" onFinish={onRegister} requiredMark={false}>
              <Form.Item
                name="email"
                label="邮箱"
                rules={[
                  { required: true, message: '请输入邮箱' },
                  { type: 'email', message: '请输入有效的邮箱地址' }
                ]}
              >
                <Input
                  size="large"
                  placeholder="请输入邮箱"
                  prefix={<MailIcon />}
                  autoComplete="email"
                />
              </Form.Item>

              <Form.Item label="邮箱验证码" required>
                <Row gutter={8}>
                  <Col span={15}>
                    <Form.Item
                      name="code"
                      noStyle
                      rules={[{ required: true, message: '请输入验证码' }]}
                    >
                      <Input size="large" placeholder="6位验证码" />
                    </Form.Item>
                  </Col>
                  <Col span={9}>
                    <Button 
                      size="large" 
                      style={{ width: '100%' }}
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
                rules={[{ required: true, message: '请输入密码' }, { min: 6, message: '密码至少6位' }]}
              >
                <Input.Password
                  size="large"
                  placeholder="请输入至少6位密码"
                  prefix={<LockIcon />}
                  autoComplete="new-password"
                />
              </Form.Item>

              <Form.Item
                name="confirmPassword"
                label="确认密码"
                dependencies={['password']}
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
                  size="large"
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
                >
                  注册
                </Button>
              </Form.Item>

              <div className="register-link">
                已有账号？ <a onClick={(e) => { e.preventDefault(); setActiveTab('login'); }}>去登录</a>
              </div>
            </Form>
          )}

          <div className="agreement">
            登录即表示你同意 <a href="#">《用户协议》</a> 和 <a href="#">《隐私政策》</a>
          </div>
        </div>
      </div>
    </div>
  );
}