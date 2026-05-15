import logoUrl from '@/assets/logo.png';

/** 站内品牌图统一用 `@/assets/logo.png`。浏览器标签图标走 Umi 约定：`src/favicon.png`（应与 logo 同步，换标时复制覆盖即可）。 */

type BrandLogoProps = {
  /** CSS height in px; width follows aspect ratio. */
  height?: number;
  className?: string;
  /** Empty string when decorative (text label next to logo). */
  alt?: string;
};

export default function BrandLogo({ height = 28, className, alt = '' }: BrandLogoProps) {
  return (
    <img
      src={logoUrl}
      alt={alt}
      draggable={false}
      className={className}
      style={{
        height,
        width: 'auto',
        maxWidth: '100%',
        objectFit: 'contain',
        display: 'block',
        flexShrink: 0,
        verticalAlign: 'middle',
      }}
    />
  );
}
