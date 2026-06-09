import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { DiagnosticsPanel } from './Diagnostics';

describe('DiagnosticsPanel', () => {
  it('renders compatibility, component roles, versions, resources, and RBAC checks', () => {
    render(<DiagnosticsPanel diagnostics={{
      completedReleasesSupport: 'supported',
      components: [{
        name: 'clusternet-scheduler',
        namespace: 'clusternet-system',
        kind: 'Deployment',
        image: 'ghcr.io/clusternet/clusternet-scheduler:v0.18.1',
        version: 'v0.18.1',
        role: 'scheduler',
        readyReplicas: 1,
        desiredReplicas: 1,
      }],
      resources: [{ name: 'subscriptions', group: 'apps.clusternet.io', version: 'v1alpha1', kind: 'Subscription', namespaced: true, verbs: ['get', 'list'] }],
      readOnlyRBACChecks: [{ group: 'apps.clusternet.io', resource: 'subscriptions', allowed: true }],
    }} loading={false} />);

    expect(screen.getByText('completedReleases supported')).toBeInTheDocument();
    expect(screen.getByText('scheduler')).toBeInTheDocument();
    expect(screen.getByText('v0.18.1')).toBeInTheDocument();
    expect(screen.getByText('subscriptions')).toBeInTheDocument();
    expect(screen.getByText('apps.clusternet.io / subscriptions allowed')).toBeInTheDocument();
  });
});
