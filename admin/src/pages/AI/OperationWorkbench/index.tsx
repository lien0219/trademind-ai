import {
  FileTextOutlined,
  PictureOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  ShopOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons';
import { ProCard } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails, TmProTable as ProTable } from '@/components/ui';
import type { ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import {
  Button,
  Col,
  DatePicker,
  Drawer,
  Empty,
  Input,
  Row,
  Select,
  Space,
  Statistic,
  Tag,
  Typography,
  message,
} from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { layoutTokens } from '@/constants/layoutTokens';
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

const { RangePicker } = DatePicker;

const CARD_ICONS: Record<string, React.ReactNode> = {
  aiTextReviewCount: <FileTextOutlined />,
  aiImageReviewCount: <PictureOutlined />,
  publishCheckIssueCount: <SafetyCertificateOutlined />,
  publishTaskIssueCount: <ShopOutlined />,
  todayResolvedCount: <CheckCircleOutlined />,
};

function priorityTag(priority?: string) {
  const meta = workbenchPriorityMeta(priority);
  return (
    <Tag color={meta.color as never} style={{ margin: 0 }}>
      {meta.label}
    </Tag>
  );
}

export default function AIOperationWorkbenchPage() {
  const [summary, setSummary] = useState<WorkbenchSummary | null>(null);
  const [summaryLoading, setSummaryLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [lastRefreshedAt, setLastRefreshedAt] = useState<string>('');
  const [shops, setShops] = useState<ShopListRow[]>([]);

  const [filterType, setFilterType] = useState<string>();
  const [filterPriority, setFilterPriority] = useState<string>();
  const [filterPlatform, setFilterPlatform] = useState<string>();
  const [filterShopId, setFilterShopId] = useState<string>();
  const [filterKeyword, setFilterKeyword] = useState<string>();
  const [dateRange, setDateRange] = useState<[Dayjs | null, Dayjs | null] | null>(null);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerItem, setDrawerItem] = useState<WorkbenchTodoItem | null>(null);
  const [drawerLoading, setDrawerLoading] = useState(false);

  const tableRef = useRef<{ reload: () => void } | null>(null);

  const queryParams = useMemo(() => {
    const params: Record<string, string | undefined> = {
      type: filterType,
      priority: filterPriority,
      platform: filterPlatform,
      shopId: filterShopId,
      keyword: filterKeyword?.trim() || undefined,
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
      message.error(e instanceof Error ? e.message : '加载统计失败');
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
      message.success('待办已刷新');
    } catch (e) {
      message.error(e instanceof Error ? e.message : '刷新失败');
    } finally {
      setRefreshing(false);
    }
  };

  const openDrawer = async (row: WorkbenchTodoItem) => {
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

  const columns: ProColumns<WorkbenchTodoItem>[] = [
    {
      title: '优先级',
      dataIndex: 'priority',
      width: 88,
      render: (_, row) => priorityTag(row.priority),
    },
    {
      title: '类型',
      dataIndex: 'typeLabel',
      width: 140,
      ellipsis: true,
    },
    {
      title: '商品',
      dataIndex: 'productTitle',
      ellipsis: true,
      render: (_, row) => row.productTitle || '—',
    },
    {
      title: '平台 / 店铺',
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
      title: '问题',
      dataIndex: 'title',
      ellipsis: true,
      render: (_, row) => (
        <Typography.Text ellipsis={{ tooltip: row.message }}>
          {row.title}
        </Typography.Text>
      ),
    },
    {
      title: '建议操作',
      dataIndex: 'actionLabel',
      width: 100,
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      width: 168,
      render: (_, row) => formatDateTime(row.updatedAt),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 180,
      render: (_, row) => [
        <Button key="act" type="link" size="small" onClick={() => history.push(row.actionUrl)}>
          {row.actionLabel}
        </Button>,
        <Button key="detail" type="link" size="small" onClick={() => void openDrawer(row)}>
          详情
        </Button>,
        row.productId ? (
          <Button
            key="product"
            type="link"
            size="small"
            onClick={() => history.push(`/product/drafts/${row.productId}`)}
          >
            查看商品
          </Button>
        ) : null,
      ],
    },
  ];

  return (
    <TmPageContainer
      title="AI 商品运营工作台"
      subTitle="汇总 AI 文案、AI 图片、发布检查、刊登异常与失败任务，统一处理入口"
      extra={
        <Space wrap>
          {lastRefreshedAt ? (
            <Typography.Text type="secondary">最近刷新：{formatDateTime(lastRefreshedAt)}</Typography.Text>
          ) : null}
          <Button icon={<ReloadOutlined />} loading={refreshing} onClick={() => void handleRefresh()}>
            刷新待办
          </Button>
        </Space>
      }
    >
      <Row gutter={[16, 16]} style={{ marginBottom: layoutTokens.sectionGap }}>
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
          return (
            <Col xs={24} sm={12} lg={8} xl={24 / 5} key={card.key}>
              <ProCard bordered loading={summaryLoading} style={{ height: '100%' }}>
                <Space direction="vertical" size={4} style={{ width: '100%' }}>
                  <Space>
                    {CARD_ICONS[card.key]}
                    <Typography.Text type="secondary">{card.title}</Typography.Text>
                  </Space>
                  <Statistic value={count} valueStyle={{ fontSize: 28 }} />
                  {'highKey' in card && card.highKey ? (
                    <Typography.Text type={high > 0 ? 'danger' : 'secondary'}>
                      其中 {high} 个高优先级
                      {todayNew > 0 ? ` · 今日新增 ${todayNew}` : ''}
                    </Typography.Text>
                  ) : (
                    <Typography.Text type="secondary">今日处理完成项</Typography.Text>
                  )}
                  {card.link ? (
                    <Button
                      type="primary"
                      size="small"
                      onClick={() => {
                        if ('filterType' in card && card.filterType) {
                          setFilterType(card.filterType);
                          tableRef.current?.reload();
                        }
                        if (card.link) history.push(card.link);
                      }}
                    >
                      {card.actionLabel}
                    </Button>
                  ) : (
                    <Button size="small" onClick={() => void handleRefresh()}>
                      {card.actionLabel}
                    </Button>
                  )}
                </Space>
              </ProCard>
            </Col>
          );
        })}
      </Row>

      <ProCard bordered title="筛选" style={{ marginBottom: layoutTokens.sectionGap }}>
        <Row gutter={[12, 12]}>
          <Col xs={24} sm={12} md={8} lg={6}>
            <Select
              allowClear
              placeholder="待办类型"
              style={{ width: '100%' }}
              options={WORKBENCH_TODO_TYPES.map((x) => ({ label: x.label, value: x.value }))}
              value={filterType}
              onChange={setFilterType}
            />
          </Col>
          <Col xs={24} sm={12} md={8} lg={6}>
            <Select
              allowClear
              placeholder="优先级"
              style={{ width: '100%' }}
              options={WORKBENCH_PRIORITY_OPTIONS.map((x) => ({ label: x.label, value: x.value }))}
              value={filterPriority}
              onChange={setFilterPriority}
            />
          </Col>
          <Col xs={24} sm={12} md={8} lg={6}>
            <Select
              allowClear
              placeholder="平台"
              style={{ width: '100%' }}
              options={PLATFORM_OPTIONS}
              value={filterPlatform}
              onChange={setFilterPlatform}
            />
          </Col>
          <Col xs={24} sm={12} md={8} lg={6}>
            <Select
              allowClear
              placeholder="店铺"
              style={{ width: '100%' }}
              showSearch
              optionFilterProp="label"
              options={shops.map((s) => ({ label: s.shopName, value: s.id }))}
              value={filterShopId}
              onChange={setFilterShopId}
            />
          </Col>
          <Col xs={24} sm={12} md={8} lg={6}>
            <Input.Search
              allowClear
              placeholder="商品关键词"
              onSearch={(v) => {
                setFilterKeyword(v);
                tableRef.current?.reload();
              }}
            />
          </Col>
          <Col xs={24} sm={12} md={8} lg={8}>
            <RangePicker
              style={{ width: '100%' }}
              value={dateRange}
              onChange={(v) => setDateRange(v)}
            />
          </Col>
          <Col xs={24} sm={12} md={8} lg={4}>
            <Button
              onClick={() => {
                setFilterType(undefined);
                setFilterPriority(undefined);
                setFilterPlatform(undefined);
                setFilterShopId(undefined);
                setFilterKeyword(undefined);
                setDateRange(null);
                tableRef.current?.reload();
              }}
            >
              重置
            </Button>
          </Col>
        </Row>
      </ProCard>

      <ProTable<WorkbenchTodoItem>
        actionRef={tableRef as never}
        rowKey="id"
        search={false}
        options={false}
        pagination={{ defaultPageSize: 50, showSizeChanger: true, pageSizeOptions: ['20', '50'] }}
        columns={columns}
        onRow={(row) => ({
          onClick: () => void openDrawer(row),
          style: { cursor: 'pointer' },
        })}
        request={async (params) => {
          try {
            const res = await queryWorkbenchTodos({
              ...queryParams,
              page: params.current,
              pageSize: params.pageSize,
            });
            return {
              data: res.items,
              total: res.pagination.total,
              success: true,
            };
          } catch (e) {
            message.error(e instanceof Error ? e.message : '加载待办失败');
            return { data: [], total: 0, success: false };
          }
        }}
        locale={{ emptyText: <Empty description="暂无待办，商品运营状态良好" /> }}
      />

      <Drawer
        title="待办详情"
        width={480}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
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
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <div>
              <Typography.Text type="secondary">类型</Typography.Text>
              <div>{drawerItem.typeLabel}</div>
            </div>
            <div>
              <Typography.Text type="secondary">优先级</Typography.Text>
              <div>{priorityTag(drawerItem.priority)}</div>
            </div>
            {drawerItem.productTitle ? (
              <div>
                <Typography.Text type="secondary">商品</Typography.Text>
                <div>{drawerItem.productTitle}</div>
              </div>
            ) : null}
            <div>
              <Typography.Text type="secondary">问题</Typography.Text>
              <Typography.Paragraph>{drawerItem.title}</Typography.Paragraph>
              <Typography.Paragraph type="secondary">{drawerItem.message}</Typography.Paragraph>
            </div>
            <div>
              <Typography.Text type="secondary">问题来源</Typography.Text>
              <div>{drawerItem.typeLabel}</div>
            </div>
            <div>
              <Typography.Text type="secondary">影响范围</Typography.Text>
              <Typography.Paragraph>
                {drawerItem.productTitle
                  ? `关联商品：${drawerItem.productTitle}`
                  : '可能影响批量刊登或系统任务处理进度'}
              </Typography.Paragraph>
            </div>
            <div>
              <Typography.Text type="secondary">建议操作</Typography.Text>
              <Typography.Paragraph>{drawerItem.actionLabel}：{drawerItem.message}</Typography.Paragraph>
            </div>
            <TechnicalDetails label="技术详情">
              <Space direction="vertical" size={4}>
                <div>待办编号：{drawerItem.id}</div>
                <div>来源类型：{drawerItem.sourceType}</div>
                <div>来源编号：{drawerItem.sourceId}</div>
                {drawerItem.issueCode ? <div>问题代码：{drawerItem.issueCode}</div> : null}
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
