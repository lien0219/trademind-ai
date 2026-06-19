# 批量 AI 图片 UX 验收（Phase A3.2）

## 后端自动化

- [x] `aiproductimage` 单元测试（操作类型、幂等键、质量 warning 映射）
- [x] `go test ./...` 通过
- [x] 抖店 / productpublish 回归通过

## 前端构建

- [x] `pnpm build:admin` 通过

## 人工验收清单

1. [ ] 商品草稿列表多选 → 「批量 AI 图片处理」进入向导
2. [ ] 向导 5 步：商品 → 图片 → 处理方式 → 要求 → 确认
3. [ ] 预检查返回中文 ready/warning/blocked
4. [ ] 创建批次后进入 `/product/ai-image-batches/:id`
5. [ ] 待复核项可打开原图/结果图对比弹窗
6. [ ] 单张应用（四种 applyMode）有二次确认（替换原图）
7. [ ] 批量应用已选 + 批量撤销本批次
8. [ ] 失败任务中心 `ai_image` 跳转复核页并高亮 `itemId`
9. [ ] 1366px / 1024px 布局可用
10. [ ] 无英文内部码直出（技术详情折叠）

## 真实 Provider 试跑（待执行）

- [ ] 小样本（2 商品 × 3 图 × 白底图）进入 `pending_review`
- [ ] 0 自动覆盖商品图片
- [ ] partial_success / retry-failed 行为正确

## 完成标准对照

见 Phase A3.2 需求文档第二十四节；代码与构建已通过，**真实 Provider 小样本试跑仍待人工执行**后方可开闸 A3.3。
