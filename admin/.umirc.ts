import { defineConfig } from '@umijs/max';
import routes from './config/routes';

export default defineConfig({
  title: '贸灵 TradeMind',
  npmClient: 'npm',
  antd: {
    appConfig: {},
    configProvider: {
      theme: {
        cssVar: true,
        token: {
          colorPrimary: '#2563eb',
          colorInfo: '#0891b2',
          borderRadius: 8,
          fontFamily:
            "system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, 'PingFang SC', 'Microsoft YaHei', sans-serif",
        },
        components: {
          Layout: {
            bodyBg: '#f4f6f9',
            headerBg: '#ffffff',
            footerBg: '#f4f6f9',
            siderBg: '#ffffff',
          },
          Menu: {
            itemBorderRadius: 8,
            iconSize: 16,
            collapsedIconSize: 16,
          },
          Card: {
            headerFontSize: 15,
          },
        },
      },
    },
  },
  access: {},
  model: {},
  initialState: {},
  request: {},
  layout: {
    /** 侧栏/顶栏品牌仅在 `app.tsx` 的 `logo` 中渲染，此处不设 title，避免与 logo 内文案重复 */
    title: false,
    locale: false,
    layout: 'mix',
    navTheme: 'light',
    fixedHeader: true,
    fixSiderbar: true,
    contentWidth: 'Fluid',
  },
  routes,
  proxy: {
    '/api': {
      target: 'http://127.0.0.1:8080',
      changeOrigin: true,
    },
    '/static': {
      target: 'http://127.0.0.1:8080',
      changeOrigin: true,
    },
  },
});
