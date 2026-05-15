import type { RunTimeLayoutConfig } from '@umijs/max';

export async function getInitialState(): Promise<Record<string, never>> {
  return {};
}

export const layout: RunTimeLayoutConfig = () => ({
  logo: (
    <span style={{ fontWeight: 600, fontSize: 16, letterSpacing: 0.5 }}>
      <span style={{ color: '#1677ff' }}>贸灵</span>
      <span style={{ color: '#262626' }}> TradeMind</span>
    </span>
  ),
  menu: {
    locale: false,
  },
});
