import { hasTranslation } from './dictionaries';
import { readLanguage, tr } from './index';

const errorMessages: Record<number, string> = {
  40001: '参数无效，请检查输入',
  40101: '请重新登录',
  40301: '没有执行此操作的权限',
  40302: '没有执行此操作的权限',
  40303: '没有执行此操作的权限',
  40304: '没有执行此操作的权限',
  40305: '没有执行此操作的权限',
  40306: '没有执行此操作的权限',
  40401: '未找到请求的记录',
  40901: '数据已被更新，请刷新后重试',
  50000: '操作失败，请稍后重试',
  50301: '服务暂不可用，请稍后重试',
};

/** Preserve raw server details separately; localized UI must not leak them. */
export function localizeApiError(code: number, rawMessage: string): string {
  const language = readLanguage();
  if (language === 'zh-CN') return rawMessage || tr(errorMessages[code] || '操作失败，请稍后重试');
  if (hasTranslation(language, rawMessage)) return tr(rawMessage);
  return tr(errorMessages[code] || '操作失败，请稍后重试');
}
