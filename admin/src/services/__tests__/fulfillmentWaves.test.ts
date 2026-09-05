import { request } from "@umijs/max";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  buildFulfillmentWavePickCSV,
  cancelFulfillmentWave,
  confirmFulfillmentWaveFreightQuote,
  completeFulfillmentWave,
  createFulfillmentWave,
  createFulfillmentWaveIdempotencyKey,
  generateFulfillmentWaveDocument,
  getFulfillmentWaveDocument,
  packFulfillmentWaveOrder,
  quoteFulfillmentWaveFreight,
  queryFulfillmentWaveDocuments,
  recordFulfillmentWaveDocumentPrint,
  recordFulfillmentWavePicks,
  verifyFulfillmentWavePack,
  type FulfillmentWave,
} from "../fulfillmentWaves";

const requestMock = vi.mocked(request);

describe("fulfillment wave service", () => {
  beforeEach(() => {
    requestMock.mockReset();
    requestMock.mockResolvedValue({ code: 0, message: "ok", data: {} });
  });

  it("keeps create, pick, pack, complete, and cancel contracts stable", async () => {
    const createPayload = {
      idempotencyKey: "wave-create-key",
      warehouseId: "warehouse-1",
      orderIds: ["order-1"],
      remark: "morning shift",
    };
    await createFulfillmentWave(createPayload);
    expect(requestMock).toHaveBeenLastCalledWith("/api/v1/fulfillment-waves", {
      method: "POST",
      data: createPayload,
    });

    const pickPayload = {
      expectedRevision: 2,
      idempotencyKey: "wave-pick-key",
      lines: [{ lineId: "line-1", pickedQuantity: 2, shortageQuantity: 0 }],
    };
    await recordFulfillmentWavePicks("wave/1", pickPayload);
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/fulfillment-waves/wave%2F1/picks",
      {
        method: "POST",
        data: pickPayload,
      },
    );

    const packPayload = {
      expectedRevision: 3,
      idempotencyKey: "wave-pack-key",
      carrier: "carrier",
      trackingNo: "tracking-1",
    };
    await packFulfillmentWaveOrder("wave/1", "order/1", packPayload);
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/fulfillment-waves/wave%2F1/orders/order%2F1/pack",
      { method: "POST", data: packPayload },
    );

    const verifyPayload = {
      expectedRevision: 3,
      idempotencyKey: "wave-verify-pack-key",
      scannedOrderNo: "ORDER-1",
      carrier: "carrier",
      trackingNo: "tracking-1",
      packageCode: "tracking-1",
      actualWeightGrams: 850,
      lines: [{ lineId: "line-1", scannedCode: "SKU-1", verifiedQuantity: 2 }],
    };
    await verifyFulfillmentWavePack("wave/1", "order/1", verifyPayload);
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/fulfillment-waves/wave%2F1/orders/order%2F1/verify-pack",
      { method: "POST", data: verifyPayload },
    );

    await quoteFulfillmentWaveFreight("wave/1", "order/1", 850);
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/fulfillment-waves/wave%2F1/orders/order%2F1/freight-quotes",
      { method: "GET", params: { weightGrams: 850 } },
    );

    const quotePayload = {
      expectedRevision: 4,
      idempotencyKey: "wave-freight-quote-key",
      rateTemplateId: "rate-1",
      rateTemplateRevision: 2,
      weightGrams: 850,
    };
    await confirmFulfillmentWaveFreightQuote("wave/1", "order/1", quotePayload);
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/fulfillment-waves/wave%2F1/orders/order%2F1/freight-quotes/confirm",
      { method: "POST", data: quotePayload },
    );

    const revisionPayload = {
      expectedRevision: 4,
      idempotencyKey: "wave-complete-key",
    };
    await completeFulfillmentWave("wave/1", revisionPayload);
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/fulfillment-waves/wave%2F1/complete",
      {
        method: "POST",
        data: revisionPayload,
      },
    );

    await cancelFulfillmentWave("wave/1", revisionPayload);
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/fulfillment-waves/wave%2F1/cancel",
      {
        method: "POST",
        data: revisionPayload,
      },
    );
  });

  it("creates bounded action-scoped keys", () => {
    const first = createFulfillmentWaveIdempotencyKey("complete");
    const second = createFulfillmentWaveIdempotencyKey("complete");
    expect(first).toMatch(/^admin-fulfillment-wave-complete-/);
    expect(first.length).toBeLessThanOrEqual(128);
    expect(second).not.toBe(first);
  });

  it("keeps document snapshot and print audit contracts stable", async () => {
    await queryFulfillmentWaveDocuments("wave/1", { page: 1, pageSize: 100 });
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/fulfillment-waves/wave%2F1/documents",
      { method: "GET", params: { page: 1, pageSize: 100 } },
    );

    await getFulfillmentWaveDocument("wave/1", "document/1");
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/fulfillment-waves/wave%2F1/documents/document%2F1",
      { method: "GET" },
    );

    const generatePayload = {
      expectedRevision: 4,
      idempotencyKey: "wave-document-generate-key",
    };
    await generateFulfillmentWaveDocument("wave/1", generatePayload);
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/fulfillment-waves/wave%2F1/documents",
      { method: "POST", data: generatePayload },
    );

    const printPayload = {
      documentType: "package_labels" as const,
      copies: 2,
      reason: "damaged paper",
      idempotencyKey: "wave-document-print-key",
    };
    await recordFulfillmentWaveDocumentPrint(
      "wave/1",
      "document/1",
      printPayload,
    );
    expect(requestMock).toHaveBeenLastCalledWith(
      "/api/v1/fulfillment-waves/wave%2F1/documents/document%2F1/print-events",
      { method: "POST", data: printPayload },
    );
  });

  it("builds an Excel-friendly pick CSV from persisted snapshots", () => {
    const wave = {
      id: "wave-1",
      waveNo: "FW-1",
      warehouseId: "warehouse-1",
      warehouseCode: "WH-A",
      warehouseName: "深圳仓",
      status: "picking",
      revision: 2,
      orderCount: 1,
      lineCount: 1,
      requiredQuantity: 2,
      pickedQuantity: 1,
      shortageQuantity: 1,
      fulfilledCount: 0,
      failedCount: 0,
      packingVerificationRequired: true,
      createdAt: "2026-08-24T00:00:00Z",
      updatedAt: "2026-08-24T00:00:00Z",
      orders: [
        {
          id: "wo-1",
          waveId: "wave-1",
          orderId: "order-1",
          orderNo: "ORDER-1",
          status: "blocked",
        },
      ],
      lines: [
        {
          id: "line-1",
          waveId: "wave-1",
          waveOrderId: "wo-1",
          orderId: "order-1",
          orderItemId: "item-1",
          productSkuId: "sku-1",
          productTitle: '商品,"A"',
          skuCode: "RED-L",
          skuName: '=HYPERLINK("https://invalid.test")',
          requiredQuantity: 2,
          pickedQuantity: 1,
          shortageQuantity: 1,
          status: "shortage",
        },
      ],
    } satisfies FulfillmentWave;

    const csv = buildFulfillmentWavePickCSV(wave);
    expect(csv.startsWith("\uFEFF")).toBe(true);
    expect(csv).toContain('"FW-1","WH-A · 深圳仓","ORDER-1"');
    expect(csv).toContain('"商品,""A"""');
    expect(csv).toContain('"\'=HYPERLINK(""https://invalid.test"")"');
    expect(csv).toContain('"2","1","1"');
  });
});
