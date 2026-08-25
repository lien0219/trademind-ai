import { ok } from "./envelope";

export const E2E_ALLOCATION_ORDER_ID = "e2e-order-warehouse-allocation";
export const E2E_ALLOCATION_WAREHOUSE_ID = "e2e-warehouse-allocation-main";
export const E2E_ALLOCATION_REVISION = "a".repeat(64);

export const e2eWarehouseAllocation = {
  orderId: E2E_ALLOCATION_ORDER_ID,
  status: "allocatable",
  recommendationPolicy: "single_warehouse_default_first_v1",
  recommendedWarehouseId: E2E_ALLOCATION_WAREHOUSE_ID,
  recommendedWarehouseReasons: [
    { code: "FULL_ORDER_COVERAGE", message: "可用库存覆盖整单需求" },
    { code: "DEFAULT_WAREHOUSE", message: "默认仓优先" },
    {
      code: "RECOMMENDED_BY_POLICY",
      message: "按整单可满足、默认仓优先、仓库编码稳定排序",
    },
  ],
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
      recommendationRank: 1,
      recommendationReasons: [
        { code: "FULL_ORDER_COVERAGE", message: "可用库存覆盖整单需求" },
        { code: "DEFAULT_WAREHOUSE", message: "默认仓优先" },
      ],
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
      recommendationRank: 2,
      recommendationReasons: [
        { code: "STOCK_SHORTAGE", message: "有 1 个 SKU 存在库存缺口" },
        { code: "FALLBACK_WAREHOUSE", message: "非默认仓作为备选" },
      ],
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

export const e2eBlockedWarehouseAllocation = {
  orderId: E2E_ALLOCATION_ORDER_ID,
  status: "blocked",
  recommendationPolicy: "single_warehouse_default_first_v1",
  candidateCount: 0,
  eligibleCandidateCount: 0,
  blocks: [
    {
      code: "INVENTORY_PROJECTION_MISMATCH",
      message: "仓库库存账与兼容库存投影不一致，请先完成库存对账",
    },
  ],
  candidates: [],
};

export function blockedWarehouseAllocationListResponse() {
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
        allocationStatus: "blocked",
        candidateCount: 0,
        eligibleCandidateCount: 0,
        blocks: e2eBlockedWarehouseAllocation.blocks,
      },
    ],
    pagination: { page: 1, pageSize: 20, total: 1, totalPages: 1 },
  });
}

export function allocatedWarehouseAllocationListResponse() {
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
        allocationStatus: "allocated",
        warehouseId: E2E_ALLOCATION_WAREHOUSE_ID,
        warehouseCode: "MAIN",
        warehouseName: "E2E 华东主仓",
        candidateCount: 1,
        eligibleCandidateCount: 1,
        blocks: [],
      },
    ],
    pagination: { page: 1, pageSize: 20, total: 1, totalPages: 1 },
  });
}

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
          recommendedWarehouseReasons: [
            { code: "FULL_ORDER_COVERAGE", message: "可用库存覆盖整单需求" },
          ],
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
