import { Modal } from 'antd';
import type { ModalFuncProps } from 'antd/es/modal/interface';

export type SensitiveConfirmOptions = {
  title: string;
  content: string;
  impacts?: string[];
  externalCall?: boolean;
  reversible?: boolean;
  failureHint?: string;
  onOk: () => void | Promise<void>;
};

/** Standard sensitive-operation confirm dialog (F5 audit UX). */
export function confirmSensitiveAction(opts: SensitiveConfirmOptions) {
  const lines = [opts.content];
  if (opts.impacts?.length) {
    lines.push('', '将影响：', ...opts.impacts.map((x) => `· ${x}`));
  }
  if (opts.externalCall) {
    lines.push('', '此操作可能调用外部平台接口。');
  }
  lines.push('', opts.reversible === false ? '此操作通常不可撤销。' : '部分操作可在任务中心查看失败并重试。');
  if (opts.failureHint) {
    lines.push(`失败后可在：${opts.failureHint}`);
  }
  const modalOpts: ModalFuncProps = {
    title: opts.title,
    content: <div style={{ whiteSpace: 'pre-wrap' }}>{lines.join('\n')}</div>,
    okText: '确认执行',
    cancelText: '取消',
    onOk: opts.onOk,
  };
  Modal.confirm(modalOpts);
}
