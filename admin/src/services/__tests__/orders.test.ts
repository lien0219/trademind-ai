import { describe, expect, it, vi } from "vitest";
import { ApiRequestError } from "../request";
import {
  appendOrderShipmentEvent,
  batchFulfillOrders,
  confirmWarehouseAllocation,
  createOrderBatchFulfillmentIdempotencyKey,
  createOrderAllocationIdempotencyKey,
  fulfillOrder,
  getOrderFulfillmentReconciliation,
  getWarehouseAllocation,
  getOrderShipmentEvents,
  partialOrderCreateFromError,
  queryFulfillmentReconciliation,
  queryWarehouseAllocations,
} from "../orders";
import { request } from "@umijs/max";

const requestMock = vi.mocked(request);

describe("order service helpers", () => {
  it("recognizes a persisted order in an inventory conflict response", () => {
    const order = { id: "order-1" };
    const error = new ApiRequestError({
      code: 40001,
      message: "订单已创建，但库存处理失败",
      data: {
        orderId: "order-1",
        order,
        inventoryDeduction: { linesFailed: 1 },
      },
    });

    expect(partialOrderCreateFromError(error)).toEqual({
      orderId: "order-1",
      order,
      inventoryDeduction: { linesFailed: 1 },
    });
  });

  it("rejects unrelated API errors", () => {
    const error = new ApiRequestError({
      code: 40001,
      message: "invalid order",
      data: null,
    });

    expect(partialOrderCreateFromError(error)).toBeNull();
  });

  it("sends the single-warehouse fulfillment contract exactly once", async () => {
    requestMock.mockResolvedValue({
      code: 0,
      message: "ok",
      data: {
        order: { id: "order-1" },
        shipment: { id: "shipment-1" },
        inventoryDeduction: { action: "deduct" },
      },
    });

    const payload = {
      idempotencyKey: "order-fulfillment-key",
      warehouseId: "warehouse-1",
      carrier: "carrier",
      trackingNo: "tracking-1",
      trackingUrl: "https://carrier.test/track/tracking-1",
    };
    await fulfillOrder("order-1", payload);

    expect(requestMock).toHaveBeenCalledWith("/api/v1/orders/order-1/fulfill", {
      method: "POST",
      data: payload,
    });
  });

  it("sends the batch fulfillment contract exactly once", async () => {
    requestMock.mockResolvedValue({
      code: 0,
      message: "ok",
      data: {
        batchIdempotencyKey: "batch-key",
        summary: { requested: 1, succeeded: 1, blocked: 0, inProgress: 0, failed: 0 },
        items: [],
        pickList: [],
      },
    });

    const payload = {
      batchIdempotencyKey: "batch-key",
      items: [
        {
          orderId: "order-1",
          carrier: "carrier",
          trackingNo: "tracking-1",
          trackingUrl: "https://carrier.test/track/tracking-1",
        },
      ],
    };
    await batchFulfillOrders(payload);

    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/orders/fulfillment-batch",
      { method: "POST", data: payload },
    );
  });

  it("keeps shipment event read and append contracts stable", async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: { shipment: { id: "shipment-1" }, events: [], provider: "local" },
    });
    await getOrderShipmentEvents("order-1", "shipment-1");
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/orders/order-1/shipments/shipment-1/events",
      { method: "GET" },
    );

    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: {
        event: { id: "event-1" },
        shipment: { id: "shipment-1" },
        replay: false,
      },
    });
    const payload = {
      eventKey: "manual-1",
      status: "in_transit",
      location: "Shenzhen",
      description: "已交运输商",
    };
    await appendOrderShipmentEvent("order-1", "shipment-1", payload);
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/orders/order-1/shipments/shipment-1/events",
      { method: "POST", data: payload },
    );
  });

  it("keeps warehouse allocation list, detail, and confirm contracts stable", async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: {
        list: [],
        pagination: { page: 1, pageSize: 20, total: 0, totalPages: 0 },
      },
    });
    await queryWarehouseAllocations({
      page: 1,
      pageSize: 20,
      assignment: "unallocated",
    });
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/orders/warehouse-allocations",
      {
        method: "GET",
        params: { page: 1, pageSize: 20, assignment: "unallocated" },
      },
    );

    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: { orderId: "order-1", status: "allocatable", candidates: [] },
    });
    await getWarehouseAllocation("order-1");
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/orders/order-1/warehouse-allocation",
      { method: "GET" },
    );

    const payload = {
      warehouseId: "warehouse-1",
      expectedRevision: "a".repeat(64),
      idempotencyKey: "allocation-key-1",
    };
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: { allocation: { orderId: "order-1", status: "allocated" } },
    });
    await confirmWarehouseAllocation("order-1", payload);
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/orders/order-1/warehouse-allocation",
      { method: "POST", data: payload },
    );
  });

  it("creates bounded unique warehouse allocation idempotency keys", () => {
    const first = createOrderAllocationIdempotencyKey();
    const second = createOrderAllocationIdempotencyKey();
    expect(first).not.toBe(second);
    expect(first).toMatch(/^admin-order-warehouse-allocation-/);
    expect(first.length).toBeLessThanOrEqual(128);
  });

  it("creates bounded unique batch fulfillment idempotency keys", () => {
    const first = createOrderBatchFulfillmentIdempotencyKey();
    const second = createOrderBatchFulfillmentIdempotencyKey();
    expect(first).not.toBe(second);
    expect(first).toMatch(/^admin-order-fulfillment-batch-/);
    expect(first.length).toBeLessThanOrEqual(128);
  });

  it("keeps fulfillment reconciliation read contracts stable and encodes detail ids", async () => {
    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: {
        list: [],
        pagination: { page: 2, pageSize: 50, total: 0, totalPages: 0 },
      },
    });
    await queryFulfillmentReconciliation({
      page: 2,
      pageSize: 50,
      orderNo: "SO-1001",
      warehouseId: "warehouse-1",
      status: "shipped",
      fulfillmentStatus: "fulfilled",
      reconciliationStatus: "mismatch",
    });
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/orders/fulfillment-reconciliation",
      {
        method: "GET",
        params: {
          page: 2,
          pageSize: 50,
          orderNo: "SO-1001",
          warehouseId: "warehouse-1",
          status: "shipped",
          fulfillmentStatus: "fulfilled",
          reconciliationStatus: "mismatch",
        },
      },
    );

    requestMock.mockResolvedValueOnce({
      code: 0,
      message: "ok",
      data: { orderId: "order/one", reconciliationStatus: "matched" },
    });
    await getOrderFulfillmentReconciliation("order/one");
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/orders/order%2Fone/fulfillment-reconciliation",
      { method: "GET" },
    );
  });
});
