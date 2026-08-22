# API 契约

本文件记录 TradeMind 后端 API 的公共约定。新增、删除或修改接口时，必须同步检查后端 handler / service / DTO、前端 services / types / 页面，以及本文档。

## 基础约定

- 基础路径：`/api/v1`
- 健康检查：`GET /health`、`GET /api/v1/health`（综合）；`GET /health/live`（存活）；`GET /health/ready`（就绪，DB/Redis/迁移/生产门闸）
- 可观测性（需权限）：`GET /api/v1/observability/overview` 返回 `overallStatus`、受保护指标端点、活跃系统告警、最近告警窗口评估、SLO 评估和 telemetry 导出摘要。OTLP 状态区分等待首次导出、最近成功和最近失败，未完成首次导出不会显示为健康。总体状态只表示当前可观测性运行情况，不表示系统已经完成生产发布。系统告警统一在 Admin 告警中心处置，`GET /api/v1/observability/alerts` 支持 `page`、`pageSize`（最大 200）、`status`、`severity`、`module`，并返回 `items[]` 与 `pagination`，旧 `limit` 参数继续兼容；列表字段为 `id`、`ruleId`、`severity`、`status`、`module`、`summary`、`occurrenceCount`、`lastSeenAt`；`POST /api/v1/observability/alerts/:id/ack` 确认告警，`POST /api/v1/observability/alerts/:id/silence` 必须提交 `reason` 与 1-720 小时的 `durationHours`；内部指标：`GET /internal/metrics`，由 Nginx 精确路径与 Gin CIDR 双层保护。固定返回聚合占位值的旧 `/http`、`/tasks`、`/providers`、`/security` 接口不再注册。
- 鉴权：管理端受保护接口使用 `Authorization: Bearer <token>`
- 返回格式：统一 JSON 响应，核心字段为 `code`、`message`、`data`、`traceId`
- 敏感信息：接口不得返回完整 API Key、Token、Secret、Cookie 或密码
- P7-C3 cursor 列表：Product、Order、Inventory Center、Task Center、Webhook Event、Operation Log 支持 `cursor` + `limit`，响应额外返回 `items`、`nextCursor`、`hasMore`、`limit`；旧 `page` / `pageSize` / `list` / `pagination` 兼容保留。超过深 offset 返回 `pagination_offset_too_deep`；cursor 篡改、跨租户/店铺或筛选变化分别返回 `pagination_cursor_signature_invalid`、`pagination_cursor_scope_mismatch`、`pagination_cursor_filter_mismatch`。P7-C4 隔离 Medium PostgreSQL 六类分页 runtime、Query Plan、N+1、Provider 限流、Permission Cache 失效与 Linux Race 证据已关闭；Load/Soak/Regression 仍 pending P7-V2。

## Webhook 入站（公开，无 JWT）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/webhooks/:platform/:eventType` | 平台 Webhook 接收：限体、`Content-Type: application/json`、签名/时间戳校验、幂等持久化、快速 ACK；异步由 DB 轮询 Worker 处理。开发可用 `platform=internal-test`（需 `WEBHOOK_ENABLE_TEST_VERIFIER=true`）。成功 `message=accepted`，`data.eventId` / `duplicate`。 |

## Webhook 事件（管理端）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/webhook-events` | 受保护的 Webhook 事件列表；支持 `platform`、`status`、`eventType`、`shopId`、`start`、`end`、`cursor`、`limit`。只返回元数据、摘要和状态，不返回 `payloadBody` 或签名原文。 |

签名头：`X-Webhook-Signature` 或 `X-TradeMind-Signature`；时间戳：`X-Webhook-Timestamp` / `X-TradeMind-Timestamp`（unix 秒或 RFC3339）。`internal-test` 签名为 HMAC-SHA256 hex（payload = `"{unix}.{rawBody}"`）。失败码含 `WEBHOOK_SIGNATURE_*`、`WEBHOOK_TIMESTAMP_EXPIRED`、`WEBHOOK_PAYLOAD_TOO_LARGE` 等，**不**成功 ACK。

示例：

```json
{
  "code": 0,
  "message": "ok",
  "data": {},
  "traceId": "request-id"
}
```

## 认证

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/auth/login` | 管理员登录，支持邮箱或手机号。 |
| `POST` | `/api/v1/auth/logout` | 退出登录，客户端丢弃 token。 |
| `GET` | `/api/v1/auth/profile` | 当前管理员信息。 |

## 设置

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/settings` | 读取系统设置。 |
| `PUT` | `/api/v1/settings` | 保存系统设置，敏感字段必须加密。 |
| `POST` | `/api/v1/settings/test-ai` | 经 **AI Gateway** 测试 `settings.ai`（支持 `openai` / `openai_compatible` / `deepseek` / `qwen`）。各服务商 **`{provider}_api_key` / `{provider}_base_url` / `{provider}_model`** 独立存储；可选 JSON：`provider`、`base_url`、`model`、`api_key`（写入当前 provider 对应项；`****` 占位则沿用已保存密钥）、`timeout_sec`，用于**未保存前**用当前表单试连；空 body 仅用库内配置。成功 `data`：`ok`、`message`、`provider`、`model`、`latencyMs`。 |
| `POST` | `/api/v1/settings/test-storage` | 测试 Storage Provider 配置。 |
| `POST` | `/api/v1/storage/test-public-access` | 上传探针图片并通过匿名 HTTP 验证公网可访问性（HTTPS、`image/*`、无登录跳转）；需 `settings.manage`；失败返回 `STORAGE_PUBLIC_*` 错误码。 |
| `POST` | `/api/v1/settings/storage/public-check` | 同上（P1 别名） |
| `GET` | `/api/v1/settings/storage/public-check/latest` | 最近一次公网测试结果（未执行时 `not_run`） |
| `POST` | `/api/v1/settings/test-image` | 测试 `settings.image` 图片 Provider 配置。可选 JSON：`provider`、`testMode`（`config_only` \| `live`，默认 `config_only`）、`settings`（表单覆盖项，支持未保存先测；脱敏 `****` 占位符会忽略并沿用已保存密钥）。成功 `data`：`ok`、`message`、`provider`、`latencyMs`、`supportedTasks`、`configStatus`。不返回 API Key。 |
| `POST` | `/api/v1/settings/test-ocr` | 测试 `settings.image` 中的 OCR 配置。可选 JSON：`provider`（`ai_vision` / `paddleocr` / `baidu` / `aliyun` / `tencent`）、`settings`（表单覆盖项，支持未保存先测；脱敏密钥占位符会忽略）。`paddleocr` 会用后端生成的测试图调用 OCR 服务，检查连通性、文字 `blocks` 与 `bbox`；成功 `data`：`ok`、`message`、`provider`、`latencyMs`、`blocks`、`bboxOk`。 |

## ERP 采购基础

