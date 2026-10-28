import { tr } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { ExclamationCircleFilled, InfoCircleOutlined, UserOutlined } from '@ant-design/icons';
import { Modal } from 'antd';
import { layoutTokens } from '@/constants/layoutTokens';
import SettingsMessage from '../SettingsMessage';
import './disableUserConfirm.less';

function DisableUserContent({ username }: { username: string }) {
  useLanguage();
  return (
      <div className="tm-disable-user-confirm">
        <p className="tm-disable-user-confirm__intro">{tr("请确认是否暂停以下账号的访问权限。")}</p>
        <div className="tm-disable-user-confirm__account" role="group" aria-label={tr("禁用目标账号")}>
          <span className="tm-disable-user-confirm__avatar" aria-hidden="true"><UserOutlined /></span>
          <div className="tm-disable-user-confirm__identity">
            <div className="tm-disable-user-confirm__label">{tr("目标账号")}</div>
            <div className="tm-disable-user-confirm__name">{username}</div>
          </div>
        </div>
        <div>
          <div className="tm-disable-user-confirm__heading">{tr("禁用后将产生以下影响")}</div>
          <ul className="tm-disable-user-confirm__impacts" aria-label={tr("禁用影响")}>
            <li>{tr("账号将无法登录，已登录会话将失效。")}</li>
            <li>{tr("账号数据与历史操作记录会保留。")}</li>
          </ul>
        </div>
        <div className="tm-disable-user-confirm__hint">
          <InfoCircleOutlined className="tm-disable-user-confirm__hint-icon" aria-hidden="true" />
          <span>{tr("需要恢复使用时，可在用户管理中重新启用此账号。")}</span>
        </div>
      </div>
  );
}

export function confirmDisableUser(username: string, onOk: () => void | Promise<void>) {
  Modal.confirm({
    title: <SettingsMessage source="禁用用户" />,
    icon: <ExclamationCircleFilled style={{ color: 'var(--ant-color-error)' }} />,
    centered: true,
    width: layoutTokens.modalWidthSm,
    content: <DisableUserContent username={username} />,
    okText: <SettingsMessage source="确认禁用" />,
    cancelText: <SettingsMessage source="取消" />,
    okButtonProps: { danger: true },
    onOk,
  });
}
