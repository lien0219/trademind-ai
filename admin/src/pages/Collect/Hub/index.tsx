import { useLanguage } from '@/i18n/useLanguage';
import { tr } from '@/i18n';
import {
  CloudDownloadOutlined,
  FileSearchOutlined,
  HistoryOutlined,
  LinkOutlined,
  LoginOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { history } from '@umijs/max';
import { Alert, Button, Col, List, Result, Row, Skeleton, Space, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { PAGE_COPY, commonStatusLabel } from '@/constants/copywriting';
import { layoutTokens } from '@/constants/layoutTokens';
import { EmptyState, MetricCard, OperationToolbar, SectionCard, TmPageContainer } from '@/components/ui';
import { CustomCollectModal } from '@/pages/Collect/components/CustomCollectModal';
import { PinduoduoCollectModal } from '@/pages/Collect/components/PinduoduoCollectModal';
import { TaobaoTmallCollectModal } from '@/pages/Collect/components/TaobaoTmallCollectModal';
import type { CollectProviderRow, CollectProviderStatus } from '@/services/collectProviders';
import { queryCollectProviders } from '@/services/collectProviders';
import { queryCollectRules } from '@/services/collectRules';
import { usePermission } from '@/hooks/usePermission';
import { mayReadGlobalSettings } from '@/utils/globalSettingsAccess';
import { PERMISSIONS } from '@/utils/permission';
import { fetchCollectTasks, type CollectTaskRow } from '@/services/collectTasks';
import { queryBrowserProfiles, type BrowserProfileRow } from '@/services/collectBrowserProfiles';
import { formatDateTime } from '@/utils/formatTime';
import {
  COLLECT_HUB_TYPE_HINT,
  CUSTOM_BATCH_DISABLED_TOOLTIP,
  CUSTOM_COLLECT_CARD_DESCRIPTION,
  CUSTOM_COLLECT_CARD_NOTES,
} from '@/utils/customCollectPlatform';
import {
  collectProviderStatusPresentation,
  CUSTOM_COLLECT_DISPLAY_FEATURES,
  CUSTOM_COLLECT_FEATURE_LABEL,
  NO_COLLECT_RULE_MESSAGE,
} from '@/utils/collectProviderStatus';
import {
  collectSettingsConfigButtonLabel,
  collectSettingsPath,
} from '@/utils/collectSettingsProvider';
import CollectSourceCard, {
  type CollectSourceCardCopy,
  type CollectSourceCardFeature,
} from './components/CollectSourceCard';
import './index.less';

const { Paragraph, Text, Title } = Typography;

const DEDICATED_FEATURE_LABEL: Record<string, string> = {
  title: 'Product title',
  price: 'Product price',
  mainImages: 'Main images',
  descriptionImages: 'Detail images',
  attributes: 'Attributes',
  skus: 'Variants',
  stock: 'Stock (best effort)',
};

const SOURCE_ORDER = ['1688', 'pinduoduo', 'pdd', 'taobao_tmall', 'taobao', 'aliexpress', 'shein_temu', 'custom'];

const DEDICATED_HUB_DESCRIPTION: Record<string, string> = {
  '1688': '采集 1688 商品详情，支持标题、主图、详情图、属性与 SKU。',
  pinduoduo: '采集拼多多批发商品详情，支持标题、价格、主图、规格等；发布前请核对。',
  pdd: '采集拼多多批发商品详情，支持标题、价格、主图、规格等；发布前请核对。',
  taobao_tmall:
    '采集淘宝、天猫商品详情，支持标题、价格、主图、详情图、商品参数和商品规格。部分商品可能需要登录或人工确认。',
  taobao:
    '采集淘宝、天猫商品详情，支持标题、价格、主图、详情图、商品参数和商品规格。部分商品可能需要登录或人工确认。',
  aliexpress: '采集速卖通商品详情，支持标题、图片、属性与 SKU（测试中）。',
};

type LoadState<T> = {
  loading: boolean;
  error?: string;
  data: T;
};

function providerRunnableForSingleTask(status: CollectProviderStatus) {
  return status === 'available' || status === 'beta';
}

function batchRowDisabledForProvider(p: CollectProviderRow): boolean {
  return !providerRunnableForSingleTask(p.status) || !p.batchSupported;
}

function batchButtonTooltipForProvider(p: CollectProviderRow): string | undefined {
  if (!providerRunnableForSingleTask(p.status)) return tr("当前版本暂未开放");
  if (!p.batchSupported) {
    if (p.source === 'custom') return tr(CUSTOM_BATCH_DISABLED_TOOLTIP);
    if (p.source === 'pinduoduo' || p.source === 'pdd') {
      return tr("拼多多批量采集会自动限速，建议先少量测试。部分页面可能需要登录或触发验证。");
    }
    if (p.source === 'taobao_tmall' || p.source === 'taobao') {
      return tr("淘宝/天猫批量采集会逐条打开商品页面，建议每批不要超过 20 条。遇到登录或安全验证时，请先完成验证后重试。");
    }
    return p.status === 'beta' ? tr("测试阶段暂未开放批量") : tr("该平台暂不支持批量采集");
  }
  return undefined;
}

function providerCardFeatures(p: CollectProviderRow): string[] {
  if (p.source === 'custom') {
    const fromApi = (p.features ?? []).filter((f) => f !== 'skus');
    if (fromApi.length > 0) return fromApi;
    return [...CUSTOM_COLLECT_DISPLAY_FEATURES];
  }
  if (p.source === 'pinduoduo' || p.source === 'pdd') {
    const fromApi = p.features ?? [];
    if (fromApi.length > 0) return fromApi;
    return ['title', 'price', 'mainImages', 'descriptionImages', 'attributes', 'skus'];
  }
  if (p.source === 'taobao_tmall' || p.source === 'taobao') {
    const fromApi = p.features ?? [];
    if (fromApi.length > 0) return fromApi;
    return ['title', 'price', 'mainImages', 'descriptionImages', 'attributes', 'skus'];
  }
  return p.features ?? [];
}

function featureLabelForProvider(p: CollectProviderRow, feature: string): string {
  if (p.source === 'custom') {
    return tr(CUSTOM_COLLECT_FEATURE_LABEL[feature] ?? feature);
  }
  return tr(DEDICATED_FEATURE_LABEL[feature] ?? feature);
}

function providerCardCopy(p: CollectProviderRow): CollectSourceCardCopy {
  if (p.source === 'custom') {
    return {
      description: tr(CUSTOM_COLLECT_CARD_DESCRIPTION),
      notes: tr(CUSTOM_COLLECT_CARD_NOTES),
      typeLabel: tr(COLLECT_HUB_TYPE_HINT.custom.title),
      typeHint: tr(COLLECT_HUB_TYPE_HINT.custom.summary),
    };
  }
  const key = p.source.toLowerCase();
  const description = DEDICATED_HUB_DESCRIPTION[key] ?? p.description?.trim() ?? '';
  const notes = p.notes?.trim() ?? '';
  return {
    description: tr(description),
    notes: tr(notes),
    typeLabel: tr(COLLECT_HUB_TYPE_HINT.dedicated.title),
    typeHint: '',
  };
}

function sourceOrderValue(source: string) {
  const index = SOURCE_ORDER.indexOf(source);
  return index === -1 ? SOURCE_ORDER.length : index;
}

function isLoginSensitiveSource(source: string) {
  const src = source.trim().toLowerCase();
  return src === 'pinduoduo' || src === 'pdd' || src === 'taobao_tmall' || src === 'taobao' || src === 'custom';
}

function taskStatusTagColor(status: string) {
  const key = status.trim().toLowerCase();
  if (key === 'success') return 'success';
  if (key === 'failed') return 'error';
  if (key === 'running' || key === 'retrying') return 'processing';
  if (key === 'pending') return 'default';
  return 'default';
}

async function openCustomCollectModal(
  setCustomModalOpen: (open: boolean) => void,
): Promise<void> {
  try {
    const res = await queryCollectRules({ page: 1, pageSize: 1, status: 'enabled' });
    if (!res.list?.length) {
      message.warning(tr(NO_COLLECT_RULE_MESSAGE));
    }
  } catch {
    // 仍打开 Modal，由弹窗内引导创建规则
  }
  setCustomModalOpen(true);
}

function RecentTaskList({
  loading,
  error,
  tasks,
}: {
  loading: boolean;
  error?: string;
  tasks: CollectTaskRow[];
}) {
  useLanguage();
  if (loading) {
    return <Skeleton active paragraph={{ rows: 4 }} />;
  }
  if (error) {
    return (
      <Result
        status="warning"
        title={tr("最近任务加载失败")}
        subTitle={tr("可以进入采集任务列表查看任务状态，或稍后重新打开采集中心。")}
        extra={
          <Button onClick={() => history.push('/collect/tasks')}>
            {tr("查看采集任务")}</Button>
        }
      />
    );
  }
  if (!tasks.length) {
    return (
      <EmptyState
        compact
        title={tr("暂无最近采集任务")}
        description={tr("从上方选择采集来源并提交商品链接后，任务会出现在这里。")}
        actionLabel={tr("开始采集商品")}
        actionPath="/collect/hub"
      />
    );
  }

  return (
    <List
      className="tm-collect-hub-task-list"
      dataSource={tasks}
      renderItem={(task) => (
        <List.Item
          actions={[
            <Button key="view" type="link" onClick={() => history.push(`/collect/tasks?source=${encodeURIComponent(task.source)}`)}>
              {tr("查看")}</Button>,
          ]}
        >
          <List.Item.Meta
            title={
              <Space wrap size={8} className="tm-collect-hub-task-list__title">
                <Text strong>{task.source}</Text>
                <Tag color={taskStatusTagColor(task.status)}>{commonStatusLabel(task.status)}</Tag>
              </Space>
            }
            description={
              <div className="tm-collect-hub-task-list__description">
                <Text
                  type="secondary"
                  ellipsis={{ tooltip: task.sourceUrl }}
                  className="tm-collect-hub-task-list__url"
                >
                  {task.sourceUrl}
                </Text>
                <Text type="secondary">{formatDateTime(task.createdAt)}</Text>
              </div>
            }
          />
        </List.Item>
      )}
    />
  );
}

function BrowserProfileSummary({
  loading,
  error,
  profiles,
}: {
  loading: boolean;
  error?: string;
  profiles: BrowserProfileRow[];
}) {
  useLanguage();
  if (loading) {
    return <Skeleton active paragraph={{ rows: 3 }} />;
  }
  if (error) {
    return (
      <Alert
        type="warning"
        showIcon
        message={tr("登录状态加载失败")}
        description={tr("需要登录或验证码的平台，请到采集浏览器登录状态页面重新检测后再采集。")}
        action={<Button size="small" onClick={() => history.push('/collect/browser-profiles')}>{tr("去处理")}</Button>}
      />
    );
  }
  if (!profiles.length) {
    return (
      <EmptyState
        compact
        title={tr("暂无已保存登录状态")}
        description={tr("淘宝/天猫、拼多多或部分自定义网站可能需要登录后采集。")}
        actionLabel={tr("管理登录状态")}
        actionPath="/collect/browser-profiles"
      />
    );
  }
  return (
    <Space direction="vertical" size={10} className="tm-collect-hub-profile-list">
      {profiles.map((profile) => (
        <div className="tm-collect-hub-profile" key={profile.id}>
          <div className="tm-collect-hub-profile__main">
            <Text strong className="tm-collect-hub-profile__name">
              {profile.name}
            </Text>
            <Text type="secondary" className="tm-collect-hub-profile__domain">
              {profile.domain}
            </Text>
          </div>
          <Tag color={profile.lastCheckStatus === 'public' ? 'success' : 'warning'}>
            {profile.lastCheckStatus || profile.status}
          </Tag>
        </div>
      ))}
    </Space>
  );
}

export default function CollectHubPage() {
  useLanguage();
  const { user, can } = usePermission();
  const mayManagePlatform = mayReadGlobalSettings(user);
  const canCollect = can(PERMISSIONS.PRODUCT_WRITE);
  const [providerState, setProviderState] = useState<LoadState<CollectProviderRow[]>>({
    loading: true,
    data: [],
  });
  const [recentState, setRecentState] = useState<LoadState<CollectTaskRow[]>>({
    loading: true,
    data: [],
  });
  const [failedTotal, setFailedTotal] = useState<number | undefined>();
  const [profileState, setProfileState] = useState<LoadState<BrowserProfileRow[]>>({
    loading: true,
    data: [],
  });
  const [customModalOpen, setCustomModalOpen] = useState(false);
  const [pddModalOpen, setPddModalOpen] = useState(false);
  const [tbModalOpen, setTbModalOpen] = useState(false);

  const loadProviders = useCallback(async (isActive: () => boolean = () => true) => {
    setProviderState((state) => ({ ...state, loading: true, error: undefined }));
    try {
      const rows = await queryCollectProviders();
      if (isActive()) {
        setProviderState({ loading: false, data: Array.isArray(rows) ? rows : [] });
      }
    } catch (error) {
      if (isActive()) {
        setProviderState({
          loading: false,
          data: [],
          error: error instanceof Error ? error.message : tr("采集来源加载失败"),
        });
      }
    }
  }, []);

  useEffect(() => {
    let active = true;
    void loadProviders(() => active);
    return () => {
      active = false;
    };
  }, [loadProviders]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      setRecentState((state) => ({ ...state, loading: true, error: undefined }));
      try {
        const [recent, failed] = await Promise.all([
          fetchCollectTasks({ page: 1, pageSize: 5 }),
          fetchCollectTasks({ page: 1, pageSize: 1, status: 'failed' }),
        ]);
        if (!cancelled) {
          setRecentState({ loading: false, data: recent.list ?? [] });
          setFailedTotal(failed.pagination?.total ?? 0);
        }
      } catch (error) {
        if (!cancelled) {
          setRecentState({
            loading: false,
            data: [],
            error: error instanceof Error ? error.message : tr("最近采集任务加载失败"),
          });
          setFailedTotal(undefined);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      setProfileState((state) => ({ ...state, loading: true, error: undefined }));
      try {
        const res = await queryBrowserProfiles({ page: 1, pageSize: 4, status: 'active' });
        if (!cancelled) {
          setProfileState({ loading: false, data: res.list ?? [] });
        }
      } catch (error) {
        if (!cancelled) {
          setProfileState({
            loading: false,
            data: [],
            error: error instanceof Error ? error.message : tr("登录状态加载失败"),
          });
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const sortedProviders = useMemo(
    () =>
      providerState.data.filter((provider) => mayManagePlatform || provider.source !== 'custom').sort(
        (a, b) => sourceOrderValue(a.source) - sourceOrderValue(b.source) || a.name.localeCompare(b.name),
      ),
    [providerState.data, mayManagePlatform],
  );

  const runnableProviders = sortedProviders.filter((provider) => providerRunnableForSingleTask(provider.status));
  const batchProviders = sortedProviders.filter((provider) => providerRunnableForSingleTask(provider.status) && provider.batchSupported);
  const loginSensitiveProviders = sortedProviders.filter((provider) => isLoginSensitiveSource(provider.source));
  const primaryProvider = runnableProviders[0];

  const openSingleCollect = (provider: CollectProviderRow) => {
    if (provider.source === 'custom') {
      void openCustomCollectModal(setCustomModalOpen);
    } else if (provider.source === 'pinduoduo' || provider.source === 'pdd') {
      setPddModalOpen(true);
    } else if (provider.source === 'taobao_tmall' || provider.source === 'taobao') {
      setTbModalOpen(true);
    } else {
      history.push(`/collect/tasks?source=${encodeURIComponent(provider.source)}`);
    }
  };

  const pageExtra = (
    <OperationToolbar>
      <Button icon={<HistoryOutlined />} onClick={() => history.push('/collect/tasks')}>
        {tr("采集任务")}</Button>
      {mayManagePlatform ? <Button icon={<SettingOutlined />} onClick={() => history.push('/settings/collector')}>
        {tr("采集设置")}</Button> : null}
    </OperationToolbar>
  );

  return (
    <TmPageContainer
      title={PAGE_COPY.collectHub.title}
      subTitle={PAGE_COPY.collectHub.description}
      contentMaxWidth={layoutTokens.dashboardMaxWidth}
      extra={pageExtra}
    >
      <div className="tm-collect-hub">
        <section className="tm-collect-hub-hero">
          <div className="tm-collect-hub-hero__main">
            <Text className="tm-collect-hub-hero__eyebrow">{tr("跨境商品采集入口")}</Text>
            <Title level={4} className="tm-collect-hub-hero__title">
              {tr("先选择来源，再把商品链接转成可运营草稿")}</Title>
            <Paragraph className="tm-collect-hub-hero__desc">
              {tr("采集任务会进入队列处理。遇到登录、验证码或平台限制时，请先完成采集浏览器登录状态检测，再重试或批量恢复失败任务。")}</Paragraph>
            <OperationToolbar>
              <Button
                type="primary"
                size="large"
                icon={<CloudDownloadOutlined />}
                disabled={!canCollect || !primaryProvider || providerState.loading}
                onClick={() => primaryProvider && openSingleCollect(primaryProvider)}
              >
                {tr("开始采集商品")}</Button>
              <Button size="large" icon={<FileSearchOutlined />} disabled={!canCollect} onClick={() => history.push('/collect/batches')}>
                {tr("批量采集")}</Button>
              {mayManagePlatform ? <Button size="large" type="link" onClick={() => history.push('/collect/rules')}>
                {tr("管理采集规则")}</Button> : null}
            </OperationToolbar>
          </div>
          <div className="tm-collect-hub-hero__side">
            <MetricCard
              title={tr("支持来源")}
              value={providerState.loading ? '—' : sortedProviders.length}
              description={tr("按接口返回的采集器列表")}
              intent="primary"
              icon={<LinkOutlined />}
            />
            <MetricCard
              title={tr("可单条采集")}
              value={providerState.loading ? '—' : runnableProviders.length}
              description={tr("已可用或测试中")}
              intent="success"
              icon={<CloudDownloadOutlined />}
            />
            <MetricCard
              title={tr("批量入口")}
              value={providerState.loading ? '—' : batchProviders.length}
              description={tr("支持批量提交的来源")}
              intent="data"
              icon={<FileSearchOutlined />}
            />
            <MetricCard
              title={tr("失败待恢复")}
              value={recentState.loading ? '—' : failedTotal ?? '—'}
              description={tr("来自采集任务接口")}
              intent="warning"
              icon={<WarningOutlined />}
              onClick={() => history.push('/collect/tasks?status=failed')}
            />
          </div>
        </section>

        <Row gutter={[16, 16]}>
          <Col xs={24} lg={16}>
            <SectionCard
              title={tr("采集来源")}
              description={tr("专用采集器优先用于已适配平台；自定义采集器适合未适配网站，使用前建议先测试规则。")}
              headerExtra={
                <Button
                  icon={<ReloadOutlined />}
                  loading={providerState.loading}
                  onClick={() => void loadProviders()}
                >
                  {tr("重新加载")}</Button>
              }
            >
              {providerState.loading ? (
                <Row gutter={[16, 16]}>
                  {Array.from({ length: 3 }).map((_, index) => (
                    <Col xs={24} md={12} xl={8} key={index}>
                      <Skeleton active paragraph={{ rows: 6 }} />
                    </Col>
                  ))}
                </Row>
              ) : providerState.error ? (
                <Result
                  status="warning"
                  title={tr("采集来源加载失败")}
                  subTitle={tr("请检查采集服务配置或稍后重试。已有任务可以继续在采集任务页面查看。")}
                  extra={
                    <Space wrap>
                      <Button type="primary" onClick={() => history.push('/settings/collector')}>
                        {tr("检查采集设置")}</Button>
                      <Button onClick={() => history.push('/collect/tasks')}>
                        {tr("查看采集任务")}</Button>
                    </Space>
                  }
                />
              ) : sortedProviders.length === 0 ? (
                <EmptyState
                  title={tr("暂无可用采集来源")}
                  description={tr("请先到采集设置中检查采集服务配置，或确认后端已返回采集器列表。")}
                  actionLabel={tr("检查采集设置")}
                  actionPath="/settings/collector"
                />
              ) : (
                <Row gutter={[16, 16]}>
                  {sortedProviders.map((provider) => {
                    const statusTag = collectProviderStatusPresentation(provider.source, provider.status);
                    const copy = providerCardCopy(provider);
                    const features: CollectSourceCardFeature[] = providerCardFeatures(provider).map((feature) => ({
                      key: feature,
                      label: featureLabelForProvider(provider, feature),
                    }));
                    return (
                      <Col xs={24} md={12} xl={8} key={provider.source}>
                        <CollectSourceCard
                          provider={provider}
                          copy={copy}
                          statusTag={statusTag}
                          features={features}
                          singleDisabled={!canCollect || !providerRunnableForSingleTask(provider.status)}
                          singleTooltip={providerRunnableForSingleTask(provider.status) ? undefined : tr("当前版本暂未开放")}
                          batchDisabled={!canCollect || batchRowDisabledForProvider(provider)}
                          batchTooltip={(() => {
                            const tooltip = batchButtonTooltipForProvider(provider);
                            return tooltip ? tr(tooltip) : undefined;
                          })()}
                          onSingleCollect={() => openSingleCollect(provider)}
                          onBatchCollect={() => history.push(`/collect/batches?source=${encodeURIComponent(provider.source)}`)}
                          onSettings={() => history.push(collectSettingsPath(provider.source))}
                          showSettings={mayManagePlatform}
                          settingsLabel={tr(collectSettingsConfigButtonLabel(provider.status))}
                        />
                      </Col>
                    );
                  })}
                </Row>
              )}
            </SectionCard>
          </Col>

          <Col xs={24} lg={8}>
            <Space direction="vertical" size={16} className="tm-collect-hub__side-stack">
              <SectionCard
                title={tr("登录与验证风险")}
                description={tr("平台登录态和验证码会影响采集成功率。")}
                compact
              >
                <div className="tm-collect-hub-risk-list">
                  {loginSensitiveProviders.map((provider) => (
                    <div className="tm-collect-hub-risk" key={provider.source}>
                      <LoginOutlined />
                      <div className="tm-collect-hub-risk__content">
                        <Text strong>{provider.name}</Text>
                        <Text type="secondary">{tr("可能需要采集浏览器登录或人工验证")}</Text>
                      </div>
                    </div>
                  ))}
                </div>
              </SectionCard>

              <SectionCard
                title={tr("浏览器登录状态")}
                description={tr("用于处理登录页、验证码和安全验证。")}
                headerExtra={
                  <Button type="link" onClick={() => history.push('/collect/browser-profiles')}>
                    {tr("管理")}</Button>
                }
                compact
              >
                <BrowserProfileSummary
                  loading={profileState.loading}
                  error={profileState.error}
                  profiles={profileState.data}
                />
              </SectionCard>

              <SectionCard
                title={tr("失败恢复")}
                description={tr("失败任务保留原因和重试入口。")}
                compact
              >
                <div className="tm-collect-hub-recovery">
                  <SafetyCertificateOutlined />
                  <div>
                    <Text strong>{failedTotal ?? '—'} {tr("个失败任务")}</Text>
                    <Text type="secondary">{tr("进入任务列表查看原因、登录提示和重试入口。")}</Text>
                  </div>
                  <Button onClick={() => history.push('/collect/tasks?status=failed')}>{tr("去处理")}</Button>
                </div>
              </SectionCard>
            </Space>
          </Col>
        </Row>

        <Row gutter={[16, 16]}>
          <Col xs={24} lg={16}>
            <SectionCard
              title={tr("最近采集任务")}
              description={tr("用于确认任务是否已进入队列，以及失败后下一步处理方式。")}
              headerExtra={
                <Button type="link" onClick={() => history.push('/collect/tasks')}>
                  {tr("查看全部")}</Button>
              }
            >
              <RecentTaskList
                loading={recentState.loading}
                error={recentState.error}
                tasks={recentState.data}
              />
            </SectionCard>
          </Col>
          <Col xs={24} lg={8}>
            <SectionCard title={tr("相关管理入口")} description={tr("采集前后的常用配置和记录。")} compact>
              <div className="tm-collect-hub-links">
                <Button block onClick={() => history.push('/collect/tasks')}>{tr("采集任务")}</Button>
                <Button block onClick={() => history.push('/collect/batches')}>{tr("批量采集")}</Button>
                <Button block onClick={() => history.push('/collect/browser-profiles')}>{tr("浏览器登录状态")}</Button>
                {mayManagePlatform ? <Button block onClick={() => history.push('/collect/rules')}>{tr("采集规则")}</Button> : null}
                <Button block onClick={() => history.push('/collect/monitor')}>{tr("采集监控")}</Button>
                {mayManagePlatform ? <Button block onClick={() => history.push('/settings/collector')}>{tr("采集设置")}</Button> : null}
              </div>
            </SectionCard>
          </Col>
        </Row>
      </div>

      {mayManagePlatform ? <CustomCollectModal open={customModalOpen} onClose={() => setCustomModalOpen(false)} /> : null}
      <PinduoduoCollectModal open={pddModalOpen} onClose={() => setPddModalOpen(false)} />
      <TaobaoTmallCollectModal open={tbModalOpen} onClose={() => setTbModalOpen(false)} />
    </TmPageContainer>
  );
}
