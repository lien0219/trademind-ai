import type { DashboardRecentItem } from '@/services/dashboard';
import { aiTaskTypeLabel } from '@/constants/aiPrompts';
import { COLLECT_TASK_STATUS } from '@/constants/status';
import { taskTypeLabel, translateLayoutWarningLabel } from '@/services/imageTasks';

export const BATCH_OPERATION_LABEL: Record<string, string> = {
  title_optimize: '批量标题优化',
  description_generate: '批量描述生成',
  image_remove_background: '批量去背景',
  image_generate_scene: '批量场景图',
  image_replace_background: '批量换背景',
  image_batch_generate_main: '批量主图生成',
  image_score: '批量图片评分',
  image_select_best_main: '批量自动选主图',
};

export const RECENT_SOURCE_LABEL: Record<string, string> = {
  '1688': '1688',
  pinduoduo: '拼多多',
  pdd: '拼多多',
  taobao: '淘宝',
  aliexpress: '速卖通',
  custom: '自定义链接',
  manual: '手动创建',
};

export const RECENT_STATUS_COLOR: Record<string, string> = {
  已完成: 'success',
  '成功（有警告）': 'warning',
  部分成功: 'warning',
  失败: 'error',
  处理中: 'processing',
  等待中: 'default',
  重试中: 'warning',
  已取消: 'default',
};

/** Map backend recent status (Chinese label or raw code) to display label. */
export function recentStatusLabel(status: string | undefined): string {
  const s = status?.trim() || '';
  if (!s) return '—';
  const mapped = COLLECT_TASK_STATUS[s as keyof typeof COLLECT_TASK_STATUS];
  if (mapped) return mapped.text;
  return s;
}

/** Map recent status to Ant Design Tag color. */
export function recentStatusColor(status: string | undefined): string | undefined {
  const s = status?.trim() || '';
  if (!s) return undefined;
  if (RECENT_STATUS_COLOR[s]) return RECENT_STATUS_COLOR[s];
  const mapped = COLLECT_TASK_STATUS[s as keyof typeof COLLECT_TASK_STATUS];
  if (mapped) return mapped.color;
  return 'default';
}

/** Map translate warning subtitle (machine code or human text) for recent activity rows. */
export function recentTranslateWarningSubtitle(subtitle: string | undefined): string | undefined {
  const s = subtitle?.trim();
  if (!s) return undefined;
  const mapped = translateLayoutWarningLabel(s);
  return mapped !== s ? mapped : s;
}

const IMAGE_TASK_RAW = new Set([
  'remove_background',
  'replace_background',
  'generate_scene',
  'remove_watermark',
  'remove_logo',
  'remove_badge',
  'remove_qrcode',
  'cleanup',
  'enhance_detail',
  'upscale',
  'generate_marketing',
  'generate_main_image',
  'batch_generate_main',
  'score_image',
  'select_best_main',
  'translate_image_text',
  'resize',
  'enhance',
]);

/** Map backend recent row to display title / subtitle (handles legacy raw keys). */
export function formatRecentItem(item: DashboardRecentItem): { title: string; subtitle?: string } {
  let title = (item.title || '').trim();
  let subtitle = (item.subtitle || '').trim();

  switch (item.type) {
    case 'image_task':
      if (IMAGE_TASK_RAW.has(title)) {
        title = taskTypeLabel(title);
      }
      if (subtitle && (IMAGE_TASK_RAW.has(subtitle) || subtitle === item.title)) {
        subtitle = '';
      }
      break;
    case 'ai_task':
      if (title && aiTaskTypeLabel(title) !== title) {
        title = aiTaskTypeLabel(title);
      }
      break;
    case 'ai_batch':
      if (subtitle) {
        subtitle = BATCH_OPERATION_LABEL[subtitle] ?? subtitle;
      }
      break;
    case 'collect':
      if (subtitle) {
        subtitle = RECENT_SOURCE_LABEL[subtitle.toLowerCase()] ?? subtitle;
      }
      break;
    default:
      break;
  }

  if (subtitle && subtitle === title) {
    subtitle = '';
  }

  return { title: title || '—', subtitle: subtitle || undefined };
}
