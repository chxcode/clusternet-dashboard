import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';

const json = (value: unknown) => new Response(JSON.stringify(value));
const sub = (name: string) => ({ name, namespace: 'apps', status: 'Progressing', desiredReleases: 2, completedReleases: 0,
  observedReleases: 1, completionKnown: false, bindingClusterCount: 2, bindingClusters: [] });

beforeEach(() => window.history.replaceState({}, '', '/'));
afterEach(() => vi.unstubAllGlobals());

describe('workspace navigation', () => {
  it('recovers a subscription and filter from a direct URL and ignores a stale response after history navigation', async () => {
    window.history.replaceState({}, '', '/propagation?namespace=apps&name=one&filter=offline');
    let finishOld: (r: Response) => void = () => {};
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const path = String(input);
      if (path.endsWith('/context')) return json({ name: 'hub-test', configured: true });
      if (path.endsWith('/subscriptions')) return json({ subscriptions: [sub('one'), sub('two')] });
      if (path.endsWith('/apps/one')) return new Promise<Response>(resolve => { finishOld = resolve; });
      if (path.endsWith('/apps/two')) return json({ summary: sub('two'), feeds: [], targetClusters: [] });
      return json({});
    });
    vi.stubGlobal('fetch', fetchMock);
    render(<App />);
    expect(await screen.findByText(/正在加载 apps \/ one/)).toBeInTheDocument();
    act(() => { window.history.pushState({}, '', '/propagation?namespace=apps&name=two&filter=offline'); window.dispatchEvent(new PopStateEvent('popstate')); });
    expect(within(await screen.findByRole('region', { name: '订阅详情' })).getByText('two')).toBeInTheDocument();
    expect(screen.getByLabelText('按分发状态筛选')).toHaveValue('offline');
    await act(async () => { finishOld(json({ summary: sub('one'), feeds: [], targetClusters: [] })); });
    expect(within(screen.getByRole('region', { name: '订阅详情' })).getByText('two')).toBeInTheDocument();
    expect(fetchMock.mock.calls.some(([path]) => String(path).endsWith('/diagnostics'))).toBe(false);
    fireEvent.click(screen.getByRole('link', { name: '集群诊断' }));
    await waitFor(() => expect(window.location.pathname).toBe('/diagnostics'));
    expect(screen.getByRole('heading', { name: '集群诊断' })).toBeInTheDocument();
  });
});
