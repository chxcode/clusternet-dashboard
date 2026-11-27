import { useState } from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import type { HelmRelease } from './api';
import { Releases } from './Releases';

beforeAll(() => {
  if (!HTMLDialogElement.prototype.showModal) {
    HTMLDialogElement.prototype.showModal = function () { this.setAttribute('open', ''); };
    HTMLDialogElement.prototype.close = function () { this.removeAttribute('open'); };
  }
});

function Workbench({ items, initial = '' }: { items: HelmRelease[]; initial?: string }) {
  const [params, setParams] = useState(() => new URLSearchParams(initial));
  return <Releases releases={items} params={params} onQuery={(changes) => {
    const next = new URLSearchParams(params);
    Object.entries(changes).forEach(([key, value]) => value ? next.set(key, value) : next.delete(key));
    setParams(next);
  }} />;
}

describe('Releases', () => {
  afterEach(() => { vi.unstubAllGlobals(); });

  it('reads the owning Description on opening a release and labels its reason separately', async () => {
    const fetch = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ phase: 'Failure', reason: 'controller timed out' })))
      .mockResolvedValueOnce(new Response(JSON.stringify({ phase: 'Success' })));
    vi.stubGlobal('fetch', fetch);
    render(<Workbench items={[{ name: 'app', namespace: 'edge', phase: 'failed',
      atomic: true, upgradeAtomic: false, descriptionOwner: { name: 'distribution', uid: 'description-id' } }]} />);
    expect(fetch).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'app' }));
    const recovery = within(screen.getByRole('region', { name: '恢复发布' }));
    expect(recovery.getByText(/可能先卸载再安装/)).toBeVisible();
    expect(recovery.getByText(/upgradeAtomic.*已关闭/)).toBeVisible();
    const evidence = within(screen.getByRole('region', { name: '分发控制器回报' }));
    expect(await evidence.findByText('controller timed out')).toBeInTheDocument();
    expect(fetch.mock.calls[0][0]).toContain('description-status?namespace=edge&name=distribution&uid=description-id');
    fireEvent.click(evidence.getByRole('button', { name: '刷新分发回报' }));
    expect(await evidence.findByText('阶段 · Success')).toBeInTheDocument();
    expect(evidence.queryByText('controller timed out')).not.toBeInTheDocument();
  });

  it('restores a selected namespaced object independently of pagination, and links to its evidence', () => {
    const release = {
      name: 'example', clusterName: 'child-east', subscriptionNamespace: 'apps', subscriptionName: 'demo', phase: 'failed',
    };
    render(<Workbench items={[
      { ...release, namespace: 'west', description: 'west-specific message' },
      { ...release, namespace: 'east' },
    ]} initial="name=example&namespace=east&phase=failed" />);
    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByText('控制器未回报原因。')).toBeInTheDocument();
    expect(within(dialog).queryByText('west-specific message')).not.toBeInTheDocument();
    expect(within(dialog).getByRole('link', { name: 'apps / demo' })).toHaveAttribute('href', expect.stringContaining('/propagation?namespace=apps&name=demo'));
    expect(within(dialog).getByRole('link', { name: 'child-east' })).toHaveAttribute('href', expect.stringContaining('/clusters?q=child-east'));
    fireEvent.click(within(dialog).getByRole('button', { name: '关闭详情' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('groups pending phases, matches missing phases, and resets unmatched searches', () => {
    render(<Workbench items={[
      { name: 'a', phase: 'pending-install' }, { name: 'b', phase: 'pending-upgrade' }, { name: 'c', clusterName: 'west' },
    ]} initial="phase=pending" />);
    expect(screen.queryByRole('button', { name: 'c' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'b' })).toBeInTheDocument();
    fireEvent.change(screen.getByRole('combobox', { name: '阶段' }), { target: { value: 'unknown' } });
    expect(screen.getByRole('button', { name: 'c' })).toBeInTheDocument();
    fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'no-match' } });
    expect(screen.getByText(/没有匹配的发布/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '清除筛选' }));
    expect(screen.getByRole('button', { name: 'a' })).toBeInTheDocument();
  });

  it('paginates and returns to the first page on search', () => {
    const items = Array.from({ length: 22 }, (_, i) => ({ name: 'r-' + i, clusterName: 'east-' + i }));
    render(<Workbench items={items} initial="page=999" />);
    expect(screen.getByText('21–22 / 共 22 条')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'r-0' })).not.toBeInTheDocument();
    fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'east-0' } });
    expect(screen.getByRole('button', { name: 'r-0' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '下一页' })).toBeDisabled();
  });

  it('keeps stale evidence visible after a refresh error and avoids showing an empty inventory before loading', () => {
    const common = { params: new URLSearchParams(), onQuery: vi.fn() };
    const { rerender } = render(<Releases releases={undefined} {...common} />);
    expect(screen.getByRole('status')).toHaveTextContent('正在加载');
    expect(screen.queryByText('当前没有 HelmRelease。')).not.toBeInTheDocument();
    rerender(<Releases releases={[{ name: 'cached' }]} error="503" {...common} />);
    expect(screen.getByRole('alert')).toHaveTextContent('503');
    expect(screen.getByRole('button', { name: 'cached' })).toBeInTheDocument();
  });
});
