import {
  ArrowRightOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  CloudUploadOutlined,
  DatabaseOutlined,
  FileTextOutlined,
  NotificationOutlined,
  PictureOutlined,
  ReloadOutlined,
  RobotOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  ShopOutlined,
  UnorderedListOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { formatDateTime } from '@/utils/formatTime';
import { history } from '@umijs/max';
import {
  Button,
  Col,
  DatePicker,
  Empty,
  Result,
  Row,
  Select,
  Skeleton,
  Space,
  Tag,
  Typography,
} from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import type { ReactNode } from 'react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { PLATFORM_OPTIONS } from '@/constants/userFriendly';
import {
  DEFAULT_FUNNEL,
  DEFAULT_QUICK_LINKS,
  EMPTY_SUMMARY,
  mergeExceptions,
  mergeFunnel,
  mergeTodos,
} from '@/constants/dashboardDefaults';
import {
  formatRecentItem,
  recentStatusColor,
  recentStatusLabel,
  recentTranslateWarningSubtitle,
} from '@/constants/dashboardRecent';
import {
  queryProductOperationDashboard,
  type DashboardException,
  type DashboardFunnelStep,
  type DashboardRecentItem,
  type DashboardSummary,
  type DashboardTodo,
  type ProductOperationDashboard,
} from '@/services/dashboard';
import { queryShops, type ShopListRow } from '@/services/shops';

const { RangePicker } = DatePicker;

const SOURCE_OPTIONS = [
  { label: '1688', value: '1688' },
  { label: '拼多多', value: 'pinduoduo' },
  { label: '自定义链接', value: 'custom' },
  { label: '速卖通', value: 'aliexpress' },
  { label: '手动创建', value: 'manual' },
];

const RECENT_TYPE_META: Record<string, { icon: ReactNode; color: string; bg: string }> = {
  采集: { icon: <CloudUploadOutlined />, color: '#2563eb', bg: '#eff6ff' },
  'AI 文本': { icon: <RobotOutlined />, color: '#7c3aed', bg: '#f5f3ff' },
  'AI 批次': { icon: <FileTextOutlined />, color: '#6366f1', bg: '#eef2ff' },
  'AI 图片': { icon: <PictureOutlined />, color: '#0891b2', bg: '#ecfeff' },
  刊登: { icon: <ShopOutlined />, color: '#059669', bg: '#ecfdf5' },
  库存: { icon: <WarningOutlined />, color: '#ea580c', bg: '#fff7ed' },
  刊登失败: { icon: <ShopOutlined />, color: '#dc2626', bg: '#fef2f2' },
  库存同步失败: { icon: <WarningOutlined />, color: '#dc2626', bg: '#fef2f2' },
  采集失败: { icon: <CloudUploadOutlined />, color: '#dc2626', bg: '#fef2f2' },
  告警: { icon: <NotificationOutlined />, color: '#b45309', bg: '#fffbeb' },
  失败: { icon: <WarningOutlined />, color: '#dc2626', bg: '#fef2f2' },
};

const ellipsizedText: React.CSSProperties = {
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
  display: 'block',
  maxWidth: '100%',
};

function RecentActivityRow({
  item,
  bucket,
}: {
  item: DashboardRecentItem;
  bucket: string;
}) {
  const meta = RECENT_TYPE_META[bucket] ?? RECENT_TYPE_META['AI 图片'];
  const { title, subtitle } = formatRecentItem(item);
  const statusLabel = recentStatusLabel(item.status);
  const statusColor = recentStatusColor(item.status);
  const displaySubtitle =
    item.type === 'image_task' ? recentTranslateWarningSubtitle(subtitle) ?? subtitle : subtitle;

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => history.push(item.link)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') history.push(item.link);
      }}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 14,
        padding: '14px 16px',
        borderRadius: 10,
        border: '1px solid var(--ant-color-border-secondary, #f0f0f0)',
        background: '#fff',
        cursor: 'pointer',
        transition: 'border-color 0.2s, box-shadow 0.2s',
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.borderColor = meta.color;
        e.currentTarget.style.boxShadow = `0 2px 8px ${meta.color}14`;
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.borderColor = 'var(--ant-color-border-secondary, #f0f0f0)';
        e.currentTarget.style.boxShadow = 'none';
      }}
    >
      <div
        style={{
          width: 40,
          height: 40,
          borderRadius: 12,
          background: meta.bg,
          color: meta.color,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: 18,
          flexShrink: 0,
        }}
      >
        {meta.icon}
      </div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <Space wrap size={6} style={{ marginBottom: 6 }}>
          <Tag
            bordered={false}
            style={{ margin: 0, background: meta.bg, color: meta.color, fontSize: 12 }}
          >
            {bucket}
          </Tag>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {formatDateTime(item.occurredAt)}
          </Typography.Text>
          {item.status ? <Tag color={statusColor}>{statusLabel}</Tag> : null}
        </Space>
        <Typography.Text strong style={{ ...ellipsizedText, fontSize: 14 }} title={title}>
          {title}
        </Typography.Text>
        {displaySubtitle ? (
          <Typography.Text
            type="secondary"
            style={{ ...ellipsizedText, fontSize: 12, marginTop: 4 }}
            title={displaySubtitle}
          >
            {displaySubtitle}
          </Typography.Text>
        ) : null}
      </div>
      <Button
        type="link"
        size="small"
        icon={<ArrowRightOutlined />}
        onClick={(e) => {
          e.stopPropagation();
          history.push(item.link);
        }}
      >
        查看
      </Button>
    </div>
  );
}

