import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { SubscriptionPicker } from './SubscriptionPicker';

describe('SubscriptionPicker', () => {
  it('lets users select a subscription from the propagation page', () => {
    const onSelect = vi.fn();
    render(<SubscriptionPicker
      subscriptions={[{
        name: 'demo-app',
        namespace: 'demo-app',
        status: 'Progressing',
        desiredReleases: 18,
        completedReleases: 0,
        observedReleases: 17,
        completionKnown: false,
        bindingClusterCount: 18,
        bindingClusters: [],
      }]}
      loading={false}
      error={undefined}
      onSelect={onSelect}
    />);

    expect(screen.getByText('选择 Subscription')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /demo-app/ }));

    expect(onSelect).toHaveBeenCalledWith(expect.objectContaining({ name: 'demo-app', namespace: 'demo-app' }));
  });
});
