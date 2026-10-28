import { useLanguage } from '@/i18n/useLanguage';
import { tr, trFormat } from '@/i18n';
import {
  FileTextOutlined,
  PictureOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  ShopOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons';
import {
  MetricCard,
  OperationToolbar,
  SectionCard,
  TmPageContainer,
  TechnicalDetails,
  TmProTable as ProTable,
} from '@/components/ui';
import type { ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import {
  Alert,
  Button,
  DatePicker,
  Drawer,
  Input,
  Select,
  Space,
  Tag,
  Typography,
  message,
} from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { PLATFORM_OPTIONS } from '@/constants/userFriendly';
import {
  WORKBENCH_PRIORITY_OPTIONS,
  WORKBENCH_SUMMARY_CARDS,
  WORKBENCH_TODO_TYPES,
  workbenchPriorityMeta,
} from '@/constants/aiOperationWorkbench';
import {
  getWorkbenchTodo,
  queryWorkbenchSummary,
  queryWorkbenchTodos,
  refreshWorkbenchTodos,
  type WorkbenchSummary,
  type WorkbenchTodoItem,
} from '@/services/aiOperationWorkbench';
import { queryShops, type ShopListRow } from '@/services/shops';
import { formatDateTime } from '@/utils/formatTime';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
import { useUrlDrawerState, useUrlQueryState } from '@/hooks/useUrlState';
import KeywordSafetyHint from '@/components/common/KeywordSafetyHint';
import {
  KEYWORD_MAX_LENGTH,
  KEYWORD_TOO_LONG_MESSAGE,
  looksLikeSensitiveKeyword,
  normalizeSearchKeyword,
} from '@/utils/keywordSafety';
import './index.less';

const { RangePicker } = DatePicker;

const WORKBENCH_QUERY_KEYS = [
  'type',
  'priority',
  'platform',
  'shopId',
  'keyword',
  'start',
  'end',
  'page',
  'pageSize',
] as const;

const CARD_ICONS: Record<string, React.ReactNode> = {
  aiTextReviewCount: <FileTextOutlined />,
  aiImageReviewCount: <PictureOutlined />,
  publishCheckIssueCount: <SafetyCertificateOutlined />,
  publishTaskIssueCount: <ShopOutlined />,
  todayResolvedCount: <CheckCircleOutlined />,
};

type SummaryCardConfig = (typeof WORKBENCH_SUMMARY_CARDS)[number];

function priorityTag(priority?: string) {
  const meta = workbenchPriorityMeta(priority);
  return (
    <Tag color={meta.color as never} className="tm-ai-workbench__priority-tag">
      {meta.label}
    </Tag>
  );
}

function parseRange(start?: string, end?: string): [Dayjs | null, Dayjs | null] | null {
  if (!start && !end) return null;
  const s = start ? dayjs(start) : null;
  const e = end ? dayjs(end) : null;
  return [s?.isValid() ? s : null, e?.isValid() ? e : null];
}

function parsePositiveInt(value?: string, fallback = 1) {
  const n = Number(value);
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : fallback;
}

function workbenchUrlPatch(input: {
  filterType?: string;
  filterPriority?: string;
  filterPlatform?: string;
  filterShopId?: string;
  filterKeyword?: string;
  dateRange: [Dayjs | null, Dayjs | null] | null;
  tablePage: number;
  tablePageSize: number;
}) {
  const { value: keyword } = normalizeSearchKeyword(input.filterKeyword);
  return {
    type: input.filterType,
    priority: input.filterPriority,
    platform: input.filterPlatform,
    shopId: input.filterShopId,
    keyword,
    start: input.dateRange?.[0] ? input.dateRange[0].startOf('day').toISOString() : undefined,
    end: input.dateRange?.[1] ? input.dateRange[1].endOf('day').toISOString() : undefined,
    page: input.tablePage > 1 ? String(input.tablePage) : undefined,
    pageSize: input.tablePageSize !== 50 ? String(input.tablePageSize) : undefined,
  };
}

function sameWorkbenchUrlPatch(
  next: ReturnType<typeof workbenchUrlPatch>,
  urlState: Record<string, string | undefined>,
) {
  const keys = ['type', 'priority', 'platform', 'shopId', 'keyword', 'start', 'end', 'page', 'pageSize'] as const;
  return keys.every((k) => (next[k] ?? undefined) === (urlState[k] ?? undefined));
}

export default function AIOperationWorkbenchPage() {
  useLanguage();
  const { state: urlState, setState: setUrlState, clearState: clearUrlState } =
    useUrlQueryState<Record<(typeof WORKBENCH_QUERY_KEYS)[number], string | undefined>>(
      WORKBENCH_QUERY_KEYS,
    );
  const todoDrawer = useUrlDrawerState('todo');
  const { id: drawerTodoId, openDrawer: openTodoDrawer, closeDrawer: closeTodoDrawer } = todoDrawer;
  const [summary, setSummary] = useState<WorkbenchSummary | null>(null);
  const [summaryLoading, setSummaryLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [lastRefreshedAt, setLastRefreshedAt] = useState<string>('');
  const [shops, setShops] = useState<ShopListRow[]>([]);

  const [filterType, setFilterType] = useState<string | undefined>(() => urlState.type);
  const [filterPriority, setFilterPriority] = useState<string | undefined>(() => urlState.priority);
  const [filterPlatform, setFilterPlatform] = useState<string | undefined>(() => urlState.platform);
  const [filterShopId, setFilterShopId] = useState<string | undefined>(() => urlState.shopId);
  const [filterKeyword, setFilterKeyword] = useState<string | undefined>(() => urlState.keyword);
  const [keywordSensitive, setKeywordSensitive] = useState(() => looksLikeSensitiveKeyword(urlState.keyword));
  const [dateRange, setDateRange] = useState<[Dayjs | null, Dayjs | null] | null>(() =>
    parseRange(urlState.start, urlState.end),
  );
  const [tablePage, setTablePage] = useState(() => parsePositiveInt(urlState.page, 1));
  const [tablePageSize, setTablePageSize] = useState(() => parsePositiveInt(urlState.pageSize, 50));

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerItem, setDrawerItem] = useState<WorkbenchTodoItem | null>(null);
  const [drawerLoading, setDrawerLoading] = useState(false);

  const tableRef = useRef<{ reload: () => void } | null>(null);
  const emptyLocale = useListEmptyLocale('aiOperationWorkbench');
  const hasActiveFilters = Boolean(
    filterType || filterPriority || filterPlatform || filterShopId || filterKeyword || dateRange?.[0] || dateRange?.[1],
  );

  useEffect(() => {
    setFilterType(urlState.type);
    setFilterPriority(urlState.priority);
    setFilterPlatform(urlState.platform);
    setFilterShopId(urlState.shopId);
    setFilterKeyword(urlState.keyword);
    setKeywordSensitive(looksLikeSensitiveKeyword(urlState.keyword));
    setDateRange(parseRange(urlState.start, urlState.end));
    setTablePage(parsePositiveInt(urlState.page, 1));
    setTablePageSize(parsePositiveInt(urlState.pageSize, 50));
  }, [
    urlState.end,
    urlState.keyword,
    urlState.page,
    urlState.pageSize,
    urlState.platform,
    urlState.priority,
    urlState.shopId,
    urlState.start,
    urlState.type,
  ]);

  useEffect(() => {
    const next = workbenchUrlPatch({
      filterType,
      filterPriority,
      filterPlatform,
      filterShopId,
      filterKeyword,
      dateRange,
      tablePage,
      tablePageSize,
    });
    if (sameWorkbenchUrlPatch(next, urlState)) return;
    const { truncated } = normalizeSearchKeyword(filterKeyword);
    if (truncated) message.warning(tr(KEYWORD_TOO_LONG_MESSAGE));
    setUrlState(
      {
        type: next.type,
        priority: next.priority,
        platform: next.platform,
        shopId: next.shopId,
        keyword: next.keyword,
        start: next.start,
        end: next.end,
        page: next.page,
        pageSize: next.pageSize,
      },
      { replace: true },
    );
  }, [
    dateRange,
    filterKeyword,
    filterPlatform,
    filterPriority,
    filterShopId,
    filterType,
    setUrlState,
    tablePage,
    tablePageSize,
    urlState,
  ]);

  const queryParams = useMemo(() => {
    const params: Record<string, string | undefined> = {
      type: filterType,
      priority: filterPriority,
      platform: filterPlatform,
      shopId: filterShopId,
      keyword: (() => {
        const { value, truncated } = normalizeSearchKeyword(filterKeyword);
        if (truncated) message.warning(tr(KEYWORD_TOO_LONG_MESSAGE));
        return value;
      })(),
    };
    if (dateRange?.[0]) {
      params.start = dateRange[0].startOf('day').toISOString();
    }
    if (dateRange?.[1]) {
      params.end = dateRange[1].endOf('day').toISOString();
    }
    return params;
  }, [filterType, filterPriority, filterPlatform, filterShopId, filterKeyword, dateRange]);

  const loadSummary = useCallback(async () => {
    setSummaryLoading(true);
    try {
      const s = await queryWorkbenchSummary(queryParams);
      setSummary(s);
    } catch (e) {
      message.error(e instanceof Error ? e.message : tr("加载统计失败"));
    } finally {
      setSummaryLoading(false);
    }
  }, [queryParams]);

  useEffect(() => {
    void loadSummary();
  }, [loadSummary]);

  useEffect(() => {
    void queryShops({ page: 1, pageSize: 200 }).then((res) => setShops(res.list ?? []));
  }, []);

  const handleRefresh = async () => {
    setRefreshing(true);
    try {
      const res = await refreshWorkbenchTodos(queryParams);
      setSummary(res.summary);
      setLastRefreshedAt(res.refreshedAt);
      tableRef.current?.reload();
      message.success(tr("待办已刷新"));
    } catch (e) {
      message.error(e instanceof Error ? e.message : tr("刷新失败"));
    } finally {
      setRefreshing(false);
    }
  };

  const handleSummaryAction = (card: SummaryCardConfig) => {
    if ('filterType' in card && card.filterType) {
      setFilterType(card.filterType);
      setTablePage(1);
      tableRef.current?.reload();
    }
    if (card.link) {
      history.push(card.link);
      return;
    }
    void handleRefresh();
  };

  const openDrawer = async (row: WorkbenchTodoItem) => {
    openTodoDrawer(row.id);
    setDrawerOpen(true);
    setDrawerLoading(true);
    setDrawerItem(row);
    try {
      const detail = await getWorkbenchTodo(row.id, queryParams);
      setDrawerItem(detail);
    } catch {
      // keep row snapshot
    } finally {
      setDrawerLoading(false);
    }
  };

  useEffect(() => {
    if (!drawerTodoId) return;
    if (drawerItem?.id === drawerTodoId && drawerOpen) return;
    void (async () => {
      setDrawerOpen(true);
      setDrawerLoading(true);
      try {
        const detail = await getWorkbenchTodo(drawerTodoId, queryParams);
        setDrawerItem(detail);
      } catch (e) {
        message.error(e instanceof Error ? e.message : tr("加载待办详情失败"));
        closeTodoDrawer();
        setDrawerOpen(false);
      } finally {
        setDrawerLoading(false);
      }
    })();
  }, [closeTodoDrawer, drawerItem?.id, drawerOpen, drawerTodoId, queryParams]);

  const columns: ProColumns<WorkbenchTodoItem>[] = [
    {
      title: tr("优先级"),
      dataIndex: 'priority',
      width: 88,
      render: (_, row) => priorityTag(row.priority),
    },
    {
      title: tr("类型"),
      dataIndex: 'typeLabel',
      width: 140,
      ellipsis: true,
    },
    {
      title: tr("商品"),
      dataIndex: 'productTitle',
      width: 220,
      ellipsis: true,
      render: (_, row) =>
        row.productTitle ? (
          <Typography.Text ellipsis={{ tooltip: row.productTitle }} className="tm-ai-workbench__product-title">
            {row.productTitle}
          </Typography.Text>
        ) : (
          '—'
        ),
    },
    {
      title: tr("平台 / 店铺"),
      width: 140,
      ellipsis: true,
      render: (_, row) => {
        const plat = row.platformLabel || row.platform;
        const shop = row.shopName || row.shopId;
        if (!plat && !shop) return '—';
        return [plat, shop].filter(Boolean).join(' · ');
      },
    },
    {
      title: tr("问题"),
      dataIndex: 'title',
      width: 260,
      ellipsis: true,
      render: (_, row) => (
        <Typography.Text ellipsis={{ tooltip: row.message }} className="tm-ai-workbench__issue-title">
          {row.title}
        </Typography.Text>
      ),
    },
    {
      title: tr("建议操作"),
      dataIndex: 'actionLabel',
      width: 100,
    },
    {
      title: tr("更新时间"),
      dataIndex: 'updatedAt',
      width: 168,
      render: (_, row) => formatDateTime(row.updatedAt),
    },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 180,
      render: (_, row) => [
        <Button
          key="act"
          type="link"
          size="small"
          onClick={(event) => {
            event.stopPropagation();
            history.push(row.actionUrl);
          }}
        >
          {row.actionLabel}
        </Button>,
        <Button
          key="detail"
          type="link"
          size="small"
          onClick={(event) => {
            event.stopPropagation();
            void openDrawer(row);
          }}
        >
          {tr("详情")}
        </Button>,
        row.productId ? (
          <Button
            key="product"
            type="link"
            size="small"
            onClick={(event) => {
              event.stopPropagation();
              history.push(`/product/drafts/${row.productId}`);
            }}
          >
            {tr("查看商品")}
          </Button>
        ) : null,
      ],
    },
  ];

  return (
    <TmPageContainer
      className="tm-ai-workbench-page"
      title={tr("AI 商品运营工作台")}
      subTitle={tr("汇总 AI 文案、AI 图片、发布检查、刊登异常与失败任务，统一处理入口")}
      extra={
        <OperationToolbar
          extra={
            lastRefreshedAt ? (
              <Typography.Text type="secondary">{tr("最近刷新：")}{formatDateTime(lastRefreshedAt)}</Typography.Text>
            ) : null
          }
        >
          <Button icon={<ReloadOutlined />} loading={refreshing} onClick={() => void handleRefresh()}>
            {tr("刷新待办")}
          </Button>
        </OperationToolbar>
      }
    >
      <div className="tm-ai-workbench">
        <KeywordSafetyHint visible={keywordSensitive} />

        <div className="tm-ai-workbench__metric-grid">
          {WORKBENCH_SUMMARY_CARDS.map((card) => {
            const count = summary ? Number(summary[card.key as keyof WorkbenchSummary] ?? 0) : 0;
            const high =
              'highKey' in card && card.highKey
                ? Number(summary?.[card.highKey as keyof WorkbenchSummary] ?? 0)
                : 0;
            const todayNew =
              'todayKey' in card && card.todayKey
                ? Number(summary?.[card.todayKey as keyof WorkbenchSummary] ?? 0)
                : 0;
            const description =
              'highKey' in card && card.highKey
                ? trFormat('高优先级 {high}{today}', {
                    high,
                    today: todayNew > 0 ? trFormat(' · 今日新增 {count}', { count: todayNew }) : '',
                  })
                : tr("今日处理完成项");
            return (
              <MetricCard
                key={card.key}
                loading={summaryLoading}
                title={card.title}
                value={
                  <div className="tm-ai-workbench__metric-value">
                    <span>{count}</span>
                    <Button
                      size="small"
                      loading={card.key === 'todayResolvedCount' ? refreshing : false}
                      onClick={() => handleSummaryAction(card)}
                    >
                      {card.actionLabel}
                    </Button>
                  </div>
                }
                icon={CARD_ICONS[card.key]}
                intent={high > 0 ? 'warning' : card.key === 'todayResolvedCount' ? 'success' : 'default'}
                description={
                  <span className={high > 0 ? 'tm-ai-workbench__high-text' : undefined}>{description}</span>
                }
              />
            );
          })}
        </div>

        <Alert
          className="tm-ai-workbench__priority-alert"
          type={Number(summary?.highPriorityCount ?? 0) > 0 ? 'warning' : 'info'}
          showIcon
          message={
            summaryLoading
              ? tr("正在加载高优先级概览")
              : Number(summary?.highPriorityCount ?? 0) > 0
              ? trFormat('当前有 {count} 个高优先级事项', { count: Number(summary?.highPriorityCount ?? 0) })
              : tr("当前没有高优先级事项")
          }
          description={tr("高优先级事项来自现有待办汇总，仅作为处理顺序提示。")}
        />

        <SectionCard
          title={tr("筛选待办")}
          description={
            hasActiveFilters
              ? tr("当前筛选已生效，列表和汇总会按条件更新。")
              : tr("按业务环节、优先级、平台、店铺和时间范围定位待处理事项。")
          }
          headerExtra={
            hasActiveFilters ? (
              <Tag color="blue" className="tm-ai-workbench__active-filter-tag">
                {tr("筛选中")}
              </Tag>
            ) : null
          }
          compact
        >
          <div className="tm-ai-workbench__filters">
            <div className="tm-ai-workbench__filter-item">
              <Typography.Text type="secondary">{tr("待办类型")}</Typography.Text>
              <Select
                allowClear
                placeholder={tr("待办类型")}
                options={WORKBENCH_TODO_TYPES.map((x) => ({ label: x.label, value: x.value }))}
                value={filterType}
                onChange={(v) => {
                  setFilterType(v);
                  setTablePage(1);
                }}
              />
            </div>
            <div className="tm-ai-workbench__filter-item">
              <Typography.Text type="secondary">{tr("优先级")}</Typography.Text>
              <Select
                allowClear
                placeholder={tr("优先级")}
                options={WORKBENCH_PRIORITY_OPTIONS.map((x) => ({ label: x.label, value: x.value }))}
                value={filterPriority}
                onChange={(v) => {
                  setFilterPriority(v);
                  setTablePage(1);
                }}
              />
            </div>
            <div className="tm-ai-workbench__filter-item">
              <Typography.Text type="secondary">{tr("平台")}</Typography.Text>
              <Select
                allowClear
                placeholder={tr("平台")}
                options={PLATFORM_OPTIONS}
                value={filterPlatform}
                onChange={(v) => {
                  setFilterPlatform(v);
                  setTablePage(1);
                }}
              />
            </div>
            <div className="tm-ai-workbench__filter-item">
              <Typography.Text type="secondary">{tr("店铺")}</Typography.Text>
              <Select
                allowClear
                placeholder={tr("店铺")}
                showSearch
                optionFilterProp="label"
                options={shops.map((s) => ({ label: s.shopName, value: s.id }))}
                value={filterShopId}
                onChange={(v) => {
                  setFilterShopId(v);
                  setTablePage(1);
                }}
              />
            </div>
            <div className="tm-ai-workbench__filter-item tm-ai-workbench__filter-item--keyword">
              <Typography.Text type="secondary">{tr("商品关键词")}</Typography.Text>
              <Input.Search
                allowClear
                maxLength={KEYWORD_MAX_LENGTH}
                value={filterKeyword}
                placeholder={tr("商品关键词")}
                onChange={(e) => {
                  setFilterKeyword(e.target.value);
                  setKeywordSensitive(looksLikeSensitiveKeyword(e.target.value));
                }}
                onClear={() => {
                  setFilterKeyword(undefined);
                  setKeywordSensitive(false);
                  setTablePage(1);
                  setUrlState({ keyword: undefined, page: undefined }, { replace: true });
                  tableRef.current?.reload();
                }}
                onSearch={(v) => {
                  const { value, truncated } = normalizeSearchKeyword(v);
                  if (truncated) message.warning(tr(KEYWORD_TOO_LONG_MESSAGE));
                  setFilterKeyword(value);
                  setKeywordSensitive(looksLikeSensitiveKeyword(value));
                  setTablePage(1);
                  tableRef.current?.reload();
                }}
              />
            </div>
            <div className="tm-ai-workbench__filter-item tm-ai-workbench__filter-item--range">
              <Typography.Text type="secondary">{tr("日期范围")}</Typography.Text>
              <RangePicker
                value={dateRange}
                onChange={(v) => {
                  setDateRange(v);
                  setTablePage(1);
                }}
              />
            </div>
            <div className="tm-ai-workbench__filter-item tm-ai-workbench__filter-item--action">
              <Button
                onClick={() => {
                  setFilterType(undefined);
                  setFilterPriority(undefined);
                  setFilterPlatform(undefined);
                  setFilterShopId(undefined);
                  setFilterKeyword(undefined);
                  setDateRange(null);
                  setTablePage(1);
                  setTablePageSize(50);
                  closeTodoDrawer();
                  setDrawerOpen(false);
                  setDrawerItem(null);
                  clearUrlState(WORKBENCH_QUERY_KEYS, { replace: true });
                  tableRef.current?.reload();
                }}
              >
                {tr("重置")}
              </Button>
            </div>
          </div>
        </SectionCard>

        <SectionCard
          title={tr("待办列表")}
          description={tr("点击行查看详情，使用操作入口进入对应业务页面处理。")}
          compact
        >
          <ProTable<WorkbenchTodoItem>
            actionRef={tableRef as never}
            rowKey="id"
            search={false}
            options={false}
            scroll={{ x: 1120 }}
            pagination={{
              current: tablePage,
              pageSize: tablePageSize,
              showSizeChanger: true,
              pageSizeOptions: ['20', '50'],
              onChange: (page, pageSize) => {
                setTablePage(page);
                setTablePageSize(pageSize);
              },
            }}
            columns={columns}
            onRow={(row) => ({
              onClick: () => void openDrawer(row),
              className: row.priority === 'P0' || row.priority === 'P1' ? 'tm-ai-workbench__row--high' : undefined,
            })}
            request={async (params) => {
              try {
                const res = await queryWorkbenchTodos({
                  ...queryParams,
                  page: params.current || tablePage,
                  pageSize: params.pageSize || tablePageSize,
                });
                return {
                  data: res.items,
                  total: res.pagination.total,
                  success: true,
                };
              } catch (e) {
                message.error(e instanceof Error ? e.message : tr("加载待办失败"));
                return { data: [], total: 0, success: false };
              }
            }}
            locale={emptyLocale}
          />
        </SectionCard>
      </div>

      <Drawer
        title={tr("待办详情")}
        width="min(560px, calc(100vw - 32px))"
        rootClassName="tm-ai-workbench-drawer"
        open={drawerOpen}
        onClose={() => {
          setDrawerOpen(false);
          setDrawerItem(null);
          closeTodoDrawer();
        }}
        loading={drawerLoading}
        extra={
          drawerItem?.actionUrl ? (
            <Button type="primary" onClick={() => history.push(drawerItem.actionUrl)}>
              {drawerItem.actionLabel}
            </Button>
          ) : null
        }
      >
        {drawerItem ? (
          <Space direction="vertical" size="middle" className="tm-ai-workbench-drawer__body">
            <div>
              <Typography.Text type="secondary">{tr("类型")}</Typography.Text>
              <div>{drawerItem.typeLabel}</div>
            </div>
            <div>
              <Typography.Text type="secondary">{tr("优先级")}</Typography.Text>
              <div>{priorityTag(drawerItem.priority)}</div>
            </div>
            {drawerItem.productTitle ? (
              <div>
                <Typography.Text type="secondary">{tr("商品")}</Typography.Text>
                <div className="tm-ai-workbench-drawer__text">{drawerItem.productTitle}</div>
              </div>
            ) : null}
            <div>
              <Typography.Text type="secondary">{tr("问题")}</Typography.Text>
              <Typography.Paragraph className="tm-ai-workbench-drawer__text">{drawerItem.title}</Typography.Paragraph>
              <Typography.Paragraph type="secondary" className="tm-ai-workbench-drawer__text">
                {drawerItem.message}
              </Typography.Paragraph>
            </div>
            <div>
              <Typography.Text type="secondary">{tr("问题来源")}</Typography.Text>
              <div>{drawerItem.typeLabel}</div>
            </div>
            <div>
              <Typography.Text type="secondary">{tr("影响范围")}</Typography.Text>
              <Typography.Paragraph className="tm-ai-workbench-drawer__text">
                {drawerItem.productTitle
                  ? trFormat('关联商品：{title}', { title: drawerItem.productTitle })
                  : tr("可能影响批量刊登或系统任务处理进度")}
              </Typography.Paragraph>
            </div>
            <div>
              <Typography.Text type="secondary">{tr("建议操作")}</Typography.Text>
              <Typography.Paragraph className="tm-ai-workbench-drawer__text">
                {drawerItem.actionLabel}：{drawerItem.message}
              </Typography.Paragraph>
            </div>
            <TechnicalDetails label={tr("技术详情")}>
              <Space direction="vertical" size={4}>
                <div>{tr("待办编号：")}{drawerItem.id}</div>
                <div>{tr("来源类型：")}{drawerItem.sourceType}</div>
                <div>{tr("来源编号：")}{drawerItem.sourceId}</div>
                {drawerItem.issueCode ? <div>{tr("问题代码：")}{drawerItem.issueCode}</div> : null}
                {drawerItem.technicalDetails
                  ? Object.entries(drawerItem.technicalDetails).map(([k, v]) => (
                      <div key={k}>
                        {k}：{typeof v === 'object' ? JSON.stringify(v) : String(v)}
                      </div>
                    ))
                  : null}
              </Space>
            </TechnicalDetails>
          </Space>
        ) : null}
      </Drawer>
    </TmPageContainer>
  );
}
