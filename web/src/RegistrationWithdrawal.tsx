import { useEffect, useId, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Trash2, X } from 'lucide-react';
import { CopyButton } from './CopyButton';
import { apiPath } from './basePath';
import type { Registration } from './api';
import type { DataResult } from './useResource';
import './clusterLabels.css';
import './registrationWithdrawal.css';

type Receipt = { name: string; uid: string; clusterID?: string; previous?: Registration[] };
export function RegistrationReceipt({ receipt, result, refresh }: {
  receipt: Receipt; result: DataResult<Registration[]>; refresh?: () => void;
}) {
  const current = result.data?.find(r => r.name === receipt.name);
  let text = '已提交撤销请求，等待父集群列表确认。';
  if (result.data && result.data !== receipt.previous && !result.error && !result.loading) {
    if (!current) text = '旧请求已消失，尚未观察到同名的新请求。';
    else if (current.uid === receipt.uid) text = '父集群仍有原 UID 的请求，等待删除完成。';
    else if (current.uid && current.clusterID && current.clusterID === receipt.clusterID) {
      text = '已观察到同一集群 ID 的同名新请求。新 UID ' + current.uid + '，审批结果 ' + (current.result || '未上报') + '。';
    } else text = '同名记录的身份已变化，尚未核实为原集群的新请求。';
  } else if (result.error) text = '已提交撤销请求，最新查询失败，请刷新核对。';
  return <div className={result.error ? 'inventory-warning' : 'registration-receipt'} role="status"><strong>{receipt.name}</strong> · 旧 UID <code>{receipt.uid}</code> · {text}
    {refresh && <button type="button" className="text-button" onClick={refresh} disabled={result.loading}>刷新核对</button>}
  </div>;
}

export function RegistrationWithdrawal({ row, onRefresh, onAccepted }: {
  row: Registration; onRefresh?: () => void; onAccepted: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [typed, setTyped] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const submitting = useRef(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const title = useRef<HTMLHeadingElement>(null);
  const titleId = useId();
  useEffect(() => {
    const node = dialog.current, returnTo = trigger.current;
    if (!open || !node) return;
    if (typeof node.showModal === 'function') node.showModal(); else node.setAttribute('open', '');
    title.current?.focus({ preventScroll: true });
    return () => {
      node.close?.();
      if (returnTo?.isConnected) returnTo.focus({ preventScroll: true });
    };
  }, [open]);
  function close() {
    if (submitting.current) return;
    setOpen(false); setTyped(''); setError('');
  }
  if (!row.uid || !row.resourceVersion || !row.deletionMode || row.deleting) return null;
  const label = row.deletionMode === 'failed' ? '删除失败请求' : row.deletionMode === 'denied' ? '删除已拒绝请求' : '删除未批准请求';

  async function submit() {
    if (submitting.current || error || typed !== row.name) return;
    submitting.current = true; setPending(true); setError('');
    try {
      const response = await fetch(apiPath('/registrations/' + encodeURIComponent(row.name)), {
        method: 'DELETE', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ uid: row.uid, resourceVersion: row.resourceVersion, confirmName: typed }),
      });
      if (response.status === 202 && (await response.json()).status === 'deletionRequested') {
        setOpen(false); onAccepted(); return;
      }
      const msg = response.status === 409 ? '身份、状态或集群分配情况已变化，操作被拒绝。'
        : response.status === 403 ? '当前工作区或 Kubernetes 权限禁止撤销。' : '未能确认撤销结果。';
      setError(msg + ' 请刷新核对注册请求。');
    } catch { setError('连接中断，撤销结果未知，请刷新核对注册请求。'); }
    finally { setPending(false); submitting.current = false; }
  }
  return <>
    <button ref={trigger} className="registration-delete-trigger" type="button" aria-haspopup="dialog" onClick={() => setOpen(true)}>
      <Trash2 size={13} aria-hidden="true" />删除请求
    </button>
    {open && createPortal(<dialog ref={dialog} className="label-editor registration-dialog" aria-labelledby={titleId}
      onCancel={event => { event.preventDefault(); close(); }}>
      <header><h2 id={titleId} ref={title} tabIndex={-1}>{label}</h2>
        <button type="button" className="label-icon" disabled={pending} aria-label="关闭注册确认窗口" onClick={close}><X size={17} /></button></header>
      <dl className="registration-identity">
        <dt>请求名称</dt><dd className="registration-name"><code>{row.name}</code><CopyButton value={row.name} label="复制完整请求名称" /></dd>
        <dt>旧对象 UID</dt><dd><code>{row.uid}</code></dd>
        <dt>申请命名空间</dt><dd>{row.requestedNamespace || '自动分配'}</dd>
        <dt>审批结果</dt><dd>{row.result || '尚未批准 · 结果未上报'}</dd>
      </dl>
      <p>父集群将删除这个注册请求，并保留已有的命名空间和子集群资源。</p>
      <p className="label-impact">{row.deletionMode === 'pendingNamespace' ? '先核对并修正子端 agent 申请的命名空间。' : '先核对并修正子端 agent 的注册配置。'}
        正在等待审批的 agent 会继续查询旧请求。相同集群 ID 的 agent 重新进入引导注册并拥有权限时，才可能提交同名新请求。</p>
      <form aria-label={label} onSubmit={e => { e.preventDefault(); void submit(); }}>
        <label>输入完整请求名称以确认<input autoComplete="off" spellCheck={false} value={typed} disabled={pending}
          onChange={e => setTyped(e.target.value)} /></label>
        {error && <p className="label-error" role="alert">{error} {onRefresh && <button type="button" className="text-button" onClick={onRefresh}>刷新列表</button>}</p>}
        <footer><button type="button" disabled={pending} onClick={close}>取消</button>
          <button type="submit" className="registration-submit" disabled={pending || typed !== row.name || !!error}>{pending ? '正在提交…' : '确认删除'}</button></footer>
      </form>
    </dialog>, document.body)}
  </>;
}

export type RegistrationDeletionReceipt = Receipt;
