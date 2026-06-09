import { useEffect, useState } from 'react';
import { Activity, Boxes, ShieldCheck } from 'lucide-react';
import {
  fetchClusters,
  fetchDiagnostics,
  fetchFeedInventories,
  fetchGlobalizations,
  fetchHelmReleases,
  fetchLocalizations,
  fetchManifests,
  fetchSubscriptionDetail,
  fetchSubscriptions,
  type Cluster,
  type Diagnostics,
  type FeedInventory,
  type HelmRelease,
  type Manifest,
  type OverrideResource,
  type Subscription,
  type SubscriptionDetail,
} from './api';
import { AppResources } from './AppResources';
import { ClusterFilters, type ClusterFilterValue } from './ClusterFilters';
import { ClusterList } from './ClusterList';
import { DiagnosticsPanel } from './Diagnostics';
import { filterClusters } from './clusterFiltering';
import { Overview } from './Overview';
import { PropagationFilterControls } from './PropagationFilterControls';
import { PropagationSummary } from './PropagationSummary';
import { filterTargets, type PropagationFilter } from './propagationFiltering';
import { SubscriptionDetailView } from './SubscriptionDetail';
import { SubscriptionPicker } from './SubscriptionPicker';
import { pageFromPath, pathForPage, type DashboardPage } from './routing';
import './styles.css';

const cards = [
  {
    title: '内网部署',
    description: '第一版默认不内建登录，建议放在 VPN、Ingress Auth 或统一网关后面。',
    icon: ShieldCheck,
  },
  {
    title: '只读 ServiceAccount',
    description: '后端使用部署时绑定的只读 RBAC 访问 parent / hub cluster。',
    icon: Activity,
  },
  {
    title: 'Helm 独立升级',
    description: 'Dashboard 作为非必须组件独立交付，可以单独 helm upgrade。',
    icon: Boxes,
  },
];

const pages: Array<{ id: DashboardPage; label: string; description: string }> = [
  { id: 'overview', label: 'Overview', description: '全局总览' },
  { id: 'managed-clusters', label: 'Managed Clusters', description: '子集群状态与标签' },
  { id: 'subscriptions', label: 'Subscriptions', description: '订阅、Feed 与调度关系' },
  { id: 'propagation-status', label: 'Propagation Status', description: '资源分发结果' },
  { id: 'diagnostics', label: 'Diagnostics', description: '组件、CRD 与 RBAC 诊断' },
];

