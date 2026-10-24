import { describe, expect, it, vi } from 'vitest';
import { generatePassword, passwordError } from '../password';

describe('console password policy', () => {
  it.each(['Test-password-123', 'simplepass123', `${'a'.repeat(71)}1`])('accepts a valid ASCII password', (value) => {
    expect(passwordError(value)).toBeUndefined();
  });

  it.each(['', 'Pass123!', `${'a'.repeat(72)}1`, 'Password123中文', 'Password123Ａ', 'Password123😀', 'Password 1234', 'Password123\n', 'Password123\x7f', '123456789012!', 'long-password!'])('rejects an invalid password', (value) => {
    expect(passwordError(value)).toBeTruthy();
  });

  it('generates 20 characters with letters, digits and symbols using Web Crypto', () => {
    const secureRandom = vi.spyOn(crypto, 'getRandomValues');
    const password = generatePassword();
    expect(secureRandom).toHaveBeenCalled();
    expect(password).toHaveLength(20);
    expect(password).toMatch(/[A-Z]/);
    expect(password).toMatch(/[a-z]/);
    expect(password).toMatch(/[0-9]/);
    expect(password).toMatch(/[^A-Za-z0-9]/);
    expect(passwordError(password)).toBeUndefined();
  });

  it('fails closed if secure randomness is unavailable', () => {
    vi.spyOn(crypto, 'getRandomValues').mockImplementation(() => { throw new Error('unavailable'); });
    expect(() => generatePassword()).toThrow();
  });
});
