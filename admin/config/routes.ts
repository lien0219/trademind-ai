/**
 * 后台路由与侧栏菜单（名称即菜单文案）。
 * component 相对 `src/pages/`。
 */
export function createInternalInventorySyncRoutes(
  enabled = process.env.NODE_ENV !== 'production',
) {
  if (!enabled) return [];

  return [
    {
      path: '/ops/inventory-sync',
      name: '库存同步',
      icon: 'InboxOutlined',
      component: './Ops/InventorySync',
      hideInMenu: true,
    },
    {
      path: '/ops/inventory-sync/runs/:runId',
      name: '库存同步运行详情',
      component: './Ops/InventorySync/RunDetail',
      hideInMenu: true,
    },
    {
      path: '/ops/inventory-sync/calibration',
      name: 'SKU 校准',
      icon: 'AimOutlined',
      component: './Ops/InventorySync/Calibration',
      hideInMenu: true,
    },
    {
      path: '/ops/inventory-sync/manual-bindings',
      name: '人工绑定',
      icon: 'ApartmentOutlined',
      component: './Ops/InventorySync/ManualBindings',
      hideInMenu: true,
    },
    {
      path: '/ops/inventory-sync/bindings',
      name: '绑定历史',
      icon: 'BranchesOutlined',
      component: './Ops/InventorySync/Bindings',
      hideInMenu: true,
    },
    {
      path: '/ops/inventory-sync/bindings/:bindingId',
      name: '绑定历史详情',
      component: './Ops/InventorySync/BindingDetail',
      hideInMenu: true,
    },
  ];
}

