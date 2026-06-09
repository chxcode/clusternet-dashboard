import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { SubscriptionDetailView } from './SubscriptionDetail';

describe('SubscriptionDetailView', () => {
  it('renders feeds and target cluster observed status', () => {
    render(<SubscriptionDetailView detail={{
      summary: {
        name: 'demo-app',
        namespace: 'demo-app',
        status: 'Progressing',
        desiredReleases: 2,
        completedReleases: 0,
        observedReleases: 1,
        completionKnown: false,
        bindingClusterCount: 2,
        bindingClusters: ['clusters/child-a', 'clusters/child-b'],
      },
      feeds: [{ apiVersion: 'apps.clusternet.io/v1alpha1', kind: 'HelmChart', name: 'demo-app', namespace: 'demo-app' }],
      targetClusters: [
        { namespace: 'clusters', name: 'child-a', online: true, status: 'Online', observed: true },
        { namespace: 'clusters', name: 'child-b', online: false, status: 'Offline', observed: false },
      ],
    }} />);

    expect(screen.getByText('HelmChart / demo-app')).toBeInTheDocument();
    expect(screen.getByText('child-a')).toBeInTheDocument();
    expect(screen.getByText('Observed')).toBeInTheDocument();
    expect(screen.getByText('Not observed')).toBeInTheDocument();
    expect(screen.getByLabelText('child-a online')).toHaveClass('status-dot--online');
  });
});
