---
name: frontend-design
description: TradeMind Admin UI 设计规范、共享组件规范、布局规范、响应式验收和 AI 实施流程的唯一完整来源
license: Complete terms in LICENSE.txt
---

# TradeMind Admin UI 设计与 AI 实施规范

本文件是 TradeMind Admin UI 设计与实施规范的唯一完整来源。任何 AI 工具（Cursor、Claude Code、Codex、Copilot、Continue、Windsurf、Trae 或其他 Agent）处理 Admin 前端任务时，必须以本规范为准；其他入口只应引用本文件，不应复制另一套完整 UI 规范。Admin UI 实施除 UI 和 E2E 验收外，还必须遵循 `.agents/skills/code-quality/SKILL.md`。

大型页面拆分、共享 UI 边界、跨页面模块设计、公共 type/service 和 DraftDetail 演进遵循 `.agents/skills/modular-architecture/SKILL.md`；本规范只定义 UI 视觉、布局、响应式和交互验收。

## 1. 自动适用规则

本规范不仅适用于用户明确要求“UI 设计”“UI 优化”“响应式验收”或“使用 frontend-design skill”的任务。任何涉及 Admin 前端的页面、组件、样式、布局、响应式、交互、状态展示、文案、可访问性或视觉 Bug，均自动适用本规范，任务发起者无需重复指定。

出现以下任一语义时，必须自动判定为 UI 相关任务并读取本规范。典型用户描述包括但不限于：“修复这个页面显示问题”“新增一个 Admin 页面”“表格在手机上溢出”“按钮点击两次”“改个按钮”“改个表格”“优化页面”。

- 页面视觉：页面不好看、页面需要优化、页面样式调整、内容没有对齐、间距不一致、字号不合理、颜色不统一、卡片层级不清楚、页面太拥挤、页面太空、内容区域过窄、页面宽度异常、按钮层级不合理、状态展示不清楚。
- 页面布局：Header 与 Content 不对齐、Breadcrumb 与正文不对齐、padding / margin 问题、栅格布局问题、Flex / Grid 问题、页面根节点横向滚动、表格撑破页面、Tabs 溢出、Card 宽度问题、侧边栏展开或收起后布局异常、固定栏 / 工具栏 / 分页错位。
- 响应式和移动端：手机端适配、平板适配、小屏幕显示异常、Modal 超出屏幕、Drawer 宽度异常、按钮在移动端被截断、表格移动端不可用、长文本撑破布局、视口切换后错位。
- 组件与交互：Button、Form、Table / ProTable、Card、Tabs、Modal、Drawer、Popconfirm、Tooltip、Select、Upload、Pagination、EmptyState、Alert、Tag、Toolbar、PageContainer、Dashboard 卡片、状态面板的新增或修改。
- 页面状态：loading、empty、error、readonly、disabled、submitting、refreshing、success、failure、partial data、无权限、无数据、请求失败。
- 用户交互问题：点击无反应、点击两次、重复提交、Modal 无法关闭、Drawer 再次打开状态异常、表单默认值错误、表单校验展示问题、按钮 disabled 不清楚、Tab 切换状态错误、深链定位错误、浏览器刷新后页面状态错误、键盘操作问题、focus 状态问题。
- UI 文案：按钮文案、页面标题、页面描述、状态文案、空状态文案、错误提示、风险提示、创建草稿与正式发布语义、任务创建与任务完成语义、用户可执行建议。
- 可访问性：aria-selected、aria-label、role、键盘导航、focus、表单错误关联、图标按钮无名称、Tab 可访问性、Modal / Drawer 可访问性。

涉及以下目录或文件时，应主动判断是否包含 UI；只要包含页面渲染、组件结构、`className`、样式、交互或状态展示，就必须自动应用本规范：

- `admin/src/pages/**`
- `admin/src/components/**`
- `admin/src/layouts/**`
- `admin/src/app.tsx`
- `admin/src/global.less`
- `**/*.tsx`
- `**/*.jsx`
- `**/*.less`
- `**/*.css`
- `**/*.scss`

