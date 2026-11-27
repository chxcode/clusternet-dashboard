import { act, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { DistributionCatalog } from './ResourceCatalog';

const chart = { namespace: 'apps', name: 'web', version: '1.2.0', targetNamespace: 'shop' };
const result = { loading: false, data: [chart] };
const east = { namespace: 'east', name: 'edge', uid: 'mcls-east' };
const west = { namespace: 'west', name: 'edge', uid: 'mcls-west' };
const candidates = { clusters: [
  { ...east, clusterId: 'reg-east', labels: { area: 'east' }, online: false, status: 'Offline' },
  { ...west, clusterId: 'list-registration-west', labels: { area: 'west' }, online: true, status: 'Online' }, { name: 'no-identity', status: 'Unknown' },
] };
const preview = { namespace: 'apps', name: 'web-qa', chartName: 'web', chartVersion: '1.2.0', targetNamespace: 'shop', targets: [
  { ...west, clusterId: 'preview-registration-west' },
], fingerprint: 'first', conflicts: [] };
const success = { namespace: 'apps', name: 'web-qa', uid: 'new-sub' };
const json = (value: unknown, status = 200) => Response.json(value, { status });
const button = (name: string) => screen.getByRole('button', { name });

afterEach(() => { vi.unstubAllGlobals(); delete window.__CLUSTERNET_DASHBOARD_BASE_PATH__; });

async function selectWest() {
  fireEvent.click(button('部署到子集群'));
  fireEvent.change(screen.getByRole('textbox', { name: '新订阅名称' }), { target: { value: 'web-qa' } });
  fireEvent.click(await screen.findByRole('checkbox', { name: /west\/edge/ }));
  fireEvent.click(button('预览部署'));
}

it('fetches clusters only in the enabled dialog, starts empty and cancels without a mutation', async () => {
  const fetch = vi.fn().mockResolvedValueOnce(json(candidates));
  vi.stubGlobal('fetch', fetch);
  const { rerender } = render(<DistributionCatalog view="helmcharts" result={result} />);
  expect(screen.queryByRole('button', { name: '部署到子集群' })).toBeNull();
  rerender(<DistributionCatalog view="helmcharts" deploymentEnabled result={result} />);
  expect(fetch).not.toHaveBeenCalled();
  fireEvent.click(button('部署到子集群'));
  const modal = within(screen.getByRole('dialog'));
  expect(await modal.findByRole('checkbox', { name: /east\/edge.*未就绪/ })).not.toBeChecked();
  expect(modal.getByRole('checkbox', { name: /west\/edge.*在线/ })).toBeEnabled();
  expect(modal.getByText('选择目标集群 · 已选 0 个')).toBeVisible();
  expect(modal.getByRole('checkbox', { name: /no-identity.*连接未知/ })).toBeDisabled();
  expect(button('预览部署')).toBeDisabled();
  const input = modal.getByRole('textbox', { name: '新订阅名称' });
  for (const invalid of ['WEB', 'bad..name', 'ends-', 'x'.repeat(53)]) {
    fireEvent.change(input, { target: { value: invalid } });
    expect(modal.getByRole('alert')).toHaveTextContent('最长 52 个字符');
  }
  fireEvent.change(input, { target: { value: 'x'.repeat(52) } });
  fireEvent.click(button('选中当前筛选结果（2）'));
  expect(button('预览部署')).toBeEnabled();
  fireEvent.click(button('清除选中'));
  expect(button('预览部署')).toBeDisabled();
  fireEvent.click(button('取消'));
  expect(screen.queryByRole('dialog')).toBeNull();
  expect(fetch).toHaveBeenCalledOnce();
});

it('previews the selected identity after a label search and creates once using the reviewed fingerprint', async () => {
  window.__CLUSTERNET_DASHBOARD_BASE_PATH__ = '/workspace/fleet';
  let complete!: (response: Response) => void;
  const fixedSelector = 'clusters.clusternet.io/cluster-id in (preview-registration-west)';
  const fetch = vi.fn().mockResolvedValueOnce(json(candidates)).mockResolvedValueOnce(json({ ...preview, chartPhase: 'Found', targetMode: 'fixed', selector: fixedSelector }))
    .mockImplementationOnce(() => new Promise<Response>(resolve => { complete = resolve; }));
  vi.stubGlobal('fetch', fetch);
  render(<DistributionCatalog view="helmcharts" result={result} deploymentEnabled />);
  fireEvent.click(button('部署到子集群'));
  const modal = within(screen.getByRole('dialog'));
  await modal.findByText('west/edge');
  expect(modal.getByText('集群 ID list-registration-west')).toBeVisible();
  expect(modal.getByText('对象 UID mcls-west')).toBeVisible();
  fireEvent.change(modal.getByRole('searchbox'), { target: { value: 'area=west' } });
  expect(modal.queryByText('east/edge')).toBeNull();
  fireEvent.click(button('选中当前筛选结果（1）'));
  fireEvent.change(modal.getByRole('searchbox'), { target: { value: 'REG-EAST' } });
  expect(modal.getByRole('checkbox', { name: /east\/edge/ })).not.toBeChecked();
  expect(modal.getByText('选择目标集群 · 已选 1 个')).toBeVisible();
  fireEvent.change(modal.getByRole('textbox', { name: '新订阅名称' }), { target: { value: 'web-qa' } });
  fireEvent.click(button('预览部署'));
  expect(await modal.findByRole('region', { name: '部署预览' })).toHaveTextContent('apps/web-qa');
  expect(modal.getByText('全量复制（Replication）')).toBeVisible();
  expect(modal.getByText('Found')).toBeVisible();
  expect(modal.getByText('集群 ID preview-registration-west')).toBeVisible();
  expect(modal.getByText('对象 UID mcls-west')).toBeVisible();
  expect(modal.queryByText('集群 ID list-registration-west')).toBeNull();
  fireEvent.click(modal.getByText('查看固定目标的标签选择器'));
  expect(modal.getByText(fixedSelector)).toBeVisible();
  expect(modal.getByText(/新注册的集群不会自动加入/)).toBeVisible();
  expect(fetch.mock.calls[1][0]).toBe('/workspace/fleet/api/helmcharts/apps/web/deployment-preview');
  expect(JSON.parse(fetch.mock.calls[1][1].body)).toEqual({ name: 'web-qa', targetMode: 'fixed', targets: [west] });
  expect(fetch.mock.calls[1][1].method).toBe('POST');
  expect(fetch).toHaveBeenCalledTimes(2);
  fireEvent.click(button('确认创建订阅'));
  expect(button('正在创建…')).toBeDisabled();
  fireEvent.click(button('正在创建…'));
  expect(button('取消')).toBeDisabled();
  expect(fetch.mock.calls[2][0]).toBe('/workspace/fleet/api/helmcharts/apps/web/deployments');
  expect(JSON.parse(fetch.mock.calls[2][1].body)).toEqual({ name: 'web-qa', targetMode: 'fixed', targets: [west], fingerprint: 'first' });
  await act(async () => complete(json(success, 201)));
  expect(modal.getByRole('status')).toHaveTextContent('已创建订阅 apps/web-qa');
  expect(modal.getByRole('link', {name: '查看分发详情'})).toHaveAttribute('href', '/workspace/fleet/propagation?namespace=apps&name=web-qa');
  expect(fetch).toHaveBeenCalledTimes(3);
});

it('requires a new preview and confirmation on conflict, and blocks a preview that returns a different object', async () => {
  const fetch = vi.fn().mockResolvedValueOnce(json(candidates))
    .mockResolvedValueOnce(json({ ...preview, conflicts: ['目标已被另一订阅使用'] }))
    .mockResolvedValueOnce(json({ ...preview, targets: [{ ...west, uid: 'replacement' }] }))
    .mockResolvedValueOnce(json(preview)).mockResolvedValueOnce(new Response(null, { status: 409 }))
    .mockResolvedValueOnce(json({ ...preview, fingerprint: 'changed' })).mockResolvedValueOnce(json(success, 201));
  vi.stubGlobal('fetch', fetch);
  render(<DistributionCatalog view="helmcharts" deploymentEnabled result={result} />);
  await selectWest();
  expect(await screen.findByRole('alert')).toHaveTextContent('目标已被另一订阅使用');
  expect(screen.getByText('未上报', { selector: 'dd' })).toBeVisible();
  expect(button('确认创建订阅')).toBeDisabled();
  fireEvent.click(button('返回修改'));
  fireEvent.click(button('预览部署'));
  expect(await screen.findByRole('alert')).toHaveTextContent('与本次选择不一致');
  expect(screen.queryByRole('button', { name: '确认创建订阅' })).toBeNull();
  fireEvent.click(button('预览部署'));
  fireEvent.click(await screen.findByRole('button', { name: '确认创建订阅' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('重新预览');
  expect(screen.queryByRole('button', { name: '确认创建订阅' })).toBeNull();
  fireEvent.click(button('预览部署'));
  fireEvent.click(await screen.findByRole('button', { name: '确认创建订阅' }));
  expect(await screen.findByRole('link', { name: '查看分发详情' })).toBeVisible();
  expect(JSON.parse(fetch.mock.calls[fetch.mock.calls.length - 1][1].body).fingerprint).toBe('changed');
});

it('retains stage-specific errors and leaves a link to check an uncertain write', async () => {
  const fetch = vi.fn().mockResolvedValueOnce(new Response(null, { status: 403 }))
    .mockResolvedValueOnce(json(candidates)).mockResolvedValueOnce(json(preview)).mockRejectedValueOnce(new TypeError('offline'));
  vi.stubGlobal('fetch', fetch);
  render(<DistributionCatalog view="helmcharts" deploymentEnabled result={result} />);
  fireEvent.click(button('部署到子集群'));
  expect(await screen.findByRole('alert')).toHaveTextContent('读取权限');
  expect(button('预览部署')).toBeDisabled();
  fireEvent.click(button('刷新集群'));
  fireEvent.change(screen.getByRole('textbox', { name: '新订阅名称' }), { target: { value: 'web-qa' } });
  fireEvent.click(await screen.findByRole('checkbox', { name: /west\/edge/ }));
  fireEvent.click(button('预览部署'));
  fireEvent.click(await screen.findByRole('button', { name: '确认创建订阅' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('连接中断');
  expect(screen.getByRole('link', { name: '核对同名订阅' })).toBeVisible();
  expect(screen.queryByText(/已创建订阅/)).toBeNull();
});

it('creates a continuing label rule with zero matches and requests another review when the matching set changes', async () => {
  const selector = 'region in (west,east),env=qa';
  const canonical = 'env=qa,region in (east,west)';
  const first = { ...preview, targetMode: 'selector', selector: canonical, scopeWarnings: ['集群标签可能变化。'] };
  const second = { ...first, fingerprint: 'no-longer-matches', targets: [] };
  const fetch = vi.fn().mockResolvedValueOnce(json(candidates)).mockResolvedValueOnce(json(first))
    .mockResolvedValueOnce(new Response(null, { status: 409 })).mockResolvedValueOnce(json(second)).mockResolvedValueOnce(json(success, 201));
  vi.stubGlobal('fetch', fetch);
  render(<DistributionCatalog view="helmcharts" deploymentEnabled result={result} />);
  fireEvent.click(button('部署到子集群'));
  fireEvent.change(screen.getByRole('textbox', { name: '新订阅名称' }), { target: { value: 'web-qa' } });
  fireEvent.click(await screen.findByRole('checkbox', { name: /west\/edge/ }));
  fireEvent.change(screen.getByRole('combobox', { name: '目标范围' }), { target: { value: 'selector' } });
  expect(screen.queryByRole('checkbox')).toBeNull();
  const input = screen.getByRole('textbox', { name: /Kubernetes 标签选择器/ });
  fireEvent.change(input, { target: { value: '  ' } });
  expect(button('预览部署')).toBeDisabled();
  fireEvent.change(input, { target: { value: selector } });
  fireEvent.click(button('预览部署'));
  expect(await screen.findByText(canonical)).toBeVisible();
  expect(screen.getByText('当前匹配集群 · 1')).toBeVisible();
  expect(screen.getByText(/可在分发详情查看实际绑定和发布回报/)).toBeVisible();
  expect(screen.getByText('集群标签可能变化。')).toBeVisible();
  const requested = { name: 'web-qa', targetMode: 'selector', selector };
  expect(JSON.parse(fetch.mock.calls[1][1].body)).toEqual(requested);
  fireEvent.click(button('确认创建订阅'));
  expect(await screen.findByRole('alert')).toHaveTextContent('重新预览');
  expect(screen.queryByRole('button', { name: '确认创建订阅' })).toBeNull();
  expect(JSON.parse(fetch.mock.calls[2][1].body)).toEqual({ ...requested, fingerprint: 'first' });
  fireEvent.click(button('预览部署'));
  expect(await screen.findByText(/当前匹配 0 个集群/)).toBeVisible();
  expect(screen.getByText(/后续注册或更新标签/)).toBeVisible();
  expect(fetch).toHaveBeenCalledTimes(4);
  fireEvent.click(button('确认创建订阅'));
  expect(await screen.findByRole('link', { name: '查看分发详情' })).toBeVisible();
  expect(JSON.parse(fetch.mock.calls[4][1].body)).toEqual({ ...requested, fingerprint: 'no-longer-matches' });
});

it('only accepts an explicit all-clusters plan and sends no target array or selector on confirmation', async () => {
  const fetch = vi.fn().mockResolvedValueOnce(json(candidates)).mockResolvedValueOnce(json(preview))
    .mockResolvedValueOnce(json({ ...preview, targetMode: 'all', targets: [west, east] })).mockResolvedValueOnce(json(success, 201));
  vi.stubGlobal('fetch', fetch);
  render(<DistributionCatalog view="helmcharts" deploymentEnabled result={result} />);
  fireEvent.click(button('部署到子集群'));
  fireEvent.change(screen.getByRole('textbox', { name: '新订阅名称' }), { target: { value: 'web-qa' } });
  const mode = screen.getByRole('combobox', { name: '目标范围' });
  expect(mode).toHaveValue('fixed');
  await screen.findByText('west/edge');
  expect(button('预览部署')).toBeDisabled();
  fireEvent.change(mode, { target: { value: 'all' } });
  expect(screen.getByText(/当前及后续注册的全部/)).toBeVisible();
  fireEvent.click(button('预览部署'));
  expect(await screen.findByRole('alert')).toHaveTextContent('与本次选择不一致');
  expect(screen.queryByRole('button', { name: '确认订阅全部集群' })).toBeNull();
  fireEvent.click(button('预览部署'));
  const confirm = await screen.findByRole('button', { name: '确认订阅全部集群' });
  expect(screen.getByText('当前匹配集群 · 2')).toBeVisible();
  expect(screen.getByText(/包括后续新注册的集群/)).toBeVisible();
  const requested = { name: 'web-qa', targetMode: 'all' };
  expect(JSON.parse(fetch.mock.calls[2][1].body)).toEqual(requested);
  fireEvent.click(confirm);
  expect(await screen.findByRole('link', { name: '查看分发详情' })).toBeVisible();
  expect(JSON.parse(fetch.mock.calls[3][1].body)).toEqual({ ...requested, fingerprint: 'first' });
});
