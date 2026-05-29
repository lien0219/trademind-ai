import { ModalForm, ProFormCheckbox, ProFormRadio, ProFormSelect } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Alert, Collapse, Form, Image, Typography, message } from 'antd';
import { useEffect, useMemo } from 'react';
import { useImageProviders } from '@/hooks/useImageProviders';
import type { ProductImageRow } from '@/services/products';
import {
  buildTranslateImageTextInput,
  createImageTask,
  TRANSLATE_IMAGE_TEXT_AI_SETTINGS_HINT,
  TRANSLATE_IMAGE_TEXT_LAYOUT_MODE_OPTIONS,
  TRANSLATE_IMAGE_TEXT_RENDER_MODE_OPTIONS,
  TRANSLATE_IMAGE_TEXT_SOURCE_LANG_OPTIONS,
  TRANSLATE_IMAGE_TEXT_TARGET_LANG_OPTIONS,
  type TranslateImageTextLayoutMode,
  type TranslateRenderMode,
} from '@/services/imageTasks';

export function TranslateImageTextAiSettingsHint() {
  return (
    <Alert
      type="info"
      showIcon
      style={{ marginBottom: 16 }}
      message="识别与翻译使用 AI 设置"
      description={
        <>
          {TRANSLATE_IMAGE_TEXT_AI_SETTINGS_HINT}{' '}
          <Typography.Link onClick={() => history.push('/settings/ai')}>前往 AI 设置</Typography.Link>
        </>
      }
    />
  );
}

export type TranslateImageTextPrefill = {
  productId?: string;
  sourceImageId?: string;
  sourceImageUrl?: string;
  provider?: string;
  sourceLanguage?: string;
  targetLanguage?: string;
};

type FormValues = {
  sourceLanguage: string;
  targetLanguage: string;
  layoutMode: TranslateImageTextLayoutMode;
  autoSaveToProductImages: boolean;
  outputAsDetail: boolean;
  autoSetAsMain: boolean;
  // Advanced
  provider?: string;
  ocrProvider?: string;
  renderMode: TranslateRenderMode;
  eraseMode?: string;
  advancedJson?: string;
};

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess?: () => void;
  prefill?: TranslateImageTextPrefill;
  fixedProductId?: string;
  sourceImage?: ProductImageRow;
};