不能仅因为用户把任务描述为“修复 Bug”“小问题”“调整一下”“改个按钮”“改个表格”“修复显示”或“优化页面”就跳过本规范。不得询问用户是否需要使用规范，不得把规范作为可选项。

纯前端任务只有在确认完全不影响 DOM、`className`、样式、用户交互、页面状态、loading / error / empty 和响应式后，才可以不执行完整 UI 工作流。可排除完整 UI 工作流的场景包括：纯 TypeScript 类型修复、纯 service 封装、纯 API 参数修复、纯数据转换、纯工具函数、纯构建配置、纯测试配置、纯依赖升级、纯性能计算逻辑、不影响 DOM / 样式 / 交互的内部重构。只要其中任一项变化，仍应自动应用本规范。

混合任务必须拆分处理：UI 部分按本规范执行；API、后端、数据库和业务逻辑部分按现有 service、DTO、权限和业务规则执行。不得因为任务包含 API 就跳过 UI 规范，也不得以 UI 优化为名修改 API、payload、权限、readonly 或状态机。

只有以下情况需要先询问用户：需要改变业务流程、API、payload、权限、readonly、状态机；存在多个会明显影响产品方向的方案；规范与用户明确要求发生冲突。

## 2. 适用范围

必须默认使用本规范的目录和文件：

- `admin/src/pages/**`
- `admin/src/components/**`
- `admin/src/layouts/**`
- `admin/src/app.tsx`
- `admin/src/global.less`
- Admin 相关 CSS / LESS / TSX / JSX
- Admin 路由对应的新增和修改任务

必须使用本规范的任务：

- 新增 Admin 页面、修改已有 Admin 页面、页面重构
- 页面视觉优化、布局调整、响应式修复、UI Bug 修复、可访问性修复
- 新增或修改共享组件
- 新增表格、筛选区、工具栏、表单、详情页
- Modal、Drawer、Popconfirm 开发或调整
- Dashboard、工作台、空状态、错误态、加载态、readonly / disabled 状态开发
- 修改文案和交互状态
- 移动端适配

纯后端任务、数据库任务和非 Admin 前端任务不强制套用完整 UI 工作流。

## 3. 总体设计原则

必须：

- 优先保持项目现有设计语言，新增和修改必须视觉一致。
- 信息层级优先于装饰，结构、状态、反馈先清楚，再考虑视觉表现。
- 业务状态必须真实，不创建虚假指标、虚假进度或虚假结论。
- 高风险操作必须清晰可辨，但不能大面积滥用红色。
- 文案准确表达真实业务影响，不夸大系统已经完成的动作。
- 每次 UI 修改必须同时改善用户体验、开发体验和 AI Agent 的使用体验，并保护现有功能；具体要求见 `docs/ai-coding-rules.md` 的“三方体验与功能保护”。

禁止：

- 为单个页面建立独立设计体系。
- 使用营销落地页式夸张设计。
- 使用大面积渐变、玻璃拟态、无业务意义装饰。
- 为了“看起来高级”增加视觉噪声。
- 使用虚假数据、虚假指标、虚假进度或虚假结论。

### 3.1 首次交付的视觉质量

UI 首次交付必须保证布局合理、比例协调、视觉整洁，包含整体构图、空间利用、留白、对齐、密度和操作层级的自检与必要修正，不能把明显视觉问题留给用户二次指出。编译通过、功能可用、没有横向溢出，都不能代替视觉验收。

必须：

