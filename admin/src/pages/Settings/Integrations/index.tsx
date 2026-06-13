import { Link } from '@umijs/renderer-react';
import {
  ApiOutlined,
  CloudOutlined,
  MailOutlined,
  PictureOutlined,
  RightOutlined,
  RobotOutlined,
} from '@ant-design/icons';
import { ProCard } from '@ant-design/pro-components';
import { TmPageContainer } from '@/components/ui';
import { Alert, Col, Row, Space, Spin, Statistic, Tag, Typography } from 'antd';
import type { ComponentType, CSSProperties, ReactNode } from 'react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { PAGE_COPY } from '@/constants/copywriting';
import {
  aiTextProviderLabel,
  imageProviderLabel,
  imageSubServiceStatusLabel,
  integrationConfiguredTag,
  storageKindLabel,
} from '@/constants/integrations';
import { PLATFORM_PROVIDER_STATUS } from '@/constants/status';
import { preferredPlatformTabOrder } from '@/services/platformOpen';
import { fetchIntegrationsOverview, type IntegrationOverviewData } from '@/services/settings';

const { Text, Paragraph, Title } = Typography;

type HubCardProps = {
  title: string;
  desc: string;
  configured: boolean;
  to: string;
  Icon: ComponentType<{ style?: CSSProperties }>;
  extra?: ReactNode;
};

