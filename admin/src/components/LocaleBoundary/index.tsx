import { Fragment, useEffect, useState, type ReactNode } from 'react';
import { useAntdConfigSetter } from '@umijs/max';
import zhCN from 'antd/locale/zh_CN';
import enUS from 'antd/locale/en_US';
import jaJP from 'antd/locale/ja_JP';
import zhTW from 'antd/locale/zh_TW';
import koKR from 'antd/locale/ko_KR';
import thTH from 'antd/locale/th_TH';
import dayjs from 'dayjs';
import { applyLanguage, readLanguage, subscribeLanguage } from '@/i18n';

const locales = { 'zh-CN': zhCN, 'en-US': enUS, 'ja-JP': jaJP, 'zh-TW': zhTW, 'ko-KR': koKR, 'th-TH': thTH };
const dayjsLocales = { 'zh-CN': 'zh-cn', 'en-US': 'en', 'ja-JP': 'ja', 'zh-TW': 'zh-tw', 'ko-KR': 'ko', 'th-TH': 'th' };

/** Recreate the current view when the locale changes, without reloading auth. */
export default function LocaleBoundary({ children }: { children: ReactNode }) {
  const [language, setSelectedLanguage] = useState(readLanguage);
  const setAntdConfig = useAntdConfigSetter();
  useEffect(() => subscribeLanguage(setSelectedLanguage), []);
  useEffect(() => {
    applyLanguage(language);
    dayjs.locale(dayjsLocales[language]);
    setAntdConfig({ locale: locales[language] });
  }, [language, setAntdConfig]);
  return <Fragment key={language}>{children}</Fragment>;
}
