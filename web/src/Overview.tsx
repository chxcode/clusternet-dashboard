import { ArrowRight, ChevronRight, CircleAlert, Clock3, Server, ServerOff, type LucideIcon } from 'lucide-react';
import type { Cluster, HelmRelease, Subscription } from './api';
import { releasePhaseGroup } from './releasePhase';
import { pathForPage, type DashboardPage } from './routing';
import type { DataResult } from './useResource';
import './overview.css';

export type MaterialCounts = {
  manifests: number;
  globalizations: number;
  localizations: number;
  feedInventories: number;
};

type OverviewProps = {
  clusters: DataResult<Cluster[]>;
  releases: DataResult<HelmRelease[]>;
  subscriptions: DataResult<Subscription[]>;
  materials: DataResult<MaterialCounts>;
};

const phases = [
  { value: 'deployed', label: '已部署', color: 'var(--chart-good)' },
  { value: 'failed', label: '失败', color: 'var(--chart-failed)' },
  { value: 'pending', label: '等待中', color: 'var(--chart-pending)' },
  { value: 'unknown', label: '未知', color: 'var(--chart-unknown)' },
  { value: 'other', label: '其他', color: 'var(--chart-other)' },
] as const;

const materialTypes = [
  ['manifests', 'Manifest'],
  ['globalizations', 'Globalization'],
  ['localizations', 'Localization'],
  ['feedInventories', 'FeedInventory'],
] as const;

function linkTo(page: DashboardPage, params?: Record<string, string>) {
  const path = pathForPage(page);
  return params ? `${path}?${new URLSearchParams(params)}` : path;
}

function sourceStatus(source: DataResult<unknown>) {
  if (source.data === undefined) return source.error ? '读取失败' : '加载中…';
  if (source.error) return '刷新失败 · 显示上次数据';
  if (source.loading) return '正在刷新…';
  return undefined;
}

function SourceNotice({ source }: { source: DataResult<unknown> }) {
  const status = sourceStatus(source);
  if (!status) return null;
  return <p className={source.error ? 'ov-notice ov-notice--error' : 'ov-notice'} role="status">{status}</p>;
}

function subscriptionCoverage(clusters: DataResult<Cluster[]>, subscriptions: DataResult<Subscription[]>) {
  if (clusters.error || subscriptions.error) return '集群或订阅读取失败，暂时无法统计绑定覆盖数。';
  if (!clusters.data || !subscriptions.data) return '正在读取集群与订阅的绑定关系…';

  const registered = new Set<string>();
  const uniqueByName = new Map<string, string | null>();
  for (const cluster of clusters.data) {
    const key = (cluster.namespace || '') + '/' + cluster.name;
    if (registered.has(key)) continue;
    registered.add(key);
    uniqueByName.set(cluster.name, uniqueByName.has(cluster.name) ? null : key);
  }
  const bound = new Set<string>();
  let incomplete = false;
  for (const sub of subscriptions.data) {
    if (!Array.isArray(sub.bindingClusters)) {
      incomplete = true;
      continue;
    }
    if (sub.bindingClusterCount != null && sub.bindingClusterCount > sub.bindingClusters.length) incomplete = true;
    for (const binding of sub.bindingClusters) {
      if (typeof binding !== 'string' || !binding) {
        incomplete = true;
        continue;
      }
      if (binding.includes('/')) {
        const parts = binding.split('/');
        if (parts.length !== 2 || !parts[0] || !parts[1]) incomplete = true;
        else if (registered.has(binding)) bound.add(binding);
        else if (registered.has('/' + parts[1])) incomplete = true;
      } else {
        const key = uniqueByName.get(binding);
        if (key) bound.add(key);
        else if (key === null) incomplete = true;
      }
    }
  }
  const population = `已注册 ${registered.size} 个子集群`;
  if (incomplete && bound.size < registered.size) {
    return bound.size
      ? `${population}，已确认其中 ${bound.size} 个至少绑定一个订阅，部分绑定目标无法确定。`
      : `${population}，部分绑定目标无法确定，订阅覆盖数未知。`;
  }
  return `${population}，其中 ${bound.size} 个至少绑定一个订阅。`;
}

