import TechnicalDetails from '@/components/ui/TechnicalDetails';
import {
  checkPublishTargets,
  createPublishTargetDrafts,
  fetchPublishTargets,
  type PublishTargetCheckResult,
  type PublishTargetPlatform,
  type PublishTargetRef,
  type PublishTargetsCheckResponse,
  type PublishTargetsCreateDraftsResponse,
} from '@/services/productPublish';
import { publishCapabilityLabel, publishTargetStatusLabel } from '@/constants/publishLabels';
import { Link } from '@umijs/renderer-react';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Collapse,
  Descriptions,
  Space,
  Spin,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';

type SelectedTarget = PublishTargetRef & { targetKey: string; shopName?: string };

function targetKey(platform: string, shopId?: string | null) {
  const p = (platform || '').trim().toLowerCase();
  const s = (shopId || '').trim();
  return s ? `${p}:${s}` : p;
}

function issueTag(severity: string) {
  const s = (severity || '').toLowerCase();
  if (s === 'error') return <Tag color="red">需处理</Tag>;
  return <Tag color="orange">建议检查</Tag>;
}

function targetStatusColor(status: string) {
  if (status === 'ready') return 'green';
  if (status === 'warning') return 'orange';
  if (status === 'blocked') return 'red';
  return 'default';
}

