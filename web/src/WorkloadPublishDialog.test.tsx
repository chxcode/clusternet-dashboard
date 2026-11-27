import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { AppResources } from './AppResources';
import type { Subscription } from './api';
import { templates } from './workloadDraft';
import type { Origin } from './nativeWorkload';
import { WorkloadPublishDialog } from './WorkloadPublishDialog';
import * as yamlEditor from './YamlEditor';

const button = (name: string) => screen.getByRole('button', { name });
const input = (name: string, value: string) => fireEvent.change(screen.getByLabelText(name), { target: { value } });
const result = (value: object, status = 200) => ({ ok: status >= 200 && status < 300, status, json: async () => value });
const source = { apiVersion: 'v1', kind: 'ConfigMap', namespace: 'apps', name: 'web-config' } as const;
const resource = { ...source, action: 'create' };
const id = { namespace: 'apps', name: 'app-release' };
const sub = { ...id, feedKinds: ['ConfigMap'], distribution: undefined, bindingClusters: null } as unknown as Subscription;
const base = { ...id, targetMode: 'selector', selector: 'env=prod', targets: [], resources: [resource],
  subscriptionAction: 'create', subscriptionYAML: 'kind: Subscription', fingerprint: 'preview-1', conflicts: [], scopeWarnings: [] };
const doc = { ...source, uid: 'cm-1', baseline: 'cm-original', yaml: templates.ConfigMap, subscriptions: [] };
const bundle = { ...id, yaml: templates.ConfigMap, targetMode: 'selector', selector: 'env=prod', baseline: 'sub-1', sourceBaselines: { 'ConfigMap/web-config': 'cm-original' } };
const manifest = { name: 'generated-123', namespace: 'reserved', templateKind: 'ConfigMap', templateName: 'web-config', templateNamespace: 'apps', templateAPIVersion: 'v1' };
const openNew = () => render(<WorkloadPublishDialog origin={{ type: 'new' }} onClose={vi.fn()}/>);
const fillNew = () => { input('命名空间', id.namespace); input('订阅名称', id.name); input('工作负载 YAML', templates.ConfigMap); input('标签选择器', 'env=prod'); };
function request(fetch: ReturnType<typeof vi.fn>, index: number) {
  const [url, init] = fetch.mock.calls[index];
  expect(init.method).toBe('POST'); expect(init.headers['Content-Type']).toBe('application/json');
  return { url, body: JSON.parse(init.body) };
}
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); delete window.__CLUSTERNET_DASHBOARD_BASE_PATH__; });

it('exposes no online buttons when disabled and offers only explicit native identities without fetching catalog source', () => {
  const fetch = vi.fn(); vi.stubGlobal('fetch', fetch);
  const manifests = [manifest, { ...manifest, name: 'secret', templateKind: 'Secret' }, { ...manifest, name: 'wrong-api', templateAPIVersion: 'other/v1' },
    { ...manifest, name: 'no-ns', templateNamespace: undefined }];
  const { rerender } = render(<AppResources subscriptions={[sub]} manifests={manifests}/>);
  expect(button('编辑本地 YAML')).toBeVisible(); expect(screen.queryByRole('button', { name: '新建原生发布' })).toBeNull();
  rerender(<AppResources subscriptions={[sub, { ...sub, name: 'mixed', feedKinds: ['ConfigMap', 'Secret'] }]} manifests={manifests} nativePublishing/>);
  expect(screen.getAllByRole('button', { name: '编辑工作负载' })).toHaveLength(1);
  expect(screen.getAllByRole('button', { name: '编辑原生发布' })).toHaveLength(1);
  expect(fetch).not.toHaveBeenCalled();
});

it('previews zero dynamic matches, requires separate confirmation, sends the fingerprint and links to observed distribution', async () => {
  window.__CLUSTERNET_DASHBOARD_BASE_PATH__ = '/platform';
  const fetch = vi.fn().mockResolvedValueOnce(result(base)).mockResolvedValueOnce(result({ status: 'saved', resources: [resource], subscriptionSaved: true }));
  vi.stubGlobal('fetch', fetch); const onPublished = vi.fn();
  render(<AppResources subscriptions={[]} manifests={[]} nativePublishing onPublished={onPublished}/>);
  fireEvent.click(button('新建原生发布')); expect(button('预览发布范围')).toBeDisabled(); fillNew();
  fireEvent.click(button('预览发布范围')); expect(await screen.findByText('当前匹配集群 · 0')).toBeVisible();
  expect(request(fetch, 0)).toEqual({ url: '/platform/api/native-workloads/apps/app-release/preview', body: { targetMode: 'selector', selector: 'env=prod', yaml: templates.ConfigMap } });
  expect(fetch).toHaveBeenCalledTimes(1); expect(onPublished).not.toHaveBeenCalled();
  const confirm = button('确认发布'); fireEvent.click(confirm); fireEvent.click(confirm);
  expect(await screen.findByText('源文件与订阅已保存')).toBeVisible();
  expect(request(fetch, 1)).toEqual({ url: '/platform/api/native-workloads/apps/app-release/apply', body: { targetMode: 'selector', selector: 'env=prod', yaml: templates.ConfigMap, fingerprint: 'preview-1' } });
  expect(fetch).toHaveBeenCalledTimes(2); expect(onPublished).toHaveBeenCalledOnce();
  expect(screen.getByRole('link', { name: '查看分发详情' })).toHaveAttribute('href', '/platform/propagation?namespace=apps&name=app-release');
});

