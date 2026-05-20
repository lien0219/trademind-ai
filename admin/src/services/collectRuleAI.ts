import { postJSON } from './request';
import type { CollectRuleDetail, CollectRuleTestResult } from './collectRules';

export type CollectRuleAIFieldHit = {
  field: string;
  label: string;
  inRule: boolean;
  extracted: boolean;
  points: number;
  maxPoints: number;
};

export type CollectRuleAIQualityGate = {
  score: number;
  allowSaveEnabled: boolean;
  allowSaveDraft: boolean;
  blockReasons?: string[];
  suggestions?: string[];
  fieldHits?: CollectRuleAIFieldHit[];
  scoreBreakdown?: Record<string, number>;
};

export type CollectRuleAIGenerateResult = {
  rule: unknown;
  domain: string;
  suggestedName: string;
  confidence: number;
  explanation: string;
  warnings?: string[];
  missingGeneratedFields?: string[];
  qualityGate: CollectRuleAIQualityGate;
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
  status?: 'enabled' | 'disabled';
  profileId?: string;
  useBrowserProfile?: boolean;
  targetFields?: string[];
  ruleName?: string;
}) {
  return postJSON<CollectRuleDetail>('/api/v1/collect/rules/ai-generate-and-save', payload);
}
