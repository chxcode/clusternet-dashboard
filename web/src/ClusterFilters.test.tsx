import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ClusterFilters } from './ClusterFilters';

describe('ClusterFilters', () => {
  it('renders app label options and emits filter changes', () => {
    const onChange = vi.fn();
    render(<ClusterFilters clusters={[
      { name: 'a', status: 'Online', online: true, labels: { app: 'sample-app', customer: 'demo-app' } },
      { name: 'b', status: 'Offline', online: false, labels: { app: 'vdinsight', customer: 'vd' } },
    ]} value={{ status: 'all', app: 'all', labelQuery: '' }} onChange={onChange} />);

    fireEvent.change(screen.getByLabelText('按应用筛选'), { target: { value: 'sample-app' } });
    fireEvent.change(screen.getByLabelText('按状态筛选'), { target: { value: 'offline' } });
    fireEvent.change(screen.getByLabelText('按标签搜索'), { target: { value: 'customer=demo' } });

    expect(onChange).toHaveBeenCalledWith({ status: 'all', app: 'sample-app', labelQuery: '' });
    expect(onChange).toHaveBeenCalledWith({ status: 'offline', app: 'all', labelQuery: '' });
    expect(onChange).toHaveBeenCalledWith({ status: 'all', app: 'all', labelQuery: 'customer=demo' });
  });
});
