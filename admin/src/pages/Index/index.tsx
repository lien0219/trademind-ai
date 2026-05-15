import { history } from '@umijs/max';
import { useLayoutEffect } from 'react';
import { AUTH_TOKEN_KEY } from '@/constants/auth';

/**
 * 根路径不要用 route.redirect（Umi 的 Navigate）与 layout.onPageChange 同时改 URL，会打成死循环。
 */
export default function IndexPage() {
  useLayoutEffect(() => {
    if (localStorage.getItem(AUTH_TOKEN_KEY)) {
      history.replace('/dashboard');
    } else {
      history.replace('/user/login?redirect=/');
    }
  }, []);
  return null;
}
