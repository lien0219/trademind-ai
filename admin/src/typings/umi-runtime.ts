/**
 * Umi runtime 配置类型（与 src/app.tsx 一致）。
 * 不直接从 @umijs/max 导入：未运行 max setup 时官方包可能缺少这些导出声明。
 */

export type RequestConfig = {
  requestInterceptors?: Array<
    (
      url: string,
      options: Record<string, unknown>,
    ) =>
      | { url: string; options: Record<string, unknown> }
      | Promise<{ url: string; options: Record<string, unknown> }>
  >;
  responseInterceptors?: unknown[];
  errorConfig?: {
    errorHandler?: (error: unknown) => void;
    errorThrower?: (response: unknown) => void;
  };
  [key: string]: unknown;
};

export type RunTimeLayoutConfig = (initData: {
  initialState?: { currentUser?: API.CurrentUser };
  [key: string]: unknown;
}) => Record<string, unknown>;

/** 与 app.tsx `getInitialState` 返回值一致 */
export type InitialState = {
  currentUser?: API.CurrentUser;
};

/** Umi `@@initialState` model 形状 */
export type InitialStateModel = {
  initialState?: InitialState;
  setInitialState: (
    updater: InitialState | ((state: InitialState) => InitialState),
  ) => Promise<void>;
  refresh?: () => Promise<InitialState>;
};
