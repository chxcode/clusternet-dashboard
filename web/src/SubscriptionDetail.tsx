import { Fragment, useState } from 'react';
import type { Feed, SubscriptionDetail, SubscriptionTargetCluster } from './api';
import { filterTargets, targetConnectivity, type PropagationFilter } from './propagationFiltering';
import { pathForPage } from './routing';
import './propagation.css';

type SubscriptionDetailViewProps = {
  detail: SubscriptionDetail;
  filter?: PropagationFilter;
};

const pageSize = 20;
const connectivityLabels = { online: '在线', offline: '未就绪', unknown: '未知' };
const replicaLabels: Record<string, string> = {
  replicas: '副本数', currentReplicas: '当前副本', readyReplicas: '就绪副本', availableReplicas: '可用副本',
  unavailableReplicas: '不可用副本', updatedReplicas: '已更新副本', active: '活跃任务',
  succeeded: '成功任务', failed: '失败任务', observedGeneration: '已观测代数',
};

function feedKey(feed: Feed) {
  return JSON.stringify([feed.apiVersion || '', feed.kind || '', feed.namespace || '', feed.name || '']);
}

function FeedReports({ target, feeds }: { target: SubscriptionTargetCluster; feeds: Feed[] }) {
  const reports = target.feedReports || [];
  const uncertain = target.unattributedFeeds || [];
  const reportedKeys = new Set(reports.map(feedKey));
  const uncertainKeys = new Set(uncertain.map(feedKey));
  const missing = feeds.filter(feed => !reportedKeys.has(feedKey(feed)) && !uncertainKeys.has(feedKey(feed)));
  if (target.observed && !reports.length) return <p>父集群未提供分项回报。</p>;
  if (!reports.length && !missing.length && !uncertain.length) return <p>此目标没有 Feed 回报。</p>;
  return <ul className="pd-report-list" aria-label={target.name + ' 的 Feed 回报'}>
    {reports.map((report, index) => {
      const replicas = report.replicaStatus || {};
      const counters = Object.entries(replicaLabels).filter(([field]) => replicas[field] !== undefined);
      const availability = report.available === undefined ? '可用性未上报' : report.available ? '可用' : '不可用';
      return <li key={feedKey(report) + '/' + index}>
        <strong>{report.kind || '未知类型'} / {report.name || '未标识资源'}</strong>
        <small>{[report.namespace, report.apiVersion].filter(Boolean).join(' · ')}</small>
        <p>已回报 · {availability}</p>
        {counters.length > 0 && <dl className="pd-replicas">{counters.map(([field, label]) => <div key={field}>
          <dt>{label}</dt><dd>{replicas[field]}</dd></div>)}</dl>}
      </li>;
    })}
    {uncertain.map(feed => <li key={feedKey(feed)}>
      <strong>{feed.kind || '未知类型'} / {feed.name || '未标识资源'}</strong>
      <small>{[feed.namespace, feed.apiVersion].filter(Boolean).join(' · ')}</small>
      <p>回报归属待确认 · 集群标识不足以匹配此目标</p>
    </li>)}
    {missing.map(feed => <li key={feedKey(feed)}>
      <strong>{feed.kind || '未知类型'} / {feed.name || '未标识资源'}</strong>
      <small>{[feed.namespace, feed.apiVersion].filter(Boolean).join(' · ')}</small>
      <p>未回报</p>
    </li>)}
  </ul>;
}

