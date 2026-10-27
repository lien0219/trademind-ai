import { isAppLanguage, translateMessage, type AppLanguage } from './dictionaries';

export const LOCALE_STORAGE_KEY = 'trademind_admin_language';
const LOCALE_EVENT = 'trademind:locale-change';
let volatileLanguage: AppLanguage | undefined;
const dateFormats: Record<AppLanguage,Intl.DateTimeFormatOptions> = {
  'zh-CN':{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',hour12:false},
  'en-US':{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit'},
  'ja-JP':{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',hour12:false},
  'zh-TW':{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',hour12:false},
  'ko-KR':{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit'},
  'th-TH':{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit'},
};

export function readLanguage():AppLanguage {
  if (typeof window==='undefined') return 'zh-CN';
  if (volatileLanguage) return volatileLanguage;
  try {
    const language=window.localStorage.getItem(LOCALE_STORAGE_KEY);
    return isAppLanguage(language) ? language : 'zh-CN';
  } catch { return volatileLanguage || 'zh-CN'; }
}
export function applyLanguage(language:AppLanguage):void {
  if (typeof document==='undefined') return;
  document.documentElement.lang=language;
  document.documentElement.setAttribute('data-language',language);
}
export function setLanguage(language:AppLanguage):void {
  if (typeof window==='undefined' || !isAppLanguage(language)) return;
  try {window.localStorage.setItem(LOCALE_STORAGE_KEY,language); volatileLanguage = undefined;}
  catch { volatileLanguage = language; }
  applyLanguage(language);
  window.dispatchEvent(new CustomEvent(LOCALE_EVENT,{detail:language}));
}
export function subscribeLanguage(onChange:(next:AppLanguage)=>void):()=>void {
  if (typeof window==='undefined') return ()=>{};
  const receive=(event:Event)=>{
    if (event instanceof StorageEvent && event.key !== null && event.key !== LOCALE_STORAGE_KEY) return;
    if (event instanceof StorageEvent) volatileLanguage = undefined;
    const detail = (event as CustomEvent<unknown>).detail;
    onChange(isAppLanguage(detail) ? detail : readLanguage());
  };
  window.addEventListener(LOCALE_EVENT,receive);
  window.addEventListener('storage',receive);
  return ()=>{window.removeEventListener(LOCALE_EVENT,receive);window.removeEventListener('storage',receive);};
}
export function tr(source:string,language:AppLanguage=readLanguage()):string {
  return translateMessage(language,source);
}
export const DATE_FORMATS = ['YYYY-MM-DD', 'DD/MM/YYYY', 'MM/DD/YYYY'] as const;
export type DateFormat = typeof DATE_FORMATS[number];
const REGIONAL_STORAGE_KEY = 'trademind_admin_regional';
type RegionalPreferences = { dateFormat: DateFormat; timeZone?: string };
let volatileRegional: RegionalPreferences = { dateFormat: 'YYYY-MM-DD' };

export function isDateFormat(value: unknown): value is DateFormat {
  return typeof value === 'string' && (DATE_FORMATS as readonly string[]).includes(value);
}

function validTimeZone(value: unknown): value is string {
  if (typeof value !== 'string') return false;
  try { new Intl.DateTimeFormat('en-US', { timeZone: value }); return true; }
  catch { return false; }
}

export function readRegionalPreferences(): RegionalPreferences {
  if (typeof window === 'undefined') return volatileRegional;
  try {
    const stored: unknown = JSON.parse(window.localStorage.getItem(REGIONAL_STORAGE_KEY) || 'null');
    if (!stored || typeof stored !== 'object') return { dateFormat: 'YYYY-MM-DD' };
    const candidate = stored as Record<string, unknown>;
    return {
      dateFormat: isDateFormat(candidate.dateFormat) ? candidate.dateFormat : 'YYYY-MM-DD',
      timeZone: validTimeZone(candidate.timeZone) ? candidate.timeZone : undefined,
    };
  } catch { return volatileRegional; }
}

export function setRegionalPreferences(preferences: { dateFormat?: unknown; timeZone?: unknown }): void {
  volatileRegional = {
    dateFormat: isDateFormat(preferences.dateFormat) ? preferences.dateFormat : 'YYYY-MM-DD',
    timeZone: validTimeZone(preferences.timeZone) ? preferences.timeZone : undefined,
  };
  if (typeof window !== 'undefined') {
    try { window.localStorage.setItem(REGIONAL_STORAGE_KEY, JSON.stringify(volatileRegional)); }
    catch { /* Keep preferences usable when browser storage is unavailable. */ }
  }
}

export function formatLocalizedDate(value:Date|string|number,language:AppLanguage=readLanguage(),timeZone?:string):string {
  const date=new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  const preferences = readRegionalPreferences();
  const { hour12: _hour12, ...localeFormat } = dateFormats[language];
  const parts = new Intl.DateTimeFormat(language, {
    ...localeFormat, calendar: 'gregory', numberingSystem: 'latn', hourCycle: 'h23',
    timeZone: timeZone || preferences.timeZone,
  }).formatToParts(date);
  const part = (type: Intl.DateTimeFormatPartTypes) => parts.find(item => item.type === type)?.value || '';
  const fields = { YYYY: part('year'), MM: part('month'), DD: part('day') };
  const formattedDate = preferences.dateFormat.replace(/YYYY|MM|DD/g, key => fields[key as keyof typeof fields]);
  return `${formattedDate} ${part('hour')}:${part('minute')}:${part('second')}`;
}

export function formatLocalizedNumber(value: number, options?: Intl.NumberFormatOptions, language: AppLanguage = readLanguage()): string {
  return new Intl.NumberFormat(language, options).format(value);
}

// Currency is always supplied by the business fact. A display preference must
// never relabel an amount as another currency or imply an exchange conversion.
export function formatLocalizedCurrency(value: number, currency: string, language: AppLanguage = readLanguage()): string {
  return formatLocalizedNumber(value, { style: 'currency', currency }, language);
}
