import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Button, Form, Input, InputNumber, Select, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { fetchSettingsList, saveSettingsItems } from '@/services/settings';
import { pickGroup, toPutItems, type FieldSpec } from '@/utils/settingsForm';

const GROUP = 'image';

const FIELDS: Record<string, FieldSpec> = {
  provider: {},
  removebg_api_key: { encrypted: true },
  removebg_base_url: {},
  openai_image_base_url: {},
  openai_image_api_key: { encrypted: true },
  openai_image_model: {},
  openai_image_size: {},
  openai_image_quality: {},
  openai_image_background: {},
  comfyui_base_url: {},
  comfyui_workflow_json: { valueType: 'json' },
  timeout_sec: {},
};

export default function ImageSettingsPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const { items } = await fetchSettingsList();
      const g = pickGroup(items, GROUP);
      form.setFieldsValue({
        provider: g.provider || 'noop',
        removebg_api_key: g.removebg_api_key || '',
        removebg_base_url: g.removebg_base_url || 'https://api.remove.bg/v1.0',
        openai_image_base_url: g.openai_image_base_url || '',
        openai_image_api_key: g.openai_image_api_key || '',
        openai_image_model: g.openai_image_model || 'gpt-image-1',
        openai_image_size: g.openai_image_size || '1024x1024',
        openai_image_quality: g.openai_image_quality || 'standard',
        openai_image_background: g.openai_image_background || '',
        comfyui_base_url: g.comfyui_base_url || '',
        comfyui_workflow_json: g.comfyui_workflow_json || '{}',
        timeout_sec: g.timeout_sec ? Number(g.timeout_sec) : 60,
      });
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <PageContainer title="图片 AI 设置">
      <ProCard
        bordered
        extra={
          <Button type="link" onClick={load} disabled={loading}>
            重新加载
          </Button>
        }
      >
        <Form
          form={form}
          layout="vertical"
          style={{ maxWidth: 640 }}
          onFinish={async (values) => {
            try {
              const payload = {
                ...values,
                timeout_sec: String(values.timeout_sec ?? ''),
                comfyui_workflow_json: String(values.comfyui_workflow_json ?? '{}'),
              };
              await saveSettingsItems(toPutItems(GROUP, FIELDS, payload));
              message.success('已保存');
              await load();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '保存失败');
            }
          }}
        >
          <Form.Item label="默认 Provider" name="provider" rules={[{ required: true }]}>
            <Select
              options={[
                { label: 'noop（占位 / 演示）', value: 'noop' },
                { label: 'remove.bg 去背景', value: 'removebg' },
                { label: 'OpenAI Image', value: 'openai_image' },
                { label: 'ComfyUI（预留）', value: 'comfyui', disabled: true },
              ]}
            />
          </Form.Item>
          <Form.Item
            label="remove.bg Base URL"
            name="removebg_base_url"
            extra="默认 https://api.remove.bg/v1.0；一般无需修改"
          >
            <Input placeholder="https://api.remove.bg/v1.0" />
          </Form.Item>
          <Form.Item
            label="remove.bg API Key"
            name="removebg_api_key"
            extra="密文存储；列表与表单中使用脱敏展示"
          >
            <Input.Password placeholder="可选，真实接入时填写" autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            label="OpenAI Images Base URL"
            name="openai_image_base_url"
            extra="留空时后端默认 https://api.openai.com/v1；兼容自建代理"
          >
            <Input placeholder="https://api.openai.com/v1" />
          </Form.Item>
          <Form.Item
            label="OpenAI Image API Key"
            name="openai_image_api_key"
            extra="密文存储并脱敏；首版不推荐复用全局 AI api_key（后端侧亦未默认回退）"
          >
            <Input.Password placeholder="sk-…" autoComplete="new-password" />
          </Form.Item>
          <Form.Item label="OpenAI Image 模型" name="openai_image_model">
            <Input placeholder="gpt-image-1 或 dall-e-3 等" />
          </Form.Item>
          <Form.Item label="OpenAI Image 尺寸" name="openai_image_size">
            <Input placeholder="例如 1024x1024" />
          </Form.Item>
          <Form.Item
            label="OpenAI Image 质量档位"
            name="openai_image_quality"
            extra="gpt-image：low/medium/high；legacy 仍可填 standard→映射为中等"
          >
            <Input placeholder="standard / hd / medium…" />
          </Form.Item>
          <Form.Item
            label="OpenAI Image 背景（可选）"
            name="openai_image_background"
            extra="部分模型接受 transparent | opaque | auto"
          >
            <Input placeholder="留空则由模型默认" />
          </Form.Item>

          <Form.Item label="ComfyUI Base URL（预留）" name="comfyui_base_url">
            <Input placeholder="http://127.0.0.1:8188" />
          </Form.Item>
          <Form.Item label="ComfyUI Workflow JSON（预留）" name="comfyui_workflow_json">
            <Input.TextArea rows={6} placeholder="{}" style={{ fontFamily: 'monospace' }} />
          </Form.Item>
          <Form.Item label="超时（秒）" name="timeout_sec" rules={[{ required: true }]}>
            <InputNumber min={5} max={600} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading}>
              保存
            </Button>
          </Form.Item>
        </Form>
      </ProCard>
    </PageContainer>
  );
}
