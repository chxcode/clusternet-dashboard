import { act, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import type { Registration } from './api';
import { RegistrationCatalog } from './ResourceCatalog';

const old: Registration = { name:'clusternet-edge', clusterID:'one', uid:'old-uid', resourceVersion:'7',
  requestedNamespace:'busy', namespaceEvidence:'occupied', deletionMode:'pendingNamespace' };
function row(target: string) { return screen.getByRole('rowheader', {name:target}).closest('tr')!; }
function deleteTrigger(target: string) { return within(row(target)).getByRole('button', {name:'删除请求'}); }
afterEach(() => vi.unstubAllGlobals());

it('filters explicitly requested namespaces and outcomes independently', () => {
  render(<RegistrationCatalog result={{data:[old,{name:'approved',requestedNamespace:'busy',result:'Approved'},
    {name:'another',requestedNamespace:'other',result:'Failed'},{name:'auto'}],loading:false}} deletionEnabled={false} />);
  fireEvent.change(screen.getByLabelText('申请命名空间'),{target:{value:'busy'}});
  expect(screen.queryByRole('rowheader',{name:'another'})).toBeNull();
  fireEvent.change(screen.getByLabelText('审批结果'),{target:{value:'unreported'}});
  expect(screen.getByText('申请名称已被占用')).toBeVisible();
  expect(screen.queryByRole('rowheader',{name:'approved'})).toBeNull();
  expect(screen.getByText('删除未启用')).toBeVisible();
  expect(screen.queryByRole('button',{name:'删除请求'})).toBeNull();
});

it('requires explicit current identity and tracks disappearance versus a same-cluster replacement',async () => {
  const call = vi.fn().mockResolvedValue(new Response(JSON.stringify({status:'deletionRequested'}),{status:202}));
  vi.stubGlobal('fetch',call);
  const refresh=vi.fn();
  const page=(data:Registration[],error?:string)=><RegistrationCatalog result={{data,loading:false,error}} deletionEnabled onRefresh={refresh}/>;
  const ui=render(page([old]));
  fireEvent.click(deleteTrigger(old.name));
  const input=screen.getByLabelText('输入完整请求名称以确认');
  const confirm=screen.getByRole('button',{name:'确认删除'});
  fireEvent.change(input,{target:{value:'wrong'}});expect(confirm).toBeDisabled();
  expect(screen.getByText(/正在等待审批的 agent/)).toBeVisible();
  fireEvent.change(input,{target:{value:old.name}});
  await act(async()=>fireEvent.click(confirm));
  expect(call).toHaveBeenCalledOnce();
  expect(call.mock.calls[0][0]).toMatch(/\/api\/registrations\/clusternet-edge$/);
  expect(call.mock.calls[0][1].method).toBe('DELETE');
  expect(JSON.parse(call.mock.calls[0][1].body)).toEqual({uid:'old-uid',resourceVersion:'7',confirmName:old.name});
  expect(refresh).toHaveBeenCalledOnce();
  ui.rerender(page([], 'temporary failure'));expect(screen.getByText(/最新查询失败/)).toBeVisible();
  expect(screen.queryByText(/旧请求已消失/)).toBeNull();
  fireEvent.click(screen.getByRole('button',{name:'刷新核对'}));
  expect(refresh).toHaveBeenCalledTimes(2);
  ui.rerender(page([]));expect(screen.getByText(/旧请求已消失/)).toBeVisible();
  ui.rerender(page([{...old,uid:'wrong-cluster',clusterID:'another'}]));expect(screen.getByText(/身份已变化/)).toBeVisible();
  ui.rerender(page([{...old,uid:'new-uid',result:'Approved',deletionMode:undefined}]));expect(screen.getByText(/同一集群 ID 的同名新请求/)).toHaveTextContent('new-uid，审批结果 Approved');
  expect(call).toHaveBeenCalledOnce();
});

it('drops typed confirmation on a resource version change and hides stale actions',()=>{
  const page=(data:Registration[],error?:string)=><RegistrationCatalog result={{data,loading:false,error}} deletionEnabled/>;
  const ui=render(page([{...old,result:'Denied',deletionMode:'denied'}]));
  fireEvent.click(deleteTrigger(old.name));
  fireEvent.change(screen.getByLabelText('输入完整请求名称以确认'),{target:{value:old.name}});
  ui.rerender(page([{...old,resourceVersion:'8'}]));
  expect(screen.queryByRole('textbox')).toBeNull();
  ui.rerender(page([old],'stale'));expect(screen.queryByRole('button',{name:'删除请求'})).toBeNull();
});

it('opens above the table, copies the full name and returns focus after cancel', async () => {
  const record: Registration = {...old,result:'Failed',deletionMode:'failed'};
  render(<RegistrationCatalog result={{data:[record],loading:false}} deletionEnabled/>);
  const trigger=deleteTrigger(old.name);
  fireEvent.click(trigger);
  const dialog=screen.getByRole('dialog',{name:'删除失败请求'});
  expect(dialog.parentElement).toBe(document.body);
  const writeText=vi.fn().mockResolvedValue(undefined);
  const previous=Object.getOwnPropertyDescriptor(navigator,'clipboard');
  Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText}});
  try {
    await act(async()=>fireEvent.click(within(dialog).getByRole('button',{name:'复制完整请求名称'})));
    expect(writeText).toHaveBeenCalledWith(old.name);
  } finally {
    if(previous) Object.defineProperty(navigator,'clipboard',previous);
    else Reflect.deleteProperty(navigator,'clipboard');
  }
  for(const value of [old.name,old.uid!,'busy','Failed']) expect(within(dialog).getByText(value)).toBeVisible();
  fireEvent.change(within(dialog).getByLabelText('输入完整请求名称以确认'),{target:{value:old.name}});
  fireEvent.click(within(dialog).getByRole('button',{name:'取消'}));
  expect(screen.queryByRole('dialog')).toBeNull();
  expect(trigger).toHaveFocus();
  fireEvent.click(trigger);
  expect(screen.getByRole('button',{name:'确认删除'})).toBeDisabled();
});

