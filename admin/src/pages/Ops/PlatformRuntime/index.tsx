import { TmPageContainer } from '@/components/ui';
import { PLATFORM_STATUS_META } from '@/constants/platformAppConfig';
import {
  isPlatformRuntimeSupported,
  platformRuntimeHref,
  resolvePlatformRuntimeTab,
} from '@/constants/platformRuntime';
import { preferredPlatformTabOrder } from '@/services/platformOpen';
import { queryPlatformProviders, type PlatformProviderMeta } from '@/services/shops';
import { history, useLocation } from '@umijs/max';
import { Alert, Spin, Tabs, Tag } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import DouyinRuntimePanel from './DouyinRuntimePanel';
import PlatformRuntimeUnavailablePanel from './PlatformRuntimeUnavailablePanel';

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
  const [providers, setProviders] = useState<PlatformProviderMeta[]>([]);

  const loadProviders = useCallback(async () => {
    setLoadingProviders(true);
    try {
      const { list } = await queryPlatformProviders();
      setProviders(list ?? []);
    } catch {
      setProviders([]);
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
        <span>
          {p.name}
          {runtimeReady ? null : (
            <Tag color="default" style={{ marginLeft: 6, marginRight: 0 }}>
              未接入
            </Tag>
          )}
          {st && p.status !== 'available' ? (
            <Tag color={st.color} style={{ marginLeft: 6, marginRight: 0 }}>
              {st.label}
            </Tag>
          ) : null}
        </span>
      ),
      children: renderPlatformPanel(p),
    };
  });

  return (
    <TmPageContainer
      title="平台运行状态"
      subTitle="按平台查看健康检查、运行指标、运行控制与发布门禁；未接入运行时的平台仅展示说明，不可操作。"
    >
      <Spin spinning={loadingProviders}>
        {tabProviders.length === 0 ? (
          <Alert
            showIcon
            type="info"
            message="暂无平台"
            description="请刷新页面或先在平台接入设置中确认平台接入方已注册。"
          />
        ) : (
          <Tabs activeKey={activePlatform} onChange={onTabChange} items={tabItems} destroyInactiveTabPane />
        )}
      </Spin>
    </TmPageContainer>
  );
}
