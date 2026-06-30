import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { formatDateTime } from '@/utils/formatTime';

import { Tag, Tooltip, Typography } from 'antd';
import dayjs from 'dayjs';
import { useRef } from 'react';
import { PAGE_COPY, TASK_COPY, commonStatusLabel } from '@/constants/copywriting';
import {
  OPERATION_LOG_ACTION_OPTIONS,
  OPERATION_LOG_RESOURCE_OPTIONS,
  operationLogActionLabel,
  operationLogResourceLabel,
} from '@/constants/operationLogs';
import { fetchOperationLogs, type OperationLogRow } from '@/services/operationLogs';
import { useListEmptyLocale } from '@/hooks/useListEmptyLocale';

function statusTag(s: string) {
  const k = (s || '').trim().toLowerCase();
  const label = commonStatusLabel(k);
  if (k === 'success') return <Tag color="success">{label}</Tag>;
  if (k === 'failed') return <Tag color="error">{label}</Tag>;
  return <Tag>{label === '—' ? s || '—' : label}</Tag>;
}

function mappedCellLabel(label: string, raw?: string) {
  const text = (label || '—').trim();
  const key = (raw || '').trim();
  const content = <Typography.Text>{text}</Typography.Text>;
  if (!key || text === key) return content;
  return <Tooltip title={`原始值：${key}`}>{content}</Tooltip>;
}

function mappedResourceTag(resource?: string) {
  const key = (resource || '').trim();
  const label = operationLogResourceLabel(key);
  const tag = (
    <Tag bordered={false} color="processing">
      {label}
    </Tag>
  );
  if (!key || label === key) return tag;
  return <Tooltip title={`原始值：${key}`}>{tag}</Tooltip>;
}

export default function OperationLogsPage() {
  const emptyLocale = useListEmptyLocale('operationLogs');
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
      width: 168,
      ellipsis: true,
      valueType: 'select',
      fieldProps: {
        showSearch: true,
        optionFilterProp: 'label',
        options: OPERATION_LOG_ACTION_OPTIONS,
      },
      render: (_, row) => mappedCellLabel(operationLogActionLabel(row.action), row.action),
    },
    {
      title: '资源',
      dataIndex: 'resource',
      width: 140,
      ellipsis: true,
      valueType: 'select',
      fieldProps: {
        showSearch: true,
        optionFilterProp: 'label',
        options: OPERATION_LOG_RESOURCE_OPTIONS,
      },
      render: (_, row) => mappedResourceTag(row.resource),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 96,
      search: false,
      render: (_, row) => statusTag(row.status),
    },
    {
      title: '来源 IP',
      dataIndex: 'ip',
      width: 132,
      search: false,
    },
    {
      title: TASK_COPY.requestId,
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
    <TmPageContainer title={PAGE_COPY.operationLogs.title} subTitle={PAGE_COPY.operationLogs.description}>
      <ProTable<OperationLogRow>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={{ labelWidth: 'auto', defaultCollapsed: false }}
        options={{ reload: true, density: true, setting: true }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        locale={emptyLocale}
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
    </TmPageContainer>
  );
}
