/**
 * Declarative rule shape passed from Go → Collector options.rule (JSON).
 */
export type CustomAttrKind =
  | 'text'
  | 'html'
  | 'src'
  | 'href'
  | 'content'
  | 'data-src'
  | 'data-original';

export type CustomImageFilters = {
  minWidth?: number;
  minHeight?: number;
  excludeKeywords?: string[];
  dedupeByImageKey?: boolean;
};

export type CustomFieldRule = {
  selectors?: string[];
  attr?: CustomAttrKind | string;
  multiple?: boolean;
  limit?: number;
  fallback?: string;
  filters?: CustomImageFilters;
  /** Scroll to first selector before extracting (detail images). */
  scrollIntoView?: boolean;
};

export type CustomAttributesRule = {
  mode?: 'pairs' | 'row' | 'text_all' | 'disabled';
  rowSelector?: string;
  keySelector?: string;
  valueSelector?: string;
  /** For text_all: selector whose text is parsed as "key：value" pairs */
  textSelector?: string;
};

export type CustomSkusRule = {
  mode?: 'disabled' | 'simple';
};

export type CustomFallbacksRule = {
  jsonLd?: boolean;
  openGraph?: boolean;
  meta?: boolean;
};

export type CustomRuleDecl = {
  title?: CustomFieldRule;
  price?: CustomFieldRule;
  currency?: CustomFieldRule;
  mainImages?: CustomFieldRule;
  descriptionImages?: CustomFieldRule;
  attributes?: CustomAttributesRule;
  skus?: CustomSkusRule;
  fallbacks?: CustomFallbacksRule;
};

export type CustomCollectOptions = {
  ruleId?: string;
  ruleName?: string;
  domain?: string;
  matchPattern?: string;
  rule?: CustomRuleDecl;
  /** task = 正式采集；rule_test 请走 POST /v1/collect/custom-rule-test */
  mode?: 'task' | 'rule_test';
  /** 使用 collect_browser_profiles.profile_key 的持久化登录态 */
  useBrowserProfile?: boolean;
  profileKey?: string;
  profileId?: string;
  /** @deprecated 使用 useBrowserProfile */
  customUseBrowserProfile?: boolean;
  customBrowserProfileName?: string;
  customCookieProfileId?: string;
  /** Optional page scroll before detail image extraction */
  scrollForDetailImages?: boolean;
};
