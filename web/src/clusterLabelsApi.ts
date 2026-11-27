import { apiPath } from './basePath';

export type LabelChange = { key: string; before: string | null; after: string | null };

export class LabelSaveError extends Error {
  constructor(public readonly status: number, message: string) { super(message); }
}

export async function saveClusterLabels(namespace: string, name: string, uid: string, changes: LabelChange[]) {
  const response = await fetch(apiPath(`/clusters/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/labels`), {
    method: 'PATCH', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ uid, changes }),
  });
  if (!response.ok) {
    const messages: Record<number, string> = {
      400: '标签格式无效，请检查键和值。', 401: '登录状态已失效。',
      403: '编辑被拒绝，请检查标签编辑开关、访问域名和集群写权限。',
      404: '此集群对象已不存在。', 409: '集群对象或正在编辑的标签已变更。请重新载入最新标签后再编辑。',
      422: '集群拒绝了这些标签，请检查标签格式和集群策略。',
    };
    throw new LabelSaveError(response.status, messages[response.status] || `保存失败（HTTP ${response.status}）`);
  }
  const result = await response.json() as { labels: Record<string, string> | null };
  return result.labels ?? {};
}
