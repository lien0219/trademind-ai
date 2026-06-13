import type { ReactNode } from 'react';
import { Alert, type AlertProps } from 'antd';

export type ErrorAlertProps = Omit<AlertProps, 'type'> & {
  /** 发生了什么 */
  title: ReactNode;
  /** 用户可以怎么做 */
  actionHint?: ReactNode;
};

/** 用户可见错误提示：发生了什么 + 可以怎么做 */
export default function ErrorAlert({ title, actionHint, description, ...rest }: ErrorAlertProps) {
  const desc = actionHint ?? description;
  return (
    <Alert
      {...rest}
      type="error"
      showIcon
      message={title}
      description={desc}
      className={['tm-error-alert', rest.className].filter(Boolean).join(' ')}
    />
  );
}
