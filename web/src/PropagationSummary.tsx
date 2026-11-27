import type { SubscriptionDetail } from './api';
import { targetConnectivity } from './propagationFiltering';
import { currentBindingCount, SubscriptionPolicy } from './SubscriptionPolicy';
import { pathForPage } from './routing';

export function PropagationSummary({ detail }: { detail: SubscriptionDetail }) {
  const targets = detail.targetClusters;
  const bindings = currentBindingCount(detail.summary);
  const observed = targets.filter(target => target.observed).length;
  const uncertain = targets.filter(target => !target.observed && target.unattributedFeeds?.length).length;
  const connections = { online: 0, offline: 0, unknown: 0 };
  for (const target of targets) connections[targetConnectivity(target)]++;
  return (
    <section className="pd-summary" aria-label="分发概况">
      <div className="sub-overview">
        <h2>分发方式与目标选择</h2>
        <SubscriptionPolicy subscription={detail.summary} previousSelectors={detail.subscriberSelectors} explain />
        <dl className="sub-coverage">
          <div><dt>已注册</dt><dd>{detail.coverage?.registeredClusters ?? '未知'}</dd></div>
          <div><dt>标签匹配</dt><dd>{detail.coverage?.labelMatchedClusters ?? '未知'}</dd></div>
          <div><dt>当前已绑定</dt><dd>{bindings ?? '未上报'}</dd></div>
        </dl>
        {bindings !== undefined && !!detail.coverage?.unboundMatches?.length && <details className="sub-unbound">
          <summary>标签匹配、未列入当前绑定（{detail.coverage.unboundMatches.length}）</summary>
          <ul>{detail.coverage.unboundMatches.map(cluster => <li key={cluster.namespace + '/' + cluster.name}>
            <a className="inventory-link" href={pathForPage('managed-clusters') + '?' + new URLSearchParams({ q: cluster.name })}>
              {cluster.namespace}/{cluster.name}
            </a>
          </li>)}</ul>
        </details>}
      </div>
      {bindings === undefined ? <div className="sub-bindings-unknown">绑定名单未上报，暂无法统计各目标的回报与连接状态。</div> : <>
        <div><h2>目标回报</h2><dl>
          <div><dt>已回报</dt><dd>{observed}</dd></div>
          <div><dt>未回报</dt><dd>{targets.length - observed - uncertain}</dd></div>
          {uncertain > 0 && <div><dt>归属待确认</dt><dd>{uncertain}</dd></div>}
        </dl></div>
        <div><h2>集群连接</h2><dl>
          <div><dt>在线</dt><dd>{connections.online}</dd></div>
          <div><dt>未就绪</dt><dd>{connections.offline}</dd></div>
          {connections.unknown > 0 && <div><dt>未知</dt><dd>{connections.unknown}</dd></div>}
        </dl></div>
      </>}
    </section>
  );
}
