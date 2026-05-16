import type { ProductSku } from '../../types/product.js';

export type AeBrowserMeta = {
  ogTitle?: string;
  ogImage?: string;
  ogDescription?: string;
  twitterTitle?: string;
  priceCurrency?: string;
};

export type AeBrowserPayload = {
  finalUrl: string;
  docTitle: string;
  meta: AeBrowserMeta;
  headingText: string;
  galleryUrls: string[];
  detailUrls: string[];
  paramPairs: Array<{ key: string; value: string }>;
  scriptSnippets: string[];
};

export type AeAssembleOutput = {
  title: string;
  currency: string;
  mainImages: string[];
  descriptionImages: string[];
  attributes: Record<string, string>;
  skus: ProductSku[];
  rawShell: Omit<AeRawShellBase, 'stateDigest'>;
  blocked: boolean;
  stateDigest: Record<string, unknown>;
};

/** Top-level raw *additions* before merge with standard keys */
export type AeRawShellBase = {
  title: string | unknown;
  url: string;
  mainImageCandidates: string[];
  detailImageCandidates: string[];
  attributeCandidates: Record<string, string>;
  skuCandidates: unknown[];
  pageMeta: Record<string, unknown>;
  stateDigest: Record<string, unknown>;
  extractedAt: string;
};
