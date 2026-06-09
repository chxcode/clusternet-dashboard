import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { ClusterList } from './ClusterList';

describe('ClusterList', () => {
  it('renders a green online dot for online child clusters', () => {
    render(<ClusterList clusters={[{ name: 'child-a', status: 'Online', online: true }]} loading={false} />);

    const dot = screen.getByLabelText('child-a online');
    expect(dot).toHaveClass('status-dot--online');
    expect(screen.getByText('child-a')).toBeInTheDocument();
    expect(screen.getByText('Online')).toBeInTheDocument();
  });

  it('renders managed cluster labels', () => {
    render(<ClusterList clusters={[{
      name: 'child-with-labels',
      status: 'Online',
      online: true,
      labels: { customer: 'demo-app', app: 'sample-app' },
    }]} loading={false} />);

    expect(screen.getByText('customer=demo-app')).toBeInTheDocument();
    expect(screen.getByText('app=sample-app')).toBeInTheDocument();
  });

  it('renders heartbeat and puts abnormal clusters first', () => {
    render(<ClusterList clusters={[
      { name: 'online-child', status: 'Online', online: true, lastObservedTime: '2026-06-09T10:00:00Z' },
      { name: 'offline-child', status: 'Offline', online: false, lastObservedTime: '2026-06-08T10:00:00Z' },
    ]} loading={false} />);

    const rows = screen.getAllByRole('article');
    expect(rows[0]).toHaveTextContent('offline-child');
    expect(screen.getByText('最后心跳 2026-06-08T10:00:00Z')).toBeInTheDocument();
    expect(screen.getByText('离线 / 异常 1')).toBeInTheDocument();
  });

  it('renders empty state when there are no child clusters', () => {
    render(<ClusterList clusters={[]} loading={false} />);

    expect(screen.getByText('当前没有发现子集群')).toBeInTheDocument();
  });
});
