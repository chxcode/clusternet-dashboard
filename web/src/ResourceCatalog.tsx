import { useId, useState, type ReactNode } from 'react';
import type { DistributionBase, Feed, FeedInventory, HelmChart, OverrideResource, Registration } from './api';
import { pathForPage } from './routing';
import type { DataResult } from './useResource';
import { ChartVersionEditor } from './ChartVersionEditor';
import { SourceFileEditor } from './SourceFileEditor';
import { ChartDeployment } from './DeploymentDialog';
import { RegistrationWithdrawal, RegistrationReceipt, type RegistrationDeletionReceipt } from './RegistrationWithdrawal';
import { CopyButton } from './CopyButton';
import './catalog.css';

export const distributionTabs = {
  globalizations: '全局覆盖', localizations: '局部覆盖', helmcharts: 'HelmChart',
  bases: 'Base', feedinventories: 'FeedInventory',
};
export type DistributionTab = keyof typeof distributionTabs;
export function distributionView(value: string | null): DistributionTab | undefined {
  return value && Object.prototype.hasOwnProperty.call(distributionTabs, value) ? value as DistributionTab : undefined;
}
export function CatalogNavigation({ clusters, view = '' }: { clusters?: boolean; view?: string }) {
  const tabs = clusters ? { '': '集群列表', registrations: '注册请求' } : { '': '订阅与 Manifest', ...distributionTabs };
  const base = pathForPage(clusters ? 'managed-clusters' : 'subscriptions');
  return <nav className="catalog-nav" aria-label={clusters ? '集群视图' : '分发资源视图'}>
    {Object.entries(tabs).map(([id, label]) => <a key={id} aria-current={id === view ? 'page' : undefined}
      href={id ? base + '?view=' + id : base}>{label}</a>)}
  </nav>;
}

function clusterLink(name?: string) {
  return name ? <a className="inventory-link" href={pathForPage('managed-clusters') + '?' + new URLSearchParams({q: name})}>{name}</a> : '未提供';
}

function feedName(feed: Feed) {
  return [feed.kind, feed.namespace, feed.name].filter(Boolean).join(' / ');
}
function feedList(feeds?: Feed[] | null) {
  return feeds?.length ? <ul>{feeds.map((f, i) => <li key={i}>{feedName(f)}{f.apiVersion && <small>{f.apiVersion}</small>}</li>)}</ul> : '未提供 Feed 引用';
}
function localTime(value?: string) {
  if (!value) return '未提供';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : <time dateTime={value} title={value}>{date.toLocaleString('zh-CN', { hour12: false })}</time>;
}
type Row = {
  id: string; name: string; namespace?: string; search: string;
  cells: ReactNode[]; details: ReactNode; status?: string; requestedNamespace?: string;
  clusterName?: string; deleting?: boolean; rowAction?: ReactNode;
};

function RegistrationRow({ row }: { row: Row }) {
  const [expanded, setExpanded] = useState(false);
  const detailId = useId();
  return <>
    <tr>
      <th scope="row" aria-label={row.name}>
        <strong title={row.clusterName || row.name}>{row.clusterName || row.name}</strong>
        {row.clusterName && <small className="registration-request-name" title={row.name}>{row.name}</small>}
        <small className="registration-mobile-meta">{row.requestedNamespace || '自动分配'} · {row.deleting ? '正在删除' : row.status || '结果未上报'}</small>
      </th>
      {row.cells.map((cell, i) => <td key={i}>{cell}</td>)}
      <td className="registration-actions">
        {row.rowAction}
        <button type="button" className="registration-detail-toggle" aria-expanded={expanded}
          aria-controls={expanded ? detailId : undefined} onClick={() => setExpanded(!expanded)}>{expanded ? '收起详情' : '详情'}</button>
      </td>
    </tr>
    {expanded && <tr id={detailId}><td className="registration-expanded" colSpan={row.cells.length + 2}>{row.details}</td></tr>}
  </>;
}