- 先确定页面类型、主任务、信息分组、阅读顺序、内容宽度与主操作位置，再组合组件；不能把组件堆满后才考虑布局。
- 按内容选择宽度：列表和表格充分使用空间，短配置表单控制阅读与输入宽度；相关短字段可合理分列，长文本与复杂输入保留足够宽度。不得在超宽屏上把少量短字段无限拉长，也不为填满屏幕拉高卡片或增加装饰区块。
- 宽屏必须检查整页的视觉重心和有效空间分配，禁止把唯一的短表单放在靠左窄卡片中，使右侧成为大片无意义空白；不得保留无内容的栅格列或用过大的固定间距、固定高度制造空区。留白应帮助分组和阅读，不能靠增加说明、虚构指标或装饰区块填空。
- 独立的短配置、个人偏好页默认在页面容器层统一限宽并居中，页头、正文和操作区共用同一内容轨道；确有多个业务分组时再合理分栏，窄屏恢复单列。例如只有语言、主题、日期格式的偏好页，应形成紧凑完整的设置区，不能保留“左侧一张窄卡片、右侧大半页空白”的布局。
- 明确区分页面外边距、卡片内边距、区块间距和控件间距；同类区域使用统一基线，内容不贴边。先检查父容器已有留白，再选择共享组件、Flex / Grid 的 `gap` 或局部样式，避免重复 padding。
- 页面标题、正文与主操作遵循同一内容轨道；卡片内部的标题、字段和底部操作对齐。字体、按钮尺寸、状态样式沿用现有设计体系。
- 提示区、表单主体和保存 / 取消操作区必须清楚分隔；独立操作区与前一内容区至少保留 `layoutTokens.fieldGroupGap` 的间距，不能出现提示框与保存按钮紧贴、按钮贴边或操作区被挤压。
- 复查五档视口下的布局比例、留白、长文本和操作区；短表单、设置页及修改最大内容宽度时，另查至少一档 1920px 或更宽桌面视口。用户截图暴露超宽屏或特殊尺寸问题时，按截图实际宽高复现并补验，不能只缩到常规视口检查。发现大片单侧空白、拥挤、松散、错位或不合理拉伸时，先修正再交付。

桌面默认间距基线如下；实施时复用共享组件或 Token，数值以 `admin/src/constants/layoutTokens.ts` 为准，不复制成另一套常量。移动端按第 11 节适配，紧凑场景优先使用已有 compact 能力。

| 区域 | 现有 Token | 当前基线 |
| --- | --- | --- |
| 页面横向 / 纵向 / 底部留白 | `pagePaddingX` / `pagePaddingY` / `pagePaddingBottom` | 24 / 20 / 32px |
| 卡片常规 / 紧凑内边距 | `cardPadding` / `cardPaddingCompact` | 24 / 20px |
| 区块 / 卡片间距 | `sectionGap` / `cardGap` | 24 / 16px |
| 表单行 / 列间距 | `formRowGap` / `formColumnGap` | 20 / 24px |
| 标签与控件间距 | `labelControlGap` | 8px |
| 独立操作区与前一内容区间距 | `fieldGroupGap` | 至少 16px |

视觉自检应查看实际渲染结果，验证方式继续遵循 `.agents/skills/admin-e2e-testing/SKILL.md` 的生产维护策略。未实际查看时明确标为“视觉未验证 / 待人工验收”，不得宣称视觉通过；不新增截图报告或长期验收产物。

## 4. 必须优先复用的共享能力

实施前必须先检查并优先复用：

- `TmPageContainer`
- `SectionCard`
- `MetricCard`
- `OperationToolbar`
- `TmProTable`
- `EmptyState`
- `AppDrawer`
- `layoutTokens`
- Ant Design Token
- Pro Components

规则：

- 已有共享组件能够满足时不得重复实现。
- 不创建与 `TmPageContainer`、`SectionCard`、`TmProTable` 职责重复的组件。
- 不为单页复制一套共享布局。
- 只有三个以上真实同构场景才考虑抽象新共享组件。
- 新组件必须职责单一。
- 共享组件不得耦合页面业务 API、权限或状态机。

## 5. 页面容器和横向基线

必须：

