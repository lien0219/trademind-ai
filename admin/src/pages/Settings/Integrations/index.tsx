import { Link } from '@umijs/renderer-react';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Alert, Descriptions, Spin, Tag, Typography } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { PLATFORM_PROVIDER_STATUS } from '@/constants/status';
import { fetchIntegrationsOverview, type IntegrationOverviewData } from '@/services/settings';

function boolTag(ok: boolean) {
  return ok ? <Tag color="success">已配置</Tag> : <Tag>未就绪</Tag>;
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

  return (
    <PageContainer title="第三方集成总览">
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="贸灵不提供也不内置任何第三方密钥"
        description={
          <>
            请自行在各开放平台、云厂商、模型供应商处注册应用并获取凭据；仅在后台填写后由服务端 AES-GCM 加密入库。前端不会直连 OpenAI、云存储 bucket、电商平台 API 或 SMTP；所有出网调用由后端完成。
            {data?.disclaimerShort ? ` ${data.disclaimerShort}` : ''}
          </>
        }
      />
      <Spin spinning={loading}>
        {data && (
          <ProCard bordered title="配置状态（摘要）" style={{ marginBottom: 16 }}>
            <Descriptions column={1} bordered size="small">
              <Descriptions.Item label="AI 大模型（settings.ai）">{boolTag(data.ai.configured)}</Descriptions.Item>
              <Descriptions.Item label="图片 AI（settings.image）">
                <Typography.Text>
                  remove.bg {data.image.removebg ? '· Key 已填' : '· 未填 Key'}；OpenAI Image{' '}
                  {data.image.openaiImage ? '· Key 已填' : '· 未填 Key'}；ComfyUI{' '}
                  {data.image.comfyui ? '· 地址已填' : '· 未填地址'}
                  {data.image.providerCurrent ? (
                    <Typography.Text type="secondary">{`（当前 provider=${data.image.providerCurrent}）`}</Typography.Text>
                  ) : null}
                </Typography.Text>
              </Descriptions.Item>
              <Descriptions.Item label="存储（settings.storage）">
                {boolTag(data.storage.configured)}
                {data.storage.kind ? (
                  <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
                    kind={data.storage.kind}
                  </Typography.Text>
                ) : null}
              </Descriptions.Item>
              <Descriptions.Item label="邮箱 SMTP（settings.mail，兼容 legacy email）">{boolTag(data.mail.configured)}</Descriptions.Item>
              <Descriptions.Item label="自定义采集规则">
                <Link to="/collect/rules">规则数量：{data.collectRulesCount}</Link>
              </Descriptions.Item>
            </Descriptions>
          </ProCard>
        )}
        {data?.platforms?.length ? (
          <ProCard bordered title="平台开放配置（应用级 settings.platform_*）">
            <Typography.Paragraph type="secondary">
              店铺授权后的 token 保存在「店铺 → 授权配置」，勿写入此处。
            </Typography.Paragraph>
            <Descriptions column={1} bordered size="small">
              {data.platforms.map((p) => {
                const st = PLATFORM_PROVIDER_STATUS[p.status as keyof typeof PLATFORM_PROVIDER_STATUS];
                return (
                  <Descriptions.Item
                    key={p.platform}
                    label={
                      <span>
                        {p.name}{' '}
                        <Typography.Text type="secondary" code>
                          {p.platform}
                        </Typography.Text>
                      </span>
                    }
                  >
                    {boolTag(p.appConfigured)} · 运行时{' '}
                    <Tag>{st?.text ?? p.status}</Tag>
                    {p.groupKey ? (
                      <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
                        {p.groupKey}
                      </Typography.Text>
                    ) : null}
                  </Descriptions.Item>
                );
              })}
            </Descriptions>
            <Link to="/settings/platforms">前往「平台开放配置」编辑</Link>
          </ProCard>
        ) : null}
      </Spin>
    </PageContainer>
  );
}
