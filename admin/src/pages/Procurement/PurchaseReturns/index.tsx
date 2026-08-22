import { EyeOutlined, PlusOutlined } from '@ant-design/icons';
import { history, useSearchParams } from '@umijs/max';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Select, Space, Tag } from 'antd';
import { useEffect, useRef, useState } from 'react';
import PermissionGuard from '@/components/PermissionGuard';
import { ErrorAlert, TmPageContainer, TmPageHeaderExtra, TmProTable } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import {
  extractProcurementAPIError,
  listPurchaseReturns,
  procurementErrorMessage,
  type PurchaseReturn,
} from '@/services/procurement';
import { PERMISSIONS } from '@/utils/permission';
import '../index.less';

const RETURN_STATUS: Record<string, { text: string; color: string }> = {
  draft: { text: '草稿', color: 'default' },
  pending_approval: { text: '待审批', color: 'gold' },
  approved: { text: '待执行', color: 'blue' },
  completed: { text: '已完成', color: 'success' },
  cancelled: { text: '已取消', color: 'error' },
};

export default function PurchaseReturnsPage() {
  const { can, readonly } = usePermission();
  const canManage = !readonly && can(PERMISSIONS.PROCUREMENT_MANAGE);
  const actionRef = useRef<ActionType>();
  const [searchParams, setSearchParams] = useSearchParams();
  const purchaseOrderId = searchParams.get('purchaseOrderId')?.trim() || undefined;
  const [status, setStatus] = useState<string>();
  const [error, setError] = useState('');

  useEffect(() => {
    void actionRef.current?.reload();
  }, [purchaseOrderId, status]);

  const columns: ProColumns<PurchaseReturn>[] = [
    {
      title: '退货单号', dataIndex: 'returnNo', width: 190, copyable: true, ellipsis: true,
      render: (_, row) => <Button type="link" size="small" onClick={() => history.push(`/procurement/purchase-returns/${row.id}`)}>{row.returnNo}</Button>,
    },
    { title: '采购单号', dataIndex: 'purchaseOrderNo', width: 190, copyable: true, ellipsis: true },
    { title: '供应商', dataIndex: 'supplierName', minWidth: 160, ellipsis: true, render: (_, row) => row.supplierName || row.supplierId },
    { title: '退货仓库', dataIndex: 'warehouseName', minWidth: 150, ellipsis: true, render: (_, row) => row.warehouseName || row.warehouseId },
    { title: '明细', dataIndex: 'itemCount', width: 72, align: 'right' },
    {
      title: '状态', dataIndex: 'status', width: 105,
      render: (_, row) => { const meta = RETURN_STATUS[row.status] || { text: '未知状态', color: 'default' }; return <Tag color={meta.color}>{meta.text}</Tag>; },
    },
    { title: '创建时间', dataIndex: 'createdAt', valueType: 'dateTime', width: 180 },
    { title: '操作', valueType: 'option', width: 88, render: (_, row) => <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => history.push(`/procurement/purchase-returns/${row.id}`)}>查看</Button> },
  ];

  return (
    <PermissionGuard require={PERMISSIONS.PROCUREMENT_VIEW} showForbiddenPage>
      <TmPageContainer
        className="tm-procurement-page"
        title="采购退货"
        subTitle="从原收货记录发起供应商退货，审批后由独立执行岗扣减仓库库存。"
        extra={<TmPageHeaderExtra><Button type="primary" icon={<PlusOutlined />} disabled={!canManage} onClick={() => history.push('/procurement/purchase-orders')}>选择采购单</Button></TmPageHeaderExtra>}
      >
        {readonly ? <Alert type="info" showIcon message="当前账号为只读模式，可查看采购退货记录但不能发起或执行操作。" /> : null}
        {purchaseOrderId ? <Alert type="info" showIcon message="当前仅显示指定采购单的退货记录。" action={<Button size="small" onClick={() => setSearchParams({})}>查看全部</Button>} /> : null}
        {error ? <ErrorAlert title={error} actionHint={<Button onClick={() => actionRef.current?.reload()}>重新加载</Button>} /> : null}
        <TmProTable<PurchaseReturn>
          className="tm-procurement-table"
          rowKey="id"
          actionRef={actionRef}
          columns={columns}
          search={false}
          cardBordered
          scroll={{ x: 1220 }}
          locale={{ emptyText: error ? '采购退货列表暂不可用' : '暂无采购退货记录。' }}
          toolBarRender={() => [
            <Select
              key="status"
              allowClear
              placeholder="全部状态"
              value={status}
              style={{ width: 140 }}
              options={Object.entries(RETURN_STATUS).map(([value, meta]) => ({ value, label: meta.text }))}
              onChange={setStatus}
            />,
          ]}
          request={async (params) => {
            try {
              const result = await listPurchaseReturns({ page: params.current, pageSize: params.pageSize, status, purchaseOrderId });
              setError('');
              return { data: result.list || [], success: true, total: result.total || 0 };
            } catch (nextError) {
              setError(procurementErrorMessage(extractProcurementAPIError(nextError), '采购退货列表加载失败，请稍后重试。'));
              return { data: [], success: false, total: 0 };
            }
          }}
        />
      </TmPageContainer>
    </PermissionGuard>
  );
}