所有接口均需 JWT、租户上下文和对应权限。采购状态写使用 `expectedRevision` 防止并发覆盖；收货使用调用方生成的稳定 `idempotencyKey`，同一采购单下同键同 payload 返回原结果，同键不同 payload 返回 `409`。

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/warehouses` | `warehouse.view` | 当前租户仓库列表。 |
| `POST` | `/api/v1/warehouses` | `warehouse.manage` | 创建仓库；JSON：`code`、`name`、`isDefault`。 |
| `PUT` | `/api/v1/warehouses/:id` | `warehouse.manage` | 更新仓库名称、启停状态和默认仓；JSON：`name`、`status`、`isDefault`。默认仓必须启用，同租户默认仓在事务内唯一切换。 |
| `GET` | `/api/v1/products/:id/skus/:skuId/warehouse-balances` | `inventory.view` | 读取当前租户下该规格的分仓余额；返回仓库名称、在手、预占、在途、残损、可用量和版本。 |
| `POST` | `/api/v1/products/:id/skus/:skuId/adjust-stock` | `inventory.operate` | 人工调整所选仓库的在手库存；JSON：`warehouseId`、`stock`、`idempotencyKey`、可选 `reason` / `remark`。同键同 payload 幂等返回，同键不同 payload 返回 `409`；仓库余额、不可变流水、兼容变更日志与 `product_skus.stock` 兼容聚合字段在同一事务提交，并保留尚未迁移订单路径形成的差额，不创建平台同步任务。 |
| `GET` | `/api/v1/inventory/warehouse-ledger/reconciliation` | `inventory.view` | 分页对账仓库在手合计与 `product_skus.stock`；支持 `page`、`pageSize`、`status=matched|unmigrated|mismatch`。 |
| `POST` | `/api/v1/inventory/warehouse-ledger/migrate-legacy` | `inventory.operate` | 重复安全地迁移一批尚无仓库余额的历史规格；JSON：`limit`（默认 100，最大 500）。优先进入启用的默认仓，没有默认仓时创建/复用租户级 `PENDING_ALLOCATION` 待分配仓。 |
| `GET` | `/api/v1/inventory/warehouse-transfers` | `inventory.view` | 分页查看当前租户调拨单；支持 `page`、`pageSize`、`status`。 |
| `GET` | `/api/v1/inventory/warehouse-transfers/:id` | `inventory.view` | 查看调拨单明细和当前 revision。 |
| `POST` | `/api/v1/inventory/warehouse-transfers` | `inventory.operate` | 创建单仓到单仓调拨草稿；JSON：`idempotencyKey`、`sourceWarehouseId`、`targetWarehouseId`、`reason`、`remark`、`items[]`，第一版每个 SKU 仅允许一条明细。 |
| `POST` | `/api/v1/inventory/warehouse-transfers/:id/submit` | `inventory.operate` | 草稿提交审批；JSON：`expectedRevision`、`idempotencyKey`、可选 `reason`。 |
| `POST` | `/api/v1/inventory/warehouse-transfers/:id/approve` | `inventory.approve` | 审批调拨单；与调拨创建、发出和收货分离，支持 reviewer 职责分离。 |
| `POST` | `/api/v1/inventory/warehouse-transfers/:id/dispatch` | `inventory.operate` | 发出调拨，校验可用库存并把源仓在手转入在途；事务内写不可变流水。 |
| `POST` | `/api/v1/inventory/warehouse-transfers/:id/receive` | `inventory.operate` | 目标仓收货，把源仓在途转为目标仓在手；同一 action 幂等。 |
| `POST` | `/api/v1/inventory/warehouse-transfers/:id/cancel` | `inventory.operate` | 取消草稿、待审批或已审批调拨；发出后不可取消。 |
| `GET` | `/api/v1/inventory/stocktakes` | `inventory.view` | 分页查看当前租户盘点单；支持 `page`、`pageSize`、`status`。 |
| `GET` | `/api/v1/inventory/stocktakes/:id` | `inventory.view` | 查看盘点快照、实盘数量和当前 revision。 |
| `POST` | `/api/v1/inventory/stocktakes` | `inventory.operate` | 创建盘点中草稿；JSON：`idempotencyKey`、`warehouseId`、`reason`、`remark`、`items[]`。创建时记录仓库余额快照并确保历史 SKU 已进入仓库账。 |
| `PATCH` | `/api/v1/inventory/stocktakes/:id/items/:itemId` | `inventory.operate` | 录入或更正一条实盘数量；JSON：`expectedRevision`、`idempotencyKey`、`countedOnHand`、`remark`。同键同 payload 幂等返回，同键不同 payload 返回 `409`。 |
| `POST` | `/api/v1/inventory/stocktakes/:id/submit` | `inventory.operate` | 提交盘点审核；所有明细必须已有实盘数量。 |
| `POST` | `/api/v1/inventory/stocktakes/:id/approve` | `inventory.approve` | 审核盘点结果；与盘点创建、录入和过账职责分离。 |
| `POST` | `/api/v1/inventory/stocktakes/:id/post` | `inventory.operate` | 过账盘点差异；校验快照版本，事务内更新仓库在手、不可变流水、兼容聚合和变更日志；快照过期返回 `409` 且不产生部分写入。 |
| `POST` | `/api/v1/inventory/stocktakes/:id/cancel` | `inventory.operate` | 取消盘点中、待审核或已审核盘点；过账后不可取消。 |
| `GET` | `/api/v1/suppliers` | `supplier.view` | 当前租户供应商列表；无 `pii.read_full` 时电话和邮箱脱敏。 |
| `POST` | `/api/v1/suppliers` | `supplier.manage` | 创建供应商；JSON：`code`、`name`、`contactName`、`phone`、`email`。 |
| `PUT` | `/api/v1/suppliers/:id` | `supplier.manage` | 更新供应商名称、启停状态和联系方式；JSON：`name`、`status`、`contactName`，可选 `phone`、`email`，敏感字段省略时保留原值，响应继续按权限脱敏。 |
| `GET` | `/api/v1/suppliers/:id/skus` | `supplier.view` | 查询供应商关联的本地商品规格及供应商货号、采购价、起订量和交期。 |
| `POST` | `/api/v1/suppliers/:id/skus` | `supplier.manage` | 绑定本地 SKU；JSON：`productSkuId`、`supplierSkuCode`、`unitCostMinor`、`currency`、`minOrderQty`、`leadTimeDays`。 |
| `GET` | `/api/v1/purchase-orders` | `procurement.view` | 采购单分页列表，支持 `page`、`pageSize`。 |
| `POST` | `/api/v1/purchase-orders` | `procurement.manage` | 幂等创建采购单；JSON：`idempotencyKey`、`supplierId`、`warehouseId`、`currency`、`remark`、`items[]`。明细含 `productSkuId`、可选 `supplierSkuId`、`quantity`、`unitCostMinor`。 |
| `GET` | `/api/v1/purchase-orders/:id` | `procurement.view` | 采购单及明细；明细附带租户内商品标题、规格编码和规格名称作为只读展示字段。 |
| `POST` | `/api/v1/purchase-orders/:id/submit` | `procurement.manage` | 草稿提交审批；JSON：`expectedRevision`、可选 `reason`。 |
| `POST` | `/api/v1/purchase-orders/:id/approve` | `procurement.approve` | 审批采购单；JSON 同上。 |
| `POST` | `/api/v1/purchase-orders/:id/cancel` | `procurement.manage` | 取消尚未收货的采购单；JSON 同上。 |
| `POST` | `/api/v1/purchase-orders/:id/close` | `procurement.manage` | 关闭已审批或部分收货采购单；JSON 同上。 |
| `POST` | `/api/v1/purchase-orders/:id/receipts` | `procurement.receive` | 分批收货；JSON：`expectedRevision`、`idempotencyKey`、`items[]`，明细含 `purchaseOrderItemId`、`quantity`。采购明细、收货记录、库存余额、库存流水和兼容聚合库存在同一事务提交。 |
| `GET` | `/api/v1/purchase-orders/:id/returnable-receipt-items` | `procurement.view` | 查询原收货明细的已收、有效退货占用和剩余可退数量；已取消退货不占用额度。 |
| `GET` | `/api/v1/purchase-returns` | `procurement.view` | 采购退货分页列表；支持 `page`、`pageSize`、`status`、`purchaseOrderId`。 |
| `POST` | `/api/v1/purchase-returns` | `procurement.manage` | 幂等创建采购退货草稿；JSON：`idempotencyKey`、`purchaseOrderId`、必填 `reason`、`remark`、`items[]`，明细含 `goodsReceiptItemId`、`quantity`。所有未取消退货单共同占用原收货可退额度。 |
| `GET` | `/api/v1/purchase-returns/:id` | `procurement.view` | 采购退货详情及原收货关联明细。 |
| `POST` | `/api/v1/purchase-returns/:id/submit` | `procurement.manage` | 提交退货审批；JSON：`expectedRevision`、`idempotencyKey`、可选 `reason`。 |
| `POST` | `/api/v1/purchase-returns/:id/approve` | `procurement.approve` | 审批采购退货；审批人与最终执行人必须为不同账号。 |
| `POST` | `/api/v1/purchase-returns/:id/complete` | `procurement.return` | 执行退货；校验仓库可用量，单事务扣减仓库在手、写不可变流水、兼容库存投影和变更日志。库存不足整笔回滚。 |
| `POST` | `/api/v1/purchase-returns/:id/cancel` | `procurement.manage` | 取消草稿、待审批或已审批退货并释放原收货可退额度；完成后不可取消。 |

金额字段均为整数最小货币单位。`400` 表示字段或租户资源无效，`404` 表示资源在当前租户不可见，`409` 表示 revision、状态、超收/超退、库存不足、库存账差异、职责分离或幂等冲突。采购工作台位于采购菜单下，库存账迁移与对账位于库存菜单下；这些 API 不触发真实平台库存同步，也不包含供应商退款或财务结算。

## 订单库存生命周期

订单、订单明细和库存 effect 均按当前租户隔离。订单库存只支持单订单单仓：手工订单首次应用库存时必须提供启用的 `warehouseId`；平台订单首次处理时绑定当前租户启用的默认仓。已有成功库存 effect 后不能修改订单仓库、订单明细数量或删除明细；订单删除前必须先完成释放或回补。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/orders` | 订单列表（需要 `order.view`）；支持订单、支付、履约、库存 effect 和同步状态筛选。 |
| `POST` | `/api/v1/orders` | 创建订单（需要 `order.operate`）；可含 `warehouseId`、`items[]`、`deductInventory`、`syncInventory`。当创建后应用库存且为手工订单时仓库必填。若订单已持久化但库存处理失败，返回 `409`，`data` 包含 `orderId`、`order` 和 `inventoryDeduction`，可在详情中重试。 |
| `GET` | `/api/v1/orders/:id` | 订单详情（需要 `order.view`）；返回订单级 `warehouseId` 与库存生命周期摘要。 |
| `PUT` | `/api/v1/orders/:id` | 更新订单基础字段（需要 `order.operate`）；可提交 `warehouseId` / `setWarehouseIdNil`，有库存 effect 后不得改变仓库。 |
| `POST` | `/api/v1/orders/:id/deduct-inventory` | 按订单状态应用库存（需要 `order.operate`）：已支付/处理中增加 `reserved`，已发货/已履约扣减 `on_hand` 并消费预占。JSON：`warehouseId`、`syncInventory`。 |
| `POST` | `/api/v1/orders/:id/restore-inventory` | 取消/退款补偿（需要 `order.operate`）：发货前释放 `reserved`，已出库订单回补 `on_hand`。JSON：`warehouseId`、`syncInventory`、`reason`。 |
| `GET` | `/api/v1/orders/:id/inventory-effects` | 查询订单库存 effect（需要 `order.view`），支持 `page`、`pageSize`；每条记录含 effect 类型、仓库、数量和兼容库存前后值。 |

预占不会提前修改 `product_skus.stock`；实际出库和回补会在同一事务更新仓库余额、不可变 `inventory_movements`、兼容变更日志、`order_inventory_effects` 与兼容聚合字段。重复处理按订单行和 effect 类型幂等，旧成功扣减 effect 会在首次补偿时绑定租户与仓库。`syncInventory` 只沿现有库存同步任务与 fail-closed 平台边界处理，不代表已经向真实平台写入库存。

