import { request } from '@umijs/max';
import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Button, Form, message } from 'antd';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import PasswordInput from '../PasswordInput';
import { passwordError } from '@/utils/password';

const noopMessage = Object.assign(() => {}, {
  then: Promise.resolve(false).then.bind(Promise.resolve(false)),
}) as ReturnType<typeof message.success>;

afterEach(async () => { cleanup(); await act(async () => { message.destroy(); }); vi.restoreAllMocks(); });
beforeEach(() => {
  vi.spyOn(message, 'success').mockImplementation(() => noopMessage);
  vi.mocked(window.matchMedia).mockImplementation((query) => ({
    matches: false, media: query, onchange: null, addListener: vi.fn(), removeListener: vi.fn(),
    addEventListener: vi.fn(), removeEventListener: vi.fn(), dispatchEvent: vi.fn(),
  }));
});

describe('PasswordInput', () => {
  it('generates into the real form and copies exactly the submitted password without requests', async () => {
    const user = userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue();
    const onFinish = vi.fn();
    render(<Form onFinish={onFinish}><Form.Item name="password" label="初始密码"><PasswordInput /></Form.Item><Button htmlType="submit">提交</Button></Form>);
    expect(screen.getByRole('button', { name: '复制密码' })).toBeDisabled();
    await user.click(screen.getByRole('button', { name: '随机生成密码' }));
    const generated = (screen.getByLabelText('初始密码') as HTMLInputElement).value;
    expect(generated).toHaveLength(20);
    expect(passwordError(generated)).toBeUndefined();
    await user.click(screen.getByRole('button', { name: '复制密码' }));
    expect(writeText).toHaveBeenCalledWith(generated);
    await user.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() => expect(onFinish).toHaveBeenCalledWith({ password: generated }));
    expect(request).not.toHaveBeenCalled();
  });

  it('retains the password after clipboard rejection and reports a manual copy fallback', async () => {
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(new Error('denied'));
    const error = vi.spyOn(message, 'error').mockImplementation(() => noopMessage);
    render(<PasswordInput value="Synthetic123!" readOnly />);
    await user.click(screen.getByRole('button', { name: '复制密码' }));
    await waitFor(() => expect(error).toHaveBeenCalledWith('复制失败，请显示密码后手动复制'));
    expect(screen.getByLabelText('一次性临时密码')).toHaveValue('Synthetic123!');
    expect(screen.queryByRole('button', { name: '随机生成密码' })).not.toBeInTheDocument();
    expect(request).not.toHaveBeenCalled();
  });

  it('blocks repeated copying while the clipboard promise is pending', async () => {
    const user = userEvent.setup();
    let release: () => void = () => {};
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockImplementation(() => new Promise<void>((resolve) => { release = resolve; }));
    render(<PasswordInput value="Synthetic123!" />);
    const copy = screen.getByRole('button', { name: '复制密码' });
    try {
      await user.click(copy);
      expect(copy).toBeDisabled();
      await user.click(copy);
      expect(writeText).toHaveBeenCalledTimes(1);
    } finally { await act(async () => { release(); }); }
    await waitFor(() => expect(copy).toBeEnabled());
  });

  it('retains the current value when secure randomness is unavailable', async () => {
    const user = userEvent.setup();
    vi.spyOn(crypto, 'getRandomValues').mockImplementation(() => { throw new Error('unavailable'); });
    const error = vi.spyOn(message, 'error').mockImplementation(() => noopMessage);
    const onChange = vi.fn();
    render(<PasswordInput value="Synthetic123!" onChange={onChange} />);
    await user.click(screen.getByRole('button', { name: '随机生成密码' }));
    expect(onChange).not.toHaveBeenCalled();
    expect(error).toHaveBeenCalledWith('当前浏览器无法安全生成密码，请手动输入');
  });
});
