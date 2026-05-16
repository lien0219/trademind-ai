import {
  CloudDownloadOutlined,
  FileImageOutlined,
  RobotOutlined,
  SettingOutlined,
  ShoppingOutlined,
} from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Col, Row } from 'antd';
import type { ReactNode } from 'react';

const QUICK_LINKS: {
  title: string;
  path: string;
  icon: ReactNode;
  accent: string;
}[] = [
  { title: '商品草稿', path: '/product/drafts', icon: <ShoppingOutlined />, accent: '#2563eb' },
  { title: '采集中心', path: '/collect', icon: <CloudDownloadOutlined />, accent: '#0891b2' },
  { title: 'AI', path: '/ai/prompts', icon: <RobotOutlined />, accent: '#7c3aed' },
  { title: '文件', path: '/files', icon: <FileImageOutlined />, accent: '#ea580c' },
  { title: '设置', path: '/settings/system', icon: <SettingOutlined />, accent: '#475569' },
];

export default function DashboardPage() {
  return (
    <PageContainer title="工作台">
      <Row gutter={[16, 16]}>
        {QUICK_LINKS.map((item) => (
          <Col xs={24} sm={12} lg={8} xl={6} key={item.path}>
            <ProCard
              bordered
              hoverable
              bodyStyle={{ padding: '20px 22px' }}
              onClick={() => history.push(item.path)}
              style={{ cursor: 'pointer' }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
                <span
                  style={{
                    width: 44,
                    height: 44,
                    borderRadius: 12,
                    background: `${item.accent}14`,
                    color: item.accent,
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: 20,
                  }}
                >
                  {item.icon}
                </span>
                <span style={{ fontWeight: 600, fontSize: 15, color: '#0f172a' }}>{item.title}</span>
              </div>
            </ProCard>
          </Col>
        ))}
      </Row>
    </PageContainer>
  );
}
