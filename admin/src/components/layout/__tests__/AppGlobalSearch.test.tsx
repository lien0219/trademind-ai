import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { MenuDataItem } from '@umijs/route-utils';
import { ConfigProvider } from 'antd';
import { describe, expect, it, vi } from 'vitest';
import AppGlobalSearch, { buildGlobalSearchItems } from '../AppGlobalSearch';

const menuItems: MenuDataItem[] = [
  {
    name: '商品',
    path: '/product',
    children: [
      { name: '商品草稿', path: '/product/drafts' },
      {
        name: '商品详情',
        path: '/product/drafts/:id',
        hideInMenu: true,
      },
    ],
  },
  {
    name: '订单',
    path: '/orders',
    children: [{ name: '订单异常', path: '/orders/exceptions' }],
  },
];

const allowAllPaths = () => true;

describe('AppGlobalSearch', () => {
  it('builds results only from visible, directly navigable menu leaves', () => {
    expect(
      buildGlobalSearchItems(
        menuItems,
        (path) => path !== '/orders/exceptions',
      ),
    ).toEqual([
      {
        path: '/product/drafts',
        title: '商品草稿',
        breadcrumb: '商品 / 商品草稿',
        searchableText: '商品 / 商品草稿 /product/drafts',
      },
    ]);
  });

  it('searches the visible menu and navigates to the selected result', async () => {
    const onNavigate = vi.fn();
    const interaction = userEvent.setup();
    render(
      <ConfigProvider theme={{ token: { motion: false } }}>
        <AppGlobalSearch
          items={menuItems}
          canAccessPath={allowAllPaths}
          onNavigate={onNavigate}
        />
      </ConfigProvider>,
    );

    const trigger = screen.getByRole('button', { name: '搜索功能或页面' });
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    await interaction.click(trigger);
    expect(trigger).toHaveAttribute('aria-expanded', 'true');
    await interaction.type(
      screen.getByRole('textbox', { name: '搜索功能或页面' }),
      '订单异常',
    );
    await interaction.click(
      screen.getByRole('button', { name: /订单异常/ }),
    );

    expect(onNavigate).toHaveBeenCalledWith('/orders/exceptions');
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    expect(
      screen.queryByRole('dialog', { name: '搜索功能' }),
    ).not.toBeInTheDocument();
  });

  it('opens from the global keyboard shortcut', () => {
    render(
      <AppGlobalSearch
        items={menuItems}
        canAccessPath={allowAllPaths}
        onNavigate={vi.fn()}
      />,
    );

    fireEvent.keyDown(window, { key: 'k', ctrlKey: true });

    expect(screen.getByRole('dialog', { name: '搜索功能' })).toBeInTheDocument();
  });
});
