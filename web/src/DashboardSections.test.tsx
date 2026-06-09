import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { DashboardSections } from './DashboardSections';

describe('DashboardSections', () => {
  it('renders the Clusternet information architecture sections', () => {
    render(<DashboardSections />);

    expect(screen.getByText('Overview')).toBeInTheDocument();
    expect(screen.getByText('Managed Clusters')).toBeInTheDocument();
    expect(screen.getByText('Subscriptions')).toBeInTheDocument();
    expect(screen.getByText('Propagation Status')).toBeInTheDocument();
    expect(screen.getByText('Diagnostics')).toBeInTheDocument();
  });
});
