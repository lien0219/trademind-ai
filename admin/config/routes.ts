/**
 * 后台路由与侧栏菜单（名称即菜单文案）。
 * component 相对 `src/pages/`。
 */
export default [
  {
    path: '/user/login',
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
        name: '商品运营看板',
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
        name: 'AI 批次（旧版）',
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
        path: '/product/ai-image-batches/:id',
        name: 'AI 图片批次复核',
        component: './Product/AIImageBatchDetail',
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
        name: '库存中心',
        component: './Inventory',
      },
      {
        path: '/inventory/alerts',
        name: '库存预警',
        component: './Inventory/Alerts',
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
    ],
  },
  {
    path: '*',
    layout: false,
    component: './404',
  },
];
