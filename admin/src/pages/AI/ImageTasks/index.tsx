import type { ActionType, ProColumns, ProFormInstance } from '@ant-design/pro-components';
import { TmPageContainer, TechnicalDetails, TaskJsonBlock, TmProTable as ProTable } from '@/components/ui';
import { formatDateTime } from '@/utils/formatTime';

import { CopyOutlined, EditOutlined } from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Drawer,
  Image,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  message,
  Typography,
} from 'antd';
import dayjs from 'dayjs';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useLocation } from '@umijs/max';
import { CreateImageTaskModal } from '@/components/CreateImageTaskModal';
import type {
  ImageTaskDetail,
  ImageTaskItemRow,
  ImageTaskListRow,
  TranslateManualEditBlock,
  TranslateManualEditState,
} from '@/services/imageTasks';
import { displayNameForProvider } from '@/constants/imageProviders';
import { useImageProviders } from '@/hooks/useImageProviders';
import {
  applyImageTaskResult,
  getImageTask,
  getTranslateManualEditState,
  IMAGE_TASK_TEMPLATES,
  isImageTaskSuccessStatus,
  isImageTaskUsableForProduct,
  listImageTaskItems,
  parseTranslateTaskOutput,
  queryImageTasks,
  retryImageTask,
  manualRenderTranslateImageTask,
  saveImageTaskItemToProduct,
  setImageTaskItemAsMain,
  taskTypeLabel,
  translateLangLabel,
  translateRenderQualityLevel,
  translateRenderModeLabel,
  translateTaskWarnings,
  pureTextHardFailureLabel,
  buildTranslateImageTextInput,
  createImageTask,
} from '@/services/imageTasks';
import { PAGE_COPY, commonStatusLabel } from '@/constants/copywriting';
import { COLLECT_TASK_STATUS } from '@/constants/status';

function statusTag(status: string) {
  const s = status?.trim() || '';
  const m = COLLECT_TASK_STATUS[s as keyof typeof COLLECT_TASK_STATUS];
  if (m) return <Tag color={m.color}>{m.text}</Tag>;
  const label = commonStatusLabel(s);
  return <Tag>{label === '—' ? s || '—' : label}</Tag>;
}

function validationModeLabel(mode?: string) {
  if (mode === 'validatePureTextReplace') return '纯文字替换校验';
  return mode || '—';
}

function ocrProviderLabel(provider?: string): string {
  const p = provider?.trim();
  if (!p) return '—';
  const map: Record<string, string> = {
    ai_vision: 'AI 视觉 OCR',
    paddleocr: '本地 PaddleOCR',
    aliyun: '阿里云 OCR',
    tencent: '腾讯云 OCR',
  };
  return map[p] ?? p;
}

