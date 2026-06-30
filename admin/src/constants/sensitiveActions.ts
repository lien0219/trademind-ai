import { confirmSensitiveAction } from '@/utils/sensitiveConfirm';

type OkFn = () => void | Promise<void>;

const TASK_CENTER_HINT = '失败任务中心（/ops/task-center/failures）';

/** 应用 AI 文案（标题/描述） */
export function confirmApplyAiText(label: string, onOk: OkFn) {
  confirmSensitiveAction({
    title: `应用${label}`,
    content: `将把 AI 生成的${label}写入商品草稿，并更新运营进度。`,
    impacts: [`商品草稿${label}`, '刊登检查状态'],
    reversible: true,
    failureHint: '商品详情页可撤销，或在 AI 批次详情查看记录',
    onOk,
  });
}

/** 撤销 AI 文案 */
export function confirmUndoAiText(label: string, onOk: OkFn) {
  confirmSensitiveAction({
    title: `撤销${label}`,
    content: `将清除已应用的 AI ${label}，恢复为应用前的内容。`,
    impacts: [`商品草稿${label}`, '运营进度标记'],
    reversible: false,
    onOk,
  });
}

/** 应用 AI 图片 */
export function confirmApplyAiImage(mode: 'append' | 'replace', onOk: OkFn) {
  const modeLabel = mode === 'replace' ? '替换现有主图' : '追加到商品图片';
  confirmSensitiveAction({
    title: '应用 AI 图片',
    content: `将把 AI 处理结果${modeLabel}。`,
    impacts: ['商品主图/详情图', '刊登检查状态'],
    reversible: true,
    failureHint: 'AI 图片批次详情或商品详情可撤销',
    onOk,
  });
}

/** 撤销 AI 图片 */
export function confirmUndoAiImage(onOk: OkFn) {
  confirmSensitiveAction({
    title: '撤销 AI 图片',
    content: '将移除本次 AI 图片应用的结果。',
    impacts: ['商品图片列表'],
    reversible: false,
    onOk,
  });
}

/** 创建单商品平台草稿（local_draft_only 不写成真实平台调用） */
export function confirmCreatePlatformDraft(localDraftOnly: boolean, onOk: OkFn) {
  confirmSensitiveAction({
    title: localDraftOnly ? '创建本地刊登草稿' : '创建平台商品草稿',
    content: localDraftOnly
      ? '将在系统内创建本地刊登草稿，不会调用真实平台接口。'
      : '将为当前商品创建平台侧草稿，可能调用平台 OpenAPI。',
    impacts: ['刊登草稿记录', '商品运营进度'],
    externalCall: !localDraftOnly,
    reversible: false,
    failureHint: TASK_CENTER_HINT,
    onOk,
  });
}

/** 批量创建刊登草稿 */
export function confirmBatchPublishDraft(count: number, localDraftOnly: boolean, onOk: OkFn) {
  confirmSensitiveAction({
    title: '批量创建刊登草稿',
    content: localDraftOnly
      ? `将为 ${count} 个商品创建本地刊登草稿，不会调用真实平台接口。`
      : `将为 ${count} 个商品创建平台草稿，可能调用平台 OpenAPI。`,
    impacts: [`${count} 个商品的刊登草稿`, '刊登批次任务'],
    externalCall: !localDraftOnly,
    reversible: false,
    failureHint: '刊登批次详情或' + TASK_CENTER_HINT,
    onOk,
  });
}

/** SKU 人工绑定 */
export function confirmSkuManualBind(onOk: OkFn) {
  confirmSensitiveAction({
    title: '人工绑定规格',
    content: '将把平台规格与本地 SKU 建立绑定关系，用于订单匹配与库存同步。',
    impacts: ['SKU 绑定状态', '订单匹配结果', '库存同步能力'],
    externalCall: false,
    reversible: true,
    failureHint: '商品详情或订单异常页可解除绑定',
    onOk,
  });
}

/** 解除 SKU 绑定 */
export function confirmSkuUnbind(onOk: OkFn) {
  confirmSensitiveAction({
    title: '解除规格绑定',
    content: '解除后订单匹配与库存同步可能中断，需重新绑定。',
    impacts: ['SKU 绑定状态', '订单匹配', '库存同步任务'],
    reversible: false,
    failureHint: TASK_CENTER_HINT,
    onOk,
  });
}

