import { afterEach, describe, expect, it } from 'vitest';
import { setLanguage } from '@/i18n';
import { hasTranslation, supportedLanguages } from '@/i18n/dictionaries';
import {
  AI_CONTEXT_PLATFORM_OPTIONS,
  AI_LANGUAGE_OPTIONS,
  AI_TARGET_PLATFORM_OPTIONS,
  AI_TONE_OPTIONS,
  AI_TASK_TYPE_LABEL,
  AI_PROMPT_CODE_LABEL,
  AI_TASK_TYPE_OPTIONS,
  aiTaskTypeLabel,
  aiPromptCodeLabel,
} from '../aiPrompts';

afterEach(() => setLanguage('zh-CN'));

describe('AI prompt platform options', () => {
  it('keeps the existing display names as request values', () => {
    expect(AI_TARGET_PLATFORM_OPTIONS.map(({ value }) => value)).toEqual([
      '跨境通用',
      '抖店',
      'TikTok Shop',
      'Shopee',
      'Lazada',
      'Amazon',
    ]);
  });

  it('uses Chinese labels while preserving stable language and tone values', () => {
    expect(AI_LANGUAGE_OPTIONS).toContainEqual({ value: 'en', label: '英语' });
    expect(AI_LANGUAGE_OPTIONS).toContainEqual({ value: 'zh', label: '中文' });
    expect(AI_TONE_OPTIONS).toContainEqual({ value: 'professional', label: '专业稳健' });
    expect(AI_TONE_OPTIONS).toContainEqual({ value: 'friendly', label: '亲切自然' });
  });

  it('keeps internal platform codes for AI conversation context', () => {
    expect(AI_CONTEXT_PLATFORM_OPTIONS).toContainEqual({ value: 'douyin_shop', label: '抖店' });
    expect(AI_CONTEXT_PLATFORM_OPTIONS).toContainEqual({ value: 'tiktok', label: 'TikTok Shop' });
    expect(AI_CONTEXT_PLATFORM_OPTIONS).toContainEqual({ value: 'manual', label: '手动' });
  });

  it('localizes known display labels without changing task codes or raw unknown identifiers', () => {
    const codes = AI_TASK_TYPE_OPTIONS.map(option => option.value);
    setLanguage('en-US');
    expect(aiTaskTypeLabel('title_optimize')).toBe('Title optimization');
    expect(aiPromptCodeLabel('product_description_generate')).toBe('Product description generation');
    expect(aiTaskTypeLabel('设置')).toBe('设置');
    expect(aiPromptCodeLabel('订单')).toBe('订单');
    expect(AI_TASK_TYPE_OPTIONS.map(option => option.value)).toEqual(codes);
  });

  it.each(supportedLanguages)('covers every known task and template label in %s', language => {
    for (const label of [...Object.values(AI_TASK_TYPE_LABEL), ...Object.values(AI_PROMPT_CODE_LABEL)]) {
      expect(hasTranslation(language, label)).toBe(true);
    }
  });
});
