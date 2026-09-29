import { getJSON, getWithParams, postJSON } from "@/services/request";

export type FulfillmentWaveStatus =
  | "draft"
  | "picking"
  | "packing"
  | "completing"
  | "partial"
  | "completed"
  | "cancelled";

export type FulfillmentWaveOrderStatus =
  | "pending"
  | "blocked"
  | "ready_to_pack"
  | "packed"
  | "fulfilled"
  | "failed";

export type FulfillmentWaveOrder = {
  id: string;
  waveId: string;
  orderId: string;
  shopId?: string;
  orderNo: string;
  status: FulfillmentWaveOrderStatus;
  carrier?: string;
  trackingNo?: string;
  trackingUrl?: string;
  packageCode?: string;
  actualWeightGrams?: number;
  failureCode?: string;
  failureReason?: string;
  shipmentId?: string;
  packedAt?: string;
  fulfilledAt?: string;
  freightQuote?: FulfillmentWaveFreightQuote;
};

export type FulfillmentWaveFreightQuote = {
  id: string;
  version: number;
  sourceRevision: number;
  rateTemplateId: string;
  rateTemplateCode: string;
  rateTemplateName: string;
  rateTemplateRevision: number;
  channelId: string;
  channelCode: string;
  channelName: string;
  carrier: string;
  destinationCountryCode: string;
  destinationRegion?: string;
  destinationPostalCode?: string;
  weightGrams: number;
  minWeightGrams: number;
  maxWeightGrams: number;
  amountMinor: number;
  currency: string;
  explanation: string;
  createdAt: string;
};

export type FulfillmentFreightQuoteCandidate = {
  rateTemplateId: string;
  rateTemplateCode: string;
  rateTemplateName: string;
  rateTemplateRevision: number;
  channelId: string;
  channelCode: string;
  channelName: string;
  carrier: string;
  weightGrams: number;
  minWeightGrams: number;
  maxWeightGrams: number;
  amountMinor: number;
  currency: string;
  explanation: string;
};

export type FulfillmentFreightQuoteResult = {
  destinationCountryCode: string;
  destinationRegion?: string;
  destinationPostalCode?: string;
  candidates: FulfillmentFreightQuoteCandidate[];
};

export type FulfillmentWaveLine = {
  id: string;
  waveId: string;
  waveOrderId: string;
  orderId: string;
  orderItemId: string;
  productId?: string;
  productSkuId: string;
  productTitle?: string;
  skuCode?: string;
  skuName?: string;
  barcode?: string;
  locationId?: string;
  locationCode?: string;
  locationName?: string;
  requiredQuantity: number;
  pickedQuantity: number;
  shortageQuantity: number;
  status: "pending" | "picked" | "shortage";
};

export type FulfillmentWave = {
  id: string;
  waveNo: string;
  warehouseId: string;
  warehouseCode?: string;
  warehouseName?: string;
  status: FulfillmentWaveStatus;
  revision: number;
  remark?: string;
  orderCount: number;
  lineCount: number;
  requiredQuantity: number;
  pickedQuantity: number;
  shortageQuantity: number;
  fulfilledCount: number;
  failedCount: number;
  packingVerificationRequired: boolean;
  startedAt?: string;
  completedAt?: string;
  cancelledAt?: string;
  createdAt: string;
  updatedAt: string;
  orders?: FulfillmentWaveOrder[];
  lines?: FulfillmentWaveLine[];
  packVerifications?: FulfillmentWavePackVerification[];
  packScans?: FulfillmentWavePackScan[];
};

export type FulfillmentWavePackVerification = {
  id: string;
  waveId: string;
  waveOrderId: string;
  orderId: string;
  packageCode: string;
  scannedOrderNo: string;
  carrier: string;
  trackingNo: string;
  trackingUrl?: string;
  actualWeightGrams?: number;
  lineCount: number;
  verifiedQuantity: number;
  createdAt: string;
};

export type FulfillmentWavePackScan = {
  id: string;
  waveLineId: string;
  verificationId: string;
  expectedCode: string;
  scannedCode: string;
  expectedQuantity: number;
  verifiedQuantity: number;
  validated: boolean;
  createdAt: string;
};

export type FulfillmentWaveList = {
  list: FulfillmentWave[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
    totalPages: number;
  };
};

export type FulfillmentWaveRevisionPayload = {
  expectedRevision: number;
  idempotencyKey: string;
};

export type FulfillmentWaveDocumentType =
  | "pick_list"
  | "packing_list"
  | "sku_labels"
  | "package_labels";

export type FulfillmentWaveDocumentSnapshotOrder = {
  waveOrderId: string;
  orderId: string;
  orderNo: string;
  status: FulfillmentWaveOrderStatus;
  carrier?: string;
  trackingNo?: string;
  trackingUrl?: string;
  packageCode?: string;
  actualWeightGrams?: number;
};

