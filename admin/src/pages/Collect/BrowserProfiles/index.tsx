import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { ModalForm, PageContainer, ProFormText, ProTable } from '@ant-design/pro-components';
import { Button, Input, Popconfirm, Space, Tag, Typography, message } from 'antd';
import dayjs from 'dayjs';
import { useRef, useState } from 'react';
import {
  checkBrowserProfile,
  createBrowserProfile,
  disableBrowserProfile,
  openBrowserProfileLogin,
  queryBrowserProfiles,
  type BrowserProfileRow,
} from '@/services/collectBrowserProfiles';
import { accessStatusLabel } from '@/constants/collectAccess';

function formatTs(s?: string) {
  if (!s) return '—';
  const d = dayjs(s);
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : s;
}

const CHECK_STATUS: Record<string, { text: string; color: string }> = {
  logged_in: { text: '已登录', color: 'success' },
  login_required: { text: '需登录', color: 'warning' },
  verify_required: { text: '需验证', color: 'error' },
  unknown: { text: '未知', color: 'default' },
  failed: { text: '失败', color: 'error' },
};

export default function CollectBrowserProfilesPage() {
  const actionRef = useRef<ActionType>();
  const [createOpen, setCreateOpen] = useState(false);
  const [checkUrl, setCheckUrl] = useState('');

  const columns: ProColumns<BrowserProfileRow>[] = [
    { title: '名称', dataIndex: 'name', ellipsis: true },
    { title: '域名', dataIndex: 'domain', copyable: true, width: 140 },
    { title: 'Provider', dataIndex: 'provider', width: 100, search: false },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (_, row) =>
        row.status === 'active' ? <Tag color="success">启用</Tag> : <Tag>停用</Tag>,
    },
    {
      title: '最近检测',
      dataIndex: 'lastCheckStatus',
      width: 110,
      search: false,
      render: (_, row) => {
        const m = CHECK_STATUS[row.lastCheckStatus ?? ''] ?? { text: row.lastCheckStatus || '—', color: 'default' };
        return <Tag color={m.color}>{m.text}</Tag>;
      },
    },
    {
      title: '检测时间',
      dataIndex: 'lastCheckAt',
      width: 168,
      search: false,
      render: (_, row) => formatTs(row.lastCheckAt),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 280,
      render: (_, row) => (
        <Space wrap size="small">
          <a
            onClick={() => {
              const u = checkUrl.trim() || row.lastCheckUrl || `https://${row.domain}/`;
              void openBrowserProfileLogin(row.id, { url: u })
                .then((res) => message.success(res.message))
                .catch((e) => {
                  const msg = e instanceof Error ? e.message : '打开失败';
                  if (msg.includes('HEADED_BROWSER_REQUIRED')) {
                    message.error('Collector 需 headed 模式（COLLECTOR_HEADLESS=0）');
                  } else {
                    message.error(msg);
                  }
                });
            }}
          >
            打开登录
          </a>
          <a
            onClick={() => {
              const u = checkUrl.trim() || row.lastCheckUrl;
              if (!u) {
                message.warning('请在页顶填写检测 URL');
                return;
              }
              void checkBrowserProfile(row.id, { url: u })
                .then((res) => {
                  message.info(`${accessStatusLabel(res.accessStatus).text}：${res.message}`);
                  actionRef.current?.reload();
                })
                .catch((e) => message.error(e instanceof Error ? e.message : '检测失败'));
            }}
          >
            检测状态
          </a>
          <Popconfirm
            title="停用该 Profile？"
            onConfirm={() =>
              void disableBrowserProfile(row.id)
                .then(() => {
                  message.success('已停用');
                  actionRef.current?.reload();
                })
                .catch((e) => message.error(e instanceof Error ? e.message : '失败'))
            }
          >
            <a>停用</a>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer
      title="采集浏览器 Profile"
      subTitle="自定义链接采集登录态（不保存账号密码，Cookie 仅存于 Collector 本地目录）"
    >
      <Typography.Paragraph type="secondary">
        Profile 目录由采集服务管理，请勿在公共电脑保存敏感登录态。验证码需用户自行完成，系统不提供破解能力。
      </Typography.Paragraph>
      <Space style={{ marginBottom: 16 }}>
        <Input
          style={{ width: 420 }}
          placeholder="检测 / 打开登录用的商品页 URL"
          value={checkUrl}
          onChange={(e) => setCheckUrl(e.target.value)}
        />
        <Button type="primary" onClick={() => setCreateOpen(true)}>
          新建 Profile
        </Button>
      </Space>
      <ProTable<BrowserProfileRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto' }}
        request={async (params) => {
          const res = await queryBrowserProfiles({
            page: params.current,
            pageSize: params.pageSize,
            domain: params.domain as string | undefined,
            status: (params.status as string) || undefined,
          });
          return { data: res.list, success: true, total: res.pagination.total };
        }}
      />
      <ModalForm<{ name: string; domain: string }>
        title="新建采集浏览器 Profile"
        open={createOpen}
        onOpenChange={setCreateOpen}
        onFinish={async (vals) => {
          try {
            await createBrowserProfile({
              name: vals.name.trim(),
              domain: vals.domain.trim(),
              provider: 'custom',
            });
            message.success('已创建');
            actionRef.current?.reload();
            return true;
          } catch (e) {
            message.error(e instanceof Error ? e.message : '创建失败');
            return false;
          }
        }}
      >
        <ProFormText name="name" label="名称" rules={[{ required: true }]} />
        <ProFormText
          name="domain"
          label="域名"
          placeholder="jd.com"
          rules={[{ required: true }]}
        />
      </ModalForm>
    </PageContainer>
  );
}
