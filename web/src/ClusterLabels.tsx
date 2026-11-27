import { useEffect, useRef, useState } from 'react';
import { Pencil, Plus, Trash2, X } from 'lucide-react';
import type { Cluster } from './api';
import { LabelSaveError, saveClusterLabels, type LabelChange } from './clusterLabelsApi';
import './clusterLabels.css';

type Row = { id: number; key: string; value: string };
const labelPart = /^([a-zA-Z0-9]([-a-zA-Z0-9_.]*[a-zA-Z0-9])?)$/;
const prefixPart = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;
function validKey(key: string) {
  const parts = key.split('/');
  const name = parts.pop() || '';
  return name.length <= 63 && labelPart.test(name) && (parts.length === 0 ||
    (parts.length === 1 && parts[0].length <= 253 && parts[0].split('.').every(part => prefixPart.test(part))));
}

function changesFor(labels: Record<string, string>, rows: Row[]) {
  const original = new Map(Object.entries(labels));
  const draft = new Map<string, string>();
  const changes: LabelChange[] = [];
  for (const row of rows) {
    if (!validKey(row.key)) return { changes, error: '请输入有效的 Kubernetes 标签键。' };
    if (draft.has(row.key)) return { changes, error: '每个标签键只能出现一次。' };
    if (row.value.length > 63 || (row.value && !labelPart.test(row.value))) return { changes, error: '标签值格式无效，最多 63 个字符。' };
    draft.set(row.key, row.value);
  }
  for (const key of new Set([...original.keys(), ...draft.keys()])) {
    const before = original.get(key) ?? null;
    const after = draft.get(key) ?? null;
    if (before !== after) changes.push({ key, before, after });
  }
  return { changes, error: '' };
}

