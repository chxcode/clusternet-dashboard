import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { PropagationSummary } from './PropagationSummary';

describe('PropagationSummary', () => {
  it('renders observed/not observed and online/offline summaries', () => {
    render(<PropagationSummary detail={{
      summary: {
        name: 'demo-app', status: 'Progressing', desiredReleases: 2, completedReleases: 0,
        observedReleases: 1, completionKnown: false, bindingClusterCount: 2, bindingClusters: [],
      },
      feeds: [],
      targetClusters: [
        { name: 'a', online: true, status: 'Online', observed: true },
        { name: 'b', online: false, status: 'Offline', observed: false },
      ],
    }} />);

    expect(screen.getByText('Observed 1')).toBeInTheDocument();
    expect(screen.getByText('Not observed 1')).toBeInTheDocument();
    expect(screen.getByText('Online 1')).toBeInTheDocument();
    expect(screen.getByText('Offline 1')).toBeInTheDocument();
  });
});
