import { ok } from "./envelope";

export const E2E_FULFILLMENT_WAVE_ID = "e2e-fulfillment-wave-1";
export const E2E_FULFILLMENT_WAVE_ORDER_ID = "e2e-fulfillment-wave-order-1";
export const E2E_FULFILLMENT_WAVE_LINE_ID = "e2e-fulfillment-wave-line-1";

export const e2eFulfillmentWave = {
  id: E2E_FULFILLMENT_WAVE_ID,
  waveNo: "FW20260824-E2E0000001",
  warehouseId: "e2e-warehouse-allocation-main",
  warehouseCode: "MAIN",
  warehouseName: "E2E 华东主仓",
  status: "picking",
  revision: 2,
  remark: "E2E 早班",
  orderCount: 1,
  lineCount: 1,
  requiredQuantity: 3,
  pickedQuantity: 0,
  shortageQuantity: 0,
  fulfilledCount: 0,
  failedCount: 0,
  packingVerificationRequired: true,
  createdAt: "2026-08-24T02:00:00Z",
  updatedAt: "2026-08-24T02:01:00Z",
  orders: [
    {
      id: E2E_FULFILLMENT_WAVE_ORDER_ID,
      waveId: E2E_FULFILLMENT_WAVE_ID,
      orderId: "e2e-order-warehouse-allocation",
      orderNo: "SO-E2E-ALLOC-0001",
      status: "pending",
    },
  ],
  lines: [
    {
      id: E2E_FULFILLMENT_WAVE_LINE_ID,
      waveId: E2E_FULFILLMENT_WAVE_ID,
      waveOrderId: E2E_FULFILLMENT_WAVE_ORDER_ID,
      orderId: "e2e-order-warehouse-allocation",
      orderItemId: "e2e-order-item-blue",
      productSkuId: "e2e-allocation-sku-blue",
      productTitle: "E2E 蓝色商品",
      skuCode: "BLUE-01",
      skuName: "蓝色",
      requiredQuantity: 3,
      pickedQuantity: 0,
      shortageQuantity: 0,
      status: "pending",
    },
  ],
};

export function fulfillmentWaveResponse(path: string) {
  if (path === "/api/v1/fulfillment-waves") {
    return ok({
      list: [e2eFulfillmentWave],
      pagination: { page: 1, pageSize: 20, total: 1, totalPages: 1 },
    });
  }
  if (path === `/api/v1/fulfillment-waves/${E2E_FULFILLMENT_WAVE_ID}`) {
    return ok(e2eFulfillmentWave);
  }
  return null;
}
