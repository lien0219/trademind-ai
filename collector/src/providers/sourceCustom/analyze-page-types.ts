import type { AccessStatus } from '../../types/access-status.js';

export type DigestCandidate = {
  selector: string;
  sample?: string;
  attr?: string;
  count: number;
  confidence: number;
};

export type DigestImageSample = {
  selector: string;
  srcSample: string;
  naturalWidth?: number;
  naturalHeight?: number;
  count: number;
};

export type PageStructureDigest = {
  url: string;
  finalUrl: string;
  accessStatus: AccessStatus;
  title: string;
  meta: {
    title: string;
    description: string;
    ogTitle: string;
    ogImage: string;
  };
  candidates: {
    title: DigestCandidate[];
    price: DigestCandidate[];
    mainImages: DigestCandidate[];
    descriptionImages: DigestCandidate[];
    attributes: DigestCandidate[];
    sku: DigestCandidate[];
  };
  domHints: string[];
  textSamples: string[];
  imageSamples: DigestImageSample[];
};

export type AnalyzePageOptions = {
  profileKey?: string;
  useBrowserProfile?: boolean;
  maxCandidates?: number;
};
