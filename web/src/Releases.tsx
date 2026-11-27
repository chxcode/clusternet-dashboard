import { useCallback, useEffect, useRef, useState } from 'react';
import { ArrowRight, ChevronLeft, ChevronRight, Search, X } from 'lucide-react';
import { fetchDescriptionStatus, type HelmRelease } from './api';
import { useResource } from './useResource';
import { ReleaseEvents } from './ReleaseEvents';
import { canDeleteRelease, ReleaseDelete, useReleaseDeletion, type ReleaseDeletion } from './ReleaseDelete';
import { ReleaseTriage, triageLabels } from './ReleaseTriage';
import { ReleaseRecovery } from './ReleaseRecovery';
import { releasePhaseGroup, releasePhaseLabel } from './releasePhase';
import { pathForPage } from './routing';
import './releases.css';

type ReleasesProps = {
  releases: HelmRelease[] | undefined;
  error?: string;
  revision?: number;
  releasesUpdatedAt?: number;
  triageEnabled?: boolean;
  onTriageSaved?: () => void;
  deleteEnabled?: boolean;
  onDeleted?: () => void;
  onRefresh?: () => void;
  params: URLSearchParams;
  onQuery: (updates: Record<string, string>, replace?: boolean) => void;
};

const pageSize = 20;
const phases = [
  ['failed', '失败'], ['pending', '进行中'], ['deployed', '已部署'],
  ['unknown', '未知'], ['other', '其他'],
];

function Status({ phase }: { phase?: string }) {
  return <span className={'release-phase phase-' + releasePhaseGroup(phase)}>{releasePhaseLabel(phase)}</span>;
}

function timestamp(value?: string) {
  if (!value || value.startsWith('0001-01-01')) return '未提供';
  const time = new Date(value);
  if (Number.isNaN(time.getTime())) return value;
  return time.toLocaleString('zh-CN', { hour12: false });
}

function DescriptionEvidence({ namespace, owner, revision }: { namespace: string; owner: NonNullable<HelmRelease['descriptionOwner']>; revision: number }) {
  const [retry, setRetry] = useState(0);
  const load = useCallback((signal: AbortSignal) => fetchDescriptionStatus(namespace, owner.name, owner.uid, signal),
    [namespace, owner.name, owner.uid]);
  const result = useResource(load, true, retry, String(revision));
  return <section className="release-evidence" aria-label="分发控制器回报">
    <h4>分发控制器回报 · Description</h4>
    <small>{namespace} / {owner.name}</small>
    {result.loading && <p role="status">正在读取分发状态…</p>}
    {result.error && <p role="alert">{result.error}{result.data && ' · 显示上次读取的数据'}</p>}
    {result.data && <><p>阶段 · {result.data.phase || '未上报'}</p><p>{result.data.reason || '分发控制器未回报原因。'}</p></>}
    <button type="button" className="text-button" disabled={result.loading}
      onClick={() => setRetry(n => n + 1)}>{result.error ? '重试' : '刷新分发回报'}</button>
  </section>;
}

