import type { Cluster } from './api';

type ClusterListProps = {
  clusters: Cluster[];
  loading: boolean;
  error?: string;
};

export function ClusterList({ clusters, loading, error }: ClusterListProps) {
  if (loading) {
    return <section className="cluster-panel">正在加载子集群...</section>;
  }

  if (error) {
    return <section className="cluster-panel cluster-panel--error">{error}</section>;
  }

  const sortedClusters = [...clusters].sort((left, right) => Number(left.online) - Number(right.online));
  const offlineCount = clusters.filter((cluster) => !cluster.online).length;

  return (
    <section className="cluster-panel" aria-label="子集群列表">
      <div className="section-heading">
        <p className="eyebrow">Managed Clusters</p>
        <h2>子集群状态</h2>
        {clusters.length > 0 && <span className="section-badge">离线 / 异常 {offlineCount}</span>}
      </div>

      {clusters.length === 0 ? (
        <p className="empty-state">当前没有发现子集群</p>
      ) : (
        <div className="cluster-list">
          {sortedClusters.map((cluster) => (
            <article className="cluster-row" key={`${cluster.namespace}/${cluster.name}`}>
              <span
                aria-label={`${cluster.name} ${cluster.online ? 'online' : 'offline'}`}
                className={`status-dot ${cluster.online ? 'status-dot--online' : 'status-dot--offline'}`}
              />
              <div className="cluster-main">
                <strong>{cluster.name}</strong>
                <span>{cluster.namespace || 'cluster-scoped'}</span>
                <ClusterLabels labels={cluster.labels} />
              </div>
              <div className="cluster-meta">
                <strong>{cluster.status}</strong>
                <span>{cluster.kubernetesVersion || cluster.agentVersion || '版本未知'}</span>
                {cluster.lastObservedTime && <span>最后心跳 {cluster.lastObservedTime}</span>}
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}

function ClusterLabels({ labels }: { labels?: Record<string, string> }) {
  const entries = Object.entries(labels ?? {}).sort(([left], [right]) => left.localeCompare(right));
  if (entries.length === 0) {
    return null;
  }
  return (
    <div className="label-list" aria-label="cluster labels">
      {entries.map(([key, value]) => (
        <span className="label-chip" key={key}>{key}={value}</span>
      ))}
    </div>
  );
}
