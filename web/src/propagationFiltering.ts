import type { SubscriptionTargetCluster } from './api';

export type PropagationFilter = 'all' | 'observed' | 'not-observed' | 'online-not-observed' | 'offline' | 'unknown';

export function targetConnectivity(cluster: SubscriptionTargetCluster): 'online' | 'offline' | 'unknown' {
  if (cluster.online) return 'online';
  return cluster.status.toLowerCase() === 'unknown' ? 'unknown' : 'offline';
}

export function filterTargets(targets: SubscriptionTargetCluster[], filter: PropagationFilter): SubscriptionTargetCluster[] {
  switch (filter) {
    case 'observed':
      return targets.filter((target) => target.observed);
    case 'not-observed':
      return targets.filter((target) => !target.observed && !target.unattributedFeeds?.length);
    case 'online-not-observed':
      return targets.filter((target) => target.online && !target.observed && !target.unattributedFeeds?.length);
    case 'offline':
    case 'unknown':
      return targets.filter((target) => targetConnectivity(target) === filter);
    case 'all':
    default:
      return targets;
  }
}
