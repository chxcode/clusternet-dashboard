import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { DistributionCatalog } from './ResourceCatalog';
import { ReleaseRecovery } from './ReleaseRecovery';

const chart = { namespace: 'feeds', name: 'api', chart: 'api-chart', version: '1.0.0' };
const result = { loading: false, data: [chart] };
const impact = {
  uid: 'chart-uid', namespace: chart.namespace, name: chart.name, currentVersion: '1.0.1', fingerprint: 'first',
  subscriptions: [
    { namespace: 'apps', name: 'stable', boundTargets: 2 }, { namespace: 'apps', name: 'canary', boundTargets: 1 },
  ],
  boundTargets: 2, targets: ['east', 'west'],
};
function json(body: unknown) { return new Response(JSON.stringify(body)); }
afterEach(() => vi.unstubAllGlobals());

describe('HelmChart version changes', () => {
  it('previews subscriptions and targets in a read-only workspace without offering a write', async () => {
    const fetch = vi.fn().mockResolvedValueOnce(json(impact));
    vi.stubGlobal('fetch', fetch);
    render(<DistributionCatalog view="helmcharts" result={result} />);
    expect(fetch).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', {name:'查看影响范围'}));
    const modal = within(screen.getByRole('dialog', {name:'HelmChart 影响范围'}));
    expect(await modal.findByText('apps / stable')).toBeVisible();
    fireEvent.click(modal.getByText('查看绑定目标标识'));
    expect(modal.getByText('west')).toBeVisible();
    expect(modal.queryByRole('textbox')).toBeNull();
    expect(modal.queryByRole('button', {name:'确认更改版本'})).toBeNull();
    fireEvent.click(modal.getByRole('button', {name:'关闭'}));
    expect(fetch).toHaveBeenCalledOnce();
    expect(fetch.mock.calls[0][0]).toMatch(/\/version-impact$/);
    expect(fetch.mock.calls[0][1].method ?? 'GET').toBe('GET');
  });

  it('only loads the live impact when an operator opens the editor in an enabled workspace', async () => {
    const fetch = vi.fn().mockResolvedValueOnce(json(impact)).mockResolvedValueOnce(json({version:'1.1.0-rc.2'}));
    vi.stubGlobal('fetch', fetch);
    const refresh = vi.fn();
    const { rerender } = render(<DistributionCatalog view="helmcharts" result={result} />);
    expect(screen.queryByRole('button', {name:'更改 Chart 版本'})).toBeNull();
    rerender(<DistributionCatalog view="helmcharts" result={result} versionEditing onVersionSaved={refresh} />);
    expect(fetch).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', {name:'更改 Chart 版本'}));
    const modal = within(screen.getByRole('dialog'));
    expect(await modal.findByText('1.0.1')).toBeVisible();
    expect(modal.getByText('2 个直接引用的订阅 · 2 个当前绑定目标（去重）')).toBeVisible();
    expect(modal.getByText('apps / stable')).toBeVisible();
    expect(modal.getByText('apps / canary')).toBeVisible();
    fireEvent.click(modal.getByText('查看绑定目标标识'));
    expect(modal.getByText('east')).toBeVisible();
    expect(modal.getByText('west')).toBeVisible();
    const input = modal.getByRole('textbox', {name:'新版本'});
    const save = modal.getByRole('button', {name:'确认更改版本'});
    for (const version of ['1.x', 'v1.0.1', '2.0.0-01']) {
      fireEvent.change(input, {target:{value:version}});
      expect(save).toBeDisabled();
    }
    fireEvent.change(input, {target:{value:'1.1.0-rc.2'}});
    expect(fetch).toHaveBeenCalledTimes(1);
    fireEvent.click(save);
    expect(await screen.findByText('源版本已保存为 1.1.0-rc.2')).toBeVisible();
    expect(fetch.mock.calls[0][0]).toMatch(/\/api\/helmcharts\/feeds\/api\/version-impact$/);
    const [url, options] = fetch.mock.calls[1];
    expect(url).toMatch(/\/api\/helmcharts\/feeds\/api\/version$/);
    expect(options.method).toBe('POST');
    expect(JSON.parse(options.body)).toEqual({uid:impact.uid, currentVersion:'1.0.1', newVersion:'1.1.0-rc.2', fingerprint:'first'});
    expect(refresh).toHaveBeenCalledOnce();
  });

  it('requires a fresh impact snapshot and a second explicit confirmation after a conflict', async () => {
    const fetch = vi.fn().mockResolvedValueOnce(json(impact)).mockResolvedValueOnce(new Response(null, {status:409}))
      .mockResolvedValueOnce(json({...impact, currentVersion:'', fingerprint:'second', subscriptions:[], boundTargets:0, targets:[]}))
      .mockResolvedValueOnce(json({version:'2.0.0'}));
    vi.stubGlobal('fetch', fetch);
    render(<DistributionCatalog view="helmcharts" result={result} versionEditing />);
    fireEvent.click(screen.getByRole('button', {name:'更改 Chart 版本'}));
    fireEvent.change(await screen.findByRole('textbox', {name:'新版本'}), {target:{value:'2.0.0'}});
    fireEvent.click(screen.getByRole('button', {name:'确认更改版本'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('影响范围已变更');
    expect(screen.queryByRole('button', {name:'确认更改版本'})).toBeNull();
    fireEvent.click(screen.getByRole('button', {name:'重新预览'}));
    expect(await screen.findByText('0 个直接引用的订阅 · 0 个当前绑定目标（去重）')).toBeVisible();
    expect(fetch).toHaveBeenCalledTimes(3);
    fireEvent.click(screen.getByRole('button', {name:'确认更改版本'}));
    expect(await screen.findByText('源版本已保存为 2.0.0')).toBeVisible();
    expect(JSON.parse(fetch.mock.calls[3][1].body)).toMatchObject({fingerprint:'second', currentVersion:''});
  });

  it('blocks writes when subscription impact cannot be read', async () => {
    const fetch = vi.fn().mockResolvedValueOnce(new Response(null, {status:403}));
    vi.stubGlobal('fetch', fetch);
    render(<DistributionCatalog view="helmcharts" result={result} versionEditing />);
    fireEvent.click(screen.getByRole('button', {name:'更改 Chart 版本'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('权限不足');
    expect(screen.queryByRole('button', {name:'确认更改版本'})).toBeNull();
    fireEvent.click(screen.getByRole('button', {name:'取消'}));
    expect(fetch).toHaveBeenCalledOnce();
  });

  it('links through the known subscription or catalog without guessing the source object', () => {
    const release = {name:'child-generated', chart:'package-name', subscriptionNamespace:'ops', subscriptionName:'stable'};
    const { rerender } = render(<ReleaseRecovery release={release}/>);
    expect(screen.getByRole('link', {name:'查看该订阅的 Feed'}).getAttribute('href')).toMatch(/propagation\?namespace=ops&name=stable$/);
    expect(screen.getByRole('link', {name:'打开 HelmChart 列表'}).getAttribute('href')).toMatch(/subscriptions\?view=helmcharts$/);
    rerender(<ReleaseRecovery release={{name:'orphan'}}/>);
    expect(screen.getByRole('link', {name:'查找订阅'}).getAttribute('href')).toMatch(/subscriptions$/);
  });
});
