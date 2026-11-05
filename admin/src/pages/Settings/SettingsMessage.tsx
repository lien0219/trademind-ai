import { trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';

/** Keeps text owned by Ant Design's static confirmation portal subscribed. */
export default function SettingsMessage({ source, values = {} }: {
  source: string;
  values?: Record<string, string | number>;
}) {
  const language = useLanguage();
  return <>{trFormat(source, values, language)}</>;
}
