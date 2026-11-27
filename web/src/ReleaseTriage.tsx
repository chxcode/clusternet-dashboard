import { useState } from 'react';
import type { HelmRelease } from './api';
import { apiPath } from './basePath';
import { releasePhaseGroup } from './releasePhase';

export const triageLabels = { open: '待跟进', investigating: '处理中', resolved: '已处理' } as const;
type TriageState = keyof typeof triageLabels;

const actions: { state: TriageState; label: string }[] = [
  { state: 'investigating', label: '标记处理中' },
  { state: 'resolved', label: '标记已处理' },
  { state: 'open', label: '重新打开' },
];
const failures: Record<number, string> = {
  401: '登录状态已失效。', 403: '保存被拒绝，请检查跟进功能开关、访问域名和写入权限。',
  404: '原发布对象已不存在。', 409: '发布或跟进状态已变更，请重新载入后确认。',
};

type Result = { source: HelmRelease; state?: TriageState; error?: string; conflict?: boolean };

export function ReleaseTriage({ release, enabled, onRefresh }: { release: HelmRelease; enabled: boolean; onRefresh?: () => void }) {
  const [saving, setSaving] = useState(false);
  const [result, setResult] = useState<Result>();
  const current = result?.source === release ? result : undefined;
  const state = current?.state ?? release.triage ?? 'open';
  const failed = releasePhaseGroup(release.phase) === 'failed';

  async function save(next: TriageState) {
    if (!enabled || !failed || !release.namespace || !release.uid || !release.triageIncident || saving || current?.conflict) return;
    setSaving(true);
    try {
      const path = '/helmreleases/' + encodeURIComponent(release.namespace) + '/' + encodeURIComponent(release.name) + '/triage';
      const response = await fetch(apiPath(path), {
        method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ uid: release.uid, incident: release.triageIncident, before: state, state: next }),
      });
      if (!response.ok) {
        setResult({ source: release, state, error: failures[response.status] ?? `保存失败（HTTP ${response.status}）。`,
          conflict: response.status === 409 || response.status === 404 });
        return;
      }
      setResult({ source: release, state: next });
      onRefresh?.();
    } catch {
      setResult({ source: release, state, error: '未收到保存结果，请重新载入核对当前状态。' });
    } finally {
      setSaving(false);
    }
  }

  if ((!failed || !enabled) && !release.triage) return null;
  return <section className="release-triage" aria-label="故障跟进">
    <h4>故障跟进</h4>
    <small>由工作台单独保存此发布的人工跟进记录。</small>
    <p role="status">{triageLabels[state]}{saving ? ' · 保存中…' : current?.state && !current.error ? ' · 已保存' : ''}</p>
    {enabled && failed && release.namespace && release.uid && release.triageIncident && <div className="release-triage-actions">
      {actions.filter(action => action.state !== state).map(action =>
        <button key={action.state} type="button" disabled={saving || current?.conflict} onClick={() => save(action.state)}>{action.label}</button>)}
    </div>}
    {current?.error && <div role="alert">{current.error}{onRefresh && <button className="text-button" type="button" onClick={onRefresh}>重新载入</button>}</div>}
  </section>;
}
