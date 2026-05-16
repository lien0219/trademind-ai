import { PageContainer } from '@ant-design/pro-components';
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
  Row,
  Select,
  Space,
  Spin,
  Tag,
  Typography,
} from 'antd';
import dayjs from 'dayjs';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { CUSTOMER_CONVERSATION_STATUS } from '@/constants/status';
import {
  acceptReplySuggestion,
  createMessage,
  discardReplySuggestion,
  generateCustomerReply,
  getConversation,
  queryMessages,
  updateReplySuggestion,
  type ConversationDetail,
  type CustomerMessageRow,
  type GenerateReplyResult,
} from '@/services/customer';

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
    message.success('已采纳为人工回复（仅记录，不对外发送）');
    setSuggestionId(null);
    setAiMeta(null);
    setEditedReply('');
    loadAll();
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

  return (
    <PageContainer title="AI 客服工作台" onBack={() => history.push('/customer/conversations')}>
      <Spin spinning={loading}>
        {conv && (
          <Descriptions size="small" column={3} style={{ marginBottom: 16 }}>
            <Descriptions.Item label="客户">{conv.customerName}</Descriptions.Item>
            <Descriptions.Item label="platform">{conv.platform}</Descriptions.Item>
            <Descriptions.Item label="状态">
              {statusMap ? <Tag color={statusMap.color}>{statusMap.text}</Tag> : <Tag>{conv.status}</Tag>}
            </Descriptions.Item>
          </Descriptions>
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
                                  {dayjs(item.createdAt).format('YYYY-MM-DD HH:mm')}
                                </Typography.Text>
                                <Tag>{item.role}</Tag>
                                <Tag>{item.source}</Tag>
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
                  <>
                    <Descriptions size="small" column={1} bordered>
                      <Descriptions.Item label="intent">{aiMeta.intent || '—'}</Descriptions.Item>
                      <Descriptions.Item label="sentiment">{aiMeta.sentiment || '—'}</Descriptions.Item>
                      <Descriptions.Item label="riskLevel">{riskTag(aiMeta.riskLevel)}</Descriptions.Item>
                      <Descriptions.Item label="notes">{aiMeta.notes || '—'}</Descriptions.Item>
                    </Descriptions>
                    <div>
                      <Typography.Text strong>编辑回复</Typography.Text>
                      <Input.TextArea
                        rows={6}
                        value={editedReply}
                        onChange={(e) => setEditedReply(e.target.value)}
                        style={{ marginTop: 8 }}
                      />
                    </div>
                    <Space wrap>
                      <Button onClick={() => void onSaveEdit()} disabled={!suggestionId}>
                        保存编辑
                      </Button>
                      <Button type="primary" onClick={() => void onAccept()} disabled={!suggestionId}>
                        采纳为回复
                      </Button>
                      <Button danger onClick={() => void onDiscard()} disabled={!suggestionId}>
                        废弃建议
                      </Button>
                      <Button onClick={() => void onCopy()}>复制回复</Button>
                    </Space>
                  </>
                )}
              </Space>
            </Card>
          </Col>
        </Row>
      </Spin>
    </PageContainer>
  );
}
