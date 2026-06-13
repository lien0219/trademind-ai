import type { ReactNode } from 'react';
import { PageContainer, type PageContainerProps } from '@ant-design/pro-components';
import { layoutTokens } from '@/constants/layoutTokens';

export type TmPageContainerProps = PageContainerProps & {
  /** 页面内容最大宽度，默认 settings 1440 */
  contentMaxWidth?: number;
};

/**
 * 统一页面容器：标题 + 说明分行，内边距与最大宽度一致。
 */
export default function TmPageContainer({
  title,
  subTitle,
  contentMaxWidth = layoutTokens.settingsMaxWidth,
  children,
  className,
  ...rest
}: TmPageContainerProps) {
  return (
    <PageContainer
      {...rest}
      className={['tm-page-container', className].filter(Boolean).join(' ')}
      title={title}
      subTitle={subTitle}
      contentStyle={{
        maxWidth: contentMaxWidth,
        marginInline: 'auto',
        paddingInline: layoutTokens.pagePaddingX,
        paddingBlock: `${layoutTokens.pagePaddingY}px ${layoutTokens.pagePaddingBottom}px`,
        ...(rest.contentStyle as object),
      }}
    >
      {children}
    </PageContainer>
  );
}

export function TmPageHeaderExtra({ children }: { children: ReactNode }) {
  return <div className="tm-page-header-extra">{children}</div>;
}
