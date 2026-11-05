import { useSyncExternalStore } from 'react';
import { readLanguage, subscribeLanguage } from '.';

/** Subscribe at render boundaries; changing language must not remount forms. */
export function useLanguage() {
  return useSyncExternalStore(subscribeLanguage, readLanguage, () => 'zh-CN' as const);
}
