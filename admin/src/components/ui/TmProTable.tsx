import { ColumnHeightOutlined, ReloadOutlined, SettingOutlined } from '@ant-design/icons';
import { ProTable } from '@ant-design/pro-components';
import type { ActionType, ProTableProps } from '@ant-design/pro-components';
import { Button, Tooltip } from 'antd';
import type { ReactNode } from 'react';
import { useCallback, useMemo, useRef, useState } from 'react';

export type TmProTableProps<
  T extends Record<string, unknown>,
  U extends Record<string, unknown> = Record<string, unknown>,
> = ProTableProps<T, U> & {
  /** Search and filter controls rendered on the left side of the table toolbar. */
  filterBar?: ReactNode;
};

type TmToolBarRender<T extends Record<string, unknown>, U extends Record<string, unknown>> = Exclude<
  ProTableProps<T, U>['toolBarRender'],
  false | undefined
>;

/**
 * 统一 ProTable：用可点击的 Button 承接刷新（修复工具栏内置 span 图标在某些布局下点击无效的问题）。
 */
export default function TmProTable<
  T extends Record<string, unknown>,
  U extends Record<string, unknown> = Record<string, unknown>,
>({
  actionRef: userActionRef,
  filterBar,
  headerTitle,
  options,
  toolBarRender,
  onLoadingChange,
  className,
  ...rest
}: TmProTableProps<T, U>) {
  const innerRef = useRef<ActionType>();
  const actionRef = userActionRef ?? innerRef;
  const [loading, setLoading] = useState(false);

  const mergedOptions = useMemo(() => {
    if (options === false) {
      return false;
    }
    const base = options ?? {};
    const setting = base.setting === false
      ? false
      : {
          ...(typeof base.setting === 'object' ? base.setting : {}),
          settingIcon:
            (typeof base.setting === 'object' ? base.setting.settingIcon : undefined) ?? (
              <Button type="text" aria-label="列设置" icon={<SettingOutlined />} />
            ),
        };
    return {
      density: true,
      ...base,
      densityIcon: base.densityIcon ?? (
        <Button type="text" aria-label="表格密度" icon={<ColumnHeightOutlined />} />
      ),
      setting,
      // 内置 reload 为 span+图标，点击区域易失效；改由 toolBarRender 中的 Button 触发。
      reload: false,
    };
  }, [options]);

  const mergedToolBarRender = useCallback(
    (action: ActionType | undefined, config: Parameters<TmToolBarRender<T, U>>[1]) => {
      const userNodes = typeof toolBarRender === 'function' ? toolBarRender(action, config) ?? [] : [];
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
      className={[
        'tm-pro-table',
        filterBar ? 'tm-pro-table--has-filter-bar' : undefined,
        className,
      ]
        .filter(Boolean)
        .join(' ')}
      actionRef={actionRef}
      headerTitle={
        filterBar ? (
          <div className="tm-table-filter-bar">
            {headerTitle === false ? null : headerTitle}
            {filterBar}
          </div>
        ) : (
          headerTitle
        )
      }
      options={mergedOptions}
      toolBarRender={mergedToolBarRender}
      onLoadingChange={(isLoading) => {
        setLoading(!!isLoading);
        onLoadingChange?.(isLoading);
      }}
    />
  );
}
