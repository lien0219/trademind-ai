/** Generic page access state for custom link collector (not platform-specific login). */
export type AccessStatus =
  | 'public'
  | 'login_required'
  | 'verify_required'
  | 'blocked'
  | 'timeout'
  | 'navigation_failed'
  | 'unknown';

export type ExtractedFieldsSummary = {
  title: boolean;
  price: boolean;
  mainImage: boolean;
  mainImagesCount?: number;
  detailImagesCount: number;
  attributesCount: number;
  /** Rule test: matched title text */
  titleText?: string;
  /** Rule test: selector that matched title */
  titleSelector?: string;
  /** Rule test: high | medium | low */
  titleConfidence?: string;
  titleSuspectWrong?: boolean;
  attributeSamples?: { key: string; value: string }[];
};

export type QualityScoreSummary = {
  titleOk: boolean;
  priceOk: boolean;
  mainImagesOk: boolean;
  descriptionImagesOk: boolean;
  attributesOk: boolean;
  skuSupported: boolean;
  score: number;
  hints: string[];
};

export type CustomAccessReport = {
  accessStatus: AccessStatus;
  finalUrl: string;
  httpStatus?: number;
  extractedFields: ExtractedFieldsSummary;
  missingFields: string[];
  warnings: string[];
  qualityScore?: QualityScoreSummary;
  errorCode?: string;
  suggestion: string;
  /** Reserved for future logged-in custom profiles */
  customUseBrowserProfile?: boolean;
  customBrowserProfileName?: string;
  customCookieProfileId?: string;
};
