import type { ReactNode } from 'react';
import EmptyState from '@/components/ui/EmptyState';
import { LIST_EMPTY_COPY, type ListEmptyCopy, type ListEmptyKey } from '@/constants/copywriting';

export type ListEmptyOptions = {
  /** 运营/只读账号可能因店铺权限看不到数据 */
  permissionScoped?: boolean;
  /** 只读账号不展示写操作按钮 */
  readonly?: boolean;
  /** 覆盖描述 */
  description?: ReactNode;
  /** 覆盖操作按钮 */
  actionLabel?: string;
  actionPath?: string;
  onAction?: () => void;
};

/** 构建 ProTable / Table 的 locale.emptyText，统一 EmptyState 体验（F7）。 */
export function buildListEmptyLocale(key: ListEmptyKey, opts?: ListEmptyOptions) {
  const copy = LIST_EMPTY_COPY[key] as ListEmptyCopy;
  let description: ReactNode = opts?.description ?? copy.description;
  if (opts?.permissionScoped && copy.permissionHint) {
    description = (
      <>
        {description}
        <div style={{ marginTop: 8 }}>{copy.permissionHint}</div>
      </>
    );
  }
  const actionLabel = opts?.actionLabel ?? copy.action;
  const actionPath = opts?.actionPath ?? copy.actionPath;
  const onAction = opts?.onAction ?? copy.onAction;
  const showAction = !opts?.readonly && actionLabel && (actionPath || onAction);
  return {
    emptyText: (
      <EmptyState
        title={copy.title}
        description={description}
        actionLabel={showAction ? actionLabel : undefined}
        actionPath={showAction ? actionPath : undefined}
        onAction={showAction ? onAction : undefined}
      />
    ),
  };
}