## 图片 AI

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/image/providers` | 图片 Provider 能力矩阵（`status` / `supportedTasks` / 难度等，不含密钥）。 |
| `POST` | `/api/v1/image/tasks` | 创建图片任务；创建时校验 Provider 与 `taskType` 组合。 |
| `GET` | `/api/v1/image/tasks` | 图片任务列表。 |
| `GET` | `/api/v1/image/tasks/:id` | 图片任务详情。 |
| `POST` | `/api/v1/image/tasks/:id/retry` | 重试失败任务。 |
| `GET` | `/api/v1/image/tasks/:id/translate-edit-state` | 图片文字翻译人工编辑态：返回原图、已擦除底图、结果图、图片尺寸与可编辑文字块（译文、排版框、擦除框、样式）。 |
| `POST` | `/api/v1/image/tasks/:id/manual-render` | 图片文字翻译人工兜底渲染：按人工编辑后的文字块重新擦除原文并规则重绘译文，结果上传 Storage Provider 并回写任务为 `success_with_review`。 |
| `GET` | `/api/v1/image/tasks/:id/items` | 任务子项列表（源图→结果图、评分 JSON）。 |
| `POST` | `/api/v1/image/tasks/:id/apply` | 将成功任务结果写入 `product_images`（不覆盖原图）。 |
| `GET` | `/api/v1/image/tasks/monitor` | 队列与任务监控快照。 |
| `POST` | `/api/v1/ai/image/tasks` | 创建 AI 图片任务（与 `/image/tasks` 等价）。 |
| `GET` | `/api/v1/ai/image/tasks` | AI 图片任务列表。 |
| `GET` | `/api/v1/ai/image/tasks/:id` | AI 图片任务详情。 |
| `GET` | `/api/v1/ai/image/tasks/:id/translate-edit-state` | 与 `/image/tasks/:id/translate-edit-state` 等价，用于管理端 AI 图片任务页。 |
| `POST` | `/api/v1/ai/image/tasks/:id/manual-render` | 与 `/image/tasks/:id/manual-render` 等价，用于管理端 AI 图片任务页。 |
| `POST` | `/api/v1/ai/image/task-items/:id/save-to-product` | 将任务子项结果保存为新商品图（`applyMode`: main/detail/marketing/ai_generated）。 |
| `POST` | `/api/v1/ai/image/task-items/:id/set-as-main` | 将任务子项结果设为主图（`is_best_main`）。 |
| `POST` | `/api/v1/ai/image/score` | 同步商品图评分（返回 overall/clarity/cleanliness 等维度）。 |

`translate_image_text`（图片文字翻译）读取「设置 → 图片 AI 设置」里的 OCR 配置：`ai_vision` 使用当前 AI 设置中的视觉模型；`paddleocr` 使用本地 PaddleOCR 服务；`aliyun` 会真实调用阿里云 OCR；`tencent` 会真实调用腾讯云 OCR，支持 `GeneralBasicOCR` 与 `GeneralFastOCR`。该任务采用严格 OCR 模式：配置哪个 OCR Provider 就必须实际调用哪个 Provider；OCR 未配置、配置不完整、调用失败或未识别到文字时任务直接失败，不会自动改用其他 OCR。详情输出会包含 `ocr.provider`、`ocr.apiName`、`ocr.configuredOcrProvider`、`ocr.actualOcrProvider`、`ocr.textBlocksCount`、`ocr.averageConfidence`、`ocr.filteredBlocksCount`、`ocr.errorMessage`、`ocr.blocks`、`ocr.groups`、`layout.layoutTemplate` 与 `renderQuality`。每个 OCR block 会补充 `blockClass`、`standardTranslation` 与 `compactTranslation`；顶层会补充 `blockClassifications`、`eraseBBoxCount`、`layoutBBoxCount`、`badgeCount`、`abnormalBadgeCount`、`backgroundPatchScore`、`overlapScore` 与 `finalQualityStatus` 分级：`success`（商用分≥85）、`success_with_review`（75–84，可下载，保存到商品前建议人工检查）、`failed_render_validation`（<65 或中文残留/溢出/遮挡商品主体等硬失败）。调试输出：`debugOriginalUrl`、`debugMaskUrl`、`debugErasedUrl`、`debugFinalUrl`（对应 original/mask/erased/final.png）。65–74 分同任务内自动质量重试一次（`qualityAutoRetried`）。人工兜底使用 `translate-edit-state` 读取可编辑块，再用 `manual-render` 基于原图/已擦除图重新擦除原文并规则重绘译文；输出会记录 `manualEdit`（baseImage、blocks、editedAt、editedBy、eraseMode 等），任务回写为 `success_with_review`。`layout` 还包含 `eraseMode`、`eraseAreaRatio`、`patchAreaRatio`、`flatFillRatio`、`largePatchDetected`、`retryStrategies`、`simulation` 等渲染诊断；顶层同步输出 `configuredOcrProvider`、`actualOcrProvider`、`ocrBlocksCount`、`ocrAverageConfidence`、`detected_source_blocks`、`translated_blocks`、`rendered_blocks`、`target_language_present`、`source_language_residue`、`overflow_blocks`、`style_mismatch_count`、`patch_area_ratio`、`render_quality_score`、`overall_confidence` 便于任务详情和批量排查。`renderQuality` 包含 `textAppliedScore`、`sourceTextRemovedScore`、`layoutScore`、`styleConsistencyScore`、`readabilityScore`、`productPreservationScore`、`commercialUsabilityScore`、`passed` 与 `warnings`；当出现异常 badge、文字重叠、背景补丁、原文残留、版面失衡或商用评分不达标时，任务会以 `low_quality` 返回，不应推荐保存到商品图片或设为主图/详情图。

## 文件

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/files/upload` | 上传文件。 |
| `GET` | `/api/v1/files` | 文件列表。 |
| `DELETE` | `/api/v1/files/:id` | 删除文件。 |

## 商品

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/products` | 商品草稿列表；支持 `operationStep`（`collect_review` / `title` / `description` / `images` / `pricing` / `publish_check` / `ready`）筛选，并在列表行返回轻量 `operationProgress` 摘要。 |
| `POST` | `/api/v1/products` | 创建商品草稿。 |
| `GET` | `/api/v1/products/:id` | 商品详情。 |
| `GET` | `/api/v1/products/:id/operation-progress` | 商品运营进度摘要；只读聚合商品、图片、SKU 与既有发布前检查，不调用平台 API、不创建任务、不修改商品。 |
| `PUT` | `/api/v1/products/:id` | 更新商品草稿。 |
| `DELETE` | `/api/v1/products/:id` | 删除或归档商品。 |
| `POST` | `/api/v1/products/:id/skus` | 创建 SKU 元数据；需 `product.write`。JSON 可含 `skuCode`、`skuName`、`attrs`、价格字段与 `imageUrl`，不得含 `stock`；新 SKU 的兼容聚合库存固定从 `0` 开始。 |
| `PUT` | `/api/v1/products/:id/skus/:skuId` | 更新 SKU 元数据；需 `product.write`，不得通过 `stock` 修改库存。 |
| `PUT` | `/api/v1/products/:id/skus/:skuId/stock-settings` | 更新 `warningStock` 与 `safetyStock`；需 `product.write`，不修改在手库存。 |
| `DELETE` | `/api/v1/products/:id/skus/:skuId` | 删除 SKU；需 `product.write`。 |
| `GET` | `/api/v1/product-skus/search` | 已认证的本地 SKU 搜索；仅返回可信认证上下文所属 Tenant 的 SKU。Query 保持 `keyword`、`productId`、`limit`（默认 20、最大 50），响应保持 `data.list`。 |
| `POST` | `/api/v1/products/:id/apply-ai-title` | 应用 AI 标题；body 支持 `aiTitle`、`taskId`、`expectedUpdatedAt`、`sourceSnapshotHash`，冲突时返回 `AI_CONTENT_APPLY_CONFLICT`，不会静默覆盖人工修改。 |
| `POST` | `/api/v1/products/:id/undo-ai-title` | 安全撤销最近一次 AI 标题应用；若应用后字段又被人工修改，返回 `AI_CONTENT_UNDO_CONFLICT`。 |
| `POST` | `/api/v1/products/:id/apply-ai-description` | 应用 AI 描述；body 支持 `aiDescription`、`taskId`、`expectedUpdatedAt`、`sourceSnapshotHash`，冲突时返回 `AI_CONTENT_APPLY_CONFLICT`。 |
| `POST` | `/api/v1/products/:id/undo-ai-description` | 安全撤销最近一次 AI 描述应用；若应用后字段又被人工修改，返回 `AI_CONTENT_UNDO_CONFLICT`。 |

SKU 元数据写接口只允许访问当前租户可见商品；跨租户商品统一按 `404` 处理。`POST` / `PUT .../skus` 一旦收到 `stock` 即返回 `400`，库存调整必须改用需 `inventory.operate` 的分仓接口 `POST /api/v1/products/:id/skus/:skuId/adjust-stock`。手工新建 SKU 不隐式创建历史库存事实；历史导入数据继续通过有界库存账迁移接口处理。

### 本地 SKU 搜索安全合同

`GET /api/v1/product-skus/search` 必须经过认证，并从可信认证上下文取得正数 `TenantID` 与有效、启用的 Tenant Membership。普通列表、关键词、`productId`、排序和 `limit` 窗口均强制按 `products.tenant_id` 隔离；缺少认证/可信 Tenant 时返回 `401 authentication_required`，Membership 无效或不匹配时返回 `403 permission_denied`，且不会执行 SKU 搜索 SQL。

- `keyword` 仅搜索 SKU Code、SKU Name 和 Product Title；`productId` 只在当前 Tenant 内匹配。
- 跨 Tenant `productId` 返回相同成功 Envelope 下的空 `list`，不泄露 Product 是否存在。
- `tenantId`、`tenant_id` 及其他客户端 Tenant 选择字段在当前 Query 解析方式下被忽略（若未来改为严格绑定也可拒绝），绝不参与数据范围。
- 现有 API 不包含 Barcode/Status 搜索、Count、Offset/Keyset Pagination 或分页元数据；本次安全修复不扩展这些合同。
- 响应 DTO、字段名、字段类型、`data.list` 结构及 Request ID/Trace ID 行为保持不变。

**批量 AI 文案（Phase A3.1）**

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/products/ai-text/batches/check` | 创建前检查；返回 `summary` + 每商品×类型 `items`（`ready` / `warning` / `blocked`）。 |
| `POST` | `/api/v1/products/ai-text/batches` | 创建批次；支持 `operationTypes`: `title` / `description`；幂等键 `idempotencyKey`；**不自动应用**。 |
| `GET` | `/api/v1/products/ai-text/batches` | 批次列表。 |
| `GET` | `/api/v1/products/ai-text/batches/:id` | 批次详情 + 复核子项；query `status` 筛选。 |
| `POST` | `/api/v1/products/ai-text/batches/:id/retry-failed` | 重试失败、pending、running 子项（含服务重启后的孤儿项）。 |
| `POST` | `/api/v1/products/ai-text/batches/:id/cancel-pending` | 取消 pending 子项。 |
| `POST` | `/api/v1/products/ai-text/batches/:id/apply-selected` | 批量应用；body `itemIds[]`；逐条冲突保护，`partial_success`。 |
| `POST` | `/api/v1/products/ai-text/batches/:id/undo-applied` | 撤销本批次已应用项。 |
| `POST` | `/api/v1/products/ai-text/items/:id/regenerate` | 单条重新生成。 |
| `POST` | `/api/v1/products/ai-text/items/:id/update-edited-text` | 保存编辑文案。 |
| `POST` | `/api/v1/products/ai-text/items/:id/apply` | 单条应用；冲突 409 + `AI_CONTENT_APPLY_CONFLICT`。 |
| `POST` | `/api/v1/products/ai-text/items/:id/reject` | 放弃建议。 |

