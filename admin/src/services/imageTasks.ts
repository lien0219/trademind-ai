import { deleteJSON, getJSON, getWithParams, postJSON } from '@/services/request';

export type ImageTaskListRow = {
  id: string;
  taskType: string;
  provider: string;
  status: string;
  productId?: string;
  sourceImageId?: string;
  sourceImageUrl?: string;
  resultFileId?: string;
  resultUrl?: string;
  errorMessage?: string;
  retryCount?: number;
  maxRetries?: number;
  nextRetryAt?: string;
  retryEnqueuedAt?: string;
  createdBy?: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type ImageTaskDetail = ImageTaskListRow & {
  input?: unknown;
  output?: unknown;
};

export type ImageTasksPagination = {
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

type ListResponse = {
  list: ImageTaskListRow[];
  pagination: ImageTasksPagination;
};

export async function queryImageTasks(params: {
  page?: number;
  pageSize?: number;
  taskType?: string;
  status?: string;
  provider?: string;
  productId?: string;
  start?: string;
  end?: string;
}): Promise<ListResponse> {
  return getWithParams<ListResponse>('/api/v1/ai/image/tasks', {
    page: params.page,
    pageSize: params.pageSize,
    taskType: params.taskType || undefined,
    status: params.status || undefined,
    provider: params.provider || undefined,
    productId: params.productId || undefined,
    start: params.start || undefined,
    end: params.end || undefined,
  });
}

export async function getImageTask(id: string): Promise<ImageTaskDetail> {
  return getJSON<ImageTaskDetail>(`/api/v1/ai/image/tasks/${id}`);
}

export async function createImageTask(payload: {
  taskType: string;
  provider?: string;
  productId?: string;
  sourceImageId?: string;
  sourceImageUrl?: string;
  input?: Record<string, unknown>;
}): Promise<ImageTaskDetail> {
  const body: Record<string, unknown> = {
    taskType: payload.taskType,
    productId: payload.productId ?? '',
    sourceImageId: payload.sourceImageId ?? '',
    sourceImageUrl: payload.sourceImageUrl ?? '',
    input: payload.input ?? {},
  };
  const p = payload.provider?.trim();
  if (p) {
    body.provider = p;
  }
  return postJSON<ImageTaskDetail>('/api/v1/ai/image/tasks', body);
}

export async function retryImageTask(id: string): Promise<ImageTaskDetail> {
  return postJSON<ImageTaskDetail>(`/api/v1/image/tasks/${id}/retry`, {});
}

export type ImageQueueMonitorQueue = {
  enabled: boolean;
  name: string;
  redisAvailable: boolean;
  depth: number;
  workerEnabled: boolean;
  workerRunning: boolean;
  concurrency: number;
};

export type ImageTaskMonitorSnapshot = {
  queue: ImageQueueMonitorQueue;
  worker: { enabled: boolean; concurrency: number; running: boolean };
  tasks: {
    pending: number;
    running: number;
    retrying: number;
    success: number;
    failed: number;
    cancelled: number;
  };
  retry: {
    enabled: boolean;
    maxRetries: number;
    baseDelaySeconds: number;
    maxDelaySeconds: number;
    nextRetryDueCount: number;
    oldestRetryingSeconds?: number;
  };
  recentRetrying: Array<{
    id: string;
    taskType: string;
    provider: string;
    productId?: string;
    retryCount: number;
    maxRetries: number;
    nextRetryAt?: string;
    errorMessage?: string;
    updatedAt: string;
  }>;
  recentFailures: Array<{
    id: string;
    taskType: string;
    provider: string;
    productId?: string;
    errorMessage: string;
    updatedAt: string;
  }>;
};

export async function applyImageTaskResult(
  taskId: string,
  payload: { productId: string; itemId?: string; applyMode?: string; setBest?: boolean },
) {
  return postJSON(`/api/v1/image/tasks/${taskId}/apply`, payload);
}

export async function saveImageTaskItemToProduct(
  itemId: string,
  payload: { productId: string; applyMode?: string; setBest?: boolean },
) {
  return postJSON(`/api/v1/ai/image/task-items/${itemId}/save-to-product`, payload);
}

export async function setImageTaskItemAsMain(itemId: string, payload: { productId: string }) {
  return postJSON(`/api/v1/ai/image/task-items/${itemId}/set-as-main`, payload);
}

export type ImageScoreResult = {
  overallScore: number;
  clarityScore: number;
  cleanlinessScore: number;
  compositionScore: number;
  mainSuitabilityScore: number;
  detailSuitabilityScore: number;
  issues: string[];
  suggestion: string;
  width?: number;
  height?: number;
  source?: string;
};

export async function scoreProductImage(payload: {
  productId?: string;
  sourceImageId?: string;
  sourceImageUrl?: string;
  imageType?: string;
}) {
  return postJSON<ImageScoreResult>('/api/v1/ai/image/score', payload);
}

export type ImageTaskItemRow = {
  id: string;
  taskId: string;
  sourceImageId?: string;
  sourceImageUrl?: string;
  outputImageUrl?: string;
  outputStorageKey?: string;
  outputFileId?: string;
  scoreJson?: unknown;
  isSelectedBest?: boolean;
  status: string;
  errorMessage?: string;
  createdAt: string;
  updatedAt: string;
};

export async function listImageTaskItems(taskId: string): Promise<{ list: ImageTaskItemRow[] }> {
  return getJSON(`/api/v1/image/tasks/${taskId}/items`);
}

export async function deleteImageTaskItem(taskId: string, itemId: string) {
  return deleteJSON(`/api/v1/image/tasks/${taskId}/items/${itemId}`);
}

export const IMAGE_TASK_TYPE_OPTIONS: { label: string; value: string; group?: string }[] = [
  { label: '去背景', value: 'remove_background', group: '基础' },
  { label: '换背景', value: 'replace_background', group: '基础' },
  { label: '场景图', value: 'generate_scene', group: '基础' },
  { label: '去水印', value: 'remove_watermark', group: '清理' },
  { label: '去 Logo', value: 'remove_logo', group: '清理' },
  { label: '去角标/贴纸', value: 'remove_badge', group: '清理' },
  { label: '去二维码', value: 'remove_qrcode', group: '清理' },
  { label: '综合清理', value: 'cleanup', group: '清理' },
  { label: '详情图增强', value: 'enhance_detail', group: '增强' },
  { label: '高清修复', value: 'upscale', group: '增强' },
  { label: '图片文字翻译', value: 'translate_image_text', group: '翻译' },
  { label: '营销图生成', value: 'generate_marketing', group: '生成' },
  { label: '主图生成', value: 'generate_main_image', group: '生成' },
  { label: '批量主图生成', value: 'batch_generate_main', group: '生成' },
  { label: '商品图评分', value: 'score_image', group: '评分' },
  { label: '自动选最佳主图', value: 'select_best_main', group: '评分' },
  { label: '缩放', value: 'resize', group: '其他' },
  { label: '增强', value: 'enhance', group: '其他' },
];

export const IMAGE_TASK_TEMPLATES: { title: string; taskType: string; description: string }[] = [
  { title: '去水印', taskType: 'remove_watermark', description: '去除商品图水印，结果自动入库' },
  { title: '去 Logo', taskType: 'remove_logo', description: '去除品牌 Logo 与角标' },
  { title: '去角标/贴纸', taskType: 'remove_badge', description: '去除角标、贴纸等装饰元素' },
  { title: '去二维码', taskType: 'remove_qrcode', description: '去除二维码、条码等扫描元素' },
  { title: '综合清理', taskType: 'cleanup', description: '一次性清理水印/Logo/贴纸/二维码' },
  { title: '去背景', taskType: 'remove_background', description: '白底图 / 抠图（remove.bg）' },
  { title: '高清修复', taskType: 'upscale', description: '提升清晰度，适合模糊主图' },
  {
    title: '图片文字翻译',
    taskType: 'translate_image_text',
    description: '识别图中文字并翻译为目标语言，生成新图片',
  },
  { title: '营销图生成', taskType: 'generate_marketing', description: '基于商品图生成营销图' },
  { title: '详情图增强', taskType: 'enhance_detail', description: '增强详情图清晰度并去杂' },
  { title: '批量主图生成', taskType: 'batch_generate_main', description: '为多商品批量生成主图候选' },
  { title: '商品图评分', taskType: 'score_image', description: '多维评分与优化建议' },
  { title: '自动选最佳主图', taskType: 'select_best_main', description: '评分并推荐/自动设主图' },
];

export function taskTypeLabel(taskType: string): string {
  const hit = IMAGE_TASK_TYPE_OPTIONS.find((t) => t.value === taskType);
  return hit?.label ?? taskType;
}

/** Default task types shown in beginner-friendly create modal. */
export const BEGINNER_IMAGE_TASK_TYPE_VALUES = [
  'remove_watermark',
  'remove_logo',
  'remove_background',
  'cleanup',
  'generate_marketing',
  'enhance_detail',
  'upscale',
  'translate_image_text',
  'score_image',
  'select_best_main',
] as const;

export type ImageTaskResultMode = 'auto_save' | 'set_main' | 'set_detail' | 'result_only';

export const IMAGE_TASK_RESULT_MODE_OPTIONS: { label: string; value: ImageTaskResultMode; description: string }[] = [
  {
    label: '自动保存到商品图片库',
    value: 'auto_save',
    description: '处理成功后追加为 AI 生成图，不覆盖原图',
  },
  {
    label: '处理完成后设为主图',
    value: 'set_main',
    description: '保存结果并设为主图 / 最佳主图',
  },
  {
    label: '处理完成后设为详情图',
    value: 'set_detail',
    description: '保存结果并标记为详情图',
  },
  {
    label: '仅生成结果，不自动写入商品图片',
    value: 'result_only',
    description: '结果可在 AI 图片任务页手动保存',
  },
];

export function buildResultHandlingInput(mode: ImageTaskResultMode): Record<string, unknown> {
  switch (mode) {
    case 'auto_save':
      return { autoSaveToProduct: true };
    case 'set_main':
      return { autoSaveToProduct: true, autoSetMain: true };
    case 'set_detail':
      return { autoSaveToProduct: true, autoSetDetail: true };
    default:
      return {};
  }
}

/** Task types that may omit a single source image when productId is set. */
export function imageTaskAllowsNoSource(taskType: string): boolean {
  return taskType === 'select_best_main';
}

export const TRANSLATE_IMAGE_TEXT_SOURCE_LANG_OPTIONS = [
  { label: '自动识别', value: 'auto' },
  { label: '中文', value: 'zh' },
  { label: '英文', value: 'en' },
];

export const TRANSLATE_IMAGE_TEXT_TARGET_LANG_OPTIONS = [
  { label: '英文', value: 'en' },
  { label: '中文', value: 'zh' },
];

export function translateLangLabel(code: string): string {
  const all = [...TRANSLATE_IMAGE_TEXT_SOURCE_LANG_OPTIONS, ...TRANSLATE_IMAGE_TEXT_TARGET_LANG_OPTIONS];
  return all.find((o) => o.value === code)?.label ?? code;
}

export type TranslateImageTextLayoutMode = 'auto' | 'preserve' | 'readable';

export type TranslateImageTextInputOpts = {
  sourceLanguage?: string;
  targetLanguage?: string;
  renderMode?: TranslateRenderMode;
  ocrProvider?: string;
  eraseMode?: string;
  advancedJson?: string;
  verifyOutputText?: boolean;
  preserveLayout?: boolean;
  removeOriginalText?: boolean;
  keepProductUnchanged?: boolean;
  autoSaveToProductImages?: boolean;
  outputAsDetail?: boolean;
  autoSetAsMain?: boolean;
  outputFormat?: string;
  layoutMode?: TranslateImageTextLayoutMode;
  autoLayout?: boolean;
  autoWrap?: boolean;
  autoFontSize?: boolean;
  allowTextBoxExpand?: boolean;
  allowTextSimplify?: boolean;
  minFontSize?: number;
  maxFontSize?: number;
  lineHeightRatio?: number;
  maxLines?: number;
  textPadding?: number;
  maskPadding?: number;
  layoutTemplate?: 'preserve_original' | 'ecommerce_clean' | 'title_badge' | 'product_relayout' | 'auto';
  compactTranslation?: boolean;
  allowTextOverflow?: boolean;
  styleMode?: 'preserve' | 'recreate';
};

export const TRANSLATE_IMAGE_TEXT_LAYOUT_MODE_OPTIONS = [
  { label: '自动适配，推荐', value: 'auto' as const },
  { label: '尽量保持原图', value: 'preserve' as const },
  { label: '优先清晰可读', value: 'readable' as const },
];

/** User-facing hint: OCR/translate uses AI settings; production path uses deterministic render. */
export const TRANSLATE_IMAGE_TEXT_AI_SETTINGS_HINT =
  '图片文字翻译必须先在「设置 → 图片 AI 设置」配置并测试 OCR 服务；系统不会自动切换 OCR。翻译走「设置 → AI 设置」里的文本模型，默认使用纯文字替换（先擦除原字再绘制译文，不添加白底/气泡）。';

export type TranslateRenderMode =
  | 'pure_text_replace'
  | 'remove_text_then_render'
  | 'hybrid'
  | 'deterministic'
  | 'ai_edit';

export const TRANSLATE_IMAGE_TEXT_RENDER_MODE_OPTIONS = [
  { label: '纯文字替换（推荐）', value: 'pure_text_replace' as const },
  { label: '去字后绘制', value: 'remove_text_then_render' as const },
  { label: 'AI 擦除 + 程序排版', value: 'hybrid' as const },
  { label: '程序排版渲染', value: 'deterministic' as const },
  { label: 'AI 编辑实验模式', value: 'ai_edit' as const },
];

export function translateRenderModeLabel(mode?: string): string {
  return TRANSLATE_IMAGE_TEXT_RENDER_MODE_OPTIONS.find((o) => o.value === mode)?.label ?? mode ?? '—';
}

/** Build input JSON for translate_image_text tasks. */
export function buildTranslateImageTextInput(opts: TranslateImageTextInputOpts): Record<string, unknown> {
  const layoutMode = opts.layoutMode ?? 'auto';
  const outputImageType = opts.autoSetAsMain ? 'main' : opts.outputAsDetail !== false ? 'detail' : 'ai_generated';
  const input: Record<string, unknown> = {
    sourceLanguage: opts.sourceLanguage ?? 'auto',
    targetLanguage: opts.targetLanguage ?? 'en',
    renderMode: opts.renderMode ?? 'pure_text_replace',
    ocrProvider: opts.ocrProvider ?? undefined,
    eraseMode: opts.eraseMode ?? 'text_pixel_mask',
    advancedJson: opts.advancedJson ?? undefined,
    verifyOutputText: opts.verifyOutputText !== false,
    preserveLayout: opts.preserveLayout !== false,
    removeOriginalText: opts.removeOriginalText !== false,
    keepProductUnchanged: opts.keepProductUnchanged !== false,
    outputFormat: opts.outputFormat ?? 'webp',
    autoSaveToProductImages: opts.autoSaveToProductImages !== false,
    autoSetAsMain: opts.autoSetAsMain === true,
    outputImageType,
    layoutMode,
    autoLayout: opts.autoLayout !== false,
    autoWrap: opts.autoWrap !== false,
    autoFontSize: opts.autoFontSize !== false,
    allowTextBoxExpand: opts.allowTextBoxExpand === true,
    allowTextSimplify: opts.allowTextSimplify !== false,
    compactTranslation: opts.compactTranslation !== false,
    allowTextOverflow: opts.allowTextOverflow === true,
    minFontSize: opts.minFontSize ?? 14,
    maxFontSize: opts.maxFontSize ?? 52,
    lineHeightRatio: opts.lineHeightRatio ?? 1.15,
    maxLines: opts.maxLines ?? 3,
    textPadding: opts.textPadding ?? 6,
    maskPadding: opts.maskPadding ?? 2,
    layoutTemplate: opts.layoutTemplate ?? 'preserve_original',
    compactTranslation: opts.compactTranslation !== false,
    allowTextOverflow: opts.allowTextOverflow === true,
    styleMode: opts.styleMode ?? undefined,
  };
  if (layoutMode === 'preserve') {
    input.allowTextBoxExpand = opts.allowTextBoxExpand === true;
    input.allowTextSimplify = opts.allowTextSimplify === true;
  }
  if (layoutMode === 'readable') {
    input.minFontSize = opts.minFontSize ?? 16;
    input.maxLines = opts.maxLines ?? 4;
  }
  if (opts.autoSaveToProductImages !== false) {
    input.autoSaveToProduct = true;
    if (opts.autoSetAsMain) {
      input.autoSetMain = true;
    } else if (opts.outputAsDetail !== false) {
      input.autoSetDetail = true;
    }
  }
  return input;
}

export type TranslateLayoutSummary = {
  autoLayout?: boolean;
  layoutTemplate?: string;
  eraseAreaRatio?: number;
  patchAreaRatio?: number;
  backgroundDeltaScore?: number;
  flatFillRatio?: number;
  largePatchDetected?: boolean;
  retryStrategies?: string[];
  textBlocksCount?: number;
  autoWrappedBlocks?: number;
  fontResizedBlocks?: number;
  simplifiedBlocks?: number;
  overflowBlocks?: number;
  minFontSizeUsed?: number;
  warnings?: string[];
};

export type TranslateTaskOutput = {
  sourceLanguage?: string;
  targetLanguage?: string;
  renderMode?: string;
  ocr?: {
    provider?: string;
    apiName?: string;
    configuredOcrProvider?: string;
    actualOcrProvider?: string;
    fallback?: boolean;
    ocrFallbackUsed?: boolean;
    ocrFallbackReason?: string;
    ocrErrorCode?: string;
    averageConfidence?: number;
    ocrAverageConfidence?: number;
    filteredBlocksCount?: number;
    errorMessage?: string;
    detectedLanguage?: string;
    textBlocksCount?: number;
    ocrBlocksCount?: number;
    blocks?: Array<{
      id?: string;
      blockClass?: string;
      text?: string;
      translatedText?: string;
      standardTranslation?: string;
      shortTranslatedText?: string;
      compactTranslation?: string;
      drawText?: string;
      confidence?: number;
    }>;
    groups?: Array<{
      id?: string;
      groupType?: string;
      translatedLines?: string[];
    }>;
    coordinateMeta?: TranslateTaskOutput['coordinateMeta'];
  };
  configuredOcrProvider?: string;
  actualOcrProvider?: string;
  ocrFallbackUsed?: boolean;
  ocrFallbackReason?: string;
  ocrErrorCode?: string;
  ocrBlocksCount?: number;
  ocrAverageConfidence?: number;
  translate?: {
    sourceLanguage?: string;
    targetLanguage?: string;
    textBlocksCount?: number;
    translatedBlocksCount?: number;
    renderedBlocksCount?: number;
    verifiedBlocksCount?: number;
  };
  layout?: TranslateLayoutSummary & { renderMode?: string; eraseMode?: string };
  verification?: {
    imageChanged?: boolean;
    targetTextDetected?: boolean;
    sourceTextMayRemain?: boolean;
    confidence?: number;
    outputTextVerifyFailed?: boolean;
  };
  quality?: {
    textBlocksCount?: number;
    translatedBlocksCount?: number;
    lowConfidenceBlocksCount?: number;
    layoutPreserved?: boolean;
    layout?: TranslateLayoutSummary;
    warnings?: string[];
  };
  renderQuality?: {
    textAppliedScore?: number;
    sourceTextRemovedScore?: number;
    layoutScore?: number;
    styleConsistencyScore?: number;
    readabilityScore?: number;
    productPreservationScore?: number;
    commercialUsabilityScore?: number;
    passed?: boolean;
    warnings?: string[];
  };
  coordinateMeta?: {
    ocrImageWidth?: number;
    ocrImageHeight?: number;
    renderImageWidth?: number;
    renderImageHeight?: number;
    scaleX?: number;
    scaleY?: number;
    coordScaleApplied?: boolean;
    bboxCorrectionCount?: number;
    coordMappingValid?: boolean;
  };
  eraseBlocks?: number;
  eraseBBoxCount?: number;
  layoutBBoxCount?: number;
  eraseRetryCount?: number;
  renderedTextCount?: number;
  overflowTextCount?: number;
  blockClassifications?: Array<{
    id?: string;
    text?: string;
    blockClass?: string;
    standard_translation?: string;
    standardTranslation?: string;
    compact_translation?: string;
    compactTranslation?: string;
  }>;
  badgeCount?: number;
  abnormalBadgeCount?: number;
  backgroundPatchScore?: number;
  overlapScore?: number;
  finalQualityStatus?: string;
  qualityAutoRetried?: boolean;
  debugOriginalUrl?: string;
  debugMaskUrl?: string;
  debugErasedUrl?: string;
  debugFinalUrl?: string;
  pureTextReplaceMode?: boolean;
  validationMode?: string;
  pureTextValidation?: {
    validationMode?: string;
    sourceTextRemainDetected?: boolean;
    targetTextDetected?: boolean;
    textOverflowCount?: number;
    productOcclusionRatio?: number;
    extraBackgroundLayerDetected?: boolean;
    translatedTextOverlapOldText?: boolean;
    hardFailures?: string[];
    softWarnings?: string[];
    overallScore?: number;
    hardPassed?: boolean;
  };
  validationFailureReasons?: string[];
  badgeShapeAbnormal?: boolean;
  textOverlap?: boolean;
};

const TRANSLATE_LAYOUT_WARNING_LABELS: Record<string, string> = {
  translated_text_too_long: '部分翻译文字较长，可能影响图片排版，请检查结果图。',
  translated_text_overflow: '部分翻译文字较长，可能影响图片排版，请检查结果图。',
  font_size_auto_adjusted: '系统已自动调整部分文字大小。',
  translated_text_simplified: '系统已自动精简部分翻译文案以适配排版。',
  partial_text_detected: '部分图片文字可能未全部识别，请检查结果图是否仍有未翻译文字。',
  ocr_hallucination_filtered: '已过滤疑似非原图文字，仅翻译图片中真实可见的文字。',
  source_text_may_remain: '图片中可能仍有部分原文字，请检查结果图。',
  IMAGE_NOT_CHANGED: '生成图片没有变化，请重新生成或切换处理方式。',
  IMAGE_TEXT_NOT_APPLIED: '翻译文字没有成功写入图片，请重新生成。',
  OUTPUT_TEXT_VERIFY_FAILED: '系统无法确认文字替换效果，请人工检查图片。',
  background_patch_visible: '背景修补痕迹明显。',
  layout_not_natural: '排版不够自然。',
  text_overflow: '部分文字超出推荐区域。',
  text_not_applied: '翻译文字没有成功写入图片。',
  commercial_usability_low: '商用评分偏低，不建议直接作为商品图使用。',
  style_inherited: '已按商品图标题 / 标签模板继承原图样式。',
  erase_area_too_large: '擦除区域过大，可能破坏背景。',
  ocr_coord_scaled: 'OCR 坐标已按渲染图尺寸缩放。',
  badge_shape_abnormal: '标签形状异常（如圆块过大）。',
  text_overlap: '译文与原文或商品主体发生重叠。',
  erase_failed: '原文擦除未通过校验。',
  layout_unbalanced: '版面不均衡，译文位置或面积异常。',
  product_subject_overlap: '译文可能遮挡商品主体。',
  pure_text_source_not_erased: '旧文字未删除干净，请重新生成。',
  pure_text_extra_background: '检测到额外背景层（白底/气泡/标签补片），纯文字替换模式不允许。',
  pure_text_overlap: '英文译文与未擦除的中文发生重叠，请重新生成。',
  pure_text_font_size_off: '字号与原图略有偏差，建议人工检查。',
};

const PURE_TEXT_HARD_FAILURE_LABELS: Record<string, string> = {
  source_text_remain: '原中文残留',
  target_text_missing: '新英文未渲染',
  text_overflow: '英文溢出',
  product_occlusion: '遮挡商品主体',
  extra_background_layer: '出现额外背景层',
  translated_overlap_old_text: '英文与旧中文重叠',
};

export function pureTextHardFailureLabel(code: string): string {
  return PURE_TEXT_HARD_FAILURE_LABELS[code?.trim() || ''] ?? code;
}

/** Map machine layout warning codes to user-friendly Chinese labels. */
export function translateLayoutWarningLabel(code: string): string {
  const key = code?.trim() || '';
  return TRANSLATE_LAYOUT_WARNING_LABELS[key] ?? code;
}

/** Merge quality.warnings (already humanized) with layout.warnings (machine codes). */
export function translateTaskWarnings(output: TranslateTaskOutput | null): string[] {
  if (!output) return [];
  const seen = new Set<string>();
  const out: string[] = [];
  const add = (msg: string) => {
    const m = msg?.trim();
    if (!m || seen.has(m)) return;
    seen.add(m);
    out.push(m);
  };
  for (const w of output.quality?.warnings ?? []) {
    add(w);
  }
  for (const w of output.quality?.layout?.warnings ?? []) {
    add(translateLayoutWarningLabel(w));
  }
  for (const w of output.renderQuality?.warnings ?? []) {
    add(translateLayoutWarningLabel(w));
  }
  return out;
}

export function translateRenderQualityLevel(
  output: TranslateTaskOutput | null,
  taskStatus?: string,
): {
  text: string;
  color: string;
  recommendMain: boolean;
} {
  if (taskStatus === 'low_quality' || taskStatus === 'failed_render_validation') {
    return { text: '失败：质量不达标', color: 'error', recommendMain: false };
  }
  if (taskStatus === 'success_with_review') {
    return { text: '结果可用，建议人工检查排版', color: 'warning', recommendMain: true };
  }
  if (taskStatus === 'need_manual_review') {
    return { text: '需人工检查', color: 'warning', recommendMain: false };
  }
  const rq = output?.renderQuality;
  const score = rq?.commercialUsabilityScore ?? 0;
  if (
    output?.pureTextReplaceMode ||
    output?.renderMode === 'pure_text_replace' ||
    output?.validationMode === 'validatePureTextReplace'
  ) {
    const pv = output?.pureTextValidation;
    if (pv?.hardPassed === false && (output?.validationFailureReasons?.length || pv?.hardFailures?.length)) {
      return { text: '失败：渲染校验未通过', color: 'error', recommendMain: false };
    }
    if (taskStatus === 'success_with_review' || (pv?.hardPassed && (pv?.overallScore ?? score) >= 60 && (pv?.overallScore ?? score) < 75)) {
      return { text: '结果可用，建议人工检查排版', color: 'warning', recommendMain: true };
    }
  }
  if (
    output?.verification?.sourceTextMayRemain ||
    output?.badgeShapeAbnormal ||
    (output?.abnormalBadgeCount ?? 0) > 0 ||
    (output?.overlapScore ?? 0) > 0.01 ||
    output?.finalQualityStatus === 'low_quality' ||
    output?.resultUnavailable === true ||
    (output?.overflowTextCount ?? 0) > 0
  ) {
    return { text: '低质量（不建议商用）', color: 'error', recommendMain: false };
  }
  if (rq?.passed && score >= 75) {
    return { text: '可直接使用', color: 'success', recommendMain: true };
  }
  if (score >= 60) {
    return { text: '建议检查', color: 'warning', recommendMain: false };
  }
  return { text: '不建议使用', color: 'error', recommendMain: false };
}

export function parseTranslateTaskOutput(output: unknown): TranslateTaskOutput | null {
  if (!output || typeof output !== 'object') return null;
  return output as TranslateTaskOutput;
}

export function isImageTaskSuccessStatus(status: string): boolean {
  return status === 'success' || status === 'success_with_warnings' || status === 'success_with_review';
}

export function isImageTaskUsableForProduct(status: string): boolean {
  return status === 'success' || status === 'success_with_warnings';
}

export function isImageTaskReviewBeforeProduct(status: string): boolean {
  return status === 'success_with_review';
}