it('loads from the original type and identity and opens a referencing subscription with its entire editable source set', async () => {
  const service = { apiVersion: 'v1', kind: 'Service', namespace: 'apps', name: 'web', action: 'unchanged' };
  const yaml = templates.ConfigMap + '---\n' + templates.Service;
  const sourceBaselines = { ...bundle.sourceBaselines, 'Service/web': 'svc-orig' };
  const fetch = vi.fn().mockResolvedValueOnce(result({ ...doc, subscriptions: ['apps/app-release', 'team/another'] }))
    .mockResolvedValueOnce(result({ ...bundle, yaml, sourceBaselines })).mockResolvedValueOnce(result({ ...base, resources: [resource, service], scopeWarnings: ['文件有其他订阅引用'], conflicts: ['共享源，请先核对其他订阅'] }));
  vi.stubGlobal('fetch', fetch);
  render(<AppResources subscriptions={[]} manifests={[manifest]} nativePublishing/>); fireEvent.click(button('编辑工作负载'));
  expect(await screen.findByRole('heading', { name: '已有订阅直接引用此源 · 2' })).toBeVisible();
  expect(screen.getByRole('textbox', { name: '工作负载 YAML' })).toHaveAttribute('readonly');
  expect(request(fetch, 0)).toEqual({ url: '/api/native-sources/apps/web-config/ConfigMap', body: {} });
  expect(screen.queryByRole('button', { name: '确认发布' })).toBeNull();
  fireEvent.click(button('打开发布 apps/app-release'));
  await waitFor(() => expect(screen.getByLabelText('工作负载 YAML')).toHaveValue(yaml));
  expect(screen.getByLabelText('工作负载 YAML')).not.toHaveAttribute('readonly');
  expect(request(fetch, 1)).toEqual({ url: '/api/native-workloads/apps/app-release/source', body: {} });
  const preview = button('预览发布范围');
  await waitFor(() => expect(preview).toBeEnabled());
  fireEvent.click(preview);
  expect(await screen.findByText('存在发布冲突')).toBeVisible();
  expect(request(fetch, 2).body).toMatchObject({ baseline: 'sub-1', sourceBaselines });
  expect(button('确认发布')).toBeDisabled(); expect(fetch).toHaveBeenCalledTimes(3);
});

it('prevents a Secret returned for a listed ConfigMap from rendering and refuses unsupported submitted documents', async () => {
  const secret = 'apiVersion: v1\nkind: Secret\nmetadata: {name: web-config}\ndata: {value: hidden-test-data}\n';
  const fetch = vi.fn().mockResolvedValueOnce(result({ ...doc, yaml: secret })); vi.stubGlobal('fetch', fetch);
  const origin: Origin = { type: 'source', id: source };
  const view = render(<WorkloadPublishDialog origin={origin} onClose={vi.fn()}/>);
  expect(await screen.findByRole('alert')).toBeVisible(); expect(screen.queryByText(/hidden-test-data/)).toBeNull();
  expect(screen.queryByRole('textbox', { name: '工作负载 YAML' })).toBeNull();
  view.unmount(); openNew(); fillNew(); input('工作负载 YAML', secret); fireEvent.click(button('预览发布范围'));
  expect(await screen.findByRole('alert')).toHaveTextContent('Deployment、Service 或 ConfigMap'); expect(fetch).toHaveBeenCalledTimes(1);
});