function ReleaseDetail({ release, revision, releasesUpdatedAt, onClose, triageEnabled, onTriageSaved, deleteEnabled, deletion, selectedPath, onRefresh }: {
  release?: HelmRelease; revision: number; onClose: () => void; triageEnabled: boolean; onTriageSaved?: () => void;
  deleteEnabled: boolean; deletion: ReleaseDeletion; selectedPath: string; releasesUpdatedAt?: number; onRefresh?: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const deletedUid = deletion.receipts[selectedPath];
  useEffect(() => {
    const node = dialog.current;
    if (!node) return;
    if (typeof node.showModal === 'function') {
      node.showModal();
      return () => node.close();
    }
    node.setAttribute('open', '');
    return () => node.removeAttribute('open');
  }, []);

  return (
    <dialog
      ref={dialog}
      className="release-inspector"
      aria-labelledby="release-detail-title"
      onCancel={(event) => { event.preventDefault(); onClose(); }}
      onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}
    >
      <div className="release-inspector-body">
        <header className="inventory-heading">
          <h2 id="release-detail-title">发布详情</h2>
          <button type="button" className="inventory-icon-button" aria-label="关闭详情" autoFocus onClick={onClose}><X size={18} /></button>
        </header>
        {deletedUid && <p className="release-deletion-receipt" role="status">
          已受理 <code>{selectedPath}</code> 的删除请求（UID <code>{deletedUid}</code>）。
          {release?.uid && release.uid !== deletedUid && <>当前显示的是同名的新对象，UID <code>{release.uid}</code>，阶段 {release.phase || '未知'}。</>}
          {onRefresh && <button type="button" className="text-button" onClick={onRefresh}>刷新发布状态</button>}
        </p>}
        {release ? (
          <div className="release-detail-content">
            <div className="release-object">
              <Status phase={release.phase} />
              <h3>{release.name}</h3>
              <p>{release.namespace || '命名空间未提供'}</p>
              {release.deleting && <p className="release-deleting">正在删除 · 等待控制器清理</p>}
            </div>
            <section className="release-evidence">
              <h4>HelmRelease 回报</h4>
              <p>{release.description || '控制器未回报原因。'}</p>
            </section>
            {release.namespace && release.descriptionOwner &&
              <DescriptionEvidence namespace={release.namespace} owner={release.descriptionOwner} revision={revision} />}
            {releasePhaseGroup(release.phase) === 'failed' && <ReleaseRecovery release={release} />}
            <ReleaseTriage key={JSON.stringify([release.uid, release.triageIncident])} release={release} enabled={triageEnabled} onRefresh={onTriageSaved} />
            <h4>关联资源</h4>
            <dl className="release-properties">
              <dt>目标集群</dt>
              <dd>{release.clusterName ? (
                <a className="inventory-link" href={pathForPage('managed-clusters') + '?' + new URLSearchParams({ q: release.clusterName })}>
                  {release.clusterName} <ArrowRight size={13} aria-hidden="true" />
                </a>
              ) : '未提供'}</dd>
              <dt>关联订阅</dt>
              <dd>{release.subscriptionName && release.subscriptionNamespace ? (
                <a className="inventory-link" href={pathForPage('propagation-status') + '?' + new URLSearchParams({ namespace: release.subscriptionNamespace, name: release.subscriptionName })}>
                  {release.subscriptionNamespace} / {release.subscriptionName} <ArrowRight size={13} aria-hidden="true" />
                </a>
              ) : release.subscriptionName || '未提供'}</dd>
            </dl>
            <h4>发布记录</h4>
            <dl className="release-properties">
              <dt>Helm 发布名</dt><dd>{release.releaseName || '未提供'}</dd>
              <dt>目标命名空间</dt><dd>{release.targetNamespace || '未提供'}</dd>
              <dt>Chart</dt><dd>{release.chart || '未提供'}</dd>
              <dt>Chart 版本</dt><dd>{release.chartVersion || '未提供'}</dd>
              <dt>阶段</dt><dd>{release.phase || '未知'}</dd>
              <dt>修订号</dt><dd>{release.revision ?? '未提供'}</dd>
              <dt>首次部署</dt><dd>{timestamp(release.firstDeployed)}</dd>
              <dt>最近部署</dt><dd>{timestamp(release.lastDeployed)}</dd>
            </dl>
            {release.uid && release.namespace && <ReleaseEvents namespace={release.namespace} uid={release.uid} revision={revision} />}
            {deleteEnabled && canDeleteRelease(release) &&
              <ReleaseDelete key={JSON.stringify([release.uid, release.resourceVersion])} release={release} deletion={deletion} readAt={releasesUpdatedAt} onRefresh={onRefresh} />}
          </div>
        ) : <p className="inventory-empty">当前数据中找不到该发布对象。</p>}
      </div>
    </dialog>
  );
}

