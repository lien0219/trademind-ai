import { history, Outlet, useLocation } from '@umijs/max';
import { Result } from 'antd';
import { useLayoutEffect } from 'react';
import { TmPageContainer } from '@/components/ui';
import { useInitialStateModel } from '@/hooks/useInitialStateModel';
import { canAccessPath } from '@/utils/menuAccess';
import { PERMISSION_DENIED_MESSAGE } from '@/utils/permission';
import { isProductionBuild } from '@/utils/runtimeEnvironment';

const ENTRY_PATHS = [
  '/ops/workers/monitor',
  '/ops/task-center/failures',
  '/ops/task-center/alerts',
  '/ops/task-center/operation-tasks',
  '/ops/observability',
  '/ops/platform-runtime',
];

const INTERNAL_ENTRY_PATHS = isProductionBuild ? [] : ['/ops/inventory-sync'];

export default function OpsGroupLayout() {
  const { pathname } = useLocation();
  const { initialState } = useInitialStateModel();
  const user = initialState?.currentUser;
  const isParent = pathname.replace(/\/+$/, '') === '/ops';
  const allowed = (path: string) => canAccessPath(path, user?.role, user?.permissions, user?.platformOperator === true, user?.tenantId, user?.isRoot === true);
  const destination = [...ENTRY_PATHS, ...INTERNAL_ENTRY_PATHS].find((path) =>
    allowed(path),
  );

  useLayoutEffect(() => {
    if (isParent && destination) history.replace(destination);
  }, [isParent, destination]);

  if ((isParent && !destination) || (!isParent && !allowed(pathname))) {
    return (
      <TmPageContainer title="运维">
        <Result status="403" title="无权限" subTitle={PERMISSION_DENIED_MESSAGE} />
      </TmPageContainer>
    );
  }
  return <Outlet />;
}
