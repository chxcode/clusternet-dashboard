import { Fragment, useCallback, useEffect, useRef, useState, type MouseEvent } from 'react';
import { Activity, Boxes, ChevronRight, CircleDot, GitFork, Layers3, LayoutDashboard, ListChecks, Menu, Network, RefreshCw, ScanHeart, ShieldCheck, Tag, Waypoints, X } from 'lucide-react';
import { fetchClusters, fetchContext, fetchDiagnostics, fetchFeedInventories, fetchGlobalizations, fetchHelmReleases, fetchLocalizations, fetchManifests, fetchSubscriptionDetail, fetchSubscriptions, type Subscription } from './api';
import { AppResources } from './AppResources';
import { CatalogNavigation, DistributionCatalog, RegistrationCatalog, distributionView } from './ResourceCatalog';
import { fetchBases, fetchHelmCharts, fetchRegistrations, fetchTriageStates } from './api';
import { Releases } from './Releases';
import { ClusterFilters, type ClusterFilterValue } from './ClusterFilters';
import { ClusterList } from './ClusterList';
import { DiagnosticsPanel } from './Diagnostics';
import { filterClusters } from './clusterFiltering';
import { Overview } from './Overview';
import { PropagationFilterControls } from './PropagationFilterControls';
import { PropagationSummary } from './PropagationSummary';
import { type PropagationFilter } from './propagationFiltering';
import { SubscriptionDetailView } from './SubscriptionDetail';
import { SubscriptionPicker } from './SubscriptionPicker';
import { pageFromPath, pathForPage, type DashboardPage } from './routing';
import { useResource, type DataResult } from './useResource';
import { ThemeControls } from './ThemeControls';
import './styles.css';

const pages = [
  { id: 'overview', label: '运行总览', icon: LayoutDashboard },
  { id: 'managed-clusters', label: '子集群', icon: Network },
  { id: 'releases', label: '发布记录', icon: Layers3 },
  { id: 'subscriptions', label: '订阅与清单', icon: Boxes },
  { id: 'propagation-status', label: '分发状态', icon: GitFork },
  { id: 'diagnostics', label: '集群诊断', icon: ScanHeart },
] as const;
const propagationFilters: PropagationFilter[] = ['all', 'observed', 'not-observed', 'online-not-observed', 'offline', 'unknown'];

function workspaceLocation() { return window.location.pathname + window.location.search; }
function lastUpdate(sources: DataResult<unknown>[]) {
  const times = sources.map(source => source.updatedAt).filter((t): t is number => t !== undefined);
  return times.length ? new Date(Math.min(...times)).toLocaleTimeString('zh-CN', { hour12: false }) : undefined;
}

