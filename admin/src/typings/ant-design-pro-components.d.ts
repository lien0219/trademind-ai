/**
 * @ant-design/pro-card 的 CardProps 未包含 antd Card 的 variant；
 * 业务侧统一写 variant="outlined"，此处补充类型，避免 IDE 报错。
 */
export {};

type ProCardVariant = 'borderless' | 'outlined' | 'filled';

type ProCardPropsWithVariant = import('@ant-design/pro-components').ProCardProps & {
  variant?: ProCardVariant;
};

declare module '@ant-design/pro-components' {
  import type { FC } from 'react';

  export const ProCard: FC<ProCardPropsWithVariant> & {
    isProCard: boolean;
    Divider: FC<Record<string, unknown>>;
    TabPane: FC<Record<string, unknown>>;
    Group: FC<ProCardPropsWithVariant>;
  };
}