function LabelEditor({ cluster, onClose, onSave, onReload }: {
  cluster: Cluster; onClose: () => void; onSave: (labels: Record<string,string>) => void; onReload: () => void;
}) {
  const [baseline] = useState(() => cluster.labels || {});
  const [rows, setRows] = useState<Row[]>(() => Object.entries(baseline).sort(([a], [b]) => a.localeCompare(b))
    .map(([key, value], id) => ({ id, key, value })));
  const next = useRef(rows.length);
  const [saving, setSaving] = useState(false);
  const [failure, setFailure] = useState('');
  const [conflict, setConflict] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const node = dialog.current!;
    if (typeof node.showModal === 'function') node.showModal(); else node.setAttribute('open', '');
    return () => { if (typeof node.close === 'function') node.close(); };
  }, []);
  const { changes, error } = changesFor(baseline, rows);
  function edit(id: number, field: 'key' | 'value', text: string) {
    setRows(old => old.map(row => row.id === id ? { ...row, [field]: text } : row));
    setFailure('');
  }
  const changesManagedLabel = changes.some(({key}) => /^(?:[a-z0-9.-]+\.)?(?:clusternet\.io|kubernetes\.io|k8s\.io)\//.test(key));
  async function save() {
    if (!cluster.namespace || !cluster.uid || saving || error || !changes.length || conflict) return;
    setSaving(true); setFailure('');
    try {
      const labels = await saveClusterLabels(cluster.namespace, cluster.name, cluster.uid, changes);
      onSave(labels);
    } catch (cause) {
      setFailure(cause instanceof Error ? cause.message : '无法保存，请检查网络后重试。');
      setConflict(cause instanceof LabelSaveError && (cause.status === 409 || cause.status === 404));
    } finally { setSaving(false); }
  }
  return <dialog ref={dialog} className="label-editor" aria-labelledby="label-editor-title"
    onCancel={event => { event.preventDefault(); if (!saving) onClose(); }}>
    <header><h2 id="label-editor-title">编辑集群标签</h2>
      <button type="button" className="label-icon" aria-label="关闭标签编辑" disabled={saving} onClick={onClose}><X size={17} /></button></header>
    <p className="label-target">{cluster.namespace} / {cluster.name}</p>
    <p className="label-impact">保存将更新此 ManagedCluster 的标签，使用这些标签的订阅可能会重新匹配目标集群。</p>
    <fieldset disabled={saving || conflict}>
      <div className="label-fields">
        {rows.map((row, index) => <div className="label-field-row" key={row.id}>
          <label>标签键<input aria-label={`第 ${index + 1} 个标签键`} value={row.key} onChange={event => edit(row.id, 'key', event.target.value)} /></label>
          <label>标签值<input aria-label={`第 ${index + 1} 个标签值`} value={row.value} onChange={event => edit(row.id, 'value', event.target.value)} /></label>
          <button type="button" className="label-icon label-remove" aria-label={`移除标签 ${row.key || index + 1}`}
            onClick={() => setRows(old => old.filter(item => item.id !== row.id))}><Trash2 size={15} /></button>
        </div>)}
      </div>
      <button type="button" className="label-add" onClick={() => { const id = next.current++; setRows(old => [...old, {id, key:'', value:''}]); }}>
        <Plus size={14} />新增标签</button>
    </fieldset>
    {error && <p className="label-error" role="alert">{error}</p>}
    {!!changes.length && !error && <details className="label-change-details"><summary>待保存的修改（{changes.length}）</summary>
      <ul>{changes.map(change => <li key={change.key}><b>{change.before === null ? '新增' : change.after === null ? '删除' : '修改'}</b>{' '}
        <code>{change.key}</code>{' · '}{change.before !== null && (change.before || '空值')}{change.before !== null && change.after !== null && ' → '}{change.after !== null && (change.after || '空值')}</li>)}</ul>
    </details>}
    {changesManagedLabel && <p className="label-impact">本次修改包含系统标签，可能影响控制器识别集群；控制器也可能再次更新这些值。</p>}
    {failure && <p className="label-error" role="alert">{failure}</p>}
    <footer><button type="button" disabled={saving} onClick={onClose}>取消</button>
      {conflict ? <button type="button" className="label-save" onClick={onReload}>关闭并重新载入</button>
      : <button type="button" className="label-save" disabled={saving || !!error || !changes.length} onClick={save}>
        {saving ? '保存中…' : '保存修改'}</button>}</footer>
  </dialog>;
}

function Chips({ entries }: { entries: [string,string][] }) {
  return <ul className="label-chips">{entries.map(([key,value]) => <li key={key} title={key + '=' + value}><code>{key}={value}</code></li>)}</ul>;
}

export function ClusterLabels({ cluster, editable, onSaved }: { cluster: Cluster; editable: boolean; onSaved?: () => void }) {
  const [open, setOpen] = useState(false);
  const [saved, setSaved] = useState<{ source: Cluster, labels: Record<string,string> }>();
  const labels = saved?.source === cluster ? saved.labels : cluster.labels;
  const entries = Object.entries(labels || {}).sort(([a],[b]) => Number(a.includes('/')) - Number(b.includes('/')) || a.localeCompare(b));
  return <div className="cluster-labels">
    {entries.length === 0 ? <span className="cw-muted">无标签</span> : <Chips entries={entries.slice(0,2)} />}
    {entries.length > 2 && <details><summary>另 {entries.length - 2} 个标签</summary><Chips entries={entries.slice(2)} /></details>}
    {editable && cluster.uid && cluster.namespace && <button type="button" className="label-edit" onClick={() => setOpen(true)}><Pencil size={13} />编辑标签</button>}
    {saved && <small role="status">标签已保存</small>}
    {open && <LabelEditor cluster={{...cluster, labels}} onClose={() => setOpen(false)}
      onSave={updated => { setSaved({source:cluster,labels:updated}); setOpen(false); onSaved?.(); }}
      onReload={() => { setOpen(false); onSaved?.(); }} />}
  </div>;
}