export default function App() {
  const [location, setLocation] = useState(workspaceLocation);
  const [refresh, setRefresh] = useState(0);
  const [mobileNav, setMobileNav] = useState(false);
  const menuButton = useRef<HTMLButtonElement>(null);
  const url = new URL(location, window.location.origin);
  const page = pageFromPath(url.pathname);
  const params = url.searchParams;
  const context = useResource(fetchContext, true, refresh);
  const ready = context.data?.configured === true;
  const parentName = context.data?.displayName || (context.data?.inCluster ? '集群内连接' : context.data?.name)
    || (context.loading ? '读取中' : '未配置');
  const version = context.data?.version || (context.loading ? '读取中' : '未知');
  // A refresh starts with the context request. Keep each cluster's cached data under its own identity.
  const dataRevision = context.updatedAt ?? 0;
  const scope = context.data?.name ?? '';
  const overview = page === 'overview';
  const propagation = page === 'propagation-status';
  const registrationPage = page === 'managed-clusters' && params.get('view') === 'registrations';
  const title = registrationPage ? '子集群注册' : pages.find(item => item.id === page)!.label;
  const distributionTab = page === 'subscriptions' ? distributionView(params.get('view')) : undefined;
  const subscriptionList = page === 'subscriptions' && !distributionTab;
  const registrations = useResource(fetchRegistrations, ready && registrationPage, dataRevision, scope);
  const helmCharts = useResource(fetchHelmCharts, ready && distributionTab === 'helmcharts', dataRevision, scope);
  const bases = useResource(fetchBases, ready && distributionTab === 'bases', dataRevision, scope);
  const clusters = useResource(fetchClusters, ready && (overview || (page === 'managed-clusters' && !registrationPage)), dataRevision, scope);
  const releases = useResource(fetchHelmReleases, ready && (overview || page === 'releases'), dataRevision, scope);
  const trackReleases = !!context.data?.releaseTriage;
  const deleteEnabled = !!context.data?.releaseDeletion;
  const refreshReleases = () => setRefresh(n => n + 1);
  const followUp = useResource(fetchTriageStates, ready && trackReleases && page === 'releases', dataRevision, scope);
  const releaseRows = trackReleases && followUp.data ? releases.data?.map(release => ({
    ...release, triage: release.triageIncident ? followUp.data?.[release.triageIncident] : undefined,
  })) : releases.data;
  const subscriptions = useResource(fetchSubscriptions, ready && (overview || subscriptionList || propagation), dataRevision, scope);
  const manifests = useResource(fetchManifests, ready && (overview || subscriptionList), dataRevision, scope);
  const globalizations = useResource(fetchGlobalizations, ready && (overview || distributionTab === 'globalizations'), dataRevision, scope);
  const localizations = useResource(fetchLocalizations, ready && (overview || distributionTab === 'localizations'), dataRevision, scope);
  const feedInventories = useResource(fetchFeedInventories, ready && (overview || distributionTab === 'feedinventories'), dataRevision, scope);
  const diagnostics = useResource(fetchDiagnostics, ready && page === 'diagnostics', dataRevision, scope);
  const selectedName = params.get('name') || '';
  const selectedNamespace = params.get('namespace') || 'default';
  const loadDetail = useCallback((signal: AbortSignal) =>
    fetchSubscriptionDetail(selectedNamespace, selectedName, signal), [selectedNamespace, selectedName]);
  const detail = useResource(loadDetail, ready && propagation && !!selectedName, dataRevision, scope);

  const materialSources = [manifests, globalizations, localizations, feedInventories];
  const materials = {
    data: manifests.data && globalizations.data && localizations.data && feedInventories.data
      ? { manifests: manifests.data.length, globalizations: globalizations.data.length, localizations: localizations.data.length, feedInventories: feedInventories.data.length }
      : undefined,
    loading: materialSources.some(source => source.loading),
    error: materialSources.find(source => source.error)?.error,
  };
  const distributionSources = { globalizations, localizations, feedinventories: feedInventories, helmcharts: helmCharts, bases };
  const sourcesByPage: Record<DashboardPage, DataResult<unknown>[]> = {
    overview: [clusters, releases, subscriptions, ...materialSources],
    'managed-clusters': registrationPage ? [registrations] : [clusters], releases: trackReleases ? [releases, followUp] : [releases],
    subscriptions: distributionTab ? [distributionSources[distributionTab]] : [subscriptions, manifests],
    'propagation-status': selectedName ? [subscriptions, detail] : [subscriptions], diagnostics: [diagnostics],
  };
  const sources = ready ? sourcesByPage[page] : [context];
  const busy = context.loading || sources.some(source => source.loading);
  const updated = ready ? lastUpdate(sources) : undefined;
  const partial = Boolean(context.error || sources.some(source => source.error));
  const refreshLabel = busy ? '正在更新…' : partial ? ('部分数据未更新' + (updated ? ' · ' + updated : '')) : updated ? '更新于 ' + updated : '';
  const status = params.get('status');
  const clusterFilter: ClusterFilterValue = {
    status: status === 'online' || status === 'offline' || status === 'unknown' || status === 'attention' ? status : 'all',
    app: params.get('app') || 'all', labelQuery: params.get('q') || '',
  };
  const filter = propagationFilters.find(value => value === params.get('filter')) || 'all';

  function navigate(href: string, replace = false) {
    const oldPath = window.location.pathname;
    if (workspaceLocation() !== href) window.history[replace ? 'replaceState' : 'pushState']({}, '', href);
    setLocation(workspaceLocation());
    setMobileNav(false);
    if (oldPath !== window.location.pathname) document.documentElement.scrollTop = 0;
  }
  function updateQuery(updates: Record<string, string>, replace = false) {
    const next = new URLSearchParams(window.location.search);
    for (const [key, value] of Object.entries(updates)) {
      if (value) next.set(key, value);
      else next.delete(key);
    }
    const query = next.toString();
    navigate(window.location.pathname + (query ? '?' + query : ''), replace);
  }
  function handleLink(event: MouseEvent<HTMLDivElement>) {
    if (event.defaultPrevented || event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
    const anchor = event.target instanceof Element ? event.target.closest('a[href]') : null;
    if (!(anchor instanceof HTMLAnchorElement) || anchor.target || anchor.hasAttribute('download')) return;
    const dest = new URL(anchor.href);
    if (dest.origin !== window.location.origin || !pages.some(item => pathForPage(item.id) === dest.pathname)) return;
    event.preventDefault();
    navigate(dest.pathname + dest.search);
  }
  function selectSubscription(sub: Subscription) {
    updateQuery({ namespace: sub.namespace || 'default', name: sub.name, filter: '' });
  }

  useEffect(() => {
    const onPop = () => { setLocation(workspaceLocation()); setMobileNav(false); };
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);
  useEffect(() => { document.title = title + ' · Clusternet Dashboard'; }, [title]);
  useEffect(() => {
    if (!mobileNav) return;
    const escape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { setMobileNav(false); menuButton.current?.focus(); }
    };
    window.addEventListener('keydown', escape);
    return () => window.removeEventListener('keydown', escape);
  }, [mobileNav]);

  return <div className="app-frame" onClick={handleLink}>
    <aside className={mobileNav ? 'sidebar is-open' : 'sidebar'}>
      <a href={pathForPage('overview')} className="sidebar-brand">
        <span className="brand-mark"><Waypoints size={23} strokeWidth={2.1} /></span><strong>Clusternet</strong>
      </a>
      <button ref={menuButton} className="nav-trigger" type="button" aria-label={mobileNav ? '关闭导航' : '打开导航'}
        aria-controls="workspace-navigation" aria-expanded={mobileNav} onClick={() => setMobileNav(!mobileNav)}>
        {mobileNav ? <X size={20} /> : <Menu size={20} />}
      </button>
      <div className="sidebar-sections">
        <nav id="workspace-navigation" className="sidebar-nav" aria-label="工作区导航">
          {pages.map(({ id, label, icon: Icon }) =>
            <Fragment key={id}>
              <a href={pathForPage(id)} aria-current={id === page && !registrationPage ? 'page' : undefined}>
                <Icon size={18} aria-hidden="true" /><span>{label}</span>
              </a>
              {id === 'managed-clusters' && <a className="registration-shortcut" href={pathForPage(id) + '?view=registrations'}
                aria-current={registrationPage ? 'page' : undefined}><ListChecks size={15} aria-hidden="true" /><span>子集群注册</span></a>}
            </Fragment>)}
        </nav>
        <div className="sidebar-footer"><Tag size={15} aria-hidden="true" /><span className="dashboard-version"
          title={'Dashboard 版本 ' + version}>版本 {version}</span></div>
      </div>
    </aside>
    <main className="dashboard-content">
      <header className="workspace-bar">
        <div className="workspace-context"><Network size={16} aria-hidden="true" /><span>父集群</span>
          <ChevronRight size={13} aria-hidden="true" /><strong title={parentName}>{parentName}</strong></div>
        <div className="workspace-actions">
          <span className="access-mode" title={[context.data?.labelEditing && '集群标签', trackReleases && '失败发布跟进',
            context.data?.chartVersionEditing && 'HelmChart 版本', deleteEnabled && '删除失败发布', context.data?.chartDeployment && '部署 HelmChart', context.data?.chartSourceEditing && '编辑 Chart 配置', context.data?.nativePublishing && '发布工作负载',
            context.data?.failedRegistrationDeletion && '删除未批准的注册请求'].filter(Boolean).join('、') || '只读'}>
            <ShieldCheck size={14} />{trackReleases || context.data?.chartVersionEditing || deleteEnabled || context.data?.chartDeployment || context.data?.chartSourceEditing || context.data?.nativePublishing || context.data?.failedRegistrationDeletion ? '操作已启用' : context.data?.labelEditing ? '标签编辑已启用' : '只读'}
          </span>
          <ThemeControls />
        </div>
      </header>
      <div className="page-inner">
        <header className="page-heading"><h1>{title}</h1><div className="refresh-controls">
          <span aria-live="polite">{refreshLabel}</span>
          <button type="button" className="refresh-button" disabled={busy} onClick={() => setRefresh(n => n + 1)}>
            <RefreshCw size={14} className={busy ? 'spinning' : ''} />刷新
          </button>
        </div></header>
        {context.error && <p className="feedback-error" role="alert">无法更新父集群信息 · {context.error}</p>}
        {!ready ? <section className="workspace-message"><Activity size={22} />
          {context.loading ? '正在读取父集群配置…' : context.error ? '无法读取工作区，请重试刷新。' : '尚未配置 Kubernetes 客户端。配置父集群后重新启动服务即可查看数据。'}
        </section> : <>
          {overview && <Overview clusters={clusters} releases={releases} subscriptions={subscriptions} materials={materials} />}
          {page === 'managed-clusters' && <CatalogNavigation clusters view={registrationPage ? 'registrations' : ''} />}
          {registrationPage && <RegistrationCatalog result={registrations} deletionEnabled={context.data?.failedRegistrationDeletion} onRefresh={() => setRefresh(n => n + 1)} />}
          {page === 'managed-clusters' && !registrationPage && <>
            <ClusterFilters clusters={clusters.data || []} value={clusterFilter} onChange={next => updateQuery({
              status: next.status === 'all' ? '' : next.status, app: next.app === 'all' ? '' : next.app, q: next.labelQuery,
            }, next.labelQuery !== clusterFilter.labelQuery)} />
            <ClusterList key={JSON.stringify([scope, clusterFilter])} labelEditing={context.data?.labelEditing} onSaved={() => setRefresh(n => n + 1)} clusters={filterClusters(clusters.data || [], clusterFilter)}
              totalCount={clusters.data?.length} loading={clusters.data === undefined && clusters.loading}
              error={clusters.error} onClear={() => updateQuery({status:'',app:'',q:''})} />
          </>}
          {page === 'releases' && trackReleases && followUp.error && <p className="feedback-error" role="alert">工作台跟进记录读取失败 · {followUp.error}</p>}
          {page === 'releases' && trackReleases && !followUp.data && !followUp.error && <p role="status">正在读取工作台跟进记录…</p>}
          {page === 'releases' && <Releases key={scope} releases={releaseRows} error={releases.error}
            triageEnabled={trackReleases && !!followUp.data && !followUp.error} onTriageSaved={refreshReleases}
            deleteEnabled={deleteEnabled} onDeleted={refreshReleases} onRefresh={refreshReleases} releasesUpdatedAt={releases.updatedAt}
            revision={dataRevision} params={params} onQuery={updateQuery} />}
          {page === 'subscriptions' && <CatalogNavigation view={distributionTab} />}
          {distributionTab === 'globalizations' && <DistributionCatalog view={distributionTab} result={globalizations} />}
          {distributionTab === 'localizations' && <DistributionCatalog view={distributionTab} result={localizations} />}
          {distributionTab === 'helmcharts' && <DistributionCatalog key={scope} view={distributionTab} result={helmCharts}
            versionEditing={context.data?.chartVersionEditing} deploymentEnabled={context.data?.chartDeployment} sourceEditing={context.data?.chartSourceEditing} onVersionSaved={() => setRefresh(n => n + 1)} />}
          {distributionTab === 'bases' && <DistributionCatalog view={distributionTab} result={bases} />}
          {distributionTab === 'feedinventories' && <DistributionCatalog view={distributionTab} result={feedInventories} />}
          {subscriptionList && <AppResources nativePublishing={context.data?.nativePublishing} onPublished={() => setRefresh(n => n + 1)} subscriptions={subscriptions.data} manifests={manifests.data}
            subscriptionError={subscriptions.error} manifestError={manifests.error} />}
          {propagation && <>
            <SubscriptionPicker subscriptions={subscriptions.data || []} loading={!subscriptions.data && subscriptions.loading}
              error={subscriptions.error} selectedSubscription={selectedName ? {name:selectedName, namespace:selectedNamespace} : undefined} onSelect={selectSubscription} />
            {selectedName ? <>
              {detail.error && <p className="feedback-error" role="alert">订阅详情未更新 · {detail.error}</p>}
              {detail.data ? <>
                <PropagationSummary detail={detail.data} />
                <PropagationFilterControls value={filter} onChange={value => updateQuery({filter:value === 'all' ? '' : value})} />
                <SubscriptionDetailView detail={detail.data} filter={filter} />
              </> : detail.loading && <p className="workspace-message" role="status">正在加载 {selectedNamespace} / {selectedName}…</p>}
            </> : <p className="workspace-message"><CircleDot size={17} />选择订阅以查看 Feed、目标集群和回报状态。</p>}
          </>}
          {page === 'diagnostics' && <DiagnosticsPanel diagnostics={diagnostics.data} loading={!diagnostics.data && diagnostics.loading} error={diagnostics.error} />}
        </>}
      </div>
    </main>
  </div>;
}
