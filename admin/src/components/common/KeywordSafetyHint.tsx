import { Alert } from 'antd';
import { tr } from '@/i18n';
import { KEYWORD_SENSITIVE_HINT } from '@/utils/keywordSafety';

type Props = {
  visible?: boolean;
  style?: React.CSSProperties;
};

export default function KeywordSafetyHint({ visible, style }: Props) {
  if (!visible) return null;
  return (
    <Alert
      type="warning"
      showIcon
      banner
      style={{ marginBottom: 12, ...style }}
      message={tr(KEYWORD_SENSITIVE_HINT)}
    />
  );
}
