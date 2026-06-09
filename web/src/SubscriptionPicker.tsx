import type { Subscription } from './api';
import { SubscriptionSummaryMeta } from './SubscriptionSummary';

type SubscriptionPickerProps = {
  subscriptions: Subscription[];
  loading: boolean;
  error?: string;
  selectedSubscription?: Subscription;
  onSelect: (subscription: Subscription) => void;
};

export function SubscriptionPicker({ subscriptions, loading, error, selectedSubscription, onSelect }: SubscriptionPickerProps) {
  if (loading) {
    return <section className="detail-panel">正在加载 Subscriptions...</section>;
  }

  if (error) {
    return <section className="detail-panel detail-panel--error">{error}</section>;
  }

  return (
    <section className="detail-panel" aria-label="Subscription selector">
      <div className="section-heading">
        <p className="eyebrow">Propagation Status</p>
        <h2>选择 Subscription</h2>
      </div>
      {subscriptions.length === 0 ? (
        <p className="empty-state">当前没有发现 Subscriptions</p>
      ) : (
        <div className="resource-list">
          {subscriptions.map((subscription) => {
            const selected = selectedSubscription?.name === subscription.name && (selectedSubscription.namespace || 'default') === (subscription.namespace || 'default');
            return (
              <button
                type="button"
                className={selected ? 'resource-row resource-row--button resource-row--selected' : 'resource-row resource-row--button'}
                key={`${subscription.namespace}/${subscription.name}`}
                onClick={() => onSelect(subscription)}
              >
                <div>
                  <strong>{subscription.name}</strong>
                  <span>{subscription.namespace || 'default'}</span>
                </div>
                <div>
                  <strong>{subscription.status}</strong>
                  <span>{subscription.bindingClusterCount} clusters</span>
                </div>
                <SubscriptionSummaryMeta subscription={subscription} />
              </button>
            );
          })}
        </div>
      )}
    </section>
  );
}
