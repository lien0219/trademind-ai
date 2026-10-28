import { describe, expect, it } from 'vitest';

import { supportedLanguages, hasTranslation } from '@/i18n/dictionaries';
import { ACCESS_STATUS_LABELS, accessStatusHint, accessStatusLabel } from '@/constants/collectAccess';
import { COLLECT_FIELD_LABELS, mapCollectFieldLabel, mapCollectorErrorCodeDetail, mapCollectorErrorCodeLabel } from '@/constants/collectErrors';
import { collectProviderStatusPresentation, CUSTOM_COLLECT_FEATURE_LABEL, NO_COLLECT_RULE_MESSAGE } from '@/utils/collectProviderStatus';
import {
  COLLECT_SETTINGS_PROVIDER_OPTIONS,
  collectSettingsConfigButtonLabel,
} from '@/utils/collectSettingsProvider';

function expectCatalogKey(source: string) {
  for (const language of supportedLanguages) {
    if (language === 'zh-CN') continue;
    expect(hasTranslation(language, source), `${language}: ${source}`).toBe(true);
  }
}

describe('shared collector labels', () => {
  it('keeps shared collector source keys covered by the six-language catalog', () => {
    const errorCodes = [
      'LOGIN_REQUIRED', 'PAGE_BLOCKED_OR_VERIFY_REQUIRED', 'CUSTOM_RULE_MISSING', 'CUSTOM_RULE_INVALID',
      'PARSE_FAILED_TITLE_MISSING', 'PARSE_FAILED_IMAGE_MISSING', 'PARSE_FAILED', 'NAVIGATION_FAILED',
      'TIMEOUT', 'PROFILE_NOT_FOUND', 'PROFILE_LOGIN_REQUIRED', 'HEADED_BROWSER_REQUIRED', 'AI_RULE_INVALID',
      'PRODUCT_NOT_FOUND', 'MAIN_IMAGES_EMPTY', 'ACCESS_DENIED', 'PRICE_NOT_FOUND', 'SKU_INCOMPLETE',
      'DETAIL_IMAGES_INCOMPLETE', 'UNKNOWN_COLLECT_ERROR', 'INVALID_URL', 'UNSUPPORTED_PINDUODUO_URL',
      'UNSUPPORTED_TAOBAO_URL', 'TITLE_NOT_FOUND', 'ATTRIBUTES_EMPTY', 'STOCK_UNKNOWN', 'WECHAT_AUTH_REQUIRED',
      'APP_REDIRECT',
    ] as const;
    const keys = [
      ...Object.values(ACCESS_STATUS_LABELS).map((label) => label.text),
      accessStatusLabel('unknown').text,
      accessStatusHint('login_required'),
      accessStatusHint('verify_required'),
      ...Object.values(COLLECT_FIELD_LABELS),
      mapCollectFieldLabel('title'),
      ...errorCodes.map((code) => mapCollectorErrorCodeLabel(code)),
      ...errorCodes.flatMap((code) => [
        mapCollectorErrorCodeDetail(code),
        mapCollectorErrorCodeDetail(code, 'pinduoduo'),
        mapCollectorErrorCodeDetail(code, 'taobao_tmall'),
      ]),
      ...Object.values(CUSTOM_COLLECT_FEATURE_LABEL),
      NO_COLLECT_RULE_MESSAGE,
      ...COLLECT_SETTINGS_PROVIDER_OPTIONS.map((option) => option.label),
      collectSettingsConfigButtonLabel('planned'),
      collectSettingsConfigButtonLabel('available'),
      ...[
        collectProviderStatusPresentation('1688', 'available'),
        collectProviderStatusPresentation('custom', 'beta'),
        collectProviderStatusPresentation('1688', 'beta'),
        collectProviderStatusPresentation('1688', 'planned'),
        collectProviderStatusPresentation('1688', 'disabled'),
      ].map((status) => status.text),
    ];

    for (const key of keys) {
      if (key) expectCatalogKey(key);
    }
  });

  it('preserves collector status presentation semantics', () => {
    expect(collectProviderStatusPresentation('custom', 'beta')).toEqual({ text: '基础可用', color: 'blue' });
    expect(collectProviderStatusPresentation('1688', 'available')).toEqual({ text: '已可用', color: 'success' });
    expect(collectProviderStatusPresentation('1688', 'planned')).toEqual({ text: '规划中', color: 'default' });
  });
});
