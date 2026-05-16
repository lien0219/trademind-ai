import { Outlet, history, useLocation } from '@umijs/max';
import { useLayoutEffect } from 'react';

const PARENT = '/customer';
const DEFAULT_CHILD = '/customer/conversations';

export default function CustomerGroupLayout() {
  const { pathname } = useLocation();
  useLayoutEffect(() => {
    if (pathname === PARENT || pathname === `${PARENT}/`) {
      history.replace(DEFAULT_CHILD);
    }
  }, [pathname]);
  return <Outlet />;
}