/** 库存同步 */
export function confirmInventorySync(targetLabel: string, externalCall: boolean, onOk: OkFn) {
  confirmSensitiveAction({
    title: '库存同步',
    content: `将对${targetLabel}发起库存同步。`,
    impacts: ['本地库存快照', '平台库存记录', '库存同步任务'],
    externalCall,
    reversible: false,
    failureHint: '库存同步任务页或' + TASK_CENTER_HINT,
    onOk,
  });
}

/** 库存人工修正 */
export function confirmInventoryManualAdjust(onOk: OkFn) {
  confirmSensitiveAction({
    title: '人工修正库存',
    content: '将直接修改本地 SKU 库存数量，不会自动同步到平台。',
    impacts: ['本地 SKU 库存', '库存预警状态'],
    externalCall: false,
    reversible: false,
    onOk,
  });
}

/** 客服回复发送 */
export function confirmCustomerReplySend(externalCall: boolean, onOk: OkFn) {
  confirmSensitiveAction({
    title: '发送客服回复',
    content: externalCall
      ? '将通过平台接口向买家发送回复，发送后通常不可撤回。'
      : '将记录人工回复（演示/手动会话，不调用外部平台）。',
    impacts: ['会话消息记录', '买家可见回复'],
    externalCall,
    reversible: false,
    failureHint: TASK_CENTER_HINT,
    onOk,
  });
}

/** 失败任务重试 */
export function confirmFailureTaskRetry(count: number, onOk: OkFn) {
  const n = count > 1 ? `${count} 条失败任务` : '该失败任务';
  confirmSensitiveAction({
    title: count > 1 ? '批量重试失败任务' : '重试失败任务',
    content: `将重新执行${n}，可能再次调用外部平台或 AI 服务。`,
    impacts: [n, '相关任务状态'],
    externalCall: true,
    reversible: false,
    failureHint: TASK_CENTER_HINT,
    onOk,
  });
}

/** 解除店铺授权 */
export function confirmRevokeStoreAuth(shopName: string, onOk: OkFn) {
  confirmSensitiveAction({
    title: '解除店铺授权',
    content: `将解除「${shopName}」的平台授权，订单/库存/客服同步将停止。`,
    impacts: ['店铺授权状态', '订单同步', '库存同步', '客服消息同步'],
    externalCall: true,
    reversible: false,
    failureHint: '店铺管理页重新授权',
    onOk,
  });
}

/** 平台配置保存 */
export function confirmPlatformConfigSave(onOk: OkFn) {
  confirmSensitiveAction({
    title: '保存平台配置',
    content: '将更新平台接入配置，可能影响店铺授权与同步行为。',
    impacts: ['平台应用配置', '店铺授权有效性', '同步任务'],
    externalCall: false,
    reversible: true,
    failureHint: '配置状态中心查看风险项',
    onOk,
  });
}

/** Storage 公网测试 */
export function confirmStoragePublicTest(onOk: OkFn) {
  confirmSensitiveAction({
    title: '测试存储公网访问',
    content: '将上传测试文件并尝试通过公网 URL 访问，用于验证存储配置。',
    impacts: ['临时测试文件'],
    externalCall: true,
    reversible: true,
    onOk,
  });
}

/** 修改用户角色 */
export function confirmChangeUserRole(username: string, roleLabel: string, onOk: OkFn) {
  confirmSensitiveAction({
    title: '修改用户角色',
    content: `将把用户「${username}」的角色调整为「${roleLabel}」。`,
    impacts: ['用户权限范围', '菜单可见性', '写操作能力'],
    reversible: true,
    onOk,
  });
}

/** 分配店铺权限 */
export function confirmAssignStorePermissions(username: string, onOk: OkFn) {
  confirmSensitiveAction({
    title: '保存店铺权限',
    content: `将更新用户「${username}」的店铺授权范围。`,
    impacts: ['店铺数据可见范围', '写操作授权'],
    reversible: true,
    onOk,
  });
}

/** 禁用用户 */
export function confirmDisableUser(username: string, onOk: OkFn) {
  confirmSensitiveAction({
    title: '禁用用户',
    content: `将禁用用户「${username}」，该账号将无法登录。`,
    impacts: ['账号登录状态'],
    reversible: true,
    onOk,
  });
}
