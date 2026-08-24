import type { CSSProperties, ReactNode } from 'react';
import { PageContainer, type PageContainerProps } from '@ant-design/pro-components';
import { Space } from 'antd';
import { layoutTokens } from '@/constants/layoutTokens';

export type TmPageContainerProps = PageContainerProps & {
  /** 页面内容首选最大宽度；流式布局会在更宽视口继续扩展，避免两侧留白过大。 */
  contentMaxWidth?: number;
  /** 是否限制超宽视口的外层留白；窄表单等固定内容轨道可显式关闭。 */
  fluid?: boolean;
  /** 关闭内容区左右 padding，仅用于嵌入式特殊场景。 */
  padded?: boolean;
  /** 页面内容包裹层样式。 */
  contentWrapperStyle?: CSSProperties;
};

/**
 * 统一页面容器：标题 + 说明分行，内边距与最大宽度一致。
 */
export default function TmPageContainer({
  title,
  subTitle,
  contentMaxWidth = layoutTokens.pageMaxWidth,
  fluid = true,
  padded = true,
  contentWrapperStyle,
  children,
  className,
  ...rest
}: TmPageContainerProps) {
  const wrapperStyle: CSSProperties = {
    '--tm-page-content-max-width': `${contentMaxWidth}px`,
    ...contentWrapperStyle,
  } as CSSProperties;

  return (
    <PageContainer
      {...rest}
      className={['tm-page-container', fluid ? 'tm-page-container--fluid' : '', className]
        .filter(Boolean)
        .join(' ')}
      style={{
        '--tm-page-content-max-width': `${contentMaxWidth}px`,
        ...rest.style,
      } as CSSProperties}
      title={title}
      subTitle={subTitle}
    >
      <div
        className={['tm-page-container__content', padded ? '' : 'tm-page-container__content--unpadded']
          .filter(Boolean)
          .join(' ')}
        style={wrapperStyle}
      >
        {children}
      </div>
    </PageContainer>
  );
}

export function TmPageHeaderExtra({ children }: { children: ReactNode }) {
  return (
    <Space className="tm-page-header-extra" size={8} wrap>
      {children}
    </Space>
  );
}
