import { tr, trFormat } from '@/i18n';
import { useLanguage } from '@/i18n/useLanguage';
import { formatDateTime } from '@/utils/formatTime';
import { TmPageContainer } from '@/components/ui';
import { history, useParams, useSearchParams } from '@umijs/max';
import {
  Button,
  Card,
  Col,
  Descriptions,
  Empty,
  Input,
  List,
  message,
  Modal,
  Row,
  Select,
  Space,
  Spin,
  Tag,
  Typography,
  Alert,
} from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  CUSTOMER_CONVERSATION_STATUS,
  ORDER_FULFILLMENT_STATUS,
  ORDER_INVENTORY_DEDUCT_SUMMARY,
  ORDER_PAYMENT_STATUS,
  ORDER_SHIPMENT_STATUS,
  ORDER_SKU_MATCH_SUMMARY,
  ORDER_STATUS,
  SHOP_AUTH_STATUS,
  SHOP_STATUS,
} from '@/constants/status';
import { platformLabel } from '@/constants/userFriendly';
import {
  AI_CONTEXT_PLATFORM_OPTIONS,
  AI_LANGUAGE_OPTIONS,
  AI_TONE_OPTIONS,
} from '@/constants/aiPrompts';
import { confirmCustomerReplySend } from '@/constants/sensitiveActions';
import {
  acceptReplySuggestion,
  createMessage,
  discardReplySuggestion,
  generateCustomerReply,
  getConversation,
  queryMessages,
  sendPlatformMessage,
  updateConversation,
  updateReplySuggestion,
  type ConversationDetail,
  type CustomerMessageRow,
  type GenerateReplyResult,
} from '@/services/customer';
import { queryOrders, type OrderListRow } from '@/services/orders';
import { queryShops } from '@/services/shops';

function mapBizStatus(raw: string, dictionary: Record<string, { text: string; color: string }>) {
  const k = dictionary[raw as keyof typeof dictionary];
  if (!k) return <Tag>{raw}</Tag>;
  return <Tag color={k.color as never}>{k.text}</Tag>;
}

function riskTag(level: string) {
  const l = (level || '').toLowerCase();
  if (l === 'high') return <Tag color="error">high</Tag>;
  if (l === 'medium') return <Tag color="warning">medium</Tag>;
  return <Tag color="success">low</Tag>;
}

