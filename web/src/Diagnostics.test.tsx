import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { DiagnosticsPanel } from './Diagnostics';

describe('DiagnosticsPanel', () => {
  it('shows component details, all discovered resources, and distinguishes failed checks from denials', () => {
    render(<DiagnosticsPanel diagnostics={{
      completedReleasesSupport: 'not-observed',
      components: [{
        name: 'clusternet-scheduler', namespace: 'clusternet-system', kind: 'Deployment',
        image: 'ghcr.io/clusternet/clusternet-scheduler:v0.18.1', version: 'v0.18.1', role: 'scheduler',
        readyReplicas: 1, desiredReplicas: 1,
        availableReplicas: 1, updatedReplicas: 1, unavailableReplicas: 0, generation: 8, observedGeneration: 7,
        conditions: [{ type: 'Available', status: 'True', reason: 'MinimumReplicasAvailable', message: 'Minimum availability reached', lastTransitionTime: '2026-01-02T03:04:05Z' }],
      }],
      resources: Array.from({ length: 15 }, (_, i) => ({
        name: `resource-${i}`, group: 'apps.clusternet.io', version: 'v1alpha1', kind: 'Resource', namespaced: true, verbs: ['get', 'list'],
      })),
      readOnlyRBACChecks: [
        { group: 'apps.clusternet.io', resource: 'subscriptions', allowed: true },
        { group: 'apps.clusternet.io', resource: 'subscriptions', verb: 'get', allowed: false },
        { group: 'apps.clusternet.io', resource: 'manifests', allowed: false },
        { group: 'clusters.clusternet.io', resource: 'managedclusters', allowed: false, error: 'timeout' },
      ],
    }} loading={false} />);
    expect(screen.getByText(/当前订阅均未上报完成数/)).toBeInTheDocument();
    expect(screen.getByText('scheduler')).toBeInTheDocument();
    expect(screen.getByText('v0.18.1')).toBeInTheDocument();
    expect(screen.getByText('就绪 1/1')).toBeInTheDocument();
    const disclosure = screen.getByText('查看副本与状态条件');
    fireEvent.click(disclosure);
    expect(disclosure.closest('details')).toHaveAttribute('open');
    expect(screen.getByText('不可用副本').nextElementSibling).toHaveTextContent('0');
    expect(screen.getByText('期望代数 8 / 已观测代数 7')).toBeVisible();
    expect(screen.getByText('Available').closest('dt')).toHaveTextContent('True');
    expect(screen.getByText('MinimumReplicasAvailable')).toBeVisible();
    expect(screen.getByText('Minimum availability reached')).toBeVisible();
    expect(screen.getByText(/最近状态变化/).querySelector('time')).toHaveAttribute('datetime', '2026-01-02T03:04:05Z');
    fireEvent.click(disclosure);
    expect(disclosure.closest('details')).not.toHaveAttribute('open');
    expect(within(screen.getByRole('table', { name: 'API 资源' })).getAllByRole('rowheader')).toHaveLength(15);
    expect(screen.getByText('resource-14')).toBeInTheDocument();
    expect(screen.getByText('允许').closest('li')).toHaveTextContent('subscriptions');
    expect(screen.getByText('list apps.clusternet.io/manifests').closest('li')).toHaveTextContent('拒绝');
    expect(screen.getByText('get apps.clusternet.io/subscriptions').closest('li')).toHaveTextContent('拒绝');
    expect(screen.getByText('检查失败').closest('li')).toHaveTextContent('timeout');
  });

  it('keeps permissions visible when a diagnostic source cannot be read', () => {
    render(<DiagnosticsPanel diagnostics={{
      components: [], resources: [], completedReleasesSupport: 'unknown',
      readOnlyRBACChecks: [{ group: 'apps', resource: 'deployments', verb: 'list', allowed: false }],
      sectionErrors: { components: 'Forbidden', subscriptions: 'ServiceUnavailable', resources: 'Unknown' },
    }} loading={false} />);
    expect(screen.getByText('list apps/deployments').closest('li')).toHaveTextContent('拒绝');
    expect(screen.getByText(/组件读取失败/)).toHaveTextContent('缺少读取权限');
    expect(screen.getByText(/订阅读取失败/)).toHaveTextContent('暂不可用');
    expect(screen.getByText(/API 资源发现未完成/)).toBeVisible();
    expect(screen.queryByText('未发现控制面组件')).not.toBeInTheDocument();
    expect(screen.queryByText('未发现 Clusternet API 资源')).not.toBeInTheDocument();
  });
});
