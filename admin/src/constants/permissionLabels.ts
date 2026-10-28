import { tr } from '@/i18n';
import { PERMISSIONS, type PermissionKey } from '@/utils/permission';

export const ROLE_OPTIONS = [
  { get label() { return tr("管理员"); }, value: 'admin' },
  { get label() { return tr("运营"); }, value: 'operator' },
  { get label() { return tr("只读"); }, value: 'readonly' },
  { get label() { return tr("审核"); }, value: 'reviewer' },
];

export function roleLabel(code: string, name?: string): string {
  return ROLE_OPTIONS.find((role) => role.value === (code || '').toLowerCase())?.label || name || '自定义角色';
}

const PERMISSION_LABELS: Record<PermissionKey, string> = {
  [PERMISSIONS.PRODUCT_VIEW]: '查看商品',
  [PERMISSIONS.PRODUCT_WRITE]: '编辑商品',
  [PERMISSIONS.AI_TEXT_APPLY]: '应用商品文案',
  [PERMISSIONS.AI_IMAGE_APPLY]: '应用商品图片',
  [PERMISSIONS.PUBLISH_CREATE_DRAFT]: '创建刊登草稿',
  [PERMISSIONS.ORDER_VIEW]: '查看订单',
  [PERMISSIONS.ORDER_OPERATE]: '处理订单',
  [PERMISSIONS.SKU_BIND]: '绑定商品规格',
  [PERMISSIONS.INVENTORY_VIEW]: '查看库存',
  [PERMISSIONS.INVENTORY_OPERATE]: '操作库存',
  [PERMISSIONS.INVENTORY_APPROVE]: '审核库存操作',
  [PERMISSIONS.CUSTOMER_VIEW]: '查看客服信息',
  [PERMISSIONS.CUSTOMER_OPERATE]: '处理客服业务',
  [PERMISSIONS.TASK_RETRY]: '重试任务',
  [PERMISSIONS.OPERATION_TASK_AUDIT_READ]: '查看运营任务审计',
  [PERMISSIONS.OPERATION_TASK_EDIT]: '编辑运营任务',
  [PERMISSIONS.OPERATION_TASK_EXECUTE]: '执行运营任务',
  [PERMISSIONS.OPERATION_TASK_REVIEW]: '审核运营任务',
  [PERMISSIONS.OPERATION_TASK_RETRY]: '重试运营任务',
  [PERMISSIONS.SETTINGS_MANAGE]: '管理系统设置',
  [PERMISSIONS.USER_MANAGE]: '管理用户与角色',
  [PERMISSIONS.OPERATIONLOG_VIEW]: '查看操作日志',
  [PERMISSIONS.STORE_VIEW]: '查看店铺',
  [PERMISSIONS.STORE_OPERATE]: '运营店铺',
  [PERMISSIONS.WAREHOUSE_VIEW]: '查看仓库',
  [PERMISSIONS.WAREHOUSE_MANAGE]: '管理仓库',
  [PERMISSIONS.LOGISTICS_VIEW]: '查看物流',
  [PERMISSIONS.LOGISTICS_MANAGE]: '管理物流',
  [PERMISSIONS.SUPPLIER_VIEW]: '查看供应商',
  [PERMISSIONS.SUPPLIER_MANAGE]: '管理供应商',
  [PERMISSIONS.PROCUREMENT_VIEW]: '查看采购',
  [PERMISSIONS.PROCUREMENT_MANAGE]: '管理采购',
  [PERMISSIONS.PROCUREMENT_APPROVE]: '审核采购',
  [PERMISSIONS.PROCUREMENT_RECEIVE]: '采购收货',
  [PERMISSIONS.PROCUREMENT_RETURN]: '处理采购退货',
  [PERMISSIONS.SALES_RETURN_VIEW]: '查看销售售后',
  [PERMISSIONS.SALES_RETURN_MANAGE]: '管理销售售后',
  [PERMISSIONS.SALES_RETURN_APPROVE]: '审核销售售后',
  [PERMISSIONS.SALES_RETURN_RECEIVE]: '售后退货入库',
  [PERMISSIONS.SALES_RETURN_REFUND]: '处理售后退款',
  [PERMISSIONS.AUDIT_READ]: '查看审计记录',
  [PERMISSIONS.AUDIT_EXPORT]: '导出审计记录',
  [PERMISSIONS.CONFIG_READ]: '查看系统配置',
  [PERMISSIONS.CONFIG_MANAGE]: '管理系统配置',
  [PERMISSIONS.EXPORT_CREATE]: '创建数据导出',
  [PERMISSIONS.EXPORT_READ]: '查看数据导出',
  [PERMISSIONS.ORDER_PROFIT_VIEW]: '查看订单利润',
  [PERMISSIONS.ORDER_PROFIT_EXPORT]: '导出订单利润',
  [PERMISSIONS.SETTLEMENT_VIEW]: '查看结算',
  [PERMISSIONS.SETTLEMENT_IMPORT]: '导入结算',
  [PERMISSIONS.SETTLEMENT_EXPORT]: '导出结算',
  [PERMISSIONS.WAREHOUSE_FEE_VIEW]: '查看仓储费用',
  [PERMISSIONS.WAREHOUSE_FEE_MANAGE]: '管理仓储费用',
  [PERMISSIONS.ADVERTISING_FEE_VIEW]: '查看广告费用',
  [PERMISSIONS.ADVERTISING_FEE_IMPORT]: '导入广告费用',
  [PERMISSIONS.ADVERTISING_FEE_MANAGE]: '管理广告费用',
  [PERMISSIONS.FREIGHT_FEE_VIEW]: '查看物流费用',
  [PERMISSIONS.FREIGHT_FEE_IMPORT]: '导入物流费用',
  [PERMISSIONS.FREIGHT_FEE_MANAGE]: '管理物流费用',
  [PERMISSIONS.PII_READ_FULL]: '查看完整敏感数据',
  [PERMISSIONS.PII_READ_MASKED]: '查看脱敏数据',
  [PERMISSIONS.PII_EXPORT]: '导出敏感数据',
  [PERMISSIONS.SECURITY_SESSION_MANAGE]: '管理登录会话',
  [PERMISSIONS.SECURITY_KEY_ROTATE]: '轮换系统密钥',
  [PERMISSIONS.OBSERVABILITY_READ]: '查看运行监控',
  [PERMISSIONS.OBSERVABILITY_MANAGE]: '管理运行监控',
  [PERMISSIONS.ALERTS_READ]: '查看系统告警',
  [PERMISSIONS.ALERTS_ACK]: '确认系统告警',
  [PERMISSIONS.ALERTS_SILENCE]: '静默系统告警',
  [PERMISSIONS.SLO_READ]: '查看服务目标',
  [PERMISSIONS.SLO_MANAGE]: '管理服务目标',
  [PERMISSIONS.INVENTORY_SYNC_READ]: '查看库存同步',
  [PERMISSIONS.INVENTORY_SYNC_RUN]: '执行库存同步',
  [PERMISSIONS.INVENTORY_SYNC_RERUN]: '重新执行库存同步',
  [PERMISSIONS.INVENTORY_SYNC_AUDIT_READ]: '查看库存同步审计',
  [PERMISSIONS.INVENTORY_SNAPSHOT_READ]: '查看库存快照',
  [PERMISSIONS.SKU_BINDING_READ]: '查看规格映射',
  [PERMISSIONS.SKU_BINDING_MANAGE]: '管理规格映射',
  [PERMISSIONS.SKU_BINDING_RESOLVE_MANUAL]: '人工处理规格映射',
};