it('preserves source baselines, shows acknowledged partial writes and requires a fresh review before another apply', async () => {
  const all = { ...base, targetMode: 'all', selector: '', resources: [{ ...resource, action: 'update' }] };
  const fetch = vi.fn().mockResolvedValueOnce(result(doc)).mockResolvedValueOnce(result(all))
    .mockResolvedValueOnce(result({ status: 'partial', resources: all.resources, subscriptionSaved: false, message: '订阅存储失败' }, 207))
    .mockResolvedValueOnce(result({ ...all, fingerprint: 'preview-2', resources: [{ ...resource, action: 'unchanged' }] }));
  vi.stubGlobal('fetch', fetch); const onPublished = vi.fn();
  render(<WorkloadPublishDialog origin={{ type: 'source', id: source }} onClose={vi.fn()} onPublished={onPublished}/>);
  await waitFor(() => expect(screen.getByLabelText('工作负载 YAML')).toHaveValue(templates.ConfigMap));
  input('订阅名称', id.name); input('目标范围', 'all');
  fireEvent.click(button('预览发布范围')); expect(await screen.findByText('持续覆盖所有已注册及后续注册的集群。')).toBeVisible();
  expect(request(fetch, 1).body).toEqual({ yaml: doc.yaml, sourceBaselines: bundle.sourceBaselines, targetMode: 'all' });
  fireEvent.click(button('确认发布')); expect(await screen.findByRole('heading', { name: '部分内容已保存' })).toBeVisible();
  const receipt = within(screen.getByRole('region', { name: '发布回执' }));
  expect(receipt.getByText('ConfigMap · apps/web-config')).toBeVisible(); expect(receipt.getByText(/尚未确认保存订阅/)).toBeVisible();
  expect(onPublished).toHaveBeenCalledOnce();
  fireEvent.click(button('保留草稿，返回核对')); expect(screen.getByLabelText('工作负载 YAML')).toHaveValue(templates.ConfigMap);
  expect(screen.queryByRole('button', { name: '确认发布' })).toBeNull();
  fireEvent.click(button('预览发布范围')); expect(await screen.findByText('保持现状')).toBeVisible();
  expect(request(fetch, 3).body).not.toHaveProperty('fingerprint');
});

