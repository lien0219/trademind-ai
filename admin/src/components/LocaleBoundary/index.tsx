import { useEffect, useRef, type ReactNode } from 'react';
import { useAntdConfigSetter } from '@umijs/max';
import zhCN from 'antd/locale/zh_CN';
import enUS from 'antd/locale/en_US';
import jaJP from 'antd/locale/ja_JP';
import zhTW from 'antd/locale/zh_TW';
import koKR from 'antd/locale/ko_KR';
import thTH from 'antd/locale/th_TH';
import dayjs from 'dayjs';
import { applyLanguage } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';

const locales = { 'zh-CN': zhCN, 'en-US': enUS, 'ja-JP': jaJP, 'zh-TW': zhTW, 'ko-KR': koKR, 'th-TH': thTH };
const dayjsLocales = { 'zh-CN': 'zh-cn', 'en-US': 'en', 'ja-JP': 'ja', 'zh-TW': 'zh-tw', 'ko-KR': 'ko', 'th-TH': 'th' };

/** Update component locales while preserving the mounted view and its state. */
export default function LocaleBoundary({ children }: { children: ReactNode }) {
  const language = useLanguage();
  const setAntdConfig = useAntdConfigSetter();
  const appliedLanguage = useRef<typeof language>();
  useEffect(() => {
    // Umi replaces the setter after every config update. Apply only a new
    // language so that writing the locale cannot trigger an update loop.
    if (appliedLanguage.current === language) return;
    appliedLanguage.current = language;
    applyLanguage(language);
    dayjs.locale(dayjsLocales[language]);
    setAntdConfig({ locale: locales[language] });
  }, [language, setAntdConfig]);
  return <>{children}</>;
}
