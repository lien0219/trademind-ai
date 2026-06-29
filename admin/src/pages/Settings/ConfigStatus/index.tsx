import { Button, Card, Col, Row, Tag, Typography } from 'antd';
import { history } from '@umijs/max';
import { useEffect, useState } from 'react';
import { TmPageContainer } from '@/components/ui';
import PermissionGuard from '@/components/PermissionGuard';
import { fetchConfigStatusOverview, type ConfigStatusItem } from '@/services/configStatus';
import { PERMISSIONS } from '@/utils/permission';

function statusColor(status: string) {
  if (status.includes('已配置') || status.includes('运行中')) return 'success';
  if (status.includes('异常') || status.includes('配置异常')) return 'error';
  if (status.includes('关闭') || status.includes('未配置')) return 'default';
  if (status.includes('待')) return 'warning';
  return 'processing';
}

function StatusCard({ item }: { item: ConfigStatusItem }) {
  return (
    <Card size="small" title={item.title} extra={<Tag color={statusColor(item.status)}>{item.status}</Tag>}>
      {item.summary ? <Typography.Paragraph type="secondary">{item.summary}</Typography.Paragraph> : null}
      {item.nextAction ? (
        <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
          下一步：{item.nextAction}
        </Typography.Text>
      ) : null}
      {item.settingsUrl && item.settingsUrl.startsWith('/') ? (
        <Button type="link" size="small" onClick={() => history.push(item.settingsUrl!)}>
          前往配置
        </Button>
      ) : null}
    </Card>
  );
}

export default function ConfigStatusPage() {
  const [items, setItems] = useState<ConfigStatusItem[]>([]);
  const [demo, setDemo] = useState<ConfigStatusItem | null>(null);
  const [generatedAt, setGeneratedAt] = useState('');

  useEffect(() => {
    fetchConfigStatusOverview()
      .then((res) => {
        setItems(res.items || []);
        setDemo(res.demoData || null);
        setGeneratedAt(res.generatedAt || '');
      })
      .catch(() => {
        setItems([]);
      });
  }, []);

  return (
    <PermissionGuard require={PERMISSIONS.SETTINGS_MANAGE} showForbiddenPage>
      <TmPageContainer
        title="配置状态中心"
        subTitle="聚合 AI / OCR / Storage / 平台凭证 / Worker 等配置健康状态（不含密钥明文）"
      >
        {generatedAt ? (
          <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
            快照时间：{generatedAt}
          </Typography.Text>
        ) : null}
        <Row gutter={[16, 16]}>
          {items.map((item) => (
            <Col key={item.key} xs={24} sm={12} lg={8}>
              <StatusCard item={item} />
            </Col>
          ))}
          {demo ? (
            <Col xs={24} sm={12} lg={8}>
              <StatusCard item={demo} />
            </Col>
          ) : null}
        </Row>
      </TmPageContainer>
    </PermissionGuard>
  );
}
