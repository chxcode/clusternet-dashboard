import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Send, X } from 'lucide-react';
import { fetchClusters, type HelmChart } from './api';
import { downloadYAML } from './yamlDownload';
import { apiPath } from './basePath';
import { pathForPage } from './routing';
import { useResource } from './useResource';
import './clusterLabels.css';
import './deploymentDialog.css';

type Target = { namespace: string; name: string; uid: string };
type TargetMode = 'fixed' | 'selector' | 'all';
type Scope = { targetMode: 'fixed'; targets: Target[] } | { targetMode: 'selector'; selector: string } | { targetMode: 'all' };
const modes: Record<TargetMode, string> = { fixed: '固定集群', selector: '按标签持续匹配', all: '全部集群（持续）' };
type Preview = {
  namespace: string; name: string; chartName: string; chartVersion: string; chartPhase?: string; targetNamespace: string;
  targets: (Target & { clusterId?: string })[]; fingerprint: string; conflicts: string[];
  targetMode?: TargetMode; subscriptionYAML?: string; selector?: string; scopeWarnings?: string[];
};
const dnsName = /^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?(?:\.[a-z0-9](?:[-a-z0-9]*[a-z0-9])?)*$/;
const jsonHeaders = { 'Content-Type': 'application/json' };

function validPreview(value: Preview, chart: HelmChart, name: string, scope: Scope) {
  if (!value || value.namespace !== chart.namespace || value.name !== name || value.chartName !== chart.name ||
      typeof value.fingerprint !== 'string' || !value.fingerprint || !Array.isArray(value.targets) ||
      !Array.isArray(value.conflicts) || !value.conflicts.every(conflict => typeof conflict === 'string') ||
      (value.scopeWarnings != null && (!Array.isArray(value.scopeWarnings) || !value.scopeWarnings.every(item => typeof item === 'string'))) ||
      (value.targetMode ?? 'fixed') !== scope.targetMode) return false;
  if (scope.targetMode !== 'fixed') {
    const validRule = scope.targetMode === 'all' ? !value.selector : typeof value.selector === 'string' && value.selector.trim() !== '';
    return validRule &&
      value.targets.every(target => target && typeof target.namespace === 'string' && typeof target.name === 'string' && typeof target.uid === 'string');
  }
  const key = (target: Target) => JSON.stringify([target.namespace, target.name, target.uid]);
  const expected = new Set(scope.targets.map(key));
  return value.targets.length === expected.size && value.targets.every(target => target && expected.delete(key(target)));
}

function responseMessage(status: number) {
  switch (status) {
    case 400: case 422: return '请检查订阅名称、目标集群或标签选择器后重新预览。';
    case 401: return '登录已失效，请重新登录。';
    case 403: return '当前请求被拒绝，请核对部署开关、访问域名和集群授权。';
    case 404: case 410: return '源 HelmChart 或目标集群已不存在，请刷新后重新选择。';
    case 409: return '源、当前匹配范围或订阅已发生变化，请重新预览并核对目标。';
    default: return `请求失败（HTTP ${status}），请重新核对订阅及目标集群。`;
  }
}

function subscriptionURL(namespace: string, name: string) {
  return pathForPage('propagation-status') + '?' + new URLSearchParams({ namespace, name });
}

function ClusterIdentity({ clusterId, uid }: { clusterId?: string; uid?: string }) {
  return <><small>集群 ID {clusterId || '未提供'}</small><small>对象 UID {uid || '未提供'}</small></>;
}

