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
    redirect: '/dashboard',
    access: 'canAdmin',
  },
  {
    path: '/dashboard',
    name: '工作台',
    icon: 'DashboardOutlined',
    component: './Dashboard',
    access: 'canAdmin',
  },
  {
    path: '/system/operation-logs',
    name: '操作日志',
    icon: 'AuditOutlined',
    component: './System/OperationLogs',
    access: 'canAdmin',
  },
  {
    path: '/files',
    name: '文件管理',
    icon: 'FileImageOutlined',
    component: './Files',
    access: 'canAdmin',
  },
  {
    path: '/product',
    name: '商品',
    icon: 'ShoppingOutlined',
    access: 'canAdmin',
    redirect: '/product/drafts',
    routes: [
      {
        path: '/product/drafts',
        name: '商品草稿',
        component: './Product/Drafts',
        access: 'canAdmin',
      },
    ],
  },
  {
    path: '/collect',
    name: '采集',
    icon: 'CloudDownloadOutlined',
    access: 'canAdmin',
    redirect: '/collect/tasks',
    routes: [
      {
        path: '/collect/tasks',
        name: '采集任务',
        component: './Collect/Tasks',
        access: 'canAdmin',
      },
    ],
  },
  {
    path: '/settings',
    name: '设置',
    icon: 'SettingOutlined',
    access: 'canAdmin',
    redirect: '/settings/system',
    routes: [
      {
        path: '/settings/system',
        name: '系统设置',
        component: './Settings/System',
        access: 'canAdmin',
      },
      {
        path: '/settings/ai',
        name: 'AI 设置',
        component: './Settings/AI',
        access: 'canAdmin',
      },
      {
        path: '/settings/storage',
        name: '存储设置',
        component: './Settings/Storage',
        access: 'canAdmin',
      },
      {
        path: '/settings/collector',
        name: '采集服务',
        component: './Settings/Collector',
        access: 'canAdmin',
      },
      {
        path: '/settings/security',
        name: '安全设置',
        component: './Settings/Security',
        access: 'canAdmin',
      },
    ],
  },
  {
    path: '*',
    layout: false,
    component: './404',
  },
];
