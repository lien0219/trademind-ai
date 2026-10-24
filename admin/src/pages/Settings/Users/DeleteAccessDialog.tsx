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
  const label = target?.kind === 'role' ? '角色' : '用户';
  return <Modal title={`删除${label}`} open={target !== null} centered width={layoutTokens.modalWidthSm}
    onCancel={onCancel} onOk={onConfirm} confirmLoading={submitting} destroyOnHidden
    closable={!submitting} maskClosable={!submitting} keyboard={!submitting}
    okText={`确认删除${label}`} cancelText="取消" cancelButtonProps={{ disabled: submitting }}
    okButtonProps={{ danger: true, disabled: submitting, 'aria-label': `确认删除${label}` }}>
    <Alert type="warning" showIcon message={<span style={{ overflowWrap: 'anywhere' }}>{`将删除「${target?.name || ''}」`}</span>}
      description={target?.kind === 'role'
        ? '删除后无法再向用户分配此角色；角色的历史操作记录会保留。'
        : '该账号将无法登录，现有登录凭据及店铺授权会失效；历史业务数据与操作记录会保留。'} />
    <p style={{ marginTop: 16, marginBottom: 0, color: 'var(--ant-color-text-secondary)' }}>删除后无法在此页面恢复，请确认后操作。</p>
  </Modal>;
}
