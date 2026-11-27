import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';

const json = (value: unknown) => new Response(JSON.stringify(value));
const release = { name: 'demo-release', namespace: 'test', phase: 'failed', clusterName: 'child', subscriptionNamespace: 'apps', subscriptionName: 'demo' };

beforeEach(() => window.history.replaceState({}, '', '/'));
afterEach(() => vi.unstubAllGlobals());

describe('dashboard workspace', () => {
  it('keeps unrelated metrics available, links to failed releases and retains previous data when refresh fails', async () => {
    let failed = false;
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      const path = String(input);
      if (path.endsWith('/context')) return json({ name: 'hub-test', configured: true });
      if (path.endsWith('/clusters')) return json({ clusters: [{ name: 'child', online: true }] });
      if (path.endsWith('/helmreleases')) return failed ? new Response('', { status: 503 }) : json({ helmReleases: [release] });
      if (path.endsWith('/manifests')) return new Response('', { status: 403 });
      return json({});
    }));
    render(<App />);
    const metrics = await screen.findByRole('region', { name: '概览指标' });
    await waitFor(() => expect(within(metrics).getByRole('link', { name: /失败发布/ })).toHaveTextContent('1'));
    expect(screen.getByText('hub-test')).toBeInTheDocument();
    expect(within(screen.getByRole('region', { name: '分发物料' })).getByText('读取失败')).toBeInTheDocument();
    fireEvent.click(within(metrics).getByRole('link', { name: /失败发布/ }));
    expect(window.location.search).toBe('?phase=failed');
    expect(await screen.findByText('demo-release')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('button', { name: '刷新' })).toBeEnabled());
    failed = true;
    fireEvent.click(screen.getByRole('button', { name: '刷新' }));
    await screen.findByRole('alert');
    expect(screen.getByText('demo-release')).toBeInTheDocument();
  });

  it('keeps cached release rows scoped to the parent context', async () => {
    window.history.replaceState({}, '', '/releases');
    let contextName = 'parent-a';
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      if (String(input).endsWith('/context')) return json({name: contextName, configured: true});
      return contextName === 'parent-a' ? json({helmReleases: [release]}) : new Response('', {status: 403});
    }));
    render(<App />);
    expect(await screen.findByText('demo-release')).toBeInTheDocument();
    contextName = 'parent-b';
    fireEvent.click(screen.getByRole('button', {name:'刷新'}));
    expect(await screen.findByText('parent-b')).toBeInTheDocument();
    expect(await screen.findByRole('alert')).toHaveTextContent('403');
    expect(screen.queryByText('demo-release')).not.toBeInTheDocument();
  });

  it('reports unconfigured clients without showing zero fleet metrics', async () => {
    const fetchMock = vi.fn(async () => json({ name: '', configured: false }));
    vi.stubGlobal('fetch', fetchMock);
    render(<App />);
    expect(await screen.findByText(/尚未配置 Kubernetes 客户端/)).toBeInTheDocument();
    expect(screen.queryByRole('region', { name: '概览指标' })).not.toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('identifies an in-cluster connection and prefers an operator-supplied name', async () => {
    let displayName = '';
    vi.stubGlobal('fetch', vi.fn(async (url: RequestInfo | URL) => String(url).endsWith('/context')
      ? json({ name: 'in-cluster', configured: true, inCluster: true, displayName, version: 'v1.2.3', labelEditing: true })
      : json({})));
    render(<App />);
    expect(await screen.findByText('集群内连接')).toBeInTheDocument();
    expect(screen.getByText('版本 v1.2.3')).toBeInTheDocument();
    expect(screen.getByText('标签编辑已启用')).toBeInTheDocument();
    displayName = 'Operations hub';
    await waitFor(() => expect(screen.getByRole('button', { name: '刷新' })).toBeEnabled());
    fireEvent.click(screen.getByRole('button', { name: '刷新' }));
    expect(await screen.findByText('Operations hub')).toBeInTheDocument();
  });
});
