import TechnicalDetails from '@/components/ui/TechnicalDetails';
import { TmPageContainer } from '@/components/ui';
import {
  COMMON_PUBLISH_CONFIG_LABEL,
  publishBatchStatusLabel,
  publishCapabilityLabel,
  publishTargetStatusLabel,
} from '@/constants/publishLabels';
import { PRODUCT_STATUS } from '@/constants/status';
import { productSourceLabel } from '@/constants/userFriendly';
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
import { Link, history, useLocation } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Col,
  Descriptions,
  Form,
  Image,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Spin,
  Steps,
  Table,
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

export default function PublishBatchWizardPage() {
  const location = useLocation();
  const initialIds = useMemo(() => parseProductIdsFromSearch(location.search), [location.search]);

  const [step, setStep] = useState(0);
  const [products, setProducts] = useState<ProductListRow[]>([]);
  const [loadingProducts, setLoadingProducts] = useState(true);
  const [platforms, setPlatforms] = useState<PublishTargetPlatform[]>([]);
  const [loadingPlatforms, setLoadingPlatforms] = useState(false);
  const [selectedTargets, setSelectedTargets] = useState<Record<string, SelectedTarget>>({});
  const [commonConfig, setCommonConfig] = useState<Record<string, unknown>>({});
  const [overrides, setOverrides] = useState<PublishConfigOverrides>({});
  const [checkResult, setCheckResult] = useState<BatchTargetsCheckResponse | null>(null);
  const [checking, setChecking] = useState(false);
  const [creating, setCreating] = useState(false);
  const [matrixPage, setMatrixPage] = useState(1);

  const productIds = useMemo(() => products.map((p) => p.id), [products]);
  const selectedTargetList = useMemo(() => Object.values(selectedTargets), [selectedTargets]);
  const expectedTasks = productIds.length * selectedTargetList.length;

  const loadProducts = useCallback(async (ids: string[]) => {
    if (!ids.length) {
      setProducts([]);
      setLoadingProducts(false);
      return;
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
        next[key] = { platform: platform.platform, shopId, shopName, targetKey: key };
      } else {
        delete next[key];
      }
      return next;
    });
    setCheckResult(null);
  };

  const runCheck = async () => {
    if (!productIds.length || !selectedTargetList.length) {
      message.warning('请先选择商品和刊登目标');
      return;
    }
    setChecking(true);
    try {
      const res = await checkBatchPublishTargets({
        productIds,
        targets: selectedTargetList.map(({ platform, shopId }) => ({ platform, shopId })),
        commonConfig,
        overrides,
      });
      setCheckResult(res);
      setStep(4);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '检查失败');
    } finally {
      setChecking(false);
    }
  };

  const runCreate = async (onlyReady: boolean) => {
    setCreating(true);
    try {
      const res = await createBatchPublishDrafts({
        productIds,
        targets: selectedTargetList.map(({ platform, shopId }) => ({ platform, shopId })),
        commonConfig,
        overrides,
        onlyReady,
        includeWarnings: !onlyReady,
        name: `批量刊登 ${productIds.length} 商品`,
      });
      message.success(`批次已创建：${res.successCount} 成功，${res.failedCount} 失败，${res.skippedCount} 跳过`);
      history.push(`/product/publish-batches/${res.batchId}`);
    } catch (e: unknown) {
      message.error((e as Error)?.message || '创建失败');
    } finally {
      setCreating(false);
    }
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
      onBack={() => history.push('/product/drafts')}
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
            type="info"
            showIcon
            style={{ marginTop: 16 }}
            message={`已选 ${selectedTargetList.length} 个刊登目标，预计 ${expectedTasks} 个子任务`}
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
        <Card title="第 3 步：统一配置">
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="统一配置将作用于本批次所有商品和刊登目标，可在下一步按商品 / 平台 / 店铺单独覆盖。"
          />
          <Form layout="vertical">
            <Row gutter={16}>
              <Col xs={24} md={12}>
                <Form.Item label={COMMON_PUBLISH_CONFIG_LABEL.priceRule}>
                  <Input
                    placeholder="例如：成本价 × 2 + 运费"
                    value={String(commonConfig.priceRule ?? '')}
                    onChange={(e) => setCommonConfig((p) => ({ ...p, priceRule: e.target.value }))}
                  />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={COMMON_PUBLISH_CONFIG_LABEL.imageStrategy}>
                  <Select
                    allowClear
                    placeholder="选择图片策略"
                    value={commonConfig.imageStrategy as string | undefined}
                    onChange={(v) => setCommonConfig((p) => ({ ...p, imageStrategy: v }))}
                    options={[
                      { label: '使用主图', value: 'main_only' },
                      { label: '主图 + 详情图', value: 'main_and_detail' },
                    ]}
                  />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={COMMON_PUBLISH_CONFIG_LABEL.stockStrategy}>
                  <Select
                    allowClear
                    placeholder="选择库存策略"
                    value={commonConfig.stockStrategy as string | undefined}
                    onChange={(v) => setCommonConfig((p) => ({ ...p, stockStrategy: v }))}
                    options={[
                      { label: '同步本地库存', value: 'sync_local' },
                      { label: '固定库存', value: 'fixed' },
                    ]}
                  />
                </Form.Item>
              </Col>
              <Col xs={24} md={8}>
                <Form.Item label={COMMON_PUBLISH_CONFIG_LABEL.packageWeight}>
                  <InputNumber
                    style={{ width: '100%' }}
                    min={0}
                    addonAfter="kg"
                    value={commonConfig.packageWeight as number | undefined}
                    onChange={(v) => setCommonConfig((p) => ({ ...p, packageWeight: v }))}
                  />
                </Form.Item>
              </Col>
              <Col xs={24} md={16}>
                <Form.Item label={COMMON_PUBLISH_CONFIG_LABEL.packageSize}>
                  <Input
                    placeholder="长×宽×高 cm"
                    value={String(commonConfig.packageSize ?? '')}
                    onChange={(e) => setCommonConfig((p) => ({ ...p, packageSize: e.target.value }))}
                  />
                </Form.Item>
              </Col>
              <Col span={24}>
                <Form.Item label={COMMON_PUBLISH_CONFIG_LABEL.remark}>
                  <Input.TextArea
                    rows={2}
                    value={String(commonConfig.remark ?? '')}
                    onChange={(e) => setCommonConfig((p) => ({ ...p, remark: e.target.value }))}
                  />
                </Form.Item>
              </Col>
            </Row>
          </Form>
          <div style={{ marginTop: 16, display: 'flex', justifyContent: 'space-between' }}>
            <Button onClick={() => setStep(1)}>上一步</Button>
            <Button type="primary" onClick={() => setStep(3)}>
              下一步
            </Button>
          </div>
        </Card>
      )}

      {step === 3 && (
        <Card title="第 4 步：单独覆盖（可选）">
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="单独配置会优先生效，不会影响其他商品、平台或店铺。"
          />
          <Form layout="vertical">
            <Form.Item label="按商品覆盖价格规则">
              <Select
                allowClear
                placeholder="选择商品"
                style={{ width: '100%', marginBottom: 8 }}
                onChange={(pid) => {
                  if (!pid) return;
                  const val = window.prompt('输入该商品的价格规则覆盖');
                  if (val == null) return;
                  setOverrides((o) => ({
                    ...o,
                    products: { ...(o.products ?? {}), [pid]: { priceRule: val } },
                  }));
                }}
                options={products.map((p) => ({ label: p.title, value: p.id }))}
              />
            </Form.Item>
            <Form.Item label="按平台覆盖图片策略">
              <Select
                allowClear
                placeholder="选择平台"
                style={{ width: '100%' }}
                onChange={(plat) => {
                  if (!plat) return;
                  setOverrides((o) => ({
                    ...o,
                    platforms: {
                      ...(o.platforms ?? {}),
                      [plat]: { imageStrategy: 'main_only' },
                    },
                  }));
                  message.success('已添加平台覆盖（可在技术详情查看）');
                }}
                options={selectedTargetList.map((t) => ({ label: t.platform, value: t.platform }))}
              />
            </Form.Item>
            <Form.Item label="按店铺覆盖包裹重量">
              <Select
                allowClear
                placeholder="选择店铺"
                style={{ width: '100%' }}
                onChange={(shopId) => {
                  if (!shopId) return;
                  setOverrides((o) => ({
                    ...o,
                    shops: { ...(o.shops ?? {}), [shopId]: { packageWeight: 0.5 } },
                  }));
                  message.success('已添加店铺覆盖（可在技术详情查看）');
                }}
                options={selectedTargetList.map((t) => ({
                  label: t.shopName || t.shopId,
                  value: t.shopId,
                }))}
              />
            </Form.Item>
          </Form>
          {Object.keys(overrides.products ?? {}).length +
            Object.keys(overrides.platforms ?? {}).length +
            Object.keys(overrides.shops ?? {}).length >
            0 && (
            <TechnicalDetails label="已配置的覆盖项">
              <pre style={{ fontSize: 12, margin: 0 }}>{JSON.stringify(overrides, null, 2)}</pre>
            </TechnicalDetails>
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
            <Descriptions.Item label="仅本地草稿">
              {checkResult.summary.localDraftOnlyCount}
            </Descriptions.Item>
          </Descriptions>

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
    </TmPageContainer>
  );
}
