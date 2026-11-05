import { describe, expect, it } from 'vitest';
import { hasTranslation, supportedLanguages, translateMessage } from '../dictionaries';

const procurementUiCopy = [
  '采购单详情',
  '采购单操作',
  '确认',
  '未命名规格',
  '采购退货详情',
  '采购退货操作',
  '确认后会在同一事务中扣减仓库可用库存并写入不可变流水；审批人不能执行本次退货。',
  '未命名商品',
  '选择供应商后显示',
  '正在加载供应商信息',
  '联系方式按当前权限完整展示',
  '联系方式按当前权限脱敏展示',
  '新建供应商',
  '保存供应商',
  '创建供应商',
  '原号码已脱敏；留空将保留原值，填写新号码才会替换。',
  '原邮箱已脱敏；留空将保留原值，填写新邮箱才会替换。',
  '供应商采购信息',
  '编辑规格采购信息',
  '关联商品规格',
  '保存采购信息',
  '确认关联',
  '正在加载仓库信息',
  '新建仓库',
  '保存仓库',
  '创建仓库',
  '· 未收',
  '· 收货单',
  '· 可退',
  '默认仓：',
  '尚未设置',
  '仓库列表暂不可用',
  '暂无仓库，请先新建仓库。',
  '供应商列表暂不可用',
  '暂无供应商，请先新建供应商。',
  '采购信息暂不可用',
  '尚未关联商品规格。',
  '采购单列表暂不可用',
  '暂无采购单。',
  '采购退货列表暂不可用',
  '暂无采购退货记录。',
  '补货建议暂不可用',
  '暂无符合条件的规格',
  '请选择目标仓库',
  '采购单没有明细。',
  '退货单没有明细。',
];

describe('Procurement UI copy', () => {
  it('resolves every migrated message in all six supported languages', () => {
    expect(supportedLanguages).toHaveLength(6);
    for (const source of procurementUiCopy) {
      for (const language of supportedLanguages) {
        if (language === 'zh-CN') continue;
        expect(hasTranslation(language, source), `${language}: ${source}`).toBe(true);
        expect(translateMessage(language, source).trim(), `${language}: ${source}`).not.toBe('');
        // "未命名商品" is also correct Traditional Chinese.
        if (['en-US', 'ko-KR', 'th-TH'].includes(language)) {
          expect(translateMessage(language, source), `${language}: ${source}`).not.toBe(source);
        }
      }
    }
  });
});