const RECENT_TYPE_LABEL: Record<string, string> = {
  collect: '采集',
  ai_task: 'AI 文本',
  ai_batch: 'AI 批次',
  image_task: 'AI 图片',
  product_publish: '刊登',
  inventory_alert: '库存',
  failed_publish: '刊登失败',
  failed_inventory_sync: '库存同步失败',
  failed_collect: '采集失败',
  task_alert: '告警',
};

const TODO_ACTION_LABEL: Record<string, string> = {
  missing_ai_title: '去优化',
  missing_ai_description: '去生成',
  readiness_blocked: '去检查',
  publishable: '去刊登',
  inventory_alerts: '去处理',
  ai_image_failed: '去查看',
  collect_failed: '去重试',
  publish_failed: '去处理',
  order_exceptions: '去处理',
  failures: '去查看',
};

type FilterState = {
  range?: [Dayjs, Dayjs];
  platform?: string;
  shopId?: string;
  source?: string;
};

function KpiCard(props: {
  title: string;
  value: number;
  tone?: string;
  onClick?: () => void;
}) {
  return (
    <ProCard
      bordered
      hoverable={!!props.onClick}
      bodyStyle={{ padding: '14px 16px', cursor: props.onClick ? 'pointer' : 'default' }}
      onClick={props.onClick}
    >
      <Typography.Text type="secondary" style={{ fontSize: 13 }}>
        {props.title}
      </Typography.Text>
      <div style={{ fontSize: 28, fontWeight: 600, color: props.tone ?? '#111827', lineHeight: 1.2, marginTop: 4 }}>
        {props.value ?? 0}
      </div>
    </ProCard>
  );
}

function TodoCardItem({ item }: { item: DashboardTodo }) {
  const actionLabel = TODO_ACTION_LABEL[item.key] ?? '去处理';
  const hasCount = (item.count ?? 0) > 0;
  return (
    <ProCard
      bordered
      bodyStyle={{ padding: '16px', height: '100%' }}
      style={hasCount ? { borderColor: '#f97316' } : undefined}
    >
      <Space direction="vertical" size={8} style={{ width: '100%' }}>
        <Space align="center" style={{ justifyContent: 'space-between', width: '100%' }}>
          <Typography.Text strong>{item.title}</Typography.Text>
          <Typography.Title level={3} style={{ margin: 0 }}>
            {item.count ?? 0}
          </Typography.Title>
        </Space>
        <Button type="primary" block onClick={() => history.push(item.link)}>
          {actionLabel}
        </Button>
      </Space>
    </ProCard>
  );
}

