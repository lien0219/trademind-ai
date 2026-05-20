import { Alert, Button, Space, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import {
  checkPinduoduoLogin,
  openPinduoduoLoginBrowser,
  type ProviderPinduoduoAuthStatus,
  type ProviderPinduoduoAuthStatusValue,
} from '@/services/collectAuth';
import { hasPinduoduoLoginContext, resolvePinduoduoLoginTargetUrl } from '@/utils/pinduoduoUrl';

type AuthDisplayStatus = 'unchecked' | 'checking' | ProviderPinduoduoAuthStatusValue;

const AUTH_STATUS_LABEL: Record<
  AuthDisplayStatus,
  { text: string; badge?: 'success' | 'error' | 'warning' | 'default' }
> = {
  unchecked: { text: '未检测' },
  checking: { text: '检测中…' },
  ok: { text: '已登录', badge: 'success' },
  not_logged_in: { text: '需要登录', badge: 'error' },
  wechat_auth_required: { text: '需要微信扫码授权', badge: 'warning' },
  app_redirect: { text: 'App 引导页', badge: 'warning' },
  verification_required: { text: '需要验证', badge: 'warning' },
  homepage_only: { text: '只能访问首页，无法确认是否已登录', badge: 'warning' },
  unknown: { text: '暂时无法确认登录状态', badge: 'default' },
};

function resolveDisplayStatus(
  status: ProviderPinduoduoAuthStatus | null,
  checking: boolean,
  loaded: boolean,
): AuthDisplayStatus {
  if (checking) return 'checking';
  if (!loaded) return 'unchecked';
  if (!status) return 'unknown';
  if (status.status) return status.status;
  if (status.needVerification) return 'verification_required';
  if (status.loggedIn) return 'ok';
  return 'not_logged_in';
}

type Props = {
  /** 失败任务 / 采集弹窗中的商品或批发链接，用于打开登录与重新检测 */
  loginUrl?: string;
  compact?: boolean;
  onAuthChange?: (status: ProviderPinduoduoAuthStatus | null) => void;
};

export function PinduoduoLoginPanel({ loginUrl, compact, onAuthChange }: Props) {
  const [authStatus, setAuthStatus] = useState<ProviderPinduoduoAuthStatus | null>(null);
  const [authChecking, setAuthChecking] = useState(false);
  const [loginOpening, setLoginOpening] = useState(false);
  const [loaded, setLoaded] = useState(false);

  const contextUrl = loginUrl?.trim() ?? '';
  const hasContext = hasPinduoduoLoginContext(contextUrl);
  const targetUrl = resolvePinduoduoLoginTargetUrl(hasContext ? contextUrl : undefined);

  const loadAuthStatus = useCallback(async () => {
    setAuthChecking(true);
    try {
      const data = await checkPinduoduoLogin(
        hasContext ? { url: contextUrl } : undefined,
      );
      setAuthStatus(data);
      onAuthChange?.(data);
    } catch (e: unknown) {
      setAuthStatus(null);
      onAuthChange?.(null);
      message.error((e as Error)?.message || '拼多多登录态检测失败');
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
      const result = await openPinduoduoLoginBrowser(targetUrl);
      message.success(
        result.message ||
          (hasContext
            ? '已打开目标页面，请完成登录或授权后点击「重新检测」'
            : '已打开批发入口；建议从失败任务打开具体商品链接'),
        8,
      );
    } catch (e: unknown) {
      message.error((e as Error)?.message || '打开采集浏览器失败');
    } finally {
      setLoginOpening(false);
    }
  };

  const hint =
    displayKey === 'homepage_only'
      ? '只能访问拼多多首页，无法确认是否已登录。拼多多首页可能游客也能访问。请从失败任务或采集弹窗中使用具体商品链接重新检测。'
      : displayKey === 'app_redirect'
        ? '当前打开的是拼多多 App 引导页，无法确认采集浏览器是否已登录。请从具体商品链接或失败任务中打开登录，再重新检测。'
        : displayKey === 'wechat_auth_required'
          ? '拼多多可能需要微信扫码授权，请在弹出的采集浏览器中完成授权。'
          : displayKey === 'not_logged_in'
            ? '请先打开采集浏览器登录拼多多，然后重新检测。'
            : displayKey === 'verification_required'
              ? '拼多多页面可能出现验证码或安全验证，请在采集浏览器中手动完成验证后重试。'
              : displayKey === 'unknown'
                ? '请确认采集浏览器中是否已完成登录或微信授权，然后使用具体商品链接重新检测。'
                : hasContext
                  ? '将使用当前商品链接打开采集浏览器，便于在网页中完成登录或微信授权。系统不会保存账号密码。'
                  : '建议从失败任务或采集弹窗中打开登录，系统会直接打开需要采集的商品页。若无上下文，将打开批发入口（非移动端 App 首页）。';

  return (
    <Alert
      type={
        displayKey === 'ok'
          ? 'success'
          : displayKey === 'not_logged_in' ||
              displayKey === 'wechat_auth_required' ||
              displayKey === 'app_redirect' ||
              displayKey === 'homepage_only'
            ? 'warning'
            : 'info'
      }
      showIcon
      message={
        compact ? (
          <Space wrap>
            <Typography.Text strong>拼多多登录状态</Typography.Text>
            <Typography.Text type={meta.badge === 'success' ? 'success' : undefined}>
              {meta.text}
            </Typography.Text>
          </Space>
        ) : (
          '拼多多登录状态'
        )
      }
      description={
        <Space direction="vertical" size="small" style={{ width: '100%' }}>
          {!compact ? (
            <Typography.Paragraph style={{ marginBottom: 4 }} type="secondary">
              {hint}
            </Typography.Paragraph>
          ) : null}
          {hasContext ? (
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              检测/登录目标：{contextUrl.length > 72 ? `${contextUrl.slice(0, 72)}…` : contextUrl}
            </Typography.Text>
          ) : null}
          {authStatus?.message && displayKey !== 'unchecked' ? (
            <Typography.Text type="secondary">{authStatus.message}</Typography.Text>
          ) : null}
          {authStatus?.lastCheckedAt ? (
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              上次检测：{authStatus.lastCheckedAt}
            </Typography.Text>
          ) : null}
          <Space wrap align="start">
            <Button size="small" onClick={() => void loadAuthStatus()} loading={authChecking}>
              重新检测
            </Button>
            <Space direction="vertical" size={2}>
              <Button
                size="small"
                type="primary"
                onClick={() => void handleOpenLogin()}
                loading={loginOpening}
              >
                打开拼多多采集浏览器登录
              </Button>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                如跳转到微信页面，请用微信扫码完成授权。
              </Typography.Text>
            </Space>
          </Space>
        </Space>
      }
    />
  );
}
