import { useLanguage } from '@/i18n/useLanguage';
import { useEffect, useRef, useState } from 'react';
import { Alert, Button, Card, Form, Modal, Select, Space, Spin, message } from 'antd';
import { TmPageContainer } from '@/components/ui';
import { getJSON, putJSON } from '@/services/request';
import { applyThemeMode, persistThemeMode } from '@/theme';
import { readRegionalPreferences, setLanguage, setRegionalPreferences, tr } from '@/i18n';
import { isAppLanguage, languageNames, supportedLanguages } from '@/i18n/dictionaries';
import { captureTenantContext, isTenantContextCurrent } from '@/utils/tenantContext';
import { useAntdConfigSetter } from '@umijs/max';
import { createAdminThemeConfig } from '@/theme';

type Preferences={language:string;theme:'light'|'dark';dateFormat:string};
type PreferenceState={values:Preferences;version:number;inherited:boolean};
const url='/api/v1/tenant/preferences';

export default function PersonalPreferencesPage(){
  useLanguage();
  const setAntdConfig=useAntdConfigSetter();
  const [form]=Form.useForm<Preferences>();
  const [snapshot,setSnapshot]=useState<PreferenceState>();
  const [loading,setLoading]=useState(true);
  const [saving,setSaving]=useState(false);
  const mutationPending=useRef(false);
  const readSequence=useRef(0);
  const [error,setError]=useState('');
  const load=async()=>{
    if(mutationPending.current)return;
    const sequence=++readSequence.current;
    const context=captureTenantContext();
    setLoading(true);setError('');setSnapshot(undefined);
    try{const s=await getJSON<PreferenceState>(url);if(sequence!==readSequence.current||!isTenantContextCurrent(context))return;setSnapshot(s);}
    catch(e){if(sequence===readSequence.current&&isTenantContextCurrent(context))setError((e as Error)?.message||tr('个人偏好加载失败'));}
    finally{if(sequence===readSequence.current)setLoading(false);}
  };
  useEffect(()=>{void load();return()=>{++readSequence.current;};},[]);
  // Avoid writing to an AntD Form instance before the conditional Form mounts.
  useEffect(()=>{if(!loading && snapshot)form.setFieldsValue(snapshot.values);},[loading,snapshot,form]);
  const restore=()=>{
    if(loading || mutationPending.current)return;
    if(!form.isFieldsTouched()){void load();return;}
    Modal.confirm({
      title:tr('放弃未保存的更改？'),
      content:tr('放弃并重新加载当前设置？'),
      okText:tr('放弃更改'),
      cancelText:tr('继续编辑'),
      onOk:()=>{if(!mutationPending.current)void load();},
    });
  };
  const save=async()=>{if(!snapshot||loading||mutationPending.current)return;
    mutationPending.current=true;
    const context=captureTenantContext();
    try{
    const values=await form.validateFields();
    if(!isTenantContextCurrent(context))return;
    setSaving(true);
    const next=await putJSON<PreferenceState,{values:Preferences;expectedVersion:number}>(url,{values,expectedVersion:snapshot.version});
    if(!isTenantContextCurrent(context))return;
    setSnapshot(next);
    setRegionalPreferences({...readRegionalPreferences(),dateFormat:next.values.dateFormat});
    persistThemeMode(next.values.theme);
    applyThemeMode(next.values.theme);
    setAntdConfig({theme:createAdminThemeConfig(next.values.theme)});
    message.success(tr('个人偏好已保存；部分设置将在重新进入页面后应用'));
    if(isAppLanguage(next.values.language))setLanguage(next.values.language);
  }catch(e){
    // Field validation already shows its own inline errors.
    if(e && typeof e==='object' && 'errorFields' in e)return;
    if(isTenantContextCurrent(context))message.error((e as Error)?.message||tr('保存失败，请刷新后重试'));
  }finally{mutationPending.current=false;setSaving(false)}};
  return <TmPageContainer title={tr("个人偏好")} subTitle={tr("仅对当前企业的当前账号有效，不改变其他成员、店铺或企业的业务规则。")}>
    <Card style={{maxWidth:720}}>
      {error?<Alert type="error" message={error} action={<Button disabled={saving} onClick={()=>void load()}>{tr('重试')}</Button>}/>:null}
      {loading?<Spin/>:snapshot?<Form form={form} layout="vertical" disabled={saving}>
        <Form.Item name="language" label={tr("首选语言")} rules={[{required:true}]}><Select options={supportedLanguages.map(value=>({value,label:languageNames[value]}))}/></Form.Item>
        <Form.Item name="theme" label={tr("界面主题")} rules={[{required:true}]}><Select options={[{value:'light',label:tr('浅色模式')},{value:'dark',label:tr('深色模式')}]}/></Form.Item>
        <Form.Item name="dateFormat" label={tr('首选日期格式')} rules={[{required:true}]}><Select options={[{value:'YYYY-MM-DD',label:'2026-10-27'},{value:'DD/MM/YYYY',label:'27/10/2026'},{value:'MM/DD/YYYY',label:'10/27/2026'}]}/></Form.Item>
        <Alert type="info" showIcon message={tr("偏好生效范围")} description={tr("语言和主题立即生效；日期格式适用于已接入统一日期组件的页面。")}/>
        <Space style={{marginTop:16}}><Button type="primary" loading={saving} onClick={()=>void save()}>{tr("保存个人偏好")}</Button><Button disabled={saving} onClick={restore}>{tr("还原")}</Button></Space>
      </Form>:null}
    </Card>
  </TmPageContainer>;
}
