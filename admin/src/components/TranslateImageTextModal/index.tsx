import { useLanguage } from '@/i18n/useLanguage';
import { tr } from '@/i18n';
import { ModalForm, ProFormCheckbox, ProFormRadio, ProFormSelect } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import {
  Alert,
  Button,
  Collapse,
  Form,
  Image,
  Space,
  Spin,
  Tag,
  Typography,
  message,
} from 'antd';
import {
  CloudUploadOutlined,
  FileImageOutlined,
  FontColorsOutlined,
  InfoCircleOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  TranslationOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { useCallback, useEffect, useMemo, useState } from 'react';
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
import { testOCRConnection } from '@/services/settings';
import './index.less';

export function TranslateImageTextAiSettingsHint() {
  useLanguage();
  return (
    <Alert
      type="info"
      showIcon
      className="tm-translate-image-text-modal__settings-hint"
      message={tr("识别与翻译使用 AI 设置")}
      description={
        <>
          {tr(TRANSLATE_IMAGE_TEXT_AI_SETTINGS_HINT)}{' '}
          <Typography.Link onClick={() => history.push('/settings/image')}>{tr("前往 OCR 配置")}</Typography.Link>
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
  useLanguage();
  const [form] = Form.useForm<FormValues>();
  const { optionsForTask } = useImageProviders();
  const [ocrChecking, setOcrChecking] = useState(false);
  const [ocrReady, setOcrReady] = useState(false);
  const [ocrMessage, setOcrMessage] = useState('');
  const [ocrCheckFailed, setOcrCheckFailed] = useState(false);
  const [sourceImageFailed, setSourceImageFailed] = useState(false);
  const watchedSourceLanguage = Form.useWatch('sourceLanguage', form) ?? prefill?.sourceLanguage ?? 'auto';
  const watchedTargetLanguage = Form.useWatch('targetLanguage', form) ?? prefill?.targetLanguage ?? 'en';

  const productId = (fixedProductId || prefill?.productId || '').trim();
  const sourceImageId = (prefill?.sourceImageId || sourceImage?.id || '').trim();
  const sourceImageUrl = (
    prefill?.sourceImageUrl ||
    sourceImage?.publicUrl ||
    sourceImage?.originUrl ||
    ''
  ).trim();

  const providerOptions = useMemo(() => optionsForTask('translate_image_text'), [optionsForTask]);
  const sourceLanguageLabel = TRANSLATE_IMAGE_TEXT_SOURCE_LANG_OPTIONS.find((item) => item.value === watchedSourceLanguage)?.label ?? tr("自动识别");
  const targetLanguageLabel = TRANSLATE_IMAGE_TEXT_TARGET_LANG_OPTIONS.find((item) => item.value === watchedTargetLanguage)?.label ?? tr("英文");
  const ocrStatusType = ocrChecking ? 'info' : ocrReady ? 'success' : ocrCheckFailed ? 'error' : 'warning';
  const ocrStatusTitle = ocrChecking
    ? tr("正在检测 OCR 服务")
    : ocrReady
      ? tr("OCR 服务可用")
      : ocrCheckFailed
        ? tr("OCR 检测请求失败")
        : tr("图片文字翻译需要 OCR 服务");

  useEffect(() => {
    if (!open) return;
    form.setFieldsValue({
      sourceLanguage: prefill?.sourceLanguage ?? 'auto',
      targetLanguage: prefill?.targetLanguage ?? 'en',
      layoutMode: 'auto',
      autoSaveToProductImages: true,
      outputAsDetail: true,
      autoSetAsMain: false,
      renderMode: 'pure_text_replace',
      provider: prefill?.provider ?? '',
      ocrProvider: '',
      eraseMode: 'text_pixel_mask',
      advancedJson: '',
    });
  }, [open, form, prefill]);

  useEffect(() => {
    if (!open) return;
    setSourceImageFailed(false);
  }, [open, sourceImageUrl]);

  const checkOCRReady = useCallback(async () => {
    setOcrChecking(true);
    setOcrCheckFailed(false);
    try {
      const res = await testOCRConnection();
      setOcrReady(Boolean(res.ok));
      setOcrMessage(res.message || '');
    } catch (e: unknown) {
      setOcrReady(false);
      setOcrCheckFailed(true);
      setOcrMessage((e as Error)?.message || '');
    } finally {
      setOcrChecking(false);
    }
  }, []);

  useEffect(() => {
    if (!open) {
      setOcrReady(false);
      setOcrMessage('');
      setOcrCheckFailed(false);
      return;
    }
    void checkOCRReady();
  }, [open, checkOCRReady]);

  return (
    <ModalForm<FormValues>
      form={form}
      title={tr("图片文字翻译")}
      open={open}
      onOpenChange={onOpenChange}
      width={560}
      modalProps={{ destroyOnHidden: true, className: 'tm-translate-image-text-modal' }}
      submitter={{
        render: (props) => {
          const submitButtonProps = props.submitButtonProps === false ? undefined : props.submitButtonProps;

          return [
            <Button key="cancel" onClick={() => onOpenChange(false)}>
              {tr("取消")}
            </Button>,
            <Button
              key="submit"
              type="primary"
              loading={ocrChecking || submitButtonProps?.loading}
              disabled={!ocrReady}
              onClick={() => props.submit?.()}
            >
              {tr("创建翻译任务")}
            </Button>,
          ];
        },
      }}
      onFinish={async (values) => {
        if (!sourceImageId && !sourceImageUrl) {
          message.error(tr("请选择要翻译的商品图片"));
          return false;
        }
        if (!ocrReady) {
          message.error(tr("图片文字翻译需要 OCR 服务。请先到「设置 → 图片 AI 设置」选择 OCR 服务并测试通过。"));
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
            message.success(tr("图片文字翻译任务已提交，正在后台处理"));
          } else if (task.status === 'success' || task.status === 'success_with_warnings') {
            message.success(
              task.status === 'success_with_warnings'
                ? tr("翻译完成（存在警告，请人工检查）")
                : tr("翻译已完成"),
            );
          } else if (task.status === 'failed') {
            message.warning(task.errorMessage || tr("任务失败"));
          } else {
            message.success(tr("已创建"));
          }
          onSuccess?.();
          return true;
        } catch (e: unknown) {
          message.error((e as Error)?.message || tr("提交失败"));
          return false;
        }
      }}
    >
      <div className="tm-translate-image-text-modal__body">
        <section className="tm-translate-image-text-modal__intro">
          <div className="tm-translate-image-text-modal__intro-icon">
            <TranslationOutlined />
          </div>
          <div className="tm-translate-image-text-modal__intro-copy">
            <Typography.Text strong>{tr("提交后将创建后台图片翻译任务")}</Typography.Text>
            <Typography.Paragraph type="secondary">
              {tr("系统会识别图片文字并翻译为目标语言，再生成新的处理后图片。原图不会被覆盖，处理结果用于刊登前建议人工检查文字、版式和商品主体；AI 处理可能消耗模型额度。")}
            </Typography.Paragraph>
          </div>
        </section>

        <section className="tm-translate-image-text-modal__section">
          <div className="tm-translate-image-text-modal__section-head">
            <Typography.Text strong>
              <FileImageOutlined />
              {tr("待翻译图片")}
            </Typography.Text>
            {sourceImageId ? <Tag bordered={false}>{tr("源图 ID")} {sourceImageId}</Tag> : <Tag bordered={false}>{tr("未提供源图 ID")}</Tag>}
          </div>
          {sourceImageUrl ? (
            <div className="tm-translate-image-text-modal__image-panel">
              {sourceImageFailed ? (
                <div className="tm-translate-image-text-modal__image-fallback">
                  <FileImageOutlined />
                  <Typography.Text type="secondary">{tr("图片加载失败，请检查源图地址")}</Typography.Text>
                </div>
              ) : (
                <Image
                  src={sourceImageUrl}
                  className="tm-translate-image-text-modal__image"
                  onError={() => setSourceImageFailed(true)}
                />
              )}
              <Typography.Text
                type="secondary"
                className="tm-translate-image-text-modal__image-url"
                title={sourceImageUrl}
              >
                {sourceImageUrl}
              </Typography.Text>
            </div>
          ) : (
            <Alert
              type="warning"
              showIcon
              message={tr("未提供源图地址")}
              description={tr("当前任务缺少可翻译的图片 URL，请返回图片列表选择带有图片地址的商品图片。")}
            />
          )}
        </section>

        <section className="tm-translate-image-text-modal__section">
          <div className="tm-translate-image-text-modal__section-head">
            <Typography.Text strong>
              <SafetyCertificateOutlined />
              {tr("OCR 服务状态")}
            </Typography.Text>
            {ocrChecking ? <Spin size="small" /> : null}
          </div>
          <TranslateImageTextAiSettingsHint />
          <Alert
            type={ocrStatusType}
            showIcon
            message={ocrStatusTitle}
            description={
              <span>
                {ocrMessage || tr("请先到「设置 → 图片 AI 设置」选择 OCR 服务并测试通过。")}{' '}
                {!ocrReady ? <Typography.Link onClick={() => history.push('/settings/image')}>{tr("去配置 OCR")}</Typography.Link> : null}
              </span>
            }
            action={
              <Button size="small" loading={ocrChecking} onClick={() => void checkOCRReady()}>
                {tr("重新检测")}
              </Button>
            }
          />
        </section>

        <section className="tm-translate-image-text-modal__section">
          <div className="tm-translate-image-text-modal__section-head">
            <Typography.Text strong>
              <FontColorsOutlined />
              {tr("翻译设置")}
            </Typography.Text>
            <Space size={[6, 4]} wrap>
              <Tag bordered={false}>{sourceLanguageLabel}</Tag>
              <Tag color="blue" bordered={false}>
                {targetLanguageLabel}
              </Tag>
            </Space>
          </div>
          <div className="tm-translate-image-text-modal__field-grid">
            <ProFormSelect
              name="sourceLanguage"
              label={tr("源语言")}
              options={TRANSLATE_IMAGE_TEXT_SOURCE_LANG_OPTIONS}
              rules={[{ required: true, message: tr("请选择源语言") }]}
              extra={tr("自动识别会先尝试判断图片中文字语言，仍建议检查结果。")}
            />
            <ProFormSelect
              name="targetLanguage"
              label={tr("目标语言")}
              options={TRANSLATE_IMAGE_TEXT_TARGET_LANG_OPTIONS}
              rules={[{ required: true, message: tr("请选择目标语言") }]}
              extra={tr("生成的新图片会使用该语言绘制译文。")}
            />
          </div>

          <ProFormRadio.Group
            name="layoutMode"
            label={tr("排版方式")}
            options={TRANSLATE_IMAGE_TEXT_LAYOUT_MODE_OPTIONS}
            rules={[{ required: true, message: tr("请选择排版方式") }]}
            extra={tr("自动适配偏均衡；保持原图更保守；清晰可读会优先译文阅读体验。")}
          />
        </section>

        <section className="tm-translate-image-text-modal__section">
          <div className="tm-translate-image-text-modal__section-head">
            <Typography.Text strong>
              <CloudUploadOutlined />
              {tr("结果处理")}
            </Typography.Text>
            <Tag color="default" bordered={false}>
              {tr("不覆盖原图")}
            </Tag>
          </div>
          <div className="tm-translate-image-text-modal__result-options">
            <div className="tm-translate-image-text-modal__option">
              <ProFormCheckbox name="autoSaveToProductImages">{tr("自动保存到商品图片库")}</ProFormCheckbox>
              <Typography.Text type="secondary">{tr("处理结果会作为新图片保存，便于后续选择和刊登。")}</Typography.Text>
            </div>
            <div className="tm-translate-image-text-modal__option">
              <ProFormCheckbox name="outputAsDetail">{tr("处理后设为详情图")}</ProFormCheckbox>
              <Typography.Text type="secondary">{tr("保存后会把处理结果作为详情图用途。")}</Typography.Text>
            </div>
            <div className="tm-translate-image-text-modal__option tm-translate-image-text-modal__option--main">
              <ProFormCheckbox name="autoSetAsMain">{tr("处理后设为主图")}</ProFormCheckbox>
              <Typography.Text type="secondary">{tr("影响更高：处理结果可能成为商品主图，发布前请重点检查。")}</Typography.Text>
            </div>
          </div>
        </section>

        <Collapse
          className="tm-translate-image-text-modal__advanced"
          items={[
            {
              key: 'advanced',
              label: (
                <span className="tm-translate-image-text-modal__advanced-label">
                  <SettingOutlined />
                  {tr("高级设置")}
                  <Typography.Text type="secondary">{tr("普通翻译通常无需修改")}</Typography.Text>
                </span>
              ),
              children: (
                <>
                  <ProFormRadio.Group
                    name="renderMode"
                    label={tr("覆盖渲染方式")}
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
                          label={tr("覆盖图片 AI 服务")}
                          options={providerOptions}
                          extra={tr("仅在 AI 编辑实验模式下生效")}
                        />
                      );
                    }}
                  </Form.Item>
                  <ProFormSelect
                    name="eraseMode"
                    label={tr("覆盖擦除方式")}
                    options={[
                      { label: tr("默认（读取设置）"), value: '' },
                      { label: tr("自动"), value: 'auto' },
                      { label: tr("精细擦字（推荐）"), value: 'precise_mask' },
                      { label: tr("背景采样"), value: 'background_sample' },
                      { label: tr("模糊填充"), value: 'blur_fill' },
                      { label: tr("OpenCV 修复"), value: 'opencv_inpaint' },
                      { label: tr("AI 局部擦除"), value: 'ai_inpaint' },
                    ]}
                  />
                </>
              ),
            },
          ]}
        />

        {!ocrReady ? (
          <div className="tm-translate-image-text-modal__submit-note">
            <WarningOutlined />
            <Typography.Text type="secondary">{tr("OCR 服务检测通过后才能创建翻译任务。")}</Typography.Text>
          </div>
        ) : (
          <div className="tm-translate-image-text-modal__submit-note">
            <InfoCircleOutlined />
            <Typography.Text type="secondary">{tr("点击“创建翻译任务”后会进入后台处理，结果以任务状态为准。")}</Typography.Text>
          </div>
        )}
      </div>
    </ModalForm>
  );
}
