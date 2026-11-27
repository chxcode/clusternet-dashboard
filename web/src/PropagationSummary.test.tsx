import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { PropagationSummary } from './PropagationSummary';
import type { SubscriptionDetail } from './api';

describe('PropagationSummary', () => {
  const detail: SubscriptionDetail = {
    summary: { name: 'demo', status: 'Progressing', desiredReleases: 5, completedReleases: 0, observedReleases: 0,
      completionKnown: false, bindingClusterCount: 3, bindingClusters: [], distribution: {
        strategy: 'Replication', selectors: [''], targetScope: 'all', subgroupConfigured: true, subgroupMinClusters: [0],
      } },
    coverage: { registeredClusters: 5, labelMatchedClusters: 5, unboundMatches: [{ namespace: 'east', name: 'remote' }] },
    targetClusters: [], feeds: [],
  };
  it('distinguishes unrestricted labels from subgroup scheduling, actual bindings and the exact unbound list', () => {
    render(<PropagationSummary detail={detail} />);
    const panel = screen.getByRole('region', { name: '分发概况' });
    expect(panel).toHaveTextContent('全量复制Replication');
    expect(panel).toHaveTextContent('空选择器，匹配任意标签');
    expect(panel).toHaveTextContent('子组 minClusters = 0');
    expect(panel).toHaveTextContent('向每个被调度选中的目标集群分发一份完整应用');
    expect(panel).toHaveTextContent('已注册5标签匹配5当前已绑定3');
    fireEvent.click(screen.getByText('标签匹配、未列入当前绑定（1）'));
    expect(screen.getByRole('link', { name: 'east/remote' })).toHaveAttribute('href', expect.stringMatching(/clusters\?q=remote$/));
  });
  it('shows dynamic strategy details, distinct empty and unreadable rules, and explicit zero coverage', () => {
    const { rerender } = render(<PropagationSummary detail={{ ...detail, coverage: { registeredClusters: 5 },
      summary: { ...detail.summary, distribution: { strategy: 'Dividing', dividingType: 'Dynamic', dynamicStrategy: 'Spread',
        targetScope: 'unknown', selectors: [null, 'region in (east,west)', ''] } },
    }} />);
    expect(screen.getByText('动态 · 按容量 · Spread')).toBeVisible();
    expect(screen.getByText(/可分摊工作负载的期望副本总数/)).toBeVisible();
    expect(screen.getByText('标签范围未知')).toBeVisible();
    expect(screen.getByText('选择器缺失或无法解析')).toBeVisible();
    expect(screen.getByText('region in (east,west)')).toBeVisible();
    fireEvent.click(screen.getByText('其余 1 项选择器'));
    expect(screen.getByText(/空选择器/)).toBeVisible();
    expect(screen.getByRole('region', { name: '分发概况' })).toHaveTextContent('已注册5标签匹配未知当前已绑定3');
    expect(screen.queryByText(/未列入当前绑定/)).toBeNull();

    rerender(<PropagationSummary detail={{ ...detail, coverage: { registeredClusters: 0, labelMatchedClusters: 0 },
      summary: { ...detail.summary, distribution: { strategy: 'Custom', selectors: [null], targetScope: 'none' } },
    }} />);
    expect(screen.getByText('未识别的分发策略')).toBeVisible();
    expect(screen.getByText('Custom')).toBeVisible();
    expect(screen.getByText('无可匹配的标签选择器')).toBeVisible();
    expect(screen.getByRole('region', { name: '分发概况' })).toHaveTextContent('已注册0标签匹配0当前已绑定3');
  });
  it('shows unreported bindings without outputting zero feedback or connectivity', () => {
    render(<PropagationSummary detail={{ ...detail, summary: { ...detail.summary, bindingClusters: null, bindingClusterCount: null } }} />);
    expect(screen.getByRole('region', { name: '分发概况' })).toHaveTextContent('当前已绑定未上报');
    expect(screen.queryByRole('heading', { name: '目标回报' })).toBeNull();
    expect(screen.queryByRole('heading', { name: '集群连接' })).toBeNull();
    expect(screen.queryByText(/未列入当前绑定/)).toBeNull();
  });
  it('separates observed and connected targets and keeps unknown connectivity distinct', () => {
    render(<PropagationSummary detail={{
      summary: {
        name: 'sample', status: 'Progressing', desiredReleases: 3, completedReleases: 0,
        observedReleases: 1, completionKnown: false, bindingClusterCount: 3, bindingClusters: [],
      },
      feeds: [],
      targetClusters: [
        { name: 'a', online: true, status: 'Online', observed: true },
        { name: 'b', online: false, status: 'Offline', observed: false },
        { name: 'c', online: false, status: 'Unknown', observed: false },
      ],
    }} />);
    expect(screen.getByRole('region', { name: '分发概况' })).toHaveTextContent('已回报1未回报2');
    expect(screen.getByRole('region', { name: '分发概况' })).toHaveTextContent('在线1未就绪1未知1');
  });
});
