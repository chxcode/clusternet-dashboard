import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { pathForPage } from './routing';
import { downloadYAML } from './yamlDownload';
import { WorkloadLabelPicker } from './WorkloadLabelPicker';
import { useYamlAnalysis, MAX_YAML_BYTES, YamlEditor } from './YamlEditor';
import { appendDocument, templates, validate, type Kind } from './workloadDraft';
import { actionText, NativeRequestError, nativeRequest, sourceIdentities, sourcePath, subscriptionId, validNamespace,
  validPlan, validReceipt, validSubscriptionName, workloadPath, type Baselines, type Identity,
  type Mode, type Origin, type Plan, type PlanResource, type Receipt } from './nativeWorkload';
import './clusterLabels.css';
import './workloadDraftEditor.css';

const detailURL = (id: Identity) => pathForPage('propagation-status') + '?' + new URLSearchParams({ namespace: id.namespace, name: id.name });
const errorMessage = (error: unknown) => error instanceof TypeError ? '连接中断，请核对当前状态。' : error instanceof Error ? error.message : '操作未完成。';
function Resources({ resources }: { resources: PlanResource[] }) {
  return <ul className="native-resources">{resources.map(r => <li key={r.kind + '/' + r.name}>
    <span>{actionText[r.action]}</span> <code>{r.kind} · {r.namespace}/{r.name}</code></li>)}</ul>;
}
function Preview({ plan }: { plan: Plan }) {
  return <section aria-label="发布预览">
    <h3>源文件变更 · {plan.resources.length}</h3><Resources resources={plan.resources}/>
    <h3>{actionText[plan.subscriptionAction]}订阅 · {plan.namespace}/{plan.name}</h3>
    <p>{plan.targetMode === 'all' ? '持续覆盖所有已注册及后续注册的集群。' : <>持续按标签匹配 <code>{plan.selector}</code>，后续符合条件的集群也会加入候选范围。</>}</p>
    <details><summary>当前匹配集群 · {plan.targets.length}</summary>
      {plan.targets.length ? <ul>{plan.targets.map(t => <li key={t.uid || t.namespace + '/' + t.name}>{t.namespace}/{t.name} {t.clusterId && <code> · {t.clusterId}</code>}</li>)}</ul>
      : <p>当前没有匹配集群，仍可保存规则，等待后续集群匹配。</p>}</details>
    <p className="label-target">父集群保存源文件和订阅后，由 Clusternet 调度。实际绑定与工作负载回报可在分发详情查看。</p>
    {plan.scopeWarnings.length > 0 && <ul className="label-impact" aria-label="影响范围提示">{plan.scopeWarnings.map((s, i) => <li key={i}>{s}</li>)}</ul>}
    {plan.subscriptionYAML && <details><summary>查看订阅 YAML</summary><pre>{plan.subscriptionYAML}</pre>
      <button type="button" className="text-button" onClick={() => downloadYAML(plan.subscriptionYAML, plan.name + '-subscription.yaml')}>下载订阅 YAML</button></details>}
    {plan.conflicts.length > 0 && <div className="label-error" role="alert"><strong>存在发布冲突</strong><ul>{plan.conflicts.map((s, i) => <li key={i}>{s}</li>)}</ul></div>}
  </section>;
}
type Props = { origin: Origin; onClose: () => void; onPublished?: () => void };

