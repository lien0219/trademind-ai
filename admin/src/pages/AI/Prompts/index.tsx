import type { ActionType, ProColumns } from '@ant-design/pro-components';
import {
  ModalForm,
  PageContainer,
  ProFormDigit,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { Button, Popconfirm, Tag, message } from 'antd';
import dayjs from 'dayjs';
import { useRef, useState } from 'react';
import {
  createAIPrompt,
  deleteAIPrompt,
  disableAIPrompt,
  enableAIPrompt,
  fetchAIPrompts,
  updateAIPrompt,
  type AIPromptRow,
} from '@/services/aiPrompts';

function schemaToString(v: unknown): string {
  if (v == null || v === '') return '';
  if (typeof v === 'string') return v;
  try {
    return JSON.stringify(v, null, 2);
  } catch {
    return '';
  }
}

function parseSchemaField(raw: string | undefined): unknown | undefined {
  const s = raw?.trim();
  if (!s) return undefined;
  try {
    return JSON.parse(s) as unknown;
  } catch {
    throw new Error('输出格式说明需为合法 JSON');
  }
}

export default function AIPromptsPage() {
  const actionRef = useRef<ActionType>();
  const [createOpen, setCreateOpen] = useState(false);
  const [editRow, setEditRow] = useState<AIPromptRow | null>(null);

  const columns: ProColumns<AIPromptRow>[] = [
    { title: '模板编号', dataIndex: 'code', width: 180, ellipsis: true },
    { title: '名称', dataIndex: 'name', width: 160, ellipsis: true },
    { title: '使用场景', dataIndex: 'scene', width: 120, search: false },
    { title: 'AI 服务商', dataIndex: 'provider', width: 140, search: false },
    { title: '模型', dataIndex: 'model', width: 140, ellipsis: true, search: false },
    {
      title: '启用',
      dataIndex: 'enabled',
      width: 88,
      search: false,
      render: (_, row) => (row.enabled ? <Tag color="green">是</Tag> : <Tag>否</Tag>),
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      width: 176,
      search: false,
      render: (_, row) => dayjs(row.updatedAt).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 220,
      render: (_, row) => [
        <Button key="edit" type="link" onClick={() => setEditRow(row)}>
          编辑
        </Button>,
        row.enabled ? (
          <Popconfirm key="dis" title="禁用该技能模板？" onConfirm={async () => {
            try {
              await disableAIPrompt(row.id);
              message.success('已禁用');
              actionRef.current?.reload();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '失败');
            }
          }}>
            <Button type="link">禁用</Button>
          </Popconfirm>
        ) : (
          <Button
            key="en"
            type="link"
            onClick={async () => {
              try {
                await enableAIPrompt(row.id);
                message.success('已启用');
                actionRef.current?.reload();
              } catch (e: unknown) {
                message.error((e as Error)?.message || '失败');
              }
            }}
          >
            启用
          </Button>
        ),
        <Popconfirm
          key="del"
          title="确定删除？"
          onConfirm={async () => {
            try {
              await deleteAIPrompt(row.id);
              message.success('已删除');
              actionRef.current?.reload();
            } catch (e: unknown) {
              message.error((e as Error)?.message || '失败');
            }
          }}
        >
          <Button type="link" danger>
            删除
          </Button>
        </Popconfirm>,
      ],
    },
  ];

  return (
    <PageContainer
      title="AI 技能模板"
      subTitle="配置标题优化、描述生成、客服建议等场景的提示词；一般使用系统内置模板即可。"
    >
      <ProTable<AIPromptRow>
        headerTitle={false}
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={false}
        toolBarRender={() => [
          <Button key="add" type="primary" onClick={() => setCreateOpen(true)}>
            新建
          </Button>,
        ]}
        request={async () => {
          const { list } = await fetchAIPrompts();
          return { data: list, success: true };
        }}
      />

      <ModalForm
        title="新建技能模板"
        open={createOpen}
        onOpenChange={setCreateOpen}
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (values) => {
          try {
            const outputSchema = parseSchemaField(values.outputSchemaStr as string | undefined);
            await createAIPrompt({
              code: String(values.code ?? ''),
              name: String(values.name ?? ''),
              scene: values.scene ? String(values.scene) : undefined,
              provider: values.provider ? String(values.provider) : undefined,
              model: values.model ? String(values.model) : undefined,
              systemPrompt: values.systemPrompt ? String(values.systemPrompt) : undefined,
              userPrompt: values.userPrompt ? String(values.userPrompt) : undefined,
              outputSchema,
              temperature: values.temperature != null ? Number(values.temperature) : undefined,
              maxTokens: values.maxTokens != null ? Number(values.maxTokens) : undefined,
              enabled: values.enabled != null ? Boolean(values.enabled) : true,
            });
            message.success('已创建');
            actionRef.current?.reload();
            return true;
          } catch (e: unknown) {
            message.error((e as Error)?.message || '失败');
            return false;
          }
        }}
      >
        <ProFormText name="code" label="模板编号" rules={[{ required: true }]} extra="英文标识，如 product_title_optimize" />
        <ProFormText name="name" label="名称" rules={[{ required: true }]} />
        <ProFormText name="scene" label="使用场景" />
        <ProFormText name="provider" label="指定 AI 服务商（可选）" />
        <ProFormText name="model" label="指定模型（可选）" />
        <ProFormDigit name="temperature" label="随机度" fieldProps={{ step: 0.1 }} initialValue={0.7} />
        <ProFormDigit name="maxTokens" label="最大输出长度" initialValue={512} />
        <ProFormSwitch name="enabled" label="启用" initialValue={true} />
        <ProFormTextArea name="systemPrompt" label="系统提示词" fieldProps={{ rows: 6 }} />
        <ProFormTextArea name="userPrompt" label="用户提示词" fieldProps={{ rows: 6 }} rules={[{ required: true }]} />
        <ProFormTextArea
          name="outputSchemaStr"
          label="输出格式说明（高级，JSON）"
          fieldProps={{ rows: 4 }}
          extra="仅高级用户需要填写，用于约束 AI 返回结构"
        />
      </ModalForm>

      <ModalForm<Record<string, unknown>>
        title={`编辑技能模板 — ${editRow?.code ?? ''}`}
        open={!!editRow}
        onOpenChange={(v) => {
          if (!v) setEditRow(null);
        }}
        key={editRow?.id}
        initialValues={
          editRow
            ? {
                code: editRow.code,
                name: editRow.name,
                scene: editRow.scene,
                provider: editRow.provider,
                model: editRow.model,
                systemPrompt: editRow.systemPrompt,
                userPrompt: editRow.userPrompt,
                temperature: editRow.temperature,
                maxTokens: editRow.maxTokens,
                enabled: editRow.enabled,
                outputSchemaStr: schemaToString(editRow.outputSchema),
              }
            : undefined
        }
        modalProps={{ destroyOnHidden: true }}
        onFinish={async (values) => {
          if (!editRow) return false;
          try {
            const body: Record<string, unknown> = {
              name: values.name,
              scene: values.scene,
              provider: values.provider,
              model: values.model,
              systemPrompt: values.systemPrompt,
              userPrompt: values.userPrompt,
              temperature: values.temperature,
              maxTokens: values.maxTokens,
              enabled: values.enabled,
            };
            if (typeof values.outputSchemaStr === 'string' && values.outputSchemaStr.trim()) {
              body.outputSchema = parseSchemaField(values.outputSchemaStr);
            }
            await updateAIPrompt(editRow.id, body);
            message.success('已保存');
            actionRef.current?.reload();
            setEditRow(null);
            return true;
          } catch (e: unknown) {
            message.error((e as Error)?.message || '失败');
            return false;
          }
        }}
      >
        <ProFormText name="code" label="模板编号" disabled />
        <ProFormText name="name" label="名称" rules={[{ required: true }]} />
        <ProFormText name="scene" label="使用场景" />
        <ProFormText name="provider" label="指定 AI 服务商（可选）" />
        <ProFormText name="model" label="指定模型（可选）" />
        <ProFormDigit name="temperature" label="随机度" fieldProps={{ step: 0.1 }} />
        <ProFormDigit name="maxTokens" label="最大输出长度" />
        <ProFormSwitch name="enabled" label="启用" />
        <ProFormTextArea name="systemPrompt" label="系统提示词" fieldProps={{ rows: 6 }} />
        <ProFormTextArea name="userPrompt" label="用户提示词" fieldProps={{ rows: 6 }} rules={[{ required: true }]} />
        <ProFormTextArea name="outputSchemaStr" label="输出格式说明（高级，JSON）" fieldProps={{ rows: 4 }} />
      </ModalForm>
    </PageContainer>
  );
}
