import { getJSON } from '@/services/request';

export type ConfigStatusItem = {
  key: string;
  title: string;
  status: string;
  summary?: string;
  nextAction?: string;
  settingsUrl?: string;
};

export type ConfigStatusOverview = {
  generatedAt: string;
  items: ConfigStatusItem[];
  demoData: ConfigStatusItem;
};

export async function fetchConfigStatusOverview() {
  return getJSON<ConfigStatusOverview>('/api/v1/settings/config-status');
}