function TranslateResultPanel({ output, taskStatus }: { output: unknown; taskStatus?: string }) {
  const parsed = parseTranslateTaskOutput(output);
  if (!parsed) return null;
  const blocks = parsed.ocr?.blocks ?? [];
  const layout = parsed.layout ?? parsed.quality?.layout;
  const translate = parsed.translate;
  const verification = parsed.verification;
  const renderQuality = parsed.renderQuality;
  const coord = parsed.coordinateMeta ?? (parsed.ocr as { coordinateMeta?: typeof parsed.coordinateMeta })?.coordinateMeta;
  const qualityLevel = translateRenderQualityLevel(parsed, taskStatus);
  const warnings = translateTaskWarnings(parsed);
  const hasOverflow = (layout?.overflowBlocks ?? 0) > 0;
  const ocrInfo = parsed.ocr;
  const configuredOcr = parsed.configuredOcrProvider ?? ocrInfo?.configuredOcrProvider;
  const actualOcr = parsed.actualOcrProvider ?? ocrInfo?.actualOcrProvider ?? ocrInfo?.provider;
  const fallbackUsed = parsed.ocrFallbackUsed ?? ocrInfo?.ocrFallbackUsed ?? ocrInfo?.fallback;
  const fallbackReason = parsed.ocrFallbackReason ?? ocrInfo?.ocrFallbackReason ?? ocrInfo?.errorMessage;
  const ocrErrorCode = parsed.ocrErrorCode ?? ocrInfo?.ocrErrorCode;
  const ocrBlocksCount =
    parsed.ocrBlocksCount ?? ocrInfo?.ocrBlocksCount ?? ocrInfo?.textBlocksCount ?? blocks.length;
  const ocrAverageConfidence = parsed.ocrAverageConfidence ?? ocrInfo?.ocrAverageConfidence ?? ocrInfo?.averageConfidence;
  const pureValidation = parsed.pureTextValidation;
  const validationMode = parsed.validationMode ?? pureValidation?.validationMode;
  const failureReasons =
    parsed.validationFailureReasons ??
    (pureValidation?.hardFailures ?? []).map((c) => pureTextHardFailureLabel(c));
  const hasSourceRemain =
    pureValidation?.sourceTextRemainDetected ||
    verification?.sourceTextMayRemain ||
    warnings.some((w) => w.includes('原文字') || w.includes('未删除干净'));
  const hasExtraBackground =
    pureValidation?.extraBackgroundLayerDetected ||
    warnings.some((w) => w.includes('额外背景'));
  const hasPureOverlap = warnings.some((w) => w.includes('重叠'));
  const hasPatchVisible =
    warnings.some((w) => w.includes('背景修补') || w.includes('补丁')) || hasExtraBackground;
  const eraseAreaTooLarge = warnings.some((w) => w.includes('擦除区域过大')) || (layout?.eraseAreaRatio ?? 0) > 0.12;
  return (
    <Card size="small" title="翻译结果摘要" style={{ marginBottom: 24 }}>
      <Descriptions column={2} size="small">
        <Descriptions.Item label="渲染方式">
          {translateRenderModeLabel(parsed.renderMode ?? layout?.renderMode)}
          {parsed.pureTextReplaceMode || parsed.renderMode === 'pure_text_replace' ? (
            <Tag color="blue" style={{ marginLeft: 8 }}>
              纯文字替换
            </Tag>
          ) : null}
        </Descriptions.Item>
        <Descriptions.Item label="渲染质量">
          <Tag color={qualityLevel.color}>{qualityLevel.text}</Tag>
          {renderQuality?.commercialUsabilityScore != null ? ` ${renderQuality.commercialUsabilityScore}` : ''}
        </Descriptions.Item>
        <Descriptions.Item label="源语言">
          {translateLangLabel(parsed.sourceLanguage || parsed.ocr?.detectedLanguage || '—')}
        </Descriptions.Item>
        <Descriptions.Item label="目标语言">{translateLangLabel(parsed.targetLanguage || '—')}</Descriptions.Item>
        <Descriptions.Item label="识别文字数量">
          {translate?.textBlocksCount ?? parsed.quality?.textBlocksCount ?? parsed.ocr?.textBlocksCount ?? blocks.length}
        </Descriptions.Item>
        <Descriptions.Item label="已翻译数量">
          {translate?.translatedBlocksCount ?? parsed.quality?.translatedBlocksCount ?? '—'}
        </Descriptions.Item>
        <Descriptions.Item label="已绘制数量">{translate?.renderedBlocksCount ?? '—'}</Descriptions.Item>
        <Descriptions.Item label="校验通过数量">{translate?.verifiedBlocksCount ?? '—'}</Descriptions.Item>
        <Descriptions.Item label="自动换行数量">{layout?.autoWrappedBlocks ?? 0}</Descriptions.Item>
        <Descriptions.Item label="自动缩小字号数量">{layout?.fontResizedBlocks ?? 0}</Descriptions.Item>
        <Descriptions.Item label="自动精简文案数量">{layout?.simplifiedBlocks ?? 0}</Descriptions.Item>
        <Descriptions.Item label="文字溢出数量">{layout?.overflowBlocks ?? 0}</Descriptions.Item>
        <Descriptions.Item label="是否存在文字溢出">{hasOverflow ? '是' : '否'}</Descriptions.Item>
        <Descriptions.Item label="擦除面积占比">
          {layout?.eraseAreaRatio != null ? `${(layout.eraseAreaRatio * 100).toFixed(2)}%` : '—'}
        </Descriptions.Item>
        <Descriptions.Item label="补丁面积占比">
          {layout?.patchAreaRatio != null ? `${(layout.patchAreaRatio * 100).toFixed(2)}%` : '—'}
        </Descriptions.Item>
        <Descriptions.Item label="背景破坏分">
          {layout?.backgroundDeltaScore != null ? layout.backgroundDeltaScore.toFixed(2) : '—'}
        </Descriptions.Item>
        <Descriptions.Item label="平铺填充占比">
          {layout?.flatFillRatio != null ? `${(layout.flatFillRatio * 100).toFixed(1)}%` : '—'}
        </Descriptions.Item>
        <Descriptions.Item label="检测到大背景块">
          {layout?.largePatchDetected ? '是' : '否'}
        </Descriptions.Item>
      </Descriptions>
      <TechnicalDetails label="渲染技术参数">
        <Descriptions column={2} size="small">
          <Descriptions.Item label="校验模式">{validationModeLabel(validationMode)}</Descriptions.Item>
          <Descriptions.Item label="擦除方式">{layout?.eraseMode ?? '—'}</Descriptions.Item>
          <Descriptions.Item label="版式模板">{layout?.layoutTemplate ?? '—'}</Descriptions.Item>
          <Descriptions.Item label="自动重试策略">
            {layout?.retryStrategies?.length ? layout.retryStrategies.join(' / ') : '—'}
          </Descriptions.Item>
          <Descriptions.Item label="最终质量状态">
            {commonStatusLabel(parsed.finalQualityStatus ?? taskStatus)}
          </Descriptions.Item>
        </Descriptions>
        <Typography.Text strong style={{ display: 'block', marginTop: 12 }}>
          OCR 配置与执行
        </Typography.Text>
        <Descriptions column={2} size="small" style={{ marginTop: 8 }}>
          <Descriptions.Item label="配置 OCR">{ocrProviderLabel(configuredOcr)}</Descriptions.Item>
          <Descriptions.Item label="实际 OCR">{ocrProviderLabel(actualOcr)}</Descriptions.Item>
          <Descriptions.Item label="接口类型">{ocrInfo?.apiName ?? '—'}</Descriptions.Item>
          <Descriptions.Item label="是否降级">{fallbackUsed ? '是' : '否'}</Descriptions.Item>
          <Descriptions.Item label="降级原因">{fallbackReason || '—'}</Descriptions.Item>
          <Descriptions.Item label="OCR 错误码">{ocrErrorCode || '—'}</Descriptions.Item>
          <Descriptions.Item label="识别文字数量">{ocrBlocksCount ?? '—'}</Descriptions.Item>
          <Descriptions.Item label="平均置信度">
            {ocrAverageConfidence != null ? ocrAverageConfidence.toFixed(2) : '—'}
          </Descriptions.Item>
          <Descriptions.Item label="低置信度过滤数量">{ocrInfo?.filteredBlocksCount ?? 0}</Descriptions.Item>
          <Descriptions.Item label="OCR 错误">{ocrInfo?.errorMessage || fallbackReason || '—'}</Descriptions.Item>
          <Descriptions.Item label="OCR 图片尺寸">
            {coord?.ocrImageWidth && coord?.ocrImageHeight
              ? `${coord.ocrImageWidth} × ${coord.ocrImageHeight}`
              : '—'}
          </Descriptions.Item>
          <Descriptions.Item label="渲染图片尺寸">
            {coord?.renderImageWidth && coord?.renderImageHeight
              ? `${coord.renderImageWidth} × ${coord.renderImageHeight}`
              : '—'}
          </Descriptions.Item>
          <Descriptions.Item label="scaleX / scaleY">
            {coord?.scaleX != null && coord?.scaleY != null
              ? `${coord.scaleX.toFixed(4)} / ${coord.scaleY.toFixed(4)}`
              : '—'}
          </Descriptions.Item>
          <Descriptions.Item label="是否坐标缩放">{coord?.coordScaleApplied ? '是' : '否'}</Descriptions.Item>
          <Descriptions.Item label="坐标修正数量">{coord?.bboxCorrectionCount ?? 0}</Descriptions.Item>
        </Descriptions>
        <Typography.Text strong style={{ display: 'block', marginTop: 12 }}>
          擦除与渲染
        </Typography.Text>
        <Descriptions column={2} size="small" style={{ marginTop: 8 }}>
          <Descriptions.Item label="eraseMode">{layout?.eraseMode ?? '—'}</Descriptions.Item>
          <Descriptions.Item label="eraseBlocks">{parsed.eraseBlocks ?? '—'}</Descriptions.Item>
          <Descriptions.Item label="erase_bbox 数量">{parsed.eraseBBoxCount ?? parsed.eraseBlocks ?? '—'}</Descriptions.Item>
          <Descriptions.Item label="layout_bbox 数量">{parsed.layoutBBoxCount ?? parsed.renderedTextCount ?? '—'}</Descriptions.Item>
          <Descriptions.Item label="eraseRetryCount">{parsed.eraseRetryCount ?? 0}</Descriptions.Item>
          <Descriptions.Item label="layoutTemplate">{layout?.layoutTemplate ?? '—'}</Descriptions.Item>
          <Descriptions.Item label="renderedTextCount">{parsed.renderedTextCount ?? '—'}</Descriptions.Item>
          <Descriptions.Item label="overflowTextCount">{parsed.overflowTextCount ?? 0}</Descriptions.Item>
          <Descriptions.Item label="badgeCount">{parsed.badgeCount ?? 0}</Descriptions.Item>
          <Descriptions.Item label="abnormalBadgeCount">{parsed.abnormalBadgeCount ?? 0}</Descriptions.Item>
          <Descriptions.Item label="backgroundPatchScore">
            {parsed.backgroundPatchScore != null ? parsed.backgroundPatchScore.toFixed(2) : '—'}
          </Descriptions.Item>
          <Descriptions.Item label="overlapScore">
            {parsed.overlapScore != null ? parsed.overlapScore.toFixed(2) : '—'}
          </Descriptions.Item>
          <Descriptions.Item label="badgeShapeAbnormal">{parsed.badgeShapeAbnormal ? '是' : '否'}</Descriptions.Item>
          <Descriptions.Item label="textOverlap">{parsed.textOverlap ? '是' : '否'}</Descriptions.Item>
        </Descriptions>
      </TechnicalDetails>
      <div style={{ marginTop: 12 }}>
        <Typography.Text strong>渲染检查</Typography.Text>
        <Descriptions column={2} size="small" style={{ marginTop: 8 }}>
          <Descriptions.Item label="原文残留">{hasSourceRemain ? '是' : '否'}</Descriptions.Item>
          <Descriptions.Item label="额外背景层">{hasExtraBackground ? '是' : '否'}</Descriptions.Item>
          <Descriptions.Item label="译文重叠">{hasPureOverlap ? '是' : '否'}</Descriptions.Item>
          <Descriptions.Item label="背景补丁">{hasPatchVisible ? '是' : '否'}</Descriptions.Item>
          <Descriptions.Item label="擦除区域过大">{eraseAreaTooLarge ? '是' : '否'}</Descriptions.Item>
          <Descriptions.Item label="是否可商用">{qualityLevel.text}</Descriptions.Item>
          <Descriptions.Item label="最终建议">
            {qualityLevel.recommendMain ? '可作为商品图' : '不建议直接使用'}
          </Descriptions.Item>
        </Descriptions>
        {!qualityLevel.recommendMain ? (
          <Alert
            style={{ marginTop: 8 }}
            type="error"
            showIcon
            message={
              failureReasons.length > 0
                ? `渲染校验失败：${failureReasons.join('、')}`
                : '当前结果排版异常，不建议直接使用，请重新生成或人工调整。'
            }
          />
        ) : null}
        {taskStatus === 'success_with_review' ? (
          <Alert
            style={{ marginTop: 8 }}
            type="warning"
            showIcon
            message="结果可用，建议人工检查排版。"
          />
        ) : null}
      </div>
      {renderQuality ? (
        <div style={{ marginTop: 12 }}>
          <Typography.Text strong>质量评分</Typography.Text>
          <Descriptions column={2} size="small" style={{ marginTop: 8 }}>
            <Descriptions.Item label="文字写入">{renderQuality.textAppliedScore ?? '—'}</Descriptions.Item>
            <Descriptions.Item label="原文清理">{renderQuality.sourceTextRemovedScore ?? '—'}</Descriptions.Item>
            <Descriptions.Item label="排版自然度">{renderQuality.layoutScore ?? '—'}</Descriptions.Item>
            <Descriptions.Item label="风格一致性">{renderQuality.styleConsistencyScore ?? '—'}</Descriptions.Item>
            <Descriptions.Item label="可读性">{renderQuality.readabilityScore ?? '—'}</Descriptions.Item>
            <Descriptions.Item label="商品保护">{renderQuality.productPreservationScore ?? '—'}</Descriptions.Item>
          </Descriptions>
          {!renderQuality.passed ? (
            <Alert
              style={{ marginTop: 8 }}
              type="warning"
              showIcon
              message="当前结果排版一般，建议重新生成或人工检查。"
            />
          ) : null}
        </div>
      ) : null}
      {(parsed.debugOriginalUrl || parsed.debugMaskUrl || parsed.debugErasedUrl || parsed.debugFinalUrl) ? (
        <div style={{ marginTop: 16 }}>
          <Typography.Text strong>查看修图过程</Typography.Text>
          <Typography.Paragraph type="secondary" style={{ marginTop: 4, marginBottom: 8 }}>
            对比原图、文字 mask、擦除后背景与最终成图，定位 mask、擦除、背景或重绘问题。
          </Typography.Paragraph>
          <Space wrap size="middle">
            {parsed.debugOriginalUrl ? (
              <div>
                <Typography.Text type="secondary">原图</Typography.Text>
                <br />
                <Image src={parsed.debugOriginalUrl} width={160} style={{ marginTop: 4 }} />
              </div>
            ) : null}
            {parsed.debugMaskUrl ? (
              <div>
                <Typography.Text type="secondary">文字区域</Typography.Text>
                <br />
                <Image src={parsed.debugMaskUrl} width={160} style={{ marginTop: 4 }} />
              </div>
            ) : null}
            {parsed.debugErasedUrl ? (
              <div>
                <Typography.Text type="secondary">擦除后背景</Typography.Text>
                <br />
                <Image src={parsed.debugErasedUrl} width={160} style={{ marginTop: 4 }} />
              </div>
            ) : null}
            {parsed.debugFinalUrl ? (
              <div>
                <Typography.Text type="secondary">最终成图</Typography.Text>
                <br />
                <Image src={parsed.debugFinalUrl} width={160} style={{ marginTop: 4 }} />
              </div>
            ) : null}
          </Space>
        </div>
      ) : null}
      {verification ? (
        <div style={{ marginTop: 12 }}>
          <Typography.Text strong>校验摘要</Typography.Text>
          <Descriptions column={2} size="small" style={{ marginTop: 8 }}>
            <Descriptions.Item label="图片已更新">{verification.imageChanged ? '是' : '否'}</Descriptions.Item>
            <Descriptions.Item label="检测到目标语言文字">
              {verification.targetTextDetected ? '是' : '否'}
            </Descriptions.Item>
            <Descriptions.Item label="可能仍有原文字残留">
              {verification.sourceTextMayRemain ? '是' : '否'}
            </Descriptions.Item>
            <Descriptions.Item label="校验置信度">
              {verification.confidence != null ? verification.confidence.toFixed(2) : '—'}
            </Descriptions.Item>
          </Descriptions>
        </div>
      ) : null}
      {warnings.length > 0 ? (
        <div style={{ marginTop: 12 }}>
          {warnings.map((w) => (
            <Tag key={w} color="warning" style={{ marginBottom: 4 }}>
              {w}
            </Tag>
          ))}
        </div>
      ) : null}
      {blocks.length > 0 ? (
        <div style={{ marginTop: 12 }}>
          <Typography.Text strong>翻译文本预览</Typography.Text>
          <ul style={{ marginTop: 8, paddingLeft: 20, marginBottom: 0 }}>
            {blocks.slice(0, 12).map((b, i) => (
              <li key={`${b.text}-${i}`} style={{ marginBottom: 4 }}>
                <Typography.Text type="secondary">{b.text || '—'}</Typography.Text>
                {b.blockClass ? (
                  <>
                    {' '}
                    <Tag>{b.blockClass}</Tag>
                  </>
                ) : null}
                {' → '}
                <Typography.Text>{b.translatedText || '—'}</Typography.Text>
                {b.drawText && b.drawText !== b.translatedText ? (
                  <>
                    {' '}
                    <Typography.Text type="secondary">（实际绘制：{b.drawText}）</Typography.Text>
                  </>
                ) : null}
                {b.shortTranslatedText && b.shortTranslatedText !== b.translatedText ? (
                  <>
                    {' '}
                    <Typography.Text type="secondary">（精简：{b.compactTranslation || b.shortTranslatedText}）</Typography.Text>
                  </>
                ) : null}
              </li>
            ))}
          </ul>
          {blocks.length > 12 ? (
            <Typography.Text type="secondary">… 共 {blocks.length} 段文字</Typography.Text>
          ) : null}
        </div>
      ) : null}
      {parsed.blockClassifications?.length ? (
        <TechnicalDetails label="高级配置">
          <TaskJsonBlock title="文字块分类结果" value={parsed.blockClassifications} last />
        </TechnicalDetails>
      ) : null}
    </Card>
  );
}

