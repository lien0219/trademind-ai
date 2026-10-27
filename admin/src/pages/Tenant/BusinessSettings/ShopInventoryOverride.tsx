import { useLanguage } from '@/i18n/useLanguage';
import { useEffect, useRef, useState } from 'react';
import { Alert, Button, Card, InputNumber, Select, Space, Tag, Typography, message } from 'antd';
import { queryShops, type ShopListRow } from '@/services/shops';
import { getJSON, putJSON } from '@/services/request';
import { usePermission } from '@/hooks/usePermission';
import { tr } from '@/i18n';
import { canOperateShop } from '@/utils/shopActionAccess';
import { captureTenantContext, isTenantContextCurrent } from '@/utils/tenantContext';

type ShopOverride = { shopId: string; warningStock: number; origin: 'enterprise' | 'shop'; version: number };
const settingUrl = (shopId: string) => `/api/v1/tenant/shops/${encodeURIComponent(shopId)}/settings/inventory`;

export default function ShopInventoryOverride() {
  useLanguage();
  const { user } = usePermission();
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [selected, setSelected] = useState('');
  const [snapshot, setSnapshot] = useState<ShopOverride>();
  const [threshold, setThreshold] = useState<number>(5);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const mutationPending = useRef(false);
  const listRequest = useRef(0);
  const overrideRequest = useRef(0);
  const selectedRef = useRef(selected);
  selectedRef.current = selected;
  const [saving, setSaving] = useState(false);
  const canWrite = selected !== '' && canOperateShop(user, selected);
  useEffect(() => {
    let active = true;
    const sequence = ++listRequest.current;
    const context = captureTenantContext();
    const current = () => active && sequence === listRequest.current && isTenantContextCurrent(context);
    // Never display a previous organization's store list during reloading.
    selectedRef.current = '';
    setSelected('');
    setSnapshot(undefined);
    setShops([]);
    setError('');
    queryShops({page:1,pageSize:100}).then((res) => {
      if (!current()) return;
      const list = res.list || [];
      setShops(list);
      if (list.length > 0) setSelected(list[0].id);
    }).catch(() => {
      if (current()) setError(tr("店铺列表不可用或当前账号没有店铺查看权限"));
    });
    return () => { active = false; ++listRequest.current; };
  }, [user?.tenantId]);
  useEffect(() => {
    let active = true;
    const sequence = ++overrideRequest.current;
    const context = captureTenantContext();
    const current = () => active && sequence === overrideRequest.current && isTenantContextCurrent(context);
    setSnapshot(undefined);
    if (!selected) { setLoading(false); return () => { active = false; ++overrideRequest.current; }; }
    setLoading(true);
    setError('');
    getJSON<ShopOverride>(settingUrl(selected)).then((res) => {
      if (!current() || res.shopId !== selected) return;
      setSnapshot(res);
      setThreshold(res.warningStock);
    }).catch(() => {
      if (current()) setError(tr("无法读取该店铺设置，请确认授权"));
    }).finally(() => { if (current()) setLoading(false); });
    return () => { active = false; ++overrideRequest.current; };
  }, [selected, user?.tenantId]);
  const save = async () => {
    if (!canWrite || !snapshot || snapshot.shopId !== selected || loading || mutationPending.current || !Number.isInteger(threshold) || threshold < 0 || threshold > 1000000) return;
    const shopId = selected;
    const context = captureTenantContext();
    mutationPending.current = true;
    setSaving(true);
    try {
      const current = await putJSON<ShopOverride, {warningStock:number;expectedVersion:number}>(
        settingUrl(shopId), {warningStock:threshold,expectedVersion:snapshot.version},
      );
      if (selectedRef.current !== shopId || !isTenantContextCurrent(context)) return;
      setSnapshot(current);
      message.success(tr("店铺阈值覆盖已保存"));
    } catch (e) {
      if (selectedRef.current === shopId && isTenantContextCurrent(context))
        message.error((e as Error)?.message || tr("保存失败，可能存在版本冲突"));
    } finally {mutationPending.current = false;setSaving(false);}
  };
  return <Card type="inner" title={tr("店铺级库存阈值覆盖")} size="small" style={{marginTop:18}}>
    <Space direction="vertical" style={{width:'100%'}}>
      <Typography.Text type="secondary">{tr("按店铺筛选库存预警时优先使用此阈值；不修改库存数量或已有规格阈值。")}</Typography.Text>
      <Select style={{width:'100%'}} placeholder={tr("选择店铺")} value={selected || undefined}
        options={shops.map(shop=>({label:`${shop.shopName} (${shop.platform})`,value:shop.id}))}
        disabled={saving} onChange={setSelected} />
      {error?<Alert type="warning" message={error} showIcon />:null}
      {snapshot?<><Tag>{snapshot.origin==='shop'?tr("店铺覆盖"):tr("继承企业默认")} · {tr('版本')} {snapshot.version}</Tag>
        <InputNumber style={{width:'100%'}} value={threshold} min={0} max={1000000} disabled={!canWrite || loading || saving}
          onChange={value=>setThreshold(value ?? 0)} />
        <Button type="primary" disabled={!canWrite || loading || saving} loading={saving} onClick={()=>void save()}>{tr("保存店铺覆盖")}</Button>
      </>:null}
    </Space>
  </Card>;
}
