import {
  ModalForm,
  ProFormDependency,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { Collapse, Form, Image, Space, Typography, Upload, message, Checkbox } from 'antd';
import { InboxOutlined } from '@ant-design/icons';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { TranslateImageTextAiSettingsHint } from '@/components/TranslateImageTextModal';
import type { UploadRequestOption } from 'rc-upload/lib/interface';
import {
  AI_IMAGE_BACKGROUND_PRESETS,
  AI_IMAGE_FIELD,
  AI_IMAGE_SCENE_PRESETS,
  AI_IMAGE_STYLE_PRESETS,
  DEFAULT_AI_IMAGE_BACKGROUND,
  DEFAULT_AI_IMAGE_SCENE,
  DEFAULT_AI_IMAGE_STYLE,
  isProviderSelectable,
} from '@/constants/imageProviders';
import { useImageProviders } from '@/hooks/useImageProviders';
import { uploadFile } from '@/services/files';
import { fetchProductDetail, type ProductImageRow } from '@/services/products';
import {
  BEGINNER_IMAGE_TASK_TYPE_VALUES,
  IMAGE_TASK_RESULT_MODE_OPTIONS,
  IMAGE_TASK_TYPE_OPTIONS,
  TRANSLATE_IMAGE_TEXT_LAYOUT_MODE_OPTIONS,
  TRANSLATE_IMAGE_TEXT_SOURCE_LANG_OPTIONS,
  TRANSLATE_IMAGE_TEXT_TARGET_LANG_OPTIONS,
  buildResultHandlingInput,
  buildTranslateImageTextInput,
  createImageTask,
  imageTaskAllowsNoSource,
  taskTypeLabel,
} from '@/services/imageTasks';

export type CreateImageTaskPrefill = {
  taskType?: string;
  productId?: string;
  sourceImageId?: string;
  sourceImageUrl?: string;
  provider?: string;
  imageSourceMode?: 'product' | 'upload' | 'url';
  resultMode?: 'auto_save' | 'set_main' | 'set_detail' | 'result_only';
};

type FormValues = {
  taskType: string;
  provider?: string;
  productId?: string;
  imageSourceMode: 'product' | 'upload' | 'url';
  selectedProductImageId?: string;
  pastedImageUrl?: string;
  resultMode: 'auto_save' | 'set_main' | 'set_detail' | 'result_only';
  prompt?: string;
  negativePrompt?: string;
  scene?: string;
  style?: string;
  size?: string;
  background?: string;
  platform?: string;
  rbPrompt?: string;
  rbNegativePrompt?: string;
  rbBackground?: string;
  rbStyle?: string;
  rbPlatform?: string;
  rbSize?: string;
  sourceLanguage?: string;
  targetLanguage?: string;
  translateLayoutMode?: 'auto' | 'preserve' | 'readable';
  translateAutoWrap?: boolean;
  translateAutoFontSize?: boolean;
  translateAllowSimplify?: boolean;
  translateKeepProduct?: boolean;
  translateAutoSave?: boolean;
  translateOutputDetail?: boolean;
  translateSetMain?: boolean;
  sourceImageId?: string;
  sourceImageUrl?: string;
  inputJson?: string;
};

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess?: () => void;
  prefill?: CreateImageTaskPrefill;
  /** When set, product picker is hidden and images load from this product. */
  fixedProductId?: string;
  productImages?: ProductImageRow[];
  /** Show product ID field for standalone pages (e.g. AI 图片任务). */
  allowProductIdInput?: boolean;
};

function imageOptionLabel(im: ProductImageRow): string {
  const typeMap: Record<string, string> = {
    main: '主图',
    detail: '详情图',
    marketing: '营销图',
    sku: 'SKU图',
    ai_generated: 'AI图',
  };
  const typeLabel = typeMap[String(im.imageType ?? '')] ?? im.imageType ?? '图片';
  const url = (im.publicUrl || im.originUrl || '').trim();
  const short = url.length > 48 ? `${url.slice(0, 48)}…` : url;
  return `${typeLabel} · ${short || im.id.slice(0, 8)}`;
}

