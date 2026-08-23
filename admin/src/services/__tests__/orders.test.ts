import { describe, expect, it, vi } from "vitest";
import { ApiRequestError } from "../request";
import { fulfillOrder, partialOrderCreateFromError } from "../orders";
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

    expect(requestMock).toHaveBeenCalledWith(
      "/api/v1/orders/order-1/fulfill",
      { method: "POST", data: payload },
    );
  });
});
