import type { PropagationFilter } from './propagationFiltering';

type PropagationFilterControlsProps = {
  value: PropagationFilter;
  onChange: (value: PropagationFilter) => void;
};

export function PropagationFilterControls({ value, onChange }: PropagationFilterControlsProps) {
  return (
    <div className="pd-filter">
      <label htmlFor="target-status">按分发状态筛选</label>
      <select id="target-status" value={value} onChange={event => onChange(event.target.value as PropagationFilter)}>
        <option value="all">全部目标集群</option>
        <option value="observed">已回报</option>
        <option value="not-observed">未回报</option>
        <option value="online-not-observed">在线但未回报</option>
        <option value="offline">集群未就绪</option>
        <option value="unknown">集群连接状态未知</option>
      </select>
    </div>
  );
}
