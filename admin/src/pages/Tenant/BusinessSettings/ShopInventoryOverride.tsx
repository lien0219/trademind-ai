import { useEffect, useState } from 'react';
import { Alert, Button, Card, InputNumber, Select, Space, Tag, Typography, message } from 'antd';
import { queryShops, type ShopListRow } from '@/services/shops';
import { getJSON, putJSON } from '@/services/request';
import { usePermission } from '@/hooks/usePermission';
import { canOperateShop } from '@/utils/shopActionAccess';

type ShopOverride = { shopId: string; warningStock: number; origin: 'enterprise' | 'shop'; version: number };
const settingUrl = (shopId: string) => `/api/v1/tenant/shops/${encodeURIComponent(shopId)}/settings/inventory`;

export default function ShopInventoryOverride() {
  const { user } = usePermission();
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [selected, setSelected] = useState('');
  const [snapshot, setSnapshot] = useState<ShopOverride>();
  const [threshold, setThreshold] = useState<number>(5);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const canWrite = selected !== '' && canOperateShop(user, selected);
  useEffect(() => {
    let active = true;
    queryShops({page:1,pageSize:100}).then((res) => {
      if (!active) return;
      const list = res.list || [];
      setShops(list);
      if (list.length > 0) setSelected(list[0].id);
    }).catch(() => {if (active) setError('店铺列表不可用或当前账号没有店铺查看权限');});
    return () => {active = false;};
  }, [user?.tenantId]);
  useEffect(() => {
    if (!selected) return;
    let active = true;
    setSnapshot(undefined);
    setLoading(true);
    setError('');
    getJSON<ShopOverride>(settingUrl(selected)).then((res) => {
      if (!active) return;
      setSnapshot(res);
      setThreshold(res.warningStock);
    }).catch(() => {if (active) setError('无法读取该店铺设置，请确认授权');})
      .finally(() => {if (active) setLoading(false);});
    return () => {active = false;};
  }, [selected]);
  const save = async () => {
    if (!canWrite || !snapshot || loading || !Number.isInteger(threshold) || threshold < 0 || threshold > 1000000) return;
    setLoading(true);
    try {
      const current = await putJSON<ShopOverride, {warningStock:number;expectedVersion:number}>(
        settingUrl(selected), {warningStock:threshold,expectedVersion:snapshot.version},
      );
      setSnapshot(current);
      message.success('店铺阈值覆盖已保存');
    } catch (e) {
      message.error((e as Error)?.message || '保存失败，可能存在版本冲突');
    } finally {setLoading(false);}
  };
  return <Card type="inner" title="店铺级库存阈值覆盖" size="small" style={{marginTop:18}}>
    <Space direction="vertical" style={{width:'100%'}}>
      <Typography.Text type="secondary">选择授权店铺单独配置新 SKU 的预警默认值；不更改已有 SKU。</Typography.Text>
      <Select style={{width:'100%'}} placeholder="选择店铺" value={selected || undefined}
        options={shops.map(shop=>({label:`${shop.shopName} (${shop.platform})`,value:shop.id}))}
        onChange={setSelected} />
      {error?<Alert type="warning" message={error} showIcon />:null}
      {snapshot?<><Tag>{snapshot.origin==='shop'?'店铺覆盖':'继承企业默认'} · 版本 {snapshot.version}</Tag>
        <InputNumber style={{width:'100%'}} value={threshold} min={0} max={1000000} disabled={!canWrite || loading}
          onChange={value=>setThreshold(value ?? 0)} />
        <Button type="primary" disabled={!canWrite || loading} loading={loading} onClick={()=>void save()}>保存店铺覆盖</Button>
      </>:null}
    </Space>
  </Card>;
}
