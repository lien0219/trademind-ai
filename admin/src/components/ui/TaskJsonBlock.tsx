import { Typography } from 'antd';

/** 任务详情抽屉内 JSON 展示 */
export function formatTaskJson(value: unknown): string {
  if (value == null || value === '') return '—';
  try {
    return typeof value === 'string' ? value : JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export type TaskJsonBlockProps = {
  title: string;
  value: unknown;
  maxHeight?: number;
  /** 是否为折叠区内最后一项（去掉段落下边距） */
  last?: boolean;
};

export default function TaskJsonBlock({ title, value, maxHeight = 200, last }: TaskJsonBlockProps) {
  return (
    <Typography.Paragraph style={{ marginBottom: last ? 0 : 8 }}>
      <Typography.Text strong>{title}</Typography.Text>
      <pre
        style={{
          fontSize: 12,
          overflow: 'auto',
          maxHeight,
          whiteSpace: 'pre-wrap',
          marginTop: 6,
          marginBottom: 0,
        }}
      >
        {formatTaskJson(value)}
      </pre>
    </Typography.Paragraph>
  );
}