- `TmPageContainer` 是登录后标准页面容器。
- Breadcrumb、Page Header、Title、Description、Header Extra 和正文必须使用同一内容轨道。
- Header 和 Content 共用同一 `max-width`、`margin-inline` 和 `padding-inline`。
- 使用 `layoutTokens.pageMaxWidth`。
- 标准列表页和表格页默认充分使用可用宽度。
- Card 外边缘应与 Page Header 内容轨道对齐。
- Header 与主要内容外边缘误差原则上不超过 4px。

允许：

- 明确的窄表单页可以使用 `max-width`，但必须有真实设计目的，并在页面容器层统一约束 Header / Content 的宽度与居中方式；仅缩窄内层表单或卡片、留下大片单侧空白不算完成布局设计。
- Card 内部文字不要求与页面标题文字完全同一 X 坐标。

禁止：

- 在页面根节点重复添加整页左右 padding。
- 使用无意义 `margin-left`。
- 写死侧边栏宽度。
- 使用 `width: calc(100vw - sidebarWidth)`。
- 通过 `transform`、负 margin 或写死偏移修复对齐。

## 6. 页面信息架构

标准页面建议顺序：

1. Breadcrumb
2. 页面标题
3. 页面描述（仅有必要补充信息时展示）
4. Header Extra / 主操作
5. 状态或上下文说明（仅展示影响判断或操作的信息）
6. 筛选和工具栏
7. 主要内容
8. 分页、记录或技术信息

必须：

- 一个页面只有一个明确主操作。
- 次级操作不得全部使用 `primary`。
- 查看、配置、提交、危险操作必须分层。
- 请求失败、空状态、未配置、readonly、权限错误必须表达真实状态。

文案语义必须准确：

- “创建草稿”不得描述为“发布成功”。
- “任务创建成功”不得描述为“任务执行完成”。
- “检查通过”不得描述为“平台已通过”。
- 请求失败不得展示成空状态。
- 未配置不得展示成加载失败。
- readonly 不得展示成权限错误，除非真实逻辑如此。

## 7. 列表和表格

必须：

- 优先使用 `TmProTable`。
- 筛选区、工具栏和表格使用清晰层级。
- 表格 Card 外边缘与页面内容轨道对齐。
- 表格可在自身容器内部横向滚动。
- 页面根节点不得横向滚动。
- 使用稳定、唯一 `rowKey`。
- 长 ID、URL、店铺名、平台名、错误信息必须省略、换行或 Tooltip。
- 状态 Tag 必须映射真实状态。
- 未知状态应安全回退，不得伪装成成功。
- 空状态、加载失败和无权限必须区分。

禁止：

- 使用数组 `index` 作为持久列表 `rowKey`。
- 使用随机值作为每次渲染变化的 `rowKey`。
- 用无意义固定宽度挤压操作列。

临时客户端记录必须生成并保持稳定的临时 ID。

## 8. 表单

必须：

- 保留 Form 的真实数据结构。
- loading / disabled / validation 状态明确。
- 表单失败后不得无故清空用户输入。
- readonly 行为遵循现有权限规则。
- Modal Form 挂载问题必须定位具体 Form 实例和 Modal 生命周期。

禁止：

- 擅自修改 `Form.Item name`。
- 擅自修改默认值。
- 自动选择第一项，除非业务原逻辑明确支持。
- 自动保存、自动提交。
- 嵌套 Form。
- Button 同时因 `htmlType="submit"` 和 `onClick` 造成双重提交。
- 通过批量 `forceRender` 猜测修复 useForm warning。

## 9. Modal、Drawer、Popconfirm

Modal 必须检查：

- title、当前业务上下文、默认值、校验
- loading、confirmLoading、disabled
- 取消、关闭后状态清理、footer
- 长文本、移动端宽度
- 单次确认只发一次请求

Drawer 必须检查：

- loading、normal、empty、error
- 长文本、内部滚动、关闭、再次打开状态
- 375px 接近全宽
- 根节点无横向溢出

高风险操作必须：

