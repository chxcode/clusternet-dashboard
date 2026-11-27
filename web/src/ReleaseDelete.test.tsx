import { act, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import type { HelmRelease } from './api';
import { Releases } from './Releases';

const release: HelmRelease = {
  namespace: 'east', name: 'web', uid: 'original-uid', resourceVersion: '12', phase: 'failed',
  descriptionOwner: { name: 'distribution', uid: 'distribution-uid' }, controlledByDescription: true,
};
const props = { releases: [release], params: new URLSearchParams({ namespace: 'east', name: 'web' }), onQuery: vi.fn(), deleteEnabled: true };
const deleteButton = () => screen.queryByRole('button', { name: '删除发布对象…' });
const accepted = () => Response.json({ status: 'deletionRequested' }, { status: 202 });

function mockRequests(remove = vi.fn().mockResolvedValue(accepted())) {
  vi.stubGlobal('fetch', (path: string, init: RequestInit) => init.method === 'DELETE'
    ? remove(path, init) : Promise.resolve(Response.json({ phase: 'Failure', reason: 'failed to distribute' })));
  return remove;
}

function openConfirmation() {
  fireEvent.click(screen.getByRole('button', { name: '删除发布对象…' }));
  return {
    type: (value: string) => fireEvent.change(screen.getByRole('textbox', { name: /输入完整/ }), { target: { value } }),
    confirm: screen.getByRole('button', { name: '确认删除' }),
  };
}

afterEach(() => vi.unstubAllGlobals());

it('only offers deletion for current failed objects with a confirmed controlling Description and write access', async () => {
  const remove = mockRequests();
  const page = render(<Releases {...props} deleteEnabled={false} />);
  await screen.findByText('failed to distribute');
  expect(deleteButton()).toBeNull();
  for (const changed of [
    { phase: 'deployed' }, { uid: undefined }, { resourceVersion: undefined }, { controlledByDescription: false },
    { descriptionOwner: undefined }, { descriptionOwner: { name: 'dist', uid: '' } }, { deleting: true },
  ]) {
    page.rerender(<Releases {...props} releases={[{ ...release, ...changed }]} />);
    expect(deleteButton()).toBeNull();
  }
  expect(screen.getByText(/等待控制器清理/)).toBeVisible();
  page.rerender(<Releases {...props} error="could not reload releases" />);
  expect(deleteButton()).toBeNull();
  expect(remove).not.toHaveBeenCalled();
});

it('requires the exact path, submits UID and resourceVersion once across dialog visits, and keeps the accepted receipt tied to its UID', async () => {
  let finish!: (response: Response) => void;
  const remove = mockRequests(vi.fn(() => new Promise<Response>(resolve => { finish = resolve; })));
  const onDeleted = vi.fn();
  const onRefresh = vi.fn();
  const page = render(<Releases {...props} onDeleted={onDeleted} onRefresh={onRefresh} />);
  await screen.findByText('failed to distribute');
  const { type, confirm } = openConfirmation();
  expect(screen.getByText(/可能卸载子集群上的 Helm 应用/)).toBeVisible();
  expect(screen.getByText(/保留终结器/)).toBeVisible();
  for (const wrong of ['web', ' EAST/web', 'east/web ']) {
    type(wrong);
    expect(confirm).toBeDisabled();
  }
  type('east/web');
  fireEvent.click(confirm);
  expect(remove).toHaveBeenCalledOnce();
  const [path, request] = remove.mock.calls[0];
  expect(path).toMatch(/\/api\/helmreleases\/east\/web$/);
  expect(request.method).toBe('DELETE');
  expect(JSON.parse(request.body)).toEqual({ uid: 'original-uid', resourceVersion: '12' });
  page.rerender(<Releases {...props} params={new URLSearchParams()} onDeleted={onDeleted} />);
  page.rerender(<Releases {...props} onDeleted={onDeleted} />);
  expect(screen.getByText(/正在提交 east\/web/)).toBeVisible();
  expect(deleteButton()).toBeNull();
  expect(onDeleted).not.toHaveBeenCalled();
  await act(async () => finish(accepted()));
  expect(onDeleted).toHaveBeenCalledOnce();
  expect(screen.getByText(/已受理/)).toHaveTextContent('original-uid');
  page.rerender(<Releases {...props} releases={[]} onDeleted={onDeleted} onRefresh={onRefresh} />);
  expect(screen.getByText(/已受理/)).toBeVisible();
  fireEvent.click(screen.getByRole('button', { name: '刷新发布状态' }));
  expect(onRefresh).toHaveBeenCalledOnce();
  page.rerender(<Releases {...props} releases={[{ ...release, uid: 'replacement-uid' }]} onDeleted={onDeleted} />);
  expect(screen.getByText(/已受理/)).toHaveTextContent('当前显示的是同名的新对象，UID replacement-uid，阶段 failed');
  expect(openConfirmation().confirm).toBeDisabled();
  expect(remove).toHaveBeenCalledOnce();
});

it('drops typed consent on version or identity changes and requires a fresh read after a conflict', async () => {
  const remove = mockRequests(vi.fn().mockResolvedValue(new Response(null, { status: 409 })));
  const onRefresh = vi.fn();
  const onDeleted = vi.fn();
  const page = render(<Releases {...props} onDeleted={onDeleted} onRefresh={onRefresh} />);
  await screen.findByText('failed to distribute');
  openConfirmation().type('east/web');
  const updated = { ...release, resourceVersion: '13' };
  page.rerender(<Releases {...props} releases={[updated]} onDeleted={onDeleted} onRefresh={onRefresh} />);
  expect(screen.queryByRole('textbox')).toBeNull();
  const form = openConfirmation();
  expect(form.confirm).toBeDisabled();
  form.type('east/web');
  fireEvent.click(form.confirm);
  const panel = within(screen.getByRole('region', { name: '删除失败发布' }));
  expect(await panel.findByRole('alert')).toHaveTextContent('发布对象已变更');
  expect(deleteButton()).toBeDisabled();
  fireEvent.click(panel.getByRole('button', { name: '刷新发布详情' }));
  expect(onRefresh).toHaveBeenCalledOnce();
  expect(onDeleted).not.toHaveBeenCalled();
  expect(JSON.parse(remove.mock.calls[0][1].body).resourceVersion).toBe('13');
  page.rerender(<Releases {...props} releases={[updated]} releasesUpdatedAt={1234} />);
  expect(openConfirmation().confirm).toBeDisabled();
  page.rerender(<Releases {...props} releases={[{ ...updated, uid: 'new-uid' }]} />);
  expect(openConfirmation().confirm).toBeDisabled();
});

it.each([
  [() => Promise.resolve(new Response(null, { status: 403 })), '删除被拒绝'],
  [() => Promise.reject(new Error('disconnected')), '未能核实'],
  [() => Promise.resolve(Response.json({ status: 'ok' }, { status: 202 })), '未能核实'],
])('offers to refresh when confirmation fails without claiming deletion', async (result, message) => {
  mockRequests(vi.fn(result));
  const onDeleted = vi.fn();
  const onRefresh = vi.fn();
  render(<Releases {...props} onDeleted={onDeleted} onRefresh={onRefresh} />);
  await screen.findByText('failed to distribute');
  const { type, confirm } = openConfirmation();
  type('east/web');
  fireEvent.click(confirm);
  expect(await screen.findByRole('alert')).toHaveTextContent(message);
  expect(deleteButton()).toBeDisabled();
  fireEvent.click(screen.getByRole('button', { name: '刷新发布详情' }));
  expect(onRefresh).toHaveBeenCalledOnce();
  expect(onDeleted).not.toHaveBeenCalled();
  expect(screen.queryByText(/已受理/)).toBeNull();
});
