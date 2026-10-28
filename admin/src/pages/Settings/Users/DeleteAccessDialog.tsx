import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { Alert, Modal } from 'antd';
import { layoutTokens } from '@/constants/layoutTokens';

export type DeleteAccessTarget = { kind: 'user' | 'role'; id: string; name: string };

type Props = {
  target: DeleteAccessTarget | null;
  submitting: boolean;
  onCancel: () => void;
  onConfirm: () => void;
};

export default function DeleteAccessDialog({ target, submitting, onCancel, onConfirm }: Props) {
  useLanguage();
  const label = target?.kind === 'role' ? tr("角色") : tr("用户");
  return <Modal title={trFormat("删除{value0}", { value0: label })} open={target !== null} centered width={layoutTokens.modalWidthSm}
    onCancel={onCancel} onOk={onConfirm} confirmLoading={submitting} destroyOnHidden
    closable={!submitting} maskClosable={!submitting} keyboard={!submitting}
    okText={trFormat("确认删除{value0}", { value0: label })} cancelText={tr("取消")} cancelButtonProps={{ disabled: submitting }}
    okButtonProps={{ danger: true, disabled: submitting, 'aria-label': trFormat("确认删除{value0}", { value0: label }) }}>
    <Alert type="warning" showIcon message={<span style={{ overflowWrap: 'anywhere' }}>{trFormat("将删除「{value0}」", { value0: target?.name || '' })}</span>}
      description={target?.kind === 'role'
        ? tr("删除后无法再向用户分配此角色；角色的历史操作记录会保留。")
        : tr("该账号将无法登录，现有登录凭据及店铺授权会失效；历史业务数据与操作记录会保留。")} />
    <p style={{ marginTop: 16, marginBottom: 0, color: 'var(--ant-color-text-secondary)' }}>{tr("删除后无法在此页面恢复，请确认后操作。")}</p>
  </Modal>;
}
