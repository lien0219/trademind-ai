import { describe, expect, it } from 'vitest';
import contract from '../../../../tests/contracts/collector-providers.json';
import { listProviderPublicMetas } from '../registry.js';

describe('Collector provider catalogue contract', () => {
  it('matches the canonical cross-service provider metadata', () => {
    expect(listProviderPublicMetas()).toEqual(contract);
  });
});
