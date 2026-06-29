# 配置状态中心设计（Phase F5）

## 路由

- 后台：`/settings/config-status`
- API：`GET /api/v1/settings/config-status`

## 模块项

AI 文案、AI 图片、OCR、Storage、Storage 公网、Redis/Worker、采集服务、抖店凭证、平台发布、订单/库存/客服同步开关、Demo 数据状态。

## 安全

不返回 API Key、Token、App Secret、完整 Prompt。  
Storage 公网测试需人工触发，不在此接口自动写平台。

## 实现

`backend/internal/modules/configstatus`
