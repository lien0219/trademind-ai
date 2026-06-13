import type { ReactNode } from 'react';

export type FormGridProps = {
  children: ReactNode;
  className?: string;
};

/** 两列表单栅格，1100px 以下自动单列 */
export function FormGrid({ children, className }: FormGridProps) {
  return <div className={['tm-form-grid', className].filter(Boolean).join(' ')}>{children}</div>;
}

/** 占满整行的表单项 */
export function FormGridFull({ children, className }: FormGridProps) {
  return (
    <div className={['tm-form-grid__item', 'tm-form-grid__item--full', className].filter(Boolean).join(' ')}>
      {children}
    </div>
  );
}

/** 普通半宽表单项 */
export function FormGridItem({ children, className }: FormGridProps) {
  return <div className={['tm-form-grid__item', className].filter(Boolean).join(' ')}>{children}</div>;
}
