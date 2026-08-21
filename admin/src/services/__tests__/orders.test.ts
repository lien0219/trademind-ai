import { describe, expect, it } from "vitest";
import { ApiRequestError } from "../request";
import { partialOrderCreateFromError } from "../orders";

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
});
