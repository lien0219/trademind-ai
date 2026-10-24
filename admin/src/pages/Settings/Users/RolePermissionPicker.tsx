import { SearchOutlined } from '@ant-design/icons';
import { Alert, Button, Checkbox, Empty, Input } from 'antd';
import { useMemo, useState } from 'react';
import { groupPermissions, permissionLabel } from '@/constants/permissionLabels';
import './rolePermissions.less';

type Props = {
  catalog: string[];
  value?: string[];
  onChange?: (value: string[]) => void;
  disabled?: boolean;
};

export default function RolePermissionPicker({ catalog, value = [], onChange, disabled }: Props) {
  const [search, setSearch] = useState('');
  const groups = useMemo(() => groupPermissions(catalog), [catalog]);
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
        <Input aria-label="搜索权限" placeholder="搜索权限名称或模块" prefix={<SearchOutlined aria-hidden="true" />} allowClear
          value={search} onChange={(event) => setSearch(event.target.value)} />
        <div className="tm-permission-picker__selection" aria-live="polite">
          <span>已选择 {value.length} 项</span>
          <Button type="link" size="small" disabled={disabled || value.length === 0} onClick={() => onChange?.([])}>清空选择</Button>
        </div>
      </div>
      {unavailable > 0 && <Alert type="warning" showIcon message={`有 ${unavailable} 项已选权限不在当前目录中，请刷新目录后核对。`} />}
      <div className="tm-permission-picker__groups">
        {visibleGroups.map((group) => {
          const count = group.permissions.filter((key) => selected.has(key)).length;
          return (
            <section key={group.key} aria-label={group.label} className="tm-permission-picker__group">
              <div className="tm-permission-picker__group-header">
                <h4>{group.label}<span>{count} / {group.permissions.length}</span></h4>
                <Checkbox aria-label={`${keyword ? '全选当前结果' : '全选'}：${group.label}`}
                  checked={count === group.permissions.length} indeterminate={count > 0 && count < group.permissions.length}
                  disabled={disabled} onChange={(event) => toggle(group.permissions, event.target.checked)}>全选</Checkbox>
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
        {!visibleGroups.length && <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={catalog.length ? '未找到匹配的权限' : '暂无可配置权限'} />}
      </div>
    </div>
  );
}
