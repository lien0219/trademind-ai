import { useLanguage } from '@/i18n/useLanguage';
import { tr } from '@/i18n';
import type { ReactNode } from 'react';
import { Result } from 'antd';
import { usePermission } from '@/hooks/usePermission';
import { PERMISSION_DENIED_MESSAGE, type PermissionKey } from '@/utils/permission';

type Props = {
  require?: PermissionKey;
  requireWrite?: PermissionKey;
  children: ReactNode;
  fallback?: ReactNode;
  showForbiddenPage?: boolean;
};

export default function PermissionGuard({
  require,
  requireWrite,
  children,
  fallback,
  showForbiddenPage,
}: Props) {
  useLanguage();
  const { can, readonly } = usePermission();
  const perm = requireWrite || require;
  if (!perm) {
    return <>{children}</>;
  }
  if (readonly && requireWrite) {
    if (showForbiddenPage) {
      return <Result status="403" title={tr("只读账号")} subTitle={tr("只读账号不可执行写操作")} />;
    }
    return <>{fallback ?? null}</>;
  }
  if (!can(perm)) {
    if (showForbiddenPage) {
      return <Result status="403" title={tr("无权限")} subTitle={tr(PERMISSION_DENIED_MESSAGE)} />;
    }
    return <>{fallback ?? null}</>;
  }
  return <>{children}</>;
}
