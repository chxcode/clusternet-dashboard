import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import App from './App';

describe('App', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders the MVP dashboard shell', async () => {
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
      return new Response('{}');
    }));

    render(<App />);

    expect(screen.getByRole('navigation', { name: 'Dashboard navigation' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Overview' })).toBeInTheDocument();
    expect(screen.getByText('默认路径 /clusternet')).toBeInTheDocument();
    expect(screen.getByText('内网部署')).toBeInTheDocument();
    expect(screen.getByText('只读 ServiceAccount')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Managed Clusters' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Propagation Status' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Diagnostics' })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText('分发物料')).toBeInTheDocument());
  });
});
