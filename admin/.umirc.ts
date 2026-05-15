import { defineConfig } from '@umijs/max';
import routes from './config/routes';

export default defineConfig({
  title: '贸灵 TradeMind',
  npmClient: 'npm',
  antd: {
    configProvider: {
      theme: {
        token: {
          colorPrimary: '#1677ff',
          borderRadius: 6,
        },
      },
    },
  },
  access: {},
  model: {},
  initialState: {},
  request: {},
  layout: {
    title: '贸灵 TradeMind',
    locale: false,
  },
  routes,
  proxy: {
    '/api': {
      target: 'http://127.0.0.1:8080',
      changeOrigin: true,
    },
  },
});
