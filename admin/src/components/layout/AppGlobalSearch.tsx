import { SearchOutlined } from '@ant-design/icons';
import { tr } from '@/i18n';
import type { MenuDataItem } from '@umijs/route-utils';
import { Empty, Input, Modal } from 'antd';
import type { InputRef } from 'antd';
import { useEffect, useMemo, useRef, useState } from 'react';
import './AppGlobalSearch.less';

const MAX_SEARCH_RESULTS = 10;

export type GlobalSearchItem = {
  path: string;
  title: string;
  breadcrumb: string;
  searchableText: string;
};

type AppGlobalSearchProps = {
  items: MenuDataItem[];
  compact?: boolean;
  canAccessPath: (path: string) => boolean;
  onNavigate: (path: string) => void;
};

function visibleChildren(item: MenuDataItem): MenuDataItem[] {
  const children: MenuDataItem[] = item.children?.length
    ? item.children
    : item.routes || [];
  return children.filter((child) => !child.hideInMenu);
}

export function buildGlobalSearchItems(
  menuItems: MenuDataItem[],
  canAccessPath: (path: string) => boolean,
): GlobalSearchItem[] {
  const searchItems = new Map<string, GlobalSearchItem>();

  const visit = (items: MenuDataItem[], parents: string[]) => {
    items.forEach((item) => {
      if (item.hideInMenu) return;

      const title = typeof item.name === 'string' ? item.name.trim() : '';
      const nextParents = title ? [...parents, title] : parents;
      const children = visibleChildren(item);

      if (children.length > 0) {
        visit(children, nextParents);
        return;
      }

      const path = typeof item.path === 'string' ? item.path : '';
      const isSearchablePath =
        path.startsWith('/') &&
        path !== '/' &&
        !path.includes(':') &&
        !path.includes('*');

      if (
        !title ||
        !isSearchablePath ||
        !canAccessPath(path) ||
        searchItems.has(path)
      ) {
        return;
      }

      const breadcrumb = nextParents.join(' / ');
      searchItems.set(path, {
        path,
        title,
        breadcrumb,
        searchableText: `${breadcrumb} ${path}`.toLocaleLowerCase('zh-CN'),
      });
    });
  };

  visit(menuItems, []);
  return [...searchItems.values()];
}

export default function AppGlobalSearch({
  items,
  compact = false,
  canAccessPath,
  onNavigate,
}: AppGlobalSearchProps) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const inputRef = useRef<InputRef>(null);
  const searchItems = useMemo(
    () => buildGlobalSearchItems(items, canAccessPath),
    [canAccessPath, items],
  );
  const normalizedQuery = query.trim().toLocaleLowerCase('zh-CN');
  const results = useMemo(
    () =>
      normalizedQuery
        ? searchItems
            .filter((item) => item.searchableText.includes(normalizedQuery))
            .slice(0, MAX_SEARCH_RESULTS)
        : [],
    [normalizedQuery, searchItems],
  );

  useEffect(() => {
    const openWithShortcut = (event: KeyboardEvent) => {
      if (
        (event.ctrlKey || event.metaKey) &&
        event.key.toLocaleLowerCase('en-US') === 'k'
      ) {
        event.preventDefault();
        setOpen(true);
      }
    };

    window.addEventListener('keydown', openWithShortcut);
    return () => window.removeEventListener('keydown', openWithShortcut);
  }, []);

  const closeSearch = () => {
    setOpen(false);
    setQuery('');
  };

  const navigateTo = (path: string) => {
    closeSearch();
    onNavigate(path);
  };

  return (
    <div
      className={`tm-app-global-search${compact ? ' tm-app-global-search--compact' : ''}`}
    >
      <button
        type="button"
        className="tm-app-global-search__trigger"
        aria-label={tr("搜索功能或页面")}
        aria-keyshortcuts="Control+K Meta+K"
        title={compact ? tr("搜索功能或页面") : undefined}
        onClick={() => setOpen(true)}
      >
        <SearchOutlined aria-hidden="true" />
        <span className="tm-app-global-search__trigger-label">{tr("搜索功能或页面")}</span>
        <kbd className="tm-app-global-search__shortcut">Ctrl K</kbd>
      </button>

      <Modal
        className="tm-app-global-search-modal"
        title={tr("搜索功能")}
        open={open}
        footer={null}
        width={560}
        onCancel={closeSearch}
        afterOpenChange={(isOpen) => {
          if (isOpen) inputRef.current?.focus();
        }}
      >
        <Input
          ref={inputRef}
          allowClear
          size="large"
          prefix={<SearchOutlined aria-hidden="true" />}
          aria-label={tr("搜索功能或页面")}
          placeholder={tr("输入功能名称，例如：商品草稿、订单异常")}
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onPressEnter={() => {
            const firstResult = results[0];
            if (firstResult) navigateTo(firstResult.path);
          }}
        />

        <div className="tm-app-global-search-modal__content">
          {normalizedQuery ? (
            results.length > 0 ? (
              <div
                className="tm-app-global-search-modal__results"
                aria-label={tr("搜索结果")}
              >
                {results.map((item) => (
                  <button
                    type="button"
                    className="tm-app-global-search-modal__result"
                    key={item.path}
                    onClick={() => navigateTo(item.path)}
                  >
                    <span className="tm-app-global-search-modal__result-title">
                      {item.title}
                    </span>
                    <span className="tm-app-global-search-modal__result-path">
                      {item.breadcrumb}
                    </span>
                  </button>
                ))}
              </div>
            ) : (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="未找到匹配功能"
              />
            )
          ) : (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="输入关键词搜索当前账号可访问的导航功能"
            />
          )}
        </div>
      </Modal>
    </div>
  );
}
