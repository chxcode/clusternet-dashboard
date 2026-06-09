import type { SubscriptionDetail } from './api';

type PropagationSummaryProps = {
  detail: SubscriptionDetail;
};

export function PropagationSummary({ detail }: PropagationSummaryProps) {
  const observed = detail.targetClusters.filter((cluster) => cluster.observed).length;
  const missing = detail.targetClusters.length - observed;
  const online = detail.targetClusters.filter((cluster) => cluster.online).length;
  const offline = detail.targetClusters.length - online;

  return (
    <section className="propagation-summary" aria-label="Propagation summary">
      <article className="metric-card metric-card--compact"><span>Observed</span><strong>Observed {observed}</strong></article>
      <article className="metric-card metric-card--compact"><span>Not observed</span><strong>Not observed {missing}</strong></article>
      <article className="metric-card metric-card--compact"><span>Online</span><strong>Online {online}</strong></article>
      <article className="metric-card metric-card--compact"><span>Offline</span><strong>Offline {offline}</strong></article>
    </section>
  );
}