export default function App() {
  const [activePage, setActivePage] = useState<DashboardPage>(() => pageFromPath(window.location.pathname));
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([]);
  const [manifests, setManifests] = useState<Manifest[]>([]);
  const [globalizations, setGlobalizations] = useState<OverrideResource[]>([]);
  const [localizations, setLocalizations] = useState<OverrideResource[]>([]);
  const [feedInventories, setFeedInventories] = useState<FeedInventory[]>([]);
  const [helmReleases, setHelmReleases] = useState<HelmRelease[]>([]);
  const [diagnostics, setDiagnostics] = useState<Diagnostics>();
  const [loadingClusters, setLoadingClusters] = useState(true);
  const [loadingResources, setLoadingResources] = useState(true);
  const [loadingHelmReleases, setLoadingHelmReleases] = useState(true);
  const [loadingDiagnostics, setLoadingDiagnostics] = useState(true);
  const [clusterError, setClusterError] = useState<string>();
  const [resourceError, setResourceError] = useState<string>();
  const [diagnosticsError, setDiagnosticsError] = useState<string>();
  const [subscriptionDetail, setSubscriptionDetail] = useState<SubscriptionDetail>();
  const [subscriptionDetailError, setSubscriptionDetailError] = useState<string>();
  const [clusterFilter, setClusterFilter] = useState<ClusterFilterValue>({ status: 'all', app: 'all', labelQuery: '' });
  const [propagationFilter, setPropagationFilter] = useState<PropagationFilter>('all');
  const filteredClusters = filterClusters(clusters, clusterFilter);
  const filteredSubscriptionDetail = subscriptionDetail
    ? { ...subscriptionDetail, targetClusters: filterTargets(subscriptionDetail.targetClusters, propagationFilter) }
    : undefined;

  function navigateToPage(page: DashboardPage) {
    setActivePage(page);
    const nextPath = pathForPage(page);
    if (window.location.pathname !== nextPath) {
      window.history.pushState({}, '', nextPath);
    }
  }

  function handleSelectSubscription(subscription: Subscription) {
    const namespace = subscription.namespace || 'default';
    setSubscriptionDetail(undefined);
    setSubscriptionDetailError(undefined);
    setPropagationFilter('all');
    navigateToPage('propagation-status');
    fetchSubscriptionDetail(namespace, subscription.name)
      .then((detail) => setSubscriptionDetail(detail))
      .catch((error: unknown) => {
        setSubscriptionDetailError(error instanceof Error ? error.message : '加载 Subscription 详情失败');
      });
  }

  useEffect(() => {
    function handlePopState() {
      setActivePage(pageFromPath(window.location.pathname));
    }
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  useEffect(() => {
    let cancelled = false;

    fetchClusters()
      .then((items) => {
        if (!cancelled) {
          setClusters(items);
          setClusterError(undefined);
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setClusterError(error instanceof Error ? error.message : '加载子集群失败');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingClusters(false);
        }
      });

    Promise.all([
      fetchSubscriptions(),
      fetchManifests(),
      fetchGlobalizations(),
      fetchLocalizations(),
      fetchFeedInventories(),
    ])
      .then(([subscriptionItems, manifestItems, globalizationItems, localizationItems, feedInventoryItems]) => {
        if (!cancelled) {
          setSubscriptions(subscriptionItems);
          setManifests(manifestItems);
          setGlobalizations(globalizationItems);
          setLocalizations(localizationItems);
          setFeedInventories(feedInventoryItems);
          setResourceError(undefined);
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setResourceError(error instanceof Error ? error.message : '加载分发资源失败');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingResources(false);
        }
      });

    fetchHelmReleases()
      .then((items) => {
        if (!cancelled) {
          setHelmReleases(items);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setHelmReleases([]);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingHelmReleases(false);
        }
      });

    fetchDiagnostics()
      .then((item) => {
        if (!cancelled) {
          setDiagnostics(item);
          setDiagnosticsError(undefined);
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setDiagnosticsError(error instanceof Error ? error.message : '加载诊断信息失败');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingDiagnostics(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <main className="app-frame">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <span className="brand-mark">C</span>
          <div>
            <strong>Clusternet</strong>
            <span>Dashboard</span>
          </div>
        </div>
        <nav className="sidebar-nav" aria-label="Dashboard navigation">
          {pages.map((page) => (
            <button
              type="button"
              key={page.id}
              className={activePage === page.id ? 'sidebar-link sidebar-link--active' : 'sidebar-link'}
              aria-label={page.label}
              onClick={() => navigateToPage(page.id)}
            >
              <strong>{page.label}</strong>
              <span>{page.description}</span>
            </button>
          ))}
        </nav>
        <div className="sidebar-footer">
          <span>只读模式</span>
          <strong>/clusternet</strong>
        </div>
      </aside>

      <section className="dashboard-content">
        <section className="hero hero--compact">
          <div>
            <p className="eyebrow">默认路径 /clusternet</p>
            <h1>{pages.find((page) => page.id === activePage)?.label || 'Clusternet Dashboard'}</h1>
            <p className="summary">
              一个用于查看 Clusternet 多集群状态、订阅分发和资源健康情况的轻量控制台。
            </p>
          </div>
          <span className="status">MVP</span>
        </section>

        {activePage === 'overview' && (
          <>
            <section className="cards" aria-label="MVP design principles">
              {cards.map((card) => {
                const Icon = card.icon;
                return (
                  <article className="card" key={card.title}>
                    <Icon aria-hidden="true" size={24} />
                    <h2>{card.title}</h2>
                    <p>{card.description}</p>
                  </article>
                );
              })}
            </section>
            <Overview
              clusters={clusters}
              subscriptions={subscriptions}
              manifests={manifests}
              globalizations={globalizations}
              localizations={localizations}
              feedInventories={feedInventories}
            />
          </>
        )}

        {activePage === 'managed-clusters' && (
          <>
            <ClusterFilters clusters={clusters} value={clusterFilter} onChange={setClusterFilter} />
            <ClusterList clusters={filteredClusters} loading={loadingClusters} error={clusterError} />
          </>
        )}

        {activePage === 'subscriptions' && (
          <AppResources
            subscriptions={subscriptions}
            manifests={manifests}
            helmReleases={helmReleases}
            helmReleasesLoading={loadingHelmReleases}
            loading={loadingResources}
            error={resourceError}
            onSelectSubscription={handleSelectSubscription}
          />
        )}

        {activePage === 'propagation-status' && (
          <>
            <SubscriptionPicker
              subscriptions={subscriptions}
              loading={loadingResources}
              error={resourceError}
              selectedSubscription={subscriptionDetail?.summary}
              onSelect={handleSelectSubscription}
            />
            {subscriptionDetailError && <section className="detail-panel detail-panel--error">{subscriptionDetailError}</section>}
            {subscriptionDetail && filteredSubscriptionDetail ? (
              <>
                <PropagationSummary detail={subscriptionDetail} />
                <PropagationFilterControls value={propagationFilter} onChange={setPropagationFilter} />
                <SubscriptionDetailView detail={filteredSubscriptionDetail} />
              </>
            ) : (
              <section className="detail-panel">
                <div className="section-heading">
                  <p className="eyebrow">Propagation Status</p>
                  <h2>分发状态</h2>
                </div>
                <p className="empty-state">请先在上方选择一个 Subscription，查看目标集群的 observed / not observed 状态。</p>
              </section>
            )}
          </>
        )}

        {activePage === 'diagnostics' && (
          <DiagnosticsPanel diagnostics={diagnostics} loading={loadingDiagnostics} error={diagnosticsError} />
        )}
      </section>
    </main>
  );
}
