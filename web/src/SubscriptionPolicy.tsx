import type { Subscription, SubscriptionDistribution } from './api';
import './subscriptionPolicy.css';

const targetLabels: Record<SubscriptionDistribution['targetScope'], string> = {
  all: '不限集群标签', selector: '按集群标签筛选', none: '无可匹配的标签选择器', unknown: '标签范围未知',
};

function strategyLabel(strategy?: string) {
  if (strategy === 'Replication') return '全量复制';
  if (strategy === 'Dividing') return '副本分摊';
  return strategy ? '未识别的分发策略' : '分发策略未知';
}

export function currentBindingCount(subscription: Subscription): number | undefined {
  return Array.isArray(subscription.bindingClusters) ? subscription.bindingClusterCount ?? subscription.bindingClusters.length : undefined;
}

export function SubscriptionPolicy({ subscription, previousSelectors, explain = false }: {
  subscription: Subscription; previousSelectors?: (string | null)[]; explain?: boolean;
}) {
  const policy = subscription.distribution;
  const strategy = policy?.strategy || subscription.schedulingStrategy;
  const selectors = policy?.selectors ?? previousSelectors ?? [];
  const scope = policy?.targetScope ?? 'unknown';
  const dividing = policy?.dividingType;

  function selectorItem(selector: string | null, index: number) {
    const min = policy?.subgroupConfigured ? policy.subgroupMinClusters?.[index] : undefined;
    const weight = dividing === 'Static' ? policy?.subscriberWeights?.[index] : undefined;
    return <li key={index}>
      <code>{selector === null ? (scope === 'unknown' ? '选择器缺失或无法解析' : '未提供选择器') : selector || '{}（空选择器，匹配任意标签）'}</code>
      {min != null && <span className="sub-min-clusters">子组 minClusters = {min}</span>}
      {weight != null && <span className="sub-min-clusters">权重 {weight}</span>}
    </li>;
  }

  return <div className="sub-policy">
    <div className="sub-strategy">
      <strong>{strategyLabel(strategy)}</strong>{strategy && <code>{strategy}</code>}
      {strategy === 'Dividing' && <span>{dividing === 'Static' ? '静态 · 按权重' : dividing === 'Dynamic' ? '动态 · 按容量' : dividing || '分摊方式未提供'}
        {dividing === 'Dynamic' && policy?.dynamicStrategy && <> · {policy.dynamicStrategy}</>}</span>}
      {policy?.subgroupConfigured && <span>按子组调度</span>}
    </div>
    {explain && strategy === 'Replication' && <p>按配置向每个被调度选中的目标集群分发一份完整应用。</p>}
    {explain && strategy === 'Dividing' && <p>调度器将可分摊工作负载的期望副本总数分配到选中的目标集群。</p>}
    <div className="sub-targets">
      <span>{targetLabels[scope] ?? targetLabels.unknown}</span>
      {selectors.length > 0 && <ul aria-label="订阅者的集群标签选择器">{selectors.slice(0, 2).map(selectorItem)}</ul>}
      {selectors.length > 2 && <details><summary>其余 {selectors.length - 2} 项选择器</summary>
        <ul>{selectors.slice(2).map((selector, index) => selectorItem(selector, index + 2))}</ul>
      </details>}
    </div>
  </div>;
}
