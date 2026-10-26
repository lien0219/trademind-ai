import { useEffect, useState } from 'react';
import { Alert, Button, Card, Form, Input, InputNumber, Select, Space, Spin, Tag, Typography, message, Popconfirm, Switch } from 'antd';
import { HistoryOutlined, ReloadOutlined, SaveOutlined } from '@ant-design/icons';
import { TmPageContainer } from '@/components/ui';
import { usePermission } from '@/hooks/usePermission';
import { PERMISSIONS } from '@/utils/permission';
import { getBusinessSettings, putBusinessSettings, listBusinessSettingsHistory, restoreBusinessSettings, type BusinessModule, type BusinessSnapshot, type BusinessRevision } from '@/services/tenantBusinessSettings';

const labels: Record<BusinessModule, string> = {
  general: '基础偏好',
  ai: 'AI 与内容',
  collect: '采集与发布',
  inventory: '库存默认规则',
  notifications: '企业通知',
  branding: '企业品牌与标签',
};
const modules: BusinessModule[] = ['general', 'ai', 'collect', 'inventory', 'notifications', 'branding'];
const languageOptions = [{value:'zh-CN',label:'简体中文'},{value:'en-US',label:'English'},{value:'ja-JP',label:'日本語'}];
const sourceOptions = [
  {value:'1688',label:'1688'}, {value:'pinduoduo',label:'拼多多'},
  {value:'taobao_tmall',label:'淘宝/天猫'}, {value:'aliexpress',label:'速卖通'},
  {value:'custom',label:'自定义采集'},
];

