import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { formatDateTime } from '@/utils/formatTime';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { Tag } from 'antd';
import dayjs from 'dayjs';
import { useRef } from 'react';
import { fetchOperationLogs, type OperationLogRow } from '@/services/operationLogs';

function statusTag(s: string) {
  const ok = s === 'success';
  return <Tag color={ok ? 'success' : 'error'}>{s}</Tag>;
}

export default function OperationLogsPage() {
  const actionRef = useRef<ActionType>();

  const columns: ProColumns<OperationLogRow>[] = [
    {
      title: '时间',
      dataIndex: 'createdAt',
      width: 172,
      search: false,
      render: (_, row) => formatDateTime(row.createdAt),
    },
    {
      title: '用户',
      dataIndex: 'username',
      width: 120,
      ellipsis: true,
    },
    {
      title: '操作',
      dataIndex: 'action',
      width: 132,
      ellipsis: true,
    },
    {
      title: '资源',
      dataIndex: 'resource',
      width: 112,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 96,
      search: false,
      render: (_, row) => statusTag(row.status),
    },
    {
      title: 'IP',
      dataIndex: 'ip',
      width: 132,
      search: false,
    },
    {
      title: 'RequestID',
      dataIndex: 'requestId',
      width: 260,
      ellipsis: true,
      copyable: true,
      search: false,
    },
    {
      title: '路径',
      dataIndex: 'path',
      ellipsis: true,
      search: false,
    },
    {
      title: '说明',
      dataIndex: 'message',
      ellipsis: true,
      search: false,
    },
    {
      title: '时间范围',
      dataIndex: 'dateRange',
      valueType: 'dateTimeRange',
      hideInTable: true,
      search: {
        transform: (value) => {
          if (!value || !Array.isArray(value) || value.length < 2) return {};
          const a = dayjs(value[0] as string | dayjs.Dayjs);
          const b = dayjs(value[1] as string | dayjs.Dayjs);
          if (!a.isValid() || !b.isValid()) return {};
          return { start: a.toISOString(), end: b.toISOString() };
        },
      },
    },
  ];

  return (
    <PageContainer title="操作日志">
      <ProTable<OperationLogRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto', defaultCollapsed: false }}
        options={{ reload: true, density: true, setting: true }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        request={async (params) => {
          const res = await fetchOperationLogs({
            page: params.current,
            pageSize: params.pageSize,
            action: params.action as string | undefined,
            username: params.username as string | undefined,
            resource: params.resource as string | undefined,
            start: params.start as string | undefined,
            end: params.end as string | undefined,
          });
          return {
            data: res.list,
            success: true,
            total: res.pagination.total,
          };
        }}
        headerTitle={false}
        toolBarRender={() => []}
      />
    </PageContainer>
  );
}
