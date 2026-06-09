import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Overview } from './Overview';

describe('Overview', () => {
  it('renders high level dashboard metrics', () => {
    render(
      <Overview
        clusters={[{ name: 'online-a', status: 'Online', online: true }, { name: 'offline-b', status: 'Offline', online: false }]}
        subscriptions={[{ name: 'sub-a', status: 'Completed', desiredReleases: 2, completedReleases: 2, observedReleases: 2, completionKnown: true, bindingClusterCount: 1, bindingClusters: [] }]}
        manifests={[{ name: 'manifest-a', templateKind: 'Deployment' }]}
        globalizations={[{ name: 'global-a', kind: 'Globalization', overrideCount: 2 }]}
        localizations={[{ name: 'local-a', namespace: 'default', kind: 'Localization', overrideCount: 1 }]}
        feedInventories={[{ name: 'feed-a', namespace: 'default', feedCount: 3 }]}
      />,
    );

    expect(screen.getByText('在线子集群')).toBeInTheDocument();
    expect(screen.getByText('1 / 2')).toBeInTheDocument();
    expect(screen.getByText('Subscriptions')).toBeInTheDocument();
    expect(screen.getByText('1')).toBeInTheDocument();
    expect(screen.getByText('分发物料')).toBeInTheDocument();
    expect(screen.getByText('4')).toBeInTheDocument();
  });
});
