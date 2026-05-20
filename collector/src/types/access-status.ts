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
  detailImagesCount: number;
  attributesCount: number;
};

export type CustomAccessReport = {
  accessStatus: AccessStatus;
  finalUrl: string;
  httpStatus?: number;
  extractedFields: ExtractedFieldsSummary;
  missingFields: string[];
  warnings: string[];
  errorCode?: string;
  suggestion: string;
  /** Reserved for future logged-in custom profiles */
  customUseBrowserProfile?: boolean;
  customBrowserProfileName?: string;
  customCookieProfileId?: string;
};