function CatalogTable({ title, columns, rows, result, action, registration = false }: {
  title: string; columns: string[]; rows: Row[]; result: DataResult<unknown>; registration?: boolean; action?: ReactNode;
}) {
  const [query, setQuery] = useState('');
  const [status, setStatus] = useState('all');
  const [requestedNS, setRequestedNS] = useState('*');
  const [page, setPage] = useState(0);
  const needle = query.trim().toLowerCase();
  const filtered = rows.filter(row => (!needle || row.search.toLowerCase().includes(needle)) &&
    (status === 'all' || (row.status || 'unreported') === status) &&
    (requestedNS === '*' || (row.requestedNamespace || '') === requestedNS));
  const pageSize = 20;
  const current = Math.min(page, Math.max(0, Math.ceil(filtered.length / pageSize) - 1));
  const start = current * pageSize;
  const statuses = [...new Set(rows.map(row => row.status).filter((v): v is string => Boolean(v)))].sort();
  const loaded = result.data !== undefined;
  return <section className="inventory-panel catalog-panel" aria-label={title}>
    <header className="inventory-heading"><h2>{title}</h2>{action}{loaded && <span>{rows.length} 项</span>}</header>
    {result.error && <p className="inventory-warning" role="alert">{loaded ? '刷新失败，显示上次读取的数据' : '读取失败'} · {result.error}</p>}
    {!loaded && !result.error && <p className="inventory-empty" role="status">正在读取{title}…</p>}
    {loaded && <>
      {rows.length > 0 && <div className="catalog-tools">
        <label>搜索<input type="search" aria-label={'搜索' + title} value={query} placeholder={registration ? '请求名称、集群、ID 或命名空间' : '资源名称、关联对象或配置'}
          onChange={event => { setQuery(event.target.value); setPage(0); }} /></label>
        {registration && rows.some(row => row.requestedNamespace) && <label>申请命名空间<select value={requestedNS} onChange={e => { setRequestedNS(e.target.value); setPage(0); }}>
          <option value="*">全部命名空间</option><option value="">自动分配</option>
          {[...new Set(rows.map(r => r.requestedNamespace).filter((ns): ns is string => Boolean(ns)))].sort().map(ns => <option key={ns}>{ns}</option>)}
        </select></label>}
        {registration && <label>审批结果<select value={status} onChange={event => { setStatus(event.target.value); setPage(0); }}>
          <option value="all">全部结果</option>{statuses.map(s => <option key={s} value={s}>{s}</option>)}<option value="unreported">结果未上报</option>
        </select></label>}
      </div>}
      {filtered.length ? <><div className="inventory-table-scroll"><table className={registration ? 'inventory-table registration-table' : 'inventory-table'} aria-label={title}>
        <thead><tr><th scope="col">{registration ? '申请集群 / 请求名称' : '资源名称'}</th>{columns.map(col => <th scope="col" key={col}>{col}</th>)}<th scope="col">{registration ? '操作' : '详情'}</th></tr></thead>
        <tbody>{filtered.slice(start, start + pageSize).map(row => registration ? <RegistrationRow key={row.id} row={row} /> : <tr key={row.id}>
          <th scope="row"><strong>{row.name}</strong>{row.namespace && <small>{row.namespace}</small>}</th>
          {row.cells.map((cell, i) => <td key={columns[i]}>{cell}</td>)}
          <td><details className="catalog-details"><summary>详情</summary>{row.details}</details></td>
        </tr>)}</tbody></table></div>
        <footer className="inventory-pagination"><span>{start + 1}–{Math.min(start + pageSize, filtered.length)} / {filtered.length} 项</span>
          {filtered.length > pageSize && <div><button disabled={current === 0} onClick={() => setPage(current - 1)}>上一页</button>
            <button disabled={start + pageSize >= filtered.length} onClick={() => setPage(current + 1)}>下一页</button></div>}</footer>
      </> : <p className="inventory-empty">{rows.length ? '没有匹配当前条件的资源。' : '当前没有此类资源。'}</p>}
    </>}
  </section>;
}

const namespaceEvidenceText = { allocated: '同一集群 ID 已有关联的专属命名空间', occupied: '申请名称已被占用',
  terminating: '同名命名空间正在终止', available: '本次查询时名称可用' };

