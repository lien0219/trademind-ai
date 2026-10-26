import { Outlet, history, useLocation } from '@umijs/max';
import { Result } from 'antd';
import { useLayoutEffect } from 'react';
import { TmPageContainer } from '@/components/ui';
import { useInitialStateModel } from '@/hooks/useInitialStateModel';
import { canAccessPath } from '@/utils/menuAccess';
import { PERMISSION_DENIED_MESSAGE } from '@/utils/permission';

export default function AiGroupLayout() {
  const { pathname } = useLocation();
  const { initialState } = useInitialStateModel();
  const user = initialState?.currentUser;
  const allowed = (path: string) => canAccessPath(path, user?.role, user?.permissions, user?.platformOperator === true, user?.tenantId, user?.isRoot === true);
  const isParent = pathname.replace(/\/+$/, '') === '/ai';
  const destination = allowed('/ai/prompts') ? '/ai/prompts' : allowed('/ai/tasks') ? '/ai/tasks' : null;
  useLayoutEffect(() => {
    if (isParent && destination) history.replace(destination);
  }, [isParent, destination]);
  if ((isParent && !destination) || (!isParent && !allowed(pathname))) {
    return <TmPageContainer title="AI"><Result status="403" title="无权限" subTitle={PERMISSION_DENIED_MESSAGE} /></TmPageContainer>;
  }
  return <Outlet />;
}
