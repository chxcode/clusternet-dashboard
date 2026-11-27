import { render, screen, fireEvent, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { DistributionCatalog, RegistrationCatalog, distributionView } from './ResourceCatalog';
import type { Registration } from './api';

describe('resource catalogs', () => {
  it('paginates registration history, filters across pages, and distinguishes unreported results', () => {
    const records: Registration[] = Array.from({length: 26}, (_, i) => ({ name: 'request-' + i, clusterID:'id-' + i, result:i===12 ? undefined : 'Approved' }));
    render(<RegistrationCatalog result={{ data:records, loading:false }} />);
    expect(within(screen.getByRole('table')).getAllByRole('row')).toHaveLength(21);
    fireEvent.click(screen.getByRole('button',{name:'下一页'}));
    expect(within(screen.getByRole('table')).getAllByRole('row')).toHaveLength(7);
    fireEvent.change(screen.getByRole('combobox'),{target:{value:'unreported'}});
    expect(within(screen.getByRole('table')).getAllByRole('row')).toHaveLength(2);
    expect(screen.getByText('request-12')).toBeVisible();
    fireEvent.change(screen.getByRole('searchbox'),{target:{value:'missing'}});
    expect(screen.getByText('没有匹配当前条件的资源。')).toBeVisible();
  });

  it('exposes scoped override rules and their targets', () => {
    render(<DistributionCatalog view="globalizations" result={{loading:false,data:[{
      name:'app-settings',kind:'Globalization',overrideCount:1,policy:'ApplyLater',priority:0,
      clusterSelector:'region=west',feed:{kind:'HelmChart',namespace:'apps',name:'web'},rules:[{name:'set-replicas',type:'MergePatch'}],
    }]}} />);
    expect(screen.getByText('region=west')).toBeVisible();
    expect(screen.getByText('HelmChart / apps / web')).toBeVisible();
    fireEvent.click(screen.getByText('详情',{selector:'summary'}));
    expect(screen.getByText('set-replicas')).toBeVisible();
    expect(distributionView('toString')).toBeUndefined();
  });
});
