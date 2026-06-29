import { Alert, Button, Space } from 'antd';
import { history } from '@umijs/max';
import { INVENTORY_SYNC_DISABLED_MESSAGE } from '@/constants/inventoryLabels';

type Props = {
  dismissible?: boolean;
  onDismiss?: () => void;
};

/** 库存同步默认关闭时的统一引导横幅（不自动开启）。 */
export default function InventorySyncDisabledBanner({ dismissible, onDismiss }: Props) {
  return (
    <Alert
      showIcon
      type="info"
      closable={dismissible}
      onClose={onDismiss}
      style={{ marginBottom: 16 }}
      message="平台库存同步未开启"
      description={
        <Space direction="vertical" size={4}>
          <span>{INVENTORY_SYNC_DISABLED_MESSAGE}</span>
          <Space wrap>
            <Button type="link" size="small" style={{ padding: 0 }} onClick={() => history.push('/settings/platforms')}>
              去配置
            </Button>
            <Button type="link" size="small" style={{ padding: 0 }} onClick={() => history.push('/settings/inventory')}>
              查看说明
            </Button>
          </Space>
        </Space>
      }
    />
  );
}
