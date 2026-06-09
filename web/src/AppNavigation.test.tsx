import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import App from './App';

describe('App navigation layout', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('uses a sidebar menu and switches between dashboard pages', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith('/clusters')) {
        return new Response(JSON.stringify({ clusters: [] }));
      }
      if (url.endsWith('/subscriptions')) {
        return new Response(JSON.stringify({ subscriptions: [] }));
      }
      if (url.endsWith('/manifests')) {
        return new Response(JSON.stringify({ manifests: [] }));
      }
      if (url.endsWith('/globalizations')) {
        return new Response(JSON.stringify({ globalizations: [] }));
      }
      if (url.endsWith('/localizations')) {
        return new Response(JSON.stringify({ localizations: [] }));
      }
      if (url.endsWith('/feedinventories')) {
        return new Response(JSON.stringify({ feedInventories: [] }));
      }
      if (url.endsWith('/helmreleases')) {
        return new Response(JSON.stringify({ helmReleases: [] }));
      }
      if (url.endsWith('/diagnostics')) {
        return new Response(JSON.stringify({ resources: [], components: [], readOnlyRBACChecks: [] }));
      }
      return new Response('{}');
    }));

    render(<App />);

    expect(screen.getByRole('navigation', { name: 'Dashboard navigation' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Managed Clusters' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Diagnostics' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Diagnostics' }));

    await waitFor(() => expect(window.location.pathname).toBe('/diagnostics'));
    expect(screen.getByRole('heading', { name: 'Diagnostics' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Overview' })).not.toBeInTheDocument();
  });
});
