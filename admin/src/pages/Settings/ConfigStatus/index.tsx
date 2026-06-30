import { Button, Card, Col, Row, Tag, Typography } from 'antd';
import { history } from '@umijs/max';
import { useEffect, useState } from 'react';
import { TmPageContainer } from '@/components/ui';
import PermissionGuard from '@/components/PermissionGuard';
import { PAGE_COPY } from '@/constants/copywriting';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';
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
          {PAGE_COPY.configStatus.nextStep}：{item.nextAction}
        </Typography.Text>
      ) : null}
      {item.settingsUrl && item.settingsUrl.startsWith('/') ? (
        <Button type="link" size="small" onClick={() => history.push(item.settingsUrl!)}>
          {PAGE_COPY.configStatus.goConfigure}
        </Button>
      ) : null}
    </Card>
  );
}

export default function ConfigStatusPage() {
  const emptyLocale = useListEmptyLocale('configStatus');
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
        title={PAGE_COPY.configStatus.title}
        subTitle={PAGE_COPY.configStatus.description}
      >
        {generatedAt ? (
          <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
            {PAGE_COPY.configStatus.snapshotAt}：{generatedAt}
          </Typography.Text>
        ) : null}
        {items.length === 0 && !demo ? (
          emptyLocale.emptyText
        ) : (
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
        )}
      </TmPageContainer>
    </PermissionGuard>
  );
}
