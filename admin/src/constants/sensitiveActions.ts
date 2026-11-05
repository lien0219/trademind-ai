import { tr, trFormat } from '@/i18n';
import { confirmSensitiveAction } from '@/utils/sensitiveConfirm';

type OkFn = () => void | Promise<void>;

const TASK_CENTER_HINT = '失败任务中心（/ops/task-center/failures）';

/** 保存商品平台刊登配置（不直接上架） */
export function confirmPlatformPublishConfigSave(onOk: OkFn) {
  confirmSensitiveAction({
    title: tr("保存平台刊登配置"),
    content: tr('将修改当前商品的平台刊登配置，不会直接上架。'),
    impacts: [tr('平台刊登配置'), tr('发布检查结果'), tr('后续刊登草稿创建')],
    externalCall: false,
    reversible: true,
    failureHint: tr('可在商品详情查看提示，或在失败任务中心（/ops/task-center/failures）查看相关任务'),
    onOk,
  });
}

/** 应用 AI 文案（标题/描述） */
export function confirmApplyAiText(label: string, onOk: OkFn) {
  confirmSensitiveAction({
    title: trFormat("应用{value0}", { value0: tr(label) }),
    content: trFormat("将把 AI 生成的{value0}写入商品草稿，并更新运营进度。", { value0: tr(label) }),
    impacts: [trFormat('商品草稿{value0}', { value0: tr(label) }), tr('刊登检查状态')],
    reversible: true,
    failureHint: tr('商品详情页可撤销，或在 AI 批次详情查看记录'),
    onOk,
  });
}

/** 撤销 AI 文案 */
export function confirmUndoAiText(label: string, onOk: OkFn) {
  confirmSensitiveAction({
    title: trFormat("撤销{value0}", { value0: tr(label) }),
    content: trFormat("将清除已应用的 AI {value0}，恢复为应用前的内容。", { value0: tr(label) }),
    impacts: [trFormat('商品草稿{value0}', { value0: tr(label) }), tr('运营进度标记')],
    reversible: false,
    onOk,
  });
}

/** 应用 AI 图片 */
export function confirmApplyAiImage(mode: 'append' | 'replace', onOk: OkFn) {
  confirmSensitiveAction({
    title: tr("应用 AI 图片"),
    content: mode === 'replace'
      ? tr('将用 AI 处理结果替换现有主图。')
      : tr('将把 AI 处理结果追加到商品图片。'),
    impacts: [tr('商品主图/详情图'), tr('刊登检查状态')],
    reversible: true,
    failureHint: tr('AI 图片批次详情或商品详情可撤销'),
    onOk,
  });
}

/** 撤销 AI 图片 */
export function confirmUndoAiImage(onOk: OkFn) {
  confirmSensitiveAction({
    title: tr("撤销 AI 图片"),
    content: tr('将移除本次 AI 图片应用的结果。'),
    impacts: [tr('商品图片列表')],
    reversible: false,
    onOk,
  });
}

/** 创建单商品平台草稿（local_draft_only 不写成真实平台调用） */
export function confirmCreatePlatformDraft(localDraftOnly: boolean, onOk: OkFn) {
  confirmSensitiveAction({
    title: localDraftOnly ? tr('创建本地刊登草稿') : tr('创建平台商品草稿'),
    content: localDraftOnly
      ? tr('将在系统内创建本地刊登草稿，不会调用真实平台接口。')
      : tr('将为当前商品创建平台侧草稿，可能调用平台 OpenAPI。'),
    impacts: [tr('刊登草稿记录'), tr('商品运营进度')],
    externalCall: !localDraftOnly,
    reversible: false,
    failureHint: tr(TASK_CENTER_HINT),
    onOk,
  });
}

/** 批量创建刊登草稿 */
export function confirmBatchPublishDraft(count: number, localDraftOnly: boolean, onOk: OkFn) {
  confirmSensitiveAction({
    title: tr("批量创建刊登草稿"),
    content: localDraftOnly
      ? trFormat("将为 {value0} 个商品创建本地刊登草稿，不会调用真实平台接口。", { value0: String(count) })
      : trFormat("将为 {value0} 个商品创建平台草稿，可能调用平台 OpenAPI。", { value0: String(count) }),
    impacts: [trFormat('{count} 个商品的刊登草稿', { count }), tr('刊登批次任务')],
    externalCall: !localDraftOnly,
    reversible: false,
    failureHint: trFormat('刊登批次详情或{value0}', { value0: tr(TASK_CENTER_HINT) }),
    onOk,
  });
}

/** SKU 人工绑定 */
export function confirmSkuManualBind(onOk: OkFn) {
  confirmSensitiveAction({
    title: tr("人工绑定规格"),
    content: tr('将把平台规格与本地 SKU 建立绑定关系，用于订单匹配与库存同步。'),
    impacts: [tr('SKU 绑定状态'), tr('订单匹配结果'), tr('库存同步能力')],
    externalCall: false,
    reversible: true,
    failureHint: tr('商品详情或订单异常页可解除绑定'),
    onOk,
  });
}

