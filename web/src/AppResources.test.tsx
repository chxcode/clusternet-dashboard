import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { AppResources } from './AppResources';

describe('AppResources', () => {
  it('renders subscriptions and manifests', () => {
    render(
      <AppResources
        subscriptions={[{
          name: 'demo-sub',
          namespace: 'default',
          status: 'Progressing',
          desiredReleases: 3,
          completedReleases: 2,
          observedReleases: 2,
          completionKnown: true,
          bindingClusterCount: 2,
          bindingClusters: ['default/child-a', 'default/child-b'],
        }]}
        manifests={[{
          name: 'demo-manifest',
          namespace: 'default',
          templateKind: 'Deployment',
          templateName: 'nginx',
          templateNamespace: 'demo',
        }]}
        helmReleases={[{
          name: 'demo-helm-demo-app',
          namespace: 'child-a',
          clusterName: 'child-a',
          subscriptionName: 'demo-app',
          subscriptionNamespace: 'demo-app',
          releaseName: 'demo-app',
          targetNamespace: 'demo-app',
          chart: 'demo-chart',
          chartVersion: '1.2.3',
          phase: 'failed',
          description: 'install failed',
          revision: 5,
        }]}
        loading={false}
      />,
    );

    expect(screen.getByText('demo-sub')).toBeInTheDocument();
    expect(screen.getByText('2 / 3')).toBeInTheDocument();
    expect(screen.getByText('demo-manifest')).toBeInTheDocument();
    expect(screen.getByText('Deployment / nginx')).toBeInTheDocument();
    expect(screen.getByText('HelmReleases')).toBeInTheDocument();
    expect(screen.getByText('demo-helm-demo-app')).toBeInTheDocument();
    expect(screen.getByText('failed')).toBeInTheDocument();
    expect(screen.getByText('install failed')).toBeInTheDocument();
  });

  it('renders compatibility hint when completed release count is not available', () => {
    render(
      <AppResources
        subscriptions={[{
          name: 'legacy-sub',
          status: 'Progressing',
          desiredReleases: 18,
          completedReleases: 0,
          observedReleases: 17,
          completionKnown: false,
          bindingClusterCount: 18,
          bindingClusters: [],
        }]}
        manifests={[]}
        loading={false}
      />,
    );

    expect(screen.getByText('17 observed / 18')).toBeInTheDocument();
    expect(screen.getByText('Observed status only')).toBeInTheDocument();
  });

  it('renders empty state when there are no application resources', () => {
    render(<AppResources subscriptions={[]} manifests={[]} loading={false} />);

    expect(screen.getByText('当前没有发现 Subscriptions、Manifests 或 HelmReleases')).toBeInTheDocument();
  });

  it('does not block subscriptions while helm releases are still loading', () => {
    render(
      <AppResources
        subscriptions={[{
          name: 'demo-sub',
          status: 'Progressing',
          desiredReleases: 1,
          completedReleases: 0,
          observedReleases: 0,
          completionKnown: false,
          bindingClusterCount: 1,
          bindingClusters: [],
        }]}
        manifests={[]}
        helmReleasesLoading={true}
        loading={false}
      />,
    );

    expect(screen.getByText('demo-sub')).toBeInTheDocument();
    expect(screen.getByText('正在加载 HelmReleases...')).toBeInTheDocument();
  });
});
