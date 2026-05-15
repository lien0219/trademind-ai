/**
 * 后台路由与侧栏菜单（名称即菜单文案）。
 * component 相对 `src/pages/`。
 */
export default [
  {
    path: '/',
    redirect: '/dashboard',
  },
  {
    path: '/dashboard',
    name: '工作台',
    icon: 'DashboardOutlined',
    component: './Dashboard',
  },
  {
    path: '/product',
    name: '商品',
    icon: 'ShoppingOutlined',
    redirect: '/product/drafts',
    routes: [
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
    redirect: '/collect/tasks',
    routes: [
      {
        path: '/collect/tasks',
        name: '采集任务',
        component: './Collect/Tasks',
      },
    ],
  },
  {
    path: '/settings',
    name: '设置',
    icon: 'SettingOutlined',
    redirect: '/settings/system',
    routes: [
      {
        path: '/settings/system',
        name: '系统设置',
        component: './Settings/System',
      },
      {
        path: '/settings/ai',
        name: 'AI 设置',
        component: './Settings/AI',
      },
      {
        path: '/settings/storage',
        name: '存储设置',
        component: './Settings/Storage',
      },
    ],
  },
  {
    path: '*',
    layout: false,
    component: './404',
  },
];