function ExceptionRow({ item }: { item: DashboardException }) {
  return (
    <ProCard
      bordered
      bodyStyle={{ padding: '16px 18px' }}
      style={{ margin: 0 }}
      hoverable
      onClick={() => history.push(item.link)}
    >
      <Row align="middle" gutter={16} wrap={false}>
        <Col flex="auto">
          <Space direction="vertical" size={6}>
            <Space>
              <Typography.Text strong>{item.title}</Typography.Text>
              <Tag color={item.count > 0 ? 'error' : 'default'}>{item.count ?? 0}</Tag>
            </Space>
            {item.lastOccurredAt ? (
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                最近：{formatDateTime(item.lastOccurredAt)}
              </Typography.Text>
            ) : null}
          </Space>
        </Col>
        <Col>
          <Button type="link" icon={<ArrowRightOutlined />}>
            去处理
          </Button>
        </Col>
      </Row>
    </ProCard>
  );
}

const QUICK_LINK_META: Record<string, { icon: ReactNode; color: string; bg: string }> = {
  '/collect/hub': { icon: <CloudUploadOutlined />, color: '#2563eb', bg: '#eff6ff' },
  '/product/drafts': { icon: <FileTextOutlined />, color: '#4f46e5', bg: '#eef2ff' },
  '/ai/batches': { icon: <RobotOutlined />, color: '#7c3aed', bg: '#f5f3ff' },
  '/ai/image-tasks': { icon: <PictureOutlined />, color: '#0891b2', bg: '#ecfeff' },
  '/product/drafts?readiness=blocked': { icon: <SafetyCertificateOutlined />, color: '#ea580c', bg: '#fff7ed' },
  '/product/publish-tasks': { icon: <ShopOutlined />, color: '#059669', bg: '#ecfdf5' },
  '/inventory/alerts': { icon: <WarningOutlined />, color: '#dc2626', bg: '#fef2f2' },
  '/ops/task-center/failures': { icon: <CloseCircleOutlined />, color: '#b91c1c', bg: '#fef2f2' },
  '/orders/exceptions': { icon: <UnorderedListOutlined />, color: '#c2410c', bg: '#fff7ed' },
  '/settings/ai': { icon: <SettingOutlined />, color: '#6366f1', bg: '#eef2ff' },
  '/settings/image': { icon: <PictureOutlined />, color: '#0d9488', bg: '#f0fdfa' },
  '/settings/storage': { icon: <DatabaseOutlined />, color: '#64748b', bg: '#f8fafc' },
};

const QUICK_LINK_GROUPS: { label: string; links: string[] }[] = [
  {
    label: '商品运营',
    links: [
      '/collect/hub',
      '/product/drafts',
      '/product/drafts?readiness=blocked',
      '/product/publish-tasks',
      '/inventory/alerts',
    ],
  },
  {
    label: 'AI 工具',
    links: ['/ai/batches', '/ai/image-tasks'],
  },
  {
    label: '运维与设置',
    links: [
      '/ops/task-center/failures',
      '/orders/exceptions',
      '/settings/ai',
      '/settings/image',
      '/settings/storage',
    ],
  },
];

function QuickLinkCard(props: { title: string; link: string }) {
  const meta = QUICK_LINK_META[props.link] ?? {
    icon: <ArrowRightOutlined />,
    color: '#64748b',
    bg: '#f8fafc',
  };

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => history.push(props.link)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') history.push(props.link);
      }}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 12,
        height: '100%',
        minHeight: 56,
        padding: '12px 14px',
        borderRadius: 10,
        border: '1px solid var(--ant-color-border-secondary, #f0f0f0)',
        background: '#fff',
        cursor: 'pointer',
        transition: 'border-color 0.2s, box-shadow 0.2s, transform 0.15s',
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.borderColor = meta.color;
        e.currentTarget.style.boxShadow = `0 4px 12px ${meta.color}18`;
        e.currentTarget.style.transform = 'translateY(-1px)';
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.borderColor = 'var(--ant-color-border-secondary, #f0f0f0)';
        e.currentTarget.style.boxShadow = 'none';
        e.currentTarget.style.transform = 'none';
      }}
    >
      <div
        style={{
          width: 36,
          height: 36,
          borderRadius: 10,
          background: meta.bg,
          color: meta.color,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: 17,
          flexShrink: 0,
        }}
      >
        {meta.icon}
      </div>
      <Typography.Text strong style={{ flex: 1, fontSize: 13, lineHeight: 1.4 }}>
        {props.title}
      </Typography.Text>
      <ArrowRightOutlined style={{ color: '#cbd5e1', fontSize: 12, flexShrink: 0 }} />
    </div>
  );
}

