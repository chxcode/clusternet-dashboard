import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { ThemeControls } from './ThemeControls';

function systemPreference(dark: boolean) {
  const media = Object.assign(new EventTarget(), { matches: dark });
  vi.stubGlobal('matchMedia', () => media);
  return (next: boolean) => {
    media.matches = next;
    act(() => { media.dispatchEvent(new Event('change')); });
  };
}

beforeEach(() => localStorage.clear());
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  delete document.documentElement.dataset.mode;
  delete document.documentElement.dataset.accent;
});

it('follows system mode, provides a one-click override and restores both settings on reload', () => {
  const setSystem = systemPreference(false);
  const view = render(<ThemeControls />);
  const root = document.documentElement;
  expect(root).toHaveAttribute('data-mode', 'light');
  setSystem(true);
  expect(root).toHaveAttribute('data-mode', 'dark');
  fireEvent.click(screen.getByRole('button', { name: '切换到白天模式' }));
  fireEvent.click(screen.getByLabelText('外观设置'));
  fireEvent.click(screen.getByRole('radio', { name: '鸢紫' }));
  expect(root).toHaveAttribute('data-accent', 'violet');
  view.unmount();
  render(<ThemeControls />);
  expect(root).toHaveAttribute('data-mode', 'light');
  expect(root).toHaveAttribute('data-accent', 'violet');
  fireEvent.click(screen.getByLabelText('外观设置'));
  fireEvent.click(screen.getByRole('radio', { name: '跟随系统' }));
  expect(root).toHaveAttribute('data-mode', 'dark');
  setSystem(false);
  expect(root).toHaveAttribute('data-mode', 'light');
});

it('returns focus after Escape and closes when the user clicks outside', () => {
  systemPreference(false);
  render(<ThemeControls />);
  const summary = screen.getByLabelText('外观设置');
  fireEvent.click(summary);
  const radio = screen.getByRole('radio', { name: '海蓝' });
  radio.focus();
  fireEvent.keyDown(radio, { key: 'Escape' });
  expect(summary).toHaveFocus();
  expect(summary.closest('details')).not.toHaveAttribute('open');
  fireEvent.click(summary);
  fireEvent.pointerDown(document.body);
  expect(summary.closest('details')).not.toHaveAttribute('open');
});

it('falls back for invalid stored values and remains usable when preference writes are blocked', () => {
  systemPreference(false);
  localStorage.setItem('clusternet.appearance', '{"mode":"__proto__","accent":5}');
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('Unavailable'); });
  render(<ThemeControls />);
  expect(document.documentElement).toHaveAttribute('data-accent', 'teal');
  fireEvent.click(screen.getByRole('button', { name: '切换到夜间模式' }));
  expect(document.documentElement).toHaveAttribute('data-mode', 'dark');
});
