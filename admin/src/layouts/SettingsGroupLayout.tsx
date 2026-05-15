import { Outlet, history, useLocation } from '@umijs/max';
import { useLayoutEffect } from 'react';

const PARENT = '/settings';
const DEFAULT_CHILD = '/settings/system';

export default function SettingsGroupLayout() {
  const { pathname } = useLocation();
  useLayoutEffect(() => {
    if (pathname === PARENT) {
      history.replace(DEFAULT_CHILD);
    }
  }, [pathname]);
  return <Outlet />;
}
