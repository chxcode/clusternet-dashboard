import { afterEach, describe, expect, it, vi } from 'vitest';
import { fetchClusters, fetchHelmReleases, fetchManifests, fetchSubscriptions } from './api';

describe('fetchClusters', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('loads clusters from dashboard API', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
      clusters: [{ name: 'child-a', status: 'Online', online: true }],
    }))));
    window.__CLUSTERNET_DASHBOARD_BASE_PATH__ = '/clusternet';

    const clusters = await fetchClusters();

    expect(fetch).toHaveBeenCalledWith('/clusternet/api/clusters');
    expect(clusters).toEqual([{ name: 'child-a', status: 'Online', online: true }]);
  });

  it('loads subscriptions from dashboard API', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
      subscriptions: [{ name: 'demo-sub', status: 'Pending', desiredReleases: 0, completedReleases: 0, observedReleases: 0, completionKnown: false, bindingClusterCount: 0, bindingClusters: [] }],
    }))));
    window.__CLUSTERNET_DASHBOARD_BASE_PATH__ = '/clusternet';

    const subscriptions = await fetchSubscriptions();

    expect(fetch).toHaveBeenCalledWith('/clusternet/api/subscriptions');
    expect(subscriptions[0].name).toBe('demo-sub');
  });

  it('loads manifests from dashboard API', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
      manifests: [{ name: 'demo-manifest', templateKind: 'Deployment' }],
    }))));
    window.__CLUSTERNET_DASHBOARD_BASE_PATH__ = '/clusternet';

    const manifests = await fetchManifests();

    expect(fetch).toHaveBeenCalledWith('/clusternet/api/manifests');
    expect(manifests[0].templateKind).toBe('Deployment');
  });

  it('loads helm releases from dashboard API', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
      helmReleases: [{ name: 'demo-release', phase: 'failed', description: 'install failed' }],
    }))));
    window.__CLUSTERNET_DASHBOARD_BASE_PATH__ = '/clusternet';

    const releases = await fetchHelmReleases();

    expect(fetch).toHaveBeenCalledWith('/clusternet/api/helmreleases');
    expect(releases[0].phase).toBe('failed');
  });
});
