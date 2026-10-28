import { tr } from '@/i18n';
import { history } from '@umijs/max';
import { Alert, Button, Space } from 'antd';

type Props = {
  blockedByCredentials?: boolean;
  releaseCandidate?: boolean;
  compact?: boolean;
};

/** 抖店上线前置提示：仅说明凭据与人工验收边界。 */
export default function DouyinE2EPrecheckBanner({
  blockedByCredentials = true,
  releaseCandidate = true,
  compact = false,
}: Props) {
  if (!blockedByCredentials && !releaseCandidate) return null;

  const message = blockedByCredentials
    ? tr('当前未配置抖店真实凭证，只能完成本地配置检查，不能调用真实平台。')
    : tr('抖店接入仍需人工验收；创建平台草稿不等于商品上架。');

  return (
    <Alert
      type="info"
      showIcon
      style={{ marginBottom: compact ? 8 : 16 }}
      message={
        <Space wrap>
          <span>{message}</span>
          {releaseCandidate ? (
            <span style={{ opacity: 0.85 }}>{tr("（抖店发布候选）")}</span>
          ) : null}
        </Space>
      }
      description={
        compact ? undefined : (
          <Space wrap style={{ marginTop: 8 }}>
            <Button size="small" onClick={() => history.push('/settings/platforms?platform=douyin_shop')}>
              {tr('去配置平台凭证')}
            </Button>
            <Button size="small" onClick={() => history.push('/settings/config-status')}>
              {tr('查看上线准备清单')}
            </Button>
            <Button size="small" onClick={() => history.push('/ops/task-center/failures?platform=douyin_shop')}>
              {tr('查看失败任务')}
            </Button>
          </Space>
        )
      }
    />
  );
}

export function douyinCredentialStatusLabel(blocked: boolean): string {
  return blocked ? '缺少真实凭证' : '前置检查可通过';
}