export function CreateImageTaskModal({
  open,
  onOpenChange,
  onSuccess,
  prefill,
  fixedProductId,
  productImages: productImagesProp,
  allowProductIdInput = false,
}: Props) {
  const [form] = Form.useForm<FormValues>();
  const { caps, optionsForTask } = useImageProviders();
  const [uploadedSource, setUploadedSource] = useState<{ id: string; url: string } | null>(null);
  const [uploading, setUploading] = useState(false);
  const [loadedImages, setLoadedImages] = useState<ProductImageRow[]>([]);
  const [loadingImages, setLoadingImages] = useState(false);
  const [showMoreTaskTypes, setShowMoreTaskTypes] = useState(false);
  const watchedProductId = Form.useWatch('productId', form);
  const effectiveProductId = (fixedProductId || watchedProductId || prefill?.productId || '').trim();

  const productImages = useMemo(() => {
    if (productImagesProp?.length) return productImagesProp;
    return loadedImages;
  }, [productImagesProp, loadedImages]);

  const beginnerTaskOptions = useMemo(
    () =>
      BEGINNER_IMAGE_TASK_TYPE_VALUES.map((value) => ({
        label: taskTypeLabel(value),
        value,
        disabled: !caps.some((c) => isProviderSelectable(c) && c.supportedTasks.includes(value)),
      })),
    [caps],
  );

  const advancedTaskOptions = useMemo(
    () =>
      IMAGE_TASK_TYPE_OPTIONS.filter((t) => !BEGINNER_IMAGE_TASK_TYPE_VALUES.includes(t.value as (typeof BEGINNER_IMAGE_TASK_TYPE_VALUES)[number]))
        .map((t) => ({
          label: t.label,
          value: t.value,
          disabled: !caps.some((c) => isProviderSelectable(c) && c.supportedTasks.includes(t.value)),
        })),
    [caps],
  );

  const applyPrefill = useCallback(() => {
    const p = prefill ?? {};
    const productId = fixedProductId || p.productId || '';
    form.setFieldsValue({
      taskType: p.taskType || 'remove_watermark',
      provider: p.provider ?? '',
      productId,
      imageSourceMode: p.imageSourceMode ?? (p.sourceImageId ? 'product' : productId ? 'product' : 'url'),
      selectedProductImageId: p.sourceImageId,
      pastedImageUrl: p.imageSourceMode === 'url' ? p.sourceImageUrl : '',
      resultMode: p.resultMode ?? (productId ? 'auto_save' : 'result_only'),
      sourceImageId: p.sourceImageId ?? '',
      sourceImageUrl: p.sourceImageUrl ?? '',
      inputJson: '{}',
      prompt: '',
      negativePrompt: '',
      scene: DEFAULT_AI_IMAGE_SCENE,
      style: DEFAULT_AI_IMAGE_STYLE,
      size: '1024x1024',
      background: DEFAULT_AI_IMAGE_BACKGROUND,
      platform: 'TikTok Shop',
      rbPrompt: '',
      rbNegativePrompt: '',
      rbBackground: DEFAULT_AI_IMAGE_BACKGROUND,
      rbStyle: DEFAULT_AI_IMAGE_STYLE,
      rbPlatform: 'TikTok Shop',
      rbSize: '1024x1024',
    });
    setShowMoreTaskTypes(
      Boolean(
        p.taskType &&
          !BEGINNER_IMAGE_TASK_TYPE_VALUES.includes(p.taskType as (typeof BEGINNER_IMAGE_TASK_TYPE_VALUES)[number]),
      ),
    );
    if (p.sourceImageUrl && p.imageSourceMode === 'upload') {
      setUploadedSource(p.sourceImageId && p.sourceImageUrl ? { id: p.sourceImageId, url: p.sourceImageUrl } : null);
    } else {
      setUploadedSource(null);
    }
  }, [form, fixedProductId, prefill]);

  useEffect(() => {
    if (!open) return;
    applyPrefill();
  }, [open, applyPrefill]);

  useEffect(() => {
    if (!open || productImagesProp?.length) return;
    const pid = effectiveProductId;
    if (!pid) {
      setLoadedImages([]);
      return;
    }
    let cancelled = false;
    setLoadingImages(true);
    void (async () => {
      try {
        const detail = await fetchProductDetail(pid);
        if (!cancelled) setLoadedImages(detail.images ?? []);
      } catch {
        if (!cancelled) setLoadedImages([]);
      } finally {
        if (!cancelled) setLoadingImages(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, effectiveProductId, productImagesProp]);

  const handleUpload = async (options: UploadRequestOption) => {
    const file = options.file as File;
    setUploading(true);
    try {
      const up = await uploadFile(file);
      setUploadedSource({ id: up.id, url: up.url });
      form.setFieldsValue({
        imageSourceMode: 'upload',
        selectedProductImageId: undefined,
        pastedImageUrl: '',
        sourceImageId: up.id,
        sourceImageUrl: up.url,
      });
      options.onSuccess?.(up);
      message.success('图片已上传');
    } catch (e: unknown) {
      options.onError?.(e as Error);
      message.error((e as Error)?.message || '上传失败');
    } finally {
      setUploading(false);
    }
  };

  const resolveSource = (values: FormValues) => {
    const advId = (values.sourceImageId ?? '').trim();
    const advUrl = (values.sourceImageUrl ?? '').trim();
    if (advId || advUrl) {
      return { sourceImageId: advId || undefined, sourceImageUrl: advUrl || undefined };
    }
    if (values.imageSourceMode === 'product') {
      const rowId = (values.selectedProductImageId ?? '').trim();
      const row = productImages.find((im) => im.id === rowId);
      const url = (row?.publicUrl || row?.originUrl || '').trim();
      return {
        sourceImageId: rowId || undefined,
        sourceImageUrl: url || undefined,
      };
    }
    if (values.imageSourceMode === 'upload' && uploadedSource) {
      return { sourceImageId: uploadedSource.id, sourceImageUrl: uploadedSource.url };
    }
    const pasted = (values.pastedImageUrl ?? '').trim();
    return { sourceImageUrl: pasted || undefined };
  };

  return (
    <ModalForm<FormValues>
      form={form}
      title="新建图片任务"
      open={open}
      onOpenChange={(v) => {
        if (!v) {
          setUploadedSource(null);
          setShowMoreTaskTypes(false);
        }
        onOpenChange(v);
      }}
      initialValues={{
        taskType: 'remove_watermark',
        provider: '',
        imageSourceMode: 'url',
        resultMode: 'result_only',
        inputJson: '{}',
      }}
      width={640}
      modalProps={{ destroyOnHidden: true }}
      submitter={{ searchConfig: { submitText: '创建任务' } }}
      onFinish={async (values) => {
        const tt = (values.taskType ?? '').trim();
        const productId = (fixedProductId || values.productId || '').trim();
        const { sourceImageId, sourceImageUrl } = resolveSource(values);

        if (!imageTaskAllowsNoSource(tt) && !sourceImageId && !sourceImageUrl) {
          message.error('请选择商品图片、上传图片或填写图片链接');
          return false;
        }
        if (tt === 'select_best_main' && !productId) {
          message.error('自动选最佳主图需要关联商品，请填写商品 ID');
          return false;
        }

        let extra: Record<string, unknown> = {};
        const rawJson = (values.inputJson ?? '').trim();
        if (rawJson) {
          try {
            extra = JSON.parse(rawJson) as Record<string, unknown>;
          } catch {
            message.error('高级参数需为合法 JSON');
            return false;
          }
        }

        const input: Record<string, unknown> = {
          ...extra,
        };
        if (tt !== 'translate_image_text') {
          Object.assign(input, buildResultHandlingInput(values.resultMode));
        }

        if (tt === 'translate_image_text') {
          Object.assign(
            input,
            buildTranslateImageTextInput({
              sourceLanguage: values.sourceLanguage,
              targetLanguage: values.targetLanguage,
              layoutMode: values.translateLayoutMode ?? 'auto',
              autoWrap: values.translateAutoWrap,
              autoFontSize: values.translateAutoFontSize,
              allowTextSimplify: values.translateAllowSimplify,
              keepProductUnchanged: values.translateKeepProduct,
              autoSaveToProductImages: values.translateAutoSave,
              outputAsDetail: values.translateOutputDetail,
              autoSetAsMain: values.translateSetMain,
              removeOriginalText: true,
              preserveLayout: values.translateLayoutMode !== 'readable',
            }),
          );
        }
        if (tt === 'generate_scene') {
          Object.assign(input, {
            prompt: (values.prompt ?? '').trim(),
            negativePrompt: (values.negativePrompt ?? '').trim(),
            scene: (values.scene ?? '').trim(),
            style: (values.style ?? '').trim(),
            size: (values.size ?? '').trim(),
            background: (values.background ?? '').trim(),
            platform: (values.platform ?? '').trim(),
          });
        }
        if (tt === 'replace_background') {
          Object.assign(input, {
            prompt: (values.rbPrompt ?? '').trim(),
            negativePrompt: (values.rbNegativePrompt ?? '').trim(),
            background: (values.rbBackground ?? '').trim(),
            style: (values.rbStyle ?? '').trim(),
            platform: (values.rbPlatform ?? '').trim(),
            size: (values.rbSize ?? '').trim(),
          });
        }

        try {
          const task = await createImageTask({
            taskType: tt,
            provider: values.provider?.trim() || undefined,
            productId: productId || undefined,
            sourceImageId,
            sourceImageUrl,
            input,
          });
          if (task.status === 'pending' || task.status === 'running') {
            message.success('图片任务已提交，正在后台处理');
          } else if (task.status === 'success' || task.status === 'success_with_warnings') {
            message.success(
              task.status === 'success_with_warnings' ? '任务完成（存在警告，请人工检查）' : '任务已完成',
            );
          } else if (task.status === 'failed') {
            message.warning(task.errorMessage || '任务失败');
          } else {
            message.success('已创建');
          }
          onSuccess?.();
          return true;
        } catch (e: unknown) {
          message.error((e as Error)?.message || '创建失败');
          return false;
        }
      }}
    >
      <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
        处理结果会自动保存到「设置 → 存储设置」当前启用的存储位置。普通用户只需选择任务类型和图片即可。
      </Typography.Paragraph>

      {showMoreTaskTypes ? (
        <ProFormSelect
          name="taskType"
          label="任务类型"
          options={[...beginnerTaskOptions, ...advancedTaskOptions]}
          rules={[{ required: true, message: '请选择任务类型' }]}
          fieldProps={{
            showSearch: true,
            optionFilterProp: 'label',
            onChange: (v: string) => {
              if (v === 'remove_background') {
                form.setFieldsValue({ provider: 'removebg' });
              }
            },
          }}
        />
      ) : (
        <>
          <ProFormSelect
            name="taskType"
            label="任务类型"
            options={beginnerTaskOptions}
            rules={[{ required: true, message: '请选择任务类型' }]}
            fieldProps={{
              onChange: (v: string) => {
                if (v === 'remove_background') {
                  form.setFieldsValue({ provider: 'removebg' });
                }
              },
            }}
          />
          <Typography.Link
            onClick={() => setShowMoreTaskTypes(true)}
            style={{ display: 'block', marginBottom: 16, marginTop: -8 }}
          >
            显示更多任务类型（换背景、场景图、缩放等）
          </Typography.Link>
        </>
      )}

      {allowProductIdInput && !fixedProductId ? (
        <ProFormText
          name="productId"
          label="关联商品（可选）"
          placeholder="填写商品 ID 后可从商品图库选择"
          extra="关联商品后，处理结果可自动写入商品图片库"
          fieldProps={{
            onBlur: () => {
              const pid = (form.getFieldValue('productId') ?? '').trim();
              if (!pid) {
                setLoadedImages([]);
                return;
              }
              void (async () => {
                setLoadingImages(true);
                try {
                  const detail = await fetchProductDetail(pid);
                  setLoadedImages(detail.images ?? []);
                } catch {
                  setLoadedImages([]);
                  message.warning('无法加载商品图片，请检查商品 ID');
                } finally {
                  setLoadingImages(false);
                }
              })();
            },
          }}
        />
      ) : fixedProductId ? (
        <ProFormText name="productId" hidden initialValue={fixedProductId} />
      ) : null}

      <ProFormDependency name={['taskType']}>
        {({ taskType }: { taskType?: string }) =>
          imageTaskAllowsNoSource(taskType ?? '') ? (
            <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
              此任务将评估当前商品的全部图片并推荐最佳主图，无需单独选择源图。
            </Typography.Text>
          ) : (
            <>
              <ProFormRadio.Group
                name="imageSourceMode"
                label="选择图片"
                options={[
                  {
                    label: '从商品图片中选择',
                    value: 'product',
                    disabled: !effectiveProductId,
                  },
                  { label: '上传新图片', value: 'upload' },
                  { label: '粘贴图片 URL', value: 'url' },
                ]}
                rules={[{ required: true }]}
              />
              <ProFormDependency name={['imageSourceMode']}>
                {({ imageSourceMode }: { imageSourceMode?: string }) => {
                  if (imageSourceMode === 'product') {
                    return (
                      <ProFormSelect
                        name="selectedProductImageId"
                        label="商品图片"
                        placeholder={loadingImages ? '加载中…' : '选择一张商品图片'}
                        options={productImages.map((im) => ({
                          label: imageOptionLabel(im),
                          value: im.id,
                        }))}
                        rules={[{ required: true, message: '请选择商品图片' }]}
                        fieldProps={{
                          loading: loadingImages,
                          notFoundContent: effectiveProductId ? '该商品暂无图片' : '请先填写商品 ID',
                        }}
                      />
                    );
                  }
                  if (imageSourceMode === 'upload') {
                    return (
                      <Form.Item label="上传图片" required>
                        <Upload.Dragger
                          accept="image/*"
                          maxCount={1}
                          showUploadList={false}
                          customRequest={(opt) => void handleUpload(opt)}
                          disabled={uploading}
                        >
                          <p className="ant-upload-drag-icon">
                            <InboxOutlined />
                          </p>
                          <p className="ant-upload-text">点击或拖拽图片到此处上传</p>
                          <p className="ant-upload-hint">上传后将使用系统存储中的图片作为源图</p>
                        </Upload.Dragger>
                        {uploadedSource ? (
                          <div style={{ marginTop: 12 }}>
                            <Image
                              src={uploadedSource.url}
                              width={120}
                              height={120}
                              style={{ objectFit: 'cover', borderRadius: 6 }}
                            />
                          </div>
                        ) : null}
                      </Form.Item>
                    );
                  }
                  return (
                    <ProFormText
                      name="pastedImageUrl"
                      label="图片链接"
                      placeholder="https://example.com/image.jpg"
                      extra="请填写公网可访问的 HTTPS 图片地址"
                      rules={[{ required: true, message: '请填写图片链接' }]}
                    />
                  );
                }}
              </ProFormDependency>
            </>
          )
        }
      </ProFormDependency>

      <ProFormDependency name={['taskType']}>
        {({ taskType }: { taskType?: string }) => (
          <ProFormSelect
            name="provider"
            label="图片处理服务"
            options={optionsForTask(taskType ?? '')}
            extra="默认使用「设置 → 图片 AI」中的默认 Provider；去背景推荐 remove.bg"
          />
        )}
      </ProFormDependency>

      <ProFormDependency name={['taskType', 'productId']}>
        {({ taskType, productId }: { taskType?: string; productId?: string }) => {
          const pid = (fixedProductId || productId || '').trim();
          if (!pid || taskType === 'score_image' || taskType === 'select_best_main' || taskType === 'translate_image_text') return null;
          return (
            <>
              <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
                {IMAGE_TASK_RESULT_MODE_OPTIONS.find((o) => o.value === 'auto_save')?.description}
              </Typography.Text>
              <ProFormRadio.Group
              name="resultMode"
              label="处理结果"
              options={IMAGE_TASK_RESULT_MODE_OPTIONS.map((o) => ({
                label: o.label,
                value: o.value,
              }))}
              rules={[{ required: true }]}
            />
            </>
          );
        }}
      </ProFormDependency>

      <ProFormDependency name={['taskType']}>
        {({ taskType }: { taskType?: string }) =>
          taskType === 'generate_scene' ? (
            <>
              <ProFormTextArea
                name="prompt"
                label={AI_IMAGE_FIELD.prompt.label}
                extra={AI_IMAGE_FIELD.prompt.extra}
                fieldProps={{ rows: 3, placeholder: AI_IMAGE_FIELD.prompt.placeholder }}
              />
              <ProFormTextArea
                name="negativePrompt"
                label={AI_IMAGE_FIELD.negativePrompt.label}
                fieldProps={{ rows: 2, placeholder: AI_IMAGE_FIELD.negativePrompt.placeholder }}
              />
              <Space wrap>
                <ProFormSelect name="scene" label={AI_IMAGE_FIELD.scene.label} options={AI_IMAGE_SCENE_PRESETS} />
                <ProFormSelect name="style" label={AI_IMAGE_FIELD.style.label} options={AI_IMAGE_STYLE_PRESETS} />
                <ProFormSelect name="background" label={AI_IMAGE_FIELD.background.label} options={AI_IMAGE_BACKGROUND_PRESETS} />
              </Space>
              <ProFormText name="size" label="尺寸（可选）" placeholder="1024x1024" />
            </>
          ) : null
        }
      </ProFormDependency>

      <ProFormDependency name={['taskType']}>
        {({ taskType }: { taskType?: string }) =>
          taskType === 'translate_image_text' ? (
            <>
              <TranslateImageTextAiSettingsHint />
              <ProFormSelect
                name="sourceLanguage"
                label="源语言"
                initialValue="auto"
                options={TRANSLATE_IMAGE_TEXT_SOURCE_LANG_OPTIONS}
                rules={[{ required: true, message: '请选择源语言' }]}
              />
              <ProFormSelect
                name="targetLanguage"
                label="目标语言"
                initialValue="en"
                options={TRANSLATE_IMAGE_TEXT_TARGET_LANG_OPTIONS}
                rules={[{ required: true, message: '请选择目标语言' }]}
              />
              <ProFormRadio.Group
                name="translateLayoutMode"
                label="排版方式"
                initialValue="auto"
                options={TRANSLATE_IMAGE_TEXT_LAYOUT_MODE_OPTIONS}
              />
              <Typography.Text strong style={{ display: 'block', marginBottom: 8 }}>
                处理选项
              </Typography.Text>
              <Form.Item name="translateAutoWrap" valuePropName="checked" initialValue>
                <Checkbox>自动换行</Checkbox>
              </Form.Item>
              <Form.Item name="translateAutoFontSize" valuePropName="checked" initialValue>
                <Checkbox>自动调整字号</Checkbox>
              </Form.Item>
              <Form.Item name="translateAllowSimplify" valuePropName="checked" initialValue>
                <Checkbox>文字太长时自动精简</Checkbox>
              </Form.Item>
              <Form.Item name="translateKeepProduct" valuePropName="checked" initialValue>
                <Checkbox>尽量不改变商品主体</Checkbox>
              </Form.Item>
              <Form.Item name="translateAutoSave" valuePropName="checked" initialValue>
                <Checkbox>自动保存到商品图片库</Checkbox>
              </Form.Item>
              <Form.Item name="translateOutputDetail" valuePropName="checked" initialValue>
                <Checkbox>处理后设为详情图</Checkbox>
              </Form.Item>
              <Form.Item name="translateSetMain" valuePropName="checked" initialValue={false}>
                <Checkbox>处理后设为主图</Checkbox>
              </Form.Item>
            </>
          ) : null
        }
      </ProFormDependency>

      <ProFormDependency name={['taskType']}>
        {({ taskType }: { taskType?: string }) =>
          taskType === 'replace_background' ? (
            <>
              <ProFormTextArea name="rbPrompt" label={AI_IMAGE_FIELD.prompt.label} fieldProps={{ rows: 3 }} />
              <ProFormTextArea name="rbNegativePrompt" label={AI_IMAGE_FIELD.negativePrompt.label} fieldProps={{ rows: 2 }} />
              <Space wrap>
                <ProFormSelect name="rbBackground" label={AI_IMAGE_FIELD.background.label} options={AI_IMAGE_BACKGROUND_PRESETS} />
                <ProFormSelect name="rbStyle" label={AI_IMAGE_FIELD.style.label} options={AI_IMAGE_STYLE_PRESETS} />
              </Space>
            </>
          ) : null
        }
      </ProFormDependency>

      <Collapse
        ghost
        items={[
          {
            key: 'advanced',
            label: '高级设置（开发调试）',
            children: (
              <>
                <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
                  普通用户无需填写。仅用于开发调试、外部图片处理或特殊任务参数覆盖。
                </Typography.Paragraph>
                <ProFormText
                  name="sourceImageId"
                  label="商品图片 ID，高级"
                  placeholder="系统内部 UUID"
                  extra="系统内部图片 ID。普通用户无需填写，请直接选择商品图片。"
                />
                <ProFormText
                  name="sourceImageUrl"
                  label="外部图片链接，高级"
                  placeholder="https://..."
                  extra="可填写公网可访问的 HTTPS 图片地址。填写后将覆盖上方选择的图片。"
                />
                <ProFormTextArea
                  name="inputJson"
                  label="高级参数，高级"
                  fieldProps={{ rows: 4, style: { fontFamily: 'monospace' } }}
                  extra="用于覆盖默认处理参数，普通用户无需填写。"
                  initialValue="{}"
                />
              </>
            ),
          },
        ]}
      />
    </ModalForm>
  );
}
