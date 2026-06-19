import TechnicalDetails from '@/components/ui/TechnicalDetails';
import { TmPageContainer } from '@/components/ui';
import type { PublishConfigLayer } from '@/constants/publishConfig';
import { validatePublishConfigClient } from '@/constants/publishConfig';
import {
  publishCapabilityLabel,
  publishTargetStatusLabel,
} from '@/constants/publishLabels';
import {
  PUBLISH_BATCH_LIMIT_MESSAGE,
  PUBLISH_BATCH_MAX_PRODUCTS,
  validatePublishBatchMatrix,
} from '@/constants/publishLimits';
import { PRODUCT_STATUS } from '@/constants/status';
import { productSourceLabel } from '@/constants/userFriendly';
import ConfigPriorityBanner from '@/pages/Product/PublishBatch/components/ConfigPriorityBanner';
import EffectiveConfigPreviewModal from '@/pages/Product/PublishBatch/components/EffectiveConfigPreviewModal';
import OverrideConfigTabs from '@/pages/Product/PublishBatch/components/OverrideConfigTabs';
import PublishConfigEditor from '@/pages/Product/PublishBatch/components/PublishConfigEditor';
import {
  useWizardDraftPersistence,
  wizardDraftKey,
  type WizardDraft,
} from '@/pages/Product/PublishBatch/hooks/useWizardDraft';
import { fetchProductDetail, fetchProductOperationProgress, type ProductListRow } from '@/services/products';
import {
  checkBatchPublishTargets,
  createBatchPublishDrafts,
  fetchGlobalPublishTargets,
  type BatchTargetCheckItem,
  type BatchTargetsCheckResponse,
  type PublishConfigOverrides,
  type PublishTargetPlatform,
  type PublishTargetRef,
} from '@/services/productPublish';
import { detectConfigReminders } from '@/utils/publishConfigMerge';
import { Link, history, useLocation, useModel } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Descriptions,
  Image,
  Modal,
  Space,
  Spin,
  Steps,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';

type SelectedTarget = PublishTargetRef & {
  targetKey: string;
  shopName?: string;
  platformLabel?: string;
};

function targetKey(platform: string, shopId?: string | null) {
  const p = (platform || '').trim().toLowerCase();
  const s = (shopId || '').trim();
  return s ? `${p}:${s}` : p;
}

function statusColor(status: string) {
  if (status === 'ready') return 'green';
  if (status === 'warning') return 'orange';
  if (status === 'blocked') return 'red';
  return 'default';
}

function parseProductIdsFromSearch(search: string): string[] {
  try {
    const sp = new URLSearchParams(search);
    const raw = sp.get('productIds')?.trim();
    if (!raw) return [];
    return raw.split(',').map((s) => s.trim()).filter(Boolean);
  } catch {
    return [];
  }
}

function parseConfigError(e: unknown): { title?: string; message: string; technical?: Record<string, unknown> } {
  const err = e as Error & { data?: Record<string, unknown> };
  const data = err.data;
  if (data && typeof data === 'object') {
    return {
      title: String(data.title || '刊登配置不正确'),
      message: String(data.message || err.message || '配置校验失败'),
      technical: data.technicalDetails as Record<string, unknown> | undefined,
    };
  }
  return { message: err.message || '操作失败' };
}

