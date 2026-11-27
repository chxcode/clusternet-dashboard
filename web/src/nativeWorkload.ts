import { parseAllDocuments } from 'yaml';
import type { Manifest } from './api';
import { apiPath } from './basePath';
import type { Kind } from './workloadDraft';

export type Identity = { namespace: string; name: string };
export type NativeSource = Identity & { kind: Kind; apiVersion: string };
export type Origin = { type: 'new'; yaml?: string } | { type: 'source'; id: NativeSource } | { type: 'subscription'; id: Identity };
export type Mode = 'selector' | 'all';
export type Baselines = Record<string, string>;
export type PlanResource = NativeSource & { action: 'create' | 'update' | 'unchanged' };
export type Plan = Identity & {
  targetMode: Mode; selector?: string; resources: PlanResource[]; targets: (Identity & { uid: string; clusterId?: string })[];
  subscriptionAction: PlanResource['action']; subscriptionYAML: string; scopeWarnings: string[]; conflicts: string[]; fingerprint: string;
};
export type Receipt = { status: 'saved' | 'partial' | 'uncertain'; resources: PlanResource[]; subscriptionSaved: boolean; message?: string };
export const actionText = { create: '新建', update: '更新', unchanged: '保持现状' } as const;
export const nativeVersion = { Deployment: 'apps/v1', Service: 'v1', ConfigMap: 'v1' } as const;
export const isKind = (value: unknown): value is Kind => typeof value === 'string' && Object.prototype.hasOwnProperty.call(nativeVersion, value);
const dnsLabel = /^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?$/;
const dnsName = /^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?(?:\.[a-z0-9](?:[-a-z0-9]*[a-z0-9])?)*$/;
export const validNamespace = (s: unknown): s is string => typeof s === 'string' && s.length <= 63 && dnsLabel.test(s);
const validName = (s: unknown): s is string => typeof s === 'string' && s.length <= 253 && dnsName.test(s);
export const validSubscriptionName = (s: string) => s.length <= 52 && validName(s);
export function sourceFromManifest(item: Manifest): NativeSource | undefined {
  if (isKind(item.templateKind) && item.templateAPIVersion === nativeVersion[item.templateKind] &&
      validNamespace(item.templateNamespace) && validName(item.templateName)) {
    return { kind: item.templateKind, apiVersion: item.templateAPIVersion, namespace: item.templateNamespace, name: item.templateName };
  }
}
export function sourceIdentities(raw: string, namespace: string): NativeSource[] {
  if (new Blob([raw]).size > 256 * 1024) throw new Error('源文件总量超过 256 KiB。');
  const docs = parseAllDocuments(raw, { version: '1.1', prettyErrors: false });
  const list: NativeSource[] = [];
  for (const doc of docs) {
    if (doc.errors.length || doc.warnings.length) throw new Error('YAML 无法解析，请检查文档格式。');
    const item = doc.toJS({ maxAliasCount: 100 });
    if (item === null) continue;
    const ns = item?.metadata?.namespace ?? namespace;
    const kind: unknown = item?.kind;
    if (!isKind(kind) || item.apiVersion !== nativeVersion[kind] || !validName(item?.metadata?.name) || ns !== namespace) {
      throw new Error('文件中的资源必须是同一命名空间的 Deployment、Service 或 ConfigMap。');
    }
    list.push({ namespace: ns, name: item.metadata.name, kind, apiVersion: item.apiVersion });
  }
  if (!list.length) throw new Error('文件中没有工作负载。');
  if (list.length > 16) throw new Error('一次发布最多包含 16 份资源。');
  return list;
}
export function subscriptionId(raw: string): Identity | undefined {
  if (typeof raw !== 'string') return;
  const parts = raw.split('/');
  if (parts.length === 2 && validNamespace(parts[0]) && validName(parts[1])) return { namespace: parts[0], name: parts[1] };
}
const encodeId = (id: Identity) => [id.namespace, id.name].map(encodeURIComponent).join('/');
export const workloadPath = (id: Identity) => '/native-workloads/' + encodeId(id);
export const sourcePath = (id: NativeSource) => '/native-sources/' + encodeId(id) + '/' + encodeURIComponent(id.kind);
export class NativeRequestError extends Error {
  constructor(readonly status: number, message: string) { super(message); }
}
export async function nativeRequest(path: string, body: object, ctrl: AbortController) {
  const response = await fetch(apiPath(path), { method: 'POST', credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), signal: ctrl.signal });
  const data = await response.json().catch(() => null);
  if (!response.ok) {
    const hint = response.status === 403 ? '访问被拒绝，请核对功能开关、访问域名和集群授权。'
      : response.status === 409 ? '源文件、引用、目标或同名订阅存在冲突，请重新读取并核对。'
      : response.status === 404 ? '找不到对应的工作负载或订阅。' : '请求未完成（HTTP ' + response.status + '）。';
    throw new NativeRequestError(response.status, hint + (typeof data?.error === 'string' ? ' ' + data.error : ''));
  }
  return { data, status: response.status };
}
const stringList = (field: unknown): field is string[] => Array.isArray(field) && field.every(s => typeof s === 'string');
const hasAction = (v: unknown): v is PlanResource['action'] => v === 'create' || v === 'update' || v === 'unchanged';
const hasResource = (r: PlanResource) => Boolean(r && isKind(r.kind) && r.apiVersion === nativeVersion[r.kind] && validNamespace(r.namespace) && validName(r.name) && hasAction(r.action));
export function validPlan(p: Plan, id: Identity, mode: Mode, sources: NativeSource[]) {
  if (!p || p.name !== id.name || p.namespace !== id.namespace || p.targetMode !== mode ||
    (mode === 'selector' ? typeof p.selector !== 'string' || !p.selector.trim() : !!p.selector) ||
    typeof p.fingerprint !== 'string' || !p.fingerprint || !hasAction(p.subscriptionAction) ||
    typeof p.subscriptionYAML !== 'string' || !stringList(p.conflicts) || !stringList(p.scopeWarnings) ||
    !Array.isArray(p.resources) || !Array.isArray(p.targets) || !p.targets.every(t => t && validName(t.name) && validNamespace(t.namespace) && typeof t.uid === 'string')) return false;
  const expected = new Set(sources.map(r => r.kind + '/' + r.name));
  return p.resources.length === expected.size && p.resources.every(r => hasResource(r) && r.namespace === id.namespace && expected.delete(r.kind + '/' + r.name));
}
export function validReceipt(result: Receipt, status: number, plan: Plan) {
  if (!result || typeof result.subscriptionSaved !== 'boolean' || !Array.isArray(result.resources)) return false;
  const expected = new Set(plan.resources.map(r => r.kind + '/' + r.name));
  if (!result.resources.every(r => hasResource(r) && r.namespace === plan.namespace && expected.delete(r.kind + '/' + r.name))) return false;
  return result.status === 'saved' ? status !== 207 && result.subscriptionSaved && expected.size === 0 :
    status === 207 && (result.status === 'partial' || result.status === 'uncertain');
}
