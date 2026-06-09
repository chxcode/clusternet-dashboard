import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { SubscriptionSummaryMeta } from './SubscriptionSummary';

describe('SubscriptionSummaryMeta', () => {
  it('renders scheduling, subscriber, and feed summary', () => {
    render(<SubscriptionSummaryMeta subscription={{
      name: 'demo-sub',
      status: 'Progressing',
      desiredReleases: 2,
      completedReleases: 1,
      observedReleases: 1,
      completionKnown: true,
      bindingClusterCount: 2,
      bindingClusters: [],
      schedulerName: 'default-scheduler',
      schedulingStrategy: 'Dividing',
      subscriberCount: 1,
      feedCount: 2,
      feedKinds: ['HelmChart', 'Deployment'],
    }} />);

    expect(screen.getByText('scheduler default-scheduler')).toBeInTheDocument();
    expect(screen.getByText('strategy Dividing')).toBeInTheDocument();
    expect(screen.getByText('subscribers 1')).toBeInTheDocument();
    expect(screen.getByText('feeds 2 · HelmChart, Deployment')).toBeInTheDocument();
  });
});
