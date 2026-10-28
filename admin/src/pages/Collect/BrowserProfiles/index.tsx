import { useLanguage } from '@/i18n/useLanguage';
import { tr } from '@/i18n';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { formatDateTime } from '@/utils/formatTime';
import { ModalForm, ProFormText } from '@ant-design/pro-components';
import { Button, Input, Popconfirm, Space, Tag, Typography, message } from 'antd';
import { useRef, useState } from 'react';
import { usePermission } from '@/hooks/usePermission';
import { PERMISSIONS } from '@/utils/permission';
import {
  checkBrowserProfile,
  createBrowserProfile,
  deleteBrowserProfile,
  disableBrowserProfile,
  enableBrowserProfile,
  openBrowserProfileLogin,
  queryBrowserProfiles,
  type BrowserProfileRow,
} from '@/services/collectBrowserProfiles';
import { accessStatusLabel } from '@/constants/collectAccess';


const CHECK_STATUS: Record<string, { text: string; color: string }> = {
  logged_in: { text: 'Logged in', color: 'success' },
  login_required: { text: 'Login required', color: 'warning' },
  verify_required: { text: 'Verification required', color: 'error' },
  unknown: { text: 'Unknown', color: 'default' },
  failed: { text: 'Failed', color: 'error' },
};

