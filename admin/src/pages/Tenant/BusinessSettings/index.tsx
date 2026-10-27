import { useEffect, useState, useRef } from 'react';
import { Alert, Button, Card, Form, Grid, Input, InputNumber, Modal, Select, Space, Spin, Tag, Typography, message, Popconfirm, Switch } from 'antd';
import { HistoryOutlined, ReloadOutlined, SaveOutlined } from '@ant-design/icons';
import { TmPageContainer } from '@/components/ui';
import { tr, formatLocalizedDate } from '@/i18n';
import { languageNames, supportedLanguages } from '@/i18n/dictionaries';
import { captureTenantContext, isTenantContextCurrent } from '@/utils/tenantContext';
import ShopInventoryOverride from './ShopInventoryOverride';
import { usePermission } from '@/hooks/usePermission';
import { PERMISSIONS } from '@/utils/permission';
import { getBusinessSettings, putBusinessSettings, listBusinessSettingsHistory, restoreBusinessSettings, type BusinessModule, type BusinessSnapshot, type BusinessRevision } from '@/services/tenantBusinessSettings';

const labels: Record<BusinessModule, string> = {
  general: '基础偏好',
  ai: 'AI 与内容',
  collect: '采集与发布',
  inventory: '库存默认规则',
  notifications: '企业通知',
  orders: '订单默认规则',
  branding: '企业品牌与标签',
};
const modules: BusinessModule[] = ['general', 'ai', 'collect', 'inventory', 'notifications', 'orders', 'branding'];
const templates: Partial<Record<BusinessModule, {name:string;values:Record<string,unknown>}[]>> = {
  ai: [
    {name:'中文内容创作',values:{defaultLanguage:'zh-CN'}},
    {name:'国际站英文内容',values:{defaultLanguage:'en-US'}},
    {name:'日本站内容',values:{defaultLanguage:'ja-JP'}},
  ],
  collect: [
    {name:'继承平台采集策略',values:{allowedSources:[],maxBatchURLs:0}},
    {name:'小批量人工采集',values:{allowedSources:[],maxBatchURLs:10}},
  ],
  inventory: [
    {name:'保守库存预警',values:{lowStockThreshold:20}},
    {name:'标准库存预警',values:{lowStockThreshold:5}},
  ],
};
const languageOptions = supportedLanguages.map(value => ({value, label: languageNames[value]}));
const sourceOptions = [
  {value:'1688',label:'1688'}, {value:'pinduoduo',label:'拼多多'},
  {value:'taobao_tmall',label:'淘宝/天猫'}, {value:'aliexpress',label:'速卖通'},
  {value:'custom',label:'自定义采集'},
];

