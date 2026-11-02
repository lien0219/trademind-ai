import { render, screen } from '@testing-library/react';
import type { CSSProperties, ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import SectionCard from '../SectionCard';

// Observe the public bodyStyle boundary; browser regressions cover the real
// ProCard DOM and the stylesheet cascade.
vi.mock('@ant-design/pro-components', () => ({
  ProCard: ({
    title,
    bodyStyle,
    children,
  }: {
    title?: ReactNode;
    bodyStyle?: CSSProperties;
    children?: ReactNode;
  }) => (
    <section>
      {title}
      <div style={bodyStyle}>{children}</div>
    </section>
  ),
}));

describe('SectionCard body spacing', () => {
  it.each([
    { compact: false, padding: '24px' },
    { compact: true, padding: '20px' },
  ])('keeps readable content padding when compact=$compact', ({ compact, padding }) => {
    render(
      <SectionCard title="配置" compact={compact}>
        <p>表单内容</p>
      </SectionCard>,
    );

    expect(screen.getByText('表单内容').parentElement).toHaveStyle({ padding });
  });

  it('preserves caller body spacing instead of replacing it with the default', () => {
    render(
      <SectionCard compact bodyStyle={{ padding: '8px 12px' }}>
        <p>嵌入式内容</p>
      </SectionCard>,
    );

    expect(screen.getByText('嵌入式内容').parentElement).toHaveStyle({
      padding: '8px 12px',
    });
  });

  it('lets nested cards own their spacing independently of a compact parent', () => {
    render(
      <SectionCard compact>
        <p>外层内容</p>
        <SectionCard>
          <p>常规内层内容</p>
        </SectionCard>
        <SectionCard compact bodyStyle={{ padding: 8 }}>
          <p>嵌入式内层内容</p>
        </SectionCard>
      </SectionCard>,
    );

    expect(screen.getByText('外层内容').parentElement).toHaveStyle({ padding: '20px' });
    expect(screen.getByText('常规内层内容').parentElement).toHaveStyle({ padding: '24px' });
    expect(screen.getByText('嵌入式内层内容').parentElement).toHaveStyle({ padding: '8px' });
  });
});
