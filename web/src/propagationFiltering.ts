import type { SubscriptionTargetCluster } from './api';

export type PropagationFilter = 'all' | 'observed' | 'not-observed' | 'online-not-observed' | 'offline';

export function filterTargets(targets: SubscriptionTargetCluster[], filter: PropagationFilter): SubscriptionTargetCluster[] {
  switch (filter) {
    case 'observed':
      return targets.filter((target) => target.observed);
    case 'not-observed':
      return targets.filter((target) => !target.observed);
    case 'online-not-observed':
      return targets.filter((target) => target.online && !target.observed);
    case 'offline':
      return targets.filter((target) => !target.online);
    case 'all':
    default:
      return targets;
  }
}