export default function TenantBusinessSettingsPage() {
  const { can } = usePermission();
  const screens = Grid.useBreakpoint();
  const writable = can(PERMISSIONS.SETTINGS_MANAGE);
  const [module, setModule] = useState<BusinessModule>('general');
  const [snapshot, setSnapshot] = useState<BusinessSnapshot | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [revisions, setRevisions] = useState<BusinessRevision[]>([]);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [preview, setPreview] = useState<{name:string;values:Record<string,unknown>} | null>(null);
  const [error, setError] = useState('');
  const [form] = Form.useForm();
  const fileInput = useRef<HTMLInputElement>(null);
  const selectedModule = useRef<BusinessModule>('general');
  const readSequence = useRef(0);
  const mutationPending = useRef(false);
  const [exporting, setExporting] = useState(false);

  const load = async (key: BusinessModule) => {
    if (mutationPending.current || selectedModule.current !== key) return;
    const request = ++readSequence.current;
    const context = captureTenantContext();
    const isCurrent = () => request === readSequence.current && selectedModule.current === key && isTenantContextCurrent(context);
    setLoading(true);
    setError('');
    setSnapshot(null);
    setHistoryOpen(false);
    setPreview(null);
    form.resetFields();
    try {
      const current = await getBusinessSettings(key);
      if (!isCurrent()) return;
      setSnapshot(current);
      form.setFieldsValue(current.values);
    } catch (e) {
      if (isCurrent())
        setError((e as Error)?.message || tr('加载失败，请重试'));
    } finally {
      if (request === readSequence.current) setLoading(false);
    }
  };
  useEffect(() => { void load(module); }, [module]);

  const chooseModule = (next:BusinessModule) => {
    if (next === selectedModule.current || mutationPending.current) return;
    const switchModule = () => {
      if (mutationPending.current) return;
      selectedModule.current = next;
      ++readSequence.current;
      setPreview(null);
      setModule(next);
    };
    if (form.isFieldsTouched()) {
      Modal.confirm({title:tr('放弃未保存的更改？'),content:tr('切换模块前请确认更改已保存。'),okText:tr('放弃并切换'),cancelText:tr('继续编辑'),onOk:switchModule});
    } else switchModule();
  };
  const save = async () => {
    if (!writable || !snapshot || snapshot.module !== module || mutationPending.current) return;
    const key = module;
    const context = captureTenantContext();
    try {
      const values = await form.validateFields();
      if (selectedModule.current !== key || mutationPending.current || !isTenantContextCurrent(context)) return;
      mutationPending.current = true;
      setSaving(true);
      const next = await putBusinessSettings(key, {expectedVersion:snapshot.version, values});
      if (selectedModule.current === key && isTenantContextCurrent(context)) {
        setSnapshot(next);
        form.setFieldsValue(next.values);
        message.success(tr('企业业务配置已保存；已接入的业务模块将在新任务中使用'));
        if (historyOpen) setRevisions((await listBusinessSettingsHistory(key)).items);
      }
    } catch (e) { message.error((e as Error)?.message || tr('配置保存失败，请检查字段')); }
    finally { mutationPending.current = false; setSaving(false); }
  };
  const openHistory = async () => {
    if (!writable || !snapshot || snapshot.module !== module) return;
    const key = module;
    const context = captureTenantContext();
    try {
      const rows = await listBusinessSettingsHistory(key);
      if (selectedModule.current !== key || !isTenantContextCurrent(context)) return;
      setRevisions(rows.items || []);
      setHistoryOpen(true);
    } catch (e) {
      if (selectedModule.current === key && isTenantContextCurrent(context)) message.error((e as Error)?.message || tr('无法加载历史'));
    }
  };
  const restore = async (version: number) => {
    if (!snapshot || !writable || snapshot.module !== module || mutationPending.current) return;
    const key = module;
    const context = captureTenantContext();
    mutationPending.current = true;
    setSaving(true);
    try {
      const next = await restoreBusinessSettings(key, {targetVersion:version, expectedVersion:snapshot.version});
      if (selectedModule.current === key && isTenantContextCurrent(context)) {
        setSnapshot(next);
        form.setFieldsValue(next.values);
        setRevisions((await listBusinessSettingsHistory(key)).items);
        message.success(tr('已恢复为新版本；历史记录未被覆盖'));
      }
    } catch (e) { message.error((e as Error)?.message || tr('恢复失败，请刷新')); }
    finally { mutationPending.current = false; setSaving(false); }
  };
  const exportCurrent = () => {
    if (!snapshot) return;
    const documentBody = JSON.stringify({format:'trademind-tenant-config-v1',module:snapshot.module,values:snapshot.values},null,2);
    const blob = new Blob([documentBody],{type:'application/json'});
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href=url;
    link.download=`tenant-${snapshot.module}-settings.json`;
    link.click();
    URL.revokeObjectURL(url);
  };
  const exportAll = async () => {
    if (exporting) return;
    const context = captureTenantContext();
    setExporting(true);
    try {
      // Export all tenant modules as read-only snapshots. Never include
      // platform provider credentials, personal preferences, or user tokens.
      const snapshots = await Promise.all(modules.map(async key => getBusinessSettings(key)));
      if (!isTenantContextCurrent(context)) return;
      const body = JSON.stringify({
        format: 'trademind-tenant-config-bundle-v1',
        exportedAt: new Date().toISOString(),
        modules: Object.fromEntries(snapshots.map(item => [item.module, {
          version: item.version, inherited: item.inherited, values: item.values,
        }])),
      }, null, 2);
      const url = URL.createObjectURL(new Blob([body], {type: 'application/json'}));
      const link = document.createElement('a');
      link.href = url;
      link.download = 'tenant-business-settings-bundle.json';
      link.click();
      URL.revokeObjectURL(url);
      message.success(tr('已导出全部企业业务配置，不包含任何密钥'));
    } catch (e) {
      message.error((e as Error)?.message || tr('导出失败，请检查读取权限后重试'));
    } finally { setExporting(false); }
  };
  const importFile = async (file: File) => {
    if (!writable || !snapshot || mutationPending.current || file.size > 8192) { message.error(tr('配置文件不可导入或超过大小限制')); return; }
    const key = module;
    const context = captureTenantContext();
    try {
      const parsed: unknown = JSON.parse(await file.text());
      if (selectedModule.current !== key || !isTenantContextCurrent(context)) return;
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error(tr('文件不是配置对象'));
      const obj = parsed as Record<string, unknown>;
      if (obj.format !== 'trademind-tenant-config-v1' || obj.module !== module || !obj.values || typeof obj.values !== 'object' || Array.isArray(obj.values)) throw new Error(tr('配置格式或模块不匹配'));
      // Import only populates the form; saving always invokes server-side schema validation and version checks.
      form.setFieldsValue(obj.values);
      message.info(tr('已载入配置预览，检查后点击保存才会生效'));
    } catch (e) { message.error((e as Error)?.message || tr('导入失败')); }
  };
  return <TmPageContainer title={tr("企业业务配置")} subTitle={tr("当前企业专属默认规则；平台安全限制与其他企业均不受影响。")}
    extra={<Space wrap>
      <Button onClick={exportCurrent} disabled={!snapshot}>{tr("导出配置")}</Button>
      <Button loading={exporting} disabled={exporting} onClick={() => void exportAll()}>{tr("导出全部模块")}</Button>
      {writable?<><input ref={fileInput} type="file" accept=".json,application/json" hidden onChange={e=>{const file=e.target.files?.[0];if(file) void importFile(file);e.target.value='';}} />
        <Button disabled={!snapshot || saving} onClick={()=>fileInput.current?.click()}>{tr("导入预览")}</Button></>:null}
      <Button icon={<ReloadOutlined />} disabled={saving} onClick={() => void load(module)} loading={loading}>{tr("刷新")}</Button>
    </Space>}>
    <div style={{display:'grid',gridTemplateColumns:screens.md ? 'minmax(150px, 210px) minmax(0, 1fr)' : 'minmax(0,1fr)',gap:16}}>
      <Card size="small" styles={{body:{padding:8}}}>
        <Space direction="vertical" style={{width:'100%'}}>
          {modules.map(item=><Button key={item} block disabled={saving} type={module===item?'primary':'text'} style={{textAlign:'left'}} onClick={()=>chooseModule(item)}>{tr(labels[item])}</Button>)}
        </Space>
      </Card>
      <Card title={<div style={{display:'flex',flexWrap:'wrap',gap:8,alignItems:'center',justifyContent:'space-between',whiteSpace:'normal'}}>
        <span>{tr(labels[module])}</span>
        <Space wrap><Tag>{snapshot?.inherited?tr('继承默认值'):`${tr('版本')} ${snapshot?.version || 0}`}</Tag>{writable?<Button icon={<HistoryOutlined />} onClick={()=>void openHistory()}>{tr("版本历史")}</Button>:null}</Space>
      </div>}>
        {error?<Alert type="error" showIcon message={error} action={<Button onClick={()=>void load(module)}>{tr("重试")}</Button>}/>:null}
        {loading || !snapshot?<div style={{padding:30,textAlign:'center'}}><Spin /></div>:
          <Form form={form} layout="vertical" disabled={!writable || saving}>
            {module==='general'?<>
              <Form.Item name="language" label={tr("默认界面语言")} rules={[{required:true}]}><Select options={languageOptions}/></Form.Item>
              <Form.Item name="timezone" label={tr("默认时区")} rules={[{required:true}]}><Select options={[
                {value:'Asia/Shanghai',label:tr("上海 / UTC+08:00")}, {value:'Asia/Tokyo',label:tr("东京 / UTC+09:00")}, {value:'UTC',label:'UTC'},
              ]}/></Form.Item>
              <Form.Item name="currency" label={tr("报表显示货币")} rules={[{required:true}]}><Select options={[
                {value:'CNY',label:tr("人民币 CNY")}, {value:'USD',label:tr("美元 USD")}, {value:'JPY',label:tr("日元 JPY")},
              ]}/></Form.Item>
              <Alert type="info" showIcon message={tr("语言与时区用于界面显示")} description={tr("个人语言优先于企业默认语言；货币偏好不执行换汇，也不改写订单金额。")}/>
            </>:null}
            {module==='ai'?<>
              <Form.Item name="defaultLanguage" label={tr("AI 文案默认语言")} rules={[{required:true}]}><Select options={languageOptions}/></Form.Item>
              <Alert type="success" showIcon message={tr("已接入 AI 文案批次")} description={tr("新建 AI 文案批次未显式指定语言时使用此默认值；已有任务保留创建时输入。")}/>
            </>:null}
            {module==='collect'?<>
              <Form.Item name="allowedSources" label={tr("允许的采集来源")} extra={tr("留空表示继承平台所有已开放来源；设置后，未选来源会被后端拒绝。")}><Select mode="multiple" allowClear options={sourceOptions.map(item => ({...item, label:tr(item.label)}))} placeholder={tr("继承全部来源")}/></Form.Item>
              <Form.Item name="maxBatchURLs" label={tr("企业采集批次链接上限")} extra={tr("0 表示继承平台上限；企业值只能收紧，不能提高平台上限。")} rules={[{required:true}]}><InputNumber min={0} max={100} style={{width:'100%'}}/></Form.Item>
              <Alert type="success" showIcon message={tr("已接入采集任务创建")} description={tr("单条与批量新任务执行服务端校验，不能仅通过绕开 UI 来突破企业限制。")}/>
            </>:null}
            {module==='inventory'?<>
              <Form.Item name="lowStockThreshold" label={tr("企业默认低库存阈值")} rules={[{required:true}]}><InputNumber min={0} max={1000000} style={{width:'100%'}}/></Form.Item>
              <Alert type="info" showIcon message={tr("已接入新商品库存阈值")} description={tr("新采集商品使用企业默认阈值；已有规格保留显式阈值，按店铺筛选预警时使用店铺覆盖。")} />
            </>:null}
            {module==='notifications'?<>
              <Form.Item name="businessAlertsEnabled" label={tr("开启企业业务通知")} valuePropName="checked"><Switch/></Form.Item>
              <Form.Item name="mailRecipients" label={tr("企业告警邮件接收人")} extra={tr("最多 10 个地址；留空继承平台通知收件人。SMTP 配置由平台统一管理。")}>
                <Select mode="tags" tokenSeparators={[',']} maxCount={10} placeholder="ops@example.com"/>
              </Form.Item>
              <Alert type="info" showIcon message={tr("已接入任务告警通知")} description={tr("关闭后停止当前企业任务告警的自动投递；不影响人工通知或平台渠道安全限制。")} />
            </>:null}
            {module==='orders'?<>
              <Form.Item name="defaultCurrency" label={tr("手工订单默认币种")} rules={[{required:true}]}>
                <Select options={[{value:'USD',label:tr("美元 USD")},{value:'CNY',label:tr("人民币 CNY")},{value:'JPY',label:tr("日元 JPY")},{value:'EUR',label:tr("欧元 EUR")},{value:'GBP',label:tr("英镑 GBP")}]}/>
              </Form.Item>
              <Alert type="success" showIcon message={tr("已接入手工订单创建")} description={tr("创建手工订单且未指定币种时应用当前企业默认值；平台订单同步不会被改写，已存在订单的币种和金额保持不变。")}/>
            </>:null}
            {module==='branding'?<>
              <Form.Item name="displayName" label={tr("企业展示名称")}><Input maxLength={120}/></Form.Item>
              <Form.Item name="logoUrl" label={tr("企业 Logo HTTPS 地址")} rules={[{type:'url'}]}><Input placeholder="https://..."/></Form.Item>
              <Form.Item name="labels" label={tr("企业标签")}><Select mode="tags" maxCount={20} placeholder={tr("添加标签，回车确认")}/></Form.Item>
            </>:null}
            {writable && (templates[module]?.length ?? 0)>0?<div style={{marginBottom:16}}>
              <Typography.Text strong>{tr("配置模板")}</Typography.Text>
              <div style={{marginTop:8}}><Space wrap>{templates[module]?.map(template=>
                <Button key={template.name} size="small" onClick={()=>setPreview(template)}>{tr(template.name)}</Button>
              )}</Space></div>
              <Typography.Text type="secondary">{tr("模板只会预填表单；保存后才会生效。")}</Typography.Text>
            </div>:null}
            <Space wrap><Button type="primary" icon={<SaveOutlined />} disabled={!writable} loading={saving} onClick={()=>void save()}>{tr("保存企业配置")}</Button><Button onClick={()=>void load(module)}>{tr("放弃更改")}</Button></Space>
          </Form>}
        {module==='inventory' && !loading && snapshot ? <ShopInventoryOverride/> : null}
        {historyOpen?<div style={{marginTop:20}}>
          <Typography.Title level={5}>{tr("配置变更历史")}</Typography.Title>
          <Space direction="vertical" style={{width:'100%'}}>{revisions.length===0?<Typography.Text type="secondary">{tr("暂无历史版本")}</Typography.Text>:revisions.map(row=><div key={row.version} style={{display:'flex',flexWrap:'wrap',justifyContent:'space-between',gap:10,alignItems:'center',padding:'8px 0',borderBottom:'1px solid var(--ant-color-border-secondary)'}}>
            <span>{tr('版本')} {row.version} · {row.action} · {formatLocalizedDate(row.createdAt)}</span>
            <Button size="small" disabled={saving} onClick={()=>setPreview({name:`${tr('历史版本')} ${row.version} ${tr('与当前配置对比')}`,values:row.values as Record<string,unknown>})}>{tr("对比当前")}</Button>
            <Popconfirm title={`${tr('恢复版本')} ${row.version}？`} description={tr("恢复会生成新版本并保留审计记录。")} onConfirm={()=>void restore(row.version)}>
              <Button size="small" disabled={saving || !writable}>{tr("恢复")}</Button>
            </Popconfirm>
          </div>)}</Space>
        </div>:null}
      </Card>
    </div>
    <Modal title={preview?`${tr('模板预览：')}${tr(preview.name)}`:tr('配置预览')} open={Boolean(preview)} onCancel={()=>setPreview(null)}
      okText={tr('应用到表单')} okButtonProps={{disabled:saving || !writable}} onOk={()=>{if(preview && !saving && writable && snapshot?.module === module)form.setFieldsValue(preview.values);setPreview(null);}}>
      <Typography.Paragraph type="secondary">{tr("仅列出模板将修改的字段，未列出的字段不变。")}</Typography.Paragraph>
      {preview?Object.entries(preview.values).map(([key,value])=><div key={key} style={{marginBottom:10}}>
        <Typography.Text strong>{key}</Typography.Text>
        <Typography.Paragraph style={{marginBottom:0}}>{tr('当前：')}{JSON.stringify(form.getFieldValue(key))}</Typography.Paragraph>
        <Typography.Paragraph style={{marginBottom:0}}>{tr('模板：')}{JSON.stringify(value)}</Typography.Paragraph>
      </div>):null}
    </Modal>
  </TmPageContainer>;
}
