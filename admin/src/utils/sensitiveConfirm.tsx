import { Modal } from 'antd';
import type { ModalFuncProps } from 'antd/es/modal/interface';
import { subscribeLanguage, tr, trFormat } from '@/i18n';

type DialogText = string | (() => string);
const resolveText = (value: DialogText) => typeof value === 'function' ? value() : value;

export type SensitiveConfirmOptions = {
  title: DialogText;
  content: DialogText;
  impacts?: string[] | (() => string[]);
  externalCall?: boolean;
  reversible?: boolean;
  failureHint?: DialogText;
  onOk: () => void | Promise<void>;
  onCancel?: () => void;
};

/** Standard sensitive-operation confirm dialog (F5 audit UX). */
export function confirmSensitiveAction(opts: SensitiveConfirmOptions) {
  let unsubscribe: (() => void) | undefined;
  const modalOptions = (): ModalFuncProps => {
    const lines = [resolveText(opts.content)];
    const impacts = typeof opts.impacts === 'function' ? opts.impacts() : opts.impacts;
    if (impacts?.length) {
      lines.push('', tr('将影响：'), ...impacts.map((item) => `· ${item}`));
    }
    if (opts.externalCall) {
      lines.push('', tr('此操作可能调用外部平台接口。'));
    }
    lines.push('', opts.reversible === false ? tr('此操作通常不可撤销。') : tr('部分操作可在任务中心查看失败并重试。'));
    if (opts.failureHint) {
      lines.push(trFormat('失败后可在：{location}', { location: resolveText(opts.failureHint) }));
    }
    return {
      title: resolveText(opts.title),
      content: <div style={{ whiteSpace: 'pre-wrap' }}>{lines.join('\n')}</div>,
      okText: tr('确认执行'),
      cancelText: tr('取消'),
      onOk: opts.onOk,
      onCancel: opts.onCancel,
      afterClose: () => unsubscribe?.(),
    };
  };
  const modal = Modal.confirm(modalOptions());
  unsubscribe = subscribeLanguage(() => modal.update(modalOptions()));
}
