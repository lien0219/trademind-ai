import { fireEvent, render, screen } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { PERMISSIONS } from '@/utils/permission';
import RolePermissionPicker from '../RolePermissionPicker';

const catalog = [PERMISSIONS.PRODUCT_VIEW, PERMISSIONS.PRODUCT_WRITE, PERMISSIONS.ORDER_VIEW];

function setup(initial: string[] = [], disabled = false) {
  const onChange = vi.fn();
  function Harness() {
    const [value, setValue] = useState(initial);
    return <RolePermissionPicker catalog={catalog} value={value} disabled={disabled} onChange={(next) => {
      onChange(next);
      setValue(next);
    }} />;
  }
  render(<Harness />);
  return onChange;
}

describe('RolePermissionPicker', () => {
  it('keeps hidden selections when adding permissions through Chinese search results', () => {
    const onChange = setup([PERMISSIONS.ORDER_VIEW]);
    fireEvent.change(screen.getByRole('textbox', { name: '搜索权限' }), { target: { value: '编辑商品' } });
    expect(screen.queryByRole('checkbox', { name: '查看订单' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('checkbox', { name: '编辑商品' }));
    expect(onChange).toHaveBeenLastCalledWith([PERMISSIONS.ORDER_VIEW, PERMISSIONS.PRODUCT_WRITE]);
    expect(screen.getByText('已选择 2 项')).toBeInTheDocument();
    fireEvent.change(screen.getByRole('textbox', { name: '搜索权限' }), { target: { value: '' } });
    expect(screen.getByRole('checkbox', { name: '查看订单' })).toBeChecked();
  });

  it('limits group selection to the displayed results and retains other permissions', () => {
    const onChange = setup([PERMISSIONS.ORDER_VIEW]);
    fireEvent.click(screen.getByRole('checkbox', { name: '全选：商品管理' }));
    expect(onChange).toHaveBeenLastCalledWith([PERMISSIONS.ORDER_VIEW, PERMISSIONS.PRODUCT_VIEW, PERMISSIONS.PRODUCT_WRITE]);
    fireEvent.change(screen.getByRole('textbox', { name: '搜索权限' }), { target: { value: '编辑商品' } });
    fireEvent.click(screen.getByRole('checkbox', { name: '全选当前结果：商品管理' }));
    expect(onChange).toHaveBeenLastCalledWith([PERMISSIONS.ORDER_VIEW, PERMISSIONS.PRODUCT_VIEW]);
  });

  it('preserves unavailable selections until the user explicitly clears them', () => {
    const onChange = setup(['future.experimental']);
    expect(screen.getByText('有 1 项已选权限不在当前目录中，请刷新目录后核对。')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('checkbox', { name: '查看商品' }));
    expect(onChange).toHaveBeenLastCalledWith(['future.experimental', PERMISSIONS.PRODUCT_VIEW]);
    fireEvent.click(screen.getByRole('button', { name: '清空选择' }));
    expect(onChange).toHaveBeenLastCalledWith([]);
  });

  it('disables every selection control while submitting', () => {
    const onChange = setup([PERMISSIONS.ORDER_VIEW], true);
    for (const checkbox of screen.getAllByRole('checkbox')) expect(checkbox).toBeDisabled();
    expect(screen.getByRole('button', { name: '清空选择' })).toBeDisabled();
    expect(onChange).not.toHaveBeenCalled();
  });

  it('distinguishes an empty directory from a search with no matches', () => {
    const view = render(<RolePermissionPicker catalog={[]} value={[]} />);
    expect(screen.getByText('暂无可配置权限')).toBeInTheDocument();
    view.rerender(<RolePermissionPicker catalog={catalog} value={[]} />);
    fireEvent.change(screen.getByRole('textbox', { name: '搜索权限' }), { target: { value: '无匹配内容' } });
    expect(screen.getByText('未找到匹配的权限')).toBeInTheDocument();
    expect(screen.queryByText('暂无可配置权限')).not.toBeInTheDocument();
  });
});
