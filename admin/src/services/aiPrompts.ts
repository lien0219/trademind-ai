import { deleteJSON, getJSON, postJSON, putJSON } from './request';

export type AIPromptRow = {
  id: string;
  code: string;
  name: string;
  scene: string;
  provider: string;
  model: string;
  systemPrompt: string;
  userPrompt: string;
  outputSchema?: unknown;
  temperature: number;
  maxTokens: number;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
};

export async function fetchAIPrompts() {
  return getJSON<{ list: AIPromptRow[] }>('/api/v1/ai/prompts');
}

export type AIPromptCreateBody = {
  code: string;
  name: string;
  scene?: string;
  provider?: string;
  model?: string;
  systemPrompt?: string;
  userPrompt?: string;
  outputSchema?: unknown;
  temperature?: number;
  maxTokens?: number;
  enabled?: boolean;
};

export async function createAIPrompt(body: AIPromptCreateBody) {
  return postJSON<AIPromptRow>('/api/v1/ai/prompts', body);
}

export async function updateAIPrompt(id: string, body: Record<string, unknown>) {
  return putJSON<AIPromptRow, Record<string, unknown>>(`/api/v1/ai/prompts/${id}`, body);
}

export async function deleteAIPrompt(id: string) {
  return deleteJSON<{ ok: boolean }>(`/api/v1/ai/prompts/${id}`);
}

export async function enableAIPrompt(id: string) {
  return postJSON<AIPromptRow>(`/api/v1/ai/prompts/${id}/enable`, {});
}

export async function disableAIPrompt(id: string) {
  return postJSON<AIPromptRow>(`/api/v1/ai/prompts/${id}/disable`, {});
}