const MODULE_LABELS: Record<string, string> = {
  product: '商品管理', ai_text: '商品文案', ai_image: '商品图片', publish: '商品刊登',
  order: '订单管理', sku: '商品规格', inventory: '库存管理', customer: '客服管理',
  task: '任务管理', operationtask: '运营任务', settings: '系统设置', user: '用户与角色',
  operationlog: '操作日志', store: '店铺管理', warehouse: '仓库管理', logistics: '物流管理',
  supplier: '供应商管理', procurement: '采购管理', sales_return: '销售售后',
  audit: '审计记录', config: '系统配置', export: '数据导出', order_profit: '订单利润',
  settlement: '结算管理', warehouse_fee: '仓储费用', advertising_fee: '广告费用', freight_fee: '物流费用',
  pii: '敏感数据', security: '安全设置', observability: '运行监控', alerts: '系统告警', slo: '服务目标',
  inventory_sync: '库存同步', inventory_snapshot: '库存快照', sku_binding: '规格映射',
};

export function permissionLabel(key: string): string {
  return Object.prototype.hasOwnProperty.call(PERMISSION_LABELS, key)
    ? PERMISSION_LABELS[key as PermissionKey] : '未识别权限';
}

export function permissionModuleLabel(key: string): string {
  const module = key.split('.')[0];
  return Object.prototype.hasOwnProperty.call(MODULE_LABELS, module) ? MODULE_LABELS[module] : '其他权限';
}

export function groupPermissions(keys: readonly string[]) {
  const groups = new Map<string, { key: string; label: string; permissions: string[] }>();
  for (const key of keys) {
    const module = key.split('.')[0] || 'other';
    const group = groups.get(module) || { key: module, label: permissionModuleLabel(key), permissions: [] };
    group.permissions.push(key);
    groups.set(module, group);
  }
  return [...groups.values()].sort((a, b) => a.label.localeCompare(b.label, 'zh-CN'));
}
