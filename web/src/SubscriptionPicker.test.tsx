import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { Subscription } from './api';
import { SubscriptionPicker } from './SubscriptionPicker';

const item: Subscription = {
  name: 'sample', namespace: 'team-a', status: 'Progressing', desiredReleases: 18,
  completedReleases: 0, observedReleases: 17, completionKnown: false, bindingClusterCount: 18, bindingClusters: [],
};

describe('SubscriptionPicker', () => {
  it('picks namespaced subscriptions with the same name and retains stale options', () => {
    const onSelect = vi.fn();
    render(<SubscriptionPicker subscriptions={[item, { ...item, namespace: 'team-b' }]}
      error="503" loading={true} selectedSubscription={item} onSelect={onSelect} />);
    const select = screen.getByRole('combobox', { name: '订阅' });
    expect(select).toHaveValue('team-a/sample');
    expect(select).toBeEnabled();
    expect(screen.getByRole('alert')).toHaveTextContent('503');
    fireEvent.change(select, { target: { value: 'team-b/sample' } });
    expect(onSelect).toHaveBeenCalledWith(expect.objectContaining({ name: 'sample', namespace: 'team-b' }));
  });

  it('shows the deep-linked selection before the list is available', () => {
    render(<SubscriptionPicker subscriptions={[]} loading selectedSubscription={item} onSelect={vi.fn()} />);
    expect(screen.getByRole('combobox', { name: '订阅' })).toHaveValue('team-a/sample');
  });
});
