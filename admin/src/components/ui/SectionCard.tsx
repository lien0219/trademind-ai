import type { ReactNode } from 'react';
import { ProCard, type ProCardProps } from '@ant-design/pro-components';
import { Space, Typography } from 'antd';

const { Text } = Typography;

export type SectionCardProps = ProCardProps & {
  description?: ReactNode;
  headerExtra?: ReactNode;
  variant?: 'borderless' | 'outlined' | 'filled';
};

/**
 * 统一区块卡片：左侧标题 + 说明，右侧操作按钮。
 */
export default function SectionCard({
  title,
  description,
  headerExtra,
  children,
  className,
  variant = 'outlined',
  bordered,
  ...rest
}: SectionCardProps) {
  const showBorder = bordered ?? variant === 'outlined';
  return (
    <ProCard
      {...rest}
      bordered={showBorder}
      className={['tm-section-card', className].filter(Boolean).join(' ')}
      title={
        title ? (
          <div className="tm-section-card__head">
            <div className="tm-section-card__head-main">
              <div className="tm-section-card__title">{title}</div>
              {description ? (
                <Text type="secondary" className="tm-section-card__desc">
                  {description}
                </Text>
              ) : null}
            </div>
            {headerExtra ? (
              <Space wrap className="tm-section-card__head-extra">
                {headerExtra}
              </Space>
            ) : null}
          </div>
        ) : undefined
      }
    >
      {children}
    </ProCard>
  );
}
