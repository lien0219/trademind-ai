import { DashboardOutlined, RocketOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Col, Row, Space, Statistic, Typography } from 'antd';

export default function DashboardPage() {
  return (
    <PageContainer title="工作台" subTitle="MVP 闭环：采集 → 草稿 → AI 优化 → 存储">
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={8}>
          <ProCard bordered hoverable>
            <Space direction="vertical" size="small">
              <Typography.Text type="secondary">当前阶段</Typography.Text>
              <Statistic title="项目地基 + 管理端骨架" value="v0.1" prefix={<RocketOutlined />} />
            </Space>
          </ProCard>
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <ProCard bordered hoverable>
            <Statistic title="快捷入口" value="左侧菜单" prefix={<DashboardOutlined />} />
            <Typography.Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>
              从「商品草稿」「采集任务」「设置」进入各模块。
            </Typography.Paragraph>
          </ProCard>
        </Col>
      </Row>
    </PageContainer>
  );
}
