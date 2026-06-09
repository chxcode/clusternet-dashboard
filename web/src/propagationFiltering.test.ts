import { describe, expect, it } from 'vitest';
import { filterTargets } from './propagationFiltering';

const targets = [
  { name: 'observed-online', online: true, status: 'Online', observed: true },
  { name: 'missing-online', online: true, status: 'Online', observed: false },
  { name: 'missing-offline', online: false, status: 'Offline', observed: false },
];

describe('filterTargets', () => {
  it('filters target clusters by propagation status', () => {
    expect(filterTargets(targets, 'not-observed').map((target) => target.name)).toEqual(['missing-online', 'missing-offline']);
    expect(filterTargets(targets, 'online-not-observed').map((target) => target.name)).toEqual(['missing-online']);
    expect(filterTargets(targets, 'offline').map((target) => target.name)).toEqual(['missing-offline']);
  });
});
