import { useLanguage } from '@/i18n/useLanguage';
import { Link } from '@umijs/renderer-react';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails, TmProTable as ProTable } from '@/components/ui';
import { formatDateTime } from '@/utils/formatTime';
import { ModalForm, ProFormDigit, ProFormSelect, ProFormSwitch, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import { Button, Popconfirm, Tag, Tooltip, Typography, message } from 'antd';
import { useEffect, useRef, useState } from 'react';
import {
  AI_PROMPT_SCENE_OPTIONS,
  AI_PROMPT_USE_SYSTEM_DEFAULT,
  AI_TEXT_PROVIDER_OPTIONS,
  aiPromptSceneLabel,
  aiTextProviderLabel,
} from '@/constants/aiPrompts';
import { fetchSettingsList } from '@/services/settings';
import { usePermission } from '@/hooks/usePermission';
import { mayReadGlobalSettings } from '@/utils/globalSettingsAccess';
import { pickGroup } from '@/utils/settingsForm';
import { tr } from '@/i18n';
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
    throw new Error(tr('输出格式说明需为合法 JSON'));
  }
}

export default function AIPromptsPage() {
  useLanguage();
  const { user } = usePermission();
  const mayReadSettings = mayReadGlobalSettings(user);
  const actionRef = useRef<ActionType>();
  const [createOpen, setCreateOpen] = useState(false);
  const [editRow, setEditRow] = useState<AIPromptRow | null>(null);
  const [systemAi, setSystemAi] = useState<{ provider: string; model: string }>({
    provider: '',
    model: '',
  });

  useEffect(() => {
    if (!mayReadSettings) return;
    fetchSettingsList()
      .then((res) => {
        const ai = pickGroup(res.items, 'ai');
        setSystemAi({
          provider: (ai.provider || 'openai_compatible').trim(),
          model: (ai.model || '').trim(),
        });
      })
      .catch(() => {
        setSystemAi({ provider: 'openai_compatible', model: '' });
      });
  }, [mayReadSettings]);

  const renderInheritedTag = (kind: 'provider' | 'model') => {
    const systemValue = kind === 'provider' ? systemAi.provider : systemAi.model;
    const systemLabel =
      kind === 'provider' ? aiTextProviderLabel(systemValue) || systemValue : systemValue;
    const tip = systemLabel
      ? tr('未单独指定时，使用「设置 → AI」默认值：{value}').replace('{value}', systemLabel)
      : tr('未单独指定时，使用「设置 → AI」中的默认配置');
    return (
      <Tooltip title={tip}>
        <Tag bordered={false} color="default">
          {tr(AI_PROMPT_USE_SYSTEM_DEFAULT)}
          {systemLabel ? ` · ${systemLabel}` : ''}
        </Tag>
      </Tooltip>
    );
  };

  const columns: ProColumns<AIPromptRow>[] = [
    { title: tr("模板编号"), dataIndex: 'code', width: 180, ellipsis: true },
    { title: tr("名称"), dataIndex: 'name', width: 160, ellipsis: true },
    {
      title: tr("使用场景"),
      dataIndex: 'scene',
      width: 120,
      search: false,
      render: (_, row) => {
        const label = tr(aiPromptSceneLabel(row.scene));
        const raw = (row.scene || '').trim();
        if (!raw || label === raw) {
          return label === '—' ? '—' : <Tag bordered={false}>{label}</Tag>;
        }
        return (
          <Tooltip title={`${tr('原始值：')}${raw}`}>
            <Tag bordered={false} color="blue">
              {label}
            </Tag>
          </Tooltip>
        );
      },
    },
    {
      title: tr("AI 服务商"),
      dataIndex: 'provider',
      width: 180,
      search: false,
      render: (_, row) => {
        const raw = (row.provider || '').trim();
        if (!raw) return renderInheritedTag('provider');
        const label = aiTextProviderLabel(raw);
        return (
          <Tooltip title={`${tr('原始值：')}${raw}`}>
            <Tag bordered={false} color="purple">
              {label}
            </Tag>
          </Tooltip>
        );
      },
    },
    {
      title: tr("模型"),
      dataIndex: 'model',
      width: 160,
      ellipsis: true,
      search: false,
      render: (_, row) => {
        const raw = (row.model || '').trim();
        if (!raw) return renderInheritedTag('model');
        return (
          <Tooltip title={`${tr('原始值：')}${raw}`}>
            <Typography.Text>{raw}</Typography.Text>
          </Tooltip>
        );
      },
    },
    {
      title: tr("启用"),
      dataIndex: 'enabled',
      width: 88,
      search: false,
      render: (_, row) => (row.enabled ? <Tag color="green">{tr('是')}</Tag> : <Tag>{tr('否')}</Tag>),
    },
    {
      title: tr("更新时间"),
      dataIndex: 'updatedAt',
      width: 176,
      search: false,
      render: (_, row) => formatDateTime(row.updatedAt),
    },
    {
      title: tr("操作"),
      valueType: 'option',
      width: 220,
      render: (_, row) => [
        <Button key="edit" type="link" onClick={() => setEditRow(row)}>
          {tr("编辑")}
        </Button>,
        row.enabled ? (
          <Popconfirm key="dis" title={tr("禁用该技能模板？")} onConfirm={async () => {
            try {
              await disableAIPrompt(row.id);
              message.success(tr("已禁用"));
              actionRef.current?.reload();
            } catch (e: unknown) {
              message.error((e as Error)?.message || tr("失败"));
            }
          }}>
            <Button type="link">{tr("禁用")}</Button>
          </Popconfirm>
        ) : (
          <Button
            key="en"
            type="link"
            onClick={async () => {
              try {
                await enableAIPrompt(row.id);
                message.success(tr("已启用"));
                actionRef.current?.reload();
              } catch (e: unknown) {
                message.error((e as Error)?.message || tr("失败"));
              }
            }}
          >
            {tr("启用")}
          </Button>
        ),
        <Popconfirm
          key="del"
          title={tr("确定删除？")}
          onConfirm={async () => {
            try {
              await deleteAIPrompt(row.id);
              message.success(tr("已删除"));
              actionRef.current?.reload();
            } catch (e: unknown) {
              message.error((e as Error)?.message || tr("失败"));
            }
          }}
        >
          <Button type="link" danger>
            {tr("删除")}
          </Button>
        </Popconfirm>,
      ],
    },
  ];

  return (
    <TmPageContainer
      title={tr("AI 技能模板")}
      subTitle={
        <>
          {tr('配置标题优化、描述生成、客服建议等场景的提示词模板；未单独指定 AI 服务商或模型时，将使用')}{' '}
          <Link to="/settings/ai">{tr('AI 设置')}</Link>{tr(' 中的默认配置。')}
        </>
      }
    >
      <ProTable<AIPromptRow>
        headerTitle={false}
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={false}
        toolBarRender={() => [
          <Button key="add" type="primary" onClick={() => setCreateOpen(true)}>
            {tr("新建")}
          </Button>,
        ]}
        request={async () => {
          const { list } = await fetchAIPrompts();
          return { data: list, success: true };
        }}
      />

      <ModalForm
        title={tr("新建技能模板")}
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
            message.success(tr("已创建"));
            actionRef.current?.reload();
            return true;
          } catch (e: unknown) {
            message.error((e as Error)?.message || tr("失败"));
            return false;
          }
        }}
      >
        <ProFormText name="code" label={tr("模板编号")} rules={[{ required: true }]} extra={tr("英文标识，如 product_title_optimize")} />
        <ProFormText name="name" label={tr("名称")} rules={[{ required: true }]} />
        <ProFormSelect
          name="scene"
          label={tr("使用场景")}
          options={AI_PROMPT_SCENE_OPTIONS.map(option => ({ ...option, label: tr(option.label) }))}
          fieldProps={{ showSearch: true, optionFilterProp: 'label' }}
        />
        <ProFormSelect
          name="provider"
          label={tr("指定 AI 服务商（可选）")}
          options={[{ value: '', label: tr(AI_PROMPT_USE_SYSTEM_DEFAULT) }, ...AI_TEXT_PROVIDER_OPTIONS]}
          extra={tr("留空则使用「设置 → AI」中的默认服务商")}
        />
        <ProFormText name="model" label={tr("指定模型（可选）")} extra={tr("留空则使用「设置 → AI」中的默认模型")} />
        <ProFormDigit name="temperature" label={tr("随机度")} fieldProps={{ step: 0.1 }} initialValue={0.7} />
        <ProFormDigit name="maxTokens" label={tr("最大输出长度")} initialValue={512} />
        <ProFormSwitch name="enabled" label={tr("启用")} initialValue={true} />
        <ProFormTextArea name="systemPrompt" label={tr("系统提示词")} fieldProps={{ rows: 6 }} />
        <ProFormTextArea name="userPrompt" label={tr("用户提示词")} fieldProps={{ rows: 6 }} rules={[{ required: true }]} />
        <TechnicalDetails label={tr("输出格式说明（高级）")}>
          <ProFormTextArea
            name="outputSchemaStr"
            label={tr("JSON 结构")}
            fieldProps={{ rows: 4, style: { fontFamily: 'monospace', fontSize: 12 } }}
            extra={tr("仅高级用户需要填写，用于约束 AI 返回结构")}
          />
        </TechnicalDetails>
      </ModalForm>

      <ModalForm<Record<string, unknown>>
        title={`${tr('编辑技能模板')} — ${editRow?.code ?? ''}`}
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
            message.success(tr("已保存"));
            actionRef.current?.reload();
            setEditRow(null);
            return true;
          } catch (e: unknown) {
            message.error((e as Error)?.message || tr("失败"));
            return false;
          }
        }}
      >
        <ProFormText name="code" label={tr("模板编号")} disabled />
        <ProFormText name="name" label={tr("名称")} rules={[{ required: true }]} />
        <ProFormSelect
          name="scene"
          label={tr("使用场景")}
          options={AI_PROMPT_SCENE_OPTIONS.map(option => ({ ...option, label: tr(option.label) }))}
          fieldProps={{ showSearch: true, optionFilterProp: 'label' }}
        />
        <ProFormSelect
          name="provider"
          label={tr("指定 AI 服务商（可选）")}
          options={[{ value: '', label: tr(AI_PROMPT_USE_SYSTEM_DEFAULT) }, ...AI_TEXT_PROVIDER_OPTIONS]}
          extra={tr("留空则使用「设置 → AI」中的默认服务商")}
        />
        <ProFormText name="model" label={tr("指定模型（可选）")} extra={tr("留空则使用「设置 → AI」中的默认模型")} />
        <ProFormDigit name="temperature" label={tr("随机度")} fieldProps={{ step: 0.1 }} />
        <ProFormDigit name="maxTokens" label={tr("最大输出长度")} />
        <ProFormSwitch name="enabled" label={tr("启用")} />
        <ProFormTextArea name="systemPrompt" label={tr("系统提示词")} fieldProps={{ rows: 6 }} />
        <ProFormTextArea name="userPrompt" label={tr("用户提示词")} fieldProps={{ rows: 6 }} rules={[{ required: true }]} />
        <TechnicalDetails label={tr("输出格式说明（高级）")}>
          <ProFormTextArea
            name="outputSchemaStr"
            label={tr("JSON 结构")}
            fieldProps={{ rows: 4, style: { fontFamily: 'monospace', fontSize: 12 } }}
          />
        </TechnicalDetails>
      </ModalForm>
    </TmPageContainer>
  );
}