export default function CustomerConversationDetailPage() {
  useLanguage();
  const { id } = useParams<{ id: string }>();
  const [searchParams] = useSearchParams();
  const [loading, setLoading] = useState(true);
  const [conv, setConv] = useState<ConversationDetail | null>(null);
  const [msgs, setMsgs] = useState<CustomerMessageRow[]>([]);

  const [newCustomerMsg, setNewCustomerMsg] = useState('');
  const [lang, setLang] = useState('en');
  const [tone, setTone] = useState('professional');
  const [platform, setPlatform] = useState('manual');
  const [shopPolicy, setShopPolicy] = useState('');
  const [focusMessageId, setFocusMessageId] = useState<string | undefined>(undefined);

  const [genLoading, setGenLoading] = useState(false);
  const [suggestionId, setSuggestionId] = useState<string | null>(null);
  const [aiMeta, setAiMeta] = useState<Omit<GenerateReplyResult, 'suggestionId' | 'taskId'> | null>(null);
  const [editedReply, setEditedReply] = useState('');

  const [orderPickOpen, setOrderPickOpen] = useState(false);
  const [orderSearchLoading, setOrderSearchLoading] = useState(false);
  const [orderFilterNo, setOrderFilterNo] = useState('');
  const [orderFilterName, setOrderFilterName] = useState('');
  const [orderHits, setOrderHits] = useState<OrderListRow[]>([]);

  const [shopPickOpen, setShopPickOpen] = useState(false);
  const [shopOpts, setShopOpts] = useState<{ label: string; value: string }[]>([]);
  const [pickedShopId, setPickedShopId] = useState<string | undefined>();

  const loadAll = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const [c, m] = await Promise.all([getConversation(id), queryMessages(id)]);
      setConv(c);
      setMsgs(m.list || []);
      setLang(c.customerLanguage || 'en');
      setPlatform(c.platform || 'manual');
    } finally {
      setLoading(false);
    }
  }, [id]);

  const runOrderSearch = useCallback(async () => {
    setOrderSearchLoading(true);
    try {
      const res = await queryOrders({
        page: 1,
        pageSize: 20,
        orderNo: orderFilterNo.trim() || undefined,
        customerName: orderFilterName.trim() || undefined,
      });
      setOrderHits(res.list || []);
    } finally {
      setOrderSearchLoading(false);
    }
  }, [orderFilterName, orderFilterNo]);

  const linkOrder = async (orderId: string) => {
    if (!id) return;
    await updateConversation(id, { orderId });
    message.success(tr("已关联订单"));
    setOrderPickOpen(false);
    loadAll();
  };

  const unlinkOrder = async () => {
    if (!id) return;
    Modal.confirm({
      title: tr("取消关联订单？"),
      onOk: async () => {
        await updateConversation(id, { orderId: '' });
        message.success(tr("已取消关联"));
        loadAll();
      },
    });
  };

  const openShopPick = async () => {
    const res = await queryShops({ page: 1, pageSize: 300 });
    setShopOpts(
      res.list.map((s) => ({
        label: `${s.shopName} (${s.platform})`,
        value: s.id,
      })),
    );
    setPickedShopId(conv?.shopId);
    setShopPickOpen(true);
  };

  const linkShop = async () => {
    if (!id || !pickedShopId) {
      message.warning(tr("请选择店铺"));
      return;
    }
    await updateConversation(id, { shopId: pickedShopId });
    message.success(tr("已关联店铺"));
    setShopPickOpen(false);
    loadAll();
  };

  const unlinkShop = async () => {
    if (!id) return;
    Modal.confirm({
      title: tr("取消关联店铺？"),
      onOk: async () => {
        await updateConversation(id, { shopId: '' });
        message.success(tr("已取消关联"));
        loadAll();
      },
    });
  };

  useEffect(() => {
    loadAll();
  }, [loadAll]);

  useEffect(() => {
    const sid = searchParams.get('suggestionId')?.trim();
    if (sid) setSuggestionId(sid);
  }, [searchParams]);

  const customerMessageOptions = useMemo(() => {
    return msgs.filter((x) => x.role === 'customer').map((m) => ({ label: m.content.slice(0, 48) + (m.content.length > 48 ? '…' : ''), value: m.id }));
  }, [msgs]);

  const onAddCustomerMessage = async () => {
    if (!id) return;
    const t = newCustomerMsg.trim();
    if (!t) {
      message.warning(tr("请输入客户消息"));
      return;
    }
    await createMessage(id, { role: 'customer', content: t, language: lang });
    setNewCustomerMsg('');
    message.success(tr("已添加"));
    loadAll();
  };

  const onGenerate = async () => {
    if (!id) return;
    setGenLoading(true);
    try {
      const res = await generateCustomerReply(id, {
        messageId: focusMessageId,
        language: lang,
        tone,
        platform,
        shopPolicy,
      });
      setSuggestionId(res.suggestionId);
      setAiMeta({
        reply: res.reply,
        intent: res.intent,
        sentiment: res.sentiment,
        riskLevel: res.riskLevel,
        notes: res.notes,
      });
      setEditedReply(res.reply);
      if (searchParams.get('suggestionId')) {
        setSuggestionId(searchParams.get('suggestionId'));
      }
      message.success(tr("已生成建议（需人工确认，不会对外发送）"));
      loadAll();
    } catch (e: unknown) {
      message.error(e instanceof Error ? e.message : tr('生成失败'));
    } finally {
      setGenLoading(false);
    }
  };

  const onCopy = async () => {
    const t = editedReply.trim();
    if (!t) {
      message.warning(tr("没有可复制内容"));
      return;
    }
    try {
      await navigator.clipboard.writeText(t);
      message.success(tr("已复制"));
    } catch {
      message.error(tr("复制失败"));
    }
  };

  const onSaveEdit = async () => {
    if (!suggestionId) {
      message.warning(tr("请先生成建议"));
      return;
    }
    await updateReplySuggestion(suggestionId, { editedReply: editedReply.trim() });
    message.success(tr("已保存编辑"));
  };

  const onAccept = async () => {
    if (!suggestionId) {
      message.warning(tr("请先生成建议"));
      return;
    }
    const finalReply = editedReply.trim();
    if (!finalReply) {
      message.warning(tr("回复内容不能为空"));
      return;
    }
    await acceptReplySuggestion(suggestionId, { finalReply });
    message.success(tr("已采纳为内部回复（仅记录，不向平台发送）"));
    setSuggestionId(null);
    setAiMeta(null);
    setEditedReply('');
    loadAll();
  };

  // Retain one idempotency key for the same pending send after uncertain outcomes.
  const pendingSendRef = useRef<{ reply: string; conversationId: string; clientMessageId: string } | null>(null);
  const onSendToPlatform = () => {
    if (!id) return;
    const finalReply = editedReply.trim();
    if (!finalReply) {
      message.warning(tr("请填写要发送到平台的回复内容"));
      return;
    }
    const existing = pendingSendRef.current;
    const clientMessageId = existing?.conversationId === id && existing.reply === finalReply
      ? existing.clientMessageId
      : crypto.randomUUID();
    pendingSendRef.current = { conversationId: id, reply: finalReply, clientMessageId };
    confirmCustomerReplySend(canSendToPlatform, async () => {
      try {
        await sendPlatformMessage(id, {
          reply: finalReply,
          clientMessageId,
          suggestionId: suggestionId || undefined,
        });
        pendingSendRef.current = null;
        message.success(tr("已发送到平台"));
        loadAll();
      } catch (e: unknown) {
        message.error(e instanceof Error ? e.message : tr('发送失败'));
        throw e;
      }
    });
  };

  const onDiscard = async () => {
    if (!suggestionId) {
      message.warning(tr("没有可废弃的建议"));
      return;
    }
    await discardReplySuggestion(suggestionId);
    message.success(tr("已废弃"));
    setSuggestionId(null);
    setAiMeta(null);
    setEditedReply('');
  };

  if (!id) {
    return null;
  }

  const statusMap = conv
    ? CUSTOMER_CONVERSATION_STATUS[conv.status as keyof typeof CUSTOMER_CONVERSATION_STATUS]
    : undefined;

  const canSendToPlatform = Boolean(conv?.shopId && conv?.externalConversationId && conv?.canWrite !== false);
  const readOnly = conv?.canWrite === false;

  return (
    <TmPageContainer title={tr("AI 客服工作台")} onBack={() => history.goBack()}>
      <Spin spinning={loading}>
        {conv && (
          <>
            {conv.openFailureCount ? (
              <Alert
                type="error"
                showIcon
                style={{ marginBottom: 16 }}
                message={trFormat("有 {count} 条未处理客服失败", { count: conv.openFailureCount })}
                action={
                  <Button size="small" onClick={() => history.push('/ops/task-center/failures?taskType=customer_failure')}>
                    {tr("查看失败任务")}
                  </Button>
                }
              />
            ) : null}
            {readOnly ? (
              <Alert type="info" showIcon style={{ marginBottom: 16 }} message={tr("当前为只读账号，不可生成建议或发送消息。")} />
            ) : null}
            {conv.contextSummary?.incompleteWarning ? (
              <Alert type="warning" showIcon style={{ marginBottom: 16 }} message={conv.contextSummary.incompleteWarning} />
            ) : null}
            <Descriptions size="small" column={{ xs: 1, sm: 2, md: 3 }} style={{ marginBottom: 16 }}>
              <Descriptions.Item label={tr("客户")}>{conv.customerNameMasked || conv.customerName}</Descriptions.Item>
              <Descriptions.Item label={tr("平台")}>{platformLabel(conv.platform)}</Descriptions.Item>
              <Descriptions.Item label={tr("状态")}>
                {statusMap ? <Tag color={statusMap.color}>{statusMap.text}</Tag> : <Tag>{conv.status}</Tag>}
              </Descriptions.Item>
              <Descriptions.Item label={tr("店铺")}>{conv.shopSummary?.shopName ?? '—'}</Descriptions.Item>
              <Descriptions.Item label={tr("外部会话 ID")} span={2}>
                <Typography.Text copyable={conv.externalConversationId ? { text: conv.externalConversationId } : false}>
                  {conv.externalConversationId ?? '—'}
                </Typography.Text>
              </Descriptions.Item>
            </Descriptions>
            <Card size="small" title={tr("关联店铺")} variant="borderless" style={{ marginBottom: 16 }}>
              <Space direction="vertical" style={{ width: '100%' }}>
                <Space wrap>
                  <Button type="primary" onClick={() => void openShopPick()}>
                    {tr("选择店铺")}
                  </Button>
                  {conv.shopId ? (
                    <Button danger onClick={() => void unlinkShop()}>
                      {tr("取消关联")}
                    </Button>
                  ) : null}
                </Space>
                {conv.shopSummary ? (
                  <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
                    <Descriptions.Item label={tr("店铺名")}>{conv.shopSummary.shopName}</Descriptions.Item>
                    <Descriptions.Item label={tr("平台")}>{platformLabel(conv.shopSummary.platform)}</Descriptions.Item>
                    <Descriptions.Item label={tr("店铺状态")}>
                      {(() => {
                        const m = SHOP_STATUS[conv.shopSummary.status as keyof typeof SHOP_STATUS];
                        return m ? <Tag color={m.color}>{m.text}</Tag> : conv.shopSummary.status || '—';
                      })()}
                    </Descriptions.Item>
                    <Descriptions.Item label={tr("授权状态")}>
                      {(() => {
                        const m = SHOP_AUTH_STATUS[conv.shopSummary.authStatus as keyof typeof SHOP_AUTH_STATUS];
                        return m ? <Tag color={m.color}>{m.text}</Tag> : conv.shopSummary.authStatus || '—';
                      })()}
                    </Descriptions.Item>
                  </Descriptions>
                ) : (
                  <Typography.Text type="secondary">{tr("未关联统一店铺（shops）；可选填便于后续按 shop_id 扩展。")} </Typography.Text>
                )}
              </Space>
            </Card>
            <Card size="small" title={tr("关联订单")} variant="borderless" style={{ marginBottom: 16 }}>
              <Space direction="vertical" style={{ width: '100%' }}>
                <Space wrap>
                  <Button
                    type="primary"
                    onClick={() => {
                      setOrderPickOpen(true);
                      void runOrderSearch();
                    }}
                  >
                    {tr("选择订单")}
                  </Button>
                  {conv.orderId ? (
                    <Button danger onClick={() => void unlinkOrder()}>
                      {tr("取消关联")}
                    </Button>
                  ) : null}
                </Space>
                {conv.orderSummary ? (
                  <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
                    <Descriptions.Item label={tr("订单号")}>{conv.orderSummary.orderNo}</Descriptions.Item>
                    <Descriptions.Item label={tr("订单状态")}>
                      {mapBizStatus(conv.orderSummary.status, ORDER_STATUS)}
                    </Descriptions.Item>
                    <Descriptions.Item label={tr("支付")}>{mapBizStatus(conv.orderSummary.paymentStatus, ORDER_PAYMENT_STATUS)}</Descriptions.Item>
                    <Descriptions.Item label={tr("履约")}>{mapBizStatus(conv.orderSummary.fulfillmentStatus, ORDER_FULFILLMENT_STATUS)}</Descriptions.Item>
                    <Descriptions.Item label={tr("订单金额")}>{`${conv.orderSummary.currency} ${conv.orderSummary.totalAmount}`}</Descriptions.Item>
                    <Descriptions.Item label={tr("SKU 匹配")}>
                      {mapBizStatus(conv.orderSummary.skuMatchStatus || 'none', ORDER_SKU_MATCH_SUMMARY)}
                    </Descriptions.Item>
                    <Descriptions.Item label={tr("库存扣减")}>
                      {mapBizStatus(conv.orderSummary.inventoryDeductStatus || 'none', ORDER_INVENTORY_DEDUCT_SUMMARY)}
                    </Descriptions.Item>
                    <Descriptions.Item label={tr("操作")} span={2}>
                      <Space wrap>
                        <Button size="small" onClick={() => history.push(`/orders/${conv.orderId}`)}>
                          {tr("查看订单详情")}
                        </Button>
                        <Button size="small" onClick={() => history.push(`/orders/${conv.orderId}?tab=exceptions`)}>
                          {tr("查看订单异常")}
                        </Button>
                        <Button size="small" onClick={() => history.push(`/orders/${conv.orderId}?tab=inventory`)}>
                          {tr("查看库存影响")}
                        </Button>
                      </Space>
                    </Descriptions.Item>
                    <Descriptions.Item label={tr("下单时间")}>
                      {conv.orderSummary.orderedAt ? formatDateTime(conv.orderSummary.orderedAt) : '—'}
                    </Descriptions.Item>
                    <Descriptions.Item label={tr("最新物流状态")} span={2}>
                      {conv.orderSummary.latestShipmentStatus
                        ? mapBizStatus(conv.orderSummary.latestShipmentStatus, ORDER_SHIPMENT_STATUS)
                        : '—'}
                    </Descriptions.Item>
                    {(conv.orderSummary.shipments?.length ?? 0) > 0 ? (
                      <Descriptions.Item label={tr("物流明细")} span={2}>
                        <Space direction="vertical" size={4} style={{ width: '100%' }}>
                          {(conv.orderSummary.shipments || []).map((s, i) => (
                            <div key={i}>
                              <Typography.Text>
                                [{mapBizStatus(s.status, ORDER_SHIPMENT_STATUS)}] {s.carrier} · {s.trackingNo || '—'}
                              </Typography.Text>
                              {s.trackingUrl ? (
                                <Typography.Link href={s.trackingUrl} target="_blank" style={{ marginLeft: 8 }}>
                                  {tr("追踪")}
                                </Typography.Link>
                              ) : null}
                            </div>
                          ))}
                        </Space>
                      </Descriptions.Item>
                    ) : null}
                  </Descriptions>
                ) : (
                  <Typography.Text type="secondary">{tr("未关联订单；生成建议时将缺少订单、商品、库存等上下文。")}</Typography.Text>
                )}
              </Space>
            </Card>
            {(conv.productContexts?.length ?? 0) > 0 ? (
              <Card size="small" title={tr("关联商品")} variant="borderless" style={{ marginBottom: 16 }}>
                <List
                  size="small"
                  dataSource={conv.productContexts}
                  renderItem={(p) => (
                    <List.Item>
                      <Space direction="vertical" size={0}>
                        <Typography.Text strong>{p.productTitle || '—'}</Typography.Text>
                        <Typography.Text type="secondary">
                          SKU: {p.skuCode || '—'} · {p.skuName || '—'} · {tr('库存')}:{' '}{p.stockStatus || '—'}
                        </Typography.Text>
                        {p.productId ? (
                          <Space>
                            <Typography.Link onClick={() => history.push(`/product/drafts/${p.productId}`)}>
                              {tr("查看商品")}
                            </Typography.Link>
                            <Typography.Link onClick={() => history.push(`/product/drafts/${p.productId}?tab=inventory`)}>
                              {tr("查看库存")}
                            </Typography.Link>
                          </Space>
                        ) : null}
                      </Space>
                    </List.Item>
                  )}
                />
              </Card>
            ) : null}
            {(conv.inventoryContexts?.length ?? 0) > 0 ? (
              <Card size="small" title={tr("库存 / SKU 上下文")} variant="borderless" style={{ marginBottom: 16 }}>
                <List
                  size="small"
                  dataSource={conv.inventoryContexts}
                  renderItem={(p) => (
                    <List.Item>
                      {p.skuCode} · {p.skuName} · {tr('库存')} {p.stock ?? '—'} · {p.stockStatus} · {tr('绑定')} {p.bindStatus}
                    </List.Item>
                  )}
                />
              </Card>
            ) : null}
            {conv.contextSummary ? (
              <Card size="small" title={tr("AI 上下文摘要")} variant="borderless" style={{ marginBottom: 16 }}>
                <Descriptions size="small" column={{ xs: 1, sm: 2 }}>
                  <Descriptions.Item label={tr("订单状态")}>{conv.contextSummary.orderStatus || '—'}</Descriptions.Item>
                  <Descriptions.Item label={tr("SKU 匹配")}>{conv.contextSummary.skuMatchStatus || '—'}</Descriptions.Item>
                  <Descriptions.Item label={tr("库存状态")}>{conv.contextSummary.inventoryStatus || '—'}</Descriptions.Item>
                  <Descriptions.Item label={tr("商品")}>{conv.contextSummary.productTitle || '—'}</Descriptions.Item>
                  <Descriptions.Item label={tr("客户问题")} span={2}>
                    {conv.contextSummary.customerQuestion || '—'}
                  </Descriptions.Item>
                </Descriptions>
              </Card>
            ) : null}
          </>
        )}

        <Row gutter={[16, 16]}>
          <Col xs={24} lg={14}>
            <Card title={tr("消息时间线")} variant="borderless">
              {msgs.length === 0 ? (
                <Empty description={tr("暂无消息")} />
              ) : (
                <List
                  itemLayout="vertical"
                  dataSource={msgs}
                  renderItem={(item) => {
                    const isCustomer = item.role === 'customer';
                    return (
                      <List.Item style={{ borderBlockEnd: 'none' }}>
                        <div
                          style={{
                            display: 'flex',
                            justifyContent: isCustomer ? 'flex-start' : 'flex-end',
                          }}
                        >
                          <Card
                            size="small"
                            style={{
                              maxWidth: '92%',
                              background: isCustomer ? 'var(--ant-color-fill-quaternary)' : 'var(--ant-color-primary-bg)',
                            }}
                            title={
                              <Space size={8}>
                                <Typography.Text type="secondary">
                                  {formatDateTime(item.createdAt)}
                                </Typography.Text>
                                <Tag>{item.role}</Tag>
                                <Tag>{item.source}</Tag>
                                {item.messageType ? <Tag>{item.messageType}</Tag> : null}
                              </Space>
                            }
                          >
                            <Typography.Paragraph style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}>
                              {item.content}
                            </Typography.Paragraph>
                          </Card>
                        </div>
                      </List.Item>
                    );
                  }}
                />
              )}
              <div style={{ marginTop: 16 }}>
                <Typography.Text strong>{tr("添加客户消息")}</Typography.Text>
                <Input.TextArea
                  rows={3}
                  value={newCustomerMsg}
                  onChange={(e) => setNewCustomerMsg(e.target.value)}
                  placeholder={tr("录入客户原话…")}
                  style={{ marginTop: 8 }}
                />
                <Button type="primary" style={{ marginTop: 8 }} onClick={() => void onAddCustomerMessage()}>
                  {tr("添加客户消息")}
                </Button>
              </div>
            </Card>
          </Col>

          <Col xs={24} lg={10}>
            <Card title={tr("AI 回复建议")} variant="borderless">
              <Space direction="vertical" style={{ width: '100%' }} size="middle">
                <Alert
                  type="warning"
                  showIcon
                  message={tr("人工审核")}
                  description={tr("AI 回复仅为建议；涉及退款、赔偿、投诉、履约承诺等事项请务必人工确认，勿直接对外生效。")}
                  style={{ width: '100%' }}
                />
                <div>
                  <Typography.Text type="secondary">{tr("针对客户消息（可选，默认取最近一条 customer）")}</Typography.Text>
                  <Select
                    allowClear
                    placeholder={tr("选择客户消息")}
                    style={{ width: '100%', marginTop: 4 }}
                    options={customerMessageOptions}
                    value={focusMessageId}
                    onChange={(v) => setFocusMessageId(v)}
                  />
                </div>
                <Space wrap>
                  <span>{tr("回复语言")}</span>
                  <Select
                    style={{ width: 140 }}
                    options={[...AI_LANGUAGE_OPTIONS]}
                    value={lang}
                    onChange={setLang}
                  />
                  <span>{tr("回复语气")}</span>
                  <Select
                    style={{ width: 140 }}
                    options={[...AI_TONE_OPTIONS]}
                    value={tone}
                    onChange={setTone}
                  />
                </Space>
                <div>
                  <Typography.Text>{tr("会话平台")}</Typography.Text>
                  <Select
                    style={{ marginTop: 4 }}
                    options={AI_CONTEXT_PLATFORM_OPTIONS}
                    value={platform}
                    onChange={setPlatform}
                    placeholder={tr("请选择会话平台")}
                  />
                </div>
                <div>
                  <Typography.Text>{tr("shopPolicy（可选）")}</Typography.Text>
                  <Input.TextArea
                    rows={3}
                    value={shopPolicy}
                    onChange={(e) => setShopPolicy(e.target.value)}
                    style={{ marginTop: 4 }}
                    placeholder={tr("店铺政策摘要，可为空")}
                  />
                </div>
                <Button type="primary" loading={genLoading} disabled={readOnly} onClick={() => void onGenerate()}>
                  {tr("AI 生成建议回复")}
                </Button>
                <Typography.Paragraph type="secondary" style={{ marginBottom: 0, fontSize: 12 }}>
                  {tr("说明：生成内容需人工确认；系统不会自动向任何外部平台发送消息。")}
                </Typography.Paragraph>

                {aiMeta && (
                  <Descriptions size="small" column={1} bordered>
                    <Descriptions.Item label={tr("买家意图")}>{aiMeta.intent || '—'}</Descriptions.Item>
                    <Descriptions.Item label={tr("情绪倾向")}>{aiMeta.sentiment || '—'}</Descriptions.Item>
                    <Descriptions.Item label={tr("风险等级")}>{riskTag(aiMeta.riskLevel)}</Descriptions.Item>
                    <Descriptions.Item label={tr("备注")}>{aiMeta.notes || '—'}</Descriptions.Item>
                  </Descriptions>
                )}
                <div>
                  <Typography.Text strong>{tr("回复内容")}</Typography.Text>
                  <Typography.Paragraph type="secondary" style={{ marginTop: 4, marginBottom: 8, fontSize: 12 }}>
                    {tr("可先用 AI 生成并编辑，或直接手写后再「采纳为内部回复」或「发送到平台」（均须人工操作）。")}
                  </Typography.Paragraph>
                  <Input.TextArea
                    rows={6}
                    value={editedReply}
                    onChange={(e) => setEditedReply(e.target.value)}
                    placeholder={tr("编辑或手写回复内容…")}
                  />
                </div>
                <Space wrap align="start">
                  <Button onClick={() => void onSaveEdit()} disabled={readOnly || !suggestionId}>
                    {tr("保存编辑")}
                  </Button>
                  <Button type="primary" onClick={() => void onAccept()} disabled={readOnly || !suggestionId}>
                    {tr("采纳为内部回复")}
                  </Button>
                  <Button
                    type="primary"
                    ghost
                    onClick={() => void onSendToPlatform()}
                    disabled={readOnly || !canSendToPlatform || !editedReply.trim()}
                  >
                    {tr("发送到平台")}
                  </Button>
                  <Button danger onClick={() => void onDiscard()} disabled={!suggestionId}>
                    {tr("废弃建议")}
                  </Button>
                  <Button onClick={() => void onCopy()}>{tr("复制回复")}</Button>
                </Space>
                {!canSendToPlatform ? (
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    {tr("「发送到平台」需本会话已关联店铺，且存在平台外部会话 ID（通常由「拉取平台消息」写入）。手工录入会话可无平台外发。")}
                  </Typography.Text>
                ) : null}
              </Space>
            </Card>
          </Col>
        </Row>

        <Modal
          title={tr("选择关联订单（手工录入的订单）")}
          open={orderPickOpen}
          onCancel={() => setOrderPickOpen(false)}
          footer={null}
          width={680}
          destroyOnHidden
        >
          <Space wrap style={{ marginBottom: 12 }}>
            <Input
              placeholder={tr("订单号筛选")}
              value={orderFilterNo}
              onChange={(e) => setOrderFilterNo(e.target.value)}
              style={{ width: 180 }}
              allowClear
            />
            <Input
              placeholder={tr("客户姓名")}
              value={orderFilterName}
              onChange={(e) => setOrderFilterName(e.target.value)}
              style={{ width: 160 }}
              allowClear
            />
            <Button loading={orderSearchLoading} onClick={() => void runOrderSearch()}>
              {tr("查询")}
            </Button>
          </Space>
          <List<OrderListRow>
            dataSource={orderHits}
            loading={orderSearchLoading}
            locale={{ emptyText: tr("暂无数据，可先输入条件再查询") }}
            renderItem={(row) => (
              <List.Item
                actions={[
                  <a key="lnk" onClick={() => void linkOrder(row.id)}>
                    {tr("关联")}
                  </a>,
                ]}
              >
                <List.Item.Meta
                  title={`${row.orderNo} · ${row.customerName}`}
                  description={
                    <Space wrap size={8}>
                      {mapBizStatus(row.status, ORDER_STATUS)}
                      {mapBizStatus(row.paymentStatus, ORDER_PAYMENT_STATUS)}
                      {mapBizStatus(row.fulfillmentStatus, ORDER_FULFILLMENT_STATUS)}
                      <Typography.Text type="secondary">
                        {row.currency} {row.totalAmount}
                      </Typography.Text>
                    </Space>
                  }
                />
              </List.Item>
            )}
          />
        </Modal>

        <Modal
          title={tr("选择关联店铺")}
          open={shopPickOpen}
          onCancel={() => setShopPickOpen(false)}
          onOk={() => void linkShop()}
          okText={tr("关联")}
          cancelText={tr("取消")}
          destroyOnHidden
        >
          <Select
            showSearch
            optionFilterProp="label"
            placeholder={tr("选择店铺（shops 统一表）")}
            style={{ width: '100%' }}
            options={shopOpts}
            value={pickedShopId}
            onChange={(v) => setPickedShopId(v)}
            allowClear
          />
        </Modal>
      </Spin>
    </TmPageContainer>
  );
}
