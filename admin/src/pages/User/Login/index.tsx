import { LoginForm, ProFormText } from '@ant-design/pro-components';
import { history, useModel } from '@umijs/max';
import { message } from 'antd';
import { useEffect } from 'react';
import { AUTH_TOKEN_KEY } from '@/constants/auth';
import { login } from '@/services/auth';

export default function LoginPage() {
  const { setInitialState, initialState } = useModel('@@initialState');

  // 依赖用布尔值，避免 currentUser 对象引用每次渲染都变导致反复 replace → Navigate 死循环
  const loggedIn = Boolean(initialState?.currentUser);
  useEffect(() => {
    if (!loggedIn) return;
    const q = new URLSearchParams(history.location.search);
    history.replace(q.get('redirect') || '/dashboard');
  }, [loggedIn]);

  return (
    <div style={{ paddingTop: 96, minHeight: '100vh', background: 'linear-gradient(135deg,#f0f5ff 0%,#fff 45%)' }}>
      <LoginForm
        title="贸灵 TradeMind"
        subTitle="管理员登录"
        onFinish={async (v) => {
          try {
            const data = await login(v.username as string, v.password as string);
            localStorage.setItem(AUTH_TOKEN_KEY, data.token);
            await setInitialState((s) => ({ ...s, currentUser: data.user }));
            message.success('登录成功');
            // 跳转交给下方 useEffect（loggedIn 变 true 后统一 replace），避免 push + replace 叠加重定向
            return true;
          } catch (e: unknown) {
            const ax = e as { response?: { data?: { message?: string } }; message?: string };
            message.error(ax?.response?.data?.message || ax?.message || '登录失败');
            return false;
          }
        }}
      >
        <ProFormText
          name="username"
          placeholder="用户名"
          fieldProps={{ size: 'large', autoComplete: 'username' }}
          rules={[{ required: true, message: '请输入用户名' }]}
        />
        <ProFormText.Password
          name="password"
          placeholder="密码"
          fieldProps={{ size: 'large', autoComplete: 'current-password' }}
          rules={[{ required: true, message: '请输入密码' }]}
        />
      </LoginForm>
    </div>
  );
}
