import { ReloadOutlined } from '@ant-design/icons';
import { ProTable } from '@ant-design/pro-components';
import type { ActionType, ProTableProps } from '@ant-design/pro-components';
import { Button, Tooltip } from 'antd';
import { useCallback, useMemo, useRef, useState } from 'react';

export type TmProTableProps<T extends Record<string, unknown>, U extends Record<string, unknown> = Record<string, unknown>> =
  ProTableProps<T, U>;

/**
 * 统一 ProTable：用可点击的 Button 承接刷新（修复工具栏内置 span 图标在某些布局下点击无效的问题）。
 */
export default function TmProTable<
  T extends Record<string, unknown>,
  U extends Record<string, unknown> = Record<string, unknown>,
>({ actionRef: userActionRef, options, toolBarRender, onLoadingChange, ...rest }: TmProTableProps<T, U>) {
  const innerRef = useRef<ActionType>();
  const actionRef = userActionRef ?? innerRef;
  const [loading, setLoading] = useState(false);

  const mergedOptions = useMemo(() => {
    if (options === false) {
      return false;
    }
    const base = options === true || options === undefined ? {} : options;
    return {
      density: true,
      setting: true,
      ...base,
      // 内置 reload 为 span+图标，点击区域易失效；改由 toolBarRender 中的 Button 触发。
      reload: false,
    };
  }, [options]);

  const mergedToolBarRender = useCallback(
    (action: ActionType | undefined, config: Parameters<NonNullable<ProTableProps<T, U>['toolBarRender']>>[1]) => {
      const userNodes = toolBarRender?.(action, config) ?? [];
      if (mergedOptions === false) {
        return userNodes;
      }
      return [
        ...userNodes,
        <Tooltip key="tm-reload" title="刷新">
          <Button
            type="text"
            aria-label="刷新"
            icon={<ReloadOutlined spin={loading} />}
            onClick={() => {
              void action?.reload?.();
            }}
          />
        </Tooltip>,
      ];
    },
    [toolBarRender, loading, mergedOptions],
  );

  return (
    <ProTable<T, U>
      {...rest}
      actionRef={actionRef}
      options={mergedOptions}
      toolBarRender={mergedToolBarRender}
      onLoadingChange={(isLoading) => {
        setLoading(!!isLoading);
        onLoadingChange?.(isLoading);
      }}
    />
  );
}
