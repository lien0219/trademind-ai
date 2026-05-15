# Cursor Rules 使用说明

本目录包含贸灵 TradeMind 项目的 Cursor 规则文件。

## 使用方式

将以下内容放到项目根目录：

```text
.cursor/rules/*.mdc
.cursorrules
README.md
.gitignore
```

其中：

- `README.md`：用于 GitHub 首页展示。
- `TradeMind.md`：内部开发约束文档，仅供 Cursor 和开发者参考，不建议提交为公开展示文档。
- `.gitignore`：已默认忽略 `TradeMind.md` 和 `PROJECT_SPEC_TradeMind.md`。

## 规则内容

```text
.cursor/rules/
├── 00-project-overview.mdc
├── 01-architecture.mdc
├── 02-backend-go-gin.mdc
├── 03-frontend-react-antd-pro.mdc
├── 04-ui-style.mdc
├── 05-ai-provider.mdc
├── 06-storage-provider.mdc
├── 07-collector-node-playwright.mdc
├── 08-api-db-security.mdc
└── 09-dev-workflow.mdc
```

## 建议

1. GitHub 展示使用根目录 `README.md`。
2. 内部开发约束使用 `TradeMind.md`。
3. Cursor 主要读取 `.cursor/rules/*.mdc` 和 `.cursorrules`。
4. 如果后续项目结构变化，需要同步更新 rules。