function Targets({ targets, total, feeds, bindingsKnown }: { targets: SubscriptionTargetCluster[]; total: number; feeds: Feed[]; bindingsKnown: boolean }) {
  const [requestedPage, setPage] = useState(1);
  const [expanded, setExpanded] = useState('');
  const pageCount = Math.max(1, Math.ceil(targets.length / pageSize));
  const page = Math.min(requestedPage, pageCount);
  const start = (page - 1) * pageSize;
  return (
    <section className="pd-panel" aria-label="目标集群">
      <header className="pd-heading"><h2>目标集群</h2>{bindingsKnown && <span>匹配 {targets.length} / 共 {total} 个</span>}</header>
      {!bindingsKnown ? <p className="pd-empty">控制器尚未提供可读取的绑定名单。</p> : targets.length ? (
        <>
          <table className="pd-table">
            <thead><tr><th scope="col">集群 / 命名空间</th><th scope="col">连接</th><th scope="col">资源回报</th></tr></thead>
            <tbody>{targets.slice(start, start + pageSize).map((target, index) => {
              const connection = targetConnectivity(target);
              const key = `${target.namespace}/${target.name}`;
              const open = expanded === key;
              let reportLabel = '未回报';
              if (target.observed) reportLabel = target.available === undefined ? '已回报' : target.available ? '已回报可用' : '已回报不可用';
              else if (target.unattributedFeeds?.length) reportLabel = '归属待确认';
              return <Fragment key={key}>
                <tr>
                  <th scope="row"><a href={pathForPage('managed-clusters') + '?' + new URLSearchParams({ q: target.name })}>{target.name}</a>
                    <small>{target.namespace || '未提供命名空间'}</small></th>
                  <td><span className={'pd-connection pd-' + connection}>{connectivityLabels[connection]}</span></td>
                  <td><button type="button" className="pd-expand" aria-expanded={open}
                    aria-controls={open ? 'pd-report-' + index : undefined}
                    aria-label={target.name + ' 的 Feed 回报 · ' + reportLabel}
                    onClick={() => setExpanded(open ? '' : key)}>{reportLabel}</button></td>
                </tr>
                {open && <tr className="pd-report-row" id={'pd-report-' + index}><td colSpan={3}>
                  <FeedReports target={target} feeds={feeds} />
                </td></tr>}
              </Fragment>;
            })}</tbody>
          </table>
          {pageCount > 1 && (
            <nav className="pd-paging" aria-label="目标集群分页">
              <button type="button" disabled={page === 1} onClick={() => setPage(page - 1)}>上一页</button>
              <span aria-live="polite">{page} / {pageCount}</span>
              <button type="button" disabled={page === pageCount} onClick={() => setPage(page + 1)}>下一页</button>
            </nav>
          )}
        </>
      ) : <p className="pd-empty">{total ? '没有与当前筛选条件匹配的目标集群。' : '此订阅尚无绑定的目标集群。'}</p>}
    </section>
  );
}

export function SubscriptionDetailView({ detail, filter = 'all' }: SubscriptionDetailViewProps) {
  const { summary, feeds, targetClusters } = detail;
  return (
    <div className="pd-details">
      <Targets key={JSON.stringify([summary.namespace, summary.name, filter])}
        targets={filterTargets(targetClusters, filter)} total={targetClusters.length} feeds={feeds} bindingsKnown={Array.isArray(summary.bindingClusters)} />
      <aside className="pd-side">
        <section className="pd-panel" aria-label="订阅详情">
          <header className="pd-heading"><h2>订阅详情</h2></header>
          <strong className="pd-name">{summary.name}</strong><small>{summary.namespace || 'default'}</small>
          <dl className="pd-facts">
            <div><dt>期望发布</dt><dd>{summary.desiredReleases}</dd></div>
            <div><dt>已确认完成</dt><dd>{summary.completionKnown ? summary.completedReleases : '未知'}</dd></div>
            {summary.subscriberCount !== undefined && <div><dt>订阅者</dt><dd>{summary.subscriberCount}</dd></div>}
            {summary.schedulerName && <div><dt>调度器</dt><dd>{summary.schedulerName}</dd></div>}
          </dl>
        </section>
        <section className="pd-panel" aria-label="订阅资源">
          <header className="pd-heading"><h2>订阅资源</h2><span>{feeds.length} 个 Feed</span></header>
          {feeds.length ? <ul className="pd-feeds">{feeds.map(feed => (
            <li key={JSON.stringify([feed.apiVersion, feed.kind, feed.namespace, feed.name])}>
              <strong>{feed.kind || '未知类型'} / {feed.name || '未命名'}</strong>
              <small>{feed.namespace || '集群级'}{feed.apiVersion && ` · ${feed.apiVersion}`}</small>
            </li>
          ))}</ul> : <p className="pd-empty">此订阅没有 Feed。</p>}
        </section>
      </aside>
    </div>
  );
}
