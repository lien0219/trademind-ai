import type { ActionType, ProFormInstance } from '@ant-design/pro-components';
import { message } from 'antd';
import { tr } from '@/i18n';
import { useCallback, useMemo, useState, type RefObject } from 'react';
import {
  KEYWORD_MAX_LENGTH,
  KEYWORD_TOO_LONG_MESSAGE,
  looksLikeSensitiveKeyword,
  normalizeSearchKeyword,
} from '@/utils/keywordSafety';
import type { UrlState } from '@/utils/urlState';

type Options = {
  setUrlState: (next: UrlState, options?: { replace?: boolean }) => void;
  formRef?: RefObject<ProFormInstance | undefined>;
  actionRef?: RefObject<ActionType | undefined>;
  setTablePage?: (page: number) => void;
};

export function useKeywordSearchField(options: Options) {
  const [sensitive, setSensitive] = useState(false);

  const clearKeyword = useCallback(() => {
    options.formRef?.current?.setFieldsValue?.({ keyword: undefined });
    options.setUrlState({ keyword: undefined, page: undefined }, { replace: true });
    options.setTablePage?.(1);
    options.actionRef?.current?.reload?.();
  }, [options]);

  const fieldProps = useMemo(
    () => ({
      allowClear: true,
      maxLength: KEYWORD_MAX_LENGTH,
      onChange: (e: React.ChangeEvent<HTMLInputElement>) => {
        setSensitive(looksLikeSensitiveKeyword(e.target.value));
      },
      onClear: clearKeyword,
    }),
    [clearKeyword],
  );

  const prepareKeyword = useCallback((raw: unknown) => {
    const { value, truncated } = normalizeSearchKeyword(raw);
    if (truncated) message.warning(tr(KEYWORD_TOO_LONG_MESSAGE));
    setSensitive(looksLikeSensitiveKeyword(value));
    return value;
  }, []);

  return { fieldProps, prepareKeyword, showSensitiveHint: sensitive, clearKeyword };
}
