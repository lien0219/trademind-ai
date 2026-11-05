import { tr } from '@/i18n';
import { TASK_FAILURE_SEVERITY_OPTIONS } from '@/constants/taskCenter';

/** 告警最低等级下拉（存库仍为 low/medium/high/critical） */
export const ALERT_SEVERITY_OPTIONS = TASK_FAILURE_SEVERITY_OPTIONS;

/** 常用时区（存库仍为 IANA 标识） */
export const SYSTEM_TIMEZONE_OPTIONS = [
  { get label() { return tr('中国标准时间（Asia/Shanghai）'); }, value: 'Asia/Shanghai' },
  { get label() { return tr('香港时间（Asia/Hong_Kong）'); }, value: 'Asia/Hong_Kong' },
  { get label() { return tr('新加坡时间（Asia/Singapore）'); }, value: 'Asia/Singapore' },
  { get label() { return tr('东京时间（Asia/Tokyo）'); }, value: 'Asia/Tokyo' },
  { label: 'UTC', value: 'UTC' },
  { get label() { return tr('伦敦时间（Europe/London）'); }, value: 'Europe/London' },
  { get label() { return tr('美国东部（America/New_York）'); }, value: 'America/New_York' },
  { get label() { return tr('美国太平洋（America/Los_Angeles）'); }, value: 'America/Los_Angeles' },
];

/** 任务中心 · 分类告警开关 */
export const TASK_ALERT_CATEGORY_TOGGLES = [
  {
    name: 'alert_on_platform_permission',
    get label() { return tr('平台权限失败'); },
    get extra() { return tr('店铺授权失效、权限不足等场景自动生成站内告警'); },
  },
  {
    name: 'alert_on_platform_config',
    get label() { return tr('平台配置不完整'); },
    get extra() { return tr('开放平台应用参数缺失或校验未通过时告警'); },
  },
  {
    name: 'alert_on_inventory_mapping_missing',
    get label() { return tr('库存映射缺失'); },
    get extra() { return tr('库存同步因规格映射缺失失败时告警'); },
  },
  {
    name: 'alert_on_worker_lease_expired',
    get label() { return tr('后台任务执行超时'); },
    get extra() { return tr('任务租约过期、后台任务进程长时间未续租时告警'); },
  },
] as const;
