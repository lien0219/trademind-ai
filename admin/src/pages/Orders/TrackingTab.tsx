import { useLanguage } from '@/i18n/useLanguage';
import { tr, trFormat } from '@/i18n';
import {
  Alert,
  Button,
  Card,
  DatePicker,
  Empty,
  Form,
  Input,
  List,
  Modal,
  Row,
  Col,
  Select,
  Space,
  Tag,
  Timeline,
  Typography,
  message,
} from "antd";
import dayjs, { type Dayjs } from "dayjs";
import { useCallback, useEffect, useMemo, useState } from "react";
import { ORDER_SHIPMENT_STATUS } from "@/constants/status";
import { formatDateTime } from "@/utils/formatTime";
import {
  appendOrderShipmentEvent,
  getOrderShipmentEvents,
  getOrderShipments,
  type OrderShipmentEventRow,
  type OrderShipmentRow,
} from "@/services/orders";

type Props = {
  orderId: string;
  readOnly?: boolean;
};

type EventFormValues = {
  eventKey: string;
  status: string;
  occurredAt: Dayjs;
  location?: string;
  description?: string;
};

const eventStatuses = Object.entries(ORDER_SHIPMENT_STATUS).map(([value, cfg]) => ({
  value,
  label: cfg.text,
}));

function shipmentTag(status: string) {
  const cfg = ORDER_SHIPMENT_STATUS[status as keyof typeof ORDER_SHIPMENT_STATUS];
  return cfg ? <Tag color={cfg.color}>{cfg.text}</Tag> : <Tag>{status || tr("未知")}</Tag>;
}

function timelineColor(status: string) {
  const cfg = ORDER_SHIPMENT_STATUS[status as keyof typeof ORDER_SHIPMENT_STATUS];
  return cfg?.color || "gray";
}

function eventTitle(event: OrderShipmentEventRow) {
  return (
    <Space size={8} wrap>
      {shipmentTag(event.status)}
      <Typography.Text strong>{formatDateTime(event.occurredAt)}</Typography.Text>
    </Space>
  );
}