export type FulfillmentWaveDocumentSnapshotLine = {
  waveLineId: string;
  waveOrderId: string;
  orderId: string;
  orderNo: string;
  productTitle?: string;
  skuCode?: string;
  skuName?: string;
  barcode?: string;
  locationCode?: string;
  locationName?: string;
  requiredQuantity: number;
  pickedQuantity: number;
  shortageQuantity: number;
  status: "pending" | "picked" | "shortage";
};

export type FulfillmentWaveDocumentSnapshot = {
  waveId: string;
  waveNo: string;
  waveStatus: FulfillmentWaveStatus;
  sourceRevision: number;
  warehouseId: string;
  warehouseCode?: string;
  warehouseName?: string;
  generatedAt: string;
  orderCount: number;
  lineCount: number;
  requiredQuantity: number;
  pickedQuantity: number;
  shortageQuantity: number;
  orders: FulfillmentWaveDocumentSnapshotOrder[];
  lines: FulfillmentWaveDocumentSnapshotLine[];
};

export type FulfillmentWaveDocumentPrintEvent = {
  id: string;
  waveId: string;
  documentId: string;
  documentType: FulfillmentWaveDocumentType;
  copies: number;
  reprint: boolean;
  reason?: string;
  actorId?: string;
  createdAt: string;
};

export type FulfillmentWaveDocumentSummary = {
  id: string;
  waveId: string;
  version: number;
  sourceRevision: number;
  snapshotHash: string;
  createdBy?: string;
  createdAt: string;
};

export type FulfillmentWaveDocumentDetail = FulfillmentWaveDocumentSummary & {
  snapshot: FulfillmentWaveDocumentSnapshot;
  printEvents: FulfillmentWaveDocumentPrintEvent[];
};

export type FulfillmentWaveDocumentList = {
  list: FulfillmentWaveDocumentSummary[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
    totalPages: number;
  };
};

export type CompleteFulfillmentWaveResult = {
  wave: FulfillmentWave;
  processed: number;
  succeeded: number;
  failed: number;
  items: Array<{
    orderId: string;
    orderNo: string;
    status: "fulfilled" | "failed";
    shipmentId?: string;
    failureReason?: string;
  }>;
};

const enc = encodeURIComponent;

