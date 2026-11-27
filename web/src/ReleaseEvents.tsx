import { useCallback, useState } from 'react';
import { fetchReleaseEvents } from './api';
import { useResource } from './useResource';

export function ReleaseEvents({ namespace, uid, revision = 0 }: { namespace: string; uid: string; revision?: number }) {
  const [open, setOpen] = useState(false);
  const [retry, setRetry] = useState(0);
  const load = useCallback((signal: AbortSignal) => fetchReleaseEvents(namespace, uid, signal), [namespace, uid]);
  const result = useResource(load, open, retry, String(revision));
  return <div className="release-events">
    <button className="text-button" type="button" aria-expanded={open} onClick={() => setOpen(!open)}>
      {open ? '收起父集群事件' : '查看父集群事件'}
    </button>
    {open && <div aria-live="polite">
      {result.loading && <p>正在获取事件…</p>}
      {result.error && <p role="alert">事件读取失败 · {result.error} <button type="button" className="text-button" onClick={() => setRetry(n => n + 1)}>重试</button></p>}
      {result.data?.length === 0 && <p>父集群当前没有保留此发布对象的事件。</p>}
      {result.data && result.data.length > 0 && <ul>{[...result.data].sort((a,b) => (b.lastSeen || '').localeCompare(a.lastSeen || '')).map(event =>
        <li key={event.name}><strong>{event.reason || event.type || '事件'}</strong> {event.message}
          {event.lastSeen && <time dateTime={event.lastSeen}>{new Date(event.lastSeen).toLocaleString('zh-CN')}</time>}</li>)}</ul>}
    </div>}
  </div>;
}
