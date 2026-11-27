import { Search } from 'lucide-react';
import type { Cluster } from './api';
import './clusters.css';

export type ClusterFilterValue = {
  status: 'all' | 'online' | 'offline' | 'unknown' | 'attention';
  app: string;
  labelQuery: string;
};

type ClusterFiltersProps = {
  clusters: Cluster[];
  value: ClusterFilterValue;
  onChange: (value: ClusterFilterValue) => void;
};

export function ClusterFilters({ clusters, value, onChange }: ClusterFiltersProps) {
  const apps = new Set(clusters.flatMap((cluster) => cluster.labels?.app ? [cluster.labels.app] : []));
  if (value.app !== 'all') apps.add(value.app);
  return (
    <section className="cw-toolbar" aria-label="集群筛选">
      <label className="cw-search">
        <span>搜索集群</span>
        <span className="cw-input-wrap">
          <Search size={16} aria-hidden="true" />
          <input type="search" aria-label="搜索集群" value={value.labelQuery} placeholder="名称、注册 ID、命名空间或标签"
            onChange={(event) => onChange({ ...value, labelQuery: event.target.value })} />
        </span>
      </label>
      <label>
        <span>状态</span>
        <select aria-label="按状态筛选" value={value.status}
          onChange={(event) => onChange({ ...value, status: event.target.value as ClusterFilterValue['status'] })}>
          <option value="all">全部状态</option>
          <option value="online">在线</option>
          <option value="attention">全部待排查</option>
          <option value="offline">未就绪</option>
          <option value="unknown">状态未知</option>
        </select>
      </label>
      <label>
        <span>应用</span>
        <select aria-label="按应用筛选" value={value.app} onChange={(event) => onChange({ ...value, app: event.target.value })}>
          <option value="all">全部应用</option>
          {[...apps].sort().map((app) => <option value={app} key={app}>{app}</option>)}
        </select>
      </label>
      {(value.app !== 'all' || value.status !== 'all' || value.labelQuery !== '') && (
        <button type="button" className="cw-clear" onClick={() => onChange({ status: 'all', app: 'all', labelQuery: '' })}>重置条件</button>
      )}
    </section>
  );
}