export function RegistrationCatalog({ result, deletionEnabled = false, onRefresh }: {
  result: DataResult<Registration[]>; deletionEnabled?: boolean; onRefresh?: () => void;
}) {
  const [receipts, setReceipts] = useState<RegistrationDeletionReceipt[]>([]);
  function deletionAction(r: Registration) {
    if (r.deleting || receipts.some(receipt => receipt.uid === r.uid)) return <small>正在删除</small>;
    if (r.result === 'Approved' || r.managedCluster || r.dedicatedNamespace || r.namespaceEvidence === 'allocated') {
      return <small title="已批准或已分配集群资源的请求受到保护">已保护</small>;
    }
    if (!deletionEnabled) return null;
    if (result.loading || result.error) return <small>{result.loading ? '正在刷新' : '需刷新'}</small>;
    if (!r.deletionMode || !r.uid || !r.resourceVersion) return <small>暂不可删除</small>;
    return <RegistrationWithdrawal key={r.uid + '/' + r.resourceVersion} row={r} onRefresh={onRefresh} onAccepted={() => {
      setReceipts(previous => [...previous.filter(p => p.name !== r.name), { name: r.name, uid: r.uid!, clusterID: r.clusterID, previous: result.data }]);
      onRefresh?.();
    }} />;
  }
  const rows = [...result.data ?? []].sort((a, b) => (b.created || '').localeCompare(a.created || '') || a.name.localeCompare(b.name)).map(r => ({
    id: r.uid || r.name, name: r.name, search: [r.name, r.clusterName, r.clusterID, r.managedCluster, r.requestedNamespace, r.dedicatedNamespace, r.result].join(' '),
    requestedNamespace: r.requestedNamespace, status: r.result, deleting: r.deleting, clusterName: r.clusterName,
    cells: [<>{r.requestedNamespace || '自动分配'}{r.namespaceEvidence && <small>{namespaceEvidenceText[r.namespaceEvidence]}</small>}</>,
      <>{r.deleting ? '正在删除' : r.result || '结果未上报'}{r.failureReason && <small>{r.failureReason}</small>}</>, localTime(r.created)],
    details: <dl><dt>请求名称</dt><dd>{r.name}<CopyButton value={r.name} label="复制请求名称" /></dd>
      <dt>申请时间</dt><dd>{localTime(r.created)}</dd>
      <dt>申请命名空间</dt><dd>{r.requestedNamespace || '自动分配'}{r.namespaceEvidence && <small>{namespaceEvidenceText[r.namespaceEvidence]}</small>}</dd>
      <dt>审批结果</dt><dd>{r.result || '结果未上报'}{r.failureReason && <small>{r.failureReason}</small>}</dd>
      <dt>集群 ID</dt><dd>{r.clusterID || '未提供'}{r.clusterID && <CopyButton value={r.clusterID} label="复制注册集群 ID" />}</dd>
      <dt>请求 UID</dt><dd>{r.uid || '未提供'}</dd><dt>资源版本</dt><dd>{r.resourceVersion || '未提供'}</dd>
      <dt>已分配集群</dt><dd>{clusterLink(r.managedCluster)}</dd><dt>已分配命名空间</dt><dd>{r.dedicatedNamespace || '未提供'}</dd>
      <dt>集群类型 / 同步模式</dt><dd>{r.clusterType || '未提供'} / {r.syncMode || '未提供'}</dd></dl>,
    rowAction: deletionAction(r),
  }));
  return <>{receipts.map(r => <RegistrationReceipt key={r.uid} receipt={r} result={result} refresh={onRefresh} />)}
    <CatalogTable registration title="注册请求" columns={['申请命名空间', '审批结果', '申请时间']} rows={rows} result={result}
      action={!deletionEnabled && <small>删除未启用</small>} /></>;
}

type DistributionProps =
  | { view: 'globalizations' | 'localizations'; result: DataResult<OverrideResource[]> }
  | { view: 'helmcharts'; result: DataResult<HelmChart[]>; versionEditing?: boolean; deploymentEnabled?: boolean; sourceEditing?: boolean; onVersionSaved?: () => void }
  | { view: 'bases'; result: DataResult<DistributionBase[]> }
  | { view: 'feedinventories'; result: DataResult<FeedInventory[]> };
