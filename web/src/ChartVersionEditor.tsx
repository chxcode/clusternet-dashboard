import { useCallback, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { ListTree, Pencil, X } from 'lucide-react';
import type { HelmChart } from './api';
import { apiPath } from './basePath';
import { useResource } from './useResource';
import './clusterLabels.css';
import './chartVersionEditor.css';

type VersionImpact = {
  uid: string;
  namespace: string;
  name: string;
  currentVersion: string;
  fingerprint: string;
  subscriptions: { namespace: string; name: string; boundTargets: number }[];
  boundTargets: number;
  targets: string[];
};

const versionFormat = /^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;
function validVersion(version: string) {
  const matches = versionFormat.exec(version);
  return Boolean(matches && !matches[4]?.split('.').some(part => /^0\d+$/.test(part)));
}

function responseMessage(status: number) {
  switch (status) {
    case 400: case 422: return '版本修改被拒绝，请核对版本格式和集群策略。';
    case 401: return '登录状态已失效，请重新登录。';
    case 403: return '权限不足，请核对版本编辑开关和集群授权。';
    case 404: return '源 HelmChart 已不存在，请刷新列表。';
    case 409: return '源 HelmChart 或影响范围已变更，请重新预览后确认。';
    default: return `请求失败（HTTP ${status}），请重新预览并核对源版本。`;
  }
}

function VersionDialog({ chart, editable, onClose, onSaved }: {
  chart: HelmChart; editable: boolean; onClose: () => void; onSaved: (version: string) => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [revision, setRevision] = useState(0);
  const [version, setVersion] = useState('');
  const [saving, setSaving] = useState(false);
  const [failure, setFailure] = useState('');
  const [needsPreview, setNeedsPreview] = useState(false);
  const url = apiPath('/helmcharts/' + encodeURIComponent(chart.namespace) + '/' + encodeURIComponent(chart.name));
  const load = useCallback(async (signal: AbortSignal): Promise<VersionImpact> => {
    const response = await fetch(url + '/version-impact', { signal, credentials: 'same-origin' });
    if (response.status === 403) throw new Error('读取权限不足，请核对 HelmChart 的读取权限和订阅的跨命名空间列表权限。');
    if (!response.ok) throw new Error(responseMessage(response.status));
    const impact: VersionImpact = await response.json();
    if (!impact.uid || !impact.fingerprint || impact.name !== chart.name || impact.namespace !== chart.namespace ||
        typeof impact.currentVersion !== 'string' || !Array.isArray(impact.subscriptions) || !Array.isArray(impact.targets)) {
      throw new Error('影响范围不完整，请重新预览。');
    }
    return impact;
  }, [url, chart.name, chart.namespace]);
  const preview = useResource(load, true, revision, String(revision));
  const impact = preview.data;
  useEffect(() => {
    const node = dialog.current!;
    if (typeof node.showModal === 'function') node.showModal();
    else node.setAttribute('open', '');
    return () => { if (typeof node.close === 'function') node.close(); };
  }, []);
  const next = version.trim();
  const validation = !next ? '' : !validVersion(next) ? '请输入精确版本，例如 1.2.3 或 1.2.3-rc.1。'
    : next.replace(/^v/, '') === impact?.currentVersion.replace(/^v/, '') ? '请输入与当前版本不同的版本。' : '';
  const canSave = Boolean(editable && impact && !preview.loading && !preview.error && !needsPreview && !saving && next && !validation);
  async function save() {
    if (!canSave || !impact) return;
    setSaving(true);
    setFailure('');
    try {
      const { uid, fingerprint, currentVersion } = impact;
      const response = await fetch(url + '/version', {
        method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ uid, fingerprint, currentVersion, newVersion: next }),
      });
      if (!response.ok) {
        setFailure(responseMessage(response.status));
        setNeedsPreview(![400, 422].includes(response.status));
        return;
      }
      const result: { version: string } = await response.json();
      onSaved(result.version);
    } catch {
      setFailure('未收到保存结果，请重新预览并核对源版本。');
      setNeedsPreview(true);
    } finally { setSaving(false); }
  }
  function reload() {
    setFailure('');
    setNeedsPreview(false);
    setRevision(n => n + 1);
  }
  return createPortal(<dialog ref={dialog} className="label-editor chart-version-dialog" aria-labelledby="chart-version-title"
    onCancel={event => { event.preventDefault(); if (!saving) onClose(); }}>
    <header><h2 id="chart-version-title">{editable ? '更改 Chart 版本' : 'HelmChart 影响范围'}</h2>
      <button type="button" className="label-icon" aria-label="关闭窗口" disabled={saving} onClick={onClose}><X size={17} /></button></header>
    <p className="label-target">HelmChart · {chart.namespace} / {chart.name}</p>
    {preview.loading && <p role="status">正在读取源版本和引用订阅…</p>}
    {preview.error && <p className="label-error" role="alert">{preview.error}</p>}
    {impact && <>
      <p>当前源版本 <strong>{impact.currentVersion || '未指定'}</strong></p>
      {editable && <label className="chart-version-input">新版本<input value={version} spellCheck={false} autoComplete="off"
        placeholder="例如 1.2.3" disabled={saving || needsPreview} onChange={event => { setVersion(event.target.value); setFailure(''); }} /></label>}
      {editable && validation && <p className="label-error" role="alert">{validation}</p>}
      {editable && <h3>影响范围</h3>}
      <p>{impact.subscriptions.length} 个直接引用的订阅 · {impact.boundTargets} 个当前绑定目标（去重）</p>
      {impact.subscriptions.length > 0 && <ul className="chart-version-subs">{impact.subscriptions.map(sub =>
        <li key={sub.namespace + '/' + sub.name}><span>{sub.namespace} / {sub.name}</span><span>{sub.boundTargets} 个绑定目标</span></li>)}</ul>}
      {impact.targets.length > 0 && <details className="label-change-details"><summary>查看绑定目标标识</summary>
        <ul className="chart-version-targets">{impact.targets.map(target => <li key={target}><code>{target}</code></li>)}</ul></details>}
      {editable && <p className="label-impact">保存将修改父集群中的源 HelmChart，正常运行的目标也可能升级。请先确认仓库中存在该版本，保存后在发布记录中核对子集群部署结果。</p>}
    </>}
    {failure && <p className="label-error" role="alert">{failure}</p>}
    <footer><button type="button" onClick={onClose} disabled={saving}>{editable ? '取消' : '关闭'}</button>
      {needsPreview || preview.error ? <button type="button" className="label-save" onClick={reload}>重新预览</button>
      : editable && <button type="button" className="label-save" onClick={save} disabled={!canSave}>{saving ? '保存中…' : '确认更改版本'}</button>}
    </footer>
  </dialog>, document.body);
}

export function ChartVersionEditor({ chart, editable = false, onSaved }: { chart: HelmChart; editable?: boolean; onSaved?: () => void }) {
  const [open, setOpen] = useState(false);
  const [saved, setSaved] = useState<string>();
  return <>
    <button className="label-edit" type="button" onClick={() => setOpen(true)}>
      {editable ? <><Pencil size={13} />更改 Chart 版本</> : <><ListTree size={13} />查看影响范围</>}
    </button>
    {editable && saved && <small role="status">源版本已保存为 {saved}</small>}
    {open && <VersionDialog key={String(editable)} chart={chart} editable={editable} onClose={() => setOpen(false)}
      onSaved={version => { setSaved(version); setOpen(false); onSaved?.(); }} />}
  </>;
}
