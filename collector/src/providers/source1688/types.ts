import type { NormalizedProduct, ProductSku } from '../../types/product.js';

/** 浏览器端从 DOM 抽到的规格维度 */
export type DomSkuDimension = {
  name: string;
  values: string[];
};

/** 浏览器端从尺码表抽到的行（含价/库存文案） */
export type DomSkuTableRow = {
  label: string;
  priceText?: string;
  stockText?: string;
};

/** 浏览器端 evaluate 返回的原始抽取结果（可序列化） */
export type BrowserExtractPayload = {
  finalUrl: string;
  docTitle: string;
  meta: {
    description?: string;
    ogTitle?: string;
    ogImage?: string;
    keywords?: string;
  };
  headingText: string;
  galleryUrls: string[];
  detailUrls: string[];
  paramPairs: Array<{ key: string; value: string }>;
  domSkuDimensions: DomSkuDimension[];
  domSkuTableRows: DomSkuTableRow[];
  /** 可能含 JSON 的 script 片段（已截断，供 Node 侧再解析） */
  scriptSnippets: string[];
};

export type Parse1688Result = Pick<
  NormalizedProduct,
  'title' | 'mainImages' | 'descriptionImages' | 'attributes' | 'skus'
> & {
  raw: Record<string, unknown>;
};

export type { ProductSku };