export default function MultiPlatformPublishCenter({
  productId,
  onDraftsCreated,
}: {
  productId: string;
  onDraftsCreated?: () => void | Promise<void>;
}) {
  const [loading, setLoading] = useState(true);
  const [platforms, setPlatforms] = useState<PublishTargetPlatform[]>([]);
  const [selected, setSelected] = useState<Record<string, SelectedTarget>>({});
  const [checking, setChecking] = useState(false);
  const [creating, setCreating] = useState(false);
  const [checkResult, setCheckResult] = useState<PublishTargetsCheckResponse | null>(null);
  const [createResult, setCreateResult] = useState<PublishTargetsCreateDraftsResponse | null>(null);
  const [lastBatchId, setLastBatchId] = useState<string | null>(null);

  const reloadTargets = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetchPublishTargets(productId);
      setPlatforms(res.platforms ?? []);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载刊登目标失败');
    } finally {
      setLoading(false);
    }
  }, [productId]);

  useEffect(() => {
    void reloadTargets();
  }, [reloadTargets]);

  const selectedList = useMemo(() => Object.values(selected), [selected]);

  const toggleShop = (platform: PublishTargetPlatform, shopId: string, shopName: string, checked: boolean) => {
    const key = targetKey(platform.platform, shopId);
    setSelected((prev) => {
      const next = { ...prev };
      if (checked) {
        next[key] = { platform: platform.platform, shopId, shopName, targetKey: key };
      } else {
        delete next[key];
      }
      return next;
    });
    setCheckResult(null);
    setCreateResult(null);
  };

  const togglePlatformShops = (platform: PublishTargetPlatform, checked: boolean) => {
    setSelected((prev) => {
      const next = { ...prev };
      for (const shop of platform.shops) {
        const key = targetKey(platform.platform, shop.shopId);
        if (checked && shop.enabled && shop.authStatus === 'authorized') {
          next[key] = {
            platform: platform.platform,
            shopId: shop.shopId,
            shopName: shop.shopName,
            targetKey: key,
          };
        } else {
          delete next[key];
        }
      }
      return next;
    });
    setCheckResult(null);
    setCreateResult(null);
  };

  const runCheck = async () => {
    if (!selectedList.length) {
      message.warning('请先选择要刊登的平台和店铺');
      return;
    }
    setChecking(true);
    try {
      const res = await checkPublishTargets(productId, {
        targets: selectedList.map(({ platform, shopId }) => ({ platform, shopId })),
      });
      setCheckResult(res);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '检查失败');
    } finally {
      setChecking(false);
    }
  };

  const runCreate = async (onlyReady: boolean, retryFailedOnly = false) => {
    if (!retryFailedOnly && !selectedList.length) {
      message.warning('请先选择要刊登的平台和店铺');
      return;
    }
    setCreating(true);
    try {
      const res = await createPublishTargetDrafts(productId, {
        targets: selectedList.map(({ platform, shopId }) => ({ platform, shopId })),
        onlyReady,
        retryFailedOnly,
        batchId: retryFailedOnly ? lastBatchId ?? undefined : undefined,
      });
      setCreateResult(res);
      setLastBatchId(res.batchId);
      message.success(
        retryFailedOnly
          ? `已重试失败目标：${res.successCount} 成功，${res.failedCount} 失败`
          : `批量创建刊登草稿完成：${res.successCount} 成功，${res.failedCount} 失败`,
      );
      await onDraftsCreated?.();
    } catch (e: unknown) {
      message.error((e as Error)?.message || '创建刊登草稿失败');
    } finally {
      setCreating(false);
    }
  };

  const renderTargetCheck = (t: PublishTargetCheckResult) => (
    <Card key={t.targetKey} size="small" style={{ marginBottom: 8 }}>
      <Space direction="vertical" style={{ width: '100%' }} size={4}>
        <Space wrap>
          <Typography.Text strong>
            {t.platformLabel}
            {t.shopName ? ` / ${t.shopName}` : ''}
          </Typography.Text>
          <Tag color={targetStatusColor(t.status)}>{t.statusLabel || publishTargetStatusLabel(t.status)}</Tag>
          <Tag>{publishCapabilityLabel(t.capability)}</Tag>
        </Space>
        {t.issues?.length ? (
          t.issues.map((iss, i) => (
            <div key={`${iss.code}-${i}`}>
              {issueTag(iss.severity)}{' '}
              <Typography.Text>{iss.title || iss.message}</Typography.Text>
              {iss.message && iss.title && iss.message !== iss.title ? (
                <Typography.Text type="secondary"> — {iss.message}</Typography.Text>
              ) : null}
              {iss.technicalDetails?.rawCode ? (
                <TechnicalDetails label="技术详情">
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    内部编号：{String(iss.technicalDetails.rawCode)}
                  </Typography.Text>
                </TechnicalDetails>
              ) : null}
            </div>
          ))
        ) : (
          <Typography.Text type="secondary">未发现问题</Typography.Text>
        )}
      </Space>
    </Card>
  );

  return (
    <Spin spinning={loading}>
      <div style={{ width: '100%', maxWidth: '100%', overflowX: 'hidden' }}>
      <Space direction="vertical" style={{ width: '100%' }} size="middle">
        <Alert
          type="info"
          showIcon
          message="多平台刊登中心"
          description={
            <>
              选择要刊登的平台和店铺，检查后可批量创建刊登草稿。当前不会直接上架；未接入真实接口的平台仅生成本地草稿与任务快照。
              抖店继续使用现有平台草稿链路。批量多商品发布将在下一阶段开放。
            </>
          }
        />

        <Card size="small" title="一、刊登目标" variant="borderless">
          <Typography.Paragraph type="secondary">选择要刊登的平台和店铺</Typography.Paragraph>
          <Space direction="vertical" style={{ width: '100%' }} size={12}>
            {platforms.map((plat) => {
              const authorizedShops = plat.shops.filter((s) => s.authStatus === 'authorized' && s.enabled);
              const allSelected =
                authorizedShops.length > 0 &&
                authorizedShops.every((s) => selected[targetKey(plat.platform, s.shopId)]);
              return (
                <div key={plat.platform}>
                  <Space align="start">
                    <Checkbox
                      checked={allSelected && authorizedShops.length > 0}
                      indeterminate={
                        authorizedShops.some((s) => selected[targetKey(plat.platform, s.shopId)]) && !allSelected
                      }
                      disabled={!authorizedShops.length}
                      onChange={(e) => togglePlatformShops(plat, e.target.checked)}
                    />
                    <Space direction="vertical" size={4}>
                      <Space wrap>
                        <Typography.Text strong>{plat.platformLabel}</Typography.Text>
                        <Tag color={plat.capability === 'real_draft_create' ? 'blue' : 'default'}>
                          {plat.capabilityLabel || publishCapabilityLabel(plat.capability)}
                        </Tag>
                        {plat.capability === 'not_configured' ? (
                          <Link to={plat.settingsPath || '/settings/platforms'}>去设置</Link>
                        ) : null}
                      </Space>
                      {plat.capability === 'local_draft_only' ? (
                        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                          暂未接入真实发布，可先生成本地刊登草稿
                        </Typography.Text>
                      ) : null}
                      {plat.shops.length ? (
                        <Space direction="vertical" size={2} style={{ paddingLeft: 8 }}>
                          {plat.shops.map((shop) => (
                            <Checkbox
                              key={shop.shopId}
                              checked={!!selected[targetKey(plat.platform, shop.shopId)]}
                              disabled={!shop.enabled || shop.authStatus !== 'authorized'}
                              onChange={(e) =>
                                toggleShop(plat, shop.shopId, shop.shopName, e.target.checked)
                              }
                            >
                              {shop.shopName}
                              {shop.authStatus !== 'authorized' ? (
                                <Typography.Text type="secondary">
                                  {' '}
                                  （{shop.authStatusLabel || '店铺未授权'}，
                                  <Link to="/shops">去授权</Link>）
                                </Typography.Text>
                              ) : null}
                            </Checkbox>
                          ))}
                        </Space>
                      ) : (
                        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                          暂无已授权店铺，请先在 <Link to="/shops">店铺管理</Link> 完成授权
                        </Typography.Text>
                      )}
                    </Space>
                  </Space>
                </div>
              );
            })}
          </Space>
        </Card>

        <Card size="small" title="二、统一刊登配置" variant="borderless">
          <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
            统一标题、描述、价格与图片策略可在各平台单独配置区覆盖。单独配置会优先生效，不会影响其他平台或店铺。
          </Typography.Paragraph>
          <Typography.Text type="secondary">本阶段统一配置预留接口，请在下方各平台区域完成具体配置。</Typography.Text>
        </Card>

        <Card size="small" title="发布检查结果" variant="borderless">
          {checkResult ? (
            <>
              <Descriptions size="small" column={{ xs: 1, sm: 2, md: 4 }} style={{ marginBottom: 12 }}>
                <Descriptions.Item label="目标数">{checkResult.summary.targetCount}</Descriptions.Item>
                <Descriptions.Item label="可以创建草稿">{checkResult.summary.readyCount}</Descriptions.Item>
                <Descriptions.Item label="需要检查">{checkResult.summary.warningCount}</Descriptions.Item>
                <Descriptions.Item label="暂不能创建">{checkResult.summary.blockedCount}</Descriptions.Item>
              </Descriptions>
              {checkResult.targets.map(renderTargetCheck)}
            </>
          ) : (
            <Typography.Text type="secondary">选择目标后点击「检查所选目标」</Typography.Text>
          )}
        </Card>

        <Space wrap>
          <Button loading={checking} onClick={() => void runCheck()}>
            检查所选目标
          </Button>
          <Button type="primary" loading={creating} onClick={() => void runCreate(false)}>
            创建刊登草稿
          </Button>
          <Button loading={creating} onClick={() => void runCreate(true)}>
            只处理可以创建草稿的目标
          </Button>
          {checkResult && (checkResult.summary.warningCount > 0 || checkResult.summary.blockedCount > 0) ? (
            <Button type="link" onClick={() => void runCheck()}>
              查看需要处理的问题
            </Button>
          ) : null}
          {lastBatchId && createResult && createResult.failedCount > 0 ? (
            <Button loading={creating} onClick={() => void runCreate(false, true)}>
              重试失败目标
            </Button>
          ) : null}
        </Space>

        {createResult ? (
          <Collapse
            size="small"
            items={[
              {
                key: 'batch',
                label: `批量创建刊登草稿结果：${createResult.statusLabel || createResult.status}`,
                children: (
                  <Space direction="vertical" style={{ width: '100%' }}>
                    {createResult.targets.map((t) => (
                      <div key={t.targetKey}>
                        <Tag color={t.status === 'success' || t.status === 'pending' || t.status === 'running' ? 'green' : t.status === 'skipped' ? 'default' : 'red'}>
                          {t.statusLabel || t.status}
                        </Tag>{' '}
                        {t.platformLabel}
                        {t.shopName ? ` / ${t.shopName}` : ''}
                        {t.localDraftOnly ? '（本地草稿）' : ''}
                        {t.errorMessage ? (
                          <Typography.Text type="danger"> — {t.errorMessage}</Typography.Text>
                        ) : null}
                      </div>
                    ))}
                  </Space>
                ),
              },
            ]}
          />
        ) : null}
      </Space>
      </div>
    </Spin>
  );
}
