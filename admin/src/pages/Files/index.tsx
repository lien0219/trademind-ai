import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { Link } from '@umijs/max';
import { Button, Image, Popconfirm, message } from 'antd';
import dayjs from 'dayjs';
import { useRef } from 'react';
import { deleteFile, fetchFiles, type FileRow } from '@/services/files';

function formatSize(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(2)} MB`;
}

function isImage(ct: string) {
  return /^image\//i.test(ct);
}

export default function FilesPage() {
  const actionRef = useRef<ActionType>();

  const columns: ProColumns<FileRow>[] = [
    {
      title: '预览',
      dataIndex: 'url',
      width: 88,
      search: false,
      render: (_, row) =>
        isImage(row.contentType) ? (
          <Image src={row.url} width={56} height={56} style={{ objectFit: 'cover' }} />
        ) : (
          <PictureOutlined style={{ fontSize: 28, color: '#bbb' }} />
        ),
    },
    {
      title: '文件名',
      dataIndex: 'filename',
      ellipsis: true,
      search: false,
    },
    {
      title: 'Content-Type',
      dataIndex: 'contentType',
      width: 160,
    },
    {
      title: '大小',
      dataIndex: 'size',
      width: 100,
      search: false,
      render: (_, row) => formatSize(row.size),
    },
    {
      title: 'URL',
      dataIndex: 'url',
      ellipsis: true,
      copyable: true,
      search: false,
    },
    {
      title: '存储',
      dataIndex: 'storageKind',
      width: 100,
      search: false,
    },
    {
      title: '上传时间',
      dataIndex: 'createdAt',
      width: 180,
      search: false,
      render: (_, row) => dayjs(row.createdAt).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 100,
      render: (_, row) => [
        <Popconfirm
          key="del"
          title="确定删除该文件？将同时删除磁盘对象与记录。"
          onConfirm={async () => {
            try {
              await deleteFile(row.id);
              message.success('已删除');
              actionRef.current?.reload();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '删除失败');
            }
          }}
        >
          <Button type="link" danger size="small">
            删除
          </Button>
        </Popconfirm>,
      ],
    },
  ];

  return (
    <PageContainer
      title="文件管理"
      subTitle="本地上传的图片元数据；删除会调用后端并移除存储中的对象。"
      extra={[
        <Link key="hint" to="/settings/storage">
          在存储设置中测试上传
        </Link>,
      ]}
    >
      <ProTable<FileRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto' }}
        options={{ reload: true, density: true, setting: true }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        request={async (params) => {
          const res = await fetchFiles({
            page: params.current,
            pageSize: params.pageSize,
            contentType: params.contentType as string | undefined,
          });
          return {
            data: res.list,
            success: true,
            total: res.pagination.total,
          };
        }}
        headerTitle="上传文件"
      />
    </PageContainer>
  );
}
