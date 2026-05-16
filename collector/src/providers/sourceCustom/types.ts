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

export type CustomFieldRule = {
  selectors?: string[];
  attr?: CustomAttrKind | string;
  multiple?: boolean;
  limit?: number;
  fallback?: string;
};

export type CustomAttributesRule = {
  mode?: 'pairs' | 'disabled';
  rowSelector?: string;
  keySelector?: string;
  valueSelector?: string;
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
};
