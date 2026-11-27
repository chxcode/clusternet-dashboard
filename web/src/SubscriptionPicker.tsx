import type { Subscription } from './api';
import './propagation.css';

type SubscriptionRef = Pick<Subscription, 'name' | 'namespace'>;

type SubscriptionPickerProps = {
  subscriptions: Subscription[];
  loading: boolean;
  error?: string;
  selectedSubscription?: SubscriptionRef;
  onSelect: (subscription: Subscription) => void;
};

function objectKey(object: SubscriptionRef) {
  return `${object.namespace || 'default'}/${object.name}`;
}

export function SubscriptionPicker({ subscriptions, loading, error, selectedSubscription, onSelect }: SubscriptionPickerProps) {
  const selectedKey = selectedSubscription ? objectKey(selectedSubscription) : '';
  const listed = subscriptions.some(item => objectKey(item) === selectedKey);
  const noOptions = subscriptions.length === 0;
  let placeholder = '选择一个订阅';
  if (noOptions) placeholder = loading ? '正在加载订阅…' : error ? '订阅列表读取失败' : '当前没有订阅';
  return (
    <section className="pd-picker" aria-label="订阅选择">
      <label htmlFor="propagation-subscription">订阅</label>
      <select id="propagation-subscription" value={selectedKey} disabled={noOptions}
        onChange={event => {
          const selected = subscriptions.find(item => objectKey(item) === event.target.value);
          if (selected) onSelect(selected);
        }}>
        <option value="" disabled>{placeholder}</option>
        {selectedKey && !listed && <option value={selectedKey}>{selectedKey}</option>}
        {subscriptions.map(item => <option key={objectKey(item)} value={objectKey(item)}>{objectKey(item)}</option>)}
      </select>
      {error && <span className="pd-warning" role="alert">订阅列表未更新 · {error}</span>}
      {loading && !noOptions && <span role="status">正在刷新…</span>}
    </section>
  );
}