设计见 [`BATCH_AI_TEXT_OPERATION_DESIGN.md`](BATCH_AI_TEXT_OPERATION_DESIGN.md)。

### 批量 AI 图片（Phase A3.2）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/products/ai-images/batches/check` | 创建前检查；body 含 `productIds`、`imageIds`、`operationTypes`；返回每图×处理方式 `items`。 |
| `POST` | `/api/v1/products/ai-images/batches` | 创建批次；**不自动应用**；幂等键 `idempotencyKey`。 |
| `GET` | `/api/v1/products/ai-images/batches` | 批次列表。 |
| `GET` | `/api/v1/products/ai-images/batches/:id` | 批次详情 + 复核子项。 |
| `POST` | `/api/v1/products/ai-images/batches/:id/retry-failed` | 重试失败 / pending / running 子项。 |
| `POST` | `/api/v1/products/ai-images/batches/:id/cancel-pending` | 取消 pending 子项。 |
| `POST` | `/api/v1/products/ai-images/batches/:id/apply-selected` | 批量应用；body `itemIds[]`、`applyMode`。 |
| `POST` | `/api/v1/products/ai-images/batches/:id/undo-applied` | 撤销本批次已应用项。 |
| `POST` | `/api/v1/products/ai-images/items/:id/regenerate` | 单条重新处理。 |
| `POST` | `/api/v1/products/ai-images/items/:id/apply` | 单条应用；body `applyMode`；冲突 409。 |
| `POST` | `/api/v1/products/ai-images/items/:id/reject` | 放弃结果。 |

`operationTypes`：`quality_check` / `remove_watermark` / `remove_logo` / `white_background` / `optimize_background` / `translate_text` / `select_best_main`。设计见 [`BATCH_AI_IMAGE_OPERATION_DESIGN.md`](BATCH_AI_IMAGE_OPERATION_DESIGN.md)。

| `POST` | `/api/v1/products/:id/images/select-best-main` | 自动评分并选择最佳主图；JSON `mode`: `score_only` / `recommend` / `auto_set`。 |
| `POST` | `/api/v1/products/:id/sync-images` | 将商品外链图片（如淘宝 alicdn）下载并保存到当前 Storage Provider；JSON `scope`: `all` / `main` / `detail`（默认 `all`）。 |
| `POST` | `/api/v1/pricing/calculate` | 单 SKU 发布价试算（不写入数据库）。 |
| `POST` | `/api/v1/products/:id/pricing/apply` | 对商品 SKU 应用定价规则；`confirm=false` 仅预览，`confirm=true` 更新 `product_skus.price`。 |
| `POST` | `/api/v1/products/pricing/batch-apply` | 批量应用定价规则；需 `productIds` 或 `filters`，空条件须 `confirmAll=true`。 |

`GET /api/v1/products/:id` 商品详情会返回统一商品草稿视图：基础字段 `source`、`sourceUrl`、`title`、`originalTitle`、`aiTitle`、`description`、`aiDescription`、`currency`、`status`；图片字段 `mainImages`、`descriptionImages`；结构字段 `attributes`、`skuGroups`、`skus`；价格 / 库存聚合字段 `costPrice`、`salePrice`、`stock`；采集与发布字段 `collectWarnings`、`publishStatus`；高级调试字段 `raw` / `rawData`。前端普通视图只展示标准字段与 warning，`raw` 仅用于高级详情。

`operationProgress` 统一使用实际数据实时计算：采集结果、标题、描述、图片、价格、通用参数、发布检查、刊登草稿准备。返回字段包括 `completionPercent`、`currentStep`、`currentStepLabel`、`nextActionLabel`、`nextActionKey`、`nextActionUrl`、`completedSteps`、`pendingSteps`、`blockers`、`warnings`、`publishReady`、`updatedAt`。列表摘要只返回完成度、当前步骤、下一步入口、阻断/建议数量和可刊登状态；列表聚合批量读取图片、SKU 与图片任务状态，禁止逐行调用平台或自动创建任务。

`pricing.rule` 支持：`costSource`（`collected` / `manual`）、`manualCostPrice`、`markupType`（`fixed` / `percent` / `multiplier` / `none`）、`markupAmount`、`markupPercent`、`markupMultiplier`、`shippingCost`、`weight`、`shippingCostPerWeight`、`platformCommissionPercent`、`exchangeRate`、`minProfit`、`minMarginPercent`、`minPublishPrice`、`roundingMode`（`none` / `integer` / `.9` / `.95` / `.99` / `9.99` / `19.90`）。试算返回 `landedCost`、`commissionFee`、`estimatedProfit`、`profitMarginPercent`；应用后写入 `product_skus.price` 并写操作日志。

`settings` 分组 **`pricing`**：默认加价方式/比例/倍率、固定运费、按重量运费单价（预留）、平台佣金、最低利润、最低利润率、汇率、尾数、平台覆盖、`batch_max_size`（默认 500）。**不**创建刊登任务、**不**调用平台 API。

发布前检查 `GET /api/v1/products/:id/readiness` 返回兼容字段 `status=ready|warning|blocked`，并新增 `result=passed|warning|failed`，以及用户可见 `statusLabel` / `resultLabel`。每个 `checks[]` 项含 `title`、`message`、`severity`（同 `level`）与 `technicalDetails.rawCode`（内部码，前端默认折叠）。`failed` 阻止创建刊登任务；`warning` 可继续但前端必须人工确认。采集 warning 码（如 `DETAIL_IMAGES_INCOMPLETE`）在后端统一中文化。

**多平台刊登中心（Phase A1.2）**

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/products/:id/publish-targets` | 可刊登平台、店铺与能力分级（`real_draft_create` / `local_draft_only` / …） |
| `POST` | `/api/v1/products/:id/publish-targets/check` | 多目标独立预检查；body 含 `targets[]`、`commonConfig`、`targetConfigs` |
| `POST` | `/api/v1/products/:id/publish-targets/create-drafts` | 批量创建非抖店刊登草稿；请求只要包含 `douyin_shop` 就在幂等/批次/任务写入前整单拒绝，抖店必须进入运营任务中心。 |
| `GET` | `/api/v1/product-publish/targets` | 全局可刊登平台与店铺（批量向导） |
| `POST` | `/api/v1/product-publish/batch-targets/check` | 多商品 × 多目标矩阵预检查；body 含 `productIds[]`、`targets[]`、`commonConfig`、`overrides` |
| `POST` | `/api/v1/product-publish/batch-targets/create-drafts` | 多商品批量创建非抖店刊登草稿；请求只要包含 `douyin_shop` 就在任何本地写入前整单拒绝。 |
| `GET` | `/api/v1/product-publish/batches` | 多商品刊登批次列表 |
| `GET` | `/api/v1/product-publish/batches/:id` | 批次详情与子任务（仅创建者可访问，历史无 `createdBy` 批次兼容） |
| `POST` | `/api/v1/product-publish/batches/:id/retry-failed` | 只重试失败的非抖店子任务；批次含抖店任务时整批拒绝且不修改任务状态。每个旧任务的认领与替代任务创建在同一事务中，替代失败时旧任务保持 `failed`。 |
| `POST` | `/api/v1/product-publish/batches/:id/cancel-pending` | 只取消 pending 子任务；任务状态、批次计数与批次状态在同一事务中更新。 |

**批量规模限制（Phase A2.1）**：环境变量 `PUBLISH_BATCH_MAX_PRODUCTS`（默认 100）、`PUBLISH_BATCH_MAX_TARGETS`（默认 20）、`PUBLISH_BATCH_MAX_TASKS`（默认 300，即商品数 × 目标数）。超限时 HTTP 400，message：`本次选择的商品和刊登目标较多，请分批创建刊登草稿。`

**权限与隔离**：采集/刊登读取需要 `product.view`，采集写入需要 `product.write`，创建刊登草稿需要 `publish.create_draft`，任务重试需要 `task.retry`，刊登 SKU 同步/绑定/解绑需要 `sku.bind`。商品、采集任务、刊登任务、publication 和批次均按当前 tenant 查询；店铺目标还要求对应店铺查看或操作授权，越权资源按未找到处理。

**幂等与原子性**：`create-drafts` 对相同 tenant + admin + 商品 + 目标 + 配置 hash 返回已有活跃批次；任务级 dedup 按 `tenant + product + platform + shop + config hash` 跳过已成功项。本地 publication、刊登任务和反向关联在同一事务中创建，多商品批次与其本地草稿也整体提交或回滚。

**配置校验（Phase A2.2）**：`batch-targets/check` 与 `create-drafts` 校验 `commonConfig` / `overrides`（数值非负、策略枚举、商品 / 平台 / 店铺越权与匹配）。失败时 HTTP 400，`code=40004`（`PUBLISH_CONFIG_INVALID`），`data` 含 `title`、`message`、`technicalDetails.field`。

**`commonConfig` 结构**：嵌套 `price` / `image` / `inventory` / `package` + `remark`（详见 [`MULTI_PLATFORM_PUBLISHING_DESIGN.md`](MULTI_PLATFORM_PUBLISHING_DESIGN.md) §A2.2）。

**`overrides` 结构**：`products`、`platforms`、`shops`、`productTargets` 四层局部覆盖；合并优先级见设计文档。

**数据库**：显式 migration 见 [`docs/PUBLISH_BATCH_MIGRATION.md`](PUBLISH_BATCH_MIGRATION.md)。

详见 [`docs/MULTI_PLATFORM_PUBLISHING_DESIGN.md`](MULTI_PLATFORM_PUBLISHING_DESIGN.md)。

刊登任务 `POST /api/v1/products/:id/publish` 只为非抖店平台保存 `product_publish_tasks`，并在 `APP_ENV=staging|production` 固定拒绝传统直发，返回 `TRADITIONAL_PUBLISH_PRODUCTION_DISABLED`。`douyin_shop` 在任务写入前固定拒绝并返回 `DOUYIN_OPERATION_TASK_REQUIRED`。非抖店任务字段包括 `productId`、`targetPlatform`、`targetStoreId`、`status`（队列态，兼容旧值）、`publishStatus`（业务态：`draft` / `checking` / `ready` / `publishing` / `success` / `failed` / `cancelled`）、`publishMode`、`title`、`description`、`images`、`skus`、`price`、`currency`、`checkResult`、`platformPayload`、`platformResult`、`errorCode`、`errorMessage`、`createdAt`、`updatedAt`。平台字段映射快照包含 `platformTitle`、`platformDescription`、`platformImages`、`platformSkus`、`platformPrice`、`platformStock`、`platformCategory`、`platformAttributes`。

## AI

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/ai/title-optimize` | AI 标题优化（同步/任务，见实现）。 |
| `POST` | `/api/v1/ai/description-generate` | AI 描述生成。 |
| `GET` | `/api/v1/ai/tasks` | AI 任务列表。 |
| `GET` | `/api/v1/ai/tasks/:id` | AI 任务详情。 |

