import { Link } from '@umijs/renderer-react';
import { history } from '@umijs/max';
import {
  CheckCircleOutlined,
  LockOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  SaveOutlined,
} from '@ant-design/icons';
import { ProCard } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import type { ProColumns } from '@ant-design/pro-components';
import {
  Alert,
  Button,
  Col,
  Descriptions,
  Form,
  Input,
  InputNumber,
  List,
  Modal,
  Progress,
  Row,
  Select,
  Space,
  Statistic,
  Switch,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { usePermission } from '@/hooks/usePermission';
import { PERMISSIONS } from '@/utils/permission';
import {
  SECURITY_FIELD_HELP,
  SECURITY_FIELD_LABEL,
  SECURITY_FIELD_PLACEHOLDER,
  SECURITY_SESSION_TIMEOUT_PRESETS,
} from '@/constants/securitySettings';
import { fetchConfigStatusOverview, type ConfigStatusItem } from '@/services/configStatus';
import { fetchSettingsList, saveSettingsItems } from '@/services/settings';
import {
  fetchAuditIntegrityStatus,
  fetchAuthSessions,
  fetchFileSecurityStats,
  fetchKeyReferences,
  fetchKeyRotationJob,
  fetchKeyRotationProgress,
  fetchKeyRotationStatus,
  fetchSecurityOverview,
  logoutAllSessions,
  pauseKeyRotation,
  prepareKeyRotation,
  resumeKeyRotation,
  revokeAuthSession,
  revokeOtherAuthSessions,
  startKeyRotation,
  verifyAuditIntegrity,
  verifyKeyRotation,
  type AuditIntegrityStatus,
  type AuthSessionRow,
  type FileSecurityStats,
  type KeyRotationJob,
  type KeyRotationStatus,
  type SecretReferenceCount,
  type SecurityOverview,
} from '@/services/security';
import { pickGroup, toPutItems, type FieldSpec } from '@/utils/settingsForm';
import { formatDateTime } from '@/utils/formatTime';

const { Paragraph, Text, Title } = Typography;

const GROUP = 'security';

const FIELDS: Record<string, FieldSpec> = {
  session_idle_timeout_min: {},
  force_https: {},
  ops_webhook_secret: { encrypted: true },
};

const ROTATION_PREPARE_PHRASE = 'ROTATE-KEYS-DRY-RUN';
const ROTATION_START_PHRASE = 'ROTATE-KEYS-START';

const TENANT_ISOLATION_FALLBACK: ConfigStatusItem[] = [
  {
    key: 'p4.tenant_context',
    title: 'Tenant Context',
    status: 'ready',
    summary: 'JWT tenant_id + TenantContext',
  },
  {
    key: 'p4.shop_scope',
    title: 'Shop Scope',
    status: 'ready',
    summary: 'adminperm 店铺授权过滤',
  },
  {
    key: 'p4.master_key_ring',
    title: 'Master Key Ring',
    status: 'ready',
    summary: 'enc:v2 密钥版本与轮换 API',
  },
  {
    key: 'p4.audit_chain',
    title: '审计 Hash Chain',
    status: 'ready',
    summary: 'operation_logs 链接哈希',
  },
  {
    key: 'p4.upload_validation',
    title: '上传安全',
    status: 'ready',
    summary: 'MIME/解码/像素限制',
  },
];

const FILE_SECURITY_STATUS_LABEL: Record<string, string> = {
  clean: '已通过',
  pending_scan: '待扫描',
  scanning: '扫描中',
  rejected: '已拒绝',
  quarantined: '已隔离',
  scan_failed: '扫描失败',
  uploaded: '已上传',
  unknown: '未知',
};

const ROTATION_STATUS_LABEL: Record<string, { text: string; color: string }> = {
  prepared: { text: '已准备', color: 'default' },
  dry_run_completed: { text: '预检完成', color: 'processing' },
  running: { text: '运行中', color: 'processing' },
  paused: { text: '已暂停', color: 'warning' },
  completed: { text: '已完成', color: 'success' },
  completed_with_warning: { text: '完成（有警告）', color: 'warning' },
  failed: { text: '失败', color: 'error' },
  verification_pending: { text: '待验证', color: 'warning' },
  verified: { text: '已验证', color: 'success' },
};

function truthyStored(v: string | undefined): boolean {
  const s = String(v ?? '')
    .trim()
    .toLowerCase();
  return s === '1' || s === 'true' || s === 'yes' || s === 'on';
}

function configStatusTag(status: string) {
  const s = (status || '').toLowerCase();
  if (s === 'ready' || s.includes('已配置') || s.includes('运行中')) {
    return <Tag color="success">{status || 'ready'}</Tag>;
  }
  if (s === 'not_ready' || s.includes('异常')) {
    return <Tag color="error">{status}</Tag>;
  }
  if (s === 'ready_with_warning' || s.includes('待') || s.includes('manual')) {
    return <Tag color="warning">{status}</Tag>;
  }
  return <Tag>{status || 'unknown'}</Tag>;
}

function sessionStatusTag(status: string, isCurrent: boolean) {
  if (isCurrent) {
    return <Tag color="blue">当前会话</Tag>;
  }
  const s = (status || '').toLowerCase();
  if (s === 'active') return <Tag color="success">活跃</Tag>;
  if (s === 'revoked') return <Tag>已撤销</Tag>;
  if (s === 'expired') return <Tag color="default">已过期</Tag>;
  return <Tag>{status}</Tag>;
}

function SecurityToggleCard({
  name,
  label,
  extra,
}: {
  name: string;
  label: string;
  extra: string;
}) {
  return (
    <div className="tm-system-settings__toggle-card">
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12 }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <Text className="tm-system-settings__toggle-label">{label}</Text>
          <Text type="secondary" className="tm-system-settings__toggle-extra">
            {extra}
          </Text>
        </div>
        <Form.Item name={name} valuePropName="checked" style={{ marginBottom: 0, flexShrink: 0 }}>
          <Switch />
        </Form.Item>
      </div>
    </div>
  );
}

