import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { AppResources } from './AppResources';
import type { Subscription } from './api';

const subscription: Subscription = {
  name: 'demo-sub',
  namespace: 'fleet',
  status: 'Progressing',
  desiredReleases: 3,
  completedReleases: 1,
  observedReleases: 2,
  completionKnown: true,
  bindingClusterCount: 3,
  bindingClusters: [],
};

describe('AppResources', () => {
  it('links to the namespaced subscription and shows observed and completed counts separately', () => {
    render(<AppResources subscriptions={[subscription]} manifests={[{
      name: 'demo-manifest', namespace: 'fleet', templateKind: 'Deployment', templateName: 'nginx',
    }]} />);

    expect(screen.getByRole('link', { name: 'demo-sub' })).toHaveAttribute('href', expect.stringContaining('/propagation?namespace=fleet&name=demo-sub'));
    expect(screen.getByText('2 / 3')).toBeInTheDocument();
    expect(screen.getByText('1 / 3')).toBeInTheDocument();
    expect(screen.getByText('Deployment / nginx')).toBeInTheDocument();
  });

  it('keeps subscription data visible when manifests cannot be read', () => {
    render(<AppResources subscriptions={[{ ...subscription, completionKnown: false }]} manifests={undefined} manifestError="403" />);
    expect(screen.getByText('完成数未知')).toBeInTheDocument();
    expect(screen.getByText('2 / 3')).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('403');
    expect(screen.queryByText('正在加载清单…')).not.toBeInTheDocument();
  });

  it('gives the two lists independent loading and empty states', () => {
    render(<AppResources subscriptions={undefined} manifests={[]} />);
    expect(screen.getByRole('status')).toHaveTextContent('正在加载订阅');
    expect(screen.getByText('当前没有 Manifest 清单。')).toBeInTheDocument();
  });

  it('shows static weights next to each subscriber and keeps actual bindings apart from reports', () => {
    render(<AppResources subscriptions={[{ ...subscription, distribution: {
      strategy: 'Dividing', dividingType: 'Static', targetScope: 'selector', selectors: ['region=east', 'region=west'],
      subscriberWeights: [0, 2], subgroupConfigured: true, subgroupMinClusters: [null, 1],
    } }]} manifests={[]} />);
    const row = within(screen.getByRole('row', { name: /demo-sub/ }));
    expect(row.getByText('副本分摊')).toBeVisible();
    expect(row.getByText('静态 · 按权重')).toBeVisible();
    expect(row.getByText('按子组调度')).toBeVisible();
    expect(row.getAllByRole('listitem').map(el => el.textContent)).toEqual([
      'region=east权重 0', 'region=west子组 minClusters = 1权重 2',
    ]);
    expect(row.getByText('3 个集群')).toBeVisible();
    expect(row.getByText('2 / 3')).toBeVisible();
    expect(row.getByText('1 / 3')).toBeVisible();
  });

  it('keeps a missing binding list distinct from an explicitly empty list', () => {
    const base = { ...subscription, bindingClusterCount: 0 };
    const { rerender } = render(<AppResources subscriptions={[{ ...base, bindingClusters: null }]} manifests={[]} />);
    expect(screen.getByText('未上报')).toBeVisible();
    expect(screen.queryByText('0 个集群')).toBeNull();
    rerender(<AppResources subscriptions={[{ ...base, bindingClusters: [] }]} manifests={[]} />);
    expect(screen.getByText('0 个集群')).toBeVisible();
  });
});
