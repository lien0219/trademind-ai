import type { CSSProperties, ReactNode } from 'react';
import { Collapse, Typography } from 'antd';
import { tr } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';

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
  label,
  defaultOpen = false,
  className,
  style,
}: TechnicalDetailsProps) {
  useLanguage();
  return (
    <Collapse
      className={['tm-technical-details', className].filter(Boolean).join(' ')}
      style={style}
      items={[
        {
          key: 'technical',
          label: <Text type="secondary">{label ?? tr('技术详情')}</Text>,
          children: <div className="tm-technical-details__body">{children}</div>,
        },
      ]}
      defaultActiveKey={defaultOpen ? ['technical'] : []}
    />
  );
}
