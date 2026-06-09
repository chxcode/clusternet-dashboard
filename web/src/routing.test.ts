import { describe, expect, it } from 'vitest';
import { pageFromPath, pathForPage } from './routing';

describe('dashboard routing', () => {
  it('maps base-path URLs to dashboard pages', () => {
    expect(pageFromPath('/clusternet/', '/clusternet')).toBe('overview');
    expect(pageFromPath('/clusternet/clusters', '/clusternet')).toBe('managed-clusters');
    expect(pageFromPath('/clusternet/subscriptions', '/clusternet')).toBe('subscriptions');
    expect(pageFromPath('/clusternet/propagation', '/clusternet')).toBe('propagation-status');
    expect(pageFromPath('/clusternet/diagnostics', '/clusternet')).toBe('diagnostics');
  });

  it('builds base-path URLs for dashboard pages', () => {
    expect(pathForPage('overview', '/clusternet')).toBe('/clusternet/overview');
    expect(pathForPage('managed-clusters', '/clusternet')).toBe('/clusternet/clusters');
    expect(pathForPage('subscriptions', '/clusternet')).toBe('/clusternet/subscriptions');
    expect(pathForPage('propagation-status', '/clusternet')).toBe('/clusternet/propagation');
    expect(pathForPage('diagnostics', '/clusternet')).toBe('/clusternet/diagnostics');
  });
});