export function createFulfillmentWaveIdempotencyKey(action: string) {
  const random =
    globalThis.crypto?.randomUUID?.() ??
    `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
  return `admin-fulfillment-wave-${action}-${random}`.slice(0, 128);
}

export async function queryFulfillmentWaves(params: {
  page?: number;
  pageSize?: number;
  keyword?: string;
  status?: FulfillmentWaveStatus;
  warehouseId?: string;
}) {
  return getWithParams<FulfillmentWaveList>(
    "/api/v1/fulfillment-waves",
    params,
  );
}

export async function getFulfillmentWave(id: string) {
  return getJSON<FulfillmentWave>(`/api/v1/fulfillment-waves/${enc(id)}`);
}

export async function createFulfillmentWave(payload: {
  idempotencyKey: string;
  warehouseId: string;
  orderIds: string[];
  remark?: string;
}) {
  return postJSON<FulfillmentWave>("/api/v1/fulfillment-waves", payload);
}

export async function startFulfillmentWave(
  id: string,
  payload: FulfillmentWaveRevisionPayload,
) {
  return postJSON<FulfillmentWave>(
    `/api/v1/fulfillment-waves/${enc(id)}/start`,
    payload,
  );
}

export async function recordFulfillmentWavePicks(
  id: string,
  payload: FulfillmentWaveRevisionPayload & {
    lines: Array<{
      lineId: string;
      pickedQuantity: number;
      shortageQuantity: number;
      scannedBarcode?: string;
      scannedLocationCode?: string;
    }>;
  },
) {
  return postJSON<FulfillmentWave>(
    `/api/v1/fulfillment-waves/${enc(id)}/picks`,
    payload,
  );
}

export async function packFulfillmentWaveOrder(
  waveId: string,
  orderId: string,
  payload: FulfillmentWaveRevisionPayload & {
    carrier: string;
    trackingNo: string;
    trackingUrl?: string;
  },
) {
  return postJSON<FulfillmentWave>(
    `/api/v1/fulfillment-waves/${enc(waveId)}/orders/${enc(orderId)}/pack`,
    payload,
  );
}

export async function verifyFulfillmentWavePack(
  waveId: string,
  orderId: string,
  payload: FulfillmentWaveRevisionPayload & {
    scannedOrderNo: string;
    carrier: string;
    trackingNo: string;
    trackingUrl?: string;
    packageCode: string;
    actualWeightGrams?: number;
    lines: Array<{
      lineId: string;
      scannedCode: string;
      verifiedQuantity: number;
    }>;
  },
) {
  return postJSON<FulfillmentWave>(
    `/api/v1/fulfillment-waves/${enc(waveId)}/orders/${enc(orderId)}/verify-pack`,
    payload,
  );
}

export async function quoteFulfillmentWaveFreight(
  waveId: string,
  orderId: string,
  weightGrams: number,
) {
  return getWithParams<FulfillmentFreightQuoteResult>(
    `/api/v1/fulfillment-waves/${enc(waveId)}/orders/${enc(orderId)}/freight-quotes`,
    { weightGrams },
  );
}

export async function confirmFulfillmentWaveFreightQuote(
  waveId: string,
  orderId: string,
  payload: FulfillmentWaveRevisionPayload & {
    rateTemplateId: string;
    rateTemplateRevision: number;
    weightGrams: number;
  },
) {
  return postJSON<FulfillmentWave>(
    `/api/v1/fulfillment-waves/${enc(waveId)}/orders/${enc(orderId)}/freight-quotes/confirm`,
    payload,
  );
}

export async function completeFulfillmentWave(
  id: string,
  payload: FulfillmentWaveRevisionPayload,
) {
  return postJSON<CompleteFulfillmentWaveResult>(
    `/api/v1/fulfillment-waves/${enc(id)}/complete`,
    payload,
  );
}

export async function cancelFulfillmentWave(
  id: string,
  payload: FulfillmentWaveRevisionPayload,
) {
  return postJSON<FulfillmentWave>(
    `/api/v1/fulfillment-waves/${enc(id)}/cancel`,
    payload,
  );
}

export async function queryFulfillmentWaveDocuments(
  waveId: string,
  params?: { page?: number; pageSize?: number },
) {
  return getWithParams<FulfillmentWaveDocumentList>(
    `/api/v1/fulfillment-waves/${enc(waveId)}/documents`,
    params,
  );
}

export async function getFulfillmentWaveDocument(
  waveId: string,
  documentId: string,
) {
  return getJSON<FulfillmentWaveDocumentDetail>(
    `/api/v1/fulfillment-waves/${enc(waveId)}/documents/${enc(documentId)}`,
  );
}

export async function generateFulfillmentWaveDocument(
  waveId: string,
  payload: FulfillmentWaveRevisionPayload,
) {
  return postJSON<FulfillmentWaveDocumentDetail>(
    `/api/v1/fulfillment-waves/${enc(waveId)}/documents`,
    payload,
  );
}

export async function recordFulfillmentWaveDocumentPrint(
  waveId: string,
  documentId: string,
  payload: {
    documentType: FulfillmentWaveDocumentType;
    copies: number;
    reason?: string;
    idempotencyKey: string;
  },
) {
  return postJSON<FulfillmentWaveDocumentPrintEvent>(
    `/api/v1/fulfillment-waves/${enc(waveId)}/documents/${enc(documentId)}/print-events`,
    payload,
  );
}

function csvCell(value: string | number) {
  const raw = String(value ?? "");
  // Spreadsheet applications may execute formula-looking cells even when a
  // CSV field is quoted. Snapshot text can originate from marketplace data,
  // so neutralize formula prefixes before applying CSV escaping.
  const text = /^[\t\r ]*[=+\-@]/.test(raw) ? `'${raw}` : raw;
  return `"${text.replace(/"/g, '""')}"`;
}

export function buildFulfillmentWavePickCSV(wave: FulfillmentWave) {
  const orderNoByID = new Map(
    (wave.orders ?? []).map((order) => [order.orderId, order.orderNo]),
  );
  const header = [
    "波次号",
    "仓库",
    "订单号",
    "商品",
    "规格编码",
    "规格名称",
    "需求数量",
    "已拣数量",
    "缺货数量",
  ];
  const rows = (wave.lines ?? []).map((line) => [
    wave.waveNo,
    [wave.warehouseCode, wave.warehouseName].filter(Boolean).join(" · ") ||
      wave.warehouseId,
    orderNoByID.get(line.orderId) ?? line.orderId,
    line.productTitle ?? "",
    line.skuCode ?? "",
    line.skuName ?? "",
    line.requiredQuantity,
    line.pickedQuantity,
    line.shortageQuantity,
  ]);
  return `\uFEFF${[header, ...rows].map((row) => row.map(csvCell).join(",")).join("\r\n")}`;
}

export function downloadFulfillmentWavePickCSV(wave: FulfillmentWave) {
  const blob = new Blob([buildFulfillmentWavePickCSV(wave)], {
    type: "text/csv;charset=utf-8",
  });
  const objectURL = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = objectURL;
  anchor.download = `${wave.waveNo}-拣货单.csv`;
  anchor.rel = "noopener";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(objectURL);
}