- 保留确认。
- 取消不得发请求。
- 危险操作使用 danger 层级。
- 不得与普通查看按钮同级。
- 不得跳过原确认逻辑。

## 10. Loading、Empty、Error、Readonly

每个异步模块至少考虑：

- initial loading
- refreshing
- success
- empty
- partial data
- request error
- business error
- readonly
- disabled
- submitting

必须：

- 错误不能伪装为空数据。
- 空数据不能伪装为错误。
- loading 期间不闪现上一个对象的数据。
- 失败后按原逻辑保留用户输入和已有数据。
- readonly 只保持现有业务语义。
- 权限覆盖不完整时记录问题，不擅自统一修改。

禁止：

- AI 主观扩展 readonly 和权限策略。

## 11. 响应式规范

强制验收视口：

- 1440×900
- 1280×800
- 1024×768
- 768×900
- 375×812

必须：

- 页面根节点无横向溢出。
- Header、Content 使用同一 gutter。
- 375px 下页面 gutter 一般保持 12px～16px。
- Card 不超出视口。
- 操作区允许换行。
- Modal 接近全宽但不超出视口。
- Drawer 接近全宽并可滚动。
- 表格只在自身容器内部滚动。
- Tabs 可以内部滚动，但不得造成页面根溢出。
- 长文本必须可换行。
- 侧边栏展开和收起均需正常。

禁止：

- 保留桌面端无意义大 padding。
- 通过页面根节点横向滚动容纳内容。

根节点横向溢出标准：

```js
document.documentElement.scrollWidth <= document.documentElement.clientWidth;
document.body.scrollWidth <= document.body.clientWidth;
```

## 12. 样式规范

必须：

- 页面样式优先写入页面局部 LESS。
- 共享容器样式放到对应共享组件或既有壳层样式。
- 优先使用 Ant Design Token。
- 优先使用 `layoutTokens`。
- 全局选择器必须有限定作用域。
- 修改 `global.less` 必须证明是页面壳层或全局共享根因。
- 修改样式时不统一改变 LF / CRLF。

禁止：

- 散落重复 HEX。
- 新增全局 `.ant-*` 覆盖。
- 新增 `!important`。
- 通过负 margin 修复布局。
- 通过 `transform` 修复对齐。
- 重写完整 LESS 文件。
- 影响登录页、错误页、Modal、Drawer 和全屏页面。

## 13. 文案规范

界面应通过布局、分组、明确命名和操作层级表达用途，文案只保留完成任务、识别状态和理解风险所必需的信息。

必须：

- 删除多余的标语、重复说明文字和无业务意义的次要标签，不为填充页面或装饰卡片添加文案。
- 删除冗余内容后，若用户仍看不懂页面用途、当前状态或下一步操作，必须重新设计受影响区域的信息架构、布局与操作层级，不得补回冗余说明来掩盖结构问题。
- 保留必要字段标签、真实业务状态、风险与不可逆后果提示、校验与错误恢复信息、可访问名称；精简不得删减现有功能或改变业务行为。
- 文案准确表达真实业务影响。
- “保存”“创建草稿”“提交”“发布”“同步”“生成任务”明确区分。
- 按钮使用明确动词。
- 高风险按钮明确结果。
- 技术错误和用户可执行建议分层展示。
- 文案修改后执行 `pnpm.cmd check:ui-copy --strict`。

例如：标题已是“商品草稿”时，删除“高效管理您的商品草稿”等重复说明；主操作难以定位时调整操作区布局。

禁止：

- 使用夸张营销文案。
- 使用含糊的“确定”“立即完成”替代真实动作。
- 未完成流程写成“已完成”。

## 14. 可访问性

必须：

- Tabs `activeKey`、`aria-selected` 和 active pane 一致。
- Button、Link、Tab、Modal、Drawer 支持键盘操作。
- focus 样式清晰。
- disabled 状态可识别。
- 图标按钮必须有可访问名称。
- 表单错误与字段关联。

禁止：

