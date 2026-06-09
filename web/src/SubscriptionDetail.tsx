import type { SubscriptionDetail, SubscriptionTargetCluster } from './api';

type SubscriptionDetailViewProps = {
  detail: SubscriptionDetail;
};

export function SubscriptionDetailView({ detail }: SubscriptionDetailViewProps) {
  return (
    <section className="detail-panel" aria-label="Subscription detail">
      <div className="section-heading">
        <p className="eyebrow">Subscription Detail</p>
        <h2>{detail.summary.namespace || 'default'} / {detail.summary.name}</h2>
      </div>

      <div className="detail-grid">
        <div>
          <h3>Feeds</h3>
          <div className="resource-list">
            {detail.feeds.map((feed) => (
              <article className="resource-row" key={`${feed.namespace}/${feed.kind}/${feed.name}`}>
                <strong>{feed.kind || 'Unknown'} / {feed.name || 'unnamed'}</strong>
                <span>{feed.namespace || 'cluster-scoped'} · {feed.apiVersion || 'unknown apiVersion'}</span>
              </article>
            ))}
          </div>
        </div>

        <div>
          <h3>Target Clusters</h3>
          <div className="resource-list">
            {detail.targetClusters.map((cluster) => (
              <article className="target-row" key={`${cluster.namespace}/${cluster.name}`}>
                <span
                  aria-label={`${cluster.name} ${cluster.online ? 'online' : 'offline'}`}
                  className={`status-dot ${cluster.online ? 'status-dot--online' : 'status-dot--offline'}`}
                />
                <div>
                  <strong>{cluster.name}</strong>
                  <span>{cluster.namespace || 'cluster-scoped'} · {cluster.status}</span>
                </div>
                <strong className={clusterStatusClass(cluster)}>
                  {clusterStatusText(cluster)}
                </strong>
              </article>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

function clusterStatusText(cluster: SubscriptionTargetCluster): string {
  if (!cluster.observed) {
    return 'Not observed';
  }
  return cluster.available ? 'Available' : 'Observed';
}

function clusterStatusClass(cluster: SubscriptionTargetCluster): string {
  if (!cluster.observed) {
    return 'observed-missing';
  }
  return cluster.available ? 'observed-ok' : 'observed-warning';
}
