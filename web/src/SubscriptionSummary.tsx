import type { Subscription } from './api';

type SubscriptionSummaryMetaProps = {
  subscription: Subscription;
};

export function SubscriptionSummaryMeta({ subscription }: SubscriptionSummaryMetaProps) {
  const feedKinds = subscription.feedKinds?.length ? ` · ${subscription.feedKinds.join(', ')}` : '';
  return (
    <div className="subscription-meta" aria-label="subscription summary">
      {subscription.schedulerName && <span>scheduler {subscription.schedulerName}</span>}
      {subscription.schedulingStrategy && <span>strategy {subscription.schedulingStrategy}</span>}
      <span>subscribers {subscription.subscriberCount ?? 0}</span>
      <span>feeds {subscription.feedCount ?? 0}{feedKinds}</span>
    </div>
  );
}
