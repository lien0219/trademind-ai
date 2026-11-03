import { tr } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { ProCard } from '@ant-design/pro-components';
import { Alert, Button, Form, Input, Select, Space, Switch, Typography, message } from 'antd';
import { useEffect, useRef, useState } from 'react';
import {
  getMailTemplate, getMailTemplateOptions, previewMailTemplate, restoreMailTemplate,
  saveMailTemplate, type MailEvent, type MailTemplate,
} from '@/services/mailCenter';

const LANGUAGE_NAMES: Record<string, string> = {
  'zh-CN': '简体中文', 'en-US': 'English', 'ja-JP': '日本語',
  'zh-TW': '繁體中文', 'ko-KR': '한국어', 'th-TH': 'ภาษาไทย',
};

export default function EmailTemplateCenter() {
  useLanguage();
  const [events, setEvents] = useState<MailEvent[]>([]);
  const [locales, setLocales] = useState<string[]>([]);
  const [eventKey, setEventKey] = useState('');
  const [locale, setLocale] = useState('zh-CN');
  const [template, setTemplate] = useState<MailTemplate>();
  const [preview, setPreview] = useState<{ subject: string; html: string }>();
  const [busy, setBusy] = useState(false);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [changed, setChanged] = useState(false);
  const [autoPreview, setAutoPreview] = useState(true);
  const seq = useRef(0);
  const [form] = Form.useForm<MailTemplate>();

  useEffect(() => {
    let active = true;
    getMailTemplateOptions().then((v) => {
      if (!active) return;
      setEvents(v.events);
      setLocales(v.locales);
      setEventKey(v.events[0]?.key || '');
    }).catch(() => { if (active) message.error(tr('邮件模板目录加载失败')); });
    return () => { active = false; seq.current++; };
  }, []);

  useEffect(() => {
    if (!eventKey || !locale) return;
    let active = true;
    const requestID = ++seq.current;
    setTemplate(undefined);
    setPreview(undefined);
    setChanged(false);
    getMailTemplate(eventKey, locale).then((record) => {
      if (!active || requestID !== seq.current) return;
      form.setFieldsValue(record);
      setTemplate(record);
      void renderPreview(record);
    }).catch(() => { if (active) message.error(tr('邮件模板加载失败')); });
    return () => { active = false; };
  }, [eventKey, locale, form]);

  const renderPreview = async (draft: MailTemplate) => {
    const requestID = ++seq.current;
    setPreviewLoading(true);
    try {
      const data = await previewMailTemplate(draft);
      if (requestID === seq.current) setPreview(data);
    } catch {
      if (requestID === seq.current) setPreview(undefined);
    } finally {
      if (requestID === seq.current) setPreviewLoading(false);
    }
  };

  const subject = Form.useWatch('subject', form);
  const html = Form.useWatch('html', form);
  useEffect(() => {
    if (!autoPreview || !template || !changed) return;
    const timer = window.setTimeout(() => {
      void renderPreview({ event: eventKey, locale, subject: subject || '', html: html || '' });
    }, 500);
    return () => window.clearTimeout(timer);
  }, [subject, html, changed, template, eventKey, locale, autoPreview]);

  const save = async () => {
    if (!template) return;
    setBusy(true);
    try {
      const fields = await form.validateFields();
      await saveMailTemplate({ event: eventKey, locale, subject: fields.subject, html: fields.html });
      const current = await getMailTemplate(eventKey, locale);
      setTemplate(current);
      setChanged(false);
      message.success(tr('邮件模板已保存'));
    } catch (error) {
      if (!(error as { errorFields?: unknown }).errorFields) message.error((error as Error)?.message || tr('邮件模板保存失败'));
    } finally { setBusy(false); }
  };

  const restore = async () => {
    setBusy(true);
    try {
      await restoreMailTemplate(eventKey, locale);
      const current = await getMailTemplate(eventKey, locale);
      form.setFieldsValue(current);
      setTemplate(current);
      setChanged(false);
      await renderPreview(current);
      message.success(tr('已恢复默认邮件模板'));
    } catch (error) {
      message.error((error as Error)?.message || tr('恢复邮件模板失败'));
    } finally { setBusy(false); }
  };

  const selected = events.find((v) => v.key === eventKey);
  const actualPreview = preview || { subject: '', html: '' };

  return (
    <ProCard
      title={tr('邮件模板中心')}
      variant="outlined"
      className="tm-system-settings__panel"
      extra={
        <Space wrap>
          <Button onClick={() => { if (template) void renderPreview({ ...template, ...form.getFieldsValue() }); }} loading={previewLoading} disabled={!template}>{tr('预览 / 刷新')}</Button>
          <Button onClick={() => void restore()} disabled={!template} loading={busy}>{tr('恢复默认模板')}</Button>
          <Button type="primary" onClick={() => void save()} disabled={!template || !changed} loading={busy}>{tr('保存模板')}</Button>
        </Space>
      }
    >
      <Space wrap style={{ marginBottom: 16 }}>
        <Typography.Text>{tr('邮件事件')}</Typography.Text>
        <Select
          aria-label={tr('邮件事件')}
          value={eventKey || undefined}
          options={events.map((e) => ({ value: e.key, label: tr(e.name) }))}
          onChange={(next) => { setEventKey(next); setChanged(false); }}
          style={{ minWidth: 220 }}
        />
        <Typography.Text>{tr('邮件语言')}</Typography.Text>
        <Select aria-label={tr('邮件语言')} value={locale}
          options={locales.map((v) => ({ value: v, label: LANGUAGE_NAMES[v] || v }))}
          onChange={(next) => { setLocale(next); setChanged(false); }} style={{ minWidth: 155 }} />
      </Space>
      {selected ? (
        <Alert
          type="info" showIcon style={{ marginBottom: 16 }}
          message={tr(selected.description)}
          description={tr('可用占位符') + ': ' + selected.placeholders.map((p) => '{{' + p + '}}').join('  ')}
        />
      ) : null}
      <Space wrap style={{ marginBottom: 12 }}>
        <Switch checked={autoPreview} onChange={setAutoPreview} />
        <Typography.Text>{tr('实时预览')}</Typography.Text>
        {template?.customized ? <Typography.Text type="secondary">{tr('已自定义')}</Typography.Text> : null}
      </Space>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(min(100%, 320px), 1fr))', gap: 16 }}>
        <Form<MailTemplate> form={form} layout="vertical" onValuesChange={() => setChanged(true)}>
          <Form.Item name="subject" label={tr('邮件主题')} rules={[{ required: true, max: 300 }]}>
            <Input maxLength={300} placeholder="[{{site_name}}]..." />
          </Form.Item>
          <Form.Item name="html" label={tr('HTML 邮件模板')} rules={[{ required: true }]}>
            <Input.TextArea rows={19} style={{ fontFamily: 'monospace', fontSize: 12 }} maxLength={100000} />
          </Form.Item>
        </Form>
        <div>
          <Typography.Text strong>{tr('邮件实时预览')}</Typography.Text>
          <div style={{ marginTop: 8, padding: '8px 12px', background: 'var(--tm-surface-muted, #f7f8fa)', borderRadius: 8, overflowWrap: 'anywhere' }}>
            {actualPreview.subject || tr('点击预览查看邮件主题')}
          </div>
          <iframe
            title={tr('邮件实时预览')}
            sandbox=""
            referrerPolicy="no-referrer"
            srcDoc={actualPreview.html}
            style={{ display: 'block', marginTop: 10, width: '100%', minHeight: 430, border: '1px solid var(--tm-border, #e5e7eb)', borderRadius: 10 }}
          />
          <Typography.Paragraph type="secondary" style={{ marginTop: 10 }}>
            {tr('预览使用安全沙箱，不会发送邮件。模板修改需要单独保存。')}
          </Typography.Paragraph>
        </div>
      </div>
    </ProCard>
  );
}