/** 解除 SKU 绑定 */
export function confirmSkuUnbind(onOk: OkFn) {
  confirmSensitiveAction({
    title: tr("解除规格绑定"),
    content: tr('解除后订单匹配与库存同步可能中断，需重新绑定。'),
    impacts: [tr('SKU 绑定状态'), tr('订单匹配'), tr('库存同步任务')],
    reversible: false,
    failureHint: tr(TASK_CENTER_HINT),
    onOk,
  });
}

/** 库存同步 */
export function confirmInventorySync(targetLabel: string, externalCall: boolean, onOk: OkFn) {
  confirmSensitiveAction({
    title: tr("库存同步"),
    content: trFormat("将对{value0}发起库存同步。", { value0: String(targetLabel) }),
    impacts: [tr('本地库存快照'), tr('平台库存记录'), tr('库存同步任务')],
    externalCall,
    reversible: false,
    failureHint: trFormat('库存同步任务页或{value0}', { value0: tr(TASK_CENTER_HINT) }),
    onOk,
  });
}

/** 库存人工修正 */
export function confirmInventoryManualAdjust(onOk: OkFn, onCancel?: () => void) {
  confirmSensitiveAction({
    title: tr("人工修正库存"),
    content: tr('将修改所选仓库的在手库存，并同步更新本地规格库存汇总；不会自动同步到平台。'),
    impacts: [tr('仓库库存余额与流水'), tr('本地规格库存汇总'), tr('库存预警状态')],
    externalCall: false,
    reversible: false,
    onOk,
    onCancel: onCancel ? () => onCancel() : undefined,
  });
}

/** 客服回复发送 */
export function confirmCustomerReplySend(externalCall: boolean, onOk: OkFn) {
  confirmSensitiveAction({
    title: tr("发送客服回复"),
    content: externalCall
      ? tr('将通过平台接口向买家发送回复，发送后通常不可撤回。')
      : tr('将记录人工回复（演示/手动会话，不调用外部平台）。'),
    impacts: [tr('会话消息记录'), tr('买家可见回复')],
    externalCall,
    reversible: false,
    failureHint: tr(TASK_CENTER_HINT),
    onOk,
  });
}

/** 失败任务重试 */
export function confirmFailureTaskRetry(count: number, onOk: OkFn) {
  const n = count > 1 ? trFormat('{count} 条失败任务', { count }) : tr('该失败任务');
  confirmSensitiveAction({
    title: count > 1 ? tr("批量重试失败任务") : tr("重试失败任务"),
    content: trFormat("将重新执行{value0}，可能再次调用外部平台或 AI 服务。", { value0: String(n) }),
    impacts: [n, tr('相关任务状态')],
    externalCall: true,
    reversible: false,
    failureHint: tr(TASK_CENTER_HINT),
    onOk,
  });
}

/** 解除店铺授权 */
export function confirmRevokeStoreAuth(shopName: string, onOk: OkFn) {
  confirmSensitiveAction({
    title: tr("解除店铺授权"),
    content: trFormat("将解除「{value0}」的平台授权，订单/库存/客服同步将停止。", { value0: String(shopName) }),
    impacts: [tr('店铺授权状态'), tr('订单同步'), tr('库存同步'), tr('客服消息同步')],
    externalCall: true,
    reversible: false,
    failureHint: tr('店铺管理页重新授权'),
    onOk,
  });
}

/** 平台配置保存 */
export function confirmPlatformConfigSave(onOk: OkFn) {
  confirmSensitiveAction({
    title: tr("保存平台配置"),
    content: tr('将更新平台接入配置，可能影响店铺授权与同步行为。'),
    impacts: [tr('平台应用配置'), tr('店铺授权有效性'), tr('同步任务')],
    externalCall: false,
    reversible: true,
    failureHint: tr('配置状态中心查看风险项'),
    onOk,
  });
}

/** Storage 公网测试 */
export function confirmStoragePublicTest(onOk: OkFn) {
  confirmSensitiveAction({
    title: tr("测试存储公网访问"),
    content: tr('将上传测试文件并尝试通过公网 URL 访问，用于验证存储配置。'),
    impacts: [tr('临时测试文件')],
    externalCall: true,
    reversible: true,
    onOk,
  });
}

/** 修改用户角色 */
export function confirmChangeUserRole(username: string, roleLabel: string, onOk: OkFn) {
  confirmSensitiveAction({
    title: tr("修改用户角色"),
    content: trFormat("将把用户「{value0}」的角色调整为「{value1}」。", { value0: String(username), value1: String(roleLabel) }),
    impacts: [tr('用户权限范围'), tr('菜单可见性'), tr('写操作能力')],
    reversible: true,
    onOk,
  });
}

/** 分配店铺权限 */
export function confirmAssignStorePermissions(username: string, onOk: OkFn) {
  confirmSensitiveAction({
    title: tr("保存店铺权限"),
    content: trFormat("将更新用户「{value0}」的店铺授权范围。", { value0: String(username) }),
    impacts: [tr('店铺数据可见范围'), tr('写操作授权')],
    reversible: true,
    onOk,
  });
}