function DeploymentDialog({ chart, onClose }: { chart: HelmChart; onClose: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const pending = useRef<AbortController | null>(null);
  const [revision, setRevision] = useState(0);
  const [name, setName] = useState('');
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<string[]>([]);
  const [targetMode, setTargetMode] = useState<TargetMode>('fixed');
  const [selector, setSelector] = useState('');
  const [preview, setPreview] = useState<Preview>();
  const [created, setCreated] = useState<Target>();
  const [busy, setBusy] = useState<'preview' | 'create'>();
  const [error, setError] = useState('');
  const [uncertain, setUncertain] = useState(false);
  const clusters = useResource(fetchClusters, targetMode === 'fixed', revision);
  const candidates = clusters.data ?? [];
  const targets: Target[] = candidates.flatMap(cluster => cluster.namespace && cluster.uid && selected.includes(cluster.uid)
    ? [{ namespace: cluster.namespace, name: cluster.name, uid: cluster.uid }] : []);
  const search = query.trim().toLowerCase();
  const matches = candidates.filter(cluster => [cluster.name, cluster.namespace, cluster.uid, cluster.clusterId,
    ...Object.entries(cluster.labels ?? {}).map(([key, value]) => key + '=' + value)]
    .some(field => field?.toLowerCase().includes(search)));
  const matchingIds = matches.flatMap(cluster => cluster.namespace && cluster.uid ? [cluster.uid] : []);
  const validName = name.length <= 52 && dnsName.test(name);
  const readyScope = targetMode === 'fixed' ? targets.length > 0 && !clusters.loading && !clusters.error : targetMode === 'all' || selector.trim() !== '';
  const canPreview = validName && readyScope && !busy;
  const path = apiPath('/helmcharts/' + encodeURIComponent(chart.namespace) + '/' + encodeURIComponent(chart.name));

  useEffect(() => {
    const node = dialog.current!;
    if (typeof node.showModal === 'function') node.showModal();
    else node.setAttribute('open', '');
    return () => { pending.current?.abort(); node.close?.(); };
  }, []);

  async function submit(step: 'preview' | 'create') {
    if (pending.current || created || (step === 'preview' ? !canPreview : !preview || preview.conflicts.length > 0)) return;
    const controller = new AbortController();
    pending.current = controller;
    setBusy(step);
    setError('');
    setUncertain(false);
    try {
      const creating = step === 'create' && preview;
      let scope: Scope;
      if (targetMode === 'fixed') {
        scope = { targetMode, targets: creating ? creating.targets.map(({ namespace, name, uid }) => ({ namespace, name, uid })) : targets };
      } else if (targetMode === 'selector') {
        scope = { targetMode, selector: selector.trim() };
      } else {
        scope = { targetMode };
      }
      const payload = { name, ...scope, ...(creating ? { fingerprint: creating.fingerprint } : {}) };
      const response = await fetch(path + (creating ? '/deployments' : '/deployment-preview'), {
        method: 'POST', credentials: 'same-origin', headers: jsonHeaders, body: JSON.stringify(payload), signal: controller.signal,
      });
      if (!response.ok) throw new Error(responseMessage(response.status));
      const value = await response.json().catch(() => null);
      if (creating) {
        if (response.status !== 201 || value?.name !== creating.name || value?.namespace !== creating.namespace || typeof value?.uid !== 'string' || !value.uid) {
          throw new Error('未取得有效的创建回执。');
        }
        setCreated(value);
      } else {
        if (!validPreview(value, chart, name, scope)) throw new Error('预览返回的订阅或目标与本次选择不一致，请重新预览。');
        setPreview(value);
      }
    } catch (cause) {
      if (controller.signal.aborted) return;
      setPreview(undefined);
      setUncertain(step === 'create');
      setError(cause instanceof TypeError ? '连接中断，请核对当前状态后重新预览。' : cause instanceof Error ? cause.message : '请求未完成，请重新预览。');
    } finally {
      if (pending.current === controller) pending.current = null;
      if (!controller.signal.aborted) setBusy(undefined);
    }
  }

  return createPortal(<dialog ref={dialog} className="label-editor deployment-dialog" aria-labelledby="deploy-title"
    onCancel={event => { event.preventDefault(); if (busy !== 'create') onClose(); }}>
    <header><h2 id="deploy-title">部署到子集群</h2><button type="button" className="label-icon" aria-label="关闭窗口"
      disabled={busy === 'create'} onClick={onClose}><X size={17} /></button></header>
    <p className="label-target">HelmChart · {chart.namespace} / {chart.name}</p>
    {created ? <>
      <p role="status">已创建订阅 <strong>{created.namespace}/{created.name}</strong>。可在分发详情查看调度与发布回报。</p>
      <footer><button type="button" onClick={onClose}>关闭</button>
        <a className="inventory-link" href={subscriptionURL(created.namespace, created.name)}>查看分发详情</a></footer>
    </> : <>
      {preview ? <section aria-label="部署预览">
        <h3>部署预览</h3>
        <dl className="deploy-facts">
          <dt>新订阅</dt><dd>{preview.namespace}/{preview.name}</dd>
          <dt>分发策略</dt><dd>全量复制（Replication）</dd>
          <dt>目标范围</dt><dd>{modes[targetMode]}</dd>
          {targetMode === 'selector' && <><dt>标签选择器</dt><dd><code>{preview.selector}</code></dd></>}
          <dt>Chart 版本</dt><dd>{preview.chartVersion || '未指定'}</dd>
          <dt>源 Chart 阶段</dt><dd>{preview.chartPhase || '未上报'}</dd>
          <dt>应用目标命名空间</dt><dd>{preview.targetNamespace || '未指定'}</dd>
        </dl>
        <h3>{targetMode === 'fixed' ? '目标集群' : '当前匹配集群'} · {preview.targets.length}</h3>
        {targetMode !== 'fixed' && <p className="deploy-candidate-note">当前名单按受管集群标签统计。订阅创建后，可在分发详情查看实际绑定和发布回报。</p>}
        {preview.targets.length > 0 ? <ul className="deploy-targets">{preview.targets.map(target => <li key={target.uid}>
          <strong>{target.namespace}/{target.name}</strong><ClusterIdentity clusterId={target.clusterId} uid={target.uid} />
        </li>)}</ul> : <p>当前匹配 0 个集群，仍可创建订阅，等待后续集群匹配。</p>}
        {targetMode === 'fixed' && preview.selector && <details className="deploy-fixed-selector">
          <summary>查看固定目标的标签选择器</summary><code>{preview.selector}</code>
        </details>}
        {preview.subscriptionYAML && <details className="deploy-fixed-selector">
          <summary>查看订阅 YAML</summary><pre className="subscription-yaml">{preview.subscriptionYAML}</pre>
          <button type="button" className="text-button" onClick={() => downloadYAML(preview.subscriptionYAML!, preview.name + '.yaml')}>下载订阅 YAML</button>
        </details>}
        {!!preview.scopeWarnings?.length && <aside className="deploy-warnings" aria-label="目标范围提示"><strong>目标范围提示</strong>
          <ul>{preview.scopeWarnings.map((warning, i) => <li key={i}>{warning}</li>)}</ul></aside>}
        {preview.conflicts.length > 0 && <div className="label-error" role="alert"><strong>发现部署冲突</strong>
          <ul>{preview.conflicts.map((conflict, index) => <li key={index}>{conflict}</li>)}</ul></div>}
        <p className="label-impact">{targetMode === 'fixed' ? '确认后将在父集群创建订阅，按此次选中的集群 ID 分发此 HelmChart。后续新注册的集群不会自动加入。'
          : targetMode === 'selector' ? '订阅会持续按此选择器调度。后续注册或更新标签后符合条件的集群也会成为分发候选。'
          : '订阅将全部受管集群纳入分发候选，包括后续新注册的集群。'}</p>
      </section> : <fieldset disabled={!!busy}>
        <label className="deploy-input">新订阅名称<input autoFocus value={name} onChange={event => { setName(event.target.value); setError(''); setUncertain(false); }}
          placeholder="例如 web-canary" autoComplete="off" spellCheck={false} /></label>
        <small>订阅命名空间 · {chart.namespace}</small>
        {name && !validName && <p className="label-error" role="alert">名称须为小写 DNS 子域名，最长 52 个字符，以便控制器生成派生资源名称。</p>}
        <label className="deploy-input deploy-mode">目标范围
          <select value={targetMode} onChange={event => {
            setTargetMode(event.target.value as TargetMode); setSelected([]); setError(''); setUncertain(false);
          }}>{Object.entries(modes).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select>
        </label>
        {targetMode === 'selector' && <label className="deploy-input deploy-mode">Kubernetes 标签选择器
          <input value={selector} placeholder="例如 env=prod,region in (east,west)" autoComplete="off" spellCheck={false}
            onChange={event => { setSelector(event.target.value); setError(''); setUncertain(false); }} />
          <small>持续匹配受管集群的标签，预览可查看服务端确认的规则和当前匹配范围。</small>
        </label>}
        {targetMode === 'all' && <p>此订阅适用于当前及后续注册的全部受管集群。</p>}
        {targetMode === 'fixed' && <>
        <div className="deploy-heading"><h3>选择目标集群 · 已选 {targets.length} 个</h3>
          <button type="button" className="text-button" disabled={clusters.loading} onClick={() => setRevision(n => n + 1)}>刷新集群</button></div>
        <input className="deploy-search" type="search" aria-label="搜索目标集群" placeholder="搜索名称、命名空间、ID 或标签" value={query} onChange={event => setQuery(event.target.value)} />
        <div className="deploy-selection-tools"><button type="button" className="text-button"
          disabled={clusters.loading || !!clusters.error || !matchingIds.some(uid => !selected.includes(uid))}
          onClick={() => setSelected(previous => [...new Set([...previous, ...matchingIds])])}>选中当前筛选结果（{matchingIds.length}）</button>
          <button type="button" className="text-button" disabled={!selected.length} onClick={() => setSelected([])}>清除选中</button></div>
        {clusters.loading && <p role="status">正在读取受管集群…</p>}
        {clusters.error && <p className="label-error" role="alert">集群读取失败 · {clusters.error}</p>}
        {clusters.data && !matches.length && <p>{candidates.length ? '没有匹配搜索条件的集群。' : '当前没有可选的受管集群。'}</p>}
        {matches.length > 0 && <ul className="deploy-targets">{matches.map(cluster =>
          <li key={cluster.namespace + '/' + cluster.name}><label>
            <input type="checkbox" checked={!!cluster.uid && selected.includes(cluster.uid)} disabled={!cluster.uid || !cluster.namespace}
              onChange={event => { const uid = cluster.uid!; const checked = event.target.checked;
                setSelected(previous => checked ? [...previous, uid] : previous.filter(item => item !== uid)); }} />
            <span><strong>{cluster.namespace || '命名空间未知'}/{cluster.name}</strong><ClusterIdentity clusterId={cluster.clusterId} uid={cluster.uid} /></span>
            <span className="deploy-connectivity">{cluster.online ? '在线' : !cluster.status || cluster.status === 'Unknown' ? '连接未知' : '未就绪'}</span>
          </label></li>)}</ul>}</>}
      </fieldset>}
      {error && <div className="label-error" role="alert">{error}{uncertain && <> <a className="inventory-link" href={subscriptionURL(chart.namespace, name)}>核对同名订阅</a></>}</div>}
      <footer><button type="button" onClick={onClose} disabled={busy === 'create'}>取消</button>
        {preview ? <><button type="button" disabled={!!busy} onClick={() => { setPreview(undefined); setError(''); }}>返回修改</button>
          <button type="button" className="label-save" disabled={!!busy || preview.conflicts.length > 0} onClick={() => submit('create')}>
            {busy === 'create' ? '正在创建…' : targetMode === 'all' ? '确认订阅全部集群' : '确认创建订阅'}</button></>
        : <button type="button" className="label-save" disabled={!canPreview} onClick={() => submit('preview')}>
          {busy === 'preview' ? '正在预览…' : '预览部署'}</button>}
      </footer>
    </>}
  </dialog>, document.body);
}

export function ChartDeployment({ chart }: { chart: HelmChart }) {
  const [open, setOpen] = useState(false);
  return <>
    <button className="label-edit" type="button" onClick={() => setOpen(true)}><Send size={13} aria-hidden="true" />部署到子集群</button>
    {open && <DeploymentDialog chart={chart} onClose={() => setOpen(false)} />}
  </>;
}
