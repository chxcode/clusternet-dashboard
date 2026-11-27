import { useRef, useState } from 'react';
import { Trash2 } from 'lucide-react';
import type { HelmRelease } from './api';
import { apiPath } from './basePath';
import { releasePhaseGroup } from './releasePhase';
import './releaseDelete.css';

type Target = HelmRelease & Required<Pick<HelmRelease, 'namespace' | 'uid' | 'resourceVersion' | 'descriptionOwner'>>;

export function canDeleteRelease(release: HelmRelease): release is Target {
  return releasePhaseGroup(release.phase) === 'failed' && !release.deleting && release.controlledByDescription === true &&
    Boolean(release.name && release.namespace && release.uid && release.resourceVersion && release.descriptionOwner?.name && release.descriptionOwner.uid);
}

type RequestState = { stage: 'pending' | 'accepted' } | {
  stage: 'error'; message: string; resourceVersion: string; readAt?: number;
};
const failures: Record<number, string> = {
  401: '登录状态已失效，请重新登录。',
  403: '删除被拒绝，请检查功能开关、访问域名和 Kubernetes 权限。',
  404: '提交时找不到这个发布对象，请刷新列表。',
  409: '发布对象已变更，请刷新列表并重新核对。',
};

export function useReleaseDeletion(onDeleted?: () => void) {
  const [requests, setRequests] = useState<Record<string, RequestState>>({});
  const [receipts, setReceipts] = useState<Record<string, string>>({});
  const locked = useRef(new Set<string>());

  async function submit(release: Target, readAt?: number) {
    if (locked.current.has(release.uid)) return;
    locked.current.add(release.uid);
    const update = (result: RequestState) => setRequests(previous => ({ ...previous, [release.uid]: result }));
    update({ stage: 'pending' });
    let message = '未能核实删除请求的结果，请刷新查看对象状态。';
    let accepted = false;
    try {
      const path = '/helmreleases/' + encodeURIComponent(release.namespace) + '/' + encodeURIComponent(release.name);
      const response = await fetch(apiPath(path), {
        method: 'DELETE', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ uid: release.uid, resourceVersion: release.resourceVersion }),
      });
      accepted = response.status === 202 && (await response.json())?.status === 'deletionRequested';
      if (!response.ok) {
        message = failures[response.status] ?? `删除请求失败（HTTP ${response.status}）。请刷新查看对象状态。`;
      }
    } catch {
      // A connection error can occur after Kubernetes has accepted the request.
    }
    if (accepted) {
      update({ stage: 'accepted' });
      setReceipts(previous => ({ ...previous, [release.namespace + '/' + release.name]: release.uid }));
      onDeleted?.();
      return;
    }
    locked.current.delete(release.uid);
    update({ stage: 'error', resourceVersion: release.resourceVersion, message, readAt });
  }
  return { requests, receipts, submit };
}

export type ReleaseDeletion = ReturnType<typeof useReleaseDeletion>;

export function ReleaseDelete({ release, deletion, readAt, onRefresh }: {
  release: Target; deletion: ReleaseDeletion; readAt?: number; onRefresh?: () => void;
}) {
  const [confirming, setConfirming] = useState(false);
  const [typed, setTyped] = useState('');
  const target = release.namespace + '/' + release.name;
  const request = deletion.requests[release.uid];
  if (request?.stage === 'accepted') return null;
  const error = request?.stage === 'error' && request.resourceVersion === release.resourceVersion && request.readAt === readAt ? request : undefined;
  const pending = request?.stage === 'pending';

  return <section className="release-delete" aria-label="删除失败发布">
    <h4>删除失败的 HelmRelease</h4>
    {pending ? <p role="status">正在提交 {target} 的删除请求…</p> : <>
      {error && <div role="alert">{error.message} {onRefresh && <button type="button" className="text-button" onClick={onRefresh}>刷新发布详情</button>}</div>}
      {confirming ? <form onSubmit={event => {
        event.preventDefault();
        if (typed !== target || error) return;
        setConfirming(false);
        setTyped('');
        void deletion.submit(release, readAt);
      }}>
        <p>父集群将删除 <code>{target}</code>。控制器清理时可能卸载子集群上的 Helm 应用及其资源，造成服务中断。</p>
        <p>工作台保留终结器，由控制器完成清理。仅当上游仍要求分发此发布时，控制器才可能重新创建对象。</p>
        <label>输入完整的命名空间/名称以确认
          <input autoFocus value={typed} onChange={event => setTyped(event.target.value)} autoComplete="off" spellCheck={false} />
        </label>
        <div className="release-delete-actions">
          <button type="button" onClick={() => { setTyped(''); setConfirming(false); }}>取消</button>
          <button type="submit" className="release-delete-submit" disabled={typed !== target || !!error}>确认删除</button>
        </div>
      </form> : <button type="button" className="release-delete-start" disabled={!!error} onClick={() => setConfirming(true)}>
        <Trash2 size={14} aria-hidden="true" /> 删除发布对象…</button>}
    </>}
  </section>;
}
