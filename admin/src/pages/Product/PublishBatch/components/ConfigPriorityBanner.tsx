import { Alert } from 'antd';

const PRIORITY_CHAIN = [
  '系统默认',
  '平台默认',
  '店铺默认',
  '统一配置',
  '商品覆盖',
  '平台覆盖',
  '店铺覆盖',
  '商品目标覆盖',
];

export default function ConfigPriorityBanner() {
  return (
    <Alert
      type="info"
      showIcon
      style={{ marginBottom: 16 }}
      message="配置优先级"
      description={
        <>
          <div style={{ marginBottom: 4 }}>{PRIORITY_CHAIN.join(' → ')}</div>
          <span style={{ color: 'rgba(0,0,0,0.45)' }}>
            越靠后的配置优先级越高。单个商品在单个店铺的配置，会覆盖前面的统一配置。
          </span>
        </>
      }
    />
  );
}
