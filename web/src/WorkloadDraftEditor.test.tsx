import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { AppResources } from './AppResources';
import { WorkloadDraftEditor } from './WorkloadDraftEditor';

const button = (name: string) => screen.getByRole('button', { name });
const source = () => screen.getByRole('textbox', { name: 'YAML 源文件' }) as HTMLTextAreaElement;
const edit = (value: string) => fireEvent.change(source(), { target: { value } });
function open() { fireEvent.click(button('编辑本地 YAML')); }
function read(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader(); reader.onload = () => resolve(String(reader.result)); reader.onerror = reject; reader.readAsText(blob);
  });
}
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

it('opens from the Manifest catalog, builds all three templates, and keeps a draft when the editor closes', () => {
  const fetch = vi.fn(); vi.stubGlobal('fetch', fetch);
  const write = vi.spyOn(Storage.prototype, 'setItem');
  render(<AppResources subscriptions={[]} manifests={[]} />);
  open();
  expect(button('下载 YAML')).toBeDisabled();
  for (const kind of ['Deployment', 'Service', 'ConfigMap']) {
    fireEvent.change(screen.getByRole('combobox', { name: '模板' }), { target: { value: kind } });
    fireEvent.click(button('添加模板'));
  }
  expect(source().value.split('---\n')).toHaveLength(3);
  fireEvent.click(button('校验 YAML'));
  expect(screen.getByRole('status')).toHaveTextContent('本地基础校验通过 · 3 个资源');
  for (const name of ['Deployment · web', 'Service · web', 'ConfigMap · web-config']) expect(screen.getByText(name)).toBeVisible();
  expect(screen.getByText('kubectl clusternet apply -f workloads.yaml')).toBeVisible();
  const draft = source().value;
  fireEvent.click(button('新建空白文件')); fireEvent.click(button('保留草稿'));
  fireEvent.click(button('关闭编辑器')); open();
  expect(source().value).toBe(draft);
  fireEvent.click(button('新建空白文件')); fireEvent.click(button('确认新建'));
  expect(source().value).toBe('');
  expect(fetch).not.toHaveBeenCalled(); expect(write).not.toHaveBeenCalled();
});

it('reads a local multi-document file and downloads the exact edited source, including comments and separators', async () => {
  const fetch = vi.fn(); vi.stubGlobal('fetch', fetch);
  const createURL = vi.fn((_blob: Blob) => 'blob:test-workload');
  vi.stubGlobal('URL', class extends URL { static createObjectURL = createURL; static revokeObjectURL = vi.fn(); });
  const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function(this: HTMLAnchorElement) {
    expect(this.download).toBe('workloads.yaml'); expect(this.href).toBe('blob:test-workload');
  });
  render(<WorkloadDraftEditor />); open();
  const yaml = '# team config\napiVersion: v1\nkind: ConfigMap\nmetadata: {name: a}\ndata:\n  flags: |-\n    one\n    two\n---\n# route\napiVersion: v1\nkind: Service\nmetadata: {name: dns}\nspec: {type: ExternalName, externalName: example.test}\n';
  fireEvent.change(screen.getByLabelText('导入 YAML 文件'), { target: { files: [new File([yaml], 'project.yml', { type: 'application/yaml' })] } });
  expect(await screen.findByText('已导入 project.yml')).toBeVisible();
  expect(source().value).toBe(yaml);
  const edited = yaml.replace('one','three'); edit(edited);
  fireEvent.click(button('下载 YAML'));
  expect(screen.getByText(/本地基础校验通过 · 2 个资源/)).toBeVisible();
  expect(click).toHaveBeenCalledOnce(); expect(await read(createURL.mock.calls[0][0])).toBe(edited);
  expect(fetch).not.toHaveBeenCalled();
});

it('reports the offending document, clears stale validation on edit, and preserves valid multi-document YAML', () => {
  render(<WorkloadDraftEditor />); open();
  const config = 'apiVersion: v1\nkind: ConfigMap\nmetadata: {name: config}\n';
  edit(config + '---\n' + config + '---\nkind: Secret\n---\ninvalid: [\n');
  fireEvent.click(button('校验 YAML'));
  const errors = screen.getByRole('alert');
  expect(errors).toHaveTextContent('文档 2 · 与前面的文档定义了同一个资源');
  expect(errors).toHaveTextContent('文档 3 · 支持的资源类型'); expect(errors).toHaveTextContent('文档 4');
  edit(config + "data: {number: '12'}\n---\n# trailing comment\n");
  expect(screen.queryByRole('alert')).toBeNull(); fireEvent.click(button('校验 YAML'));
  expect(screen.getByRole('status')).toHaveTextContent('1 个资源');
});

