import { useMemo, useState } from 'react';
import { ChevronDown, ChevronRight, Check, Plus } from 'lucide-react';
import { fetchClusters } from './api';
import { useResource } from './useResource';

const pageSize = 20;
type Props = { selector: string; onChange: (value: string) => void; disabled: boolean };

export function WorkloadLabelPicker({ selector, onChange, disabled }: Props) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [page, setPage] = useState(0);
  const [revision, setRevision] = useState(0);
  const result = useResource(fetchClusters, open, revision);
  const labels = useMemo(() => {
    const counts = new Map<string, number>();
    for (const cluster of result.data || []) {
      for (const [key, value] of Object.entries(cluster.labels || {})) {
        const label = key + '=' + value;
        counts.set(label, (counts.get(label) || 0) + 1);
      }
    }
    return Array.from(counts).sort(([left, a], [right, b]) => b - a || left.localeCompare(right));
  }, [result.data]);
  const search = query.trim().toLowerCase();
  const matches = labels.filter(([pair]) => pair.toLowerCase().includes(search));
  const pages = Math.max(1, Math.ceil(matches.length / pageSize));
  const current = Math.min(page, pages - 1);
  const selected = new Set(selector.split(',').map(part => part.trim().replace(/\s*==?\s*/, '=')));
  const prefix = selector.trim().replace(/,\s*$/, '').trimEnd();
  const Icon = open ? ChevronDown : ChevronRight;

  return <div className="native-label-picker">
    <button type="button" className="label-edit" disabled={disabled} aria-expanded={open} onClick={() => setOpen(value => !value)}>
      <Icon size={14} aria-hidden="true"/>从集群标签选择</button>
    {open && <section aria-label="集群标签选项">
      <label>筛选标签<input type="search" value={query} disabled={disabled} placeholder="输入键或值"
        onChange={e => { setQuery(e.target.value); setPage(0); }}/></label>
      {result.loading ? <p role="status">正在读取集群标签…</p> : result.error ? <p className="label-error" role="alert">
        {result.error}。仍可手工输入标签选择器。 <button type="button" className="text-button" disabled={disabled} onClick={() => setRevision(value => value + 1)}>重新读取标签</button></p>
      : <>
        <p className="label-target">{matches.length} 种标签。数字表示带有该标签的集群数，点选多项会同时要求这些条件。可在选择器中删除条件。</p>
        {matches.length > 0 ? <ul className="native-label-options">{matches.slice(current * pageSize, (current + 1) * pageSize).map(([pair, count]) => {
          const included = selected.has(pair);
          return <li key={pair}><button type="button" aria-label={pair + ' ' + count + ' 个集群' + (included ? ' 已添加' : '')} disabled={disabled || included} onClick={() => onChange(prefix ? prefix + ',' + pair : pair)}>
            {included ? <Check size={12} aria-hidden="true"/> : <Plus size={12} aria-hidden="true"/>}
            <code>{pair}</code><span>{count} 个集群</span>{included && <span>已添加</span>}</button></li>;
        })}</ul> : <p>{search ? '没有符合筛选的标签。' : '集群尚未上报标签。'}</p>}
        {pages > 1 && <div className="native-label-pages"><button type="button" className="text-button" disabled={disabled || current === 0} onClick={() => setPage(current - 1)}>上一页</button>
          <span>{current + 1} / {pages}</span><button type="button" className="text-button" disabled={disabled || current + 1 === pages} onClick={() => setPage(current + 1)}>下一页</button></div>}
      </>}
    </section>}
  </div>;
}
