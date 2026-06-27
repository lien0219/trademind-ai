import type { PublishConfigLayer } from '@/constants/publishConfig';
import { flattenEffectiveForDisplay, mergeEffectiveConfig } from '@/utils/publishConfigMerge';
import type { PublishConfigOverrides } from '@/services/productPublish';
import TechnicalDetails from '@/components/ui/TechnicalDetails';
import { Descriptions, Modal, Table } from 'antd';

export type PreviewCell = {
  productId: string;
  productTitle: string;
  platform: string;
  platformLabel: string;
  shopId?: string;
  shopName?: string;
};

type Props = {
  open: boolean;
  onClose: () => void;
  cells: PreviewCell[];
  commonConfig: PublishConfigLayer;
  overrides: PublishConfigOverrides;
};

export default function EffectiveConfigPreviewModal({ open, onClose, cells, commonConfig, overrides }: Props) {
  const rows = cells.flatMap((cell) => {
    const eff = mergeEffectiveConfig(commonConfig, overrides, cell.productId, cell.platform, cell.shopId);
    return flattenEffectiveForDisplay(eff).map((r) => ({
      key: `${cell.productId}:${cell.platform}:${cell.shopId}:${r.field}`,
      productTitle: cell.productTitle,
      target: `${cell.platformLabel}${cell.shopName ? ` / ${cell.shopName}` : ''}`,
      fieldLabel: r.label,
      value: r.value,
      source: r.source || '—',
      technicalField: r.field,
    }));
  });

  return (
    <Modal title="查看生效配置" open={open} onCancel={onClose} footer={null} width={900} destroyOnClose>
      <Table
        rowKey="key"
        size="small"
        pagination={{ pageSize: 15, showSizeChanger: false }}
        scroll={{ x: 800 }}
        dataSource={rows}
        columns={[
          { title: '商品', dataIndex: 'productTitle', width: 140, ellipsis: true },
          { title: '刊登目标', dataIndex: 'target', width: 140, ellipsis: true },
          { title: '配置项', dataIndex: 'fieldLabel', width: 140 },
          { title: '生效值', dataIndex: 'value', ellipsis: true },
          { title: '来源', dataIndex: 'source', width: 110 },
        ]}
      />
      <TechnicalDetails label="技术详情">
        <Descriptions size="small" column={1} bordered>
          {rows.slice(0, 20).map((r) => (
            <Descriptions.Item key={r.key} label={r.technicalField}>
              {r.source}
            </Descriptions.Item>
          ))}
        </Descriptions>
      </TechnicalDetails>
    </Modal>
  );
}
