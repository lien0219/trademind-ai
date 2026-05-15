/**
 * Umi/Max 在 IDE 中依赖 .umi 生成物导出 request；在未运行 dev 或路径解析不完整时
 * 会报「@umijs/max 没有导出的成员 request」。此处对 request 做最小类型补充，与运行时一致。
 */
export {};

declare module '@umijs/max' {
  export function request<Response = unknown>(
    url: string,
    options?: {
      method?: string;
      data?: unknown;
      params?: Record<string, string | number | boolean | undefined>;
      headers?: Record<string, string>;
      skipErrorHandler?: boolean;
      /** 其他与 axios / umi-request 兼容的字段 */
      [key: string]: unknown;
    },
  ): Promise<Response>;
}
