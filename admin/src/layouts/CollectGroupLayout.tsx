import { Outlet, history, useLocation } from '@umijs/max';
import { useLayoutEffect } from 'react';

const PARENT = '/collect';
const DEFAULT_CHILD = '/collect/tasks';

export default function CollectGroupLayout() {
  const { pathname } = useLocation();
  useLayoutEffect(() => {
    if (pathname === PARENT) {
      history.replace(DEFAULT_CHILD);
    }
  }, [pathname]);
  return <Outlet />;
}