function IntegrationHubCard({ title, desc, configured, to, Icon, extra }: HubCardProps) {
  const tag = integrationConfiguredTag(configured);
  return (
    <div
      style={{
        height: '100%',
        padding: '20px 20px 16px',
        borderRadius: 8,
        border: '1px solid rgba(0,0,0,0.06)',
        background: '#fff',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      <Space align="start" size="middle" style={{ marginBottom: 12 }}>
        <div
          style={{
            width: 40,
            height: 40,
            borderRadius: 8,
            background: 'linear-gradient(135deg, #e6f4ff 0%, #f0f5ff 100%)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            flexShrink: 0,
          }}
        >
          <Icon style={{ fontSize: 20, color: '#1677ff' }} />
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <Space wrap size={[8, 4]}>
            <Title level={5} style={{ margin: 0 }}>
              {title}
            </Title>
            <Tag color={tag.color}>{tag.text}</Tag>
          </Space>
          <Paragraph type="secondary" style={{ margin: '6px 0 0', fontSize: 13 }}>
            {desc}
          </Paragraph>
        </div>
      </Space>
      {extra ? <div style={{ flex: 1, marginBottom: 12 }}>{extra}</div> : <div style={{ flex: 1 }} />}
      <Link to={to} style={{ fontSize: 13 }}>
        前往配置 <RightOutlined style={{ fontSize: 11 }} />
      </Link>
    </div>
  );
}

type PlatformCardProps = {
  name: string;
  appConfigured: boolean;
  status: string;
};

function PlatformIntegrationCard({ name, appConfigured, status }: PlatformCardProps) {
  const cfg = integrationConfiguredTag(appConfigured);
  const runtime = PLATFORM_PROVIDER_STATUS[status as keyof typeof PLATFORM_PROVIDER_STATUS];
  return (
    <div
      style={{
        height: '100%',
        padding: '16px 16px 14px',
        borderRadius: 8,
        border: '1px solid rgba(0,0,0,0.06)',
        background: '#fff',
      }}
    >
      <Space wrap size={[8, 4]} style={{ marginBottom: 8 }}>
        <Text strong>{name}</Text>
        <Tag color={cfg.color}>{cfg.text}</Tag>
      </Space>
      <div style={{ marginBottom: 10 }}>
        <Text type="secondary" style={{ fontSize: 12 }}>
          运行时{' '}
        </Text>
        <Tag color={runtime?.color}>{runtime?.text ?? status}</Tag>
      </div>
      <Link to="/settings/platforms" style={{ fontSize: 13 }}>
        编辑应用参数 <RightOutlined style={{ fontSize: 11 }} />
      </Link>
    </div>
  );
}

export default function IntegrationsHubPage() {
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<IntegrationOverviewData | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const row = await fetchIntegrationsOverview();
      setData(row);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const sortedPlatforms = useMemo(() => {
    if (!data?.platforms?.length) return [];
    return [...data.platforms].sort(
      (a, b) => preferredPlatformTabOrder(a.platform) - preferredPlatformTabOrder(b.platform) || a.name.localeCompare(b.name),
    );
  }, [data?.platforms]);

  const summary = useMemo(() => {
    if (!data) return { configured: 0, total: 0 };
    const core = [data.ai.configured, data.storage.configured, data.mail.configured];
    const imageReady = data.image.removebg || data.image.openaiImage || data.image.comfyui;
    core.push(!!imageReady);
    const configured = core.filter(Boolean).length;
    return { configured, total: core.length };
  }, [data]);

  const aiDetail = useMemo(() => {
    if (!data) return null;
    const parts: string[] = [];
    if (data.ai.provider) {
      parts.push(`服务商：${aiTextProviderLabel(data.ai.provider)}`);
    }
    if (data.ai.model) {
      parts.push(`模型：${data.ai.model}`);
    }
    return parts.length ? parts.join(' · ') : '尚未填写 API 密钥与接口地址';
  }, [data]);

  const imageDetail = useMemo(() => {
    if (!data) return null;
    const items = [
      { label: 'remove.bg 抠图', ok: data.image.removebg, kind: 'key' as const },
      { label: 'OpenAI 图片', ok: data.image.openaiImage, kind: 'key' as const },
      { label: 'ComfyUI 工作流', ok: data.image.comfyui, kind: 'url' as const },
    ];
    return (
      <Space direction="vertical" size={6} style={{ width: '100%' }}>
        {data.image.providerCurrent ? (
          <Text type="secondary" style={{ fontSize: 12 }}>
            当前默认：{imageProviderLabel(data.image.providerCurrent)}
          </Text>
        ) : null}
        <Space wrap size={[4, 6]}>
          {items.map((item) => (
            <Tag key={item.label} color={item.ok ? 'success' : 'default'}>
              {item.label} · {imageSubServiceStatusLabel(item.ok, item.kind)}
            </Tag>
          ))}
        </Space>
      </Space>
    );
  }, [data]);

  const storageDetail = useMemo(() => {
    if (!data) return null;
    const kindLabel = storageKindLabel(data.storage.kind);
    return data.storage.configured ? `当前方式：${kindLabel}` : `请先完善 ${kindLabel} 所需凭据`;
  }, [data]);

  return (
    <TmPageContainer
      title={PAGE_COPY.integrations.title}
      subTitle={PAGE_COPY.integrations.description}
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="贸灵不提供也不内置任何第三方密钥"
        description={
          <>
            请自行在各开放平台、云厂商、模型供应商处注册应用并获取凭据；仅在后台填写后由服务端 AES-GCM 加密入库。前端不会直连
            OpenAI、云存储、电商平台 API 或 SMTP；所有出网调用由后端完成。
            {data?.disclaimerShort ? ` ${data.disclaimerShort}` : ''}
          </>
        }
      />
      <Spin spinning={loading}>
        {data ? (
          <>
            <ProCard variant="outlined" style={{ marginBottom: 16 }}>
              <Row gutter={[24, 16]}>
                <Col xs={24} sm={12} md={8}>
                  <Statistic title="核心集成就绪" value={`${summary.configured} / ${summary.total}`} />
                </Col>
                <Col xs={24} sm={12} md={8}>
                  <Statistic title="平台应用参数" value={sortedPlatforms.filter((p) => p.appConfigured).length} suffix={`/ ${sortedPlatforms.length}`} />
                </Col>
                <Col xs={24} sm={12} md={8}>
                  <Statistic
                    title="自定义采集规则"
                    value={data.collectRulesCount}
                    suffix={
                      <Link to="/collect/rules" style={{ fontSize: 13, marginLeft: 8 }}>
                        管理规则
                      </Link>
                    }
                  />
                </Col>
              </Row>
            </ProCard>

            <Title level={5} style={{ marginBottom: 12 }}>
              核心能力
            </Title>
            <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
              <Col xs={24} sm={12} lg={8}>
                <IntegrationHubCard
                  title="AI 大模型"
                  desc="标题优化、描述生成、客服建议等文本能力"
                  configured={data.ai.configured}
                  to="/settings/ai"
                  Icon={RobotOutlined}
                  extra={
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {aiDetail}
                    </Text>
                  }
                />
              </Col>
              <Col xs={24} sm={12} lg={8}>
                <IntegrationHubCard
                  title="图片 AI"
                  desc="抠图、场景图、ComfyUI 工作流等图片处理"
                  configured={data.image.removebg || data.image.openaiImage || data.image.comfyui}
                  to="/settings/image"
                  Icon={PictureOutlined}
                  extra={imageDetail}
                />
              </Col>
              <Col xs={24} sm={12} lg={8}>
                <IntegrationHubCard
                  title="文件存储"
                  desc="商品图片与附件的上传与访问"
                  configured={data.storage.configured}
                  to="/settings/storage"
                  Icon={CloudOutlined}
                  extra={
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {storageDetail}
                    </Text>
                  }
                />
              </Col>
              <Col xs={24} sm={12} lg={8}>
                <IntegrationHubCard
                  title="邮箱 SMTP"
                  desc="告警通知、测试邮件等系统发信"
                  configured={data.mail.configured}
                  to="/settings/email"
                  Icon={MailOutlined}
                  extra={
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {data.mail.configured ? 'SMTP 主机与发件人地址已填写' : '请填写 SMTP 主机与发件人地址'}
                    </Text>
                  }
                />
              </Col>
              <Col xs={24} sm={12} lg={8}>
                <IntegrationHubCard
                  title="自定义采集规则"
                  desc="非内置站点的 XPath / CSS 采集规则"
                  configured={data.collectRulesCount > 0}
                  to="/collect/rules"
                  Icon={ApiOutlined}
                  extra={
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      已创建 {data.collectRulesCount} 条规则
                    </Text>
                  }
                />
              </Col>
            </Row>

            {sortedPlatforms.length ? (
              <>
                <Title level={5} style={{ marginBottom: 4 }}>
                  跨境平台
                </Title>
                <Paragraph type="secondary" style={{ marginBottom: 12 }}>
                  此处为各平台开放平台应用参数（应用 Key / 密钥等）。店铺授权后的授权凭证保存在「店铺 →
                  授权配置」，请勿写入此处。
                </Paragraph>
                <Row gutter={[16, 16]}>
                  {sortedPlatforms.map((p) => (
                    <Col xs={24} sm={12} md={8} lg={6} key={p.platform}>
                      <PlatformIntegrationCard
                        name={p.name}
                        appConfigured={p.appConfigured}
                        status={p.status}
                      />
                    </Col>
                  ))}
                </Row>
                <div style={{ marginTop: 16 }}>
                  <Link to="/settings/platforms">
                    前往「平台接入设置」编辑 <RightOutlined style={{ fontSize: 11 }} />
                  </Link>
                </div>
              </>
            ) : null}
          </>
        ) : null}
      </Spin>
    </TmPageContainer>
  );
}