function manualBaseImageUrl(state: TranslateManualEditState | null, base: 'original' | 'erased' | 'result') {
  if (!state) return '';
  if (base === 'erased') return state.erasedImageUrl || state.originalImageUrl || state.baseImageUrl || '';
  if (base === 'result') return state.resultImageUrl || state.erasedImageUrl || state.originalImageUrl || state.baseImageUrl || '';
  return state.originalImageUrl || state.baseImageUrl || state.erasedImageUrl || '';
}

function ManualTranslateEditor({
  open,
  loading,
  state,
  onCancel,
  onSaved,
}: {
  open: boolean;
  loading: boolean;
  state: TranslateManualEditState | null;
  onCancel: () => void;
  onSaved: (row: ImageTaskDetail) => void;
}) {
  const [blocks, setBlocks] = useState<TranslateManualEditBlock[]>([]);
  const [baseImage, setBaseImage] = useState<'original' | 'erased' | 'result'>('original');
  const [saving, setSaving] = useState(false);
  const [previewWidth, setPreviewWidth] = useState(0);

  useEffect(() => {
    if (!open || !state) return;
    setBlocks(state.blocks ?? []);
    setBaseImage(state.originalImageUrl ? 'original' : state.erasedImageUrl ? 'erased' : 'result');
    setPreviewWidth(0);
  }, [open, state]);

  const updateBlock = useCallback((index: number, patch: Partial<TranslateManualEditBlock>) => {
    setBlocks((prev) => prev.map((b, i) => (i === index ? { ...b, ...patch } : b)));
  }, []);

  const updateBBox = useCallback((index: number, patch: Partial<TranslateManualEditBlock['bbox']>) => {
    setBlocks((prev) =>
      prev.map((b, i) => (i === index ? { ...b, bbox: { ...b.bbox, ...patch } } : b)),
    );
  }, []);

  const updateEraseBBox = useCallback((index: number, patch: Partial<TranslateManualEditBlock['eraseBBox']>) => {
    setBlocks((prev) =>
      prev.map((b, i) => (i === index ? { ...b, eraseBBox: { ...b.eraseBBox, ...patch } } : b)),
    );
  }, []);

  const baseUrl = manualBaseImageUrl(state, baseImage);
  const imageWidth = state?.imageWidth || 1;
  const imageHeight = state?.imageHeight || 1;
  const scale = previewWidth > 0 && state?.imageWidth ? previewWidth / state.imageWidth : 0.6;

  const save = async () => {
    if (!state?.taskId) return;
    setSaving(true);
    try {
      const row = await manualRenderTranslateImageTask(state.taskId, {
        baseImage,
        outputFormat: 'webp',
        note: 'manual translate text layout edit',
        verifyOutputText: false,
        blocks,
      });
      message.success('已保存人工编辑译图');
      onSaved(row);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '保存人工译图失败');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      title="人工编辑翻译图片"
      open={open}
      onCancel={onCancel}
      onOk={save}
      okText="渲染并保存"
      cancelText="取消"
      confirmLoading={saving}
      width={1180}
      destroyOnHidden
    >
      {loading ? (
        <div style={{ textAlign: 'center', padding: 48 }}>
          <Spin />
        </div>
      ) : state ? (
        <div style={{ display: 'grid', gridTemplateColumns: 'minmax(420px, 1fr) 420px', gap: 16 }}>
          <div>
            <Space style={{ marginBottom: 12 }} wrap>
              <Typography.Text strong>底图</Typography.Text>
              <Select
                value={baseImage}
                style={{ width: 180 }}
                onChange={setBaseImage}
                options={[
                  { label: '原图重新擦除', value: 'original', disabled: !state.originalImageUrl },
                  { label: '已擦除底图', value: 'erased', disabled: !state.erasedImageUrl },
                  { label: '当前结果图', value: 'result', disabled: !state.resultImageUrl },
                ]}
              />
              <Typography.Text type="secondary">
                {state.imageWidth} x {state.imageHeight}
              </Typography.Text>
            </Space>
            <div
              style={{
                position: 'relative',
                width: '100%',
                maxWidth: 680,
                border: '1px solid var(--ant-color-border)',
                background: 'var(--ant-color-fill-quaternary)',
                overflow: 'hidden',
              }}
            >
              {baseUrl ? (
                <img
                  src={baseUrl}
                  alt=""
                  style={{ display: 'block', width: '100%', height: 'auto' }}
                  onLoad={(e) => setPreviewWidth(e.currentTarget.clientWidth)}
                />
              ) : null}
              {blocks.map((b) =>
                b.hidden ? null : (
                  <div
                    key={b.id}
                    style={{
                      position: 'absolute',
                      left: `${(b.bbox.x / imageWidth) * 100}%`,
                      top: `${(b.bbox.y / imageHeight) * 100}%`,
                      width: `${(b.bbox.width / imageWidth) * 100}%`,
                      minHeight: `${(b.bbox.height / imageHeight) * 100}%`,
                      boxSizing: 'border-box',
                      border: '1px dashed #1677ff',
                      color: b.color || '#111111',
                      fontSize: Math.max(10, Math.round((b.fontSize || 18) * scale)),
                      fontWeight: b.fontWeight || undefined,
                      textAlign: (b.align as 'left' | 'center' | 'right') || 'left',
                      lineHeight: 1.15,
                      padding: 2,
                      pointerEvents: 'none',
                      whiteSpace: 'pre-wrap',
                    }}
                  >
                    {b.text}
                  </div>
                ),
              )}
            </div>
          </div>
          <div style={{ maxHeight: 650, overflow: 'auto', paddingRight: 4 }}>
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 12 }}
              message="保存时会用当前块配置重新擦除原文并绘制译文，适合清理旧中文、旧标签底和文字重叠。"
            />
            <Space direction="vertical" style={{ width: '100%' }} size={12}>
              {blocks.map((b, index) => (
                <div
                  key={b.id}
                  style={{
                    border: '1px solid var(--ant-color-border)',
                    borderRadius: 6,
                    padding: 12,
                    background: 'var(--ant-color-bg-container)',
                  }}
                >
                  <Space wrap style={{ marginBottom: 8 }}>
                    <Tag>{b.blockClass || 'text'}</Tag>
                    <Typography.Text type="secondary">{b.sourceText || b.id}</Typography.Text>
                    <Switch
                      size="small"
                      checked={!b.hidden}
                      checkedChildren="显示"
                      unCheckedChildren="隐藏"
                      onChange={(checked) => updateBlock(index, { hidden: !checked })}
                    />
                    <Switch
                      size="small"
                      checked={b.removeSourceBackground}
                      checkedChildren="清原文"
                      unCheckedChildren="仅重绘"
                      onChange={(checked) => updateBlock(index, { removeSourceBackground: checked })}
                    />
                  </Space>
                  <Input.TextArea
                    value={b.text}
                    autoSize={{ minRows: 1, maxRows: 3 }}
                    onChange={(e) => updateBlock(index, { text: e.target.value })}
                  />
                  <Space wrap size={8} style={{ marginTop: 8 }}>
                    <InputNumber prefix="x" value={b.bbox.x} onChange={(v) => updateBBox(index, { x: Number(v ?? 0) })} />
                    <InputNumber prefix="y" value={b.bbox.y} onChange={(v) => updateBBox(index, { y: Number(v ?? 0) })} />
                    <InputNumber
                      prefix="w"
                      min={1}
                      value={b.bbox.width}
                      onChange={(v) => updateBBox(index, { width: Number(v ?? 1) })}
                    />
                    <InputNumber
                      prefix="h"
                      min={1}
                      value={b.bbox.height}
                      onChange={(v) => updateBBox(index, { height: Number(v ?? 1) })}
                    />
                    <InputNumber
                      prefix="字号"
                      min={8}
                      max={96}
                      value={b.fontSize}
                      onChange={(v) => updateBlock(index, { fontSize: Number(v ?? 18) })}
                    />
                    <Input
                      prefix="颜色"
                      value={b.color || '#111111'}
                      style={{ width: 150 }}
                      onChange={(e) => updateBlock(index, { color: e.target.value })}
                    />
                    <Select
                      value={b.align || 'left'}
                      style={{ width: 100 }}
                      onChange={(align) => updateBlock(index, { align })}
                      options={[
                        { label: '左对齐', value: 'left' },
                        { label: '居中', value: 'center' },
                        { label: '右对齐', value: 'right' },
                      ]}
                    />
                  </Space>
                  {b.removeSourceBackground ? (
                    <Space wrap size={8} style={{ marginTop: 8 }}>
                      <InputNumber
                        prefix="擦除 x"
                        value={b.eraseBBox.x}
                        onChange={(v) => updateEraseBBox(index, { x: Number(v ?? 0) })}
                      />
                      <InputNumber
                        prefix="擦除 y"
                        value={b.eraseBBox.y}
                        onChange={(v) => updateEraseBBox(index, { y: Number(v ?? 0) })}
                      />
                      <InputNumber
                        prefix="擦除 w"
                        min={1}
                        value={b.eraseBBox.width}
                        onChange={(v) => updateEraseBBox(index, { width: Number(v ?? 1) })}
                      />
                      <InputNumber
                        prefix="擦除 h"
                        min={1}
                        value={b.eraseBBox.height}
                        onChange={(v) => updateEraseBBox(index, { height: Number(v ?? 1) })}
                      />
                    </Space>
                  ) : null}
                </div>
              ))}
            </Space>
          </div>
        </div>
      ) : (
        <Alert type="warning" showIcon message="暂无可编辑数据，请先生成一次图片文字翻译结果。" />
      )}
    </Modal>
  );
}

