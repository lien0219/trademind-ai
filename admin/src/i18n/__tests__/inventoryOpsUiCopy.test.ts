import { describe, expect, it } from 'vitest';
import { execFileSync } from 'node:child_process';
import { hasTranslation, supportedLanguages, translateMessage } from '../dictionaries';
import { trFormat } from '../index';

describe('Inventory and Ops UI copy', () => {
  it('has no untranslated user-facing Chinese candidates in either module', () => {
    const result = JSON.parse(execFileSync(
      process.execPath,
      ['scripts/i18n-audit.mjs', '--details'],
      { encoding: 'utf8' },
    )) as { candidates: Array<{ file: string; line: number; sample: string }> };
    const candidates = result.candidates.filter(({ file }) =>
      /^pages\/(?:Inventory|Ops)\//.test(file),
    );

    expect(candidates).toEqual([]);
  });

  it('keeps migrated fallback labels available in all target locales', () => {
    const sources = [
      '调整库存',
      '预警线',
      '计算中…',
      '每次最多迁移 100 个尚未建立仓库余额的规格。优先使用启用的默认仓；没有默认仓时写入“待分配仓”。该操作不调用平台接口，也不创建后台任务进程。',
      '盘点详情',
      '详情',
      '未命名商品',
      '编辑 SKU 绑定',
      '新增 SKU 绑定',
      '调拨详情',
      '正在加载绑定历史',
      '暂无校准说明',
      '确认人工绑定',
      '拒绝人工绑定',
      '确认绑定',
      '确认拒绝',
      '正在加载库存同步运行',
      '未知',
      '内部端点已保护',
      '端点未限制内部访问',
      '未配置路径',
      '未配置协议',
      '店铺授权异常',
      '等待状态数据',
      '尚未完成健康检查',
      '已开启',
      '已关闭',
      '最近检查：{value0}',
      '数据暂不可用',
      '当前仓库尚无 SKU 绑定',
      '{value0}：{value1} 件，已收 {value2}',
    ];

    for (const language of supportedLanguages) {
      for (const source of sources) {
        expect(hasTranslation(language, source), `${language}: ${source}`).toBe(true);
        // Shared Japanese/Chinese glyphs can be valid localized text.
        if (['en-US', 'ko-KR', 'th-TH'].includes(language)) {
          expect(translateMessage(language, source), `${language}: ${source}`).not.toBe(source);
        }
      }
    }
  });

  it('preserves the checked-at value when formatting the localized label', () => {
    const checkedAt = '2026-10-28 09:00';
    for (const language of supportedLanguages) {
      const result = trFormat('最近检查：{value0}', { value0: checkedAt }, language);
      expect(result).toContain(checkedAt);
      expect(result).not.toContain('{value0}');
    }
  });
});
