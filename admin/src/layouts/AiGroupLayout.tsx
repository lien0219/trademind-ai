import { Outlet, history, useLocation } from '@umijs/max';
import { useLayoutEffect } from 'react';

const PARENT = '/ai';
const DEFAULT_CHILD = '/ai/prompts';

export default function AiGroupLayout() {
  const { pathname } = useLocation();
  useLayoutEffect(() => {
    if (pathname === PARENT) {
      history.replace(DEFAULT_CHILD);
    }
  }, [pathname]);
  return <Outlet />;
}
