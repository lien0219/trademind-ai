import { ExclamationCircleFilled, InfoCircleOutlined, UserOutlined } from '@ant-design/icons';
import { Modal } from 'antd';
import { layoutTokens } from '@/constants/layoutTokens';
import './disableUserConfirm.less';

export function confirmDisableUser(username: string, onOk: () => void | Promise<void>) {
  Modal.confirm({
    title: '禁用用户',
    icon: <ExclamationCircleFilled style={{ color: 'var(--ant-color-error)' }} />,
    centered: true,
    width: layoutTokens.modalWidthSm,
    content: (
      <div className="tm-disable-user-confirm">
        <p className="tm-disable-user-confirm__intro">请确认是否暂停以下账号的访问权限。</p>
        <div className="tm-disable-user-confirm__account" role="group" aria-label="禁用目标账号">
          <span className="tm-disable-user-confirm__avatar" aria-hidden="true"><UserOutlined /></span>
          <div className="tm-disable-user-confirm__identity">
            <div className="tm-disable-user-confirm__label">目标账号</div>
            <div className="tm-disable-user-confirm__name">{username}</div>
          </div>
        </div>
        <div>
          <div className="tm-disable-user-confirm__heading">禁用后将产生以下影响</div>
          <ul className="tm-disable-user-confirm__impacts" aria-label="禁用影响">
            <li>账号将无法登录，已登录会话将失效。</li>
            <li>账号数据与历史操作记录会保留。</li>
          </ul>
        </div>
        <div className="tm-disable-user-confirm__hint">
          <InfoCircleOutlined className="tm-disable-user-confirm__hint-icon" aria-hidden="true" />
          <span>需要恢复使用时，可在用户管理中重新启用此账号。</span>
        </div>
      </div>
    ),
    okText: '确认禁用',
    cancelText: '取消',
    okButtonProps: { danger: true },
    onOk,
  });
}