export function DistributionCatalog(props: DistributionProps) {
  const {view} = props;
  let rows: Row[] = [];
  let columns: string[];
  if (props.view === 'helmcharts') {
    columns = ['Chart / 版本', '目标命名空间', '阶段'];
    rows = (props.result.data || []).map(c => ({ id: c.namespace + '/' + c.name, name:c.name, namespace:c.namespace,
      search:[c.name,c.namespace,c.chart,c.version,c.targetNamespace,c.phase].join(' '),
      cells:[<>{c.chart || '未提供'}<small>{c.version || '版本未指定'}</small>
        {c.namespace && <div className="chart-row-actions"><ChartVersionEditor chart={c} editable={props.versionEditing} onSaved={props.onVersionSaved} />
          {props.sourceEditing && !props.result.error && <SourceFileEditor chart={c} onSaved={props.onVersionSaved} />}
          {props.deploymentEnabled && !props.result.error && <ChartDeployment chart={c} />}</div>}</>,
        c.targetNamespace || '未指定', c.phase || '未上报'],
      details:<dl><dt>超时</dt><dd>{c.timeoutSeconds === undefined ? '未指定' : c.timeoutSeconds + ' 秒'}</dd>
        {Object.entries(c.options || {}).map(([key, on]) => <div key={key}><dt>{key}</dt><dd>{on ? '开启' : '关闭'}</dd></div>)}</dl>,
    }));
  } else if (props.view === 'bases') {
    columns = ['目标集群', '关联订阅', 'Feed 数'];
    rows = (props.result.data || []).map(b => ({ id:b.namespace+'/'+b.name, name:b.name, namespace:b.namespace,
      search:[b.name,b.namespace,b.clusterName,b.subscriptionName,b.subscriptionNamespace,...(b.feeds || []).map(feedName)].join(' '),
      cells:[clusterLink(b.clusterName), b.subscriptionName && b.subscriptionNamespace ?
        <a className="inventory-link" href={pathForPage('propagation-status') + '?' + new URLSearchParams({name:b.subscriptionName,namespace:b.subscriptionNamespace})}>{b.subscriptionNamespace} / {b.subscriptionName}</a> : b.subscriptionName || '未提供', b.feeds?.length ?? 0],
      details:feedList(b.feeds),
    }));
  } else if (props.view === 'feedinventories') {
    columns = ['Feed 数'];
    rows = (props.result.data || []).map(f => ({ id:(f.namespace || '')+'/'+f.name, name:f.name, namespace:f.namespace,
      search:[f.name,f.namespace,...(f.feeds || []).map(feedName)].join(' '), cells:[f.feedCount], details:feedList(f.feeds),
    }));
  } else {
    columns = ['覆盖目标 Feed', view === 'globalizations' ? '集群选择器' : '作用域', '优先级 / 策略'];
    rows = (props.result.data || []).map(o => {
      const target = o.feed && feedName(o.feed);
      const selector = view === 'globalizations' ? (o.invalidSelector ? '选择器无法解析' : o.clusterSelector || '所有集群') : o.namespace || '未提供';
      return { id:(o.namespace || '')+'/'+o.name, name:o.name, namespace:o.namespace,
        search:[o.name,o.namespace,target,selector,o.policy,...(o.rules || []).map(r=>[r.name,r.type].join(' '))].join(' '),
        cells:[target || '未提供', selector, <>{o.priority ?? '未指定'}<small>{o.policy || '策略未指定'}</small></>],
        details:<><strong>覆盖规则 · {o.overrideCount}</strong>{o.rules?.length ? <ul>{o.rules.map((r,i) =>
          <li key={i}>{r.name || '未命名'}<small>{r.type || '类型未提供'}</small></li>)}</ul> : null}
          {o.feed?.apiVersion && <small>Feed API · {o.feed.apiVersion}</small>}</>,
      };
    });
  }
  return <CatalogTable key={view} title={distributionTabs[view]} columns={columns} rows={rows} result={props.result}
    action={props.view === 'helmcharts' && props.sourceEditing && <SourceFileEditor onSaved={props.onVersionSaved} />} />;
}
