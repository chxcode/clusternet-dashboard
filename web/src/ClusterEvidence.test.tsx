import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { ClusterList } from './ClusterList';

describe('cluster diagnostic details', () => {
  it('opens from the cluster row and shows the latest report, health evidence and scheduling capacity', () => {
    render(<ClusterList loading={false} clusters={[{
      name: 'sample', status: 'Unknown', online: false, platform: 'linux/amd64', lastObservedTime: '2026-04-09T12:00:00Z',
      evidence: {
        conditions: [{ type: 'Ready', status: 'Unknown', reason: 'StatusUnknown', message: 'Delayed status report',
          lastTransitionTime: '2026-04-09T12:03:00Z' }],
        readyz: false, livez: true, heartbeatFrequencySeconds: 20, clusterType: 'Edge', syncMode: 'Pull',
        taints: [{ key: 'zone', effect: 'NoSchedule' }],
        nodeStatistics: { readyNodes: 2, notReadyNodes: 0 },
        capacity: { cpu: '4', memory: '16Gi', pods: '0' }, allocatable: { cpu: '3500m' },
      },
    }]} />);
    const toggle = screen.getByRole('button', { name: '查看 sample 的集群诊断' });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    fireEvent.click(toggle);
    const panel = within(screen.getByRole('region', { name: 'sample 的集群诊断' }));
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(panel.getByText('Ready = Unknown')).toBeVisible();
    expect(panel.getByText('StatusUnknown')).toBeVisible();
    expect(panel.getByText('Delayed status report')).toBeVisible();
    expect(panel.getByTitle('2026-04-09T12:00:00Z')).toBeVisible();
    expect(panel.getByTitle('2026-04-09T12:03:00Z')).toBeVisible();
    expect(panel.getByText('readyz').parentElement).toHaveTextContent('未通过');
    expect(panel.getByText('livez').parentElement).toHaveTextContent('通过');
    expect(panel.getByText('healthz').parentElement).toHaveTextContent('未上报');
    for (const value of ['20 秒', 'linux/amd64', 'Edge', 'Pull']) expect(panel.getByText(value)).toBeVisible();
    expect(panel.getByText('未就绪').parentElement).toHaveTextContent('0');
    expect(panel.getByText('失联').parentElement).toHaveTextContent('未上报');
    expect(panel.getByText('zone').closest('dt')?.nextElementSibling).toHaveTextContent('NoSchedule');
    expect(within(panel.getByRole('row', { name: /^CPU/ })).getAllByRole('cell').map(cell => cell.textContent)).toEqual(['4', '3500m']);
    expect(within(panel.getByRole('row', { name: /^Pod/ })).getAllByRole('cell').map(cell => cell.textContent)).toEqual(['0', '未上报']);
    fireEvent.click(toggle);
    expect(screen.queryByRole('region', { name: 'sample 的集群诊断' })).not.toBeInTheDocument();
  });

  it('supports older API responses without diagnostic fields', () => {
    render(<ClusterList loading={false} clusters={[{ name: 'older', status: 'Unknown', online: false }]} />);
    fireEvent.click(screen.getByRole('button', { name: '查看 older 的集群诊断' }));
    expect(screen.getByText('未上报健康条件')).toBeVisible();
    expect(screen.getByText('未提供污点信息')).toBeVisible();
    expect(screen.getByText('心跳上报周期').parentElement).toHaveTextContent('未上报');
    expect(within(screen.getByRole('row', { name: /^CPU/ })).getAllByRole('cell', { name: '未上报' })).toHaveLength(2);
  });

  it('distinguishes an empty taint list from unavailable diagnostic data', () => {
    render(<ClusterList loading={false} clusters={[{ name: 'plain', status: 'Online', online: true, evidence: { taints: [] } }]} />);
    fireEvent.click(screen.getByRole('button', { name: '查看 plain 的集群诊断' }));
    expect(screen.getByText('未配置调度污点')).toBeVisible();
  });
});
