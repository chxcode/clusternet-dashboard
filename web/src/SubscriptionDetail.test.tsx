import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { SubscriptionDetail } from './api';
import { SubscriptionDetailView } from './SubscriptionDetail';

const detail: SubscriptionDetail = {
  summary: { name: 'sample', namespace: 'apps', status: 'Progressing', desiredReleases: 2,
    completedReleases: 0, observedReleases: 1, completionKnown: false, bindingClusterCount: 2, bindingClusters: [] },
  feeds: [{ apiVersion: 'apps.clusternet.io/v1alpha1', kind: 'HelmChart', name: 'sample', namespace: 'apps' }],
  targetClusters: [
    { namespace: 'clusters', name: 'child-a', online: true, status: 'Online', observed: true, available: true },
    { namespace: 'clusters', name: 'child-b', online: false, status: 'Offline', observed: false },
  ],
};

describe('SubscriptionDetailView', () => {
  it('renders resource identifiers and keeps connection and controller feedback separate', () => {
    render(<SubscriptionDetailView detail={detail} />);
    expect(screen.getByText('HelmChart / sample')).toBeInTheDocument();
    expect(within(screen.getByRole('row', { name: /child-a/ })).getByText('已回报可用')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'child-a' })).toHaveAttribute('href', expect.stringMatching(/clusters\?q=child-a$/));
    expect(within(screen.getByRole('row', { name: /child-b/ })).getByText('未回报')).toBeInTheDocument();
  });

  it('distinguishes an empty filter from a subscription with no targets', () => {
    const { rerender } = render(<SubscriptionDetailView detail={detail} filter="online-not-observed" />);
    expect(screen.getByText('没有与当前筛选条件匹配的目标集群。')).toBeInTheDocument();
    expect(screen.getByText('匹配 0 / 共 2 个')).toBeInTheDocument();
    rerender(<SubscriptionDetailView detail={{ ...detail, targetClusters: [] }} />);
    expect(screen.getByText('此订阅尚无绑定的目标集群。')).toBeInTheDocument();
    rerender(<SubscriptionDetailView detail={{ ...detail, targetClusters: [], summary: { ...detail.summary, bindingClusters: null } }} />);
    expect(screen.getByText('控制器尚未提供可读取的绑定名单。')).toBeInTheDocument();
    expect(screen.queryByText('匹配 0 / 共 0 个')).toBeNull();
  });

  it('expands the selected target and distinguishes missing reports, unknown availability and explicit zero replicas', () => {
    const named = (name: string) => ({ kind: 'Deployment', name, apiVersion: 'apps/v1', namespace: 'apps' });
    render(<SubscriptionDetailView detail={{ ...detail,
      feeds: [named('one'), named('two'), named('three')],
      targetClusters: [{ ...detail.targetClusters[0], available: undefined, feedReports: [
        { ...named('one'), available: false, replicaStatus: { readyReplicas: 0 } },
        { ...named('two') },
      ] }],
    }} />);
    const toggle = screen.getByRole('button', { name: /child-a 的 Feed 回报/ });
    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    const reports = within(screen.getByRole('list', { name: 'child-a 的 Feed 回报' }));
    const [one, two, three] = reports.getAllByRole('listitem').map(el => within(el));
    expect(one.getByText('已回报 · 不可用')).toBeInTheDocument();
    expect(one.getByText('就绪副本')).toBeInTheDocument();
    expect(one.getByText('0')).toBeInTheDocument();
    expect(two.getByText('已回报 · 可用性未上报')).toBeInTheDocument();
    expect(three.getByText('未回报')).toBeInTheDocument();
    fireEvent.click(toggle);
    expect(screen.queryByRole('list', { name: 'child-a 的 Feed 回报' })).not.toBeInTheDocument();
  });

  it('paginates a large membership and resets the page on filter changes', () => {
    const many = { ...detail, targetClusters: Array.from({ length: 25 }, (_, n) => ({ ...detail.targetClusters[0], name: 'zone-' + n })) };
    const { rerender } = render(<SubscriptionDetailView detail={many} />);
    expect(screen.queryByText('zone-20')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '下一页' }));
    expect(screen.getByText('zone-20')).toBeInTheDocument();
    rerender(<SubscriptionDetailView detail={many} filter="observed" />);
    expect(screen.getByText('zone-0')).toBeInTheDocument();
    expect(screen.queryByText('zone-20')).not.toBeInTheDocument();
  });

  it('keeps unassignable reports out of the missing filter and withholds their status', () => {
    const ambiguous = { ...detail, targetClusters: [{ ...detail.targetClusters[1], unattributedFeeds: detail.feeds }] };
    const { rerender } = render(<SubscriptionDetailView detail={ambiguous} />);
    fireEvent.click(screen.getByRole('button', { name: /归属待确认/ }));
    expect(screen.getByText(/集群标识不足以匹配此目标/)).toBeInTheDocument();
    expect(screen.queryByText('未回报')).not.toBeInTheDocument();
    rerender(<SubscriptionDetailView detail={ambiguous} filter="not-observed" />);
    expect(screen.getByText('匹配 0 / 共 1 个')).toBeInTheDocument();
  });
});
