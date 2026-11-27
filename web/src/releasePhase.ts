export type ReleasePhaseGroup = 'failed' | 'pending' | 'deployed' | 'unknown' | 'other';

export function releasePhaseGroup(phase?: string): ReleasePhaseGroup {
  const value = phase?.trim().toLowerCase();
  if (!value || value === 'unknown') return 'unknown';
  if (value === 'failed' || value === 'deployed') return value;
  if (value === 'pending' || value.startsWith('pending-')) return 'pending';
  return 'other';
}

const phaseLabels: Record<string, string> = {
  failed: '失败',
  deployed: '已部署',
  pending: '进行中',
  'pending-install': '等待安装',
  'pending-upgrade': '等待升级',
  'pending-rollback': '等待回滚',
  unknown: '未知',
  uninstalled: '已卸载',
  uninstalling: '正在卸载',
  superseded: '已被替代',
};

export function releasePhaseLabel(phase?: string): string {
  const value = phase?.trim().toLowerCase() || 'unknown';
  return phaseLabels[value] ?? value;
}
