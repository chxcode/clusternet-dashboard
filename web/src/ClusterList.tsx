import { Fragment, useId, useState } from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import type { Cluster } from './api';
import { ClusterEvidence, ReportedTime } from './ClusterEvidence';
import { ClusterLabels } from './ClusterLabels';
import { CopyButton } from './CopyButton';
import './clusters.css';

type ClusterListProps = {
  clusters: Cluster[];
  loading: boolean;
  error?: string;
  totalCount?: number;
  onClear?: () => void;
  labelEditing?: boolean;
  onSaved?: () => void;
};

const pageSize = 20;

export function ClusterList({ clusters, loading, error, totalCount, onClear, labelEditing = false, onSaved }: ClusterListProps) {
  const [requestedPage, setPage] = useState(1);
  const [expanded, setExpanded] = useState('');
  const idPrefix = useId();
  const total = totalCount ?? clusters.length;
  const numPages = Math.max(1, Math.ceil(clusters.length / pageSize));
  const page = Math.min(requestedPage, numPages);
  const start = (page - 1) * pageSize;
  const rows = [...clusters].sort((a, b) =>
    Number(a.online) - Number(b.online) || a.name.localeCompare(b.name) || (a.namespace ?? '').localeCompare(b.namespace ?? '')
  ).slice(start, start + pageSize);

  if (loading) return <section className="cw-panel cw-message" role="status">正在加载子集群…</section>;
  if (error && totalCount === undefined && !clusters.length) {
    return <section className="cw-panel cw-message cw-error" role="alert">集群列表读取失败 · {error}</section>;
  }
  return (
    <section className="cw-panel" aria-labelledby="cluster-list-title">
      <header className="cw-section-title">
        <h2 id="cluster-list-title">子集群列表</h2>
        <span className="cw-count">{clusters.length < total ? `匹配 ${clusters.length} / 共 ${total} 个` : `共 ${total} 个`}</span>
      </header>
      {error && <p className="cw-message cw-error" role="alert">刷新失败，显示上次读取的数据 · {error}</p>}
      {clusters.length === 0 ? (
        <div className="cw-message" role="status">
          {total > 0 ? <>没有匹配当前条件的集群。{onClear && <button className="cw-clear" type="button" onClick={onClear}>清除筛选</button>}</> : '当前没有发现子集群'}
        </div>
      ) : (
        <>
          <table className="cw-table" aria-labelledby="cluster-list-title">
            <thead><tr><th scope="col">集群 / 命名空间</th><th scope="col">状态</th><th scope="col">版本</th>
              <th scope="col">最近状态观测 <small>本地时间</small></th><th scope="col">标签</th></tr></thead>
            <tbody>
              {rows.map((cluster) => {
                const key = `${cluster.namespace ?? ''}/${cluster.name}`;
                const detailId = `${idPrefix}-${key}`;
                const isOpen = expanded === key;
                return <Fragment key={key}><tr>
                  <th scope="row"><span className="cluster-name-line"><strong>{cluster.name}</strong>
                    <CopyButton value={cluster.name} label={`复制集群 ID ${cluster.name}`} /></span><small>{cluster.namespace || '集群级'}</small>
                    <button type="button" className="cluster-inspect" aria-expanded={isOpen} aria-controls={isOpen ? detailId : undefined}
                      aria-label={`${isOpen ? '收起' : '查看'} ${cluster.name} 的集群诊断`} onClick={() => setExpanded(isOpen ? '' : key)}>
                      {isOpen ? '收起诊断' : '查看诊断'}
                    </button>
                  </th>
                  <td data-label="状态"><span className={`cw-state ${cluster.online ? 'cw-good' : cluster.status === 'Unknown' ? 'cw-warning' : 'cw-bad'}`}>
                    {cluster.online ? '在线' : cluster.status === 'Unknown' ? '状态未知' : '未就绪'}
                  </span></td>
                  <td data-label="版本">{cluster.kubernetesVersion || '未上报'}
                    {cluster.agentVersion && <small>Agent {cluster.agentVersion}</small>}</td>
                  <td data-label="最近状态观测"><ReportedTime value={cluster.lastObservedTime} /></td>
                  <td data-label="标签"><ClusterLabels key={cluster.uid} cluster={cluster} editable={labelEditing} onSaved={onSaved} /></td>
                </tr>
                {isOpen && <tr className="cluster-detail-row" id={detailId}><td colSpan={5}>
                  <ClusterEvidence cluster={cluster} />
                </td></tr>}
                </Fragment>;
              })}
            </tbody>
          </table>
          {numPages > 1 && (
            <footer className="cw-footer">
              <span aria-live="polite">显示 {start + 1}–{start + rows.length} / 共 {clusters.length} 项</span>
              <nav className="cw-paging" aria-label="集群分页">
                <button type="button" disabled={page === 1} onClick={() => setPage(page - 1)}><ChevronLeft size={14} />上一页</button>
                <span>{page} / {numPages}</span>
                <button type="button" disabled={page === numPages} onClick={() => setPage(page + 1)}>下一页<ChevronRight size={14} /></button>
              </nav>
            </footer>
          )}
        </>
      )}
    </section>
  );
}
