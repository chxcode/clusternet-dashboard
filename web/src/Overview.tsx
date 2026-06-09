import type { Cluster, FeedInventory, Manifest, OverrideResource, Subscription } from './api';

type OverviewProps = {
  clusters: Cluster[];
  subscriptions: Subscription[];
  manifests: Manifest[];
  globalizations: OverrideResource[];
  localizations: OverrideResource[];
  feedInventories: FeedInventory[];
};

export function Overview({ clusters, subscriptions, manifests, globalizations, localizations, feedInventories }: OverviewProps) {
  const onlineClusters = clusters.filter((cluster) => cluster.online).length;
  const distributionMaterialCount = manifests.length + globalizations.length + localizations.length + feedInventories.length;
  const completedSubscriptions = subscriptions.filter((subscription) => subscription.status === 'Completed').length;

  return (
    <section className="overview-grid" aria-label="概览指标">
      <article className="metric-card">
        <span>在线子集群</span>
        <strong>{onlineClusters} / {clusters.length}</strong>
      </article>
      <article className="metric-card">
        <span>Subscriptions</span>
        <strong>{subscriptions.length}</strong>
        <small>{completedSubscriptions} completed</small>
      </article>
      <article className="metric-card">
        <span>分发物料</span>
        <strong>{distributionMaterialCount}</strong>
        <small>Manifest / Override / Feed</small>
      </article>
    </section>
  );
}
