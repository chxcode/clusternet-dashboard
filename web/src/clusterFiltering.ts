import type { Cluster } from './api';
import type { ClusterFilterValue } from './ClusterFilters';

export function filterClusters(clusters: Cluster[], filter: ClusterFilterValue): Cluster[] {
  const query = filter.labelQuery.trim().toLowerCase();
  return clusters.filter((cluster) => {
    if (filter.status === 'online' && !cluster.online) {
      return false;
    }
    if (filter.status === 'offline' && cluster.online) {
      return false;
    }
    if (filter.app !== 'all' && cluster.labels?.app !== filter.app) {
      return false;
    }
    if (query) {
      const labels = Object.entries(cluster.labels ?? {}).map(([key, value]) => `${key}=${value}`.toLowerCase());
      return labels.some((label) => label.includes(query));
    }
    return true;
  });
}