it('checks deployment selector agreement, service ports and ConfigMap field types', () => {
  render(<WorkloadDraftEditor />); open();
  fireEvent.click(button('添加模板'));
  const original = source().value;
  edit(original.replace('app: web', 'app: missing'));
  fireEvent.click(button('校验 YAML')); expect(screen.getByRole('alert')).toHaveTextContent('与 Pod 模板标签不匹配');
  edit(original.replace('matchLabels:\n      app: web', "matchExpressions:\n      - {key: location, operator: NotIn, values: [west]}"));
  fireEvent.click(button('校验 YAML')); expect(screen.getByRole('status')).toHaveTextContent('1 个资源');
  edit('apiVersion: v1\nkind: Service\nmetadata: {name: svc}\nspec: {ports: [{port: 65536}]}\n---\napiVersion: v1\nkind: ConfigMap\nmetadata: {name: config}\ndata: {count: 12}\n');
  fireEvent.click(button('校验 YAML'));
  expect(screen.getByRole('alert')).toHaveTextContent('文档 1 · Service 的 port');
  expect(screen.getByRole('alert')).toHaveTextContent('文档 2 · data 和 binaryData 的值应为字符串');
});

function captureDownloads() {
  const blobs: Blob[] = [];
  vi.stubGlobal('URL', class extends URL {
    static createObjectURL = (blob: Blob) => { blobs.push(blob); return 'blob:offline-draft'; };
    static revokeObjectURL = vi.fn();
  });
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
  return blobs;
}

it('keeps invalid drafts editable and downloads their exact bytes locally', async () => {
  const fetch = vi.fn(), prepare = vi.fn(); vi.stubGlobal('fetch', fetch);
  const blobs = captureDownloads();
  render(<WorkloadDraftEditor onPreparePublish={prepare}/>); open();
  const good = 'apiVersion: v1\nkind: ConfigMap\nmetadata: {name: draft}\n';
  const draft = good + 'data: [';
  edit(draft);
  expect(button('用此草稿准备发布')).toBeDisabled(); fireEvent.click(button('用此草稿准备发布'));
  await waitFor(() => expect(source()).toHaveAttribute('aria-invalid', 'true'));
  fireEvent.click(button('下载 YAML'));
  expect(await read(blobs[0])).toBe(draft);
  fireEvent.click(button('关闭编辑器')); open(); expect(source().value).toBe(draft);
  expect(prepare).not.toHaveBeenCalled(); expect(fetch).not.toHaveBeenCalled();
  edit(good); fireEvent.click(button('用此草稿准备发布'));
  expect(prepare).toHaveBeenCalledWith(good);
});

it('fully validates a ConfigMap above 256 KiB offline, downloads the original and blocks online publication', async () => {
  const fetch = vi.fn(), prepare = vi.fn(); vi.stubGlobal('fetch', fetch);
  const blobs = captureDownloads();
  render(<WorkloadDraftEditor onPreparePublish={prepare}/>); open();
  const draft = 'apiVersion: v1\nkind: ConfigMap\nmetadata: {name: large-config}\ndata:\n  payload: ' + 'x'.repeat(256 * 1024) + '\n';
  edit(draft); fireEvent.click(button('校验 YAML'));
  expect(screen.getByText('本地基础校验通过 · 1 个资源')).toBeVisible();
  expect(screen.getByText('ConfigMap · large-config')).toBeVisible();
  fireEvent.click(button('下载 YAML'));
  expect(await read(blobs[0])).toBe(draft);
  expect(screen.getByText('已下载 workloads.yaml。')).toBeVisible();
  expect(await screen.findByText(/完整检查本地草稿/)).toBeVisible();
  expect(source()).not.toHaveAttribute('aria-invalid', 'true');
  expect(button('用此草稿准备发布')).toBeDisabled(); fireEvent.click(button('用此草稿准备发布'));
  expect(prepare).not.toHaveBeenCalled(); expect(fetch).not.toHaveBeenCalled();
});
