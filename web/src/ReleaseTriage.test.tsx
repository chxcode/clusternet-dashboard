import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Releases } from './Releases';
import type { HelmRelease } from './api';

const release: HelmRelease = { name: 'web', namespace: 'demo', uid: 'release-uid', phase: 'failed', triageIncident: 'incident-id' };
const detail = new URLSearchParams({ phase: 'failed', name: 'web', namespace: 'demo' });
const props = { releases: [release], params: detail, onQuery: vi.fn(), triageEnabled: true };

afterEach(() => vi.unstubAllGlobals());

describe('release triage', () => {
  it('saves state to the selected release with a precondition and keeps failed visible', async () => {
    const fetch = vi.fn().mockResolvedValue(new Response(null, {status: 204}));
    const refresh = vi.fn();
    vi.stubGlobal('fetch', fetch);
    render(<Releases {...props} onTriageSaved={refresh}/>);
    const panel = within(screen.getByRole('region', { name: '故障跟进' }));
    fireEvent.click(panel.getByRole('button', { name: '标记处理中' }));
    expect(await panel.findByText('处理中 · 已保存')).toBeVisible();
    expect(fetch.mock.calls[0][0]).toMatch(/\/api\/helmreleases\/demo\/web\/triage$/);
    expect(fetch.mock.calls[0][1].method).toBe('POST');
    expect(JSON.parse(fetch.mock.calls[0][1].body)).toEqual({ uid: 'release-uid', incident: 'incident-id', before: 'open', state: 'investigating' });
    fireEvent.click(panel.getByRole('button', { name: '标记已处理' }));
    expect(await panel.findByText('已处理 · 已保存')).toBeVisible();
    fireEvent.click(panel.getByRole('button', { name: '重新打开' }));
    expect(await panel.findByText('待跟进 · 已保存')).toBeVisible();
    expect(JSON.parse(fetch.mock.calls[2][1].body)).toEqual({ uid: 'release-uid', incident: 'incident-id', before: 'resolved', state: 'open' });
    expect(refresh).toHaveBeenCalledTimes(3);
    expect(within(screen.getByRole('dialog')).getByText('失败')).toBeVisible();
  });

  it('requires fresh data after a conflict and offers a reload after an uncertain request', async () => {
    const fetch = vi.fn().mockResolvedValueOnce(new Response(null, {status: 409}))
      .mockRejectedValueOnce(new TypeError('network failure'));
    const refresh = vi.fn();
    vi.stubGlobal('fetch', fetch);
    const view = render(<Releases {...props} onTriageSaved={refresh}/>);
    fireEvent.click(screen.getByRole('button', { name: '标记已处理' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('状态已变更');
    expect(screen.getByRole('button', { name: '标记已处理' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: '重新载入' }));
    expect(refresh).toHaveBeenCalledOnce();
    view.rerender(<Releases {...props} releases={[{...release, triage: 'investigating'}]} onTriageSaved={refresh}/>);
    fireEvent.click(screen.getByRole('button', { name: '标记已处理' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('未收到保存结果');
    expect(JSON.parse(fetch.mock.calls[1][1].body).before).toBe('investigating');
  });

  it('handles access denial and shows historical state read-only on successful releases', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, {status: 403})));
    const view = render(<Releases {...props} />);
    fireEvent.click(screen.getByRole('button', { name: '标记处理中' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('写入权限');
    view.rerender(<Releases {...props} triageEnabled={false} />);
    expect(screen.queryByRole('button', { name: '标记处理中' })).not.toBeInTheDocument();
    view.rerender(<Releases {...props} params={new URLSearchParams({ namespace: 'demo', name: 'web' })}
      releases={[{...release, phase: 'deployed', triage: 'resolved'}]} />);
    const dialog = within(screen.getByRole('dialog'));
    expect(dialog.getByText('已处理')).toBeVisible();
    expect(dialog.queryByRole('button', { name: '重新打开' })).not.toBeInTheDocument();
  });

  it('filters failed releases by the URL follow-up state', () => {
    render(<Releases {...props} params={new URLSearchParams({phase: 'failed', triage: 'investigating'})}
      releases={[release, {...release, name:'other', triage:'investigating'}]} />);
    const table = within(screen.getByRole('table', { name:'发布记录' }));
    expect(table.getByText('跟进 · 处理中')).toBeInTheDocument();
    expect(table.queryByText('web')).not.toBeInTheDocument();
  });
});
