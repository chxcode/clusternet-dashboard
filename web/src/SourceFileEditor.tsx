import { useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { FileCode2, FilePlus2, X } from 'lucide-react';
import { parseDocument } from 'yaml';
import { downloadYAML } from './yamlDownload';
import { useYamlAnalysis, checkYamlSyntax, YamlEditor } from './YamlEditor';
import { apiPath } from './basePath';
import type { HelmChart } from './api';
import './clusterLabels.css';
import './sourceFileEditor.css';

const chartTemplate = `apiVersion: apps.clusternet.io/v1alpha1
kind: HelmChart
metadata:
  namespace: default
  name: sample-web
spec:
  repo: https://charts.example.com
  chart: sample-web
  version: "1.0.0"
  targetNamespace: workloads
  createNamespace: true
  wait: true
  timeoutSeconds: 300
`;
const fileLimit = 16 * 1024;

type Identity = { namespace: string; name: string; uid: string; resourceVersion: string };
type DraftIdentity = Pick<Identity, 'namespace' | 'name'>;
type Impact = DraftIdentity & {
  mode: string;
  changedFields: string[];
  changes: { field: string; before: unknown; after: unknown }[];
  subscriptions: { namespace: string; name: string; boundTargets: number }[];
  targets: string[];
  fingerprint: string;
};
type Props = { chart?: HelmChart; onSaved?: () => void };

function sourceIdentity(raw: string): DraftIdentity {
  if (new Blob([raw]).size > fileLimit) throw new Error('HelmChart 文件最大为 16 KiB。');
  const doc = parseDocument(raw, { prettyErrors: false });
  if (doc.errors.length) throw new Error('YAML 格式有误，请检查缩进、重复键和文档数量。');
  const value = doc.toJS({ maxAliasCount: 0 });
  if (value?.kind !== 'HelmChart' || value.apiVersion !== 'apps.clusternet.io/v1alpha1') {
    throw new Error('请选择 apps.clusternet.io/v1alpha1 的 HelmChart 文件。');
  }
  const { namespace, name } = value.metadata || {};
  if (typeof namespace !== 'string' || typeof name !== 'string' || !namespace || !name) {
    throw new Error('请填写 metadata.namespace 和 metadata.name。');
  }
  return { namespace, name };
}

async function sourceRequest(id: DraftIdentity, step: string, body: object, signal: AbortSignal) {
  const path = '/helmcharts/' + encodeURIComponent(id.namespace) + '/' + encodeURIComponent(id.name) + '/source' + step;
  const response = await fetch(apiPath(path), { method: 'POST', credentials: 'same-origin', signal,
    headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  if (!response.ok) {
    const detail = await response.json().catch(() => ({}));
    const hint = response.status === 409 ? '配置或关联订阅已变更，或资源已存在。请核对后重新预览。'
      : response.status === 403 ? '当前访问没有配置编辑权限，请检查开关、域名与集群授权。' : '请求未完成。';
    throw new Error(hint + (typeof detail?.error === 'string' ? ' ' + detail.error : ' HTTP ' + response.status));
  }
  return response.json();
}

function showValue(value: unknown) {
  return value === null || value === undefined ? '未设置' : String(value);
}

function SourceDialog({ chart, onSaved, close }: Props & { close: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const pending = useRef<AbortController | null>(null);
  const [revision, reload] = useState(0);
  const [source, setSource] = useState<Identity>();
  const [yaml, setYaml] = useState(chart ? '' : chartTemplate);
  const analysis = useYamlAnalysis(yaml, true);
  const syntaxReady = analysis.source === yaml && !analysis.tooLarge && !analysis.issues.length;
  const tooLarge = useMemo(() => yaml.length > fileLimit || new Blob([yaml]).size > fileLimit, [yaml]);
  const [plan, setPlan] = useState<Impact>();
  const [saved, setSaved] = useState<Identity>();
  const [busy, setBusy] = useState<'' | 'read' | 'import' | 'preview' | 'save'>(chart ? 'read' : '');
  const [error, setError] = useState('');
  const [readAgain, setReadAgain] = useState(false);
  const mode = chart ? 'edit' : 'create';
  const input = useRef<HTMLInputElement>(null);
  const chartNS = chart?.namespace, chartName = chart?.name;
  useEffect(() => {
    const el = dialog.current!;
    if (el.showModal) el.showModal(); else el.setAttribute('open', '');
    return () => { pending.current?.abort(); el.close?.(); };
  }, []);
  useEffect(() => {
    if (!chartNS || !chartName) return;
    const ctrl = new AbortController();
    pending.current = ctrl;
    setBusy('read'); setError(''); setReadAgain(false); setPlan(undefined); setSource(undefined);
    sourceRequest({ namespace: chartNS, name: chartName }, '', {}, ctrl.signal).then(doc => {
      if (ctrl.signal.aborted) return;
      if (!doc?.uid || !doc.resourceVersion || doc.namespace !== chartNS || doc.name !== chartName || typeof doc.yaml !== 'string') {
        throw new Error('未取得可编辑的源配置。');
      }
      setSource(doc); setYaml(doc.yaml);
    }).catch(cause => { if (!ctrl.signal.aborted) setError(cause.message); })
      .finally(() => { if (!ctrl.signal.aborted) { pending.current = null; setBusy(''); } });
    return () => ctrl.abort();
  }, [chartNS, chartName, revision]);

  async function submit(save: boolean) {
    if (pending.current || saved || !syntaxReady || tooLarge || (save && !plan) || (chart && !source)) return;
    const ctrl = new AbortController();
    pending.current = ctrl; setBusy(save ? 'save' : 'preview'); setError('');
    try {
      const id = sourceIdentity(yaml);
      if (chart && (id.name !== chart.name || id.namespace !== chart.namespace)) throw new Error('编辑时请保留原资源名称和命名空间。');
      const body = { mode, yaml, uid: source?.uid, resourceVersion: source?.resourceVersion, fingerprint: save ? plan!.fingerprint : undefined };
      const result = await sourceRequest(id, save ? '-save' : '-preview', body, ctrl.signal);
      if (ctrl.signal.aborted) return;
      if (!result || result.name !== id.name || result.namespace !== id.namespace) throw new Error('回执中的资源身份与当前文件不符。');
      if (save) {
        if (!result.uid || !result.resourceVersion || (source && source.uid !== result.uid)) throw new Error('未取得有效保存回执，请核对 HelmChart 列表。');
        setSaved(result); onSaved?.();
      } else {
        if (result.mode !== mode || !result.fingerprint || !Array.isArray(result.changes) || !Array.isArray(result.subscriptions) || !Array.isArray(result.targets)) {
          throw new Error('预览没有返回完整的变更和分发范围。');
        }
        setPlan(result);
      }
    } catch (cause) {
      if (!ctrl.signal.aborted) {
        setError((cause instanceof Error ? cause.message : '操作失败。') + (save ? ' 请在 HelmChart 列表核对保存结果。' : ''));
        setPlan(undefined); setReadAgain(Boolean(chart));
      }
    } finally { if (pending.current === ctrl) pending.current = null; if (!ctrl.signal.aborted) setBusy(''); }
  }
  async function importFile(file?: File) {
    if (!file || pending.current) return;
    const ctrl = new AbortController();
    pending.current = ctrl; setBusy('import'); setError('');
    try {
      if (file.size > fileLimit) throw new Error('HelmChart 文件最大为 16 KiB。');
      const raw = await file.text();
      if (ctrl.signal.aborted) return;
      const [issue] = checkYamlSyntax(raw, true);
      if (issue) throw new Error(`导入失败 · 第 ${issue.line} 行，第 ${issue.column} 列 · ${issue.message}`);
      sourceIdentity(raw); setYaml(raw);
    } catch (cause) {
      if (!ctrl.signal.aborted) setError(cause instanceof Error ? cause.message : '文件读取失败。');
    } finally {
      if (pending.current === ctrl) pending.current = null;
      if (!ctrl.signal.aborted) setBusy('');
    }
  }

  return createPortal(<dialog ref={dialog} className="label-editor source-editor" aria-labelledby="source-title"
    onCancel={event => { event.preventDefault(); if (busy !== 'save') close(); }}>
    <header><h2 id="source-title">{chart ? '编辑 HelmChart 配置' : '新建 HelmChart'}</h2>
      <button type="button" className="label-icon" disabled={busy === 'save'} aria-label="关闭配置窗口" onClick={close}><X size={17}/></button></header>
    {saved ? <p role="status">已保存父集群 HelmChart <strong>{saved.namespace}/{saved.name}</strong>。可在 HelmChart 列表查看阶段及分发操作。</p>
      : plan ? <section aria-label="配置变更预览">
        <h3>{mode === 'create' ? '即将创建' : '即将更新'} · {plan.namespace}/{plan.name}</h3>
        <div className="source-diff"><table><thead><tr><th>字段</th><th>当前值</th><th>保存后</th></tr></thead>
          <tbody>{plan.changes.map(c => <tr key={c.field}><th>{c.field}</th><td>{showValue(c.before)}</td><td>{showValue(c.after)}</td></tr>)}</tbody></table></div>
        <h3>直接引用的订阅 · {plan.subscriptions.length}</h3>
        {plan.subscriptions.length ? <><p>配置保存后可被这些订阅分发，当前共上报 {plan.targets.length} 个不同的绑定目标。标签订阅的覆盖范围会随集群变化。</p>
          <ul className="source-refs">{plan.subscriptions.map(s => <li key={s.namespace + '/' + s.name}>{s.namespace}/{s.name} · 已绑定 {s.boundTargets}</li>)}</ul></>
          : <p>当前没有订阅直接引用此 Chart。保存后可从 HelmChart 列表创建部署订阅。</p>}
      </section> : <>
        {busy === 'read' && <p role="status">正在读取源配置…</p>}
        {(!chart || source) && <><p className="label-target">{chart ? chart.namespace + '/' + chart.name : '在 metadata 中指定源 HelmChart 的名称和命名空间。'}</p>
          <div className="source-toolbar"><button type="button" disabled={!!busy} onClick={() => input.current?.click()}>导入 YAML</button>
            <input ref={input} type="file" hidden aria-label="导入 HelmChart YAML 文件" disabled={!!busy} accept=".yaml,.yml,.json" onChange={event => {
              const file = event.currentTarget.files?.[0]; event.currentTarget.value = '';
              void importFile(file);
            }}/>
            <button type="button" disabled={!yaml || !!busy} onClick={() => downloadYAML(yaml, (chart?.name || 'helmchart') + '.yaml')}>下载 YAML</button></div>
          <YamlEditor label="HelmChart YAML" value={yaml} analysis={analysis} singleDocument disabled={!!busy}
            onChange={value => { setYaml(value); setError(''); }}/>
          {tooLarge && <p role="alert" className="label-error">HelmChart 文件最大为 16 KiB。</p>}</>}
      </>}
    {error && <div role="alert" className="label-error">{error}</div>}
    <footer><button type="button" disabled={busy === 'save'} onClick={close}>{saved ? '完成' : '取消'}</button>
      {chart && !saved && !plan && (!source || readAgain) && <button disabled={!!busy} onClick={() => reload(n => n + 1)}>重新载入集群版本</button>}
      {plan && <button type="button" disabled={!!busy} onClick={() => setPlan(undefined)}>返回编辑</button>}
      {!saved && <button type="button" className="label-save" disabled={!!busy || !yaml.trim() || !syntaxReady || tooLarge || Boolean(chart && !source)}
        onClick={() => submit(!!plan)}>{busy === 'save' ? '正在保存…' : busy === 'preview' ? '正在预览…' : plan ? '确认保存配置' : '预览配置变更'}</button>}
    </footer>
  </dialog>, document.body);
}

export function SourceFileEditor(props: Props) {
  const [open, setOpen] = useState(false);
  const Icon = props.chart ? FileCode2 : FilePlus2;
  return <><button type="button" className="label-edit" onClick={() => setOpen(true)}><Icon size={14} aria-hidden="true"/>
    {props.chart ? '编辑配置' : '新建 HelmChart'}</button>
    {open && <SourceDialog {...props} close={() => setOpen(false)}/>}</>;
}
