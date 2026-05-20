export type TitleConfidence = 'high' | 'medium' | 'low';

export type TitleCandidate = {
  text: string;
  source: 'selector' | 'fallback' | 'jsonLd' | 'openGraph' | 'documentTitle';
  selector?: string;
  confidence: TitleConfidence;
  suspectWrongTitle: boolean;
  hint?: string;
};

const BAD_TITLE_KEYWORDS = [
  '计算器',
  '登录',
  '欢迎登录',
  '购物车',
  '客服',
  '请登录',
  '扫码登录',
  '最小单价',
  '首页',
  '网站首页',
];

const PRODUCT_TITLE_HINTS = [
  'sku-name',
  'product-title',
  'item-title',
  'goods-name',
  'p-name',
  'sku-name',
  'itemInfo-wrap',
  'product-intro',
  'tb-detail',
  'detail-title',
];

function scoreTitleText(text: string, source: TitleCandidate['source'], selector?: string): TitleCandidate {
  const t = text.trim();
  let confidence: TitleConfidence = 'medium';
  let suspectWrongTitle = false;
  const hints: string[] = [];

  if (t.length < 4) {
    confidence = 'low';
    suspectWrongTitle = true;
    hints.push('标题过短');
  }

  for (const kw of BAD_TITLE_KEYWORDS) {
    if (t.includes(kw)) {
      confidence = 'low';
      suspectWrongTitle = true;
      hints.push(`含非商品词「${kw}」`);
      break;
    }
  }

  if (selector) {
    const selLow = selector.toLowerCase();
    if (PRODUCT_TITLE_HINTS.some((h) => selLow.includes(h))) {
      if (!suspectWrongTitle) confidence = 'high';
    }
    if (selLow.includes('h1') || selLow.includes('product') || selLow.includes('sku-name')) {
      if (!suspectWrongTitle && confidence !== 'low') confidence = 'high';
    }
    if (selLow === 'title' || selLow.includes('document')) {
      if (confidence === 'medium') confidence = 'low';
      suspectWrongTitle = true;
      hints.push('可能来自页面 document.title');
    }
  }

  if (source === 'jsonLd' || source === 'openGraph') {
    if (!suspectWrongTitle) confidence = confidence === 'low' ? 'medium' : 'high';
  }

  if (source === 'documentTitle' && !suspectWrongTitle) {
    confidence = 'low';
    suspectWrongTitle = true;
    hints.push('来自浏览器 document.title');
  }

  return {
    text: t,
    source,
    selector,
    confidence,
    suspectWrongTitle,
    hint: hints.length ? hints.join('；') : undefined,
  };
}

export function evaluateTitleCandidate(
  text: string,
  source: TitleCandidate['source'],
  selector?: string,
): TitleCandidate {
  return scoreTitleText(text, source, selector);
}

export function pickBestTitle(candidates: TitleCandidate[]): TitleCandidate | undefined {
  const valid = candidates.filter((c) => c.text.trim());
  if (!valid.length) return undefined;

  const rank = (c: TitleCandidate): number => {
    let s = 0;
    if (c.confidence === 'high') s += 30;
    else if (c.confidence === 'medium') s += 15;
    if (c.source === 'selector') s += 25;
    else if (c.source === 'jsonLd') s += 20;
    else if (c.source === 'openGraph') s += 18;
    else if (c.source === 'fallback') s += 10;
    if (c.suspectWrongTitle) s -= 40;
    s += Math.min(c.text.length, 80);
    return s;
  };

  return [...valid].sort((a, b) => rank(b) - rank(a))[0];
}

export const TITLE_SUSPECT_HINT =
  '当前标题可能不是商品标题，请调整商品标题对应的页面位置，或重新使用 AI 生成规则。';