### 客服与 AI 回复

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/customer/dashboard` | 客服中心汇总。 |
| `POST` | `/api/v1/customer/conversations/:id/ai/generate-reply` | 生成 AI 回复建议，不直接外发。 |
| `POST` | `/api/v1/customer/conversations/:id/send-platform-message` | 人工确认后外发；请求必须包含 `reply` 和唯一 `clientMessageId`，可带 `suggestionId`。 |
| `GET` | `/api/v1/customer/shops/:shopId/auto-reply-policy` | 查询店铺自动回复策略及部署总开关状态。 |
| `PUT` | `/api/v1/customer/shops/:shopId/auto-reply-policy` | 管理员显式更新店铺策略；启用时 `lowRiskOnly` 必须为 `true`。 |
| `GET` | `/api/v1/customer/shops/:shopId/auto-reply-runs` | 查询店铺最近 50 条自动回复处理记录。 |

自动回复默认关闭。`GET/PUT /api/v1/customer/auto-reply-setting` 管理租户级消息同步开关、自动回复总开关和 15–3600 秒轮询间隔；设置持久化到数据库并动态生效。只有两个总开关与店铺策略同时开启才生效。响应中的 `workerAvailable` 仅在 Redis 可探活、客服消息同步 Worker、自动回复轮询调度器和自动回复消费者均运行时为 `true`。轮询器通过数据库 `next_poll_at` 原子认领已启用店铺并创建增量同步任务，单店只允许一个 `pending/running` 任务。每条入站消息只创建一个幂等运行记录；Redis ready/processing 队列可恢复未确认任务。外发前会在会话锁内重新检查最新客户消息、人工回复、会话状态与频率限制。平台发送幂等租约在调用期间持续续期；平台成功并完成本地消息落库后，即使幂等元数据收尾失败也只重放本地消息，不会再次调用平台。`generating` 租约过期可恢复，`sending` 过期或平台发送结果未知则转 `human_required/platform_send_result_unknown`，写入统一失败任务中心且不自动重发。最近运行记录可选返回截断后的 `errorMessage`。

## Dev / Demo 种子（非 production）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/dev/demo-seed/full-project-edge-cases` | **仅 dev/demo 环境**；需 **admin** 权限。写入订单 partial_success、库存同步失败、客服发送失败等样本；不调用真实外部平台。production 禁用。 |

