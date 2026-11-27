import { useState } from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { analyzeYaml, checkYamlSyntax, MAX_YAML_BYTES, YamlEditor } from './YamlEditor';

const label = '配置 YAML';
const field = () => screen.getByRole('textbox', { name: label }) as HTMLTextAreaElement;
function Editable({ initial }: { initial: string }) {
  const [value, setValue] = useState(initial);
  return <YamlEditor label={label} value={value} onChange={setValue}/>;
}
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

it('paints AST scalar types, CST comments and markers without interpreting user text as HTML', () => {
  const source = '# 文件\n---\napiVersion: v1\nimage: "<img src=x onerror=alert(1)> # literal"\ncount: 3\nenabled: true\nempty: null\nmessage: | # header\n  # text in block\n  two\n---\nname: two\n';
  const fetch = vi.fn(); vi.stubGlobal('fetch', fetch);
  const { container } = render(<Editable initial={source}/>);
  const text = (cls: string) => Array.from(container.querySelectorAll('.yaml-token-' + cls), n => n.textContent);
  expect(field()).toHaveValue(source); expect(container.querySelector('img')).toBeNull();
  expect(container.querySelector('.yaml-paint')?.textContent).toBe(source);
  expect(text('key')).toContain('apiVersion'); expect(text('number')).toContain('3');
  expect(text('boolean')).toContain('true'); expect(text('null')).toContain('null');
  expect(text('comment')).toEqual(['# 文件', '# header']); expect(text('marker')).toEqual(['---', '|', '---']);
  expect(text('string')).toContain('"'+ '<img src=x onerror=alert(1)> # literal' + '"'); expect(text('string')).toContain('  # text in block\n');
  expect(screen.getAllByRole('textbox')).toHaveLength(1); expect(fetch).not.toHaveBeenCalled();
});

it('retains native editable text and IME composition and restores syntax colors after input', () => {
  const { container } = render(<Editable initial="键: 原值"/>);
  const input = field(); input.focus(); input.setSelectionRange(3, 5);
  expect(input.value.slice(input.selectionStart,input.selectionEnd)).toBe('原值');
  fireEvent.compositionStart(input); expect(container.querySelector('.yaml-paint')).toBeNull();
  fireEvent.change(input, { target: { value: '键: 北京' } }); fireEvent.compositionEnd(input);
  expect(field()).toHaveValue('键: 北京'); expect(container.querySelector('.yaml-paint')?.textContent).toBe('键: 北京');
  expect(fireEvent.keyDown(input, {key:'Tab'})).toBe(true); expect(input).toHaveFocus();
});

it('finds duplicate keys by original line and UTF-16 column and focuses that location after CRLF import', () => {
  const raw = '# 🍃 配置\r\n配置:\r\n  名: a\r\n  名: b\r\n';
  expect(checkYamlSyntax(raw)).toEqual([{line:4,column:3,message:'映射中存在重复的键。'}]);
  render(<Editable initial={raw}/>);
  expect(field()).toHaveAttribute('aria-invalid','true');
  expect(field().closest('.yaml-editor')).toHaveAttribute('data-invalid', 'true');
  fireEvent.click(screen.getByRole('button',{name:'第 4 行，第 3 列'}));
  const input = field(); expect(input).toHaveFocus();
  expect(input.selectionStart).toBe(input.value.lastIndexOf('名'));
  expect(input.value.slice(input.selectionStart, input.selectionEnd)).toBe('名');
  fireEvent.change(input, { target:{value:'配置: 有效'} });
  expect(screen.queryByText(/YAML 校验发现/)).toBeNull();
  expect(field()).not.toHaveAttribute('aria-invalid');
});

it('accepts multiple workload documents but reports the second chart document at its marker; applies matching YAML schemas', () => {
  const source = 'name: on\n---\nvalue: 1\n';
  const workload = analyzeYaml(source), chart = analyzeYaml(source, true);
  expect(workload.issues).toEqual([]); expect(chart.issues).toEqual([{line:2,column:1,message:'此处只接受一份 YAML 文档。'}]);
  expect(workload.spans.find(s => workload.text.slice(s.start,s.end) === 'on')?.tone).toBe('boolean');
  expect(chart.spans.find(s => chart.text.slice(s.start,s.end) === 'on')?.tone).toBe('string');
  expect(checkYamlSyntax('spec: [one, "broken\n').length).toBeGreaterThan(0);
  expect(checkYamlSyntax('# a\rb: {\r')).toEqual(checkYamlSyntax('# a\nb: {\n'));
});

it('applies the exact UTF-8 limit and preserves oversized drafts as selectable text', () => {
  const tail = '汉'.repeat(Math.floor((MAX_YAML_BYTES - 1) / 3));
  const prefix = '#' + ' '.repeat(MAX_YAML_BYTES - new TextEncoder().encode(tail).length - 1);
  const exact = prefix + tail; expect(new TextEncoder().encode(exact)).toHaveLength(MAX_YAML_BYTES);
  expect(checkYamlSyntax(exact)).toEqual([]);
  const over = exact + 'x'; const analysis = analyzeYaml(over); expect(analysis.issues).toHaveLength(1);
  const { container } = render(<YamlEditor label={label} value={over} analysis={analysis} readOnly/>);
  expect(field().value).toBe(over); expect(field().readOnly).toBe(true); expect(field()).toBeEnabled();
  expect(container.querySelector('.yaml-overlay')).toBeNull();
  expect(screen.getByText(/超过 256 KiB，实时语法检查已暂停/)).toBeVisible();
  expect(field()).not.toHaveAttribute('aria-invalid'); expect(field().closest('.yaml-editor')).not.toHaveAttribute('data-invalid');
  expect(screen.queryByRole('button', { name: /第 1 行/ })).toBeNull();
});

