import { PageContainer } from '@ant-design/pro-components';
import { formatDateTime } from '@/utils/formatTime';
import { history, useParams } from '@umijs/max';
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
import dayjs from 'dayjs';
import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  CUSTOMER_CONVERSATION_STATUS,
  ORDER_FULFILLMENT_STATUS,
  ORDER_PAYMENT_STATUS,
  ORDER_SHIPMENT_STATUS,
  ORDER_STATUS,
} from '@/constants/status';
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
  const { id } = useParams<{ id: string }>();
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
    message.success('已关联订单');
    setOrderPickOpen(false);
    loadAll();
  };

  const unlinkOrder = async () => {
    if (!id) return;
    Modal.confirm({
      title: '取消关联订单？',
      onOk: async () => {
        await updateConversation(id, { orderId: '' });
        message.success('已取消关联');
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
      message.warning('请选择店铺');
      return;
    }
    await updateConversation(id, { shopId: pickedShopId });
    message.success('已关联店铺');
    setShopPickOpen(false);
    loadAll();
  };

  const unlinkShop = async () => {
    if (!id) return;
    Modal.confirm({
      title: '取消关联店铺？',
      onOk: async () => {
        await updateConversation(id, { shopId: '' });
        message.success('已取消关联');
        loadAll();
      },
    });
  };

  useEffect(() => {
    loadAll();
  }, [loadAll]);

  const customerMessageOptions = useMemo(() => {
    return msgs.filter((x) => x.role === 'customer').map((m) => ({ label: m.content.slice(0, 48) + (m.content.length > 48 ? '…' : ''), value: m.id }));
  }, [msgs]);

  const onAddCustomerMessage = async () => {
    if (!id) return;
    const t = newCustomerMsg.trim();
    if (!t) {
      message.warning('请输入客户消息');
      return;
    }
    await createMessage(id, { role: 'customer', content: t, language: lang });
    setNewCustomerMsg('');
    message.success('已添加');
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
      message.success('已生成建议（需人工确认，不会对外发送）');
      loadAll();
    } catch (e: unknown) {
      message.error(e instanceof Error ? e.message : '生成失败');
    } finally {
      setGenLoading(false);
    }
  };

  const onCopy = async () => {
    const t = editedReply.trim();
    if (!t) {
      message.warning('没有可复制内容');
      return;
    }
    try {
      await navigator.clipboard.writeText(t);
      message.success('已复制');
    } catch {
      message.error('复制失败');
    }
  };

  const onSaveEdit = async () => {
    if (!suggestionId) {
      message.warning('请先生成建议');
      return;
    }
    await updateReplySuggestion(suggestionId, { editedReply: editedReply.trim() });
    message.success('已保存编辑');
  };

  const onAccept = async () => {
    if (!suggestionId) {
      message.warning('请先生成建议');
      return;
    }
    const finalReply = editedReply.trim();
    if (!finalReply) {
      message.warning('回复内容不能为空');
      return;
    }
    await acceptReplySuggestion(suggestionId, { finalReply });
    message.success('已采纳为内部回复（仅记录，不向平台发送）');
    setSuggestionId(null);
    setAiMeta(null);
    setEditedReply('');
    loadAll();
  };

  const onSendToPlatform = async () => {
    if (!id) return;
    const finalReply = editedReply.trim();
    if (!finalReply) {
      message.warning('请填写要发送到平台的回复内容');
      return;
    }
    Modal.confirm({
      title: '确认发送到平台？',
      content: '将向买家在对应平台发送此回复，请确认内容准确。发送后将在消息时间线中记录为人工外发。',
      okText: '确认发送',
      cancelText: '取消',
      onOk: async () => {
        try {
          await sendPlatformMessage(id, {
            reply: finalReply,
            suggestionId: suggestionId || undefined,
          });
          message.success('已发送到平台');
          loadAll();
        } catch (e: unknown) {
          message.error(e instanceof Error ? e.message : '发送失败');
          return Promise.reject(e);
        }
      },
    });
  };

  const onDiscard = async () => {
    if (!suggestionId) {
      message.warning('没有可废弃的建议');
      return;
    }
    await discardReplySuggestion(suggestionId);
    message.success('已废弃');
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

  const canSendToPlatform = Boolean(conv?.shopId && conv?.externalConversationId);

  return (
    <PageContainer title="AI 客服工作台" onBack={() => history.push('/customer/conversations')}>
      <Spin spinning={loading}>
        {conv && (
          <>
            <Descriptions size="small" column={{ xs: 1, sm: 2, md: 3 }} style={{ marginBottom: 16 }}>
              <Descriptions.Item label="客户">{conv.customerName}</Descriptions.Item>
              <Descriptions.Item label="platform">{conv.platform}</Descriptions.Item>
              <Descriptions.Item label="状态">
                {statusMap ? <Tag color={statusMap.color}>{statusMap.text}</Tag> : <Tag>{conv.status}</Tag>}
              </Descriptions.Item>
              <Descriptions.Item label="店铺">{conv.shopSummary?.shopName ?? '—'}</Descriptions.Item>
              <Descriptions.Item label="外部会话 ID" span={2}>
                <Typography.Text copyable={conv.externalConversationId ? { text: conv.externalConversationId } : false}>
                  {conv.externalConversationId ?? '—'}
                </Typography.Text>
              </Descriptions.Item>
            </Descriptions>
            <Card size="small" title="关联店铺" bordered={false} style={{ marginBottom: 16 }}>
              <Space direction="vertical" style={{ width: '100%' }}>
                <Space wrap>
                  <Button type="primary" onClick={() => void openShopPick()}>
                    选择店铺
                  </Button>
                  {conv.shopId ? (
                    <Button danger onClick={() => void unlinkShop()}>
                      取消关联
                    </Button>
                  ) : null}
                </Space>
                {conv.shopSummary ? (
                  <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
                    <Descriptions.Item label="店铺名">{conv.shopSummary.shopName}</Descriptions.Item>
                    <Descriptions.Item label="平台">{conv.shopSummary.platform}</Descriptions.Item>
                    <Descriptions.Item label="店铺状态">{conv.shopSummary.status}</Descriptions.Item>
                    <Descriptions.Item label="授权">{conv.shopSummary.authStatus}</Descriptions.Item>
                  </Descriptions>
                ) : (
                  <Typography.Text type="secondary">未关联统一店铺（shops）；可选填便于后续按 shop_id 扩展。 </Typography.Text>
                )}
              </Space>
            </Card>
            <Card size="small" title="关联订单" bordered={false} style={{ marginBottom: 16 }}>
              <Space direction="vertical" style={{ width: '100%' }}>
                <Space wrap>
                  <Button
                    type="primary"
                    onClick={() => {
                      setOrderPickOpen(true);
                      void runOrderSearch();
                    }}
                  >
                    选择订单
                  </Button>
                  {conv.orderId ? (
                    <Button danger onClick={() => void unlinkOrder()}>
                      取消关联
                    </Button>
                  ) : null}
                </Space>
                {conv.orderSummary ? (
                  <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
                    <Descriptions.Item label="订单号">{conv.orderSummary.orderNo}</Descriptions.Item>
                    <Descriptions.Item label="订单状态">
                      {mapBizStatus(conv.orderSummary.status, ORDER_STATUS)}
                    </Descriptions.Item>
                    <Descriptions.Item label="支付">{mapBizStatus(conv.orderSummary.paymentStatus, ORDER_PAYMENT_STATUS)}</Descriptions.Item>
                    <Descriptions.Item label="履约">{mapBizStatus(conv.orderSummary.fulfillmentStatus, ORDER_FULFILLMENT_STATUS)}</Descriptions.Item>
                    <Descriptions.Item label="订单金额">{`${conv.orderSummary.currency} ${conv.orderSummary.totalAmount}`}</Descriptions.Item>
                    <Descriptions.Item label="下单时间">
                      {conv.orderSummary.orderedAt ? formatDateTime(conv.orderSummary.orderedAt) : '—'}
                    </Descriptions.Item>
                    <Descriptions.Item label="最新物流状态" span={2}>
                      {conv.orderSummary.latestShipmentStatus
                        ? mapBizStatus(conv.orderSummary.latestShipmentStatus, ORDER_SHIPMENT_STATUS)
                        : '—'}
                    </Descriptions.Item>
                    {(conv.orderSummary.shipments?.length ?? 0) > 0 ? (
                      <Descriptions.Item label="物流明细" span={2}>
                        <Space direction="vertical" size={4} style={{ width: '100%' }}>
                          {(conv.orderSummary.shipments || []).map((s, i) => (
                            <div key={i}>
                              <Typography.Text>
                                [{mapBizStatus(s.status, ORDER_SHIPMENT_STATUS)}] {s.carrier} · {s.trackingNo || '—'}
                              </Typography.Text>
                              {s.trackingUrl ? (
                                <Typography.Link href={s.trackingUrl} target="_blank" style={{ marginLeft: 8 }}>
                                  追踪
                                </Typography.Link>
                              ) : null}
                            </div>
                          ))}
                        </Space>
                      </Descriptions.Item>
                    ) : null}
                  </Descriptions>
                ) : (
                  <Typography.Text type="secondary">未关联手工订单；生成建议时将缺少订单、商品规格、物流等上下文。</Typography.Text>
                )}
              </Space>
            </Card>
          </>
        )}

        <Row gutter={[16, 16]}>
          <Col xs={24} lg={14}>
            <Card title="消息时间线" bordered={false}>
              {msgs.length === 0 ? (
                <Empty description="暂无消息" />
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
                              background: isCustomer ? 'var(--ant-color-fill-quaternary, #fafafa)' : '#e6f4ff',
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
                <Typography.Text strong>添加客户消息</Typography.Text>
                <Input.TextArea
                  rows={3}
                  value={newCustomerMsg}
                  onChange={(e) => setNewCustomerMsg(e.target.value)}
                  placeholder="录入客户原话…"
                  style={{ marginTop: 8 }}
                />
                <Button type="primary" style={{ marginTop: 8 }} onClick={() => void onAddCustomerMessage()}>
                  添加客户消息
                </Button>
              </div>
            </Card>
          </Col>

          <Col xs={24} lg={10}>
            <Card title="AI 回复建议" bordered={false}>
              <Space direction="vertical" style={{ width: '100%' }} size="middle">
                <Alert
                  type="warning"
                  showIcon
                  message="人工审核"
                  description="AI 回复仅为建议；涉及退款、赔偿、投诉、履约承诺等事项请务必人工确认，勿直接对外生效。"
                  style={{ width: '100%' }}
                />
                <div>
                  <Typography.Text type="secondary">针对客户消息（可选，默认取最近一条 customer）</Typography.Text>
                  <Select
                    allowClear
                    placeholder="选择客户消息"
                    style={{ width: '100%', marginTop: 4 }}
                    options={customerMessageOptions}
                    value={focusMessageId}
                    onChange={(v) => setFocusMessageId(v)}
                  />
                </div>
                <Space wrap>
                  <span>language</span>
                  <Input style={{ width: 100 }} value={lang} onChange={(e) => setLang(e.target.value)} />
                  <span>tone</span>
                  <Input style={{ width: 140 }} value={tone} onChange={(e) => setTone(e.target.value)} />
                </Space>
                <div>
                  <Typography.Text>platform</Typography.Text>
                  <Input
                    style={{ marginTop: 4 }}
                    value={platform}
                    onChange={(e) => setPlatform(e.target.value)}
                    placeholder="manual"
                  />
                </div>
                <div>
                  <Typography.Text>shopPolicy（可选）</Typography.Text>
                  <Input.TextArea
                    rows={3}
                    value={shopPolicy}
                    onChange={(e) => setShopPolicy(e.target.value)}
                    style={{ marginTop: 4 }}
                    placeholder="店铺政策摘要，可为空"
                  />
                </div>
                <Button type="primary" loading={genLoading} onClick={() => void onGenerate()}>
                  AI 生成建议回复
                </Button>
                <Typography.Paragraph type="secondary" style={{ marginBottom: 0, fontSize: 12 }}>
                  说明：生成内容需人工确认；系统不会自动向任何外部平台发送消息。
                </Typography.Paragraph>

                {aiMeta && (
                  <Descriptions size="small" column={1} bordered>
                    <Descriptions.Item label="intent">{aiMeta.intent || '—'}</Descriptions.Item>
                    <Descriptions.Item label="sentiment">{aiMeta.sentiment || '—'}</Descriptions.Item>
                    <Descriptions.Item label="riskLevel">{riskTag(aiMeta.riskLevel)}</Descriptions.Item>
                    <Descriptions.Item label="notes">{aiMeta.notes || '—'}</Descriptions.Item>
                  </Descriptions>
                )}
                <div>
                  <Typography.Text strong>回复内容</Typography.Text>
                  <Typography.Paragraph type="secondary" style={{ marginTop: 4, marginBottom: 8, fontSize: 12 }}>
                    可先用 AI 生成并编辑，或直接手写后再「采纳为内部回复」或「发送到平台」（均须人工操作）。
                  </Typography.Paragraph>
                  <Input.TextArea
                    rows={6}
                    value={editedReply}
                    onChange={(e) => setEditedReply(e.target.value)}
                    placeholder="编辑或手写回复内容…"
                  />
                </div>
                <Space wrap align="start">
                  <Button onClick={() => void onSaveEdit()} disabled={!suggestionId}>
                    保存编辑
                  </Button>
                  <Button type="primary" onClick={() => void onAccept()} disabled={!suggestionId}>
                    采纳为内部回复
                  </Button>
                  <Button
                    type="primary"
                    ghost
                    onClick={() => void onSendToPlatform()}
                    disabled={!canSendToPlatform || !editedReply.trim()}
                  >
                    发送到平台
                  </Button>
                  <Button danger onClick={() => void onDiscard()} disabled={!suggestionId}>
                    废弃建议
                  </Button>
                  <Button onClick={() => void onCopy()}>复制回复</Button>
                </Space>
                {!canSendToPlatform ? (
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    「发送到平台」需本会话已关联店铺，且存在平台外部会话 ID（通常由「拉取平台消息」写入）。手工录入会话可无平台外发。
                  </Typography.Text>
                ) : null}
              </Space>
            </Card>
          </Col>
        </Row>

        <Modal
          title="选择关联订单（手工录入的订单）"
          open={orderPickOpen}
          onCancel={() => setOrderPickOpen(false)}
          footer={null}
          width={680}
          destroyOnHidden
        >
          <Space wrap style={{ marginBottom: 12 }}>
            <Input
              placeholder="订单号筛选"
              value={orderFilterNo}
              onChange={(e) => setOrderFilterNo(e.target.value)}
              style={{ width: 180 }}
              allowClear
            />
            <Input
              placeholder="客户姓名"
              value={orderFilterName}
              onChange={(e) => setOrderFilterName(e.target.value)}
              style={{ width: 160 }}
              allowClear
            />
            <Button loading={orderSearchLoading} onClick={() => void runOrderSearch()}>
              查询
            </Button>
          </Space>
          <List<OrderListRow>
            dataSource={orderHits}
            loading={orderSearchLoading}
            locale={{ emptyText: '暂无数据，可先输入条件再查询' }}
            renderItem={(row) => (
              <List.Item
                actions={[
                  <a key="lnk" onClick={() => void linkOrder(row.id)}>
                    关联
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
          title="选择关联店铺"
          open={shopPickOpen}
          onCancel={() => setShopPickOpen(false)}
          onOk={() => void linkShop()}
          okText="关联"
          cancelText="取消"
          destroyOnHidden
        >
          <Select
            showSearch
            optionFilterProp="label"
            placeholder="选择店铺（shops 统一表）"
            style={{ width: '100%' }}
            options={shopOpts}
            value={pickedShopId}
            onChange={(v) => setPickedShopId(v)}
            allowClear
          />
        </Modal>
      </Spin>
    </PageContainer>
  );
}