export default function OrderTrackingTab({ orderId, readOnly = false }: Props) {
  useLanguage();
  const [shipments, setShipments] = useState<OrderShipmentRow[]>([]);
  const [shipmentsLoading, setShipmentsLoading] = useState(false);
  const [shipmentsError, setShipmentsError] = useState<string | null>(null);
  const [selectedShipmentId, setSelectedShipmentId] = useState<string>();
  const [events, setEvents] = useState<OrderShipmentEventRow[]>([]);
  const [provider, setProvider] = useState("local");
  const [eventsLoading, setEventsLoading] = useState(false);
  const [eventsError, setEventsError] = useState<string | null>(null);
  const [eventModalOpen, setEventModalOpen] = useState(false);
  const [eventSubmitting, setEventSubmitting] = useState(false);
  const [eventForm] = Form.useForm<EventFormValues>();

  const selectedShipment = useMemo(
    () => shipments.find((shipment) => shipment.id === selectedShipmentId),
    [shipments, selectedShipmentId],
  );

  const loadShipments = useCallback(async () => {
    setShipmentsLoading(true);
    setShipmentsError(null);
    try {
      const response = await getOrderShipments(orderId);
      const rows = response.list ?? [];
      setShipments(rows);
      setSelectedShipmentId((current) =>
        current && rows.some((shipment) => shipment.id === current)
          ? current
          : rows[0]?.id,
      );
    } catch (error: unknown) {
      setShipments([]);
      setSelectedShipmentId(undefined);
      setShipmentsError((error as Error)?.message || "物流包裹加载失败");
    } finally {
      setShipmentsLoading(false);
    }
  }, [orderId]);

  const loadEvents = useCallback(async (shipmentId: string) => {
    setEventsLoading(true);
    setEventsError(null);
    try {
      const response = await getOrderShipmentEvents(orderId, shipmentId);
      setEvents(response.events ?? []);
      setProvider(response.provider || "local");
    } catch (error: unknown) {
      setEvents([]);
      setEventsError((error as Error)?.message || "物流轨迹加载失败");
    } finally {
      setEventsLoading(false);
    }
  }, [orderId]);

  useEffect(() => {
    void loadShipments();
  }, [loadShipments]);

  useEffect(() => {
    if (!selectedShipmentId) {
      setEvents([]);
      setEventsError(null);
      return;
    }
    void loadEvents(selectedShipmentId);
  }, [loadEvents, selectedShipmentId]);

  const openEventModal = () => {
    eventForm.resetFields();
    eventForm.setFieldsValue({
      status: selectedShipment?.status || "in_transit",
      occurredAt: dayjs(),
    });
    setEventModalOpen(true);
  };

  const submitEvent = async (values: EventFormValues) => {
    if (!selectedShipment || eventSubmitting) return;
    setEventSubmitting(true);
    try {
      const response = await appendOrderShipmentEvent(orderId, selectedShipment.id, {
        eventKey: values.eventKey.trim(),
        status: values.status,
        occurredAt: values.occurredAt?.toISOString(),
        location: values.location?.trim() || undefined,
        description: values.description?.trim() || undefined,
      });
      setShipments((current) =>
        current.map((shipment) =>
          shipment.id === response.shipment.id ? response.shipment : shipment,
        ),
      );
      setEventModalOpen(false);
      message.success(response.replay ? tr("物流事件已幂等重放") : tr("物流事件已记录"));
      await loadEvents(selectedShipment.id);
    } catch (error: unknown) {
      message.error(tr((error as Error)?.message || tr("物流事件记录失败")));
    } finally {
      setEventSubmitting(false);
    }
  };

  const terminalShipment =
    selectedShipment?.status === "delivered" || selectedShipment?.status === "returned";

  return (
    <Space direction="vertical" size={16} style={{ width: "100%" }}>
      <Alert
        showIcon
        type="info"
        message={tr("物流轨迹仅记录本地履约事实")}
        description={tr("当前默认数据源为本地记录，不会自动轮询承运商，也不会向真实平台发送写请求。")}
      />

      {shipmentsLoading ? (
        <Alert type="info" message={tr("正在加载物流包裹")} />
      ) : shipmentsError ? (
        <Alert
          showIcon
          type="error"
          message={tr("物流包裹加载失败")}
          description={tr(shipmentsError)}
          action={<Button onClick={() => void loadShipments()}>{tr("重试")}</Button>}
        />
      ) : shipments.length === 0 ? (
        <Empty description={tr("暂无物流包裹")} />
      ) : (
        <Row gutter={[16, 16]}>
          <Col xs={24} md={8}>
            <Card size="small" title={trFormat("包裹（{value0}）", { value0: String(shipments.length) })}>
              <List
                dataSource={shipments}
                split
                renderItem={(shipment) => (
                  <List.Item key={shipment.id} style={{ paddingInline: 0 }}>
                    <Button
                      block
                      type={shipment.id === selectedShipmentId ? "primary" : "default"}
                      onClick={() => setSelectedShipmentId(shipment.id)}
                      style={{ height: "auto", textAlign: "left", whiteSpace: "normal" }}
                    >
                      <Space direction="vertical" size={2} style={{ width: "100%" }}>
                        <Typography.Text ellipsis={{ tooltip: shipment.trackingNo }}>
                          {shipment.carrier || tr("未填写承运商")} · {shipment.trackingNo || tr("暂无运单号")}
                        </Typography.Text>
                        <span>{shipmentTag(shipment.status)}</span>
                      </Space>
                    </Button>
                  </List.Item>
                )}
              />
            </Card>
          </Col>
          <Col xs={24} md={16}>
            <Card
              size="small"
              title={selectedShipment ? trFormat("轨迹 · {value0}", { value0: selectedShipment.trackingNo || tr("暂无运单号") }) : tr("轨迹")}
              extra={
                <Space wrap>
                  <Tag>{tr("数据源：")}{provider}</Tag>
                  <Button
                    type="primary"
                    disabled={readOnly || !selectedShipment || terminalShipment}
                    title={
                      readOnly
                        ? tr("只读账号不可录入物流事件")
                        : terminalShipment
                          ? tr("已送达或已退回的包裹不可继续录入事件")
                          : undefined
                    }
                    onClick={openEventModal}
                  >
                    {tr("录入物流事件")}
                  </Button>
                </Space>
              }
            >
              {readOnly ? (
                <Typography.Paragraph type="secondary">
                  {tr("当前账号为只读权限，只能查看物流轨迹。")}
                </Typography.Paragraph>
              ) : null}
              {eventsLoading ? (
                <Alert type="info" message={tr("正在加载物流轨迹")} />
              ) : eventsError ? (
                <Alert
                  showIcon
                  type="error"
                  message={tr("物流轨迹加载失败")}
                  description={tr(eventsError)}
                  action={
                    <Button onClick={() => selectedShipmentId && void loadEvents(selectedShipmentId)}>
                      {tr("重试")}
                    </Button>
                  }
                />
              ) : events.length === 0 ? (
                <Empty description={tr("暂无物流事件")} />
              ) : (
                <Timeline
                  items={events.map((event) => ({
                    key: event.id,
                    color: timelineColor(event.status),
                    children: (
                      <Space direction="vertical" size={4} style={{ width: "100%" }}>
                        {eventTitle(event)}
                        {event.location ? (
                          <Typography.Text type="secondary">{tr("地点：")}{event.location}</Typography.Text>
                        ) : null}
                        {event.description ? (
                          <Typography.Paragraph
                            style={{ marginBottom: 0, overflowWrap: "anywhere", whiteSpace: "pre-wrap" }}
                          >
                            {event.description}
                          </Typography.Paragraph>
                        ) : null}
                        <Typography.Text type="secondary">{tr("来源：")}{event.source || "local"}</Typography.Text>
                      </Space>
                    ),
                  }))}
                />
              )}
            </Card>
          </Col>
        </Row>
      )}

      <Modal
        title={tr("录入物流事件")}
        open={eventModalOpen}
        confirmLoading={eventSubmitting}
        width={560}
        styles={{
          body: { maxHeight: "calc(100vh - 220px)", overflowY: "auto" },
        }}
        onCancel={() => {
          if (!eventSubmitting) setEventModalOpen(false);
        }}
        onOk={() => eventForm.submit()}
        afterClose={() => eventForm.resetFields()}
      >
        <Form<EventFormValues> form={eventForm} layout="vertical" onFinish={submitEvent}>
          <Form.Item
            name="eventKey"
            label={tr("事件键")}
            rules={[{ required: true, message: tr("请填写事件键") }]}
            extra={tr("同一包裹使用相同事件键重试时会幂等返回。")}
          >
            <Input maxLength={128} placeholder={tr("例如：manual-20260823-001")} />
          </Form.Item>
          <Form.Item name="status" label={tr("物流状态")} rules={[{ required: true, message: tr("请选择物流状态") }]}>
            <Select options={eventStatuses} />
          </Form.Item>
          <Form.Item
            name="occurredAt"
            label={tr("发生时间")}
            rules={[{ required: true, message: tr("请选择发生时间") }]}
          >
            <DatePicker showTime style={{ width: "100%" }} format="YYYY-MM-DD HH:mm" />
          </Form.Item>
          <Form.Item name="location" label={tr("地点")}>
            <Input maxLength={255} placeholder={tr("例如：深圳转运中心")} />
          </Form.Item>
          <Form.Item name="description" label={tr("描述")}>
            <Input.TextArea maxLength={2000} rows={4} showCount />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  );
}
