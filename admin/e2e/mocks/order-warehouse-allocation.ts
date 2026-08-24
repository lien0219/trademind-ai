import { ok } from "./envelope";

export const E2E_ALLOCATION_ORDER_ID = "e2e-order-warehouse-allocation";
export const E2E_ALLOCATION_WAREHOUSE_ID = "e2e-warehouse-allocation-main";
export const E2E_ALLOCATION_REVISION = "a".repeat(64);

export const e2eWarehouseAllocation = {
  orderId: E2E_ALLOCATION_ORDER_ID,
  status: "allocatable",
  recommendedWarehouseId: E2E_ALLOCATION_WAREHOUSE_ID,
  candidateCount: 2,
  eligibleCandidateCount: 1,
  blocks: [],
  candidates: [
    {
      warehouseId: E2E_ALLOCATION_WAREHOUSE_ID,
      warehouseCode: "MAIN",
      warehouseName: "E2E 华东主仓",
      isDefault: true,
      eligible: true,
      shortageCount: 0,
      revision: E2E_ALLOCATION_REVISION,
      lines: [
        {
          productSkuId: "e2e-allocation-sku-blue",
          skuCode: "BLUE-01",
          skuName: "蓝色",
          required: 3,
          available: 10,
          shortage: 0,
        },
      ],
    },
    {
      warehouseId: "e2e-warehouse-allocation-secondary",
      warehouseCode: "SECOND",
      warehouseName: "E2E 华南备仓",
      isDefault: false,
      eligible: false,
      shortageCount: 1,
      revision: "b".repeat(64),
      lines: [
        {
          productSkuId: "e2e-allocation-sku-blue",
          skuCode: "BLUE-01",
          skuName: "蓝色",
          required: 3,
          available: 1,
          shortage: 2,
        },
      ],
    },
  ],
};

export function warehouseAllocationResponse(path: string) {
  if (path === "/api/v1/orders/warehouse-allocations") {
    return ok({
      list: [
        {
          id: E2E_ALLOCATION_ORDER_ID,
          platform: "douyin_shop",
          shopName: "E2E 抖音店铺",
          orderNo: "SO-E2E-ALLOC-0001",
          customerName: "买家**",
          status: "paid",
          paymentStatus: "paid",
          fulfillmentStatus: "unfulfilled",
          currency: "CNY",
          totalAmount: 299,
          createdAt: "2026-08-24T02:00:00Z",
          allocationStatus: "allocatable",
          recommendedWarehouseId: E2E_ALLOCATION_WAREHOUSE_ID,
          recommendedWarehouseCode: "MAIN",
          recommendedWarehouseName: "E2E 华东主仓",
          candidateCount: 2,
          eligibleCandidateCount: 1,
          blocks: [],
        },
      ],
      pagination: { page: 1, pageSize: 20, total: 1, totalPages: 1 },
    });
  }
  if (
    path === `/api/v1/orders/${E2E_ALLOCATION_ORDER_ID}/warehouse-allocation`
  ) {
    return ok(e2eWarehouseAllocation);
  }
  return null;
}
