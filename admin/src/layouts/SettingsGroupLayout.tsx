import { Outlet, history, useLocation } from '@umijs/max';
import { Result } from 'antd';
import { useLayoutEffect } from 'react';
import { TmPageContainer } from '@/components/ui';
import { useInitialStateModel } from '@/hooks/useInitialStateModel';
import { canAccessPath } from '@/utils/menuAccess';
import { PERMISSION_DENIED_MESSAGE } from '@/utils/permission';

export default function SettingsGroupLayout() {
  const { pathname } = useLocation();
  const { initialState } = useInitialStateModel();
  const user = initialState?.currentUser;
  const allowed = (path: string) => canAccessPath(path, user?.role, user?.permissions, user?.platformOperator === true, user?.tenantId, user?.isRoot === true);
  const isParent = pathname.replace(/\/+$/, '') === '/settings';
  const destination = allowed('/settings/system') ? '/settings/system' : allowed('/settings/users') ? '/settings/users' : null;
  useLayoutEffect(() => {
    if (isParent && destination) history.replace(destination);
  }, [isParent, destination]);
  if ((isParent && !destination) || (!isParent && !allowed(pathname))) {
    return <TmPageContainer title="设置"><Result status="403" title="无权限" subTitle={PERMISSION_DENIED_MESSAGE} /></TmPageContainer>;
  }
  return <Outlet />;
}