- Tooltip 作为唯一信息来源。
- 手写与组件状态冲突的 aria 属性。
- 通过坐标点击代替 role / locator 验收。

## 15. 业务保护规则

UI 任务默认不得修改：

- API URL
- HTTP method
- service
- DTO
- 请求 payload
- handler
- 路由
- 权限
- readonly
- 状态机
- reload 顺序
- 分页参数
- 排序
- 业务判断
- 自动刷新策略
- 任务状态
- 平台状态
- 库存语义
- 发布语义

如确实必须修改业务行为，必须先明确说明、解释原因、列出影响并获得用户确认。不得以“UI 优化”为名偷偷改变行为。

禁止新增以下自动行为，除非需求明确要求并得到确认：

- 自动保存
- 自动提交
- 自动选择
- 自动重试
- 自动轮询
- 自动创建任务
- 自动上传
- 自动同步
- 自动绑定
- 自动发布
- 一键修复全部

## 16. AI 实施工作流

### Step 1：读取规范

任务开始必须读取：

- `AGENTS.md`
- `.agents/skills/frontend-design/SKILL.md`
- 当前目录适用的 Cursor rules
- 目标页面 TSX / JSX
- 目标页面 LESS / CSS
- 相关共享组件
- `TmPageContainer`
- `layoutTokens`
- 与目标页面相关的现有 UI 封装
- 目标页面现有实现

### Step 2：检查 Git

执行：

```bash
git status --short --branch
git diff --stat
git log -5 --oneline
```

不得覆盖用户已有修改。除非用户明确要求，不得创建分支、commit、push、reset、restore、clean、stash。禁止使用 `git add .`。

### Step 3：定位真实代码

必须先定位：

- 页面 JSX
- 页面 LESS
- shared UI
- service
- types
- handler
- loading
- error
- readonly
- disabled
- URL 状态
- Modal / Drawer
- 表格 `rowKey`
- section id

不得根据界面截图猜测业务实现。

### Step 4：先分析后修改

修改前必须给出：

- 当前信息架构
- 真实业务流程
- UI 问题
- 页面类型、信息分组、内容宽度、间距 Token 与主操作位置，按第 3.1 节确定。
- 拟修改范围
- 保持不变的业务行为
- 风险
- 验证方案

对于明确、小范围任务，可以输出简短分析后继续，不必等待确认。

遇到以下情况必须停止并询问用户：

- 需要改变 API
- 需要改变 payload
- 需要改变权限
- 需要改变状态机
- 需求与现有业务明显冲突
- 存在两种不同且影响业务的实现选择
- 无法确认写操作真实效果

### Step 5：最小实施

必须：

- 优先复用共享组件。
- 只改目标范围。
- 纯 UI 任务不得改变业务行为。
- 按第 3.1 节主动复查视觉质量并修正已发现的问题，不把布局和间距完善推迟到用户反馈后。

禁止：

- 顺手重构。
- 格式化完整文件。
- 修改无关 Tab。
- 复制已有逻辑。
- 创建重复组件。

### Step 6：静态检查

至少执行：

```bash
git diff --check
pnpm.cmd check:dev
pnpm.cmd check:ui-copy --strict
pnpm.cmd build:admin
git diff --stat
git diff --numstat
git status --short --branch
```

如果项目后续新增稳定的 lint / typecheck，也应执行现有命令，但不得临时新增依赖。

### Step 7：浏览器验收

Admin UI 实施后的自动化验收要求由 `.agents/skills/admin-e2e-testing/SKILL.md` 统一定义；本 Skill 只保留 UI 验收入口，不复制完整 E2E 规范。

Admin 页面修改原则上必须使用 Playwright MCP 验收。Admin 服务由用户启动时，不得自行启动、不得停止、不得杀进程；服务不可用时停止并报告。

必须：