function QuickLinkGroups({ links }: { links: { title: string; link: string }[] }) {
  const byLink = new Map(links.map((item) => [item.link, item]));

  return (
    <Space direction="vertical" size={20} style={{ width: '100%' }}>
      {QUICK_LINK_GROUPS.map((group) => {
        const items = group.links.map((link) => byLink.get(link)).filter(Boolean) as {
          title: string;
          link: string;
        }[];
        if (!items.length) return null;

        return (
          <div key={group.label}>
            <Typography.Text
              type="secondary"
              style={{ display: 'block', fontSize: 12, marginBottom: 10, fontWeight: 500 }}
            >
              {group.label}
            </Typography.Text>
            <Row gutter={[12, 12]}>
              {items.map((item) => (
                <Col xs={24} sm={12} md={8} lg={8} xl={8} key={item.link}>
                  <QuickLinkCard title={item.title} link={item.link} />
                </Col>
              ))}
            </Row>
          </div>
        );
      })}
    </Space>
  );
}

function buildKpiCards(summary: DashboardSummary): {
  title: string;
  value: number;
  link: string;
  tone?: string;
}[] {
  return [
    {
      title: '商品草稿总数',
      value: summary.draftTotal ?? summary.draftProducts + summary.readyProducts,
      link: '/product/drafts',
    },
    {
      title: '今日新增商品',
      value: summary.todayNewProducts ?? 0,
      link: '/product/drafts',
      tone: '#2563eb',
    },
    {
      title: '待补 AI 标题',
      value: summary.missingAiTitle ?? summary.missingAiTitleCount ?? 0,
      link: '/product/drafts?missingAiTitle=1',
      tone: '#7c3aed',
    },
    {
      title: '待补 AI 描述',
      value: summary.missingAiDescription ?? summary.missingAiDescriptionCount ?? 0,
      link: '/product/drafts?missingAiDescription=1',
      tone: '#7c3aed',
    },
    {
      title: 'AI 图片待处理',
      value: summary.imageTaskPending ?? 0,
      link: '/ai/image-tasks',
      tone: '#0891b2',
    },
    {
      title: '发布检查未通过',
      value: summary.readinessBlocked ?? summary.readinessBlockedProducts ?? 0,
      link: '/product/drafts?readiness=blocked',
      tone: '#ea580c',
    },
    {
      title: '可发布商品',
      value: summary.publishable ?? 0,
      link: '/product/drafts?publishable=1',
      tone: '#059669',
    },
    {
      title: '已刊登商品',
      value: summary.published ?? summary.publishedProducts ?? 0,
      link: '/product/drafts?status=published',
      tone: '#0d9488',
    },
    {
      title: '库存预警',
      value: summary.inventoryAlerts ?? summary.lowStockSkus + summary.outOfStockSkus,
      link: '/inventory/alerts',
      tone: '#dc2626',
    },
    {
      title: '失败任务',
      value: summary.failedTaskTotal ?? summary.failedTasks ?? 0,
      link: '/ops/task-center/failures',
      tone: '#b91c1c',
    },
  ];
}

function mergeRecentItems(recent: ProductOperationDashboard['recent']): (DashboardRecentItem & { bucket: string })[] {
  if (!recent) return [];
  const buckets: { items: DashboardRecentItem[]; label: string }[] = [
    { items: recent.collectedProducts ?? [], label: '采集' },
    { items: recent.aiTasks ?? [], label: 'AI 文本' },
    { items: recent.imageTasks ?? [], label: 'AI 图片' },
    { items: recent.publishTasks ?? [], label: '刊登' },
    { items: recent.failedTasks ?? [], label: '失败' },
  ];
  return buckets
    .flatMap(({ items, label }) =>
      items.map((x) => ({
        ...x,
        bucket: RECENT_TYPE_LABEL[x.type] ?? label,
      })),
    )
    .sort((a, b) => dayjs(b.occurredAt).valueOf() - dayjs(a.occurredAt).valueOf())
    .slice(0, 10);
}

