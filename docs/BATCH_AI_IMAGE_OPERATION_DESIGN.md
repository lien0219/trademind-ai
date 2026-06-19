# 批量 AI 图片操作设计（Phase A3.2）

## 目标

商品列表多选 → 批量 AI 图片处理 → 选择处理类型 → 生成结果 → 待复核 → 单张对比 → 批量应用 → 冲突保护 → 安全撤销 → 失败任务中心联动。

## 任务模型

- **批次表** `ai_product_image_batches`：批次号 `IMGyyyyMMddNNNN`、状态、`productCount` / `imageCount` / `itemCount`、幂等键。
- **子项表** `ai_product_image_items`：每商品图片 × 处理方式一条；关联 `image_task_id`；`sourceSnapshotHash` 用于应用冲突检测。
- **应用快照** `product_image_applications`：记录 apply/undo 所需原图状态。

## 处理方式映射

| 用户选项 | imagetask 类型 |
| --- | --- |
| 图片质量检查 | `score_image` |
| 去水印 | `remove_watermark` |
| 去 Logo | `remove_logo` |
| 白底图 | `remove_background`（remove.bg 等）；通义万相等无抠图能力时批次层降级为 `replace_background` + 白底 prompt |
| 优化背景 | `replace_background` |
| 翻译图片文字 | `translate_image_text` |
| 主图优选建议 | `select_best_main` |

## 应用方式

- `save_to_gallery`：新增 AI 图库行（`ai_generated`）
- `set_main`：设为主图（保留原主图，清除其他 `is_best_main`）
- `add_detail`：添加详情图
- `replace_image`：替换原图 URL（保存完整原图快照，可撤销）

**禁止**：自动静默替换、自动删除原图、跳过人工复核。

## 安全下载

创建前检查复用 `safedownload.ValidateURL` + `Download`：SSRF 防护、大小 10MB、MIME/解码校验、中文错误文案。

## 批量限制

- 单批最多 50 商品（`ai_image_batch_max_products` 可配置）
- 单批最多 300 子项（`ai_image_batch_max_images` 可配置）

## 失败任务中心

- `taskType=ai_image`，源表 `ai_product_image_items`
- 分类：`ai_image_process_failed` / `ai_image_apply_conflict` / `ai_image_quality_warning` 等
- 深链：`/product/ai-image-batches/:batchId?itemId=:itemId`

## 明确不做

- 自动上架、抖店素材中心 OpenAPI、营销图模板系统、重做 imagetask Provider 体系。

## API

前缀 `/api/v1/products/ai-images/batches` 与 `/items/:id/*`，详见 [`api.md`](api.md)。
