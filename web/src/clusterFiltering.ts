import type { Cluster } from './api';
import type { ClusterFilterValue } from './ClusterFilters';

export function filterClusters(clusters: Cluster[], filter: ClusterFilterValue): Cluster[] {
  const query = filter.labelQuery.trim().toLowerCase();
  return clusters.filter((cluster) => {
    if (filter.status === 'online' && !cluster.online) return false;
    if (filter.status === 'attention' && cluster.online) return false;
    if (filter.status === 'unknown' && cluster.status !== 'Unknown') return false;
    if (filter.status === 'offline' && (cluster.online || cluster.status === 'Unknown')) return false;
    if (filter.app !== 'all' && cluster.labels?.app !== filter.app) return false;
    if (!query) return true;
    const searchTerms = [cluster.name, cluster.clusterId ?? '', cluster.namespace ?? '',
      ...Object.entries(cluster.labels ?? {}).map(([key, value]) => `${key}=${value}`)];
    return searchTerms.some((term) => term.toLowerCase().includes(query));
  });
}
