import { PictureOutlined } from '@ant-design/icons';
import { formatDateTime } from '@/utils/formatTime';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { TmPageContainer } from '@/components/ui';
import { ProTable } from '@ant-design/pro-components';
import { Link } from '@umijs/renderer-react';
import { Button, Image, Popconfirm, message } from 'antd';
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
      title: '访问地址',
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
      render: (_, row) => formatDateTime(row.createdAt),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 100,
      render: (_, row) => [
        <Popconfirm
          key="del"
          title="删除文件？"
          description="将删除存储对象与记录"
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
    <TmPageContainer
      title="文件管理"
      subTitle="管理已上传的商品图片与附件。"
      extra={
        <Link key="hint" to="/settings/storage">
          存储设置
        </Link>
      }
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
        headerTitle={false}
      />
    </TmPageContainer>
  );
}
