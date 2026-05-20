import { CheckCircleOutlined, CloseCircleOutlined, MinusCircleOutlined } from '@ant-design/icons';
import { Progress, Space, Tag, Typography } from 'antd';
import type { CollectRuleAIQualityGate } from '@/services/collectRuleAI';

const { Text } = Typography;

type Props = {
  gate: CollectRuleAIQualityGate;
  confidence?: number;
};

function hitStatus(hit: CollectRuleAIQualityGate['fieldHits'][number]) {
  if (hit.extracted) return { icon: <CheckCircleOutlined />, color: 'success' as const, text: '已识别' };
  if (hit.inRule) return { icon: <MinusCircleOutlined />, color: 'warning' as const, text: '规则有·未识别' };
  return { icon: <CloseCircleOutlined />, color: 'default' as const, text: '未生成' };
}

export function RuleQualityScoreCard({ gate, confidence }: Props) {
  const score = gate.score ?? 0;
  const progressStatus = score >= 60 ? 'success' : score >= 30 ? 'normal' : 'exception';

  return (
    <div className="tm-ai-rule-modal__score-card">
      <div className="tm-ai-rule-modal__score-main">
        <div className="tm-ai-rule-modal__score-ring">
          <Progress
            type="circle"
            percent={Math.min(100, Math.max(0, score))}
            size={88}
            status={progressStatus}
            format={(p) => (
              <div className="tm-ai-rule-modal__score-ring-inner">
                <span className="tm-ai-rule-modal__score-value">{p ?? 0}</span>
                <span className="tm-ai-rule-modal__score-label">质量分</span>
              </div>
            )}
          />
        </div>
        <div className="tm-ai-rule-modal__score-meta">
          {typeof confidence === 'number' ? (
            <Text type="secondary">AI 置信度 {Math.round(confidence * 100)}%</Text>
          ) : null}
          <Text type={score >= 60 ? 'success' : 'warning'}>
            {score >= 60 ? '识别效果达标，可保存并启用' : '识别效果未达标，建议重新生成或手动调整'}
          </Text>
        </div>
      </div>

      {gate.fieldHits?.length ? (
        <div className="tm-ai-rule-modal__field-hits">
          <Text type="secondary" className="tm-ai-rule-modal__field-hits-title">
            字段命中
          </Text>
          <Space wrap size={[8, 8]}>
            {gate.fieldHits.map((hit) => {
              const st = hitStatus(hit);
              return (
                <Tag key={hit.field} icon={st.icon} color={st.color}>
                  {hit.label} · {st.text}
                  {hit.maxPoints > 0 ? ` (${hit.points}/${hit.maxPoints})` : ''}
                </Tag>
              );
            })}
          </Space>
        </div>
      ) : null}
    </div>
  );
}