it('blocks an extra submission and cancel while the DELETE response is outstanding', async()=>{
  let finish!: (r: Response) => void;
  const call=vi.fn(()=>new Promise<Response>(resolve=>{finish=resolve}));
  vi.stubGlobal('fetch',call);
  render(<RegistrationCatalog result={{data:[old],loading:false}} deletionEnabled/>);
  fireEvent.click(deleteTrigger(old.name));
  const dialog=screen.getByRole('dialog');
  fireEvent.change(within(dialog).getByLabelText('输入完整请求名称以确认'),{target:{value:old.name}});
  const form=within(dialog).getByRole('form');
  fireEvent.submit(form);fireEvent.submit(form);
  expect(call).toHaveBeenCalledOnce();
  expect(within(dialog).getByRole('button',{name:'取消'})).toBeDisabled();
  fireEvent(dialog,new Event('cancel',{cancelable:true}));expect(dialog).toBeVisible();
  await act(async()=>finish(new Response(JSON.stringify({status:'deletionRequested'}),{status:202})));
  expect(screen.queryByRole('dialog')).toBeNull();
});

it('puts eligible actions directly in their own rows and keeps the approved request protected', () => {
  const call = vi.fn(); vi.stubGlobal('fetch', call);
  const records: Registration[] = [
    { ...old, name: 'request-failed', uid: 'failed-uid', requestedNamespace: 'failed-space', result: 'Failed', deletionMode: 'failed' },
    { ...old, name: 'request-approved', uid: 'approved-uid', requestedNamespace: 'approved-space', result: 'Approved', deletionMode: undefined },
    { ...old, name: 'request-denied', uid: 'denied-uid', requestedNamespace: 'denied-space', result: 'Denied', deletionMode: 'denied' },
    old,
  ];
  render(<RegistrationCatalog result={{ data: records, loading: false }} deletionEnabled/>);
  expect(screen.getAllByRole('button',{ name: '删除请求' })).toHaveLength(3);
  const approved = within(row('request-approved'));
  expect(approved.getByText('已保护')).toBeVisible();
  expect(approved.queryByRole('button', { name: '删除请求' })).toBeNull();
  const titles = { failed: '删除失败请求', denied: '删除已拒绝请求', pendingNamespace: '删除未批准请求' };
  for (const record of records.filter(r => r.deletionMode)) {
    const disclosure = within(row(record.name)).getByRole('button', { name: /详情/ });
    expect(disclosure).toHaveAttribute('aria-expanded', 'false');
    const trigger = deleteTrigger(record.name);
    expect(trigger).toBeVisible();
    expect(trigger.closest('details')).toBeNull();
    fireEvent.click(trigger);
    const dialog = screen.getByRole('dialog', { name: titles[record.deletionMode!] });
    for (const value of [record.name, record.uid!, record.requestedNamespace!]) {
      expect(within(dialog).getByText(value)).toBeVisible();
    }
    fireEvent.change(within(dialog).getByLabelText('输入完整请求名称以确认'), {target:{value:'request-approved'}});
    expect(within(dialog).getByRole('button', {name:'确认删除'})).toBeDisabled();
    fireEvent.click(within(dialog).getByRole('button', {name:'取消'}));
    expect(disclosure).toHaveAttribute('aria-expanded', 'false');
  }
  const details = approved.getByRole('button', { name: /详情/ });
  fireEvent.click(details); expect(details).toHaveAttribute('aria-expanded', 'true');
  const detailRow = row('request-approved').nextElementSibling as HTMLElement;
  expect(detailRow.tagName).toBe('TR'); expect(within(detailRow).getByText('approved-uid')).toBeVisible();
  expect(within(detailRow).queryByRole('button', {name: '删除请求'})).toBeNull();
  fireEvent.click(details); expect(details).toHaveAttribute('aria-expanded', 'false');
  expect(call).not.toHaveBeenCalled();
});

it('lets the operator re-read a rejected request without issuing a second delete', async () => {
  const call = vi.fn().mockResolvedValue(new Response('{}', {status: 409})); vi.stubGlobal('fetch', call);
  const refresh = vi.fn();
  render(<RegistrationCatalog result={{ data: [old], loading: false }} deletionEnabled onRefresh={refresh}/>);
  fireEvent.click(deleteTrigger(old.name));
  const dialog = screen.getByRole('dialog', { name: '删除未批准请求' });
  fireEvent.change(within(dialog).getByLabelText('输入完整请求名称以确认'), {target:{value:old.name}});
  fireEvent.click(within(dialog).getByRole('button', {name:'确认删除'}));
  expect(await within(dialog).findByRole('alert')).toHaveTextContent(/已变化/);
  expect(within(dialog).getByRole('button', {name:'确认删除'})).toBeDisabled();
  fireEvent.submit(within(dialog).getByRole('form'));
  expect(call).toHaveBeenCalledOnce();
  fireEvent.click(within(dialog).getByRole('button', {name:'刷新列表'}));
  expect(refresh).toHaveBeenCalledOnce(); expect(call).toHaveBeenCalledOnce();
});