- 使用五档视口。
- 检查根节点 overflow。
- 检查 Header / Content 对齐。
- 按第 3.1 节检查内容宽度、内外边距、区块分隔和操作区留白，不能只检查 overflow。
- 检查 loading / empty / error。
- 检查 Modal / Drawer / Popconfirm。
- 检查长文本。
- 检查 readonly。
- 检查 Console warning / error。

所有非 GET 请求必须使用 `browser_route` 拦截。不得执行真实平台写操作。

### Step 8：网络副作用检查

对于写操作必须捕获：

- method
- URL
- path params
- query
- payload
- 次数
- reload
- 额外请求

必须验证：

- 取消不发请求。
- 单次确认只发一次。
- 快速重复确认不重复提交。
- 失败不自动重试。
- 不触发无关业务写请求。

### Step 9：输出结果

必须输出：

- 当前分支
- 开始工作区
- 修改文件
- 修改内容
- 保持不变的行为
- 检查命令
- 浏览器视口
- overflow 数据
- 写请求
- Console 信息
- 当前 diff
- 未提交文件
- 剩余风险
- 是否可签收
- 是否适合人工验收

除非用户明确要求，不得 commit 或 push。

## 17. 新增 Admin 页面强制清单

新增页面必须：

- 使用标准登录后 Layout。
- 优先使用 `TmPageContainer`。
- Header 和 Content 共用内容轨道。
- 根据真实需要提供 breadcrumb、title、description、extra。
- 使用 `SectionCard` / `TmProTable` / `OperationToolbar` 等共享组件。
- 提供 loading、empty、error。
- 考虑 readonly、权限、长文本。
- 提供稳定 `rowKey`。
- 支持五档视口。
- 根节点无 overflow。
- 不复制已有页面布局代码。
- 不新增第二套设计 Token。
- 不修改全局样式解决单页问题。
- 不自动执行写操作。
- 补充浏览器 Mock 验收。
- 通过全部检查命令。

新增页面不得在没有说明的情况下直接使用裸 `PageContainer`、`ProTable`、`Card` 或自定义 page wrapper；应先确认项目共享封装是否已经适用。

## 18. 修改已有页面强制清单

修改前必须确认：

- 原 API
- 原 payload
- 原 handler
- 原状态机
- 原权限
- 原 readonly
- 原 loading
- 原 URL
- 原 reload
- 原写请求次数

修改后必须确认：

- 没有业务回归。
- 没有新增重复提交。
- 没有破坏深链。
- 没有新增根节点 overflow。
- 没有新增 key warning。
- 没有新增 useForm warning。
- 没有把错误展示为空状态。
- 没有把草稿描述为正式完成。
- 没有破坏移动端。
- 没有扩大无关 diff。

## 19. 禁止项总表

禁止：

- Tailwind
- shadcn
- 新增 UI 框架
- 新增与 Ant Design 重复的依赖
- 大面积全局 CSS 覆盖
- 全局 `.ant-*` 魔改
- `!important`
- 负 margin 对冲
- `transform` 位移修布局
- 写死侧边栏宽度
- 重复页面 gutter
- 卡片套卡片
- 所有按钮都 `primary`
- 所有状态都 `MetricCard`
- 虚假数据和虚假指标
- 修改 API 伪装成 UI 优化
- 为消除 warning 批量猜测性添加 `forceRender`
- 使用 `index` 作为持久表格 `rowKey`
- UI 任务中顺手改业务逻辑
- 未拦截时执行真实写操作
- 未验收就声明完成
- 未获授权时 commit 或 push

## 20. 发现性与入口维护

- `AGENTS.md` 是跨工具项目入口，只应强制要求读取本 skill 和列出门禁，不复制完整规范。
- Cursor rule 只负责对 Admin TSX / JSX / LESS / CSS 文件自动生效，并指向本 skill。
- Claude Code 如存在 `CLAUDE.md`，只应引用本 skill，不复制完整规范。
- 其他 AI instruction 文件已存在时可以增加引用；没有时不盲目创建。
- 不允许多个文件同时声明自己是 Admin UI 规范的唯一完整来源。