it('keeps high-density and off-screen documents editable with bounded paint nodes and synchronized scrolling', () => {
  const source = Array.from({length:3500}, (_, i) => 'key-' + i + ': ' + i).join('\n') + '\n';
  const analysis = analyzeYaml(source); expect(analysis.issues).toEqual([]); expect(analysis.spans.length).toBeGreaterThan(5000);
  const {container, rerender} = render(<YamlEditor label={label} value={source} analysis={analysis} readOnly/>);
  expect(container.querySelectorAll('.yaml-paint span').length).toBeLessThan(200);
  expect(container.querySelector('.yaml-paint')?.textContent).toContain('key-0: 0');
  expect(container.querySelector('.yaml-paint')?.textContent).not.toContain('key-2300:');
  fireEvent.scroll(field(), {target:{scrollTop:2300*20,scrollLeft:25}});
  expect(container.querySelector('.yaml-paint')?.textContent).toContain('key-2300: 2300');
  expect(container.querySelector('.yaml-paint')?.textContent).not.toContain('key-0: 0');
  expect((container.querySelector('.yaml-paint') as HTMLElement).style.transform).toContain('-25px');
  const dense = 'items: [' + Array(2300).fill('1').join(',') + ']\nnext: false\n';
  rerender(<YamlEditor label={label} value={dense} analysis={analyzeYaml(dense)} readOnly/>);
  fireEvent.scroll(field(), {target:{scrollTop:0,scrollLeft:0}});
  expect(container.querySelector('.yaml-paint')?.textContent).toBe(dense);
  expect(container.querySelectorAll('.yaml-paint span').length).toBeLessThan(260);
  expect(container.querySelector('.yaml-token-boolean')).toHaveTextContent('false');
});

it('shows the current native value while supplied analysis is stale and updates diagnostics after deferred large edits', async () => {
  const stale = analyzeYaml('bad: [');
  const {container, rerender} = render(<YamlEditor label={label} value="foo: ok" analysis={stale} disabled/>);
  expect(field()).toBeDisabled(); expect(container.querySelector('.yaml-paint')).toBeNull();
  expect(field()).not.toHaveAttribute('aria-invalid');
  const large = '# ' + 'x'.repeat(18000) + '\nbad: [\n';
  rerender(<YamlEditor label={label} value={large} readOnly/>);
  expect(field()).toHaveValue(large);
  await waitFor(() => expect(field()).toHaveAttribute('aria-invalid','true'));
});

it('discards a superseded background result and requests worker parsing only for the currently edited buffer', async () => {
  type Listener = (event: { data: ReturnType<typeof analyzeYaml> }) => void;
  const workers: TestWorker[] = [];
  class TestWorker {
    onmessage?: Listener;
    terminate = vi.fn();
    postMessage = vi.fn();
    constructor() { workers.push(this); }
  }
  vi.stubGlobal('Worker', TestWorker);
  const first = '# ' + 'a'.repeat(17000) + '\nbad: [';
  const second = first.replace('bad: [', 'good: 3');
  const { rerender, container } = render(<YamlEditor label={label} value={first}/>);
  await waitFor(() => expect(workers).toHaveLength(1));
  expect(workers[0].postMessage).toHaveBeenCalledWith({ source: first, singleDocument: false });
  rerender(<YamlEditor label={label} value={second}/>);
  expect(field().value).toBe(second);
  expect(workers[0].terminate).toHaveBeenCalled();
  workers[0].onmessage?.({ data: analyzeYaml(first) });
  expect(field()).not.toHaveAttribute('aria-invalid');
  await waitFor(() => expect(workers).toHaveLength(2));
  act(() => workers[1].onmessage?.({ data: analyzeYaml(second) }));
  expect(container.querySelector('.yaml-token-number')).toHaveTextContent('3');
});

it('reuses a completed worker across drafts, falls back after a worker error, and closes it when finished', async () => {
  const workers: TestWorker[] = [];
  class TestWorker {
    onmessage?: (event: { data: ReturnType<typeof analyzeYaml> }) => void;
    onerror?: () => void;
    terminate = vi.fn(); postMessage = vi.fn();
    constructor() { workers.push(this); }
  }
  vi.stubGlobal('Worker', TestWorker);
  const one = '# ' + 't'.repeat(18000) + '\nname: chart';
  const view = render(<YamlEditor label={label} value={one} singleDocument/>);
  await waitFor(() => expect(workers).toHaveLength(1));
  act(() => workers[0].onmessage?.({ data: analyzeYaml(one, true) }));
  expect(workers[0].terminate).not.toHaveBeenCalled();
  view.rerender(<YamlEditor label={label} value={one + '\nbad: ['} singleDocument/>);
  await waitFor(() => expect(workers[0].postMessage).toHaveBeenCalledTimes(2));
  expect(workers).toHaveLength(1);
  act(() => workers[0].onerror?.());
  expect(field()).toHaveAttribute('aria-invalid', 'true'); expect(workers[0].terminate).toHaveBeenCalledOnce();
  view.rerender(<YamlEditor label={label} value={one + '\ncount: 2'} singleDocument/>);
  await waitFor(() => expect(workers).toHaveLength(2));
  act(() => workers[1].onmessage?.({ data: analyzeYaml(one + '\ncount: 2', true) }));
  view.unmount(); expect(workers[1].terminate).toHaveBeenCalledOnce();
});
