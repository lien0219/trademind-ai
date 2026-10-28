import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import {
  BellOutlined,
  CheckCircleOutlined,
  CloudServerOutlined,
  ExclamationCircleOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
} from '@ant-design/icons';
import { history } from '@umijs/max';
import { Button, Descriptions, Spin, Tag, Tooltip, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { ErrorAlert, MetricCard, SectionCard, TmPageContainer } from '@/components/ui';
import {
  fetchObservabilityOverview,
  type ObservabilityOverview,
} from '@/services/observability';
import { formatDateTime } from '@/utils/formatTime';
import styles from './index.less';

const { Text } = Typography;

type StatusMeta = {
  color: 'default' | 'processing' | 'success' | 'error' | 'warning';
  text: string;
  intent: 'default' | 'success' | 'warning' | 'danger' | 'data';
};

const STATUS_META: Record<string, StatusMeta> = {
  healthy: { color: 'success', get text() { return tr("运行正常"); }, intent: 'success' },
  needs_attention: { color: 'error', get text() { return tr("需要处理"); }, intent: 'danger' },
  waiting: { color: 'processing', get text() { return tr("等待首次评估"); }, intent: 'warning' },
  warming_up: { color: 'processing', get text() { return tr("评估器预热中"); }, intent: 'warning' },
  succeeded: { color: 'success', get text() { return tr("最近评估成功"); }, intent: 'success' },
  failed: { color: 'error', get text() { return tr("最近评估失败"); }, intent: 'danger' },
  disabled: { color: 'default', get text() { return tr("未启用"); }, intent: 'default' },
  active: { color: 'success', get text() { return tr("运行中"); }, intent: 'success' },
  unavailable: { color: 'error', get text() { return tr("不可用"); }, intent: 'danger' },
  unprotected: { color: 'error', get text() { return tr("保护不完整"); }, intent: 'danger' },
  achieved: { color: 'success', get text() { return tr("目标内"); }, intent: 'success' },
  violated: { color: 'error', get text() { return tr("目标外"); }, intent: 'danger' },
  insufficient_data: { color: 'warning', get text() { return tr("数据不足"); }, intent: 'warning' },
  standard_protocol_ready: { color: 'success', get text() { return tr("最近导出成功"); }, intent: 'success' },
  export_pending: { color: 'processing', get text() { return tr("等待首次导出"); }, intent: 'warning' },
  real_backend_deferred: { color: 'default', get text() { return tr("未配置导出后端"); }, intent: 'default' },
  export_degraded: { color: 'error', get text() { return tr("导出异常"); }, intent: 'danger' },
  incomplete: { color: 'error', get text() { return tr("配置不完整"); }, intent: 'danger' },
};

function statusMeta(status?: string): StatusMeta {
  return STATUS_META[status ?? ''] ?? { color: 'default', text: status || tr("未知"), intent: 'default' };
}

function StatusValue({ status }: { status?: string }) {
  const meta = statusMeta(status);
  return <Tag color={meta.color}>{meta.text}</Tag>;
}

export default function ObservabilityCenterPage() {
  const language = useLanguage();
  const [overview, setOverview] = useState<ObservabilityOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const result = await fetchObservabilityOverview();
      setOverview(result?.data ?? null);
      setLoadError(false);
    } catch {
      message.error(tr("加载可观测性数据失败"));
      setLoadError(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const overallMeta = statusMeta(overview?.overallStatus);
  const evaluationMeta = statusMeta(overview?.evaluation?.status);
  const metricMeta = statusMeta(overview?.metrics?.status);
  const alertMeta = statusMeta(overview?.alerts?.status);
  const activeAlerts = overview?.alerts?.active ?? 0;
  const alertValue = overview?.alerts?.status === 'active' ? activeAlerts : alertMeta.text;
  const alertDescription = overview?.alerts?.status === 'active'
    ? trFormat("严重 {value0} · 警告 {value1}", { value0: overview?.alerts?.critical ?? 0, value1: overview?.alerts?.warning ?? 0 })
    : overview?.alerts?.status === 'disabled' ? tr("系统告警未启用") : tr("无法读取系统告警");
  const alertIntent = overview?.alerts?.status === 'active'
    ? activeAlerts > 0 ? 'danger' : 'success'
    : alertMeta.intent;
  const lastUpdated = useMemo(() => formatDateTime(overview?.timestamp), [overview?.timestamp, language]);

  return (
    <TmPageContainer
      title={tr("可观测性中心")}
      subTitle={tr("查看系统指标、告警评估、SLO 与遥测导出运行状态")}
      className={styles.page}
      extra={[
        <Button
          key="alerts"
          icon={<BellOutlined />}
          onClick={() => history.push('/ops/task-center/alerts?source=system')}
        >
          {tr("查看系统告警")}
        </Button>,
        <Tooltip key="reload" title={tr("刷新运行状态")}>
          <Button
            aria-label={tr("刷新运行状态")}
            icon={<ReloadOutlined />}
            onClick={() => void load()}
            loading={loading}
          />
        </Tooltip>,
      ]}
    >
      {loadError ? (
        <ErrorAlert
          title={tr("可观测性概览加载失败")}
          actionHint={tr("请检查后端健康状态后重试。")}
          className={styles.error}
        />
      ) : null}

      {!overview && loading ? (
        <div className={styles.loading} aria-label={tr("正在加载可观测性概览")}>
          <Spin size="large" />
        </div>
      ) : null}

      {overview ? <Spin spinning={loading}>
        <div className={styles.content}>
          <div className={styles.metricsGrid}>
            <MetricCard
              title={tr("总体运行状态")}
              value={overallMeta.text}
              description={trFormat("环境：{value0}", { value0: String(overview?.environment || tr('未知')) })}
              icon={overview?.overallStatus === 'healthy' ? <CheckCircleOutlined /> : <ExclamationCircleOutlined />}
              intent={overallMeta.intent}
            />
            <MetricCard
              title={tr("活跃系统告警")}
              value={alertValue}
              description={alertDescription}
              icon={<BellOutlined />}
              intent={alertIntent}
              onClick={() => history.push('/ops/task-center/alerts?source=system')}
            />
            <MetricCard
              title={tr("告警评估器")}
              value={evaluationMeta.text}
              description={trFormat("已检查 {value0} 条规则", { value0: String(overview?.evaluation?.rulesChecked ?? 0) })}
              icon={<SafetyCertificateOutlined />}
              intent={evaluationMeta.intent}
            />
            <MetricCard
              title={tr("指标注册表")}
              value={metricMeta.text}
              description={overview?.metrics?.internalOnly ? tr("内部端点已保护") : tr("端点未限制内部访问")}
              icon={<CloudServerOutlined />}
              intent={metricMeta.intent}
            />
          </div>

          <div className={styles.detailGrid}>
            <SectionCard title={tr("告警与服务目标")} variant="outlined" compact>
              <div className={styles.statusList}>
                <div className={styles.statusRow}>
                  <div>
                    <Text strong>{tr("告警规则评估")}</Text>
                    <Text type="secondary">{tr("窗口指标、样本保护与恢复检测")}</Text>
                  </div>
                  <StatusValue status={overview?.evaluation?.status} />
                </div>
                <div className={styles.statusRow}>
                  <div>
                    <Text strong>{tr("SLO 评估")}</Text>
                    <Text type="secondary">{tr("可用性、延迟和关键后台任务目标")}</Text>
                  </div>
                  <StatusValue status={overview?.slo?.status} />
                </div>
              </div>
              <Descriptions className={styles.descriptions} column={{ xs: 1, sm: 2 }} size="small">
                <Descriptions.Item label={tr("最近告警评估")}>
                  {formatDateTime(overview?.evaluation?.lastEvaluatedAt)}
                </Descriptions.Item>
                <Descriptions.Item label={tr("最近 SLO 评估")}>
                  {formatDateTime(overview?.slo?.lastEvaluatedAt)}
                </Descriptions.Item>
                <Descriptions.Item label={tr("本轮触发")}>
                  {overview?.evaluation?.alertsFired ?? 0}
                </Descriptions.Item>
                <Descriptions.Item label={tr("本轮恢复")}>
                  {overview?.evaluation?.alertsResolved ?? 0}
                </Descriptions.Item>
              </Descriptions>
            </SectionCard>

            <SectionCard title={tr("指标与遥测")} variant="outlined" compact>
              <div className={styles.statusList}>
                <div className={styles.statusRow}>
                  <div>
                    <Text strong>{tr("Prometheus 指标")}</Text>
                    <Text type="secondary" copyable={{ text: overview?.metrics?.path || '' }}>
                      {overview?.metrics?.path || tr("未配置路径")}
                    </Text>
                  </div>
                  <StatusValue status={overview?.metrics?.status} />
                </div>
                <div className={styles.statusRow}>
                  <div>
                    <Text strong>{tr("遥测导出")}</Text>
                    <Text type="secondary">{overview?.telemetry?.protocol || tr("未配置协议")}</Text>
                  </div>
                  <StatusValue status={overview?.telemetry?.status} />
                </div>
              </div>
              <Descriptions className={styles.descriptions} column={{ xs: 1, sm: 2 }} size="small">
                <Descriptions.Item label={tr("导出成功")}>
                  {overview?.telemetry?.exportSuccess ?? 0}
                </Descriptions.Item>
                <Descriptions.Item label={tr("导出失败")}>
                  {overview?.telemetry?.exportFailures ?? 0}
                </Descriptions.Item>
                <Descriptions.Item label={tr("失败丢弃")}>
                  {overview?.telemetry?.dropped ?? 0}
                </Descriptions.Item>
              </Descriptions>
            </SectionCard>
          </div>

          <div className={styles.runtimeBand}>
            <div className={styles.runtimeItems}>
              <span><Text type="secondary">{tr("模式")}</Text><Text strong>{overview?.mode || tr("未知")}</Text></span>
              <span><Text type="secondary">{tr("环境")}</Text><Text strong>{overview?.environment || tr("未知")}</Text></span>
              <span><Text type="secondary">{tr("指标端点")}</Text><Text strong>{overview?.metrics?.internalOnly ? tr("仅内部访问") : tr("未保护")}</Text></span>
              <span><Text type="secondary">{tr("最近刷新")}</Text><Text strong>{lastUpdated}</Text></span>
            </div>
            <StatusValue status={overview?.telemetry?.status} />
          </div>
        </div>
      </Spin> : null}
    </TmPageContainer>
  );
}
