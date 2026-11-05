import { describe, expect, it } from 'vitest';
import { hasTranslation, supportedLanguages, translateMessage } from '../dictionaries';
import { sharedCopyTranslations } from '../sharedCopyTranslations';

const settingsUiCopy = [
  '关闭后不再生成新的库存预警记录',
  '本地库存为 0 或低于安全线时提示',
  '平台侧库存与本地库存差异超过阈值时提示',
  '入库后按平台规格编码匹配本地规格；失败不影响订单同步',
  '默认关闭；需同时开启「平台订单自动扣库存」且订单行已绑定本地规格',
  '与新建订单弹窗内的「创建后扣库存」任一为真则生效',
  '平台同步订单到达后，按策略尝试扣减本地库存',
  '订单取消或关闭时尝试恢复已扣减的本地库存',
  '本地扣减成功后排队同步到各平台（需商品已刊登且开启库存同步）',
  '已有成功扣库记录时，仍允许人工绑定未匹配行并再扣',
  '开启后本地库存可扣成负数（谨慎使用）',
  '店铺授权失效、权限不足等场景自动生成站内告警',
  '开放平台应用参数缺失或校验未通过时告警',
  '库存同步因规格映射缺失失败时告警',
  '任务租约过期、后台任务进程长时间未续租时告警',
  '该链接属于 1688 平台，请使用「1688 采集器」。1688 采集器已针对商品标题、主图、详情图、商品参数、商品规格做专门适配，识别更稳定。',
  '该链接属于 AliExpress 平台，请使用「速卖通采集器」。专用采集器字段识别更稳定。',
  '该链接属于淘宝/天猫平台，请使用「淘宝/天猫采集器」。专用采集器字段识别更稳定，请勿使用自定义链接采集器。',
  '该链接属于拼多多平台，请使用「拼多多采集器」。专用采集器字段识别更稳定。',
  '该链接属于暂未开放专用采集器的平台。请先创建采集规则并测试采集效果，再开始采集。',
  '去使用 1688 采集器',
  '去使用速卖通采集器',
  '去使用淘宝/天猫采集器',
  '去使用拼多多采集器',
  '我知道了',
  '适合采集没有专用采集器的网站商品页，可采集商品标题、价格、图片、参数等基础信息。',
  '商品规格、库存、动态价格不保证完整。使用前建议先测试采集规则。已支持的平台请优先使用专用采集器；自定义链接批量采集暂未开放。',
  '专用采集器适合采集完整商品数据（含商品规格与库存）；已针对平台页面结构专门适配，识别更稳定。',
  '适合采集完整商品数据（含规格、库存等），已针对平台专门适配。',
  '适合采集页面基础信息（标题、价格、图片、参数）；SKU / 库存 / 动态价格不保证完整。',
  '已支持的平台请优先使用专用采集器；自定义链接批量采集暂未开放。',
  '查看更多（还有 {count} 条）',
  '去配置',
  '查看说明',
  '去配置平台凭证',
  '去配置存储',
  '查看公网访问说明',
  '库存：',
  '库存',
  '绑定',
  '存储设置',
  '未命名商品',
  '面单条码与运单号不一致',
  '履约对账详情',
  '已保存 {count} 项扩展字段',
  '未配置扩展项',
  '当前未配置抖店真实凭证，只能完成本地配置检查，不能调用真实平台。',
  '抖店接入仍需人工验收；创建平台草稿不等于商品上架。',
] as const;

describe('settings UI copy translations', () => {
  it('provides complete, non-empty translations for every supported locale', () => {
    for (const source of settingsUiCopy) {
      const translations = sharedCopyTranslations[source];
      if (translations) {
        expect(translations, source).toHaveLength(5);
        expect(translations.every((value) => value.trim() !== ''), source).toBe(true);
      }
      for (const language of supportedLanguages) {
        expect(hasTranslation(language, source), `${language}: ${source}`).toBe(true);
        expect(translateMessage(language, source).trim(), `${language}: ${source}`).not.toBe('');
        const placeholders = (value: string) => [...value.matchAll(/\{\w+\}/g)].map(([name]) => name).sort();
        expect(placeholders(translateMessage(language, source)), `${language}: ${source}`).toEqual(placeholders(source));
      }
    }
  });
});
