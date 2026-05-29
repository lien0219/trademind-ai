import { ModalForm, ProFormCheckbox, ProFormRadio, ProFormSelect } from '@ant-design/pro-components';
import { Form, Image, Typography, message } from 'antd';
import { useEffect, useMemo } from 'react';
import { useImageProviders } from '@/hooks/useImageProviders';
import type { ProductImageRow } from '@/services/products';
import {
  buildTranslateImageTextInput,
  createImageTask,
  TRANSLATE_IMAGE_TEXT_LAYOUT_MODE_OPTIONS,
  TRANSLATE_IMAGE_TEXT_SOURCE_LANG_OPTIONS,
  TRANSLATE_IMAGE_TEXT_TARGET_LANG_OPTIONS,
  type TranslateImageTextLayoutMode,
} from '@/services/imageTasks';

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
  autoWrap: boolean;
  autoFontSize: boolean;
  allowTextSimplify: boolean;
  keepProductUnchanged: boolean;
  autoSaveToProductImages: boolean;
  outputAsDetail: boolean;
  autoSetAsMain: boolean;
  provider?: string;
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
      autoWrap: true,
      autoFontSize: true,
      allowTextSimplify: true,
      keepProductUnchanged: true,
      autoSaveToProductImages: true,
      outputAsDetail: true,
      autoSetAsMain: false,
      provider: prefill?.provider ?? providerOptions[0]?.value ?? '',
    });
  }, [open, form, prefill, providerOptions]);

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
          layoutMode: values.layoutMode,
          autoWrap: values.autoWrap,
          autoFontSize: values.autoFontSize,
          allowTextSimplify: values.allowTextSimplify,
          keepProductUnchanged: values.keepProductUnchanged,
          autoSaveToProductImages: values.autoSaveToProductImages,
          outputAsDetail: values.outputAsDetail,
          autoSetAsMain: values.autoSetAsMain,
          removeOriginalText: true,
          preserveLayout: values.layoutMode !== 'readable',
        });
        try {
          const task = await createImageTask({
            taskType: 'translate_image_text',
            provider: values.provider?.trim() || undefined,
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
        识别图片中的文字，翻译成目标语言，并自动排版后生成新图片。原图不会被覆盖。
      </Typography.Paragraph>

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

      {providerOptions.length > 1 ? (
        <ProFormSelect
          name="provider"
          label="图片 AI 服务"
          options={providerOptions}
          rules={[{ required: true, message: '请选择图片 AI 服务' }]}
        />
      ) : null}

      <Typography.Text strong style={{ display: 'block', marginBottom: 8 }}>
        处理选项
      </Typography.Text>
      <ProFormCheckbox name="autoWrap">自动换行</ProFormCheckbox>
      <ProFormCheckbox name="autoFontSize">自动调整字号</ProFormCheckbox>
      <ProFormCheckbox name="allowTextSimplify">文字太长时自动精简</ProFormCheckbox>
      <ProFormCheckbox name="keepProductUnchanged">尽量不改变商品主体</ProFormCheckbox>
      <ProFormCheckbox name="autoSaveToProductImages">自动保存到商品图片库</ProFormCheckbox>
      <ProFormCheckbox name="outputAsDetail">处理后设为详情图</ProFormCheckbox>
      <ProFormCheckbox name="autoSetAsMain">处理后设为主图</ProFormCheckbox>
    </ModalForm>
  );
}
