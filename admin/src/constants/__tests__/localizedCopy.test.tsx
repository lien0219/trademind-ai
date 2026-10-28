import { act, cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { setLanguage } from '@/i18n';
import { supportedLanguages, translateMessage } from '@/i18n/dictionaries';
import { confirmApplyAiImage, confirmChangeUserRole, confirmFailureTaskRetry } from '../sensitiveActions';
import { platformDisplayLabel, shopAuthStatusLabel, productFieldLabel } from '../platformLabels';
import { productSourceLabel, platformLabel } from '../userFriendly';
import { storageConnectionSectionTitle } from '../storageSettings';
import { imageSubServiceStatusLabel } from '../integrations';
import { pinduoduoUrlHint } from '@/utils/pinduoduoUrl';
import { taobaoTmallUrlHint } from '@/utils/taobaoTmallUrl';
import { COLLECT_HUB_TYPE_HINT } from '@/utils/customCollectPlatform';
import { Modal } from 'antd';
import { ACTION_COPY, LIST_EMPTY_COPY, PAGE_COPY, commonStatusLabel } from '../copywriting';
import { ORDER_STATUS, collectTaskEventLabel, collectTaskStatusTransition } from '../status';
import StatusTag from '@/components/ui/StatusTag';
import { INVENTORY_STOCK_STATUS } from '../inventoryLabels';
import { formatUserErrorMessage, mapErrorCodeToUserMessage, sanitizeErrorForDisplay } from '../errorMessages';
import { ROLE_OPTIONS } from '../permissionLabels';
import { findCollectSettingsOption } from '@/utils/collectSettingsProvider';

afterEach(() => { cleanup(); setLanguage('zh-CN'); });

describe('shared display labels', () => {
  it('localizes known error guidance while preserving unknown diagnostic values', () => {
    for (const language of supportedLanguages.slice(1)) {
      setLanguage(language);
      const login = mapErrorCodeToUserMessage('AUTH_INVALID_CREDENTIALS');
      expect(login?.detail).not.toBe('请检查邮箱、手机号和密码后再试。');
      expect(formatUserErrorMessage('AUTH_INVALID_CREDENTIALS')).toContain(login?.detail);
      expect(mapErrorCodeToUserMessage('platform_auth')?.title).toBe(translateMessage(language, '平台授权失败'));
      expect(formatUserErrorMessage()).toBe(translateMessage(language, '操作失败，请稍后重试'));
      expect(sanitizeErrorForDisplay('vendor.trace.原始值')).toBe('vendor.trace.原始值');
    }
  });
  it('resolves inventory, role, error and collector labels in the current language', () => {
    const inventory = INVENTORY_STOCK_STATUS.normal;
    for (const language of [...supportedLanguages.slice(1), 'zh-CN'] as const) {
      setLanguage(language);
      expect(inventory.text).toBe(translateMessage(language, '正常'));
      expect(ROLE_OPTIONS[0].label).toBe(translateMessage(language, '管理员'));
      expect(mapErrorCodeToUserMessage('AUTH_INVALID_CREDENTIALS')?.title)
        .toBe(translateMessage(language, '账号或密码不正确'));
      expect(findCollectSettingsOption('pinduoduo').label)
        .toBe(translateMessage(language, '拼多多采集器'));
    }
  });
  it('reads the active locale after module import and preserves backend status keys', () => {
    const keys = Object.keys(ORDER_STATUS);
    for (const language of supportedLanguages.slice(1)) {
      setLanguage(language);
      expect(ORDER_STATUS.shipped.text).not.toBe('已发货');
      expect(commonStatusLabel('pending')).not.toBe('等待处理');
      expect(collectTaskEventLabel('task.running')).not.toBe('任务执行中');
      expect(collectTaskStatusTransition('pending', 'running')).not.toMatch(/等待处理|处理中/);
      expect(PAGE_COPY.shops.description).not.toContain('授权并管理');
      expect(LIST_EMPTY_COPY.productDrafts.title).not.toBe('暂无商品草稿');
      expect(ACTION_COPY.saveSettings).not.toBe('保存设置');
      expect(Object.keys(ORDER_STATUS)).toEqual(keys);
      expect(commonStatusLabel('vendor_status_原始值')).toBe('vendor_status_原始值');
      expect(collectTaskEventLabel('vendor.event.原始值')).toBe('vendor.event.原始值');
    }
  });

  it('localizes shared helper output across all supported languages and preserves unknown values', () => {
    const outputs = new Set<string>();
    for (const language of supportedLanguages.slice(1)) {
      setLanguage(language);
      outputs.add(platformDisplayLabel('douyin_shop'));
      expect(platformDisplayLabel('douyin_shop')).toBe(translateMessage(language, '抖店'));
      expect(platformLabel('douyin_shop')).toBe(translateMessage(language, '抖店'));
      expect(productSourceLabel('pinduoduo')).toBe(translateMessage(language, '拼多多'));
      expect(shopAuthStatusLabel('unauthorized')).toBe(translateMessage(language, '未授权'));
      expect(productFieldLabel('external links')).toBe(translateMessage(language, '外链图片'));
      expect(storageConnectionSectionTitle('local')).toBe(translateMessage(language, '本地磁盘配置'));
      expect(imageSubServiceStatusLabel(false, 'key')).toBe(translateMessage(language, '未填密钥'));
      expect(COLLECT_HUB_TYPE_HINT.dedicated.title).toBe(translateMessage(language, '专用采集器'));
      expect(COLLECT_HUB_TYPE_HINT.custom.summary).toBe(translateMessage(language, '适合采集页面基础信息（标题、价格、图片、参数）；SKU / 库存 / 动态价格不保证完整。'));
      expect(pinduoduoUrlHint('https://mobile.yangkeduo.com/goods.html?goods_id=1')).toBe(translateMessage(language, '已识别为移动端商品页。当前版本优先支持批发详情页（pifa.pinduoduo.com/goods/detail），请换用批发链接。'));
      expect(taobaoTmallUrlHint('https://item.taobao.com/item.htm?id=1')).toBe(translateMessage(language, '已识别为淘宝商品详情页'));
      expect(platformDisplayLabel('vendor_platform')).toBe('vendor_platform');
      expect(productSourceLabel('supplier_value')).toBe('supplier_value');
    }
    expect(outputs.size).toBeGreaterThan(1);
  });

  it('localizes sensitive-action sentence variants, fallback fragments, and impact labels', () => {
    const confirm = vi.spyOn(Modal, 'confirm').mockReturnValue({ update: vi.fn(), destroy: vi.fn() });
    for (const language of supportedLanguages.slice(1)) {
      setLanguage(language);
      confirmApplyAiImage('replace', vi.fn());
      const replacement = confirm.mock.calls.at(-1)?.[0];
      expect((replacement?.content as { props: { children: string } }).props.children).not.toContain('替换现有主图');
      replacement?.afterClose?.();

      confirmApplyAiImage('append', vi.fn());
      const append = confirm.mock.calls.at(-1)?.[0];
      expect((append?.content as { props: { children: string } }).props.children).not.toContain('追加到商品图片');
      append?.afterClose?.();

      confirmFailureTaskRetry(1, vi.fn());
      const retry = confirm.mock.calls.at(-1)?.[0];
      expect((retry?.content as { props: { children: string } }).props.children).not.toContain('该失败任务');
      retry?.afterClose?.();

      confirmChangeUserRole('user-1', '运营', vi.fn());
      const role = confirm.mock.calls.at(-1)?.[0];
      expect((role?.content as { props: { children: string } }).props.children).not.toContain('用户权限范围');
      expect((role?.content as { props: { children: string } }).props.children).not.toContain('菜单可见性');
      role?.afterClose?.();
    }
  });

  it('updates a mounted status tag and preserves explicit caller text', () => {
    render(<><StatusTag status="pending" /><StatusTag status="pending" text="原始状态/Store-42" /></>);
    expect(screen.getByText('等待处理')).toBeTruthy();
    act(() => setLanguage('en-US'));
    expect(screen.queryByText('等待处理')).toBeNull();
    expect(screen.getByText('原始状态/Store-42')).toBeTruthy();
  });
});
