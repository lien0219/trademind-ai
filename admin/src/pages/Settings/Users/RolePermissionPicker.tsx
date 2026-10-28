import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { SearchOutlined } from '@ant-design/icons';
import { Alert, Button, Checkbox, Empty, Input } from 'antd';
import { useMemo, useState } from 'react';
import { localizedPermissionGroups as groupPermissions, localizedPermissionLabel as permissionLabel } from './displayLabels';
import './rolePermissions.less';

type Props = {
  catalog: string[];
  value?: string[];
  onChange?: (value: string[]) => void;
  disabled?: boolean;
};

export default function RolePermissionPicker({ catalog, value = [], onChange, disabled }: Props) {
  const language = useLanguage();
  const [search, setSearch] = useState('');
  const groups = useMemo(() => groupPermissions(catalog), [catalog, language]);
  const selected = new Set(value);
  const keyword = search.trim().toLowerCase();
  const visibleGroups = groups.map((group) => ({
    ...group,
    permissions: group.permissions.filter((key) => !keyword || `${group.label} ${permissionLabel(key)} ${key}`.toLowerCase().includes(keyword)),
  })).filter((group) => group.permissions.length > 0);
  const unavailable = value.filter((key) => !catalog.includes(key)).length;
  const toggle = (keys: string[], checked: boolean) => {
    if (disabled) return;
    const next = new Set(value);
    for (const key of keys) {
      if (checked) next.add(key);
      else next.delete(key);
    }
    onChange?.([...next]);
  };

  return (
    <div className="tm-permission-picker">
      <div className="tm-permission-picker__toolbar">
        <Input aria-label={tr("搜索权限")} placeholder={tr("搜索权限名称或模块")} prefix={<SearchOutlined aria-hidden="true" />} allowClear
          value={search} onChange={(event) => setSearch(event.target.value)} />
        <div className="tm-permission-picker__selection" aria-live="polite">
          <span>{trFormat("已选择 {value0} 项", { value0: value.length })}</span>
          <Button type="link" size="small" disabled={disabled || value.length === 0} onClick={() => onChange?.([])}>{tr("清空选择")}</Button>
        </div>
      </div>
      {unavailable > 0 && <Alert type="warning" showIcon message={trFormat("有 {value0} 项已选权限不在当前目录中，请刷新目录后核对。", { value0: unavailable })} />}
      <div className="tm-permission-picker__groups">
        {visibleGroups.map((group) => {
          const count = group.permissions.filter((key) => selected.has(key)).length;
          return (
            <section key={group.key} aria-label={group.label} className="tm-permission-picker__group">
              <div className="tm-permission-picker__group-header">
                <h4>{group.label}<span>{count} / {group.permissions.length}</span></h4>
                <Checkbox aria-label={trFormat("{value0}：{value1}", { value0: keyword ? tr('全选当前结果') : tr('全选'), value1: group.label })}
                  checked={count === group.permissions.length} indeterminate={count > 0 && count < group.permissions.length}
                  disabled={disabled} onChange={(event) => toggle(group.permissions, event.target.checked)}>{tr("全选")}</Checkbox>
              </div>
              <div className="tm-permission-picker__options">
                {group.permissions.map((key) => (
                  <Checkbox key={key} checked={selected.has(key)} disabled={disabled}
                    onChange={(event) => toggle([key], event.target.checked)}>{permissionLabel(key)}</Checkbox>
                ))}
              </div>
            </section>
          );
        })}
        {!visibleGroups.length && <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={catalog.length ? tr("未找到匹配的权限") : tr("暂无可配置权限")} />}
      </div>
    </div>
  );
}