export function WorkloadPublishDialog({ origin: initial, onClose, onPublished }: Props) {
  const [origin, setOrigin] = useState(initial);
  const [revision, setRevision] = useState(0);
  const [namespace, setNamespace] = useState(initial.type === 'new' ? '' : initial.id.namespace);
  const [name, setName] = useState(initial.type === 'subscription' ? initial.id.name : '');
  const [kind, setKind] = useState<Kind>('Deployment');
  const [yaml, setYaml] = useState(initial.type === 'new' ? initial.yaml || '' : '');
  const analysis = useYamlAnalysis(yaml);
  const syntaxReady = analysis.source === yaml && !analysis.tooLarge && !analysis.issues.length;
  const [originalYaml, setOriginalYaml] = useState('');
  const [mode, setMode] = useState<Mode>('selector');
  const [selector, setSelector] = useState('');
  const [baseline, setBaseline] = useState<string>();
  const [sourceBaselines, setSourceBaselines] = useState<Baselines>({});
  const [references, setReferences] = useState<Identity[]>([]);
  const [loaded, setLoaded] = useState(initial.type === 'new');
  const [plan, setPlan] = useState<Plan>();
  const [receipt, setReceipt] = useState<Receipt>();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState<'' | 'read' | 'import' | 'preview' | 'apply'>(initial.type === 'new' ? '' : 'read');
  const dialog = useRef<HTMLDialogElement>(null);
  const pending = useRef<AbortController | null>(null);
  useEffect(() => {
    const node = dialog.current!;
    if (node.showModal) node.showModal(); else node.setAttribute('open', '');
    return () => { pending.current?.abort(); node.close?.(); };
  }, []);
  useEffect(() => {
    if (origin.type === 'new') return;
    const ctrl = new AbortController();
    const id = origin.id;
    pending.current = ctrl; setLoaded(false); setBusy('read'); setError(''); setPlan(undefined); setReceipt(undefined);
    setYaml(''); setReferences([]); setBaseline(undefined); setSourceBaselines({});
    const path = origin.type === 'source' ? sourcePath(origin.id) : workloadPath(id) + '/source';
    nativeRequest(path, {}, ctrl).then(({ data }) => {
      if (ctrl.signal.aborted) return;
      if (!data || data.name !== id.name || data.namespace !== id.namespace || typeof data.yaml !== 'string' || typeof data.baseline !== 'string' || !data.baseline) {
        throw new Error('源文件身份或版本不匹配，请刷新列表。');
      }
      const sources = sourceIdentities(data.yaml, id.namespace);
      if (origin.type === 'source') {
        const target = origin.id;
        if (data.kind !== target.kind || data.apiVersion !== target.apiVersion || !data.uid || sources.length !== 1 ||
            sources[0].kind !== target.kind || sources[0].name !== target.name || !Array.isArray(data.subscriptions)) {
          throw new Error('文件类型、名称或引用信息与所选清单不符。');
        }
        const refs = Array.from<string>(data.subscriptions).map(subscriptionId);
        if (refs.some(ref => !ref)) throw new Error('无法识别此文件的订阅引用。');
        setReferences(refs as Identity[]); setName(''); setMode('selector'); setSelector('');
        setSourceBaselines({ [target.kind + '/' + target.name]: data.baseline });
      } else {
        if ((data.targetMode !== 'selector' && data.targetMode !== 'all') || (data.targetMode === 'selector' && !data.selector) ||
           !data.sourceBaselines || !sources.every(s => typeof data.sourceBaselines[s.kind + '/' + s.name] === 'string' && data.sourceBaselines[s.kind + '/' + s.name])) {
          throw new Error('该订阅未提供可编辑的原生文件或目标规则。');
        }
        setName(id.name); setMode(data.targetMode); setSelector(data.selector || ''); setBaseline(data.baseline); setSourceBaselines(data.sourceBaselines);
      }
      setNamespace(id.namespace); setYaml(data.yaml); setOriginalYaml(data.yaml); setLoaded(true);
    }).catch(cause => { if (!ctrl.signal.aborted) setError(errorMessage(cause)); })
      .finally(() => { if (pending.current === ctrl) pending.current = null; if (!ctrl.signal.aborted) setBusy(''); });
    return () => ctrl.abort();
  }, [origin, revision]);

  async function submit(save: boolean) {
    if (pending.current || !loaded || references.length || receipt || !syntaxReady || (save && (!plan || plan.conflicts.length))) return;
    const ctrl = new AbortController(); pending.current = ctrl;
    setBusy(save ? 'apply' : 'preview'); setError('');
    try {
      const resources = sourceIdentities(yaml, namespace);
      const check = validate(yaml);
      if (check.errors.length) throw new Error(check.errors.join('\n'));
      if (origin.type === 'source' && !resources.some(r => r.kind === origin.id.kind && r.name === origin.id.name)) {
        throw new Error('请保留所选源文件的类型、名称和命名空间。');
      }
      const body = { yaml, targetMode: mode, ...(mode === 'selector' ? { selector: selector.trim() } : {}),
        ...(baseline ? { baseline } : {}), ...(Object.keys(sourceBaselines).length ? { sourceBaselines } : {}),
        ...(save ? { fingerprint: plan!.fingerprint } : {}) };
      const { status, data } = await nativeRequest(workloadPath({ namespace, name }) + (save ? '/apply' : '/preview'), body, ctrl);
      if (ctrl.signal.aborted) return;
      if (save) {
        if (!validReceipt(data, status, plan!)) throw new Error('保存回执不完整，请核对源文件和订阅。');
        setReceipt(data); onPublished?.();
      } else {
        if (!validPlan(data, { name, namespace }, mode, resources)) throw new Error('预览未能核对源文件、订阅和目标范围。');
        setPlan(data);
      }
    } catch (cause) {
      if (!ctrl.signal.aborted) {
        setError(errorMessage(cause)); setPlan(undefined);
        if (save && (!(cause instanceof NativeRequestError) || cause.status >= 500)) {
          setReceipt({ status: 'uncertain', resources: [], subscriptionSaved: false });
          onPublished?.();
        }
      }
    } finally { if (pending.current === ctrl) pending.current = null; if (!ctrl.signal.aborted) setBusy(''); }
  }

  async function importFile(file?: File) {
    if (!file || pending.current) return;
    const ctrl = new AbortController();
    pending.current = ctrl;
    setBusy('import'); setError('');
    try {
      if (file.size > MAX_YAML_BYTES) throw new Error('源文件总量超过 256 KiB。');
      const text = await file.text();
      if (ctrl.signal.aborted) return;
      if (!text.trim()) throw new Error('导入的文件为空。');
      setYaml(old => appendDocument(old, text));
    } catch (e) {
      if (!ctrl.signal.aborted) setError(errorMessage(e));
    } finally {
      if (pending.current === ctrl) pending.current = null;
      if (!ctrl.signal.aborted) setBusy('');
    }
  }
  const eligible = loaded && !references.length && syntaxReady && validNamespace(namespace) && validSubscriptionName(name) && yaml.trim() && (mode === 'all' || selector.trim());
  const canClose = busy !== 'apply';
  return createPortal(<dialog ref={dialog} className="label-editor workload-editor native-publish" aria-labelledby="native-publish-title"
    onCancel={e => { e.preventDefault(); if (canClose) onClose(); }}>
    <header><h2 id="native-publish-title">{origin.type === 'new' ? '新建原生发布' : origin.type === 'source' ? '编辑工作负载源文件' : '编辑原生发布'}</h2>
      <button type="button" className="label-icon" disabled={!canClose} onClick={onClose} aria-label="关闭发布窗口"><X size={17}/></button></header>
    {receipt ? <section aria-label="发布回执">
      <h3>{receipt.status === 'saved' ? '源文件与订阅已保存' : receipt.status === 'partial' ? '部分内容已保存' : '提交结果尚未确认'}</h3>
      <p><strong>{namespace}/{name}</strong> · {receipt.subscriptionSaved ? '已确认保存订阅' : '尚未确认保存订阅'}</p>
      {receipt.resources.length > 0 && <><h4>已确认保存的源文件</h4><Resources resources={receipt.resources}/></>}
      {receipt.message && <p>{receipt.message}</p>}
      <p className="label-impact">{receipt.status === 'saved' ? '调度器会根据订阅选择目标。请在分发详情查看实际绑定与回报。'
        : '已接受的变更会保留。请核对源文件与订阅后，再用当前草稿重新预览。'}</p>
      {(receipt.subscriptionSaved || receipt.status === 'uncertain' || origin.type === 'subscription') &&
        <a className="inventory-link" href={detailURL({ namespace, name })}>查看分发详情</a>}
    </section> : plan ? <><Preview plan={plan}/>
      <details><summary>{originalYaml && originalYaml !== yaml ? '对照修改前后的源文件' : '核对工作负载 YAML'}</summary>
        {originalYaml && originalYaml !== yaml && <><h4>读取时的版本</h4><pre>{originalYaml}</pre><h4>准备发布的版本</h4></>}
        <pre>{yaml}</pre></details></> : <>
      {busy === 'read' && <p role="status">正在读取源文件与订阅引用…</p>}
      {loaded && <>{origin.type === 'source' && <p className="label-target">{origin.id.kind} · {origin.id.namespace}/{origin.id.name}</p>}
        {references.length > 0 ? <>
          <h3>已有订阅直接引用此源 · {references.length}</h3><p>修改共享源可能影响这些订阅的目标集群。打开对应发布即可查看完整工作负载及调度范围。</p>
          <ul>{references.map(ref => <li key={ref.namespace + '/' + ref.name}><button type="button" className="text-button" onClick={() => setOrigin({ type: 'subscription', id: ref })}>打开发布 {ref.namespace}/{ref.name}</button>
            {' · '}<a className="inventory-link" href={detailURL(ref)}>分发详情</a></li>)}</ul>
        </> : <>
          <fieldset disabled={!!busy} className="native-grid">
            <label>命名空间<input value={namespace} onChange={e => setNamespace(e.target.value)} readOnly={origin.type !== 'new'} placeholder="应用命名空间"/></label>
            <label>订阅名称<input value={name} onChange={e => setName(e.target.value)} readOnly={origin.type === 'subscription'} maxLength={52} placeholder="例如 web-rollout"/></label>
            <label>目标范围<select value={mode} onChange={e => setMode(e.target.value as Mode)}><option value="selector">按标签持续匹配</option><option value="all">全部集群（持续）</option></select></label>
            {mode === 'selector' && <label>标签选择器<input value={selector} onChange={e => setSelector(e.target.value)} placeholder="例如 region=east"/></label>}
          </fieldset>
          {mode === 'selector' && <WorkloadLabelPicker selector={selector} onChange={setSelector} disabled={!!busy}/>}
          <p className="label-target">{origin.type === 'source' ? '此源当前无直接引用，可创建订阅分发。' : '源文件与订阅使用相同的命名空间。'} 最多 16 份文件，合计 256 KiB。订阅名称最多 52 个字符。</p>
        </>}
        <fieldset disabled={!!busy}>
          {!references.length && <div className="workload-toolbar"><label>源模板<select value={kind} onChange={e => setKind(e.target.value as Kind)}>{Object.keys(templates).map(k => <option key={k}>{k}</option>)}</select></label>
            <button type="button" onClick={() => { setYaml(text => appendDocument(text, templates[kind])); setError(''); }}>追加模板</button>
            <label className="workload-import">导入文件<input type="file" accept=".yaml,.yml,.json" onChange={e => { const file = e.target.files?.[0]; e.target.value = ''; void importFile(file); }}/></label></div>}
          <YamlEditor className="workload-source" label="工作负载 YAML" value={yaml} analysis={analysis}
            readOnly={references.length > 0} disabled={!!busy} onChange={value => { setYaml(value); setError(''); }}
            placeholder="可粘贴多份 YAML，以 --- 分隔。"/>
          {analysis.source === yaml && analysis.tooLarge && !references.length && <p className="label-target">
            在线发布上限为 256 KiB。请缩小源文件后重新预览，或下载当前草稿。
          </p>}
        </fieldset>
      </>}
    </>}
    {error && <p role="alert" className="label-error">{error}</p>}
    <footer><button type="button" disabled={!canClose} onClick={onClose}>{receipt?.status === 'saved' ? '完成' : '关闭'}</button>
      {yaml && <button type="button" disabled={!!busy} onClick={() => downloadYAML(yaml, (name || 'workloads') + '.yaml')}>下载源文件</button>}
      {origin.type !== 'new' && error && !receipt && !plan && <button type="button" disabled={!!busy} onClick={() => setRevision(n => n + 1)}>{loaded ? '放弃当前草稿并重读' : '重读集群版本'}</button>}
      {receipt && receipt.status !== 'saved' && <button type="button" onClick={() => { setReceipt(undefined); setPlan(undefined); setError(''); }}>保留草稿，返回核对</button>}
      {plan && !receipt && <button type="button" disabled={!!busy} onClick={() => { setPlan(undefined); setError(''); }}>返回编辑</button>}
      {!receipt && !references.length && <button type="button" className="label-save" disabled={!!busy || !eligible || Boolean(plan?.conflicts.length)} onClick={() => submit(!!plan)}>
        {busy === 'apply' ? '正在提交…' : busy === 'preview' ? '正在预览…' : plan ? '确认发布' : '预览发布范围'}</button>}
    </footer>
  </dialog>, document.body);
}
