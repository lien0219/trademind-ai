/** Safe profile_key segment for persistent context directory names. */
export function sanitizeProfileKey(raw: string): string {
  const key = raw.trim();
  if (!key || key.length > 128) {
    throw new Error('INVALID_PROFILE_KEY:empty_or_too_long');
  }
  if (!/^[a-zA-Z0-9_-]+$/.test(key)) {
    throw new Error('INVALID_PROFILE_KEY:invalid_characters');
  }
  if (key === '1688' || key === 'pinduoduo' || key.startsWith('..')) {
    throw new Error('INVALID_PROFILE_KEY:reserved');
  }
  return key;
}