export default function TenantBusinessSettingsPage() {
  const { can } = usePermission();
  const writable = can(PERMISSIONS.SETTINGS_MANAGE);
  const [module, setModule] = useState<BusinessModule>('general');
  const [snapshot, setSnapshot] = useState<BusinessSnapshot | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [revisions, setRevisions] = useState<BusinessRevision[]>([]);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [error, setError] = useState('');
  const [form] = Form.useForm();

  const load = async (key: BusinessModule) => {
    setLoading(true);
    setError('');
    setSnapshot(null);
    setHistoryOpen(false);
    try {
      const current = await getBusinessSettings(key);
      setSnapshot(current);
      form.setFieldsValue(current.values);
    } catch (e) { setError((e as Error)?.message || '加载失败，请重试'); }
    finally { setLoading(false); }
  };
  useEffect(() => { void load(module); }, [module]);

  const save = async () => {
    if (!writable || !snapshot || saving) return;
    try {
      const values = await form.validateFields();
      setSaving(true);
      const next = await putBusinessSettings(module, {expectedVersion:snapshot.version,values});
      setSnapshot(next);
      form.setFieldsValue(next.values);
      message.success('企业业务配置已保存并对新任务生效');
      if (historyOpen) setRevisions((await listBusinessSettingsHistory(module)).items);
    } catch (e) { message.error((e as Error)?.message || '配置保存失败，请检查字段'); }
    finally { setSaving(false); }
  };
  const openHistory = async () => {
    if (!writable) return;
    try {
      const rows = await listBusinessSettingsHistory(module);
      setRevisions(rows.items || []);
      setHistoryOpen(true);
    } catch (e) { message.error((e as Error)?.message || '无法加载历史'); }
  };
  const restore = async (version: number) => {
    if (!snapshot || !writable || saving) return;
    setSaving(true);
    try {
      const next = await restoreBusinessSettings(module, {targetVersion:version, expectedVersion:snapshot.version});
      setSnapshot(next);
      form.setFieldsValue(next.values);
      setRevisions((await listBusinessSettingsHistory(module)).items);
      message.success('已恢复为新版本；历史记录未被覆盖');
    } catch (e) { message.error((e as Error)?.message || '恢复失败，请刷新'); }
    finally { setSaving(false); }
  };
  return <TmPageContainer title="企业业务配置" subTitle="当前企业专属默认规则；平台安全限制与其他企业均不受影响。"
    extra={<Button icon={<ReloadOutlined />} onClick={() => void load(module)} loading={loading}>刷新</Button>}>
    <div style={{display:'grid',gridTemplateColumns:'minmax(150px, 210px) minmax(0, 1fr)',gap:16}}>
      <Card size="small" styles={{body:{padding:8}}}>
        <Space direction="vertical" style={{width:'100%'}}>
          {modules.map(item=><Button key={item} block type={module===item?'primary':'text'} style={{textAlign:'left'}} onClick={()=>setModule(item)}>{labels[item]}</Button>)}
        </Space>
      </Card>
      <Card title={labels[module]} extra={<Space><Tag>{snapshot?.inherited?'继承默认值':`版本 ${snapshot?.version || 0}`}</Tag>{writable?<Button icon={<HistoryOutlined />} onClick={()=>void openHistory()}>版本历史</Button>:null}</Space>}>
        {error?<Alert type="error" showIcon message={error} action={<Button onClick={()=>void load(module)}>重试</Button>}/>:null}
        {loading || !snapshot?<div style={{padding:30,textAlign:'center'}}><Spin /></div>:
          <Form form={form} layout="vertical" disabled={!writable || saving}>
            {module==='general'?<>
              <Form.Item name="language" label="默认界面语言" rules={[{required:true}]}><Select options={languageOptions}/></Form.Item>
              <Form.Item name="timezone" label="默认时区" rules={[{required:true}]}><Select options={[
                {value:'Asia/Shanghai',label:'上海 / UTC+08:00'}, {value:'Asia/Tokyo',label:'东京 / UTC+09:00'}, {value:'UTC',label:'UTC'},
              ]}/></Form.Item>
              <Form.Item name="currency" label="报表显示货币" rules={[{required:true}]}><Select options={[
                {value:'CNY',label:'人民币 CNY'}, {value:'USD',label:'美元 USD'}, {value:'JPY',label:'日元 JPY'},
              ]}/></Form.Item>
              <Alert type="info" showIcon message="基础偏好暂仅存储和回显" description="现有报表未统一接入汇率/本地化解析器，因此此页不会修改原始账务或改变所有页面的显示。"/>
            </>:null}
            {module==='ai'?<>
              <Form.Item name="defaultLanguage" label="AI 文案默认语言" rules={[{required:true}]}><Select options={languageOptions}/></Form.Item>
              <Alert type="success" showIcon message="已接入 AI 文案批次" description="新建 AI 文案批次未显式指定语言时使用此默认值；已有任务保留创建时输入。"/>
            </>:null}
            {module==='collect'?<>
              <Form.Item name="allowedSources" label="允许的采集来源" extra="留空表示继承平台所有已开放来源；设置后，未选来源会被后端拒绝。"><Select mode="multiple" allowClear options={sourceOptions} placeholder="继承全部来源"/></Form.Item>
              <Form.Item name="maxBatchURLs" label="企业采集批次链接上限" extra="0 表示继承平台上限；企业值只能收紧，不能提高平台上限。" rules={[{required:true}]}><InputNumber min={0} max={100} style={{width:'100%'}}/></Form.Item>
              <Alert type="success" showIcon message="已接入采集任务创建" description="单条与批量新任务执行服务端校验，不能仅通过绕开 UI 来突破企业限制。"/>
            </>:null}
            {module==='inventory'?<>
              <Form.Item name="lowStockThreshold" label="企业默认低库存阈值" rules={[{required:true}]}><InputNumber min={0} max={1000000} style={{width:'100%'}}/></Form.Item>
              <Alert type="info" showIcon message="待接入库存预警" description="当前为企业规则存储，库存 SKU 预警仍采用现有 SKU 和全局配置；不会暗中修改库存值。" />
            </>:null}
            {module==='notifications'?<>
              <Form.Item name="businessAlertsEnabled" label="开启企业业务通知" valuePropName="checked"><Switch/></Form.Item>
              <Alert type="info" showIcon message="待接入通知投递" description="当前仅持久化企业开关，邮件/Webhook 不会因为此设置自动发送。" />
            </>:null}
            {module==='branding'?<>
              <Form.Item name="displayName" label="企业展示名称"><Input maxLength={120}/></Form.Item>
              <Form.Item name="logoUrl" label="企业 Logo HTTPS 地址" rules={[{type:'url'}]}><Input placeholder="https://..."/></Form.Item>
              <Form.Item name="labels" label="企业标签"><Select mode="tags" maxCount={20} placeholder="添加标签，回车确认"/></Form.Item>
            </>:null}
            <Space><Button type="primary" icon={<SaveOutlined />} disabled={!writable} loading={saving} onClick={()=>void save()}>保存企业配置</Button><Button onClick={()=>void load(module)}>放弃更改</Button></Space>
          </Form>}
        {historyOpen?<div style={{marginTop:20}}>
          <Typography.Title level={5}>配置变更历史</Typography.Title>
          <Space direction="vertical" style={{width:'100%'}}>{revisions.length===0?<Typography.Text type="secondary">暂无历史版本</Typography.Text>:revisions.map(row=><div key={row.version} style={{display:'flex',justifyContent:'space-between',gap:10,alignItems:'center',padding:'8px 0',borderBottom:'1px solid var(--ant-color-border-secondary)'}}>
            <span>版本 {row.version} · {row.action} · {new Date(row.createdAt).toLocaleString()}</span>
            <Popconfirm title={`恢复版本 ${row.version}？`} description="恢复会生成新版本并保留审计记录。" onConfirm={()=>void restore(row.version)}>
              <Button size="small" disabled={saving || !writable}>恢复</Button>
            </Popconfirm>
          </div>)}</Space>
        </div>:null}
      </Card>
    </div>
  </TmPageContainer>;
}
