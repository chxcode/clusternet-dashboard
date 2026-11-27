import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Overview } from './Overview';
import type { Cluster, HelmRelease, Subscription } from './api';
import type { DataResult } from './useResource';

const ready = <T,>(data: T): DataResult<T> => ({ data, loading: false });
const clusters = ready<Cluster[]>([
  { name: 'online-a', status: 'Online', online: true }, { name: 'offline-b', status: 'Offline', online: false },
]);
const releases = ready<HelmRelease[]>([
  { name: 'chart-a', phase: 'deployed' }, { name: 'waiting', phase: 'pending-upgrade' },
  { name: 'failed-a', phase: 'failed', namespace: 'team-a', clusterName: 'edge-a' }, { name: 'unknown' },
]);
const subscriptions = ready<Subscription[]>([
  { name: 'sub-a', namespace: 'team-a', status: 'Progressing', desiredReleases: 3, completedReleases: 0,
    observedReleases: 2, completionKnown: false, bindingClusterCount: 3, bindingClusters: [] },
]);
const materials = ready({ manifests: 1, globalizations: 2, localizations: 1, feedInventories: 0 });
const sources = { clusters, releases, subscriptions, materials };

const registeredCluster = (namespace: string, name: string): Cluster => ({ namespace, name, online: true, status: 'Online' });
const subscription = (name: string, bindings: string[]): Subscription => ({
  ...subscriptions.data![0], name, bindingClusters: bindings, bindingClusterCount: bindings.length,
});

describe('Overview', () => {
  it('links real filtered counts and preview objects to their workspaces', () => {
    render(<Overview {...sources} />);
    const metrics = within(screen.getByRole('region', { name: '概览指标' }));
    expect(metrics.getByRole('link', { name: /在线子集群.*1.*共 2/ })).toHaveAttribute('href', expect.stringMatching(/clusters\?status=online$/));
    expect(metrics.getByRole('link', { name: /失败发布.*1/ })).toHaveAttribute('href', expect.stringMatching(/releases\?phase=failed$/));
    expect(metrics.getByRole('link', { name: /等待中的发布.*1/ })).toHaveAttribute('href', expect.stringMatching(/releases\?phase=pending$/));
    const failure = within(screen.getByRole('region', { name: '失败发布' }));
    const href = failure.getByRole('link', { name: /failed-a.*控制器未回报原因.*edge-a/ }).getAttribute('href')!;
    const params = new URL(href, 'http://localhost').searchParams;
    expect([params.get('namespace'), params.get('name')]).toEqual(['team-a', 'failed-a']);
    const table = within(screen.getByRole('row', { name: /sub-a/ }));
    expect(table.getByText('未知')).toBeInTheDocument();
    expect(table.getByText('2')).toBeInTheDocument();
    expect(screen.getByRole('region', { name: '分发物料' })).toHaveTextContent('共 4 项');
    expect(screen.getByRole('region', { name: '发布状态分布' })).toHaveTextContent('未知1');
  });

  it('keeps unknown and failed sources distinct from a loaded empty result', () => {
    render(<Overview {...sources} clusters={{ loading: true }} releases={{ loading: false, error: '503' }} subscriptions={ready([])} />);
    const metrics = within(screen.getByRole('region', { name: '概览指标' }));
    expect(metrics.getByRole('link', { name: /在线子集群/ })).toHaveTextContent('—加载中');
    expect(metrics.getByRole('link', { name: /失败发布/ })).toHaveTextContent('—读取失败');
    expect(screen.queryByText('当前没有失败的发布。')).not.toBeInTheDocument();
    expect(screen.getByText('当前没有订阅。')).toBeInTheDocument();
  });

  it('keeps the last good count visible with refresh failures', () => {
    render(<Overview {...sources} releases={{ ...releases, error: '503' }} />);
    const metrics = within(screen.getByRole('region', { name: '概览指标' }));
    expect(metrics.getByRole('link', { name: /失败发布/ })).toHaveTextContent('1刷新失败');
    expect(screen.getByRole('link', { name: /failed-a/ })).toBeInTheDocument();
  });

  it('counts distinct currently registered targets across all subscriptions including those outside the preview', () => {
    const registered = ready([registeredCluster('east','shared'), registeredCluster('west','shared'), registeredCluster('lab','unique')]);
    const entries = Array.from({length: 5}, (_, i) => subscription('release-' + i, ['east/shared', 'east/shared']));
    entries.push(subscription('sixth', ['unique', 'lab/unique', 'former/shared']));
    render(<Overview {...sources} clusters={registered} subscriptions={ready(entries)} />);
    expect(screen.getByText('已注册 3 个子集群，其中 2 个至少绑定一个订阅。')).toBeVisible();
    expect(screen.queryByText('sixth')).toBeNull();
  });

  it('shows only confirmed coverage when a legacy bare name matches multiple clusters or binding identities are missing', () => {
    const registered = ready([registeredCluster('one','same'), registeredCluster('two','same')]);
    const { rerender } = render(<Overview {...sources} clusters={registered}
      subscriptions={ready([subscription('ambiguous', ['same'])])} />);
    expect(screen.getByText(/已注册 2 个子集群，部分绑定目标无法确定，订阅覆盖数未知/)).toBeVisible();
    rerender(<Overview {...sources} clusters={registered}
      subscriptions={ready([{ ...subscription('partial', ['two/same']), bindingClusterCount: 2 }])} />);
    expect(screen.getByText(/已确认其中 1 个至少绑定一个订阅，部分绑定目标无法确定/)).toBeVisible();
  });

  it('distinguishes a successful empty subscription list from unavailable source data', () => {
    const { rerender } = render(<Overview {...sources} subscriptions={ready([])} />);
    expect(screen.getByText('已注册 2 个子集群，其中 0 个至少绑定一个订阅。')).toBeVisible();
    rerender(<Overview {...sources} subscriptions={ready([])} clusters={{ loading: true }} />);
    expect(screen.getByText('正在读取集群与订阅的绑定关系…')).toBeVisible();
    rerender(<Overview {...sources} subscriptions={ready([])} clusters={{ ...clusters, error:'403' }} />);
    expect(screen.getByText('集群或订阅读取失败，暂时无法统计绑定覆盖数。')).toBeVisible();
    expect(screen.queryByText(/其中 0 个至少绑定/)).toBeNull();
  });
});