export function TranslateImageTextModal({
  open,
  onOpenChange,
  onSuccess,
  prefill,
  fixedProductId,
  sourceImage,
}: Props) {
  const [form] = Form.useForm<FormValues>();
  const { optionsForTask } = useImageProviders();
  const renderMode = Form.useWatch('renderMode', form) ?? 'hybrid';

  const productId = (fixedProductId || prefill?.productId || '').trim();
  const sourceImageId = (prefill?.sourceImageId || sourceImage?.id || '').trim();
  const sourceImageUrl = (
    prefill?.sourceImageUrl ||
    sourceImage?.publicUrl ||
    sourceImage?.originUrl ||
    ''
  ).trim();

  const providerOptions = useMemo(() => optionsForTask('translate_image_text'), [optionsForTask]);

  useEffect(() => {
    if (!open) return;
    form.setFieldsValue({
      sourceLanguage: prefill?.sourceLanguage ?? 'auto',
      targetLanguage: prefill?.targetLanguage ?? 'en',
      layoutMode: 'auto',
      autoSaveToProductImages: true,
      outputAsDetail: true,
      autoSetAsMain: false,
      renderMode: 'hybrid',
      provider: prefill?.provider ?? '',
      ocrProvider: '',
      eraseMode: '',
      advancedJson: '',
    });
  }, [open, form, prefill]);

  return (
    <ModalForm<FormValues>
      form={form}
      title="图片文字翻译"
      open={open}
      onOpenChange={onOpenChange}
      width={560}
      modalProps={{ destroyOnHidden: true }}
      submitter={{ searchConfig: { submitText: '开始翻译图片' } }}
      onFinish={async (values) => {
        if (!sourceImageId && !sourceImageUrl) {
          message.error('请选择要翻译的商品图片');
          return false;
        }
        const input = buildTranslateImageTextInput({
          sourceLanguage: values.sourceLanguage,
          targetLanguage: values.targetLanguage,
          renderMode: values.renderMode,
          layoutMode: values.layoutMode,
          autoWrap: true,
          autoFontSize: true,
          allowTextSimplify: true,
          keepProductUnchanged: true,
          autoSaveToProductImages: values.autoSaveToProductImages,
          outputAsDetail: values.outputAsDetail,
          autoSetAsMain: values.autoSetAsMain,
          removeOriginalText: true,
          preserveLayout: values.layoutMode !== 'readable',
          ocrProvider: values.ocrProvider || undefined,
          eraseMode: values.eraseMode || undefined,
          advancedJson: values.advancedJson || undefined,
        });
        try {
          const task = await createImageTask({
            taskType: 'translate_image_text',
            provider: values.renderMode === 'ai_edit' ? values.provider?.trim() || undefined : undefined,
            productId: productId || undefined,
            sourceImageId: sourceImageId || undefined,
            sourceImageUrl: sourceImageUrl || undefined,
            input,
          });
          if (task.status === 'pending' || task.status === 'running') {
            message.success('图片文字翻译任务已提交，正在后台处理');
          } else if (task.status === 'success' || task.status === 'success_with_warnings') {
            message.success(
              task.status === 'success_with_warnings'
                ? '翻译完成（存在警告，请人工检查）'
                : '翻译已完成',
            );
          } else if (task.status === 'failed') {
            message.warning(task.errorMessage || '任务失败');
          } else {
            message.success('已创建');
          }
          onSuccess?.();
          return true;
        } catch (e: unknown) {
          message.error((e as Error)?.message || '提交失败');
          return false;
        }
      }}
    >
      <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
        识别图片中的文字，翻译成目标语言，并通过程序排版将译文绘制到新图片上。原图不会被覆盖。
      </Typography.Paragraph>

      <TranslateImageTextAiSettingsHint />

      {sourceImageUrl ? (
        <div style={{ marginBottom: 16 }}>
          <Typography.Text type="secondary">待翻译图片</Typography.Text>
          <div style={{ marginTop: 8 }}>
            <Image src={sourceImageUrl} width={160} style={{ objectFit: 'contain', borderRadius: 6 }} />
          </div>
        </div>
      ) : null}

      <ProFormSelect
        name="sourceLanguage"
        label="源语言"
        options={TRANSLATE_IMAGE_TEXT_SOURCE_LANG_OPTIONS}
        rules={[{ required: true, message: '请选择源语言' }]}
      />
      <ProFormSelect
        name="targetLanguage"
        label="目标语言"
        options={TRANSLATE_IMAGE_TEXT_TARGET_LANG_OPTIONS}
        rules={[{ required: true, message: '请选择目标语言' }]}
      />

      <ProFormRadio.Group
        name="layoutMode"
        label="排版方式"
        options={TRANSLATE_IMAGE_TEXT_LAYOUT_MODE_OPTIONS}
        rules={[{ required: true, message: '请选择排版方式' }]}
      />

      <Typography.Text strong style={{ display: 'block', marginBottom: 8 }}>
        处理选项
      </Typography.Text>
      <ProFormCheckbox name="autoSaveToProductImages">自动保存到商品图片库</ProFormCheckbox>
      <ProFormCheckbox name="outputAsDetail">处理后设为详情图</ProFormCheckbox>
      <ProFormCheckbox name="autoSetAsMain">处理后设为主图</ProFormCheckbox>

      <Collapse
        ghost
        items={[
          {
            key: 'advanced',
            label: '高级设置',
            children: (
              <>
                <ProFormRadio.Group
                  name="renderMode"
                  label="覆盖渲染方式"
                  options={TRANSLATE_IMAGE_TEXT_RENDER_MODE_OPTIONS}
                />
                <Form.Item
                  noStyle
                  shouldUpdate={(prevValues, currentValues) => prevValues.renderMode !== currentValues.renderMode}
                >
                  {({ getFieldValue }) => {
                    const mode = getFieldValue('renderMode');
                    if (mode !== 'ai_edit') return null;
                    return (
                      <ProFormSelect
                        name="provider"
                        label="覆盖图片 AI 服务"
                        options={providerOptions}
                        extra="仅在 AI 编辑实验模式下生效"
                      />
                    );
                  }}
                </Form.Item>
                <ProFormSelect
                  name="ocrProvider"
                  label="覆盖 OCR 服务"
                  options={[
                    { label: '默认（读取设置）', value: '' },
                    { label: 'PaddleOCR', value: 'paddleocr' },
                    { label: 'AI 视觉模型', value: 'ai_vision' },
                  ]}
                />
                <ProFormSelect
                  name="eraseMode"
                  label="覆盖擦除方式"
                  options={[
                    { label: '默认（读取设置）', value: '' },
                    { label: '自动', value: 'auto' },
                    { label: '精细擦字（推荐）', value: 'precise_mask' },
                    { label: '背景采样', value: 'background_sample' },
                    { label: '模糊填充', value: 'blur_fill' },
                    { label: 'OpenCV 修复', value: 'opencv_inpaint' },
                    { label: 'AI 局部擦除', value: 'ai_inpaint' },
                  ]}
                />
              </>
            ),
          },
        ]}
      />
    </ModalForm>
  );
}