export default function CollectBrowserProfilesPage() {
  useLanguage();
  const { can } = usePermission();
  const canManageProfiles = can(PERMISSIONS.PRODUCT_WRITE);
  const actionRef = useRef<ActionType>();
  const [createOpen, setCreateOpen] = useState(false);
  const [checkUrl, setCheckUrl] = useState('');
  const [checkingId, setCheckingId] = useState<string | null>(null);
  const [openingId, setOpeningId] = useState<string | null>(null);

  const handleOpenLogin = async (row: BrowserProfileRow) => {
    const u = checkUrl.trim() || row.lastCheckUrl || `https://${row.domain}/`;
    setOpeningId(row.id);
    try {
      const res = await openBrowserProfileLogin(row.id, { url: u });
      message.success(res.message);
    } catch (e) {
      const msg = e instanceof Error ? e.message : tr("打开失败");
      if (msg.includes('HEADED_BROWSER_REQUIRED')) {
        message.error(tr("无法打开登录窗口，请联系管理员开启采集浏览器的可视化模式"));
      } else {
        message.error(msg);
      }
    } finally {
      setOpeningId(null);
    }
  };

  const handleCheckStatus = async (row: BrowserProfileRow) => {
    const u = checkUrl.trim() || row.lastCheckUrl;
    if (!u) {
      message.warning(tr("请在页顶填写用于检测的商品链接"));
      return;
    }
    setCheckingId(row.id);
    message.loading({ content: tr("正在检测登录状态…"), key: 'browser-profile-check', duration: 0 });
    try {
      const res = await checkBrowserProfile(row.id, { url: u });
      message.success({
        content: `${tr(accessStatusLabel(res.accessStatus).text)}：${res.message}`,
        key: 'browser-profile-check',
      });
      actionRef.current?.reload();
    } catch (e) {
      message.error({
        content: e instanceof Error ? e.message : tr("检测失败"),
        key: 'browser-profile-check',
      });
    } finally {
      setCheckingId(null);
    }
  };

  const columns: ProColumns<BrowserProfileRow>[] = [
    { title: tr("名称"), dataIndex: 'name', ellipsis: true },
    { title: tr("适用网站"), dataIndex: 'domain', copyable: true, width: 140 },
    { title: tr("类型"), dataIndex: 'provider', width: 100, search: false, render: () => tr("自定义采集") },
    {
      title: tr("状态"),
      dataIndex: 'status',
      width: 90,
      render: (_, row) =>
        row.status === 'active' ? <Tag color="success">{tr("启用")}</Tag> : <Tag>{tr("停用")}</Tag>,
    },
    {
      title: tr("最近检测"),
      dataIndex: 'lastCheckStatus',
      width: 110,
      search: false,
      render: (_, row) => {
        const m = CHECK_STATUS[row.lastCheckStatus ?? ''] ?? { text: row.lastCheckStatus || '—', color: 'default' };
        return <Tag color={m.color}>{tr(m.text)}</Tag>;
      },
    },
    {
      title: tr("检测时间"),
      dataIndex: 'lastCheckAt',
      width: 168,
      search: false,
      render: (_, row) => formatDateTime(row.lastCheckAt),
    },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 320,
      render: (_, row) => canManageProfiles ? (
        <Space wrap size="small">
          {row.status === 'active' ? (
            <>
              <Button
                type="link"
                size="small"
                loading={openingId === row.id}
                disabled={checkingId === row.id}
                onClick={() => void handleOpenLogin(row)}
              >
                {tr("打开登录")}</Button>
              <Button
                type="link"
                size="small"
                loading={checkingId === row.id}
                disabled={openingId === row.id}
                onClick={() => void handleCheckStatus(row)}
              >
                {tr("检测状态")}</Button>
              <Popconfirm
                title={tr("停用该登录状态？")}
                onConfirm={() =>
                  void disableBrowserProfile(row.id)
                    .then(() => {
                      message.success(tr("已停用"));
                      actionRef.current?.reload();
                    })
                    .catch((e) => message.error(e instanceof Error ? e.message : tr("失败")))
                }
              >
                <a>{tr("停用")}</a>
              </Popconfirm>
            </>
          ) : (
            <Popconfirm
              title={tr("重新启用该登录状态？")}
              onConfirm={() =>
                void enableBrowserProfile(row.id)
                  .then(() => {
                    message.success(tr("已启用"));
                    actionRef.current?.reload();
                  })
                  .catch((e) => message.error(e instanceof Error ? e.message : tr("失败")))
              }
            >
              <a>{tr("启用")}</a>
            </Popconfirm>
          )}
          <Popconfirm
            title={tr("删除该登录状态？")}
            description={tr("删除后不可恢复，本地浏览器登录数据需手动清理。")}
            onConfirm={() =>
              void deleteBrowserProfile(row.id)
                .then(() => {
                  message.success(tr("已删除"));
                  actionRef.current?.reload();
                })
                .catch((e) => message.error(e instanceof Error ? e.message : tr("删除失败")))
            }
          >
            <a>{tr("删除")}</a>
          </Popconfirm>
        </Space>
      ) : <Typography.Text type="secondary">{tr("仅查看")}</Typography.Text>,
    },
  ];

  return (
    <TmPageContainer
      title={tr("采集浏览器登录状态")}
      subTitle={tr("用于需要登录才能查看的商品页；系统不保存账号密码。")}
    >
      <Typography.Paragraph type="secondary">
        {tr("登录信息保存在本机采集浏览器中，请勿在公共电脑保留。验证码需自行完成，系统不会自动破解。")}</Typography.Paragraph>
      <Space style={{ marginBottom: 16 }}>
        <Input
          style={{ width: 420 }}
          placeholder={tr("用于检测或打开登录的商品链接")}
          value={checkUrl}
          onChange={(e) => setCheckUrl(e.target.value)}
        />
        {canManageProfiles ? <Button type="primary" onClick={() => setCreateOpen(true)}>
          {tr("新建登录状态")}</Button> : null}
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
        title={tr("新建采集浏览器登录状态")}
        open={createOpen}
        onOpenChange={setCreateOpen}
        onFinish={async (vals) => {
          try {
            await createBrowserProfile({
              name: vals.name.trim(),
              domain: vals.domain.trim(),
              provider: 'custom',
            });
            message.success(tr("已创建"));
            actionRef.current?.reload();
            return true;
          } catch (e) {
            message.error(e instanceof Error ? e.message : tr("创建失败"));
            return false;
          }
        }}
      >
        <ProFormText name="name" label={tr("名称")} rules={[{ required: true }]} />
        <ProFormText
          name="domain"
          label={tr("域名")}
          placeholder="jd.com"
          rules={[{ required: true }]}
        />
      </ModalForm>
    </TmPageContainer>
  );
}
