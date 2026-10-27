import { useLanguage } from '@/i18n/useLanguage';
import { tr } from '@/i18n';
import { Result, Tabs } from 'antd';
import { useEffect, type ReactNode } from 'react';
import { TmPageContainer } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import { useUrlQueryState } from '@/hooks/useUrlState';
import { PERMISSIONS } from '@/utils/permission';
import { resolveAlertSource, type AlertSource } from './alertMeta';
import BusinessAlertsPanel from './BusinessAlertsPanel';
import SystemAlertsPanel from './SystemAlertsPanel';

const ALERT_SOURCE_QUERY_KEYS = ['source'] as const;

export default function AlertCenterPage() {
  useLanguage();
  const { can } = usePermission();
  const { state, setState } = useUrlQueryState<{ source?: string }>(ALERT_SOURCE_QUERY_KEYS);
  const access = {
    business: can(PERMISSIONS.TASK_RETRY),
    system: can(PERMISSIONS.ALERTS_READ),
  };
  const activeSource = resolveAlertSource(state.source, access);
  useEffect(() => {
    if ((access.business || access.system) && state.source !== activeSource) {
      setState({ source: activeSource }, { replace: true });
    }
  }, [access.business, access.system, activeSource, setState, state.source]);
  const items = [
    access.business
      ? { key: 'business', label: '业务告警', children: <BusinessAlertsPanel /> }
      : null,
    access.system
      ? { key: 'system', label: '系统告警', children: <SystemAlertsPanel /> }
      : null,
  ].filter(Boolean) as { key: AlertSource; label: string; children: ReactNode }[];

  if (!items.length) {
    return (
      <TmPageContainer title={tr("告警中心")}>
        <Result status="403" title="无权限" subTitle="当前账号无权限访问告警中心" />
      </TmPageContainer>
    );
  }

  return (
    <TmPageContainer title={tr("告警中心")}>
      <Tabs
        activeKey={activeSource}
        items={items}
        destroyOnHidden
        onChange={(source) => setState({ source }, { replace: true })}
      />
    </TmPageContainer>
  );
}