function Metric({ label, value, context, href, tone, icon: Icon }: {
  label: string; value?: number; context: string; href: string; tone: string; icon: LucideIcon;
}) {
  return (
    <a className={`ov-metric ov-metric--${tone}`} href={href}>
      <div className="ov-metric-title"><span className="ov-metric-icon"><Icon size={17} aria-hidden="true" /></span>{label}</div>
      <strong className="ov-metric-value">{value?.toLocaleString() ?? '—'}</strong>
      <div className="ov-metric-foot"><span>{context}</span><ArrowRight size={15} aria-hidden="true" /></div>
    </a>
  );
}

function ReleaseDistribution({ releases, counts }: {
  releases: DataResult<HelmRelease[]>;
  counts: Record<ReturnType<typeof releasePhaseGroup>, number>;
}) {
  const total = releases.data?.length ?? 0;
  let end = 0;
  const stops = phases.map(({ value, color }) => {
    const start = end;
    end += total ? 100 * counts[value] / total : 0;
    return `${color} ${start}% ${end}%`;
  });

  return (
    <section className="ov-panel" aria-label="发布状态分布">
      <header className="ov-panel-head"><h2>发布状态分布</h2></header>
      <SourceNotice source={releases} />
      {releases.data && (
        <div className="ov-distribution">
          <div className="ov-ring" style={total ? { background: `conic-gradient(${stops.join(', ')})` } : undefined}>
            <div><strong>{total.toLocaleString()}</strong><small>发布总数</small></div>
          </div>
          <ul className="ov-legend">
            {phases.filter(phase => phase.value !== 'other' || counts.other > 0).map(phase => (
              <li key={phase.value}>
                <a href={linkTo('releases', { phase: phase.value })}>
                  <i style={{ background: phase.color }} aria-hidden="true" />{phase.label}<strong>{counts[phase.value].toLocaleString()}</strong>
                </a>
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}

function FailedReleases({ releases, failures }: { releases: DataResult<HelmRelease[]>; failures: HelmRelease[] }) {
  // Surface available failure evidence first, then order by object identity for a stable preview.
  const preview = [...failures].sort((a, b) =>
    Number(Boolean(b.description?.trim())) - Number(Boolean(a.description?.trim())) ||
    (a.namespace ?? '').localeCompare(b.namespace ?? '') || a.name.localeCompare(b.name)
  ).slice(0, 4);

  return (
    <section className="ov-panel" aria-label="失败发布">
      <header className="ov-panel-head">
        <h2>失败发布</h2>
        <a className="ov-link" href={linkTo('releases', { phase: 'failed' })}>查看全部 <ArrowRight size={14} aria-hidden="true" /></a>
      </header>
      <SourceNotice source={releases} />
      {releases.data && (preview.length ? (
        <ul className="ov-issues">
          {preview.map(release => (
            <li key={`${release.namespace}/${release.name}`}>
              <a href={linkTo('releases', { phase: 'failed', namespace: release.namespace ?? 'default', name: release.name })}>
                <span className="ov-fail-mark" aria-hidden="true"><CircleAlert size={16} /></span>
                <span className="ov-object">
                  <strong title={release.name}>{release.releaseName || release.name}</strong>
                  <small title={release.description}>{release.description?.trim() || (release.descriptionOwner ? '查看 Description 分发回报' : '控制器未回报原因')}</small>
                </span>
                <span className="ov-target" title={release.clusterName}>{release.clusterName || '目标集群未知'}</span>
                <ChevronRight className="ov-row-arrow" size={15} aria-hidden="true" />
              </a>
            </li>
          ))}
        </ul>
      ) : <p className="ov-empty">当前没有失败的发布。</p>)}
    </section>
  );
}

export function Overview({ clusters, releases, subscriptions, materials }: OverviewProps) {
  const online = clusters.data?.filter(cluster => cluster.online).length;
  const needsAttention = clusters.data && online !== undefined ? clusters.data.length - online : undefined;
  const unknown = clusters.data?.filter(cluster => cluster.status === 'Unknown').length;
  const materialCounts = materials.data;
  const counts = { deployed: 0, failed: 0, pending: 0, unknown: 0, other: 0 };
  const failures: HelmRelease[] = [];
  for (const release of releases.data ?? []) {
    const group = releasePhaseGroup(release.phase);
    counts[group]++;
    if (group === 'failed') failures.push(release);
  }
  const clusterContext = sourceStatus(clusters) ?? `共 ${clusters.data?.length.toLocaleString()} 个子集群`;
  const releaseContext = sourceStatus(releases) ?? `共 ${releases.data?.length.toLocaleString()} 个发布`;

  return (
    <div className="ov">
      <section className="ov-metrics" aria-label="概览指标">
        <Metric label="在线子集群" value={online} context={clusterContext} tone="healthy" icon={Server} href={linkTo('managed-clusters', { status: 'online' })} />
        <Metric label="待排查集群" value={needsAttention} context={sourceStatus(clusters) ?? `其中 ${unknown} 个状态未知`} tone="offline" icon={ServerOff} href={linkTo('managed-clusters', { status: 'attention' })} />
        <Metric label="失败发布" value={releases.data ? counts.failed : undefined} context={releaseContext} tone="failed" icon={CircleAlert} href={linkTo('releases', { phase: 'failed' })} />
        <Metric label="等待中的发布" value={releases.data ? counts.pending : undefined} context={releaseContext} tone="pending" icon={Clock3} href={linkTo('releases', { phase: 'pending' })} />
      </section>
      <div className="ov-columns">
        <FailedReleases releases={releases} failures={failures} />
        <ReleaseDistribution releases={releases} counts={counts} />
      </div>
      <div className="ov-columns">
        <section className="ov-panel" aria-label="订阅分发进度">
          <header className="ov-panel-head"><h2>订阅分发进度</h2><a className="ov-link" href={linkTo('subscriptions')}>查看订阅 <ArrowRight size={14} aria-hidden="true" /></a></header>
          <SourceNotice source={subscriptions} />
          <p className="ov-notice">{subscriptionCoverage(clusters, subscriptions)}</p>
          {subscriptions.data && (subscriptions.data.length ? (
            <div className="ov-scroll">
              <table className="ov-subscriptions">
                <thead><tr><th scope="col">订阅</th><th scope="col">目标</th><th scope="col">已回报</th><th scope="col">已完成</th></tr></thead>
                <tbody>{subscriptions.data.slice(0, 5).map(subscription => (
                  <tr key={`${subscription.namespace}/${subscription.name}`}>
                    <th scope="row"><a href={linkTo('propagation-status', { namespace: subscription.namespace || 'default', name: subscription.name })}>
                      <strong>{subscription.name}</strong><small>{subscription.namespace || 'default'}</small>
                    </a></th>
                    <td>{subscription.desiredReleases}</td><td>{subscription.observedReleases}</td>
                    <td>{subscription.completionKnown ? subscription.completedReleases : <span className="ov-unknown">未知</span>}</td>
                  </tr>
                ))}</tbody>
              </table>
            </div>
          ) : <p className="ov-empty">当前没有订阅。</p>)}
        </section>
        <section className="ov-panel" aria-label="分发物料">
          <header className="ov-panel-head">
            <h2>分发物料</h2>
            {materialCounts && <span className="ov-total">共 {Object.values(materialCounts).reduce((sum, n) => sum + n, 0)} 项</span>}
          </header>
          <SourceNotice source={materials} />
          {materialCounts && (
            <dl className="ov-materials">
              {materialTypes.map(([key, label]) => (
                <div key={key}><dt><a className="ov-link" href={linkTo('subscriptions', key === 'manifests' ? undefined : { view: key.toLowerCase() })}>{label}</a></dt><dd>{materialCounts[key].toLocaleString()}</dd></div>
              ))}
            </dl>
          )}
        </section>
      </div>
    </div>
  );
}