export default function ImageTasksPage() {
  const location = useLocation();
  const { caps } = useImageProviders();
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const statusFromUrl = useMemo(() => {
    try {
      return new URLSearchParams(location.search || '').get('status')?.trim() || undefined;
    } catch {
      return undefined;
    }
  }, [location.search]);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState<ImageTaskDetail | null>(null);
  const [taskItems, setTaskItems] = useState<ImageTaskItemRow[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [createPrefill, setCreatePrefill] = useState<{ taskType?: string }>({});
  const [manualEditorOpen, setManualEditorOpen] = useState(false);
  const [manualEditorLoading, setManualEditorLoading] = useState(false);
  const [manualEditorState, setManualEditorState] = useState<TranslateManualEditState | null>(null);
  const translateOutput = useMemo(
    () => (detail?.taskType === 'translate_image_text' ? parseTranslateTaskOutput(detail.output) : null),
    [detail?.taskType, detail?.output],
  );
  const translateQualityLevel = useMemo(
    () => translateRenderQualityLevel(translateOutput, detail?.status),
    [translateOutput, detail?.status],
  );
  const isLowQualityTranslate = detail?.taskType === 'translate_image_text' && !translateQualityLevel.recommendMain;

  useEffect(() => {
    const iv = window.setInterval(() => {
      if (document.visibilityState !== 'visible') return;
      actionRef.current?.reload?.();
    }, 4000);
    return () => window.clearInterval(iv);
  }, []);

  useEffect(() => {
    if (!statusFromUrl) return;
    formRef.current?.setFieldsValue?.({ status: statusFromUrl });
    actionRef.current?.reload?.();
  }, [statusFromUrl]);

  useEffect(() => {
    if (!drawerOpen || !detail) return;
    if (detail.status !== 'pending' && detail.status !== 'running' && detail.status !== 'retrying') return;
    const id = detail.id;
    const iv = window.setInterval(() => {
      if (document.visibilityState !== 'visible') return;
      void (async () => {
        try {
          const row = await getImageTask(id);
          setDetail(row);
          if (row.status !== 'pending' && row.status !== 'running' && row.status !== 'retrying') {
            actionRef.current?.reload?.();
          }
        } catch {
          /* ignore transient errors during poll */
        }
      })();
    }, 4000);
    return () => window.clearInterval(iv);
  }, [drawerOpen, detail?.id, detail?.status]);

  const openDetail = useCallback(async (id: string) => {
    setDrawerOpen(true);
    setDetail(null);
    setDetailLoading(true);
    try {
      const row = await getImageTask(id);
      setDetail(row);
      try {
        const itemsRes = await listImageTaskItems(id);
        setTaskItems(itemsRes.list ?? []);
      } catch {
        setTaskItems([]);
      }
    } finally {
      setDetailLoading(false);
    }
  }, []);

  const openManualEditor = useCallback(async () => {
    if (!detail?.id) return;
    setManualEditorOpen(true);
    setManualEditorLoading(true);
    try {
      const state = await getTranslateManualEditState(detail.id);
      setManualEditorState(state);
      if (!state.blocks?.length) {
        message.warning('当前任务没有可编辑文字块，请先重新生成一次翻译结果');
      }
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载人工编辑数据失败');
      setManualEditorOpen(false);
    } finally {
      setManualEditorLoading(false);
    }
  }, [detail?.id]);

  const columns: ProColumns<ImageTaskListRow>[] = [
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      width: 172,
      search: false,
      render: (_, row) => formatDateTime(row.createdAt),
    },
    {
      title: '任务类型',
      dataIndex: 'taskType',
      width: 180,
      ellipsis: true,
      render: (_, row) => taskTypeLabel(row.taskType),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 148,
      valueType: 'select',
      valueEnum: Object.fromEntries(
        Object.entries(COLLECT_TASK_STATUS).map(([k, v]) => [k, { text: v.text }]),
      ),
      render: (_, row) => statusTag(row.status),
    },
    {
      title: '图片服务',
      dataIndex: 'provider',
      width: 120,
      ellipsis: true,
      render: (_, row) => displayNameForProvider(caps, row.provider ?? ''),
    },
    {
      title: '重试',
      width: 130,
      search: false,
      render: (_, row) => {
        const rc = row.retryCount ?? 0;
        const mr = row.maxRetries ?? 0;
        return (
          <span>
            {rc}/{mr || '—'}
          </span>
        );
      },
    },
    {
      title: '下次自动重试',
      dataIndex: 'nextRetryAt',
      width: 172,
      search: false,
      render: (_, row) => (row.nextRetryAt ? formatDateTime(row.nextRetryAt) : '—'),
    },
    {
      title: '商品 ID',
      dataIndex: 'productId',
      width: 260,
      ellipsis: true,
      copyable: true,
    },
    {
      title: '源图 URL',
      dataIndex: 'sourceImageUrl',
      width: 240,
      ellipsis: true,
      search: false,
    },
    {
      title: '结果 URL',
      dataIndex: 'resultUrl',
      width: 240,
      ellipsis: true,
      search: false,
    },
    {
      title: '错误信息',
      dataIndex: 'errorMessage',
      width: 200,
      ellipsis: true,
      search: false,
    },
    {
      title: '时间范围',
      dataIndex: 'dateRange',
      valueType: 'dateTimeRange',
      hideInTable: true,
      search: {
        transform: (value) => {
          if (!value || !Array.isArray(value) || value.length < 2) return {};
          const a = dayjs(value[0] as string | dayjs.Dayjs);
          const b = dayjs(value[1] as string | dayjs.Dayjs);
          if (!a.isValid() || !b.isValid()) return {};
          return { start: a.toISOString(), end: b.toISOString() };
        },
      },
    },
    {
      title: '操作',
      valueType: 'option',
      width: 160,
      search: false,
      render: (_, row) => [
        <Button key="detail" type="link" onClick={() => void openDetail(row.id)}>
          查看详情
        </Button>,
        row.status === 'failed' ? (
          <Button
            key="retry"
            type="link"
            onClick={async () => {
              try {
                await retryImageTask(row.id);
                message.success('已提交重试，正在后台处理');
                actionRef.current?.reload();
              } catch (e: unknown) {
                message.error((e as Error)?.message || '重试失败');
              }
            }}
          >
            重试
          </Button>
        ) : null,
      ],
    },
  ];

  return (
    <TmPageContainer title={PAGE_COPY.aiImageTasks.title} subTitle={PAGE_COPY.aiImageTasks.description}>
      <Card size="small" style={{ marginBottom: 16 }} title="快捷模板">
        <Space wrap>
          {IMAGE_TASK_TEMPLATES.map((tpl) => (
            <Button
              key={tpl.taskType}
              onClick={() => {
                setCreatePrefill({ taskType: tpl.taskType });
                setCreateOpen(true);
              }}
            >
              {tpl.title}
            </Button>
          ))}
        </Space>
        <Typography.Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>
          所有 AI 结果图会自动上传到「设置 → 存储设置」当前启用的存储位置，不会直接使用第三方临时链接。
        </Typography.Paragraph>
      </Card>
      <ProTable<ImageTaskListRow>
        rowKey="id"
        actionRef={actionRef}
        formRef={formRef}
        columns={columns}
        search={{ labelWidth: 'auto', defaultCollapsed: false }}
        options={{ reload: true, density: true, setting: true }}
        pagination={{ defaultPageSize: 20, showSizeChanger: true }}
        scroll={{ x: 1960 }}
        tableStyle={{ width: '100%', minWidth: '100%' }}
        request={async (params) => {
          const res = await queryImageTasks({
            page: params.current,
            pageSize: params.pageSize,
            taskType: params.taskType as string | undefined,
            status: params.status as string | undefined,
            provider: params.provider as string | undefined,
            productId: params.productId as string | undefined,
            start: params.start as string | undefined,
            end: params.end as string | undefined,
          });
          return {
            data: res.list,
            success: true,
            total: res.pagination.total,
          };
        }}
        headerTitle={false}
        toolBarRender={() => [
          <Button
            key="create"
            type="primary"
            onClick={() => {
              setCreatePrefill({});
              setCreateOpen(true);
            }}
          >
            新建任务
          </Button>,
        ]}
      />

      <CreateImageTaskModal
        open={createOpen}
        onOpenChange={setCreateOpen}
        prefill={createPrefill}
        allowProductIdInput
        onSuccess={() => actionRef.current?.reload()}
      />

      <Drawer
        title="图片任务详情"
        width={720}
        open={drawerOpen}
        onClose={() => {
          setDrawerOpen(false);
          setDetail(null);
        }}
        destroyOnHidden
        footer={
          <Space wrap style={{ width: '100%', justifyContent: 'flex-end' }}>
            {detail?.taskType === 'translate_image_text' ? (
              <Button icon={<EditOutlined />} onClick={() => void openManualEditor()}>
                人工编辑译图
              </Button>
            ) : null}
            {detail?.taskType === 'translate_image_text' &&
            (detail.status === 'failed' ||
              detail.status === 'success_with_warnings' ||
              detail.status === 'low_quality' ||
              detail.status === 'need_manual_review' ||
              detail.status === 'failed_render_validation') ? (
              <>
                <Button
                  onClick={async () => {
                    if (!detail?.id) return;
                    try {
                      const base =
                        detail.input && typeof detail.input === 'object'
                          ? (detail.input as Record<string, unknown>)
                          : {};
                      await createImageTask({
                        taskType: 'translate_image_text',
                        productId: detail.productId,
                        sourceImageId: detail.sourceImageId,
                        sourceImageUrl: detail.sourceImageUrl,
                        input: buildTranslateImageTextInput({
                          ...(base as Parameters<typeof buildTranslateImageTextInput>[0]),
                          renderMode: 'pure_text_replace',
                        }),
                      });
                      message.success('已提交程序排版翻译任务');
                      actionRef.current?.reload();
                    } catch (e: unknown) {
                      message.error((e as Error)?.message || '提交失败');
                    }
                  }}
                >
                  程序排版重新生成
                </Button>
                <Button
                  onClick={async () => {
                    if (!detail?.id) return;
                    try {
                      const base =
                        detail.input && typeof detail.input === 'object'
                          ? (detail.input as Record<string, unknown>)
                          : {};
                      await createImageTask({
                        taskType: 'translate_image_text',
                        productId: detail.productId,
                        sourceImageId: detail.sourceImageId,
                        sourceImageUrl: detail.sourceImageUrl,
                        input: {
                          ...buildTranslateImageTextInput({
                            ...(base as Parameters<typeof buildTranslateImageTextInput>[0]),
                            renderMode: 'pure_text_replace',
                          }),
                          minFontSize: 12,
                        },
                      });
                      message.success('已提交低字号重试任务');
                      actionRef.current?.reload();
                    } catch (e: unknown) {
                      message.error((e as Error)?.message || '提交失败');
                    }
                  }}
                >
                  降低字号重试
                </Button>
                <Button
                  onClick={async () => {
                    if (!detail?.id) return;
                    try {
                      const base =
                        detail.input && typeof detail.input === 'object'
                          ? (detail.input as Record<string, unknown>)
                          : {};
                      await createImageTask({
                        taskType: 'translate_image_text',
                        productId: detail.productId,
                        sourceImageId: detail.sourceImageId,
                        sourceImageUrl: detail.sourceImageUrl,
                        input: buildTranslateImageTextInput({
                          ...(base as Parameters<typeof buildTranslateImageTextInput>[0]),
                          renderMode: 'pure_text_replace',
                          layoutTemplate: 'preserve_original',
                        }),
                      });
                      message.success('已提交保持原版式重试任务');
                      actionRef.current?.reload();
                    } catch (e: unknown) {
                      message.error((e as Error)?.message || '提交失败');
                    }
                  }}
                >
                  保持原版式重试
                </Button>
                <Button
                  onClick={async () => {
                    if (!detail?.id) return;
                    try {
                      const base =
                        detail.input && typeof detail.input === 'object'
                          ? (detail.input as Record<string, unknown>)
                          : {};
                      await createImageTask({
                        taskType: 'translate_image_text',
                        productId: detail.productId,
                        sourceImageId: detail.sourceImageId,
                        sourceImageUrl: detail.sourceImageUrl,
                        input: buildTranslateImageTextInput({
                          ...(base as Parameters<typeof buildTranslateImageTextInput>[0]),
                          renderMode: 'deterministic',
                          eraseMode: 'opencv_inpaint',
                          layoutTemplate: 'title_badge',
                          styleMode: 'recreate',
                          maxFontSize: 36,
                        }),
                      });
                      message.success('已提交商品图模板重试任务');
                      actionRef.current?.reload();
                    } catch (e: unknown) {
                      message.error((e as Error)?.message || '提交失败');
                    }
                  }}
                >
                  使用商品图模板重试
                </Button>
              </>
            ) : null}
            {detail?.status === 'failed' ? (
              <Button
                type="primary"
                onClick={async () => {
                  if (!detail?.id) return;
                  try {
                    await retryImageTask(detail.id);
                    message.success(
                      detail.taskType === 'translate_image_text' ? '已重新提交翻译任务' : '已提交重试，正在后台处理',
                    );
                    const row = await getImageTask(detail.id);
                    setDetail(row);
                    actionRef.current?.reload();
                  } catch (e: unknown) {
                    message.error((e as Error)?.message || '重试失败');
                  }
                }}
              >
                {detail.taskType === 'translate_image_text' ? '重新翻译' : '重试'}
              </Button>
            ) : null}
            {detail?.resultUrl &&
            (isImageTaskSuccessStatus(detail.status) ||
              detail.status === 'low_quality' ||
              detail.status === 'need_manual_review' ||
              detail.status === 'failed_render_validation') ? (
              <>
              <Button
                icon={<CopyOutlined />}
                onClick={() => {
                  void navigator.clipboard.writeText(detail.resultUrl!);
                  message.success('已复制结果 URL');
                }}
              >
                复制结果 URL
              </Button>
              <Button
                href={detail.resultUrl}
                target="_blank"
                rel="noreferrer"
              >
                下载图片
              </Button>
              </>
            ) : null}
            {detail && isImageTaskUsableForProduct(detail.status) && detail.productId && (detail.resultUrl || detail.resultFileId) ? (
              <>
                <Popconfirm
                  title="确认保存到商品图片？"
                  description={
                    detail.taskType !== 'translate_image_text' || translateQualityLevel.recommendMain
                      ? '结果将追加为 AI 生成图，不覆盖原图。'
                      : '当前结果排版异常，不建议直接使用，请重新生成或人工调整。'
                  }
                  onConfirm={async () => {
                    try {
                      await applyImageTaskResult(detail.id, {
                        productId: detail.productId!,
                        applyMode: 'ai_generated',
                      });
                      message.success('已保存到商品图片库');
                    } catch (e: unknown) {
                      message.error((e as Error)?.message || '保存失败');
                    }
                  }}
                >
                  <Button type="primary" ghost disabled={isLowQualityTranslate}>
                    保存到商品图片
                  </Button>
                </Popconfirm>
                <Popconfirm
                  title="确认设为主图？"
                  description={
                    detail.taskType !== 'translate_image_text' || translateQualityLevel.recommendMain
                      ? '结果质量已通过，可以设为主图。'
                      : '当前结果排版异常，不建议直接使用，请重新生成或人工调整。'
                  }
                  onConfirm={async () => {
                    try {
                      await applyImageTaskResult(detail.id, {
                        productId: detail.productId!,
                        applyMode: 'main',
                        setBest: true,
                      });
                      message.success('已设为主图');
                    } catch (e: unknown) {
                      message.error((e as Error)?.message || '设置失败');
                    }
                  }}
                >
                  <Button danger={isLowQualityTranslate} disabled={isLowQualityTranslate}>
                    设为主图
                  </Button>
                </Popconfirm>
                <Popconfirm
                  title="确认设为详情图？"
                  description={
                    detail.taskType !== 'translate_image_text' || translateQualityLevel.recommendMain
                      ? '结果将写入商品详情图。'
                      : '当前结果排版异常，不建议直接使用，请重新生成或人工调整。'
                  }
                  onConfirm={async () => {
                    try {
                      await applyImageTaskResult(detail.id, {
                        productId: detail.productId!,
                        applyMode: 'detail',
                      });
                      message.success('已设为详情图');
                    } catch (e: unknown) {
                      message.error((e as Error)?.message || '设置失败');
                    }
                  }}
                >
                  <Button danger={isLowQualityTranslate} disabled={isLowQualityTranslate}>
                    设为详情图
                  </Button>
                </Popconfirm>
              </>
            ) : null}
          </Space>
        }
      >
        {detailLoading ? (
          <div style={{ textAlign: 'center', padding: 48 }}>
            <Spin />
          </div>
        ) : detail ? (
          <>
            <Descriptions column={1} size="small" bordered style={{ marginBottom: 24 }}>
              <Descriptions.Item label="任务类型">{taskTypeLabel(detail.taskType)}</Descriptions.Item>
              <Descriptions.Item label="状态">{statusTag(detail.status)}</Descriptions.Item>
              <Descriptions.Item label="图片服务">{detail.provider || '—'}</Descriptions.Item>
              <Descriptions.Item label="关联商品">{detail.productId || '—'}</Descriptions.Item>
              <Descriptions.Item label="创建者">{detail.createdBy || '—'}</Descriptions.Item>
              <Descriptions.Item label="开始时间">
                {detail.startedAt ? formatDateTime(detail.startedAt) : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="结束时间">
                {detail.finishedAt ? formatDateTime(detail.finishedAt) : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="创建时间">
                {formatDateTime(detail.createdAt)}
              </Descriptions.Item>
              <Descriptions.Item label="自动重试">
                {detail.retryCount ?? 0} / {detail.maxRetries ?? '—'}
              </Descriptions.Item>
              <Descriptions.Item label="下次自动重试">
                {detail.nextRetryAt ? formatDateTime(detail.nextRetryAt) : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="错误信息">{detail.errorMessage || '—'}</Descriptions.Item>
            </Descriptions>
            {(detail.sourceImageUrl || detail.resultUrl) && (
              <Space align="start" size={24} wrap style={{ marginBottom: 24 }}>
                {detail.sourceImageUrl ? (
                  <div>
                    <div style={{ marginBottom: 8, fontWeight: 600 }}>原图</div>
                    <Image src={detail.sourceImageUrl} width={200} style={{ objectFit: 'contain', borderRadius: 6 }} />
                  </div>
                ) : null}
                {detail.resultUrl ? (
                  <div>
                    <div style={{ marginBottom: 8, fontWeight: 600 }}>翻译后图片</div>
                    <Image src={detail.resultUrl} width={200} style={{ objectFit: 'contain', borderRadius: 6 }} />
                  </div>
                ) : null}
              </Space>
            )}
            {detail.taskType === 'translate_image_text' && detail.output ? (
              <TranslateResultPanel output={detail.output} taskStatus={detail.status} />
            ) : null}
            {taskItems.length > 0 ? (
              <div style={{ marginBottom: 24 }}>
                <div style={{ marginBottom: 8, fontWeight: 600 }}>子任务结果</div>
                <Space direction="vertical" style={{ width: '100%' }}>
                  {taskItems.map((item) => (
                    <Card key={item.id} size="small">
                      <Space align="start" wrap>
                        {item.sourceImageUrl ? (
                          <div>
                            <div style={{ fontSize: 12, marginBottom: 4 }}>原图</div>
                            <Image src={item.sourceImageUrl} width={120} />
                          </div>
                        ) : null}
                        {item.outputImageUrl ? (
                          <div>
                            <div style={{ fontSize: 12, marginBottom: 4 }}>结果</div>
                            <Image src={item.outputImageUrl} width={120} />
                          </div>
                        ) : null}
                        <div>
                          <div>状态：{statusTag(item.status)}</div>
                          {item.isSelectedBest ? <Tag color="gold">推荐主图</Tag> : null}
                          {item.scoreJson ? (
                            <TechnicalDetails label="评分详情">
                              <TaskJsonBlock title="评分数据" value={item.scoreJson} maxHeight={120} last />
                            </TechnicalDetails>
                          ) : null}
                          {detail.productId && item.status === 'success' && item.outputImageUrl ? (
                            <Space wrap style={{ marginTop: 8 }}>
                              <Button
                                size="small"
                                onClick={() =>
                                  void setImageTaskItemAsMain(item.id, { productId: detail.productId! }).then(() =>
                                    message.success('已设为主图'),
                                  )
                                }
                              >
                                设为主图
                              </Button>
                              <Button
                                size="small"
                                onClick={() =>
                                  void saveImageTaskItemToProduct(item.id, {
                                    productId: detail.productId!,
                                    applyMode: 'detail',
                                  }).then(() => message.success('已设为详情图'))
                                }
                              >
                                设为详情图
                              </Button>
                              <Button
                                size="small"
                                onClick={() =>
                                  void saveImageTaskItemToProduct(item.id, {
                                    productId: detail.productId!,
                                    applyMode: 'ai_generated',
                                  }).then(() => message.success('已保存到商品图库'))
                                }
                              >
                                保存到商品
                              </Button>
                            </Space>
                          ) : null}
                        </div>
                      </Space>
                    </Card>
                  ))}
                </Space>
              </div>
            ) : null}
            <TechnicalDetails label="任务技术信息">
              <Descriptions column={1} size="small" bordered style={{ marginBottom: 12 }}>
                <Descriptions.Item label="任务编号">{detail.id}</Descriptions.Item>
                {detail.sourceImageId ? (
                  <Descriptions.Item label="源图编号">{detail.sourceImageId}</Descriptions.Item>
                ) : null}
              </Descriptions>
              <TaskJsonBlock title="任务输入" value={detail.input} maxHeight={360} />
              <TaskJsonBlock title="任务输出" value={detail.output} maxHeight={360} last />
            </TechnicalDetails>
          </>
        ) : (
          <div style={{ color: 'var(--ant-color-text-secondary)' }}>未加载到图片任务详情，请从列表重新选择一条任务。</div>
        )}
      </Drawer>
      <ManualTranslateEditor
        open={manualEditorOpen}
        loading={manualEditorLoading}
        state={manualEditorState}
        onCancel={() => setManualEditorOpen(false)}
        onSaved={(row) => {
          setDetail(row);
          setManualEditorOpen(false);
          actionRef.current?.reload?.();
        }}
      />
    </TmPageContainer>
  );
}