export default [
  {
    path: '/user/login',
    layout: false,
    component: './User/Login',
  },
  {
    path: '/user/register',
    layout: false,
    component: './User/Login',
  },
  {
    path: '/',
    layout: false,
    component: './Index',
  },
  {
    path: '/dashboard',
    name: '工作台',
    icon: 'DashboardOutlined',
    component: '@/layouts/DashboardGroupLayout',
    routes: [
      {
        path: '/dashboard',
        redirect: '/dashboard/product-operations',
      },
      {
        path: '/dashboard/product-operations',
        name: '运营总览',
        component: './Dashboard/ProductOperations',
      },
    ],
  },
  {
    path: '/system/operation-logs',
    name: '操作日志',
    icon: 'AuditOutlined',
    component: './System/OperationLogs',
  },
  {
    path: '/ops',
    name: '运维',
    icon: 'ToolOutlined',
    component: '@/layouts/OpsGroupLayout',
    routes: [
      {
        path: '/ops/workers/monitor',
        name: '后台任务监控',
        icon: 'CloudServerOutlined',
        component: './Workers/Monitor',
      },
      {
        path: '/ops/task-center/failures',
        name: '失败任务中心',
        icon: 'WarningOutlined',
        component: './TaskCenter/Failures',
      },
      {
        path: '/ops/task-center/alerts',
        name: '告警中心',
        icon: 'BellOutlined',
        component: './TaskCenter/Alerts',
      },
      {
        path: '/ops/task-center/operation-tasks',
        name: '运营任务中心',
        icon: 'ProfileOutlined',
        component: './TaskCenter/OperationTasks',
      },
      {
        path: '/ops/task-center/operation-tasks/:taskId',
        name: '运营任务详情',
        component: './TaskCenter/OperationTasks/Detail',
        hideInMenu: true,
      },
      ...createInternalInventorySyncRoutes(),
      {
        path: '/ops/observability',
        name: '可观测性中心',
        icon: 'LineChartOutlined',
        component: './Ops/Observability',
      },
      {
        path: '/ops/platform-runtime',
        name: '平台运行状态',
        icon: 'ApiOutlined',
        component: './Ops/PlatformRuntime',
      },
      {
        path: '/ops/douyin/runtime',
        redirect: '/ops/platform-runtime?platform=douyin_shop',
        hideInMenu: true,
      },
    ],
  },
  {
    path: '/files',
    name: '文件管理',
    icon: 'FileImageOutlined',
    component: './Files',
  },
  {
    path: '/ai',
    name: 'AI 工具',
    icon: 'RobotOutlined',
    component: '@/layouts/AiGroupLayout',
    routes: [
      {
        path: '/ai/prompts',
        name: 'AI 技能模板',
        component: './AI/Prompts',
      },
      {
        path: '/ai/tasks',
        name: 'AI 任务记录',
        component: './AI/Tasks',
      },
      {
        path: '/ai/image-tasks',
        name: '图片任务',
        component: './AI/ImageTasks',
      },
      {
        path: '/ai/batches',
        name: '历史 AI 批次',
        hideInMenu: true,
        component: './AI/Batches',
      },
      {
        path: '/ai/text-batches',
        name: '批量文案任务',
        component: './AI/TextBatches',
      },
      {
        path: '/ai/image-batches',
        name: '批量图片任务',
        component: './AI/ImageBatches',
      },
      {
        path: '/ai/operation-workbench',
        name: '商品运营工作台',
        component: './AI/OperationWorkbench',
      },
    ],
  },
  {
    path: '/product',
    name: '商品',
    icon: 'ShoppingOutlined',
    component: '@/layouts/ProductGroupLayout',
    routes: [
      {
        path: '/product/drafts/:id',
        name: '商品详情',
        component: './Product/DraftDetail',
        hideInMenu: true,
      },
      {
        path: '/product/drafts',
        name: '商品草稿',
        component: './Product/Drafts',
      },
      {
        path: '/product/publish-batch',
        name: '批量创建刊登草稿',
        component: './Product/PublishBatch',
        hideInMenu: true,
      },
      {
        path: '/product/ai-text-batch',
        name: '批量 AI 优化',
        component: './Product/AITextBatch',
        hideInMenu: true,
      },
      {
        path: '/product/ai-text-batches',
        redirect: '/ai/text-batches',
        hideInMenu: true,
      },
      {
        path: '/product/ai-text-batches/:id',
        name: 'AI 文案批次复核',
        component: './Product/AITextBatchDetail',
        hideInMenu: true,
      },
      {
        path: '/product/ai-image-batch',
        name: '批量 AI 图片处理',
        component: './Product/AIImageBatch',
        hideInMenu: true,
      },
      {
        path: '/product/ai-image-batches',
        redirect: '/ai/image-batches',
        hideInMenu: true,
      },
      {
        path: '/product/ai-image-batches/:id',
        name: 'AI 图片批次复核',
        component: './Product/AIImageBatchDetail',
        hideInMenu: true,
      },
      {
        path: '/product/publish-batches',
        redirect: '/product/publish-tasks?tab=batches',
        hideInMenu: true,
      },
      {
        path: '/product/publish-batches/:id',
        name: '刊登批次详情',
        component: './Product/PublishBatchDetail',
        hideInMenu: true,
      },
      {
        path: '/product/publish-tasks',
        name: '刊登任务',
        component: './Product/PublishTasks',
      },
    ],
  },
  {
    path: '/collect',
    name: '采集',
    icon: 'CloudDownloadOutlined',
    component: '@/layouts/CollectGroupLayout',
    routes: [
      {
        path: '/collect',
        redirect: '/collect/hub',
      },
      {
        path: '/collect/hub',
        name: '采集中心',
        component: './Collect/Hub',
      },
      {
        path: '/collect/tasks',
        name: '采集任务',
        component: './Collect/Tasks',
      },
      {
        path: '/collect/batches',
        name: '批量采集',
        component: './Collect/Batches',
      },
      {
        path: '/collect/browser-profiles',
        name: '采集浏览器登录状态',
        component: './Collect/BrowserProfiles',
      },
      {
        path: '/collect/rules',
        name: '采集规则',
        component: './Collect/Rules',
      },
      {
        path: '/collect/monitor',
        name: '采集监控',
        component: './Collect/Monitor',
      },
    ],
  },
  {
    path: '/shops',
    name: '店铺',
    icon: 'ShopOutlined',
    component: '@/layouts/ShopGroupLayout',
    routes: [
      {
        path: '/shops',
        redirect: '/shops/manage',
      },
      {
        path: '/shops/manage',
        name: '店铺管理',
        component: './Shops',
      },
    ],
  },
  {
    path: '/orders',
    name: '订单',
    icon: 'ContainerOutlined',
    component: '@/layouts/OrderGroupLayout',
    routes: [
      {
        path: '/orders',
        redirect: '/orders/list',
      },
      {
        path: '/orders/list',
        name: '订单列表',
        component: './Orders/index',
      },
      {
        path: '/orders/:id',
        name: '订单详情',
        hideInMenu: true,
        component: './Orders/Detail',
      },
      {
        path: '/orders/sync-tasks',
        name: '同步任务',
        component: './Orders/SyncTasks',
      },
      {
        path: '/orders/sku-matches',
        name: '规格匹配',
        component: './Orders/SkuMatches',
      },
      {
        path: '/orders/exceptions',
        name: '异常工作台',
        component: './Orders/Exceptions',
      },
    ],
  },
  {
    path: '/inventory',
    name: '库存',
    icon: 'InboxOutlined',
    component: '@/layouts/InventoryGroupLayout',
    routes: [
      {
        path: '/inventory',
        component: './Inventory',
      },
      {
        path: '/inventory/overview',
        name: '库存中心',
        component: './Inventory',
      },
      {
        path: '/inventory/alerts',
        name: '库存预警',
        component: './Inventory/Alerts',
      },
      {
        path: '/inventory/warehouse-ledger',
        name: '仓库库存账',
        component: './Inventory/WarehouseLedger',
      },
      {
        path: '/inventory/warehouse-transfers',
        name: '仓库调拨',
        component: './Inventory/WarehouseTransfers',
      },
      {
        path: '/inventory/stocktakes',
        name: '库存盘点',
        component: './Inventory/Stocktakes',
      },
      {
        path: '/inventory/deductions',
        name: '库存扣减记录',
        component: './Inventory/Deductions',
      },
      {
        path: '/inventory/sync-tasks',
        name: '库存同步任务',
        component: './Inventory/SyncTasks',
      },
      {
        path: '/inventory/sync-batches',
        name: '库存同步批次',
        component: './Inventory/SyncBatches',
      },
      {
        path: '/inventory/effects',
        redirect: '/inventory/deductions',
        hideInMenu: true,
      },
      {
        path: '/inventory/logs',
        name: '库存流水',
        component: './Inventory/Logs',
      },
    ],
  },
  {
    path: '/procurement',
    name: '采购',
    icon: 'ShoppingCartOutlined',
    component: '@/layouts/ProcurementGroupLayout',
    routes: [
      {
        path: '/procurement',
        redirect: '/procurement/purchase-orders',
      },
      {
        path: '/procurement/purchase-orders/:id',
        name: '采购单详情',
        component: './Procurement/PurchaseOrders/Detail',
        hideInMenu: true,
      },
      {
        path: '/procurement/purchase-orders',
        name: '采购单',
        component: './Procurement/PurchaseOrders',
      },
      {
        path: '/procurement/purchase-returns/:id',
        name: '采购退货详情',
        component: './Procurement/PurchaseReturns/Detail',
        hideInMenu: true,
      },
      {
        path: '/procurement/purchase-returns',
        name: '采购退货',
        component: './Procurement/PurchaseReturns',
      },
      {
        path: '/procurement/replenishment-suggestions',
        name: '补货建议',
        component: './Procurement/ReplenishmentSuggestions',
      },
      {
        path: '/procurement/warehouses',
        name: '仓库管理',
        component: './Procurement/Warehouses',
      },
      {
        path: '/procurement/suppliers',
        name: '供应商管理',
        component: './Procurement/Suppliers',
      },
    ],
  },
  {
    path: '/customer',
    name: '客服',
    icon: 'CustomerServiceOutlined',
    component: '@/layouts/CustomerGroupLayout',
    routes: [
      {
        path: '/customer',
        redirect: '/customer/hub',
      },
      {
        path: '/customer/hub',
        name: '客服中心',
        component: './Customer/Hub',
      },
      {
        path: '/customer/conversations',
        name: '会话列表',
        component: './Customer/Conversations',
      },
      {
        path: '/customer/conversations/:id',
        name: 'AI 客服工作台',
        component: './Customer/ConversationDetail',
        hideInMenu: true,
      },
      {
        path: '/customer/message-sync-tasks',
        name: '消息同步任务',
        component: './Customer/MessageSyncTasks',
      },
      {
        path: '/customer/auto-reply-settings',
        name: 'AI 自动回复',
        component: './Customer/AutoReplySettings',
      },
    ],
  },
  {
    path: '/settings',
    name: '设置',
    icon: 'SettingOutlined',
    component: '@/layouts/SettingsGroupLayout',
    routes: [
      {
        path: '/settings/system',
        name: '系统设置',
        component: './Settings/System',
      },
      {
        path: '/settings/security',
        name: '安全设置',
        component: './Settings/Security',
      },
      {
        path: '/settings/email',
        name: '邮箱设置',
        component: './Settings/Email',
      },
      {
        path: '/settings/alert-notify',
        name: '告警通知配置',
        component: './Settings/AlertNotify',
      },
      {
        path: '/settings/storage',
        name: '存储设置',
        component: './Settings/Storage',
      },
      {
        path: '/settings/ai',
        name: 'AI 设置',
        component: './Settings/AI',
      },
      {
        path: '/settings/image',
        name: '图片 AI 设置',
        component: './Settings/Image',
      },
      {
        path: '/settings/collector',
        name: '采集服务',
        component: './Settings/Collector',
      },
      {
        path: '/settings/inventory',
        name: '库存 / 订单',
        component: './Settings/Inventory',
      },
      {
        path: '/settings/pricing',
        name: '商品定价',
        component: './Settings/Pricing',
      },
      {
        path: '/settings/platforms',
        name: '平台接入设置',
        component: './Settings/Platforms',
      },
      {
        path: '/settings/platform-publish',
        name: '平台刊登预设',
        component: './Settings/PlatformPublish',
      },
      {
        path: '/settings/integrations',
        name: '第三方集成总览',
        component: './Settings/Integrations',
      },
      {
        path: '/settings/config-status',
        name: '配置状态中心',
        component: './Settings/ConfigStatus',
      },
      {
        path: '/settings/users',
        name: '用户与权限',
        component: './Settings/Users',
      },
    ],
  },
  {
    path: '*',
    layout: false,
    component: './404',
  },
];
