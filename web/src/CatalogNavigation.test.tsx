import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import App from './App';

afterEach(() => { vi.unstubAllGlobals(); });

it('loads registration history and distribution resources only after opening each view', async () => {
  const called: string[] = [];
  const payloads: Record<string, unknown> = {
    context: {name:'test-hub',configured:true},
    clusters: {clusters:[]},
    registrations: {registrations:[{name:'sample-registration'}]},
    subscriptions: {subscriptions:[]}, manifests:{manifests:[]},
    bases:{bases:[{name:'base-for-test',namespace:'tenant',feeds:[]}]},
  };
  vi.stubGlobal('fetch',vi.fn(async (input: RequestInfo | URL) => {
    const endpoint = String(input).split('/').pop()!;
    called.push(endpoint);
    return new Response(JSON.stringify(payloads[endpoint]));
  }));
  window.history.replaceState({}, '', '/clusters');
  render(<App />);
  await screen.findByText('当前没有发现子集群');
  expect(called).not.toContain('registrations');
  fireEvent.click(screen.getByRole('link', {name:'注册请求'}));
  expect(await screen.findByText('sample-registration')).toBeVisible();
  expect(window.location.search).toBe('?view=registrations');
  fireEvent.click(screen.getByRole('link', {name:'订阅与清单'}));
  await screen.findByText('当前没有订阅。');
  expect(called).not.toContain('bases');
  fireEvent.click(within(screen.getByRole('navigation',{name:'分发资源视图'})).getByRole('link',{name:'Base'}));
  expect(await screen.findByText('base-for-test')).toBeVisible();
  expect(window.location.search).toBe('?view=bases');
});
