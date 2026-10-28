import { tr, trFormat } from '@/i18n';
import { TechnicalDetails } from '@/components/ui';
import { publishCapabilityLabel } from '@/constants/publishLabels';
import { Alert } from 'antd';

type Props = {
  capability?: string;
  blockedByCredentials?: boolean;
  showRcBadge?: boolean;
};

/** 刊登 / 发布边界：local_draft_only ≠ 真实平台成功 */
export default function PublishBoundaryBanner({
  capability,
  blockedByCredentials,
  showRcBadge = true,
}: Props) {
  const cap = (capability || '').trim().toLowerCase();
  const isLocalOnly = cap === 'local_draft_only';
  const isRealDraft = cap === 'real_draft_create';

  if (!isLocalOnly && !isRealDraft && !blockedByCredentials) return null;

  let message = tr("创建本地草稿不等于平台真实草稿；抖店当前仍为发布候选。");
  if (isLocalOnly) {
    message = trFormat("{capability}：仅在 TradeMind 内保存草稿记录，不会调用平台写接口。", {
      capability: publishCapabilityLabel('local_draft_only'),
    });
  } else if (isRealDraft) {
    message = tr("创建抖店草稿不等于商品上架；需人工在平台后台确认并提交，未完成人工验收时请勿视为已上线。");
  } else if (blockedByCredentials) {
    message = tr("缺少抖店真实环境凭证，当前结果仅代表配置检查，不代表平台调用成功。");
  }

  return (
    <Alert
      type="info"
      showIcon
      style={{ marginBottom: 16 }}
      message={
        <>
          {message}
          {showRcBadge ? tr("（抖店发布候选）") : null}
        </>
      }
      description={
        <TechnicalDetails label={tr("技术详情")}>
          <div>
            {trFormat("能力标识={capability}；真实凭证阻塞={blocked}", {
              capability: cap || '—',
              blocked: blockedByCredentials ? tr("是") : tr("否"),
            })}
          </div>
        </TechnicalDetails>
      }
    />
  );
}
