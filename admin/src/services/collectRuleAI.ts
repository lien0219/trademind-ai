import { postJSON } from './request';
import type { CollectRuleDetail, CollectRuleTestResult } from './collectRules';

export type CollectRuleAIGenerateResult = {
  rule: unknown;
  domain: string;
  suggestedName: string;
  confidence: number;
  explanation: string;
  warnings?: string[];
  testResult?: CollectRuleTestResult;
  plannedHint?: string;
};

export async function aiGenerateCollectRule(payload: {
  url: string;
  domain?: string;
  profileId?: string;
  useBrowserProfile?: boolean;
  targetFields?: string[];
  ruleName?: string;
}) {
  return postJSON<CollectRuleAIGenerateResult>('/api/v1/collect/rules/ai-generate', payload);
}

export async function aiGenerateAndSaveCollectRule(payload: {
  url: string;
  domain?: string;
  name: string;
  priority?: number;
  profileId?: string;
  useBrowserProfile?: boolean;
  targetFields?: string[];
  ruleName?: string;
}) {
  return postJSON<CollectRuleDetail>('/api/v1/collect/rules/ai-generate-and-save', payload);
}