it('invalidates stale previews on conflict and treats a network failure during apply as unknown', async () => {
  const fetch = vi.fn().mockResolvedValueOnce(result(base)).mockResolvedValueOnce(result({ error: '目标已变化' }, 409))
    .mockResolvedValueOnce(result({ ...base, fingerprint: 'fresh' })).mockRejectedValueOnce(new TypeError('offline'));
  vi.stubGlobal('fetch', fetch); openNew(); fillNew();
  fireEvent.click(button('预览发布范围')); fireEvent.click(await screen.findByRole('button', { name: '确认发布' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('目标已变化');
  expect(screen.queryByRole('button', { name: '确认发布' })).toBeNull();
  fireEvent.click(button('预览发布范围')); fireEvent.click(await screen.findByRole('button', { name: '确认发布' }));
  expect(await screen.findByRole('heading', { name: '提交结果尚未确认' })).toBeVisible();
  expect(request(fetch, 3).body.fingerprint).toBe('fresh');
  expect(fetch).toHaveBeenCalledTimes(4);
});

it('blocks a broken multi-document draft locally and requires a fresh preview after correction', async () => {
  const service = { apiVersion: 'v1', kind: 'Service', namespace: 'apps', name: 'web', action: 'create' };
  const corrected = templates.ConfigMap + '---\n' + templates.Service;
  const fetch = vi.fn().mockResolvedValueOnce(result(base))
    .mockResolvedValueOnce(result({ ...base, resources: [resource, service], fingerprint: 'next-preview' }))
    .mockResolvedValueOnce(result({ status: 'saved', resources: [resource, service], subscriptionSaved: true }));
  vi.stubGlobal('fetch', fetch); openNew(); fillNew();
  fireEvent.click(button('预览发布范围')); fireEvent.click(await screen.findByRole('button', { name: '返回编辑' }));
  input('工作负载 YAML', templates.ConfigMap + '---\nmetadata: [\n');
  expect(screen.getByLabelText('工作负载 YAML')).toHaveAttribute('aria-invalid', 'true');
  expect(button('预览发布范围')).toBeDisabled(); fireEvent.click(button('预览发布范围'));
  expect(screen.queryByRole('button', { name: '确认发布' })).toBeNull();
  expect(fetch).toHaveBeenCalledTimes(1);
  input('工作负载 YAML', corrected);
  expect(button('预览发布范围')).toBeEnabled(); fireEvent.click(button('预览发布范围'));
  fireEvent.click(await screen.findByRole('button', { name: '确认发布' }));
  await screen.findByRole('heading', { name: '源文件与订阅已保存' });
  expect(request(fetch, 2).body).toMatchObject({ yaml: corrected, fingerprint: 'next-preview' });
});

it('waits for analysis of the current text before enabling remote preview', () => {
  const fetch = vi.fn(); vi.stubGlobal('fetch', fetch);
  const edited = templates.ConfigMap + '# edited\n';
  const useRealAnalysis = yamlEditor.useYamlAnalysis;
  vi.spyOn(yamlEditor, 'useYamlAnalysis').mockImplementation((value, singleDocument) =>
    useRealAnalysis(value === edited ? templates.ConfigMap : value, singleDocument));
  openNew(); fillNew();
  expect(button('预览发布范围')).toBeEnabled();
  input('工作负载 YAML', edited);
  expect(screen.getByRole('textbox', { name: '工作负载 YAML' })).toHaveValue(edited);
  expect(button('预览发布范围')).toBeDisabled(); fireEvent.click(button('预览发布范围'));
  expect(fetch).not.toHaveBeenCalled();
});

it('blocks online preview of valid oversized YAML and recovers after the draft is shortened', async () => {
  const fetch = vi.fn(); vi.stubGlobal('fetch', fetch); openNew(); fillNew();
  input('工作负载 YAML', templates.ConfigMap + '  payload: ' + 'x'.repeat(256 * 1024));
  expect(button('预览发布范围')).toBeDisabled();
  await screen.findByText(/在线发布上限为 256 KiB/);
  expect(screen.getByLabelText('工作负载 YAML')).not.toHaveAttribute('aria-invalid', 'true');
  expect(button('预览发布范围')).toBeDisabled(); fireEvent.click(button('预览发布范围'));
  expect(fetch).not.toHaveBeenCalled();
  input('工作负载 YAML', templates.ConfigMap);
  await waitFor(() => expect(button('预览发布范围')).toBeEnabled());
});
it('moves an existing in-memory local draft into a new publication only on explicit request', () => {
  const fetch = vi.fn(); vi.stubGlobal('fetch', fetch);
  render(<AppResources subscriptions={[]} manifests={[]} nativePublishing/>);
  fireEvent.click(button('编辑本地 YAML')); input('YAML 源文件', templates.Service);
  fireEvent.click(button('用此草稿准备发布'));
  expect(screen.getByRole('heading', { name: '新建原生发布' })).toBeVisible();
  expect(screen.getByLabelText('工作负载 YAML')).toHaveValue(templates.Service);
  expect(fetch).not.toHaveBeenCalled();
});

it('loads cluster labels on expansion, counts and filters a bounded page, and appends equality to a handwritten selector', async () => {
  const lots = Object.fromEntries(Array.from({ length: 26 }, (_, i) => ['label-' + i, 'x']));
  const fetch = vi.fn().mockResolvedValueOnce(result({ clusters: [{ name: 'one', labels: { ...lots, rollout: 'pilot', location: 'east' } },
    { name: 'two', labels: { rollout: 'pilot', location: 'west' } }, { name: 'three' }] }));
  vi.stubGlobal('fetch', fetch); openNew(); fillNew();
  input('标签选择器', 'tier in (api,worker),!retired');
  expect(fetch).not.toHaveBeenCalled(); fireEvent.click(button('从集群标签选择'));
  await screen.findByRole('button', { name: 'rollout=pilot 2 个集群' });
  expect(fetch).toHaveBeenCalledOnce(); expect(fetch.mock.calls[0][0]).toBe('/api/clusters');
  const section = within(screen.getByRole('region', { name: '集群标签选项' }));
  expect(section.getAllByRole('listitem')).toHaveLength(20); expect(section.getByText('1 / 2')).toBeVisible();
  fireEvent.click(button('下一页')); expect(section.getByText('2 / 2')).toBeVisible();
  input('筛选标签', 'PILOT'); expect(section.getAllByRole('listitem')).toHaveLength(1);
  fireEvent.click(button('rollout=pilot 2 个集群')); expect(screen.getByLabelText('标签选择器')).toHaveValue('tier in (api,worker),!retired,rollout=pilot');
  expect(button('rollout=pilot 2 个集群 已添加')).toBeDisabled();
  input('筛选标签', 'east'); fireEvent.click(button('location=east 1 个集群'));
  expect(screen.getByLabelText('标签选择器')).toHaveValue('tier in (api,worker),!retired,rollout=pilot,location=east');
  input('标签选择器', 'rollout=pilot'); expect(button('location=east 1 个集群')).toBeEnabled();
  input('筛选标签', 'absent'); expect(section.getByText('没有符合筛选的标签。')).toBeVisible();
  expect(fetch).toHaveBeenCalledOnce();
});

it('still previews a manually entered zero-match rule when the label catalog is unavailable', async () => {
  const fetch = vi.fn().mockResolvedValueOnce(result({}, 403)).mockResolvedValueOnce(result({ ...base, selector: 'env=future' }));
  vi.stubGlobal('fetch', fetch); openNew(); fillNew(); fireEvent.click(button('从集群标签选择'));
  expect(await screen.findByRole('alert')).toHaveTextContent('仍可手工输入');
  input('标签选择器', 'env=future'); fireEvent.click(button('预览发布范围'));
  expect(await screen.findByText('当前匹配集群 · 0')).toBeVisible();
  expect(request(fetch, 1).body.selector).toBe('env=future'); expect(fetch).toHaveBeenCalledTimes(2);
});
