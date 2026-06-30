# 全局状态文案审计（Phase F6）

## 原则

1. 主 UI 不直出内部英文码
2. 技术详情折叠显示内部码（`TechnicalDetails`）
3. warning 不写成失败；failed 不写成建议
4. 错误提示含下一步

## 统一映射

`admin/src/constants/copywriting.ts` → `COMMON_STATUS_LABEL` + `commonStatusLabel()`

已收口（F6）：

| 内部码 | 用户可见 |
| --- | --- |
| pending_review | 待复核 |
| partial_success | 部分成功 |
| local_draft_only | 仅本地草稿 |
| real_draft_create | 创建平台草稿 |
| blocked_by_real_credentials | 缺少真实凭证 |
| blocked_by_provider_config | 接入服务未配置 |
| unsupported_by_provider | 当前服务不支持 |
| permission_denied | 无权限 |
| readonly_operation_forbidden | 只读账号不可操作 |
| store_permission_denied | 无店铺权限 |
| inventory_sync_enabled | 库存同步已开启 |
| manual_bound | 人工绑定 |
| ambiguous | 需要人工确认 |
| unmatched | 未匹配 |

## 模块专用

- 刊登：`admin/src/constants/publishLabels.ts`
- 商品运营：`admin/src/constants/productOperationLabels.ts`
- 任务中心：`admin/src/constants/taskCenter.ts`
- 后端：`backend/internal/pkg/opslabels`
