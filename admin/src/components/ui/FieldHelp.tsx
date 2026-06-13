import type { ReactNode } from 'react';
import { Typography } from 'antd';

const { Text } = Typography;

export type FieldHelpProps = {
  children: ReactNode;
  className?: string;
};

/** 字段下方帮助说明 */
export default function FieldHelp({ children, className }: FieldHelpProps) {
  return (
    <Text type="secondary" className={['tm-field-help', className].filter(Boolean).join(' ')}>
      {children}
    </Text>
  );
}