export default function SecuritySettingsPage() {
  const { user, can } = usePermission();
  const canRotateKeys = user?.role === 'admin' && can(PERMISSIONS.SECURITY_KEY_ROTATE);
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [overview, setOverview] = useState<SecurityOverview | null>(null);
  const [sessions, setSessions] = useState<AuthSessionRow[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(false);
  const [tenantItems, setTenantItems] = useState<ConfigStatusItem[]>(TENANT_ISOLATION_FALLBACK);
  const [rotationStatus, setRotationStatus] = useState<KeyRotationStatus | null>(null);
  const [rotationJob, setRotationJob] = useState<KeyRotationJob | null>(null);
  const [keyReferences, setKeyReferences] = useState<SecretReferenceCount[]>([]);
  const [rotationLoading, setRotationLoading] = useState(false);
  const [fileStats, setFileStats] = useState<FileSecurityStats | null>(null);
  const [auditStatus, setAuditStatus] = useState<AuditIntegrityStatus | null>(null);
  const [auditDays, setAuditDays] = useState(7);
  const [auditLoading, setAuditLoading] = useState(false);
  const [centerLoading, setCenterLoading] = useState(false);
  const [prepareOpen, setPrepareOpen] = useState(false);
  const [startOpen, setStartOpen] = useState(false);
  const [confirmPhrase, setConfirmPhrase] = useState('');
  const rotationPollRef = useRef<number | null>(null);

  const loadSettings = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const g = pickGroup(items, GROUP);
      form.setFieldsValue({
        session_idle_timeout_min: g.session_idle_timeout_min ? Number(g.session_idle_timeout_min) : 60,
        force_https: truthyStored(g.force_https),
        ops_webhook_secret: g.ops_webhook_secret || '',
      });
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  }, [form]);

  const loadSessions = useCallback(async () => {
    setSessionsLoading(true);
    try {
      const res = await fetchAuthSessions();
      setSessions(res.items || []);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '会话列表加载失败');
      setSessions([]);
    } finally {
      setSessionsLoading(false);
    }
  }, []);

  const loadTenantIsolation = useCallback(async () => {
    try {
      const res = await fetchConfigStatusOverview();
      const filtered = (res.items || []).filter(
        (item) =>
          item.key.startsWith('p4.') ||
          item.key.startsWith('p32_webhook') ||
          item.title.toLowerCase().includes('tenant'),
      );
      if (filtered.length > 0) {
        setTenantItems(filtered);
      }
    } catch {
      setTenantItems(TENANT_ISOLATION_FALLBACK);
    }
  }, []);

  const loadRotation = useCallback(async () => {
    if (!canRotateKeys) {
      setRotationStatus(null);
      setRotationJob(null);
      setKeyReferences([]);
      return;
    }
    setRotationLoading(true);
    try {
      const [status, refs] = await Promise.all([fetchKeyRotationStatus(), fetchKeyReferences()]);
      setRotationStatus(status);
      setKeyReferences(refs.items || []);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '密钥轮换状态加载失败');
    } finally {
      setRotationLoading(false);
    }
  }, [canRotateKeys]);

  const loadFileStats = useCallback(async () => {
    try {
      const stats = await fetchFileSecurityStats();
      setFileStats(stats);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '文件安全统计加载失败');
      setFileStats(null);
    }
  }, []);

  const loadAudit = useCallback(async () => {
    setAuditLoading(true);
    try {
      const status = await fetchAuditIntegrityStatus();
      setAuditStatus(status);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '审计完整性状态加载失败');
      setAuditStatus(null);
    } finally {
      setAuditLoading(false);
    }
  }, []);

  const loadSecurityCenter = useCallback(async () => {
    setCenterLoading(true);
    try {
      const ov = await fetchSecurityOverview();
      setOverview(ov);
      await Promise.all([loadSessions(), loadTenantIsolation(), loadRotation(), loadFileStats(), loadAudit()]);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '安全中心数据加载失败');
    } finally {
      setCenterLoading(false);
    }
  }, [loadAudit, loadFileStats, loadRotation, loadSessions, loadTenantIsolation]);

  const pollRotationJob = useCallback(async (jobId: string) => {
    try {
      const job = await fetchKeyRotationProgress(jobId);
      setRotationJob(job);
      if (job.status === 'running') {
        rotationPollRef.current = window.setTimeout(() => {
          void pollRotationJob(jobId);
        }, 3000);
      }
    } catch {
      // ignore polling errors
    }
  }, []);

  useEffect(() => {
    void loadSettings();
    void loadSecurityCenter();
    return () => {
      if (rotationPollRef.current != null) {
        window.clearTimeout(rotationPollRef.current);
      }
    };
  }, [loadSecurityCenter, loadSettings]);

  const rotationProgressPercent = useMemo(() => {
    if (!rotationJob || rotationJob.totalRecords <= 0) return 0;
    return Math.min(100, Math.round((rotationJob.processedRecords / rotationJob.totalRecords) * 100));
  }, [rotationJob]);

  const sessionColumns: ProColumns<AuthSessionRow>[] = [
    {
      title: '设备',
      dataIndex: 'deviceSummary',
      ellipsis: true,
      render: (_, row) => row.deviceSummary || row.userAgentSummary || '—',
    },
    {
      title: '浏览器',
      dataIndex: 'browserSummary',
      ellipsis: true,
      width: 140,
      render: (v) => v || '—',
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 120,
      render: (_, row) => sessionStatusTag(row.status, row.isCurrent),
    },
    {
      title: '最近活动',
      dataIndex: 'lastActivityAt',
      width: 180,
      render: (_, row) => formatDateTime(row.lastActivityAt || row.createdAt),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 100,
      render: (_, row) =>
        row.isCurrent ? (
          <Text type="secondary">当前</Text>
        ) : (
          <Button
            type="link"
            size="small"
            danger
            onClick={() => {
              Modal.confirm({
                title: '撤销此会话',
                content: '该设备将被强制退出登录。',
                onOk: async () => {
                  await revokeAuthSession(row.id);
                  message.success('会话已撤销');
                  await loadSessions();
                },
              });
            }}
          >
            撤销
          </Button>
        ),
    },
  ];

  const keyRefColumns: ProColumns<SecretReferenceCount>[] = [
    { title: '表', dataIndex: 'tableName', width: 140 },
    { title: '字段', dataIndex: 'fieldName', width: 140 },
    { title: 'Key ID', dataIndex: 'keyId', width: 120, ellipsis: true },
    { title: '引用数', dataIndex: 'referenceCount', width: 90 },
    {
      title: '解密失败',
      dataIndex: 'decryptFailures',
      width: 90,
      render: (v) => (Number(v) > 0 ? <Text type="danger">{v}</Text> : v),
    },
    { title: '未知格式', dataIndex: 'unknownFormat', width: 90 },
  ];

  const onFinish = async (values: Record<string, unknown>) => {
    try {
      const payload = {
        session_idle_timeout_min: String(values.session_idle_timeout_min ?? ''),
        force_https: values.force_https ? 'true' : 'false',
        ops_webhook_secret: values.ops_webhook_secret ?? '',
      };
      await saveSettingsItems(toPutItems(GROUP, FIELDS, payload));
      message.success('已保存');
      await loadSettings();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '保存失败');
    }
  };

  const handlePrepareRotation = async () => {
    try {
      const status = await prepareKeyRotation(confirmPhrase);
      setRotationStatus(status);
      message.success('密钥轮换预检完成');
      setPrepareOpen(false);
      setConfirmPhrase('');
    } catch (e: unknown) {
      message.error((e as Error)?.message || '预检失败');
    }
  };

  const handleStartRotation = async () => {
    try {
      const job = await startKeyRotation(confirmPhrase);
      setRotationJob(job);
      message.success('密钥轮换任务已启动');
      setStartOpen(false);
      setConfirmPhrase('');
      if (job.id) {
        void pollRotationJob(job.id);
      }
      await loadRotation();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '启动失败');
    }
  };

  return (
    <TmPageContainer
      title="安全设置"
      subTitle="认证会话、租户隔离、密钥轮换、文件安全与审计完整性"
    >
      <div className="tm-system-settings">
        <ProCard variant="outlined" className="tm-system-settings__hero">
          <div className="tm-system-settings__hero-inner">
            <div className="tm-system-settings__hero-icon">
              <LockOutlined />
            </div>
            <div className="tm-system-settings__hero-body">
              <Title level={5} className="tm-system-settings__hero-title">
                安全中心
              </Title>
              <Paragraph type="secondary" className="tm-system-settings__hero-desc">
                查看认证与会话状态、租户隔离就绪项、主密钥轮换进度、上传文件安全扫描统计，以及操作审计链完整性。
                基础策略（空闲超时、HTTPS、回调签名）可在下方配置区保存。
              </Paragraph>
            </div>
            <Button icon={<ReloadOutlined />} onClick={() => void loadSecurityCenter()} loading={centerLoading}>
              刷新安全中心
            </Button>
          </div>
        </ProCard>

        {overview ? (
          <ProCard variant="outlined" title="运行概览" style={{ marginTop: 16 }} loading={centerLoading}>
            <Row gutter={[16, 16]}>
              <Col xs={12} sm={8} md={6}>
                <Statistic title="认证模式" value={overview.authSessionMode || '—'} />
              </Col>
              <Col xs={12} sm={8} md={6}>
                <Statistic title="活跃会话" value={overview.activeSessionCount} suffix="个" />
              </Col>
              <Col xs={12} sm={8} md={6}>
                <Statistic title="Access TTL" value={overview.accessTokenTTLMinutes} suffix="分钟" />
              </Col>
              <Col xs={12} sm={8} md={6}>
                <Statistic title="Refresh TTL" value={overview.refreshTokenTTLDays} suffix="天" />
              </Col>
              <Col xs={12} sm={8} md={6}>
                <Statistic title="JWT Key" value={overview.jwtActiveKeyId} />
              </Col>
              <Col xs={12} sm={8} md={6}>
                <Statistic title="Master Key" value={overview.appMasterActiveKeyId} />
              </Col>
            </Row>
            {overview.productionDebugSurface ? (
              <Alert
                type="warning"
                showIcon
                style={{ marginTop: 16 }}
                message="生产调试面已开启"
                description="Swagger、调试接口或 Dev 路由仍处于启用状态，生产环境应关闭。"
              />
            ) : null}
          </ProCard>
        ) : null}

        <ProCard
          variant="outlined"
          title="认证与会话"
          style={{ marginTop: 16 }}
          extra={
            <Space wrap>
              <Button
                onClick={() => {
                  Modal.confirm({
                    title: '撤销其他会话',
                    content: '除当前浏览器外，其他已登录设备将被强制退出。',
                    onOk: async () => {
                      const res = await revokeOtherAuthSessions();
                      message.success(`已撤销 ${res.revoked} 个会话`);
                      await loadSessions();
                    },
                  });
                }}
              >
                撤销其他会话
              </Button>
              <Button
                danger
                onClick={() => {
                  Modal.confirm({
                    title: '全部登出',
                    content: '包括当前会话在内的所有设备将被强制退出，需重新登录。',
                    okType: 'danger',
                    onOk: async () => {
                      await logoutAllSessions();
                      message.success('已全部登出');
                      history.push('/user/login');
                    },
                  });
                }}
              >
                全部登出
              </Button>
              <Button icon={<ReloadOutlined />} onClick={() => void loadSessions()} loading={sessionsLoading}>
                刷新
              </Button>
            </Space>
          }
        >
          <ProTable<AuthSessionRow>
            rowKey="id"
            search={false}
            options={false}
            pagination={false}
            loading={sessionsLoading}
            dataSource={sessions}
            columns={sessionColumns}
            size="small"
          />
        </ProCard>

        <ProCard variant="outlined" title="租户隔离状态" style={{ marginTop: 16 }}>
          <Paragraph type="secondary" style={{ marginBottom: 12 }}>
            以下项来自配置状态中心的安全检查。完整清单见{' '}
            <Link to="/settings/config-status">配置状态</Link>。
          </Paragraph>
          <List
            dataSource={tenantItems}
            renderItem={(item) => (
              <List.Item>
                <List.Item.Meta
                  avatar={<CheckCircleOutlined style={{ color: 'var(--ant-color-success)', fontSize: 18 }} />}
                  title={
                    <Space>
                      <span>{item.title}</span>
                      {configStatusTag(item.status)}
                    </Space>
                  }
                  description={item.summary || item.nextAction}
                />
              </List.Item>
            )}
          />
        </ProCard>

        <ProCard
          variant="outlined"
          title="主密钥轮换"
          style={{ marginTop: 16 }}
          extra={
            <Space wrap>
              <Button disabled={!canRotateKeys} onClick={() => setPrepareOpen(true)}>预检（Dry Run）</Button>
              <Button disabled={!canRotateKeys} type="primary" onClick={() => setStartOpen(true)}>
                启动轮换
              </Button>
              <Button disabled={!canRotateKeys} icon={<ReloadOutlined />} onClick={() => void loadRotation()} loading={rotationLoading}>
                刷新
              </Button>
            </Space>
          }
        >
          {!canRotateKeys ? <Alert type="info" showIcon message="仅全局系统管理员可管理密钥轮换" /> : null}
          {rotationStatus ? (
            <Descriptions size="small" column={{ xs: 1, sm: 2, md: 3 }} style={{ marginBottom: 16 }}>
              <Descriptions.Item label="当前 Key ID">{rotationStatus.activeKeyId}</Descriptions.Item>
              <Descriptions.Item label="待重加密">{rotationStatus.pendingReencrypt}</Descriptions.Item>
              <Descriptions.Item label="历史 Key 数">{rotationStatus.previousKeyCount}</Descriptions.Item>
            </Descriptions>
          ) : null}

          {canRotateKeys && rotationJob ? (
            <div style={{ marginBottom: 16 }}>
              <Space style={{ marginBottom: 8 }}>
                <Text strong>当前任务</Text>
                <Tag color={ROTATION_STATUS_LABEL[rotationJob.status]?.color || 'default'}>
                  {ROTATION_STATUS_LABEL[rotationJob.status]?.text || rotationJob.status}
                </Tag>
                <Text type="secondary">{rotationJob.id}</Text>
              </Space>
              <Progress percent={rotationProgressPercent} status={rotationJob.status === 'failed' ? 'exception' : 'active'} />
              <Row gutter={16} style={{ marginTop: 8 }}>
                <Col>
                  <Text type="secondary">
                    已处理 {rotationJob.processedRecords} / {rotationJob.totalRecords}，成功{' '}
                    {rotationJob.reencryptedRecords}，跳过 {rotationJob.skippedRecords}，失败 {rotationJob.failedRecords}
                  </Text>
                </Col>
              </Row>
              <Space style={{ marginTop: 12 }} wrap>
                {rotationJob.status === 'running' ? (
                  <Button
                    onClick={async () => {
                      await pauseKeyRotation(rotationJob.id);
                      message.success('已暂停');
                      const job = await fetchKeyRotationJob(rotationJob.id);
                      setRotationJob(job);
                    }}
                  >
                    暂停
                  </Button>
                ) : null}
                {rotationJob.status === 'paused' ? (
                  <Button
                    type="primary"
                    onClick={async () => {
                      await resumeKeyRotation(rotationJob.id);
                      message.success('已恢复');
                      void pollRotationJob(rotationJob.id);
                    }}
                  >
                    恢复
                  </Button>
                ) : null}
                <Button
                  icon={<SafetyCertificateOutlined />}
                  onClick={async () => {
                    const res = await verifyKeyRotation(rotationJob.id);
                    message.success(res.ok ? '验证通过' : '验证未通过');
                    const job = await fetchKeyRotationJob(rotationJob.id);
                    setRotationJob(job);
                  }}
                >
                  验证轮换
                </Button>
              </Space>
            </div>
          ) : null}

          <ProTable<SecretReferenceCount>
            headerTitle="旧密钥引用统计"
            rowKey={(row) => `${row.tableName}-${row.fieldName}-${row.keyId}-${row.tenantId}`}
            search={false}
            options={false}
            pagination={false}
            loading={rotationLoading}
            dataSource={keyReferences}
            columns={keyRefColumns}
            size="small"
          />
        </ProCard>

        <ProCard
          variant="outlined"
          title="文件安全"
          style={{ marginTop: 16 }}
          extra={
            <Button icon={<ReloadOutlined />} onClick={() => void loadFileStats()}>
              刷新
            </Button>
          }
        >
          {fileStats ? (
            <>
              <Row gutter={[16, 16]}>
                <Col xs={12} sm={8} md={6}>
                  <Statistic title="文件总数" value={fileStats.total} />
                </Col>
                {Object.entries(fileStats.byStatus)
                  .filter(([, count]) => count > 0)
                  .map(([status, count]) => (
                    <Col xs={12} sm={8} md={6} key={status}>
                      <Statistic
                        title={FILE_SECURITY_STATUS_LABEL[status] || status}
                        value={count}
                        valueStyle={status === 'quarantined' || status === 'rejected' ? { color: 'var(--ant-color-error)' } : undefined}
                      />
                    </Col>
                  ))}
              </Row>
              {fileStats.partial ? (
                <Alert
                  type="info"
                  showIcon
                  style={{ marginTop: 16 }}
                  message="部分统计"
                  description={`状态分布基于最近 ${fileStats.sampled} 条文件记录估算；总数 ${fileStats.total}。完整明细请查看文件管理。`}
                />
              ) : null}
            </>
          ) : (
            <Text type="secondary">暂无文件安全统计数据</Text>
          )}
        </ProCard>

        <ProCard
          variant="outlined"
          title="审计完整性"
          style={{ marginTop: 16 }}
          extra={
            <Space wrap>
              <Select
                value={auditDays}
                style={{ width: 120 }}
                options={[
                  { label: '近 7 天', value: 7 },
                  { label: '近 14 天', value: 14 },
                  { label: '近 30 天', value: 30 },
                ]}
                onChange={setAuditDays}
              />
              <Button
                type="primary"
                loading={auditLoading}
                onClick={async () => {
                  setAuditLoading(true);
                  try {
                    const res = await verifyAuditIntegrity(auditDays);
                    setAuditStatus(res);
                    message.success(`已校验 ${res.checked} 条审计记录`);
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || '校验失败');
                  } finally {
                    setAuditLoading(false);
                  }
                }}
              >
                执行校验
              </Button>
              <Button icon={<ReloadOutlined />} onClick={() => void loadAudit()} loading={auditLoading}>
                刷新状态
              </Button>
            </Space>
          }
        >
          {auditStatus ? (
            <Space direction="vertical" style={{ width: '100%' }}>
              <Space>
                {auditStatus.ok ? (
                  <Tag icon={<CheckCircleOutlined />} color="success">
                    链完整
                  </Tag>
                ) : (
                  <Tag color="error">链异常</Tag>
                )}
                <Text>已检查 {auditStatus.checked} 条 operation_logs 记录</Text>
              </Space>
              <Button type="link" style={{ padding: 0 }} onClick={() => history.push('/system/operation-logs')}>
                查看操作日志
              </Button>
            </Space>
          ) : (
            <Text type="secondary">加载审计完整性状态中…</Text>
          )}
        </ProCard>

        <Form form={form} layout="vertical" onFinish={onFinish}>
          <ProCard variant="outlined" title="策略配置" className="tm-system-settings__panel" style={{ marginTop: 16 }}>
            <Row gutter={[24, 0]}>
              <Col xs={24} md={12} lg={10}>
                <Form.Item
                  label={SECURITY_FIELD_LABEL.sessionIdleTimeoutMin}
                  name="session_idle_timeout_min"
                  rules={[
                    { required: true, message: '请填写会话空闲超时' },
                    { type: 'number', min: 5, max: 10080, message: '范围 5–10080 分钟' },
                  ]}
                  extra={SECURITY_FIELD_HELP.sessionIdleTimeoutMin}
                >
                  <InputNumber min={5} max={10080} style={{ width: '100%' }} suffix="分钟" />
                </Form.Item>
              </Col>
              <Col xs={24} md={12} lg={10}>
                <Form.Item label="快捷选择" style={{ marginBottom: 0 }}>
                  <Select
                    placeholder="选择常用时长"
                    allowClear
                    options={SECURITY_SESSION_TIMEOUT_PRESETS.map((p) => ({ label: p.label, value: p.value }))}
                    onChange={(v) => {
                      if (typeof v === 'number') {
                        form.setFieldValue('session_idle_timeout_min', v);
                      }
                    }}
                  />
                </Form.Item>
              </Col>
            </Row>
          </ProCard>

          <ProCard variant="outlined" title="传输安全" className="tm-system-settings__panel" style={{ marginTop: 16 }}>
            <Row gutter={[16, 16]}>
              <Col xs={24} md={16} lg={14}>
                <SecurityToggleCard
                  name="force_https"
                  label={SECURITY_FIELD_LABEL.forceHttps}
                  extra={SECURITY_FIELD_HELP.forceHttps}
                />
              </Col>
            </Row>
          </ProCard>

          <ProCard variant="outlined" title="回调签名校验" className="tm-system-settings__panel" style={{ marginTop: 16 }}>
            <Paragraph type="secondary" style={{ marginBottom: 16, fontSize: 13 }}>
              用于验证外部系统向贸灵发起的运维类回调请求，与告警通知中的回调密钥相互独立。
            </Paragraph>
            <Row gutter={[24, 0]}>
              <Col xs={24} md={14} lg={12}>
                <Form.Item
                  label={SECURITY_FIELD_LABEL.opsWebhookSecret}
                  name="ops_webhook_secret"
                  extra={SECURITY_FIELD_HELP.opsWebhookSecret}
                >
                  <Input.Password autoComplete="new-password" placeholder={SECURITY_FIELD_PLACEHOLDER.opsWebhookSecret} />
                </Form.Item>
              </Col>
            </Row>
          </ProCard>

          <ProCard variant="outlined" className="tm-system-settings__footer" style={{ marginTop: 16 }}>
            <Space wrap>
              <Button type="primary" htmlType="submit" loading={loading} icon={<SaveOutlined />}>
                保存配置
              </Button>
              <Button icon={<ReloadOutlined />} onClick={() => void loadSettings()} disabled={loading}>
                重新加载
              </Button>
            </Space>
          </ProCard>
        </Form>
      </div>

      <Modal
        title="密钥轮换预检"
        open={canRotateKeys && prepareOpen}
        onCancel={() => {
          setPrepareOpen(false);
          setConfirmPhrase('');
        }}
        onOk={() => void handlePrepareRotation()}
        okButtonProps={{ disabled: confirmPhrase !== ROTATION_PREPARE_PHRASE }}
      >
        <Paragraph type="secondary">
          预检不会修改数据，仅统计待重加密的敏感字段数量。请输入确认短语{' '}
          <Text code>{ROTATION_PREPARE_PHRASE}</Text>。
        </Paragraph>
        <Input
          value={confirmPhrase}
          onChange={(e) => setConfirmPhrase(e.target.value)}
          placeholder={ROTATION_PREPARE_PHRASE}
        />
      </Modal>

      <Modal
        title="启动密钥轮换"
        open={canRotateKeys && startOpen}
        onCancel={() => {
          setStartOpen(false);
          setConfirmPhrase('');
        }}
        onOk={() => void handleStartRotation()}
        okButtonProps={{ disabled: confirmPhrase !== ROTATION_START_PHRASE, danger: true }}
        okText="确认启动"
      >
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 12 }}
          message="高风险操作"
          description="将启动后台重加密任务，期间请避免修改 Master Key 配置。"
        />
        <Paragraph type="secondary">
          请输入确认短语 <Text code>{ROTATION_START_PHRASE}</Text>。
        </Paragraph>
        <Input
          value={confirmPhrase}
          onChange={(e) => setConfirmPhrase(e.target.value)}
          placeholder={ROTATION_START_PHRASE}
        />
      </Modal>
    </TmPageContainer>
  );
}
