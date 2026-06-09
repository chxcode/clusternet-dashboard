import { describe, expect, it, beforeEach } from 'vitest';
import { apiPath, normalizeBasePath } from './basePath';

describe('normalizeBasePath', () => {
  it('normalizes empty and root to empty prefix', () => {
    expect(normalizeBasePath()).toBe('');
    expect(normalizeBasePath('/')).toBe('');
  });

  it('adds leading slash and removes trailing slash', () => {
    expect(normalizeBasePath('clusternet/')).toBe('/clusternet');
  });
});

describe('apiPath', () => {
  beforeEach(() => {
    window.__CLUSTERNET_DASHBOARD_BASE_PATH__ = '/clusternet';
  });

  it('prefixes dashboard API calls with the configured base path', () => {
    expect(apiPath('/health')).toBe('/clusternet/api/health');
  });
});