## 采集

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/collect/tasks` | 创建采集任务。`source=custom` 时若 URL 属于已有 **available/beta** 专用采集器域名，返回业务码 **40002**，`data.errorCode=CUSTOM_COLLECT_PROVIDER_CONFLICT`，含 `recommendedProvider` 与 `message`。 |
| `GET` | `/api/v1/collect/tasks` | 采集任务列表。 |
| `GET` | `/api/v1/collect/tasks/:id` | 采集任务详情。 |
| `POST` | `/api/v1/collect/tasks/:id/retry` | 重试采集任务。 |
| `POST` | `/api/v1/collect/rules/ai-generate` | AI 根据商品 URL 生成自定义采集规则（分析页面摘要 → AI → 校验 → 自动规则测试）。1688 / AliExpress 等 **available/beta** 专用平台返回 **40002**。规则非法返回 **40003** `AI_RULE_INVALID`。 |
| `POST` | `/api/v1/collect/rules/ai-generate-and-save` | 同上并直接保存为 `collect_rule`。 |
| `GET` | `/api/collector/providers/1688/auth-status` | 1688 采集浏览器登录态检测（同 `/api/v1/collector/...`）。 |
| `POST` | `/api/collector/providers/1688/open-login-browser` | 打开持久化 Playwright 浏览器供 1688 手动登录。 |
| `GET` | `/api/collector/providers/pinduoduo/auth-status` | 拼多多登录态检测（兼容 GET；内部走 check-login 逻辑）。 |
| `POST` | `/api/v1/collect/providers/pinduoduo/check-login` | 拼多多登录态检测（推荐）。body 可选 `{ "url": "商品详情链接", "testUrl": "设置页检测链接" }`；检测优先级：body.url → 最近失败任务 URL → 设置 `collect_pinduoduo_auth_check_url` → 仅 pifa 首页（`homepage_only`）。 |
| `POST` | `/api/collector/providers/pinduoduo/check-login` | 同上（`/api/collector` 别名）。 |
| `POST` | `/api/collector/providers/pinduoduo/open-login-browser` | 打开拼多多采集浏览器手动登录；body 可选 `{ "url": "商品或 pifa 链接" }`（勿传无参 `mobile.yangkeduo.com` 首页）。 |
| `POST` | `/api/v1/collect/providers/taobao_tmall/check-login` | 淘宝/天猫登录态检测（批量采集开始前也会调用）。body 可选 `{ "url": "商品详情链接", "testUrl": "设置页检测链接" }`；未登录返回业务错误文案；需安全验证时阻止批量开始。 |
| `POST` | `/api/collector/providers/taobao_tmall/check-login` | 同上（`/api/collector` 别名）。 |
| `POST` | `/api/collector/providers/taobao_tmall/open-login-browser` | 打开淘宝/天猫采集浏览器手动登录；body 可选 `{ "url": "商品链接" }`。 |

以上是 backend 鉴权代理接口。Collector 原生 `/v1/*` 不对浏览器或公网开放，除 `/health` 外均要求 backend 使用 `COLLECTOR_SERVICE_TOKEN` Bearer 鉴权；完整 Compose 不发布 Collector 宿主机端口。采集 URL 仅允许公网 HTTP/HTTPS，解析到私网、回环、链路本地、保留地址或在浏览器阶段转向受限地址时拒绝。

`GET /api/collector/providers/1688/auth-status` 返回示例：

```json
{
  "provider": "1688",
  "status": "ok",
  "loggedIn": true,
  "needVerification": false,
  "message": "1688 登录态正常",
  "lastCheckedAt": "2026-05-20T12:00:00.000Z",
  "profilePath": "/path/to/collector/data/browser-profiles/1688"
}
```

`status` 取值：`ok`（已登录）、`not_logged_in`（需要登录）、`wechat_auth_required`（微信扫码）、`app_redirect`（App 引导页）、`verification_required`（需验证）、`homepage_only`（仅首页可访问，无法确认登录）、`unknown`（暂时无法确认）。

拼多多 `check-login` 返回扩展字段（无 Cookie/HTML）：`profileKey`（`pinduoduo`）、`checkedUrl`、`finalUrl`、`accessStatus`、`urlType`（`wholesale_detail` | `goods_detail` | `homepage` | `app_redirect` | `unknown`）、`checkMode`、`evidence`（`hasProductTitle` / `hasPrice` / `hasMainImage` 等）。**仅当打开商品详情页且识别到标题/价格/主图之一，且无登录/微信/App 引导时** 才返回 `ok`；**pifa 首页可访问不判已登录**。

`POST open-login-browser` 与 `check-login` 使用同一 **`pinduoduo` Profile**（与 1688、custom 隔离）。采集浏览器登录窗口 **1280×900**。

## 店铺与平台

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/shops` | 店铺列表（现行路径；legacy `/stores` 已废弃）。 |
| `GET` | `/api/v1/shops/:id` | 店铺详情。 |
| `POST` | `/api/v1/shops/:id/sync-orders` | 手动触发订单同步。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/refresh` | 刷新抖店授权 Token（示例；各平台 OAuth 见下表）。 |

现行平台 Provider 与开放平台应用配置接口：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/platform/providers` | 返回已注册平台 Provider、能力、状态、`appConfigSchema` 与设置分组。`douyin_shop` 已注册为抖店 / Douyin Shop Provider。 |
| `GET` | `/api/v1/platform/settings/:platform` | 读取平台开放应用配置 schema 与脱敏后的当前值。敏感字段只返回 `****`。 |
| `PUT` | `/api/v1/platform/settings/:platform` | 保存平台开放应用配置。敏感字段加密存储，传入 `****` 表示保留原值。`douyin_shop` 会校验 App Key、App Secret、回调地址、环境与超时时间；发起 OAuth 还需要 `service_id`。 |
| `POST` | `/api/v1/platform/settings/:platform/test-connection` | 测试已保存的平台开放应用配置。`douyin_shop` 应用配置测试校验配置完整性与授权可用性，不做商品 / 订单 / 库存调用。 |
| `GET` | `/api/v1/shops/oauth/douyin/start` | 发起抖店 OAuth；生成 Redis state（10 分钟，绑定管理员、`platform=douyin_shop`、可选 `shopId`），返回 `redirectUrl`。 |
| `GET` | `/api/v1/shops/oauth/douyin/callback` | 抖店授权公开回调；校验 state，处理 `code` / `error`，换取 token，创建或更新 `shops` / `shop_auth_tokens`，成功跳转 `/settings/platforms?platform=douyin_shop&auth=success`。 |
| `GET` | `/api/v1/shops/:id/oauth/douyin/authorize-url` | 已有抖店店铺重新授权，返回 `redirectUrl`。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/refresh` | 使用加密保存的 refresh token 刷新抖店 access token，并用刷新响应校准店铺基础信息；失败时按场景标记 `expired` / `invalid`。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/revoke` | 本地解除抖店授权，清理 / 失效 token，保留历史数据。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/test` | 真实测试抖店店铺连接：检查授权、必要时刷新 token、读取并校准店铺基础信息；不返回 token 明文。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/sync-shop-info` | 手动同步 / 校准抖店店铺基础信息，复用 Phase 3 OpenAPI Client 与 token 自动刷新能力。 |
| `GET` | `/api/v1/platform/douyin/categories` | 读取本地缓存的抖店类目树；支持 `keyword`、`parentId`、`onlyLeaf`、`refresh=false`、`shopId`（仅 `refresh=true` 时用于手动刷新）。 |
| `POST` | `/api/v1/platform/douyin/categories/sync` | 使用已授权抖店店铺 token 同步类目缓存，body/query 传 `shopId`；写入 `platform_categories`，幂等 upsert。 |
| `GET` | `/api/v1/platform/douyin/categories/stats` | 返回抖店类目缓存数量、叶子类目数量和最近同步时间，供平台开放配置页展示。 |
| `GET` | `/api/v1/platform/douyin/categories/:categoryId/attributes` | 读取某个抖店类目的本地属性缓存；返回必填、可选项、属性值选项和同步时间，不返回 raw。 |
| `POST` | `/api/v1/platform/douyin/categories/:categoryId/attributes/sync` | 使用已授权抖店店铺 token 刷新某个叶子类目的属性缓存，body/query 传 `shopId`；写入 `platform_category_attributes`，幂等 upsert。 |
| `POST` | `/api/v1/platform/douyin/production-preflight` | 抖店上线前生产预检（配置、授权、开关、Storage 公网、数据状态）；body 可选 `{ "liveTest": true }` 对首家已授权店铺做 Token 刷新联调。 |
| `GET` | `/api/v1/platform/douyin/production-preflight/latest` | 读取最近一次预检结果（存于 settings `douyin_preflight.latest_result`）。 |
| `GET` | `/api/v1/platform/douyin/runtime-status` | 读取抖店运行状态（`normal` / `paused` / `emergency_disabled`）、原因与变更时间。 |
| `POST` | `/api/v1/platform/douyin/runtime-status/pause` | 暂停抖店任务；body `{ "reason": "..." }` 必填；记录 `douyin.platform.pause` 操作日志。 |
| `POST` | `/api/v1/platform/douyin/runtime-status/resume` | 恢复抖店运行；body `{ "reason": "..." }` 必填。 |
| `POST` | `/api/v1/platform/douyin/runtime-status/emergency-disable` | 紧急停用；阻止 Worker 调用抖店写接口；body `{ "reason": "..." }` 必填。 |
| `GET` | `/api/v1/products/:id/platform-configs/:platform` | 读取商品的平台刊登准备配置；`douyin_shop` 返回 `shopId`、`categoryId`、`categoryPath`、`platformAttributes`，以及已保存的 `mapping` / `lastMappedAt`。 |
| `PUT` | `/api/v1/products/:id/platform-configs/:platform` | 保存商品的平台刊登准备配置；`douyin_shop` 会校验类目必须为本地缓存中的叶子类目，并记录抖店类目/属性操作日志。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/build-mapping` | 根据当前商品草稿、抖店店铺/类目/属性配置生成并保存抖店刊登草稿预览；不调用抖店创建商品或图片上传接口。 |
| `GET` | `/api/v1/products/:id/platform-configs/douyin_shop/mapping` | 读取已保存的抖店刊登草稿映射。 |
| `PUT` | `/api/v1/products/:id/platform-configs/douyin_shop/mapping` | 保存人工调整后的抖店刊登草稿映射。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/validate` | 校验抖店刊登草稿映射；可传入临时映射 body，也可不传 body 校验已保存映射。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/images/upload` | 上传当前抖店刊登草稿中的待上传图片到抖店素材中心。body：`imageTypes`（`main` / `detail`）、`retryFailed`、`force`。外链会先下载并写入当前 Storage Provider，再通过后端 Douyin Client 上传；不创建抖店商品。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/images/:imageKey/retry` | 重试单张抖店图片上传。`imageKey` 可用 `localImageId`、`main:0` / `detail:0`、`storageKey` 或已有 `platformImageId`。 |
| `GET` | `/api/v1/products/:id/platform-configs/douyin_shop/images/status` | 读取当前抖店图片上传状态、Storage 状态、平台图片 ID / URL、失败原因和统计。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/create-draft` | 兼容保留但不再创建草稿。固定返回 HTTP 409，`data.errorCode=DOUYIN_OPERATION_TASK_REQUIRED`，且不写 publication、刊登任务或幂等记录。 |
| `GET` | `/api/v1/products/:id/platform-configs/douyin_shop/publish-tasks` | 列出当前商品的抖店刊登任务（分页）。 |
| `POST` | `/api/v1/product-publish/tasks/:id/cancel` | 取消 pending/running 刊登任务。 |

### 运营任务中心与唯一抖店草稿写链

所有运营任务写请求必须携带 `Idempotency-Key`，并使用登录管理员的租户上下文与细分 RBAC 权限。抖店生产任务的创建 payload 是严格意图：`schemaVersion=douyin_draft_v1`、`productId`、`shopId`、`publishMode=save_as_platform_draft`。服务在只读事务中冻结商品、SKU、平台请求和映射并计算 canonical JSON SHA-256；冻结内容提交人工审核后才允许执行，后续实时商品/映射变化不会替换已批准快照。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/operation-tasks` | 创建运营任务；真实抖店商品任务会同步生成不可编辑的冻结平台草稿，等待人工审核，不自动执行。 |
| `GET` | `/api/v1/operation-tasks` | 按 `status`、`platform`、`taskType`、签名游标分页查询。 |
| `GET` | `/api/v1/operation-tasks/:taskId` | 返回任务、最新草稿/审批/尝试和基于 RBAC/状态机计算的 `allowedActions`。 |
| `POST` | `/api/v1/operation-tasks/:taskId/cancel` | 按 revision 取消未进入终态的任务。 |
| `POST` / `PATCH` | `/api/v1/operation-tasks/:taskId/drafts` / `drafts/latest` | 本地运营任务草稿版本操作；生产抖店冻结草稿固定拒绝编辑。 |
| `GET` | `/api/v1/operation-tasks/:taskId/drafts` | 读取版本摘要；生产草稿只暴露审核视图，不返回被冻结的 Provider 请求或敏感配置。 |
| `POST` | `/api/v1/operation-tasks/:taskId/approve` / `reject` | 人工审核指定 `draftVersion + draftPayloadHash`，按 task revision 防并发漂移。 |
| `POST` | `/api/v1/operation-tasks/:taskId/execute` | 只接受已审核生产草稿的 `adapterMode=production_draft`；事务内创建 execution attempt、下游刊登任务与 outbox。 |
| `POST` | `/api/v1/operation-tasks/:taskId/retry` | 仅允许明确 `retryable=true` 的已知失败；`result_unknown` 不可重试。 |
| `GET` | `/api/v1/operation-tasks/:taskId/attempts` / `events` | 查询执行尝试与审计时间线。 |
| `POST` | `/api/v1/product-publish/tasks/:id/recover-douyin-draft` | 需要 `operationtask.execute` 与店铺操作权限；仅下游任务、执行尝试和运营任务均为 `result_unknown` 时人工触发只读 `product.detail` 对账，否则固定 409 `DOUYIN_RECOVERY_NOT_ALLOWED`。不会重新创建；确认同一 `outer_product_id` 已存在时收敛为成功，否则保持不可重试的待核对状态。别名路径为 `/:id/douyin/recover`。 |

生产调用前 Worker 会重复验证运营任务/草稿/审批/尝试/下游任务绑定、冻结请求与映射副本 Hash、商品/店铺/SKU 数、L3 开关、已授权单店、单租户白名单、active 灰度、Owner 与 Technical Lead 两名不同管理员审批，以及 provider/tenant/shop/write kill switch。`result_unknown` 禁止自动重建。平台成功后的刊登任务、publication 与 SKU 更新在同一数据库事务内完成，运营任务终态可由待交付 outbox/消费后对账重放。

L3 只代表上述 `save_as_platform_draft` 能力；不包含正式发布、上架、库存写入、自动业务重试、无审核执行或多店扩容。默认 L0 和所有真实能力关闭。旧直接创建、传统 publish、多目标/批量创建、刊登任务重试和批次重试均不得成为抖店旁路，拒绝时不产生本地写入或状态修改。

### P10 生产控制面

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/p10/status` | 返回环境、能力开关、五级 kill switch、白名单、灰度、首版范围和 `productionReady`；状态不等于发布审批。 |
| `PUT` | `/api/v1/p10/controls/kill-switches` | 带 `expectedRevision` 更新 provider/tenant/shop/read/write kill switch；缺失控制行默认全部阻断。 |
| `PUT` | `/api/v1/p10/controls/allowlist` | 维护单租户、单店白名单；`enabled=true` 在数据库层全局最多一条。 |
| `PUT` | `/api/v1/p10/gray` | 保存单店灰度与 `maxSku<=100`，修改范围会重置审批。 |
| `POST` | `/api/v1/p10/gray/approve` | 由两名不同全局管理员分别以 `owner` 和 `technical_lead` 职责审批同一 revision。岗位真实性由发布工单核验。 |
| `POST` | `/api/v1/p10/gray/activate` / `pause` / `stop` | 激活、暂停或停止灰度；所有操作带 revision 并写安全审计。 |

配置默认 L0。L3 启动必须同时满足真实 Provider/网络/凭据/草稿写/Worker 开关、`PRODUCT_PUBLISH_QUEUE_ENABLED=true`、`WORKER_REAPER_ENABLED=true`，且自动业务重试和库存 mutation 关闭。CI、真实凭据工单、备份/恢复/回滚、灰度与人工验收未完成前，不得将 API 可用描述为已上线。

抖店 SKU 绑定校准与手动兜底（Phase 9.1 / 9.2，`product_publications.id` 或 `product_publication_skus.id` 为路径参数）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/product-publications/:id/douyin/sku-bindings` | 读取当前 `product_publication_skus` 绑定状态汇总（`bound` / `skipped` / `unmatched` / `ambiguous` / `failed` 计数与行明细）；含 `platformSkus` 平台候选、`inventorySyncReady` / `inventorySyncBlockReason`。 |
| `POST` | `/api/v1/product-publications/:id/douyin/sync-sku-bindings` | 调用官方 `product.detail`（`show_draft=true`）拉取抖店 SKU 列表并校准本地映射，回写 `external_sku_id`、`bindStatus`、`bindConfidence`、`bindMessage`、`lastSyncedAt`；更新 `product_publications.skuBindingSyncedAt` 与 `raw_data.platformSkus` 缓存。已绑定 SKU 跳过；多候选标记 `ambiguous` 不强行绑定。 |
| `POST` | `/api/v1/product-publication-skus/:id/douyin/bind-sku` | 人工绑定抖店 SKU。body：`platformSkuId`（必填）、`platformSkuName`、`bindReason`（如 `manual`）。校验 publication 归属 `douyin_shop`、平台商品 ID 存在、SKU ID 非空、不与其他本地规格冲突；覆盖旧绑定时记录操作日志。成功后 `bindStatus=bound`、`bindConfidence=100`、`bindMessage=手动绑定`。 |
| `POST` | `/api/v1/product-publication-skus/:id/douyin/unbind-sku` | 解除绑定。body：`reason`（如 `manual_unbind`）。清空 `external_sku_id`，`bindStatus=unmatched`、`bindMessage=已手动解除绑定`。 |

错误码：`DOUYIN_PRODUCT_DETAIL_FAILED`、`DOUYIN_PRODUCT_NOT_FOUND`、`DOUYIN_PRODUCT_DETAIL_PERMISSION_DENIED`、`DOUYIN_SKU_BINDING_SYNC_FAILED`、`DOUYIN_SKU_BINDING_UNMATCHED`、`DOUYIN_SKU_BINDING_AMBIGUOUS`、`DOUYIN_SKU_MANUAL_BIND_FAILED`、`DOUYIN_SKU_MANUAL_UNBIND_FAILED`、`DOUYIN_PLATFORM_SKU_ID_MISSING`、`DOUYIN_SKU_BINDING_CONFLICT`、`DOUYIN_SKU_BINDING_REQUIRED`。

操作日志：`douyin.sku.binding.manual_bind`、`douyin.sku.binding.manual_unbind`、`douyin.sku.binding.recheck`、`douyin.sku.binding.conflict`（不记录 token / secret）。

抖店库存同步（Phase 9，复用既有 inventory 模块，无新增割裂路径）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/products/:id/publication-skus` | 商品详情库存 Tab 读取刊登 SKU 映射与 `inventorySyncCapability`（`douyin_shop` 为 `beta`）。 |
| `POST` | `/api/v1/product-publication-skus/:id/sync-inventory` | 单 SKU 库存同步；body：`stock`、`options`、`fromInventoryAlert`。要求 `product_publications.external_product_id` 与 `product_publication_skus.external_sku_id` 已绑定。 |
| `POST` | `/api/v1/products/:id/sync-inventory` | 单商品多 SKU 库存同步；body：`shopId`、`skuIds[]`、`options`。 |
| `GET` | `/api/v1/inventory` | 库存中心 SKU 列表（F3）；筛选 stockStatus / skuBindStatus / syncStatus / hasException 等。 |
| `GET` | `/api/v1/inventory/alerts` | 库存预警列表。 |
| `GET` | `/api/v1/inventory/effects` | 订单库存扣减/回滚影响（扣减记录页数据源）。 |
| `GET` | `/api/v1/inventory/logs` | 本地库存变更流水。 |
| `GET` | `/api/v1/inventory-sync/tasks` | 库存同步任务列表。 |
| `GET` | `/api/v1/inventory-sync/tasks/:id` | 任务详情。 |
| `POST` | `/api/v1/inventory-sync/tasks/:id/retry` | 重试 failed 任务。 |
| `POST` | `/api/v1/inventory-sync/batches` | 批量库存同步（默认低并发）。 |

Provider 调用官方 `sku.syncStock`（`incremental=false` 全量更新）；受 `inventory_sync_enabled` 开关控制（默认关闭）。缺失平台 SKU ID 或 `bindStatus=unmatched/failed` 返回 `DOUYIN_SKU_BINDING_REQUIRED`；`bindStatus=ambiguous` 返回 `DOUYIN_SKU_BINDING_AMBIGUOUS`；绑定冲突返回 `DOUYIN_SKU_BINDING_CONFLICT`；不猜测同步。库存同步前须全部 SKU 处于可同步绑定状态（bound / skipped 且已有 `external_sku_id`）。

### P9 Inventory Sync Backend API（Batch 5）

Batch 5 的 fixture/mock-only 后端 API 使用 `/api/v1/inventory-sync`，复用现有认证、租户上下文、RBAC、审计和签名 keyset cursor。所有写请求必须带 `Idempotency-Key`；JSON body 必须为受限 `application/json`，拒绝未知字段和多余 JSON 值。该 API 不接收凭证、不调用真实 Douyin、不读写真实平台库存，也不启动 worker/cron/queue。

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/api/v1/inventory-sync/runs` | `inventory_sync.run` | Create a fixture-backed sync run |
| `GET` | `/api/v1/inventory-sync/runs` | `inventory_sync.read` | Signed keyset run history |
| `GET` | `/api/v1/inventory-sync/runs/:runId` | `inventory_sync.read` | Safe run detail/statistics/error summary |
| `POST` | `/api/v1/inventory-sync/runs/:runId/rerun` | `inventory_sync.rerun` | Guarded retry of a failed/cancelled retryable run |
| `GET` | `/api/v1/inventory-sync/runs/:runId/snapshots` | `inventory_snapshot.read` | Immutable snapshot list and result filter |
| `GET` | `/api/v1/inventory-sync/snapshots/:snapshotId` | `inventory_snapshot.read` | Immutable snapshot detail |
| `GET` | `/api/v1/inventory-sync/bindings` | `sku_binding.read` | Tenant-scoped binding list |
| `GET` | `/api/v1/inventory-sync/bindings/:bindingId` | `sku_binding.read` | Safe binding detail |
| `GET` | `/api/v1/inventory-sync/bindings/:bindingId/history` | `sku_binding.read` | Calibration/manual decision history |
| `GET` | `/api/v1/inventory-sync/snapshots/:snapshotId/calibrations` | `sku_binding.read` | Versioned calibration candidates |
| `POST` | `/api/v1/inventory-sync/snapshots/:snapshotId/recalibrate` | `sku_binding.manage` | Idempotent controlled new calibration version |
| `GET` | `/api/v1/inventory-sync/manual-binding-requests` | `sku_binding.read` | Pending/status manual request list |
| `GET` | `/api/v1/inventory-sync/manual-binding-requests/:requestId` | `sku_binding.read` | Request and immutable decisions |
| `POST` | `/api/v1/inventory-sync/manual-binding-requests/:requestId/confirm` | `sku_binding.resolve_manual` | Revision-checked manual confirmation |
| `POST` | `/api/v1/inventory-sync/manual-binding-requests/:requestId/reject` | `sku_binding.resolve_manual` | Revision-checked manual rejection |
| `GET` | `/api/v1/inventory-sync/runs/:runId/audit-events` | `inventory_sync.audit.read` | Allowlisted tenant-scoped audit timeline |

List endpoints return `{items, nextCursor, hasMore, limit}` and never expose offset/page totals. DTOs intentionally omit raw provider cursors, checkpoints, payloads, credential fields, and idempotency hashes.

通用刊登任务查询接口（抖店写入只能由运营任务中心创建）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/product-publish/tasks` | 刊登任务列表 |
| `GET` | `/api/v1/product-publish/tasks/:id` | 任务详情（含 `platformPayload` 平台提交内容、`platformProductId` 抖店商品 ID、`retryable` 是否可重试） |
| `POST` | `/api/v1/product-publish/tasks/:id/retry` | 仅 development/test 可重试明确可重试的非抖店 failed 任务；staging/production 固定返回 `TRADITIONAL_PUBLISH_PRODUCTION_DISABLED` 且不修改旧任务，抖店固定返回 `DOUYIN_OPERATION_TASK_REQUIRED` 且不修改状态。 |

`product_platform_publish_configs.mapped_images` 在抖店 Phase 6 保存扩展结构：

```json
{
  "mainImages": [
    {
      "localImageId": "",
      "sourceUrl": "",
      "storageUrl": "",
      "storageKey": "",
      "platformImageId": "",
      "platformImageUrl": "",
      "imageType": "main",
      "uploadStatus": "pending|processing|uploaded|failed|skipped",
      "errorCode": "",
      "errorMessage": "",
      "uploadedAt": "",
      "processed": false
    }
  ],
  "detailImages": []
}
```

抖店 OAuth / Client / 类目 / 映射 / 图片错误码：`DOUYIN_APP_CONFIG_INCOMPLETE`、`DOUYIN_OAUTH_STATE_INVALID`、`DOUYIN_OAUTH_DENIED`、`DOUYIN_OAUTH_CODE_MISSING`、`DOUYIN_TOKEN_EXCHANGE_FAILED`、`DOUYIN_TOKEN_REFRESH_FAILED`、`DOUYIN_SHOP_INFO_FAILED`、`DOUYIN_AUTH_EXPIRED`、`DOUYIN_PERMISSION_DENIED`、`UNKNOWN_DOUYIN_AUTH_ERROR`、`DOUYIN_API_ERROR`、`DOUYIN_RATE_LIMITED`、`DOUYIN_REQUEST_TIMEOUT`、`DOUYIN_RESPONSE_PARSE_FAILED`、`UNKNOWN_DOUYIN_ERROR`、`DOUYIN_CATEGORY_SYNC_FAILED`、`DOUYIN_CATEGORY_EMPTY`、`DOUYIN_CATEGORY_NOT_SELECTED`、`DOUYIN_CATEGORY_NOT_LEAF`、`DOUYIN_CATEGORY_ATTR_SYNC_FAILED`、`DOUYIN_REQUIRED_ATTR_MISSING`、`DOUYIN_CATEGORY_CACHE_STALE`、`DOUYIN_CATEGORY_PERMISSION_DENIED`、`DOUYIN_TITLE_MISSING`、`DOUYIN_TITLE_TOO_LONG`、`DOUYIN_DESCRIPTION_MISSING`、`DOUYIN_DESCRIPTION_NEEDS_REVIEW`、`DOUYIN_MAIN_IMAGE_MISSING`、`DOUYIN_MAIN_IMAGE_NOT_UPLOADED`、`DOUYIN_MAIN_IMAGE_UPLOAD_FAILED`、`DOUYIN_DETAIL_IMAGE_UPLOAD_PARTIAL_FAILED`、`DOUYIN_IMAGE_NEED_UPLOAD`、`DOUYIN_IMAGE_UPLOAD_EXPIRED`、`DOUYIN_IMAGE_NEED_SYNC`、`DOUYIN_DETAIL_IMAGE_EMPTY`、`DOUYIN_DETAIL_IMAGE_NEED_SYNC`、`DOUYIN_ATTR_VALUE_INVALID`、`DOUYIN_SKU_MISSING`、`DOUYIN_SKU_PRICE_INVALID`、`DOUYIN_SKU_STOCK_UNCONFIRMED`、`DOUYIN_SKU_ATTR_INCOMPLETE`、`DOUYIN_PRICE_MISSING`、`DOUYIN_PRICE_INVALID`、`DOUYIN_PROFIT_TOO_LOW`、`DOUYIN_STOCK_UNCONFIRMED`、`DOUYIN_STOCK_INVALID`、`DOUYIN_COLLECT_NEEDS_REVIEW`、`IMAGE_URL_NOT_ACCESSIBLE`、`IMAGE_DOWNLOAD_FAILED`、`IMAGE_READ_FAILED`、`IMAGE_FORMAT_UNSUPPORTED`、`IMAGE_SIZE_TOO_LARGE`、`IMAGE_DIMENSION_INVALID`、`IMAGE_PROCESS_FAILED`、`STORAGE_UPLOAD_FAILED`、`DOUYIN_IMAGE_UPLOAD_FAILED`、`DOUYIN_STORE_NOT_AUTHORIZED`、`DOUYIN_CREATE_PRODUCT_FAILED`、`DOUYIN_PRODUCT_PAYLOAD_INVALID`。API 错误响应 `data.errorCode` 返回业务码；callback 失败通过 `reason` query 返回。所有响应均不得返回 App Secret、access token 或 refresh token 明文。

## 抖店可观测性 / Health & Metrics（Phase 10.4）

> **不** 提供 Prometheus `/metrics`。抖店生产监控复用进程健康、任务中心、操作日志与运营看板。真实平台行为按 [`DOUYIN_E2E_CHECKLIST.md`](DOUYIN_E2E_CHECKLIST.md) 人工验收，结论记录在 PR 或发布工单，不向仓库提交测试报告产物。

### 进程健康（含抖店相关队列）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/health` | 匿名；`data.status` 为 `up` / `degraded`；含 `checks.database`、`checks.redis`、`customerMessageSyncQueue` 与 `customerAutoReplyQueue`；自动回复队列块包含 ready/processing 长度、Redis 探活、消息同步 Worker、轮询调度器和自动回复消费者状态 |
| `GET` | `/api/v1/health` | 同上 |

`data` 中与抖店 Worker 相关的块（队列启用时）：

| 字段 | 说明 |
| --- | --- |
| `orderSyncQueue` | 订单同步 Redis 队列深度、Worker 并发、`redisAvailable` |
| `productPublishQueue` | 商品刊登（含抖店草稿创建）队列 |
| `inventorySyncQueue` | 库存同步（含 `sku.syncStock`）队列 |
| `workers` | 各 Worker 心跳；`degraded=true` 时整体 `status=degraded` |

### 抖店运行态、健康与指标（Phase 10.4，无 Prometheus）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/platform/douyin/runtime-status` | `normal` / `paused` / `emergency_disabled`、原因与时间 |
| `GET` | `/api/v1/platform/douyin/health` | 抖店聚合健康：`overallStatus`（`healthy` / `degraded` / `unhealthy` / `disabled`）、`config` / `auth` / `storage` / `tasks` / `api` 分区、`grayRelease`、`runtime`；快照写入 settings `health_snapshot` |
| `GET` | `/api/v1/platform/douyin/metrics-summary` | 滚动 24h 内存指标（API 成功率/耗时、Token 刷新、任务 stale、刊登/订单/库存/SKU 计数等）；**非** Prometheus `/metrics` |
| `GET` | `/api/v1/platform/douyin/release-gate` | Release Candidate 门禁清单：`overallConclusion`（默认 `Release Candidate`）、`items[]`（`key` / `label` / `status` / `message`）；`credentials` 项在无真实 E2E 时为 `blocked` |
| `POST` | `/api/v1/platform/douyin/run-health-check` | 执行健康聚合 + taskcenter 抖店告警 scan；返回与 `GET .../health` 相同结构并持久化快照 |
| `POST` | `/api/v1/platform/douyin/production-preflight` | 上线预检；`data.blockedByRealCredentials` 为 true 时表示无真实凭证 |
| `GET` | `/api/v1/platform/douyin/production-preflight/latest` | 最近一次预检 JSON |

### 任务中心（失败 / 告警 / 摘要）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/task-center/summary` | 失败任务与告警计数摘要 |
| `GET` | `/api/v1/task-center/failures` | 失败任务列表；`taskType` 含 `ai_text`（批量 AI 文案子项）；深链 `detailUrl` → `/product/ai-text-batches/:id?itemId=` |
| `GET` | `/api/v1/task-center/failures/:taskType/:id` | 失败详情（脱敏 raw） |
| `GET` | `/api/v1/task-center/alerts` | 站内告警列表 |
| `POST` | `/api/v1/task-center/alerts/scan` | 扫描并生成告警（dedupe） |
| `POST` | `/api/v1/task-center/alerts/:id/notify` | 手动触发已配置的邮件、通用 Webhook、飞书群机器人或企业微信群机器人通知；请求体可指定 `channels`，发送结果写入通知审计记录 |
| `GET` | `/api/v1/task-center/failure-categories` | 含 `sub:douyin_*` 分类 |

### 操作日志与运营看板

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/operation-logs` | 查询 `action`（如 `douyin.auth.success`）；不返回 Secret/Token |
| `GET` | `/api/v1/dashboard/product-operations` | 运营总览 KPI、漏斗、异常（只读 DB 聚合，不调平台 OpenAPI；含 RBAC 店铺 scope） |
| `GET` | `/api/v1/dashboard/overview` | 模块化 overview + 10 张运营卡片 |
| `GET` | `/api/v1/dashboard/todos` | 统一待办流（P0/P1/P2 优先级） |
| `GET` | `/api/v1/dashboard/health` | 子系统健康 + 配置风险摘要 |

### AI 商品运营工作台（Phase A3.3）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/ai/operation-workbench/summary` | 待办统计卡片（文案/图片/发布检查/刊登异常/今日已处理） |
| `GET` | `/api/v1/ai/operation-workbench/todos` | 分页待办列表；支持 `type` / `priority` / `platform` / `shopId` / `keyword` / 时间 |
| `GET` | `/api/v1/ai/operation-workbench/todos/:id` | 单条待办详情 |
| `POST` | `/api/v1/ai/operation-workbench/todos/refresh` | 重新聚合待办（只读，不写库、不调平台 API） |

## Operations API retirement

The application-level backup management, restore validation, release recorder and disaster-recovery drill recorder APIs have been retired. Backup execution, retention, encryption, point-in-time recovery, monitoring and restore drills are owned by the cloud database and operations platform. Existing historical operation tables are not exposed and are not dropped automatically.

## P7 Performance / Capacity API Status

The reusable P7 runtime work remains in backend configuration, database tables, pagination guards and local rate-limit middleware, but it does **not** expose public management APIs. Historical dataset/load/soak/race harnesses and generated evidence were removed from the production-maintenance working tree; no current result may be described as production performance verification.

Planned ops routes remain design-only until implemented with RBAC, re-authentication for writes and audit logging:

| 方法 | 路径 | 状态 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/ops/performance/overview` | planned | 聚合 API / DB / Worker / Provider 性能概览。 |
| `GET` | `/api/v1/ops/performance/regressions` | planned | 性能回归记录。 |
| `GET` | `/api/v1/ops/capacity/overview` | planned | 数据规模、连接池、Worker 容量与扩容建议。 |
| `GET` | `/api/v1/ops/rate-limits` | planned | 限流策略只读展示，不暴露 Redis key 或明文 PII。 |
| `PUT` | `/api/v1/ops/rate-limits/:policyId` | planned | 高权限、重认证、审计后修改受控策略。 |
| `GET` | `/api/v1/ops/quotas` | planned | Tenant / Shop / User / System 配额模板。 |
| `POST` | `/api/v1/ops/profiling/cpu` | planned | 内部高权限 profiling，duration 有上限，不返回任意路径。 |

Current code-level P7 endpoints affected: product and order list APIs reject excessive deep offset via P7 pagination guard; HTTP requests can be locally rate-limited when `RATE_LIMIT_ENABLED=true`.

## 修改 API 时的同步要求

- 后端：handler、service、DTO、权限和错误处理一起检查。
- 前端：`admin/src/services`、`admin/src/types`、相关页面字段和状态映射一起检查。
- 文档：同步本文档、`docs/module-map.md` 和必要的 README 能力描述。
- 安全：涉及密钥、Token、密码、Cookie 时同步 `SECURITY.md`。
- 任务：耗时接口必须使用任务状态，不应在 HTTP 请求中长时间阻塞。
## P3.2 Douyin Webhook Routing Addendum

For `platform=douyin_shop` / `douyin`, the public webhook route resolves the verified payload to a concrete shop binding before persistence. Accepted events carry `tenantId`, `internalShopId`, `platformShopId`, `appId`, and `bindingId` into `webhook_events` and downstream order upsert. Duplicate detection is scoped by `platform + tenant_id + platform_shop_id + event_id`, so the same platform `event_id` from two shops does not collide.

Resolution failures are non-success ACKs and may use codes such as `DOUYIN_WEBHOOK_SHOP_NOT_RESOLVED`, `DOUYIN_WEBHOOK_SHOP_AMBIGUOUS`, `DOUYIN_WEBHOOK_BINDING_REVOKED`, `DOUYIN_WEBHOOK_AUTHORIZATION_EXPIRED`, `DOUYIN_WEBHOOK_APP_BINDING_MISMATCH`, and `DOUYIN_WEBHOOK_TENANT_MISMATCH`.
