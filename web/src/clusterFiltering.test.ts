import { describe, expect, it } from 'vitest';
import { filterClusters } from './clusterFiltering';

describe('filterClusters', () => {
  const clusters = [
    { name: 'a', status: 'Online', online: true, labels: { app: 'sample-app', customer: 'demo-app' } },
    { name: 'b', status: 'Offline', online: false, labels: { app: 'vdinsight', customer: 'vd' } },
  ];

  it('filters clusters by status, app label, and label query', () => {
    expect(filterClusters(clusters, { status: 'offline', app: 'all', labelQuery: '' }).map((cluster) => cluster.name)).toEqual(['b']);
    expect(filterClusters(clusters, { status: 'all', app: 'sample-app', labelQuery: '' }).map((cluster) => cluster.name)).toEqual(['a']);
    expect(filterClusters(clusters, { status: 'all', app: 'all', labelQuery: 'customer=demo' }).map((cluster) => cluster.name)).toEqual(['a']);
  });
});
