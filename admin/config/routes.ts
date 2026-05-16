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
    component: './Dashboard',
  },
  {
    path: '/system/operation-logs',
    name: '操作日志',
    icon: 'AuditOutlined',
    component: './System/OperationLogs',
  },
  {
    path: '/workers/monitor',
    name: 'Worker 监控',
    icon: 'CloudServerOutlined',
    component: './Workers/Monitor',
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
        name: 'Prompt 模板',
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
        name: '采集中心',
        component: './Collect/Hub',
      },
      {
        path: '/collect/rules',
        name: '采集规则',
        component: './Collect/Rules',
      },
      {
        path: '/collect/batches',
        name: '批量采集',
        component: './Collect/Batches',
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
        name: '订单列表',
        component: './Orders/index',
      },
      {
        path: '/orders/sync-tasks',
        name: '同步任务',
        component: './Orders/SyncTasks',
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
    ],
  },
  {
    path: '/settings',
    name: '设置',
    icon: 'SettingOutlined',
    component: '@/layouts/SettingsGroupLayout',
    routes: [
      {
        path: '/settings/integrations',
        name: '第三方集成总览',
        component: './Settings/Integrations',
      },
      {
        path: '/settings/system',
        name: '系统设置',
        component: './Settings/System',
      },
      {
        path: '/settings/email',
        name: '邮箱设置',
        component: './Settings/Email',
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
        path: '/settings/storage',
        name: '存储设置',
        component: './Settings/Storage',
      },
      {
        path: '/settings/platforms',
        name: '平台开放配置',
        component: './Settings/Platforms',
      },
      {
        path: '/settings/collector',
        name: '采集服务',
        component: './Settings/Collector',
      },
      {
        path: '/settings/security',
        name: '安全设置',
        component: './Settings/Security',
      },
    ],
  },
  {
    path: '*',
    layout: false,
    component: './404',
  },
];
