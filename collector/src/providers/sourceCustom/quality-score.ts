import type { NormalizedProduct } from '../../types/product.js';
import type { TitleCandidate } from './title-quality.js';
import { TITLE_SUSPECT_HINT } from './title-quality.js';

export type QualityScore = {
  titleOk: boolean;
  priceOk: boolean;
  mainImagesOk: boolean;
  descriptionImagesOk: boolean;
  attributesOk: boolean;
  skuSupported: boolean;
  score: number;
  hints: string[];
};

export const SKU_LIMITATION_HINT =
  '商品规格、库存、实时价格通常由网站动态加载，自定义采集规则不一定能完整识别。如需稳定采集该平台规格和库存，建议后续使用专用采集器。';

export const DESCRIPTION_IMAGES_EMPTY_HINT =
  '详情图片可能需要滚动加载或网站专用接口，自定义规则未能识别。';

export function buildQualityScore(
  product: NormalizedProduct | undefined,
  titleDiag?: TitleCandidate,
): QualityScore {
  const hints: string[] = [];
  let score = 0;

  const title = product?.title?.trim() ?? '';
  const titleOk = !!title && !(titleDiag?.suspectWrongTitle ?? false);
  if (titleOk) score += 25;
  else if (title) {
    hints.push(TITLE_SUSPECT_HINT);
  } else {
    hints.push('未识别商品标题，请调整标题规则。');
  }

  const priceVal = product?.raw?.productPrice;
  const priceOk = typeof priceVal === 'number' && priceVal > 0;
  if (priceOk) score += 20;
  else hints.push('价格未提取，请检查价格规则或该站是否为动态价格。');

  const mainN = product?.mainImages?.length ?? 0;
  const mainImagesOk = mainN >= 2;
  if (mainN > 0) score += mainN >= 2 ? 20 : 10;
  else hints.push('未识别商品主图。');
  if (mainN === 1) hints.push('主图仅识别 1 张，轮播图可能未抓全，建议检查主图区域规则。');

  const descN = product?.descriptionImages?.length ?? 0;
  const descriptionImagesOk = descN > 0;
  if (descriptionImagesOk) score += 15;
  else hints.push(DESCRIPTION_IMAGES_EMPTY_HINT);

  const attrN = product?.attributes ? Object.keys(product.attributes).length : 0;
  const attributesOk = attrN > 0;
  if (attributesOk) score += 10;
  else hints.push('商品参数未识别，可补充 attributes 规则或手动填写。');

  const skuN = product?.skus?.length ?? 0;
  const skuSupported = skuN > 0;
  if (!skuSupported) hints.push(SKU_LIMITATION_HINT);

  if (titleDiag?.hint && titleDiag.suspectWrongTitle) {
    hints.unshift(titleDiag.hint);
  }

  return {
    titleOk,
    priceOk,
    mainImagesOk,
    descriptionImagesOk,
    attributesOk,
    skuSupported,
    score: Math.min(100, score),
    hints: [...new Set(hints)],
  };
}
