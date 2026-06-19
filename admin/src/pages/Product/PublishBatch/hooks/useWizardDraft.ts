import type { PublishConfigLayer } from '@/constants/publishConfig';
import type { PublishConfigOverrides } from '@/services/productPublish';
import { simpleHash } from '@/utils/publishConfigMerge';
import { useCallback, useEffect } from 'react';

export type WizardDraft = {
  step: number;
  commonConfig: PublishConfigLayer;
  overrides: PublishConfigOverrides;
  selectedTargetKeys: string[];
  savedAt: number;
};

const PREFIX = 'publish-batch-wizard:';

export function wizardDraftKey(userId: string, productIds: string[]) {
  const sorted = [...productIds].sort().join(',');
  return `${PREFIX}${userId || 'anon'}:${simpleHash(sorted)}`;
}

export function loadWizardDraft(key: string): WizardDraft | null {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return null;
    return JSON.parse(raw) as WizardDraft;
  } catch {
    return null;
  }
}

export function saveWizardDraft(key: string, draft: WizardDraft) {
  try {
    localStorage.setItem(key, JSON.stringify({ ...draft, savedAt: Date.now() }));
  } catch {
    // ignore quota
  }
}

export function clearWizardDraft(key: string) {
  try {
    localStorage.removeItem(key);
  } catch {
    // ignore
  }
}

export function useWizardDraftPersistence(
  draftKey: string | null,
  dirty: boolean,
  snapshot: () => WizardDraft,
) {
  useEffect(() => {
    if (!draftKey || !dirty) return;
    const t = window.setTimeout(() => saveWizardDraft(draftKey, snapshot()), 400);
    return () => window.clearTimeout(t);
  }, [draftKey, dirty, snapshot]);

  useEffect(() => {
    if (!dirty) return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [dirty]);

  const clear = useCallback(() => {
    if (draftKey) clearWizardDraft(draftKey);
  }, [draftKey]);

  return { clear };
}
