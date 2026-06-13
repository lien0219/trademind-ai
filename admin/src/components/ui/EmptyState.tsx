import type { ReactNode } from 'react';
import { Button, Empty } from 'antd';
import { history } from '@umijs/max';

export type EmptyStateProps = {
  title: string;
  description?: ReactNode;
  actionLabel?: string;
  actionPath?: string;
  onAction?: () => void;
  className?: string;
};

/** 带下一步引导的空状态 */
export default function EmptyState({
  title,
  description,
  actionLabel,
  actionPath,
  onAction,
  className,
}: EmptyStateProps) {
  const handleAction = () => {
    if (onAction) {
      onAction();
      return;
    }
    if (actionPath) history.push(actionPath);
  };

  return (
    <div className={['tm-empty-state', className].filter(Boolean).join(' ')}>
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description={
          <div className="tm-empty-state__body">
            <div className="tm-empty-state__title">{title}</div>
            {description ? <div className="tm-empty-state__desc">{description}</div> : null}
          </div>
        }
      >
        {actionLabel ? (
          <Button type="primary" onClick={handleAction}>
            {actionLabel}
          </Button>
        ) : null}
      </Empty>
    </div>
  );
}
