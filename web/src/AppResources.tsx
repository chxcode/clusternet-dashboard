import type { HelmRelease, Manifest, Subscription } from './api';
import { SubscriptionSummaryMeta } from './SubscriptionSummary';

type AppResourcesProps = {
  subscriptions: Subscription[];
  manifests: Manifest[];
  helmReleases?: HelmRelease[];
  helmReleasesLoading?: boolean;
  loading: boolean;
  error?: string;
  onSelectSubscription?: (subscription: Subscription) => void;
};

export function AppResources({ subscriptions, manifests, helmReleases = [], helmReleasesLoading = false, loading, error, onSelectSubscription }: AppResourcesProps) {
  if (loading) {
    return <section className="resource-panel">正在加载应用分发资源...</section>;
  }

  if (error) {
    return <section className="resource-panel resource-panel--error">{error}</section>;
  }

  return (
    <section className="resource-panel" aria-label="应用分发资源">
      <div className="section-heading">
        <p className="eyebrow">Resource Propagation</p>
        <h2>资源分发状态</h2>
      </div>

      {subscriptions.length === 0 && manifests.length === 0 && helmReleases.length === 0 ? (
        <p className="empty-state">当前没有发现 Subscriptions、Manifests 或 HelmReleases</p>
      ) : (
        <div className="resource-grid">
          <div>
            <h3>Subscriptions</h3>
            <div className="resource-list">
              {subscriptions.map((subscription) => (
                <article
                  className="resource-row resource-row--clickable"
                  key={`${subscription.namespace}/${subscription.name}`}
                  onClick={() => onSelectSubscription?.(subscription)}
                >
                  <div>
                    <strong>{subscription.name}</strong>
                    <span>{subscription.namespace || 'default'}</span>
                  </div>
                  <div>
                    <strong>{subscription.status}</strong>
                    <span>{formatSubscriptionProgress(subscription)}</span>
                    {!subscription.completionKnown && (
                      <span className="compatibility-hint">Observed status only</span>
                    )}
                  </div>
                  <div>
                    <span>{subscription.bindingClusterCount} clusters</span>
                    <SubscriptionSummaryMeta subscription={subscription} />
                  </div>
                </article>
              ))}
            </div>
          </div>

          <div>
            <h3>Manifests</h3>
            <div className="resource-list">
              {manifests.map((manifest) => (
                <article className="resource-row" key={`${manifest.namespace}/${manifest.name}`}>
                  <div>
                    <strong>{manifest.name}</strong>
                    <span>{manifest.namespace || 'default'}</span>
                  </div>
                  <div>
                    <strong>{manifest.templateKind || 'Unknown'}</strong>
                    <span>
                      {(manifest.templateKind || 'Unknown') + ' / ' + (manifest.templateName || 'unnamed')}
                    </span>
                  </div>
                  <span>{manifest.templateNamespace || 'cluster-scoped'}</span>
                </article>
              ))}
            </div>
          </div>

          <div className="resource-grid-full">
            <h3>HelmReleases</h3>
            {helmReleasesLoading && <p className="empty-state">正在加载 HelmReleases...</p>}
            <div className="resource-list">
              {helmReleases.map((release) => (
                <article className="resource-row" key={`${release.namespace}/${release.name}`}>
                  <div>
                    <strong>{release.name}</strong>
                    <span>{release.clusterName || release.namespace || 'unknown cluster'}</span>
                  </div>
                  <div>
                    <strong className={helmReleasePhaseClass(release.phase)}>{release.phase || 'unknown'}</strong>
                    <span>{release.releaseName || release.name} · {release.targetNamespace || 'default'}</span>
                  </div>
                  <div>
                    <span>{release.chart || 'unknown chart'}{release.chartVersion ? `:${release.chartVersion}` : ''}</span>
                    <span>{release.subscriptionNamespace || 'default'}/{release.subscriptionName || 'unknown subscription'}</span>
                  </div>
                  {release.description && <span className="release-description">{release.description}</span>}
                </article>
              ))}
            </div>
          </div>
        </div>
      )}
    </section>
  );
}

function formatSubscriptionProgress(subscription: Subscription): string {
  if (subscription.completionKnown) {
    return `${subscription.completedReleases} / ${subscription.desiredReleases}`;
  }
  if (subscription.observedReleases > 0) {
    return `${subscription.observedReleases} observed / ${subscription.desiredReleases}`;
  }
  return `unknown / ${subscription.desiredReleases}`;
}

function helmReleasePhaseClass(phase?: string): string {
  const normalized = phase?.toLowerCase();
  if (normalized === 'deployed') {
    return 'observed-ok';
  }
  if (normalized === 'failed' || normalized === 'pending-install' || normalized === 'pending-upgrade') {
    return 'observed-missing';
  }
  return 'observed-warning';
}
