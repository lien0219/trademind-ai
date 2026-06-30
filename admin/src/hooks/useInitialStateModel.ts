import { useModel } from '@umijs/max';
import type { InitialStateModel } from '@/typings/umi-runtime';

/** 类型安全的 Umi `@@initialState` model（避免 useModel 默认返回 unknown）。 */
export function useInitialStateModel(): InitialStateModel {
  return useModel('@@initialState') as InitialStateModel;
}
