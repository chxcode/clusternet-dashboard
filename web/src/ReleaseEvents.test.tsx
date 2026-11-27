import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ReleaseEvents } from './ReleaseEvents';

afterEach(() => vi.unstubAllGlobals());

it('only reads scoped parent events on demand and distinguishes a failed read from an empty result', async () => {
  const load = vi.fn().mockResolvedValueOnce(new Response('', {status: 403}))
    .mockResolvedValueOnce(new Response(JSON.stringify({events: []})));
  vi.stubGlobal('fetch', load);
  render(<ReleaseEvents namespace="test-ns" uid="object-uid" />);
  expect(load).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', {name: '查看父集群事件'}));
  expect(await screen.findByRole('alert')).toHaveTextContent('403');
  expect(load.mock.calls[0][0]).toContain('/helmrelease-events?namespace=test-ns&uid=object-uid');
  fireEvent.click(screen.getByRole('button', {name: '重试'}));
  expect(await screen.findByText(/当前没有保留/)).toBeInTheDocument();
});
