import { isAppLanguage, translateMessage, type AppLanguage } from './dictionaries';

export const LOCALE_STORAGE_KEY = 'trademind_admin_language';
const LOCALE_EVENT = 'trademind:locale-change';
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
  try {
    const language=window.localStorage.getItem(LOCALE_STORAGE_KEY);
    return isAppLanguage(language) ? language : 'zh-CN';
  } catch { return 'zh-CN'; }
}
export function applyLanguage(language:AppLanguage):void {
  if (typeof document==='undefined') return;
  document.documentElement.lang=language;
  document.documentElement.setAttribute('data-language',language);
}
export function setLanguage(language:AppLanguage):void {
  if (typeof window==='undefined') return;
  try {window.localStorage.setItem(LOCALE_STORAGE_KEY,language);} catch { /* private mode */ }
  applyLanguage(language);
  window.dispatchEvent(new CustomEvent(LOCALE_EVENT,{detail:language}));
}
export function subscribeLanguage(onChange:(next:AppLanguage)=>void):()=>void {
  if (typeof window==='undefined') return ()=>{};
  const receive=()=>onChange(readLanguage());
  window.addEventListener(LOCALE_EVENT,receive);
  window.addEventListener('storage',receive);
  return ()=>{window.removeEventListener(LOCALE_EVENT,receive);window.removeEventListener('storage',receive);};
}
export function tr(source:string,language:AppLanguage=readLanguage()):string {
  return translateMessage(language,source);
}
export function formatLocalizedDate(value:Date|string|number,language:AppLanguage=readLanguage(),timeZone?:string):string {
  const date=new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return new Intl.DateTimeFormat(language,{...dateFormats[language],timeZone}).format(date);
}