const FUNNEL_STEP_META: Record<string, { icon: ReactNode; color: string; bg: string }> = {
  collected: { icon: <CloudUploadOutlined />, color: '#2563eb', bg: '#eff6ff' },
  draft: { icon: <FileTextOutlined />, color: '#4f46e5', bg: '#eef2ff' },
  ai_text: { icon: <RobotOutlined />, color: '#7c3aed', bg: '#f5f3ff' },
  ai_image: { icon: <PictureOutlined />, color: '#0891b2', bg: '#ecfeff' },
  readiness_pass: { icon: <SafetyCertificateOutlined />, color: '#059669', bg: '#ecfdf5' },
  published: { icon: <CheckCircleOutlined />, color: '#0d9488', bg: '#f0fdfa' },
};

function FunnelSteps({ steps }: { steps: DashboardFunnelStep[] }) {
  const list = steps.length ? steps : DEFAULT_FUNNEL;
  const topCount = list[0]?.count ?? 0;
  const max = Math.max(...list.map((s) => s.count ?? 0), 1);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      {list.map((step, index) => {
        const count = step.count ?? 0;
        const meta = FUNNEL_STEP_META[step.key] ?? FUNNEL_STEP_META.collected;
        const barPct = count > 0 ? Math.max(8, Math.round((count / max) * 100)) : 0;
        const convPct = topCount > 0 ? Math.round((count / topCount) * 100) : 0;
        const isLast = index === list.length - 1;

        return (
          <div key={step.key} style={{ display: 'flex', gap: 14 }}>
            <div
              style={{
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                width: 40,
                flexShrink: 0,
              }}
            >
              <div
                style={{
                  width: 40,
                  height: 40,
                  borderRadius: 12,
                  background: meta.bg,
                  color: meta.color,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: 18,
                  boxShadow: `0 0 0 1px ${meta.color}22`,
                }}
              >
                {meta.icon}
              </div>
              {!isLast ? (
                <div
                  style={{
                    width: 2,
                    flex: 1,
                    minHeight: 20,
                    margin: '6px 0',
                    borderRadius: 1,
                    background: `linear-gradient(180deg, ${meta.color}66, #e5e7eb)`,
                  }}
                />
              ) : null}
            </div>

            <div
              role="button"
              tabIndex={0}
              onClick={() => history.push(step.link)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') history.push(step.link);
              }}
              style={{
                flex: 1,
                marginBottom: isLast ? 0 : 6,
                padding: '14px 16px',
                borderRadius: 10,
                border: '1px solid var(--ant-color-border-secondary, #f0f0f0)',
                background: count > 0 ? 'var(--ant-color-fill-quaternary, #fafafa)' : '#fff',
                cursor: 'pointer',
                transition: 'border-color 0.2s, box-shadow 0.2s',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.borderColor = meta.color;
                e.currentTarget.style.boxShadow = `0 2px 8px ${meta.color}18`;
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.borderColor = 'var(--ant-color-border-secondary, #f0f0f0)';
                e.currentTarget.style.boxShadow = 'none';
              }}
            >
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: 12,
                  marginBottom: 10,
                }}
              >
                <Typography.Text strong style={{ fontSize: 14 }}>
                  {step.title}
                </Typography.Text>
                <Space size={8} align="center">
                  <Typography.Text
                    strong
                    style={{ fontSize: 18, color: count > 0 ? meta.color : undefined, lineHeight: 1 }}
                  >
                    {count}
                  </Typography.Text>
                  {index > 0 && topCount > 0 ? (
                    <Tag
                      bordered={false}
                      style={{
                        margin: 0,
                        background: `${meta.color}14`,
                        color: meta.color,
                        fontSize: 12,
                      }}
                    >
                      {convPct}%
                    </Tag>
                  ) : null}
                  <ArrowRightOutlined style={{ color: '#9ca3af', fontSize: 12 }} />
                </Space>
              </div>
              <div
                style={{
                  height: 10,
                  borderRadius: 999,
                  background: '#eef2f6',
                  overflow: 'hidden',
                }}
              >
                <div
                  style={{
                    height: '100%',
                    width: `${barPct}%`,
                    background: `linear-gradient(90deg, ${meta.color}, ${meta.color}99)`,
                    borderRadius: 999,
                    transition: 'width 0.45s ease',
                  }}
                />
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function DashboardSkeleton() {
  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Skeleton active paragraph={{ rows: 2 }} />
      <Row gutter={[16, 16]}>
        {Array.from({ length: 6 }).map((_, i) => (
          <Col xs={24} sm={12} md={8} lg={8} xl={4} key={i}>
            <Skeleton active />
          </Col>
        ))}
      </Row>
      <Skeleton active paragraph={{ rows: 6 }} />
    </Space>
  );
}

export default function ProductOperationsDashboardPage() {
  const [filters, setFilters] = useState<FilterState>({});
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [board, setBoard] = useState<ProductOperationDashboard | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    void queryShops({ page: 1, pageSize: 200 })
      .then((res) => setShops(res?.list ?? []))
      .catch(() => setShops([]));
  }, []);

  const queryParams = useMemo(() => {
    const [start, end] = filters.range ?? [];
    return {
      start: start ? start.startOf('day').toISOString() : undefined,
      end: end ? end.endOf('day').toISOString() : undefined,
      platform: filters.platform || undefined,
      shopId: filters.shopId || undefined,
      source: filters.source || undefined,
    };
  }, [filters]);

  const loadBoard = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await queryProductOperationDashboard(queryParams);
      setBoard(res ?? null);
    } catch (e) {
      setBoard(null);
      setError(e instanceof Error ? e : new Error(String(e ?? 'load_failed')));
    } finally {
      setLoading(false);
    }
  }, [queryParams]);

  useEffect(() => {
    void loadBoard();
  }, [loadBoard]);

  const summary = board?.summary ?? EMPTY_SUMMARY;
  const todos = useMemo(() => mergeTodos(board?.todos), [board?.todos]);
  const funnelSteps = useMemo(() => mergeFunnel(board?.funnel), [board?.funnel]);
  const exceptions = useMemo(() => mergeExceptions(board?.exceptions), [board?.exceptions]);
  const quickLinks = DEFAULT_QUICK_LINKS;
  const recentFlat = useMemo(() => mergeRecentItems(board?.recent), [board?.recent]);
  const kpiCards = useMemo(() => buildKpiCards(summary), [summary]);

  const doRefresh = useCallback(() => {
    void loadBoard();
  }, [loadBoard]);

  useEffect(() => {
    if (!autoRefresh) return;
    const tick = () => {
      if (document.hidden) return;
      void loadBoard();
    };
    const id = window.setInterval(tick, 60_000);
    return () => window.clearInterval(id);
  }, [autoRefresh, loadBoard]);

  const welcomeActions: { label: string; icon: ReactNode; link: string }[] = [
    { label: '采集商品', icon: <CloudUploadOutlined />, link: '/collect/hub' },
    { label: '批量 AI 优化', icon: <RobotOutlined />, link: '/ai/batches' },
    { label: 'AI 图片任务', icon: <PictureOutlined />, link: '/ai/image-tasks' },
    { label: '查看发布检查', icon: <SafetyCertificateOutlined />, link: '/product/drafts?readiness=blocked' },
  ];

  return (
    <PageContainer
      title="工作台 / 商品运营看板"
      extra={
        <Space>
          <Button
            type={autoRefresh ? 'primary' : 'default'}
            ghost={autoRefresh}
            size="small"
            onClick={() => setAutoRefresh((v) => !v)}
          >
            {autoRefresh ? '自动刷新中' : '已暂停自动刷新'}
          </Button>
          <Button icon={<ReloadOutlined />} onClick={doRefresh} loading={loading}>
            重新加载
          </Button>
        </Space>
      }
    >
      {/* 筛选 */}
      <ProCard bordered style={{ marginBottom: 16 }} bodyStyle={{ padding: '12px 16px' }}>
        <Space wrap size={[12, 12]}>
          <RangePicker
            value={filters.range}
            onChange={(v) => setFilters((f) => ({ ...f, range: v as [Dayjs, Dayjs] | undefined }))}
            allowClear
            placeholder={['开始日期', '结束日期']}
          />
          <Select
            allowClear
            placeholder="平台"
            style={{ width: 140 }}
            options={PLATFORM_OPTIONS}
            value={filters.platform}
            onChange={(v) => setFilters((f) => ({ ...f, platform: v }))}
          />
          <Select
            allowClear
            placeholder="店铺"
            style={{ width: 180 }}
            showSearch
            optionFilterProp="label"
            options={shops.map((s) => ({
              label: s.name || s.id,
              value: s.id,
            }))}
            value={filters.shopId}
            onChange={(v) => setFilters((f) => ({ ...f, shopId: v }))}
          />
          <Select
            allowClear
            placeholder="商品来源"
            style={{ width: 140 }}
            options={SOURCE_OPTIONS}
            value={filters.source}
            onChange={(v) => setFilters((f) => ({ ...f, source: v }))}
          />
          <Button
            onClick={() =>
              setFilters({
                range: undefined,
                platform: undefined,
                shopId: undefined,
                source: undefined,
              })
            }
          >
            重置筛选
          </Button>
        </Space>
      </ProCard>

      {error ? (
        <Result
          status="error"
          title="看板数据加载失败，请稍后重试"
          subTitle={error instanceof Error ? error.message : '网络或服务异常'}
          extra={
            <Button type="primary" onClick={doRefresh}>
              重新加载
            </Button>
          }
        />
      ) : loading && !board ? (
        <DashboardSkeleton />
      ) : (
        <>
          {/* 1. 顶部欢迎区 + KPI */}
          <ProCard bordered style={{ marginBottom: 16 }} bodyStyle={{ padding: '20px 24px' }}>
            <Row align="middle" gutter={[16, 16]} wrap style={{ marginBottom: 20 }}>
              <Col flex="auto">
                <Typography.Title level={4} style={{ margin: 0 }}>
                  今日商品运营概览
                </Typography.Title>
              </Col>
              <Col>
                <Space wrap>
                  {welcomeActions.map((a) => (
                    <Button key={a.link} icon={a.icon} onClick={() => history.push(a.link)}>
                      {a.label}
                    </Button>
                  ))}
                </Space>
              </Col>
            </Row>
            <Row gutter={[12, 12]}>
              {kpiCards.map((card) => (
                <Col xs={12} sm={8} md={6} lg={4} xl={4} key={card.title}>
                  <KpiCard
                    title={card.title}
                    value={card.value}
                    tone={card.tone}
                    onClick={() => history.push(card.link)}
                  />
                </Col>
              ))}
            </Row>
          </ProCard>

          {/* 2. 今日待办 */}
          <ProCard title="今日待办" bordered style={{ marginBottom: 16 }}>
            <Row gutter={[16, 16]}>
              {todos.map((item) => (
                <Col xs={24} sm={12} md={8} lg={6} xl={6} key={item.key || item.id}>
                  <TodoCardItem item={item} />
                </Col>
              ))}
            </Row>
          </ProCard>

          <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
            {/* 4. AI 商品运营进度漏斗 */}
            <Col xs={24} lg={10}>
              <ProCard title="AI 商品运营进度" bordered bodyStyle={{ padding: '16px 20px 12px' }}>
                <FunnelSteps steps={funnelSteps} />
              </ProCard>
            </Col>

            {/* 5. 异常与失败提醒 */}
            <Col xs={24} lg={14}>
              <ProCard title="异常与失败提醒" bordered>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                  {exceptions.map((item) => (
                    <ExceptionRow key={item.key} item={item} />
                  ))}
                </div>
              </ProCard>
            </Col>
          </Row>

          {/* 最近动态 */}
          <ProCard title="最近动态" bordered style={{ marginBottom: 16 }} bodyStyle={{ padding: '12px 16px 16px' }}>
            {recentFlat.length ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                {recentFlat.map((item, idx) => (
                  <RecentActivityRow
                    key={`${item.type}-${item.occurredAt}-${idx}`}
                    item={item}
                    bucket={item.bucket}
                  />
                ))}
              </div>
            ) : (
              <Empty description="暂无最近动态">
                <Space>
                  <Button type="primary" onClick={() => history.push('/collect/hub')}>
                    去采集商品
                  </Button>
                  <Button onClick={() => history.push('/ai/image-tasks')}>去创建 AI 图片任务</Button>
                </Space>
              </Empty>
            )}
          </ProCard>

          {/* 快捷入口 */}
          <ProCard title="快捷入口" bordered bodyStyle={{ padding: '16px 20px 20px' }}>
            <QuickLinkGroups links={quickLinks} />
          </ProCard>
        </>
      )}
    </PageContainer>
  );
}
