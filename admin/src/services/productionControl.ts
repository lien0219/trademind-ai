import { tr } from '@/i18n';
import { getJSON } from '@/services/request';

export type ProductionRuntimeControl = {
  providerKillActive: boolean;
  tenantKillActive: boolean;
  shopKillActive: boolean;
  readKillActive: boolean;
  writeKillActive: boolean;
  revision: number;
};

export type ProductionScopeAllowlist = {
  tenantId: number;
  shopId: string;
  enabled: boolean;
  revision: number;
};

export type ProductionGrayPolicy = {
  tenantId: number;
  shopId: string;
  maxSku: number;
  status: string;
  ownerApproved: boolean;
  technicalLeadApproved: boolean;
  revision: number;
};

export type ProductionRuntimeStatus = {
  currentAllowedLevel: string;
  environment: string;
  realProviderEnabled: boolean;
  realPlatformNetworkEnabled: boolean;
  realCredentialsEnabled: boolean;
  realProductDraftWriteEnabled: boolean;
  backgroundWorkerEnabled: boolean;
  productPublishQueueEnabled: boolean;
  providerWriteReady: boolean;
  automaticRetryEnabled: boolean;
  control: ProductionRuntimeControl;
  allowlist?: ProductionScopeAllowlist;
  gray?: ProductionGrayPolicy;
  initialLimits: Record<string, number>;
  productionReady: boolean;
  productionAcceptancePassed: boolean;
};

export async function getProductionRuntimeStatus() {
  return getJSON<ProductionRuntimeStatus>('/api/v1/p10/status');
}

export function productionDraftBlockReason(status?: ProductionRuntimeStatus | null) {
  if (!status) return tr('无法确认生产运行控制状态，已禁止真实平台写入。');
  if (status.productionReady) return undefined;
  if (status.control?.writeKillActive) return tr('平台草稿写入开关处于阻断状态。');
  if (!status.allowlist?.enabled) return tr('当前账号可操作范围内没有启用的白名单抖店。');
  if (!status.gray || status.gray.status !== 'active' || !status.gray.ownerApproved || !status.gray.technicalLeadApproved) {
    return tr('灰度策略尚未激活或双人审批未完成。');
  }
  if (!status.providerWriteReady) return tr('抖店平台运行开关、草稿功能或平台灰度店铺尚未放行。');
  if (status.currentAllowedLevel.toUpperCase() !== 'L3') return tr('当前生产能力级别未达到 L3。');
  if (!status.productPublishQueueEnabled || !status.backgroundWorkerEnabled) return tr('商品刊登队列或后台 Worker 尚未启用。');
  return tr('真实平台、凭据、草稿写入或后台 Worker 尚未全部启用。');
}
