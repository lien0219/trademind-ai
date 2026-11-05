import { useMemo } from 'react';
import { useLanguage } from '@/i18n/useLanguage';
import { usePermission } from '@/hooks/usePermission';
import { buildListEmptyLocale, type ListEmptyOptions } from '@/utils/tableEmptyLocale';
import type { ListEmptyKey } from '@/constants/copywriting';

type UseListEmptyLocaleOpts = Omit<ListEmptyOptions, 'readonly'>;

/** 结合 RBAC 的列表空状态 locale（F7 EmptyState rollout）。 */
export function useListEmptyLocale(key: ListEmptyKey, opts?: UseListEmptyLocaleOpts) {
  const language = useLanguage();
  const { readonly } = usePermission();
  return useMemo(
    () => buildListEmptyLocale(key, { ...opts, readonly }),
    [key, opts?.permissionScoped, opts?.description, opts?.actionLabel, opts?.actionPath, opts?.onAction, readonly, language],
  );
}
