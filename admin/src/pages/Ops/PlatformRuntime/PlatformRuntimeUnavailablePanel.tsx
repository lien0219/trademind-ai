import { SectionCard } from '@/components/ui';
import { PLATFORM_STATUS_META } from '@/constants/platformAppConfig';
import type { PlatformProviderMeta } from '@/services/shops';
import { Link } from '@umijs/max';
import { Alert, Button, Space, Tag, Typography } from 'antd';

const { Paragraph, Text } = Typography;

type Props = {
  meta: PlatformProviderMeta;
};

export default function PlatformRuntimeUnavailablePanel({ meta }: Props) {
  const st = PLATFORM_STATUS_META[meta.status] ?? { label: meta.status, color: 'default' };

  return (
    <SectionCard title={`${meta.name} 运行状态`} description="该平台尚未接入运行时运维能力。">
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Alert
          showIcon
          type="warning"
          message="暂不可用"
          description={
            <Space direction="vertical" size={8}>
              <Text>
                {meta.name} 的运行控制、健康检查、24 小时指标与发布门禁尚未接入，当前仅支持查看平台接入状态。
              </Text>
              <Text type="secondary">接入后将在此 Tab 展示完整运维面板，无需新增侧栏菜单。</Text>
            </Space>
          }
        />
        <Space wrap>
          <Text type="secondary">平台状态</Text>
          <Tag color={st.color}>{st.label}</Tag>
          <Tag>未接入运行时</Tag>
        </Space>
        <Paragraph type="secondary" style={{ marginBottom: 0 }}>
          如需配置应用凭证、授权店铺或开启同步能力，请前往平台接入设置。
        </Paragraph>
        <Space wrap>
          <Link to="/settings/platforms">
            <Button type="primary">前往平台接入设置</Button>
          </Link>
          <Link to="/shops/manage">
            <Button>店铺管理</Button>
          </Link>
        </Space>
      </Space>
    </SectionCard>
  );
}
