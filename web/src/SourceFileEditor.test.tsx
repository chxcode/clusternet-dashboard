import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { DistributionCatalog } from './ResourceCatalog';

const chart = { namespace: 'apps', name: 'store', version: '1.0.0', targetNamespace: 'shop' };
const id = { namespace: 'apps', name: 'store', uid: 'chart-uid', resourceVersion: '7' };
const yaml = 'apiVersion: apps.clusternet.io/v1alpha1\nkind: HelmChart\nmetadata: {namespace: apps, name: store}\nspec: {repo: https://charts.example.com, chart: store, version: 1.0.0, targetNamespace: shop}';
const plan = { ...id, changes: [{ field: 'spec.version', before: '1.0.0', after: '1.0.1' }],
  subscriptions: [{namespace:'apps',name:'fleet',boundTargets:2}], targets:['east/a','west/b'], fingerprint:'reviewed' };
const button = (s: string) => screen.getByRole('button', { name: s });
afterEach(() => vi.unstubAllGlobals());

it('creates a source from YAML after showing its impact, including an empty catalog', async () => {
  const fetch = vi.fn().mockResolvedValueOnce(Response.json({ ...plan, mode: 'create' }))
    .mockResolvedValueOnce(Response.json(id, {status:201}));
  vi.stubGlobal('fetch', fetch);
  const saved = vi.fn();
  render(<DistributionCatalog view="helmcharts" result={{loading:false,data:[]}} sourceEditing onVersionSaved={saved} />);
  fireEvent.click(button('新建 HelmChart'));
  fireEvent.change(screen.getByLabelText('HelmChart YAML'), {target:{value:yaml}});
  expect(fetch).not.toHaveBeenCalled();
  fireEvent.click(button('预览配置变更'));
  await screen.findByText('apps/fleet · 已绑定 2');
  expect(fetch).toHaveBeenCalledTimes(1);
  fireEvent.click(button('确认保存配置'));
  await screen.findByText('apps/store');
  expect(saved).toHaveBeenCalledTimes(1);
  expect(fetch.mock.calls.map(([path])=>path)).toEqual(['/api/helmcharts/apps/store/source-preview','/api/helmcharts/apps/store/source-save']);
  expect(JSON.parse(fetch.mock.calls[1][1].body)).toEqual({mode:'create',yaml,fingerprint:'reviewed'});
});

it('rechecks existing source identity and keeps the draft after a conflict', async () => {
  const fetch = vi.fn().mockResolvedValueOnce(Response.json({...id,yaml}))
    .mockResolvedValueOnce(Response.json({...plan,mode:'edit'}))
    .mockResolvedValueOnce(Response.json({error:'stale source'}, {status:409}));
  vi.stubGlobal('fetch',fetch);
  render(<DistributionCatalog view="helmcharts" result={{loading:false,data:[chart]}} sourceEditing />);
  fireEvent.click(button('编辑配置'));
  const input=await screen.findByLabelText('HelmChart YAML');
  const changed=yaml.replace('1.0.0','1.0.1');
  fireEvent.change(input,{target:{value:changed}});
  fireEvent.click(button('预览配置变更'));
  await screen.findByText('apps/fleet · 已绑定 2');
  fireEvent.click(button('确认保存配置'));
  expect(await screen.findByRole('alert')).toHaveTextContent('stale source');
  expect(screen.getByLabelText('HelmChart YAML')).toHaveValue(changed);
  expect(JSON.parse(fetch.mock.calls[2][1].body)).toEqual({mode:'edit',yaml:changed,uid:id.uid,resourceVersion:'7',fingerprint:'reviewed'});
  expect(button('重新载入集群版本')).toBeEnabled();
});

