import { Modal } from 'antd';
import type { ModalFuncProps } from 'antd';
import { afterEach, expect, it, vi } from 'vitest';
import { confirmSensitiveAction } from '../sensitiveConfirm';
import { setLanguage, tr, trFormat } from '@/i18n';

afterEach(() => { setLanguage('zh-CN'); vi.restoreAllMocks(); });

it('updates an open confirmation without changing its business action or values', () => {
  const onOk = vi.fn();
  const onCancel = vi.fn();
  const update = vi.fn();
  const destroy = vi.fn();
  const confirm = vi.spyOn(Modal, 'confirm').mockReturnValue({ update, destroy });
  const name = '设置 {action}';
  confirmSensitiveAction({
    title: () => trFormat('删除企业：{name}', { name }),
    content: () => tr('删除后将从默认企业列表移除'),
    impacts: () => [trFormat('企业：{name}', { name })],
    reversible: false,
    onOk, onCancel,
  });
  const initial = confirm.mock.calls[0][0];
  expect(initial.title).toBe(`删除企业：${name}`);
  setLanguage('en-US');
  const changed = update.mock.calls[0][0] as ModalFuncProps;
  expect(changed.title).toBe(`Delete organization: ${name}`);
  expect(changed.okText).toBe('Confirm operation');
  expect(changed.cancelText).toBe('Cancel');
  expect(changed.onOk).toBe(onOk);
  expect(changed.onCancel).toBe(onCancel);
  expect(onOk).not.toHaveBeenCalled();
  expect(onCancel).not.toHaveBeenCalled();
  initial.afterClose?.();
  setLanguage('ja-JP');
  expect(update).toHaveBeenCalledTimes(1);
});
