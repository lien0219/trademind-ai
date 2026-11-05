import { getJSON, postJSON, putJSON } from '@/services/request';

export type MailEvent = {
  key: string;
  category: string;
  name: string;
  description: string;
  placeholders: string[];
};

export type MailTemplate = {
  event: string;
  locale: string;
  subject: string;
  html: string;
  customized?: boolean;
};

export type RegistrationMode = 'closed' | 'invite_only' | 'open';

export function getMailTemplateOptions(): Promise<{ events: MailEvent[]; locales: string[] }> {
  return getJSON('/api/v1/settings/mail-templates/options');
}

export function getMailTemplate(event: string, locale: string): Promise<MailTemplate> {
  return getJSON('/api/v1/settings/mail-templates?' + new URLSearchParams({ event, locale }).toString());
}

export function previewMailTemplate(template: MailTemplate): Promise<{ subject: string; html: string }> {
  return postJSON('/api/v1/settings/mail-templates/preview', template);
}

export function saveMailTemplate(template: MailTemplate): Promise<{ ok: boolean }> {
  return putJSON('/api/v1/settings/mail-templates', template);
}

export function restoreMailTemplate(event: string, locale: string): Promise<{ ok: boolean }> {
  return postJSON('/api/v1/settings/mail-templates/restore', { event, locale });
}

export function getRegistrationPolicy(): Promise<{ mode: RegistrationMode }> {
  return getJSON('/api/v1/settings/registration-policy');
}

export function updateRegistrationPolicy(mode: RegistrationMode): Promise<{ mode: RegistrationMode }> {
  return putJSON('/api/v1/settings/registration-policy', { mode });
}
