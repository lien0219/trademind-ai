import { act, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { setLanguage } from '@/i18n';
import SettingsMessage from '../SettingsMessage';

describe('SettingsMessage', () => {
  afterEach(() => act(() => setLanguage('zh-CN')));

  it('updates static modal copy when the locale changes without remounting', () => {
    setLanguage('zh-CN');
    const { container } = render(<SettingsMessage source="删除{value0}" values={{ value0: 'demo' }} />);
    const node = container.firstChild;
    expect(screen.getByText('删除demo')).toBeInTheDocument();
    act(() => setLanguage('en-US'));
    expect(node).toBe(container.firstChild);
    expect(screen.getByText('Delete demo')).toBeInTheDocument();
  });
});