export default function PublishBatchWizardPage() {
  const location = useLocation();
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const userId = initialState?.currentUser?.id?.toString() || initialState?.currentUser?.username || 'anon';
  const initialIds = useMemo(() => parseProductIdsFromSearch(location.search), [location.search]);

  const [step, setStep] = useState(0);
  const [products, setProducts] = useState<ProductListRow[]>([]);
  const [loadingProducts, setLoadingProducts] = useState(true);
  const [platforms, setPlatforms] = useState<PublishTargetPlatform[]>([]);
  const [loadingPlatforms, setLoadingPlatforms] = useState(false);
  const [selectedTargets, setSelectedTargets] = useState<Record<string, SelectedTarget>>({});
  const [commonConfig, setCommonConfig] = useState<PublishConfigLayer>({});
  const [overrides, setOverrides] = useState<PublishConfigOverrides>({});
  const [checkResult, setCheckResult] = useState<BatchTargetsCheckResponse | null>(null);
  const [checking, setChecking] = useState(false);
  const [creating, setCreating] = useState(false);
  const [matrixPage, setMatrixPage] = useState(1);
  const [previewOpen, setPreviewOpen] = useState(false);
  const [configError, setConfigError] = useState<{ title?: string; message: string; technical?: Record<string, unknown> } | null>(null);
  const [draftPrompted, setDraftPrompted] = useState(false);

  const productIds = useMemo(() => products.map((p) => p.id), [products]);
  const draftKey = useMemo(
    () => (productIds.length ? wizardDraftKey(userId, productIds) : null),
    [userId, productIds],
  );
  const selectedTargetList = useMemo(() => Object.values(selectedTargets), [selectedTargets]);
  const expectedTasks = productIds.length * selectedTargetList.length;
  const matrixLimitError = useMemo(
    () => validatePublishBatchMatrix(productIds.length, selectedTargetList.length),
    [productIds.length, selectedTargetList.length],
  );
  const dirty = step > 1 || Object.keys(commonConfig).length > 0 || Object.keys(overrides).length > 0;

  const draftSnapshot = useCallback(
    (): WizardDraft => ({
      step,
      commonConfig,
      overrides,
      selectedTargetKeys: Object.keys(selectedTargets),
      savedAt: Date.now(),
    }),
    [step, commonConfig, overrides, selectedTargets],
  );

  const restoreDraft = useCallback((draft: WizardDraft) => {
    setCommonConfig(draft.commonConfig || {});
    setOverrides(draft.overrides || {});
    if (draft.step >= 2 && draft.step <= 4) setStep(draft.step);
  }, []);

  const { clear: clearDraft } = useWizardDraftPersistence(draftKey, dirty, draftSnapshot);

  useEffect(() => {
    if (!draftKey || draftPrompted || !productIds.length) return;
    const existing = (() => {
      try {
        const raw = localStorage.getItem(draftKey);
        return raw ? (JSON.parse(raw) as WizardDraft) : null;
      } catch {
        return null;
      }
    })();
    if (existing && (existing.step > 1 || Object.keys(existing.commonConfig || {}).length)) {
      setDraftPrompted(true);
      Modal.confirm({
        title: '恢复未完成的批量刊登配置？',
        content: '检测到本批商品有未提交的向导草稿，是否恢复？',
        okText: '恢复草稿',
        cancelText: '重新开始',
        onOk: () => restoreDraft(existing),
        onCancel: () => clearDraft(),
      });
    }
  }, [draftKey, draftPrompted, productIds.length, restoreDraft, clearDraft]);

  const loadProducts = useCallback(async (ids: string[]) => {
    if (!ids.length) {
      setProducts([]);
      setLoadingProducts(false);
      return;
    }
    if (ids.length > PUBLISH_BATCH_MAX_PRODUCTS) {
      message.error(PUBLISH_BATCH_LIMIT_MESSAGE);
    }
    setLoadingProducts(true);
    try {
      const rows = await Promise.all(
        ids.map(async (id) => {
          try {
            const [detail, progress] = await Promise.all([
              fetchProductDetail(id),
              fetchProductOperationProgress(id).catch(() => null),
            ]);
            const cover = detail.images?.find((img) => img.imageType === 'main')?.publicUrl;
            return {
              id: detail.id,
              title: detail.title,
              source: detail.source,
              status: detail.status,
              coverUrl: cover,
              createdAt: detail.createdAt,
              operationProgress: progress ?? undefined,
            } as ProductListRow;
          } catch {
            return null;
          }
        }),
      );
      setProducts(rows.filter(Boolean) as ProductListRow[]);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载商品失败');
    } finally {
      setLoadingProducts(false);
    }
  }, []);

  useEffect(() => {
    void loadProducts(initialIds);
  }, [initialIds, loadProducts]);

  const loadPlatforms = useCallback(async () => {
    setLoadingPlatforms(true);
    try {
      const res = await fetchGlobalPublishTargets();
      setPlatforms(res.platforms ?? []);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载刊登目标失败');
    } finally {
      setLoadingPlatforms(false);
    }
  }, []);

  useEffect(() => {
    if (step === 1 && platforms.length === 0) {
      void loadPlatforms();
    }
  }, [step, platforms.length, loadPlatforms]);

  const removeProduct = (id: string) => {
    setProducts((prev) => prev.filter((p) => p.id !== id));
    setCheckResult(null);
  };

  const toggleShop = (platform: PublishTargetPlatform, shopId: string, shopName: string, checked: boolean) => {
    const key = targetKey(platform.platform, shopId);
    setSelectedTargets((prev) => {
      const next = { ...prev };
      if (checked) {
        next[key] = {
          platform: platform.platform,
          shopId,
          shopName,
          platformLabel: platform.platformLabel,
          targetKey: key,
        };
      } else {
        delete next[key];
      }
      return next;
    });
    setCheckResult(null);
  };

  const runCheck = async () => {
    const clientErr = validatePublishConfigClient(commonConfig);
    if (clientErr) {
      message.error(clientErr);
      return;
    }
    if (!productIds.length || !selectedTargetList.length) {
      message.warning('请先选择商品和刊登目标');
      return;
    }
    if (matrixLimitError) {
      message.error(matrixLimitError);
      return;
    }
    setChecking(true);
    setConfigError(null);
    try {
      const res = await checkBatchPublishTargets({
        productIds,
        targets: selectedTargetList.map(({ platform, shopId }) => ({ platform, shopId })),
        commonConfig: commonConfig as Record<string, unknown>,
        overrides,
      });
      setCheckResult(res);
      setStep(4);
    } catch (e: unknown) {
      const parsed = parseConfigError(e);
      setConfigError(parsed);
      message.error(parsed.title ? `${parsed.title}：${parsed.message}` : parsed.message);
    } finally {
      setChecking(false);
    }
  };

  const runCreate = async (onlyReady: boolean) => {
    if (matrixLimitError) {
      message.error(matrixLimitError);
      return;
    }
    setCreating(true);
    try {
      const res = await createBatchPublishDrafts({
        productIds,
        targets: selectedTargetList.map(({ platform, shopId }) => ({ platform, shopId })),
        commonConfig: commonConfig as Record<string, unknown>,
        overrides,
        onlyReady,
        includeWarnings: !onlyReady,
        name: `批量刊登 ${productIds.length} 商品`,
      });
      clearDraft();
      message.success(`批次已创建：${res.successCount} 成功，${res.failedCount} 失败，${res.skippedCount} 跳过`);
      history.push(`/product/publish-batches/${res.batchId}`);
    } catch (e: unknown) {
      const parsed = parseConfigError(e);
      message.error(parsed.title ? `${parsed.title}：${parsed.message}` : parsed.message);
    } finally {
      setCreating(false);
    }
  };

  const configReminders = useMemo(() => {
    const list: ReturnType<typeof detectConfigReminders> = [];
    products.forEach((p) => {
      selectedTargetList.forEach((t) => {
        list.push(
          ...detectConfigReminders(
            commonConfig,
            overrides,
            p.id,
            t.platform,
            t.shopId || undefined,
            p.title,
            t.platformLabel,
            t.shopName,
          ),
        );
      });
    });
    const seen = new Set<string>();
    return list.filter((r) => {
      if (seen.has(r.key)) return false;
      seen.add(r.key);
      return true;
    });
  }, [products, selectedTargetList, commonConfig, overrides]);

  const previewCells = useMemo(
    () =>
      products.flatMap((p) =>
        selectedTargetList.map((t) => ({
          productId: p.id,
          productTitle: p.title,
          platform: t.platform,
          platformLabel: t.platformLabel || t.platform,
          shopId: t.shopId || undefined,
          shopName: t.shopName,
        })),
      ),
    [products, selectedTargetList],
  );

  const handleBack = () => {
    if (dirty) {
      Modal.confirm({
        title: '离开批量刊登向导？',
        content: '未提交的配置已自动保存为草稿，可稍后恢复。',
        okText: '离开',
        cancelText: '继续编辑',
        onOk: () => history.push('/product/drafts'),
      });
      return;
    }
    history.push('/product/drafts');
  };

  const matrixColumns = [
    { title: '商品', dataIndex: 'productTitle', ellipsis: true, width: 180 },
    {
      title: '刊登目标',
      key: 'target',
      width: 160,
      render: (_: unknown, r: BatchTargetCheckItem) => (
        <span>
          {r.platformLabel}
          {r.shopName ? ` / ${r.shopName}` : ''}
        </span>
      ),
    },
    {
      title: '能力',
      dataIndex: 'capability',
      width: 120,
      render: (v: string) => publishCapabilityLabel(v),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 120,
      render: (_: unknown, r: BatchTargetCheckItem) => (
        <Tag color={statusColor(r.status)}>{r.statusLabel || publishTargetStatusLabel(r.status)}</Tag>
      ),
    },
    {
      title: '问题',
      key: 'issues',
      ellipsis: true,
      render: (_: unknown, r: BatchTargetCheckItem) =>
        r.issues?.length ? r.issues.map((i) => i.title || i.message).join('；') : '—',
    },
  ];

  const stepItems = [
    { title: '确认商品' },
    { title: '选择平台店铺' },
    { title: '统一配置' },
    { title: '单独覆盖' },
    { title: '检查并创建' },
  ];

  return (
    <TmPageContainer
      title="批量创建刊登草稿"
      subTitle="为多个商品同时创建平台草稿或本地刊登草稿（不直接上架）"
      onBack={handleBack}
    >
      <Steps current={step} items={stepItems} style={{ marginBottom: 24 }} />

      {step === 0 && (
        <Card title="第 1 步：确认商品">
          {loadingProducts ? (
            <Spin />
          ) : products.length === 0 ? (
            <Alert
              type="warning"
              showIcon
              message="未选择商品"
              description={
                <span>
                  请先在 <Link to="/product/drafts">商品草稿列表</Link> 勾选商品后点击「批量创建刊登草稿」。
                </span>
              }
            />
          ) : (
            <>
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 16 }}
                message={`已选择 ${products.length} 个商品${selectedTargetList.length ? `，预计生成 ${expectedTasks} 个刊登任务` : ''}`}
              />
              <Table
                rowKey="id"
                size="small"
                pagination={{ pageSize: 10 }}
                dataSource={products}
                scroll={{ x: 900 }}
                columns={[
                  {
                    title: '主图',
                    dataIndex: 'coverUrl',
                    width: 72,
                    render: (url: string) =>
                      url ? <Image src={url} width={48} height={48} style={{ objectFit: 'cover' }} /> : '—',
                  },
                  { title: '标题', dataIndex: 'title', ellipsis: true },
                  {
                    title: '来源',
                    dataIndex: 'source',
                    width: 100,
                    render: (v: string) => productSourceLabel(v),
                  },
                  {
                    title: '状态',
                    dataIndex: 'status',
                    width: 100,
                    render: (v: string) => {
                      const m = PRODUCT_STATUS[v as keyof typeof PRODUCT_STATUS];
                      return <Tag color={m?.color}>{m?.text ?? v}</Tag>;
                    },
                  },
                  {
                    title: '运营进度',
                    key: 'progress',
                    width: 140,
                    render: (_: unknown, row: ProductListRow) =>
                      row.operationProgress ? (
                        <Tag color={row.operationProgress.currentStep === 'ready' ? 'green' : 'blue'}>
                          {row.operationProgress.currentStepLabel || '继续完善'}
                        </Tag>
                      ) : (
                        '—'
                      ),
                  },
                  {
                    title: '操作',
                    key: 'ops',
                    width: 140,
                    render: (_: unknown, row: ProductListRow) => (
                      <Space>
                        <Link to={`/product/drafts/${row.id}`}>查看详情</Link>
                        <Typography.Link type="danger" onClick={() => removeProduct(row.id)}>
                          移除
                        </Typography.Link>
                      </Space>
                    ),
                  },
                ]}
              />
            </>
          )}
          <div style={{ marginTop: 16, textAlign: 'right' }}>
            <Button type="primary" disabled={!products.length} onClick={() => setStep(1)}>
              下一步
            </Button>
          </div>
        </Card>
      )}

      {step === 1 && (
        <Card title="第 2 步：选择平台和店铺">
          {loadingPlatforms ? (
            <Spin />
          ) : (
            <Space direction="vertical" style={{ width: '100%' }} size="middle">
              {platforms.map((plat) => (
                <Card key={plat.platform} size="small" type="inner">
                  <Space wrap style={{ marginBottom: 8 }}>
                    <Typography.Text strong>{plat.platformLabel}</Typography.Text>
                    <Tag>{publishCapabilityLabel(plat.capability)}</Tag>
                  </Space>
                  <Space direction="vertical" style={{ width: '100%' }}>
                    {plat.shops?.length ? (
                      plat.shops.map((shop) => {
                        const key = targetKey(plat.platform, shop.shopId);
                        const disabled = !shop.enabled || shop.authStatus !== 'authorized';
                        return (
                          <Checkbox
                            key={key}
                            checked={!!selectedTargets[key]}
                            disabled={disabled}
                            onChange={(e) => toggleShop(plat, shop.shopId, shop.shopName, e.target.checked)}
                          >
                            {shop.shopName}
                            {disabled ? (
                              <Typography.Text type="secondary">
                                {' '}
                                （{shop.authStatus !== 'authorized' ? '店铺未授权' : '已停用'}）
                              </Typography.Text>
                            ) : null}
                          </Checkbox>
                        );
                      })
                    ) : (
                      <Typography.Text type="secondary">暂无可用店铺</Typography.Text>
                    )}
                  </Space>
                </Card>
              ))}
            </Space>
          )}
          <Alert
            type={matrixLimitError ? 'error' : 'info'}
            showIcon
            style={{ marginTop: 16 }}
            message={
              matrixLimitError ||
              `已选 ${selectedTargetList.length} 个刊登目标，预计 ${expectedTasks} 个子任务`
            }
          />
          <div style={{ marginTop: 16, display: 'flex', justifyContent: 'space-between' }}>
            <Button onClick={() => setStep(0)}>上一步</Button>
            <Button type="primary" disabled={!selectedTargetList.length} onClick={() => setStep(2)}>
              下一步
            </Button>
          </div>
        </Card>
      )}

      {step === 2 && (
        <Card title="第 3 步：统一刊登配置">
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="这里的配置会应用到本次选择的所有商品和刊登目标。后续单独配置会优先生效。"
          />
          <ConfigPriorityBanner />
          <PublishConfigEditor value={commonConfig} onChange={setCommonConfig} />
          <div style={{ marginTop: 16, display: 'flex', justifyContent: 'space-between' }}>
            <Button onClick={() => setStep(1)}>上一步</Button>
            <Button type="primary" onClick={() => setStep(3)}>
              下一步
            </Button>
          </div>
        </Card>
      )}

      {step === 3 && (
        <Card title="第 4 步：单独配置（可选）">
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="当某些商品、平台或店铺需要不同配置时，可以在这里单独设置。单独配置会优先生效。"
          />
          <ConfigPriorityBanner />
          <OverrideConfigTabs
            products={products}
            targets={selectedTargetList}
            overrides={overrides}
            onChange={setOverrides}
            commonConfig={commonConfig}
          />
          {configReminders.length > 0 && (
            <Alert
              type="warning"
              showIcon
              style={{ marginTop: 16 }}
              message="配置提醒"
              description={
                <ul style={{ margin: 0, paddingLeft: 18 }}>
                  {configReminders.map((r) => (
                    <li key={r.key}>{r.message}</li>
                  ))}
                </ul>
              }
            />
          )}
          {configError && (
            <Alert
              type="error"
              showIcon
              style={{ marginTop: 12 }}
              message={configError.title || '刊登配置不正确'}
              description={
                <>
                  <div>{configError.message}</div>
                  {configError.technical ? (
                    <TechnicalDetails label="技术详情">
                      <pre style={{ margin: 0, fontSize: 12 }}>{JSON.stringify(configError.technical, null, 2)}</pre>
                    </TechnicalDetails>
                  ) : null}
                </>
              }
            />
          )}
          <div style={{ marginTop: 16, display: 'flex', justifyContent: 'space-between' }}>
            <Button onClick={() => setStep(2)}>上一步</Button>
            <Button type="primary" loading={checking} onClick={() => void runCheck()}>
              检查并进入下一步
            </Button>
          </div>
        </Card>
      )}

      {step === 4 && checkResult && (
        <Card title="第 5 步：检查并创建">
          <Descriptions bordered size="small" column={{ xs: 1, sm: 2, md: 3 }} style={{ marginBottom: 16 }}>
            <Descriptions.Item label="商品数">{checkResult.summary.productCount}</Descriptions.Item>
            <Descriptions.Item label="目标数">{checkResult.summary.targetCount}</Descriptions.Item>
            <Descriptions.Item label="预计任务数">{checkResult.summary.taskCount}</Descriptions.Item>
            <Descriptions.Item label="可创建">
              <Tag color="green">{checkResult.summary.readyCount}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="建议检查">
              <Tag color="orange">{checkResult.summary.warningCount}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="暂不能创建">
              <Tag color="red">{checkResult.summary.blockedCount}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="仅本地草稿">{checkResult.summary.localDraftOnlyCount}</Descriptions.Item>
          </Descriptions>

          <ConfigPriorityBanner />

          {configReminders.length > 0 && (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 12 }}
              message="配置提醒"
              description={
                <ul style={{ margin: 0, paddingLeft: 18 }}>
                  {configReminders.slice(0, 5).map((r) => (
                    <li key={r.key}>{r.message}</li>
                  ))}
                </ul>
              }
            />
          )}

          <Space style={{ marginBottom: 12 }} wrap>
            <Button onClick={() => setPreviewOpen(true)}>查看生效配置</Button>
          </Space>

          <Table
            rowKey={(r) => `${r.productId}:${r.targetKey}`}
            size="small"
            columns={matrixColumns}
            dataSource={checkResult.items}
            pagination={{
              current: matrixPage,
              pageSize: 20,
              total: checkResult.items.length,
              onChange: setMatrixPage,
              showSizeChanger: false,
            }}
            scroll={{ x: 900 }}
          />

          <Space style={{ marginTop: 16 }} wrap>
            <Button onClick={() => setStep(3)}>返回修改配置</Button>
            <Button
              type="primary"
              loading={creating}
              onClick={() => void runCreate(true)}
              disabled={checkResult.summary.readyCount === 0}
            >
              只创建可处理的草稿
            </Button>
            <Button
              loading={creating}
              onClick={() => void runCreate(false)}
              disabled={checkResult.summary.readyCount + checkResult.summary.warningCount === 0}
            >
              创建可处理项（含建议检查）
            </Button>
          </Space>
          {checkResult.summary.blockedCount > 0 && (
            <Alert
              type="warning"
              showIcon
              style={{ marginTop: 12 }}
              message="暂不能创建的项将自动跳过，不会强行提交。"
            />
          )}
        </Card>
      )}

      <EffectiveConfigPreviewModal
        open={previewOpen}
        onClose={() => setPreviewOpen(false)}
        cells={previewCells}
        commonConfig={commonConfig}
        overrides={overrides}
      />
    </TmPageContainer>
  );
}
