import type { Cluster } from './api';

export type ClusterFilterValue = {
  status: 'all' | 'online' | 'offline';
  app: string;
  labelQuery: string;
};

type ClusterFiltersProps = {
  clusters: Cluster[];
  value: ClusterFilterValue;
  onChange: (value: ClusterFilterValue) => void;
};

export function ClusterFilters({ clusters, value, onChange }: ClusterFiltersProps) {
  const apps = Array.from(new Set(clusters.map((cluster) => cluster.labels?.app).filter(Boolean) as string[])).sort();

  return (
    <div className="filters" aria-label="cluster filters">
      <label>
        按状态筛选
        <select aria-label="按状态筛选" value={value.status} onChange={(event) => onChange({ ...value, status: event.target.value as ClusterFilterValue['status'] })}>
          <option value="all">全部状态</option>
          <option value="online">在线</option>
          <option value="offline">离线 / 异常</option>
        </select>
      </label>
      <label>
        按应用筛选
        <select aria-label="按应用筛选" value={value.app} onChange={(event) => onChange({ ...value, app: event.target.value })}>
          <option value="all">全部应用</option>
          {apps.map((app) => <option value={app} key={app}>{app}</option>)}
        </select>
      </label>
      <label>
        按标签搜索
        <input aria-label="按标签搜索" value={value.labelQuery} placeholder="app=sample-app / customer=demo" onChange={(event) => onChange({ ...value, labelQuery: event.target.value })} />
      </label>
    </div>
  );
}
