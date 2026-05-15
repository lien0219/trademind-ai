import { Outlet, history, useLocation } from '@umijs/max';
import { useLayoutEffect } from 'react';

const PARENT = '/product';
const DEFAULT_CHILD = '/product/drafts';

export default function ProductGroupLayout() {
  const { pathname } = useLocation();
  useLayoutEffect(() => {
    if (pathname === PARENT) {
      history.replace(DEFAULT_CHILD);
    }
  }, [pathname]);
  return <Outlet />;
}
