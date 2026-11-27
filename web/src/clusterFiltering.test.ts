import { describe, expect, it } from 'vitest';
import { filterClusters } from './clusterFiltering';

describe('filterClusters', () => {
  it('keeps unknown readiness distinct while linking both groups from the overview', () => {
    const entries = [{ name: 'unreported', status: 'Unknown', online: false }, { name: 'unready', status: 'NotReady', online: false }];
    const apply = (status: 'unknown' | 'offline' | 'attention') => filterClusters(entries, { status, app: 'all', labelQuery: '' }).map(c => c.name);
    expect(apply('unknown')).toEqual(['unreported']);
    expect(apply('offline')).toEqual(['unready']);
    expect(apply('attention')).toEqual(['unreported', 'unready']);
  });
  const clusters = [
    { name: 'a', status: 'Online', online: true, labels: { app: 'sample-app', customer: 'demo-app' } },
    { name: 'b', status: 'Offline', online: false, labels: { app: 'vdinsight', customer: 'vd' } },
  ];

  it('filters clusters by status, app label, and label query', () => {
    expect(filterClusters(clusters, { status: 'offline', app: 'all', labelQuery: '' }).map((cluster) => cluster.name)).toEqual(['b']);
    expect(filterClusters(clusters, { status: 'all', app: 'sample-app', labelQuery: '' }).map((cluster) => cluster.name)).toEqual(['a']);
    expect(filterClusters(clusters, { status: 'all', app: 'all', labelQuery: 'customer=demo' }).map((cluster) => cluster.name)).toEqual(['a']);
  });

  it('matches cluster names and namespaces case-insensitively alongside label searches', () => {
    const data = [
      { name: 'child-east', namespace: 'tenant-red', status: 'Online', online: true },
      { name: 'child-west', namespace: 'tenant-blue', status: 'Offline', online: false },
    ];
    expect(filterClusters(data, { status: 'all', app: 'all', labelQuery: ' CHILD-wes ' })).toEqual([data[1]]);
    expect(filterClusters(data, { status: 'online', app: 'all', labelQuery: 'tenant-RED' })).toEqual([data[0]]);
  });
});
