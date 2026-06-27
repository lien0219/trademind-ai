import type { CSSProperties, ReactNode } from 'react';
import { Collapse, Typography } from 'antd';

const { Text } = Typography;

export type TechnicalDetailsProps = {
  children: ReactNode;
  label?: string;
  defaultOpen?: boolean;
  className?: string;
  style?: CSSProperties;
};

/** 技术详情折叠区（默认收起） */
export default function TechnicalDetails({
  children,
  label = '技术详情',
  defaultOpen = false,
  className,
  style,
}: TechnicalDetailsProps) {
  return (
    <Collapse
      className={['tm-technical-details', className].filter(Boolean).join(' ')}
      style={style}
      items={[
        {
          key: 'technical',
          label: <Text type="secondary">{label}</Text>,
          children: <div className="tm-technical-details__body">{children}</div>,
        },
      ]}
      defaultActiveKey={defaultOpen ? ['technical'] : []}
    />
  );
}
