export const PASSWORD_HINT = '12–72 位，需包含英文字母和数字，可使用英文符号；不支持中文和空格。';

export function passwordError(value: string): string | undefined {
  if (!value) return '请输入密码';
  if (/[^\x21-\x7e]/.test(value)) return '密码仅支持英文字母、数字和英文符号，不能包含中文或空格';
  if (value.length < 12 || value.length > 72) return '密码长度须为 12–72 位';
  if (!/[A-Za-z]/.test(value) || !/[0-9]/.test(value)) return '密码须同时包含英文字母和数字';
  return undefined;
}

function randomIndex(size: number): number {
  const limit = 0x100000000 - (0x100000000 % size);
  const random = new Uint32Array(1);
  for (let attempt = 0; attempt < 128; attempt += 1) {
    crypto.getRandomValues(random);
    if (random[0] < limit) return random[0] % size;
  }
  throw new Error('安全随机数不可用');
}

/** Generate in memory with Web Crypto; never fall back to Math.random. */
export function generatePassword(): string {
  const groups = ['ABCDEFGHJKLMNPQRSTUVWXYZ', 'abcdefghijkmnopqrstuvwxyz', '23456789', '!@#$%&*+-=?'];
  const alphabet = groups.join('');
  const chars = groups.map((group) => group[randomIndex(group.length)]);
  while (chars.length < 20) chars.push(alphabet[randomIndex(alphabet.length)]);
  for (let index = chars.length - 1; index > 0; index -= 1) {
    const other = randomIndex(index + 1);
    [chars[index], chars[other]] = [chars[other], chars[index]];
  }
  return chars.join('');
}

export async function copyPassword(value: string): Promise<void> {
  if (!value || !navigator.clipboard?.writeText) throw new Error('剪贴板不可用');
  await navigator.clipboard.writeText(value);
}
