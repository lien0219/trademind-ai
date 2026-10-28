import { tr } from '@/i18n';
import { ErrorAlert, TmPageContainer } from '@/components/ui';
import { PLATFORM_STATUS_META } from '@/constants/platformAppConfig';
import {
  isPlatformRuntimeSupported,
  platformRuntimeHref,
  resolvePlatformRuntimeTab,
} from '@/constants/platformRuntime';
import { preferredPlatformTabOrder } from '@/services/platformOpen';
import { queryPlatformProviders, type PlatformProviderMeta } from '@/services/shops';
import { history, useLocation } from '@umijs/max';
import { Alert, Button, Spin, Tabs, Tag } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import DouyinRuntimePanel from './DouyinRuntimePanel';
import PlatformRuntimeUnavailablePanel from './PlatformRuntimeUnavailablePanel';
import styles from './index.less';

function renderPlatformPanel(meta: PlatformProviderMeta) {
  if (isPlatformRuntimeSupported(meta.platform)) {
    switch (meta.platform) {
      case 'douyin_shop':
        return <DouyinRuntimePanel />;
      default:
        break;
    }
  }
  return <PlatformRuntimeUnavailablePanel meta={meta} />;
}

export default function PlatformRuntimePage() {
  const location = useLocation();
  const [loadingProviders, setLoadingProviders] = useState(true);
  const [providersError, setProvidersError] = useState(false);
  const [providers, setProviders] = useState<PlatformProviderMeta[]>([]);

  const loadProviders = useCallback(async () => {
    setLoadingProviders(true);
    try {
      const { list } = await queryPlatformProviders();
      setProviders(list ?? []);
      setProvidersError(false);
    } catch {
      setProviders([]);
      setProvidersError(true);
    } finally {
      setLoadingProviders(false);
    }
  }, []);

  useEffect(() => {
    void loadProviders();
  }, [loadProviders]);

  const tabProviders = useMemo(() => {
    return [...providers].sort(
      (a, b) => preferredPlatformTabOrder(a.platform) - preferredPlatformTabOrder(b.platform),
    );
  }, [providers]);

  const allPlatforms = useMemo(() => tabProviders.map((p) => p.platform), [tabProviders]);

  const activePlatform = useMemo(() => {
    const sp = new URLSearchParams(location.search || '');
    return resolvePlatformRuntimeTab(sp.get('platform'), allPlatforms);
  }, [location.search, allPlatforms]);

  useEffect(() => {
    if (loadingProviders || allPlatforms.length === 0) {
      return;
    }
    const sp = new URLSearchParams(location.search || '');
    const current = (sp.get('platform') || '').trim().toLowerCase();
    if (current !== activePlatform) {
      history.replace(platformRuntimeHref(activePlatform));
    }
  }, [activePlatform, allPlatforms, loadingProviders, location.search]);

  const onTabChange = (platform: string) => {
    history.replace(platformRuntimeHref(platform));
  };

  const tabItems = tabProviders.map((p) => {
    const st = PLATFORM_STATUS_META[p.status];
    const runtimeReady = isPlatformRuntimeSupported(p.platform);
    return {
      key: p.platform,
      label: (
        <span className={styles.platformTabLabel}>
          <span>{p.name}</span>
          <span className={styles.platformTabTags}>
            {runtimeReady ? null : <Tag color="default">{tr("未接入")}</Tag>}
            {st && p.status !== 'available' ? <Tag color={st.color}>{st.label}</Tag> : null}
          </span>
        </span>
      ),
      children: renderPlatformPanel(p),
    };
  });

  return (
    <TmPageContainer
      title={tr("平台运行状态")}
      subTitle={tr("按平台查看健康检查、运行指标、运行控制与发布门禁；未接入运行时的平台仅展示说明，不可操作。")}
      className={styles.page}
    >
      {providersError ? (
        <ErrorAlert
          title={tr("平台列表加载失败")}
          actionHint={tr("请检查后端服务后重试。")}
          action={
            <Button size="small" onClick={() => void loadProviders()}>
              {tr("重新加载")}
            </Button>
          }
          className={styles.providersError}
        />
      ) : null}
      <Spin spinning={loadingProviders} className={styles.providerSpin}>
        {providersError ? null : tabProviders.length === 0 ? (
          <Alert
            showIcon
            type="info"
            message={tr("暂无平台")}
            description={tr("请刷新页面或先在平台接入设置中确认平台接入方已注册。")}
          />
        ) : (
          <Tabs
            activeKey={activePlatform}
            onChange={onTabChange}
            items={tabItems}
            destroyOnHidden
            className={styles.platformTabs}
          />
        )}
      </Spin>
    </TmPageContainer>
  );
}