it('hides remote editing when disabled and rejects an incompatible YAML document locally', async () => {
  const fetch=vi.fn(); vi.stubGlobal('fetch',fetch);
  const {rerender}=render(<DistributionCatalog view="helmcharts" result={{loading:false,data:[chart]}}/>);
  expect(screen.queryByRole('button',{name:'编辑配置'})).toBeNull();
  rerender(<DistributionCatalog view="helmcharts" sourceEditing result={{loading:false,data:[chart]}}/>);
  fireEvent.click(button('新建 HelmChart'));
  fireEvent.change(screen.getByLabelText('HelmChart YAML'),{target:{value:'apiVersion: v1\nkind: ConfigMap'}});
  fireEvent.click(button('预览配置变更'));
  expect(await screen.findByRole('alert')).toHaveTextContent('请选择');
  expect(fetch).not.toHaveBeenCalled();
});

it('keeps a reviewed HelmChart single-document and previews the corrected source before saving', async () => {
  const fetch = vi.fn().mockResolvedValueOnce(Response.json({ ...plan, mode: 'create' }))
    .mockResolvedValueOnce(Response.json({ ...plan, mode: 'create', fingerprint: 'corrected' }))
    .mockResolvedValueOnce(Response.json(id, { status: 201 }));
  vi.stubGlobal('fetch', fetch);
  render(<DistributionCatalog view="helmcharts" result={{ loading: false, data: [] }} sourceEditing/>);
  fireEvent.click(button('新建 HelmChart'));
  const changeYaml = (value: string) => fireEvent.change(screen.getByLabelText('HelmChart YAML'), { target: { value } });
  changeYaml(yaml); fireEvent.click(button('预览配置变更'));
  fireEvent.click(await screen.findByRole('button', { name: '返回编辑' }));
  for (const invalid of [yaml + '\n---\n' + yaml, yaml + '\nbroken: [']) {
    changeYaml(invalid);
    expect(screen.getByLabelText('HelmChart YAML')).toHaveAttribute('aria-invalid', 'true');
    expect(button('预览配置变更')).toBeDisabled(); fireEvent.click(button('预览配置变更'));
    expect(screen.queryByRole('button', { name: '确认保存配置' })).toBeNull();
    expect(fetch).toHaveBeenCalledTimes(1);
  }
  const corrected = yaml.replace('1.0.0', '1.0.1');
  changeYaml(corrected); fireEvent.click(button('预览配置变更'));
  fireEvent.click(await screen.findByRole('button', { name: '确认保存配置' }));
  await screen.findByText('apps/store');
  expect(JSON.parse(fetch.mock.calls[2][1].body)).toMatchObject({ yaml: corrected, fingerprint: 'corrected' });
});

it('locks review during an import and retains the HelmChart draft when the imported file is invalid', async () => {
  const fetch = vi.fn(); vi.stubGlobal('fetch', fetch);
  render(<DistributionCatalog view="helmcharts" result={{ loading: false, data: [] }} sourceEditing/>);
  fireEvent.click(button('新建 HelmChart'));
  const editor = screen.getByLabelText('HelmChart YAML') as HTMLTextAreaElement;
  const original = editor.value;
  let finishRead!: (content: string) => void;
  const file = { size: 8, text: () => new Promise<string>(resolve => { finishRead = resolve; }) };
  fireEvent.change(screen.getByLabelText('导入 HelmChart YAML 文件'), { target: { files: [file] } });
  expect(editor).toBeDisabled(); expect(button('预览配置变更')).toBeDisabled();
  fireEvent.click(button('预览配置变更'));
  finishRead('spec: [');
  expect(await screen.findByRole('alert')).toHaveTextContent(/第 \d+ 行，第 \d+ 列/);
  expect(editor).toHaveValue(original); expect(button('预览配置变更')).toBeEnabled();
  fireEvent.change(editor, { target: { value: yaml + '\n#' + 'x'.repeat(16 * 1024) } });
  await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('16 KiB'));
  expect(button('预览配置变更')).toBeDisabled(); expect(fetch).not.toHaveBeenCalled();
});
