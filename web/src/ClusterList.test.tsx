import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ClusterList } from './ClusterList';

describe('ClusterList', () => {
  it('puts abnormal clusters first and retains the precise heartbeat and labels', () => {
    render(<ClusterList clusters={[
      { name: 'online-child', status: 'Online', online: true },
      { name: 'offline-child', status: 'Offline', online: false, lastObservedTime: '2026-06-08T10:00:00Z',
        labels: { app: 'sample-app', customer: 'demo-app', 'example.com/zone': 'west' } },
    ]} loading={false} />);
    const rows = within(screen.getByRole('table')).getAllByRole('row');
    expect(rows[1]).toHaveTextContent('offline-child');
    expect(rows[1]).toHaveTextContent('未就绪');
    expect(rows[2]).toHaveTextContent('在线');
    expect(within(rows[1]).getByTitle('2026-06-08T10:00:00Z')).toHaveAttribute('datetime', '2026-06-08T10:00:00Z');

    const summary = within(rows[1]).getByText('另 1 个标签');
    expect(summary.closest('details')).not.toHaveAttribute('open');
    fireEvent.click(summary);
    expect(summary.closest('details')).toHaveAttribute('open');
    expect(within(rows[1]).getByText('example.com/zone=west')).toBeInTheDocument();
    expect(within(rows[1]).getByText('customer=demo-app')).toBeInTheDocument();
    expect(within(rows[1]).getByText('app=sample-app')).toBeInTheDocument();
  });

  it('distinguishes empty inventory from zero matches and keeps stale data visible', () => {
    const clear = vi.fn();
    const { rerender } = render(<ClusterList clusters={[]} loading={false} totalCount={0} />);
    expect(screen.getByText('当前没有发现子集群')).toBeInTheDocument();
    rerender(<ClusterList clusters={[]} loading={false} totalCount={42} onClear={clear} />);
    expect(screen.getByText('匹配 0 / 共 42 个')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '清除筛选' }));
    expect(clear).toHaveBeenCalledOnce();
    rerender(<ClusterList clusters={[{ name: 'last-good', status: 'Online', online: true }]} totalCount={1} loading={false} error="HTTP 503" />);
    expect(screen.getByRole('alert')).toHaveTextContent('上次读取');
    expect(screen.getByText('last-good')).toBeInTheDocument();
  });

  it('paginates locally and clamps to a valid page when inventory shrinks', () => {
    const clusters = Array.from({ length: 22 }, (_, i) => ({ name: `child-${String(i).padStart(2, '0')}`, online: true, status: 'Online' }));
    const { rerender } = render(<ClusterList clusters={clusters} loading={false} />);
    expect(screen.getAllByRole('rowheader')).toHaveLength(20);
    fireEvent.click(screen.getByRole('button', { name: '下一页' }));
    expect(screen.getAllByRole('rowheader')).toHaveLength(2);
    expect(screen.getByText('child-21')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '下一页' })).toBeDisabled();
    rerender(<ClusterList clusters={clusters.slice(0, 1)} loading={false} />);
    expect(screen.getByText('child-00')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '下一页' })).not.toBeInTheDocument();
  });
});
