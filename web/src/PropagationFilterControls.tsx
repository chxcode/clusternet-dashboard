import type { PropagationFilter } from './propagationFiltering';

type PropagationFilterControlsProps = {
  value: PropagationFilter;
  onChange: (value: PropagationFilter) => void;
};

export function PropagationFilterControls({ value, onChange }: PropagationFilterControlsProps) {
  return (
    <div className="filters filters--single" aria-label="propagation filters">
      <label>
        按分发状态筛选
        <select aria-label="按分发状态筛选" value={value} onChange={(event) => onChange(event.target.value as PropagationFilter)}>
          <option value="all">全部目标集群</option>
          <option value="observed">Observed</option>
          <option value="not-observed">Not observed</option>
          <option value="online-not-observed">Online but not observed</option>
          <option value="offline">Offline</option>
        </select>
      </label>
    </div>
  );
}
