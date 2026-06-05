import { Alert, Button, Space, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import {
  checkTaobaoTmallLogin,
  openTaobaoTmallLoginBrowser,
  type ProviderTaobaoTmallAuthStatus,
  type ProviderTaobaoTmallAuthStatusValue,
} from '@/services/collectAuth';
import {
  hasTaobaoTmallLoginContext,
  resolveTaobaoTmallLoginTargetUrl,
} from '@/utils/taobaoTmallUrl';

type AuthDisplayStatus = 'unchecked' | 'checking' | ProviderTaobaoTmallAuthStatusValue;

const AUTH_STATUS_LABEL: Record<AuthDisplayStatus, { text: string }> = {
  unchecked: { text: '未检测' },
  checking: { text: '检测中…' },
  logged_in: { text: '已登录' },
  login_required: { text: '需要登录' },
  verify_required: { text: '需要验证' },
  unknown: { text: '暂时无法确认' },
};

function resolveDisplayStatus(
  status: ProviderTaobaoTmallAuthStatus | null,
  checking: boolean,
  loaded: boolean,
): AuthDisplayStatus {
  if (checking) return 'checking';
  if (!loaded) return 'unchecked';
  if (!status) return 'unknown';
  if (status.status) return status.status;
  if (status.needVerification) return 'verify_required';
  if (status.loggedIn) return 'logged_in';
  return 'login_required';
}

type Props = {
  loginUrl?: string;
  compact?: boolean;
  onAuthChange?: (status: ProviderTaobaoTmallAuthStatus | null) => void;
};

export function TaobaoTmallLoginPanel({ loginUrl, compact, onAuthChange }: Props) {
  const [authStatus, setAuthStatus] = useState<ProviderTaobaoTmallAuthStatus | null>(null);
  const [authChecking, setAuthChecking] = useState(false);
  const [loginOpening, setLoginOpening] = useState(false);
  const [loaded, setLoaded] = useState(false);

  const contextUrl = loginUrl?.trim() ?? '';
  const hasContext = hasTaobaoTmallLoginContext(contextUrl);
  const targetUrl = resolveTaobaoTmallLoginTargetUrl(hasContext ? contextUrl : undefined);

  const loadAuthStatus = useCallback(async () => {
    setAuthChecking(true);
    try {
      const data = await checkTaobaoTmallLogin(hasContext ? { url: contextUrl } : undefined);
      setAuthStatus(data);
      onAuthChange?.(data);
    } catch (e: unknown) {
      setAuthStatus(null);
      onAuthChange?.(null);
      message.error((e as Error)?.message || '淘宝/天猫登录态检测失败');
    } finally {
      setAuthChecking(false);
      setLoaded(true);
    }
  }, [contextUrl, hasContext, onAuthChange]);

  useEffect(() => {
    void loadAuthStatus();
  }, [loadAuthStatus]);

  const displayKey = resolveDisplayStatus(authStatus, authChecking, loaded);
  const meta = AUTH_STATUS_LABEL[displayKey];

  const handleOpenLogin = async () => {
    setLoginOpening(true);
    try {
      const result = await openTaobaoTmallLoginBrowser(targetUrl);
      message.success(result.message || '已打开淘宝/天猫采集浏览器', 8);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '打开采集浏览器失败');
    } finally {
      setLoginOpening(false);
    }
  };

  return (
    <Alert
      type={
        displayKey === 'logged_in'
          ? 'success'
          : displayKey === 'login_required' || displayKey === 'verify_required'
            ? 'warning'
            : 'info'
      }
      showIcon
      message={compact ? `登录状态：${meta.text}` : '淘宝/天猫登录状态'}
      description={
        compact ? (
          <Space wrap style={{ marginTop: 4 }}>
            <Button size="small" onClick={() => void loadAuthStatus()} loading={authChecking}>
              重新检测
            </Button>
            <Button size="small" type="primary" onClick={() => void handleOpenLogin()} loading={loginOpening}>
              打开采集浏览器
            </Button>
          </Space>
        ) : (
          <Space direction="vertical" size="small" style={{ width: '100%' }}>
            <Typography.Paragraph type="secondary" style={{ marginBottom: 4 }}>
              部分淘宝/天猫商品需要登录后才能采集。如遇安全验证或滑块，请在采集浏览器中手动完成后再重试。
            </Typography.Paragraph>
            {authStatus?.message ? (
              <Typography.Text type="secondary">{authStatus.message}</Typography.Text>
            ) : null}
            <Space wrap>
              <Button size="small" onClick={() => void loadAuthStatus()} loading={authChecking}>
                重新检测
              </Button>
              <Button size="small" type="primary" onClick={() => void handleOpenLogin()} loading={loginOpening}>
                打开淘宝/天猫采集浏览器
              </Button>
            </Space>
          </Space>
        )
      }
    />
  );
}
