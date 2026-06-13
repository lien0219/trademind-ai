import type { ReactNode } from 'react';
import { Space } from 'antd';

export type ActionBarProps = {
  children: ReactNode;
  align?: 'left' | 'right';
  className?: string;
  sticky?: boolean;
};

/** 表单/页面底部操作栏 */
export default function ActionBar({ children, align = 'left', className, sticky }: ActionBarProps) {
  return (
    <div
      className={[
        'tm-action-bar',
        align === 'right' ? 'tm-action-bar--right' : '',
        sticky ? 'tm-action-bar--sticky' : '',
        className,
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <Space wrap className="tm-action-space">
        {children}
      </Space>
    </div>
  );
}