export function Releases({ releases, error, revision = 0, releasesUpdatedAt, triageEnabled = false, onTriageSaved, deleteEnabled = false, onDeleted, onRefresh, params, onQuery }: ReleasesProps) {
  const deletion = useReleaseDeletion(onDeleted);
  const phase = params.get('phase') || 'all';
  const query = params.get('q') || '';
  const search = query.trim().toLowerCase();
  const requestedTriage = params.get('triage') ?? '';
  const showTriageFilter = phase === 'failed' && (triageEnabled || releases?.some(item => item.triage));
  const triage = showTriageFilter && Object.prototype.hasOwnProperty.call(triageLabels, requestedTriage) ? requestedTriage : '';
  const matches = (releases ?? []).filter((item) => {
    if (phase !== 'all' && releasePhaseGroup(item.phase) !== phase) return false;
    if (triage && (item.triage ?? 'open') !== triage) return false;
    const fields = [item.name, item.namespace, item.releaseName, item.clusterName, item.subscriptionName, item.subscriptionNamespace, item.targetNamespace, item.chart];
    return !search || fields.some((field) => field?.toLowerCase().includes(search));
  });
  const pageCount = Math.max(1, Math.ceil(matches.length / pageSize));
  const requestedPage = Number(params.get('page'));
  const page = Number.isSafeInteger(requestedPage) ? Math.min(Math.max(1, requestedPage), pageCount) : 1;
  const start = (page - 1) * pageSize;
  const selectedName = params.get('name');
  const selectedNamespace = params.get('namespace') || '';
  const selected = selectedName && releases?.find((item) => item.name === selectedName && (item.namespace || '') === selectedNamespace);
  const clearFilters = () => onQuery({ q: '', phase: '', triage: '', page: '', name: '', namespace: '' });
  const customPhase = phase !== 'all' && !phases.some(([value]) => phase === value);

  return (
    <section className="inventory-panel releases-panel" aria-label="Helm 发布列表">
      <header className="inventory-heading inventory-toolbar">
        <div className="inventory-search">
          <Search size={16} aria-hidden="true" />
          <input type="search" aria-label="搜索发布" placeholder="搜索发布、集群、订阅或 Chart" value={query}
            onChange={(event) => onQuery({ q: event.target.value, page: '', name: '', namespace: '' }, true)} />
        </div>
        <label className="inventory-select">阶段
          <select value={phase} onChange={(event) => onQuery({ phase: event.target.value === 'all' ? '' : event.target.value, triage: '', page: '', name: '', namespace: '' })}>
            <option value="all">全部阶段</option>
            {phases.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            {customPhase && <option value={phase}>{phase}</option>}
          </select>
        </label>
        {showTriageFilter && <label className="inventory-select">故障跟进
          <select value={triage} onChange={event =>
            onQuery({ triage: event.target.value, page: '', name: '', namespace: '' })}>
            <option value="">全部跟进状态</option>
            {Object.entries(triageLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
        </label>}
      </header>
      {error && <p className="inventory-warning" role="alert">发布记录读取失败 · {error}</p>}
      {!releases && !error && <p className="inventory-empty" role="status">正在加载 HelmRelease…</p>}
      {releases && (matches.length > 0 ? (
        <>
          <div className="inventory-table-scroll">
            <table className="inventory-table release-table" aria-label="发布记录">
              <thead><tr><th>发布对象</th><th>阶段</th><th>目标集群</th><th>关联订阅</th><th>控制器回报</th></tr></thead>
              <tbody>
                {matches.slice(start, start + pageSize).map((item) => (
                  <tr key={(item.namespace || '') + '/' + item.name}>
                    <td>
                      <button type="button" className="release-open" aria-haspopup="dialog" onClick={() => onQuery({ namespace: item.namespace || '', name: item.name })}>{item.name}</button>
                      <small>{item.namespace || '命名空间未提供'}</small>
                    </td>
                    <td><Status phase={item.phase} />{item.triage && <small>跟进 · {triageLabels[item.triage]}</small>}</td>
                    <td>{item.clusterName || '—'}</td>
                    <td>{item.subscriptionName || '—'}</td>
                    <td><span className="release-excerpt" title={item.description}>{item.description ||
                      (item.descriptionOwner ? '详情中可查看分发回报' : '未提供')}</span></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <footer className="inventory-pagination">
            <span>{start + 1}–{Math.min(start + pageSize, matches.length)} / 共 {matches.length} 条{matches.length < releases.length && '（全部 ' + releases.length + '）'}</span>
            <div>
              <button aria-label="上一页" type="button" disabled={page === 1} onClick={() => onQuery({ page: String(page - 1) })}><ChevronLeft size={15} />上一页</button>
              <button aria-label="下一页" type="button" disabled={page >= pageCount} onClick={() => onQuery({ page: String(page + 1) })}>下一页<ChevronRight size={15} /></button>
            </div>
          </footer>
        </>
      ) : (
        <div className="inventory-empty">
          {releases.length === 0 ? '当前没有 HelmRelease。' : <>没有匹配的发布（共 {releases.length} 条）。 <button type="button" className="inventory-inline" onClick={clearFilters}>清除筛选</button></>}
        </div>
      ))}
      {selectedName && releases && <ReleaseDetail key={selectedNamespace + '/' + selectedName} release={selected || undefined}
        deleteEnabled={deleteEnabled && !error} deletion={deletion} selectedPath={selectedNamespace + '/' + selectedName} releasesUpdatedAt={releasesUpdatedAt} onRefresh={onRefresh}
        revision={revision} triageEnabled={triageEnabled} onTriageSaved={onTriageSaved} onClose={() => onQuery({ name: '', namespace: '' }, true)} />}
    </section>
  );
}
