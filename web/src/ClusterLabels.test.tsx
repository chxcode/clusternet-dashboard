import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ClusterLabels } from './ClusterLabels';
import { CopyButton } from './CopyButton';

const cluster = { name: 'child-east', namespace: 'sample', uid:'object-id', status:'Online', online:true, labels:{ app:'old', zone:'' } };
afterEach(() => vi.unstubAllGlobals());

describe('cluster label management', () => {
  it('sends the original values and UID for a combined add, modify and remove, preserving empty values', async () => {
    const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({labels:{app:'updated',team:''}})));
    vi.stubGlobal('fetch',fetch);
    const refresh = vi.fn();
    render(<ClusterLabels cluster={cluster} editable onSaved={refresh} />);
    fireEvent.click(screen.getByRole('button',{name:'编辑标签'}));
    const dialog = within(screen.getByRole('dialog',{name:'编辑集群标签'}));
    expect(dialog.getByText(/重新匹配目标集群/)).toBeInTheDocument();
    fireEvent.change(dialog.getByRole('textbox',{name:'第 1 个标签值'}),{target:{value:'updated'}});
    fireEvent.click(dialog.getByRole('button',{name:'移除标签 zone'}));
    fireEvent.click(dialog.getByRole('button',{name:'新增标签'}));
    fireEvent.change(dialog.getByRole('textbox',{name:'第 2 个标签键'}),{target:{value:'team'}});
    expect(fetch).not.toHaveBeenCalled();
    fireEvent.click(dialog.getByRole('button',{name:'保存修改'}));
    expect(await screen.findByText('标签已保存')).toBeInTheDocument();
    expect(screen.getByText('app=updated')).toBeInTheDocument();
    expect(refresh).toHaveBeenCalledOnce();
    const [url,options] = fetch.mock.calls[0];
    expect(url).toMatch(/\/api\/clusters\/sample\/child-east\/labels$/);
    expect(options.method).toBe('PATCH');
    expect(JSON.parse(options.body)).toEqual({uid:'object-id',changes:[
      {key:'app',before:'old',after:'updated'},{key:'zone',before:'',after:null},{key:'team',before:null,after:''},
    ]});
  });

  it('recovers from a concurrent edit without retrying stale changes', async () => {
    const fetch=vi.fn().mockResolvedValue(new Response('',{status:409}));
    vi.stubGlobal('fetch',fetch);
    const refresh = vi.fn();
    render(<ClusterLabels cluster={cluster} editable onSaved={refresh}/>);
    fireEvent.click(screen.getByRole('button',{name:'编辑标签'}));
    fireEvent.change(screen.getByRole('textbox',{name:'第 1 个标签值'}),{target:{value:'other'}});
    fireEvent.click(screen.getByRole('button',{name:'保存修改'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('已变更');
    fireEvent.click(screen.getByRole('button',{name:'关闭并重新载入'}));
    expect(refresh).toHaveBeenCalledOnce();
    expect(fetch).toHaveBeenCalledOnce();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('requires explicit save and hides the editor in read-only mode', () => {
    const fetch = vi.fn(); vi.stubGlobal('fetch',fetch);
    const {rerender} = render(<ClusterLabels cluster={cluster} editable={false} />);
    expect(screen.queryByRole('button',{name:'编辑标签'})).not.toBeInTheDocument();
    rerender(<ClusterLabels cluster={cluster} editable />);
    fireEvent.click(screen.getByRole('button',{name:'编辑标签'}));
    fireEvent.click(screen.getByRole('button',{name:'新增标签'}));
    fireEvent.change(screen.getByRole('textbox',{name:'第 3 个标签键'}),{target:{value:'app'}});
    expect(screen.getByRole('alert')).toHaveTextContent('只能出现一次');
    expect(screen.getByRole('button',{name:'保存修改'})).toBeDisabled();
    fireEvent.click(screen.getByRole('button',{name:'取消'}));
    expect(fetch).not.toHaveBeenCalled();
  });

  it('copies exactly the displayed ID', async () => {
    const writeText=vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('navigator',{clipboard:{writeText}});
    render(<CopyButton value="child-east" label="复制集群 ID child-east" />);
    fireEvent.click(screen.getByRole('button',{name:'复制集群 ID child-east'}));
    expect(await screen.findByText('已复制')).toBeInTheDocument();
    expect(writeText).toHaveBeenCalledWith('child-east');
  });
});
