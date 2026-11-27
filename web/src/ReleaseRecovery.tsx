import type { HelmRelease } from './api';
import { pathForPage } from './routing';

export function ReleaseRecovery({ release }: { release: HelmRelease }) {
  const { subscriptionName, subscriptionNamespace } = release;
  const subscription = subscriptionName && subscriptionNamespace
    ? pathForPage('propagation-status') + '?' + new URLSearchParams({ namespace: subscriptionNamespace, name: subscriptionName })
    : pathForPage('subscriptions');
  return <section className="release-evidence" aria-label="恢复发布">
    <h4>恢复发布</h4>
    <p>先核对控制器回报和发布事件，再从订阅的 Feed 找到源 HelmChart。若版本有误，可在 HelmChart 列表预览影响并修正版本。</p>
    {release.atomic === true && <p>此发布已启用 atomic。重新协调时，如果 Helm 上一次发布未处于 deployed 状态，控制器可能先卸载再安装。</p>}
    {release.upgradeAtomic !== undefined && <p>升级失败时自动回滚（upgradeAtomic） · {release.upgradeAtomic ? '已开启' : '已关闭'}</p>}
    <a className="inventory-link" href={subscription}>{subscriptionName && subscriptionNamespace ? '查看该订阅的 Feed' : '查找订阅'}</a>
    {' · '}<a className="inventory-link" href={pathForPage('subscriptions') + '?view=helmcharts'}>打开 HelmChart 列表</a>
  </section>;
}
