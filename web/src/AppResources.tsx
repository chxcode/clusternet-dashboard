import { useState } from 'react';
import { FilePenLine, FilePlus2 } from 'lucide-react';
import { isKind, sourceFromManifest, type Origin } from './nativeWorkload';
import { WorkloadPublishDialog } from './WorkloadPublishDialog';
import type { Manifest, Subscription } from './api';
import { WorkloadDraftEditor } from './WorkloadDraftEditor';
import { pathForPage } from './routing';
import { currentBindingCount, SubscriptionPolicy } from './SubscriptionPolicy';
import './releases.css';

type AppResourcesProps = {
  subscriptions: Subscription[] | undefined;
  manifests: Manifest[] | undefined;
  subscriptionError?: string;
  manifestError?: string;
  nativePublishing?: boolean;
  onPublished?: () => void;
};

export function AppResources({ subscriptions, manifests, subscriptionError, manifestError, nativePublishing, onPublished }: AppResourcesProps) {
  const [origin, setOrigin] = useState<Origin>();
  return (
    <div className="inventory-stack">
      <section className="inventory-panel" aria-labelledby="subscription-list-title">
        <header className="inventory-heading">
          <h2 id="subscription-list-title">订阅</h2>
          {subscriptions && <span>{subscriptions.length} 个订阅</span>}
        </header>
        {subscriptionError && <p className="inventory-warning" role="alert">订阅读取失败 · {subscriptionError}</p>}
        {!subscriptions && !subscriptionError && <p className="inventory-empty" role="status">正在加载订阅…</p>}
        {subscriptions?.length === 0 && <p className="inventory-empty">当前没有订阅。</p>}
        {!!subscriptions?.length && (
          <div className="inventory-table-scroll sub-list-scroll">
            <table className="inventory-table sub-inventory" aria-label="订阅列表">
              <thead><tr><th scope="col">订阅 / 命名空间</th><th scope="col">分发方式与目标选择</th><th scope="col">绑定与发布进度</th></tr></thead>
              <tbody>
                {subscriptions.map((sub) => {
                  const namespace = sub.namespace || 'default';
                  const detail = new URLSearchParams({ namespace, name: sub.name });
                  const bindings = currentBindingCount(sub);
                  return (
                    <tr key={namespace + '/' + sub.name}>
                      <td>
                        <a className="inventory-link" href={pathForPage('propagation-status') + '?' + detail}>{sub.name}</a>
                        <small>{namespace}</small>
                        <details className="inventory-config">
                          <summary>订阅配置</summary>
                          <dl>
                            <dt>调度器</dt><dd>{sub.schedulerName || '未提供'}</dd>
                            <dt>订阅者</dt><dd>{sub.subscriberCount ?? '未知'}</dd>
                            <dt>Feed 数</dt><dd>{sub.feedCount ?? '未知'}</dd>
                            {sub.feedKinds?.length ? <><dt>Feed 类型</dt><dd>{sub.feedKinds.join('、')}</dd></> : null}
                          </dl>
                        </details>
                        {nativePublishing && !!sub.feedKinds?.length && sub.feedKinds.every(isKind) &&
                          <button type="button" className="label-edit" onClick={() => setOrigin({ type: 'subscription', id: { namespace, name: sub.name } })}><FilePenLine size={14}/>编辑原生发布</button>}
                      </td>
                      <td><SubscriptionPolicy subscription={sub} /></td>
                      <td><dl className="sub-progress">
                        <div><dt>当前已绑定</dt><dd>{bindings === undefined ? '未上报' : bindings + ' 个集群'}</dd></div>
                        <div><dt>已回报 / 期望</dt><dd>{sub.observedReleases} / {sub.desiredReleases}</dd></div>
                        <div><dt>已确认完成</dt><dd>{sub.completionKnown ? sub.completedReleases + ' / ' + sub.desiredReleases : '完成数未知'}</dd></div>
                      </dl>{sub.completionKnown && <small>{sub.status}</small>}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>
      <section className="inventory-panel" aria-labelledby="manifest-list-title">
        <header className="inventory-heading">
          <h2 id="manifest-list-title">Manifest 清单</h2>
          <div className="native-actions"><WorkloadDraftEditor onPreparePublish={nativePublishing ? yaml => setOrigin({ type: 'new', yaml }) : undefined}/>
            {nativePublishing && <button type="button" className="label-edit" onClick={() => setOrigin({ type: 'new' })}><FilePlus2 size={14}/>新建原生发布</button>}</div>
          {manifests && <span>{manifests.length} 个清单</span>}
        </header>
        {manifestError && <p className="inventory-warning" role="alert">清单读取失败 · {manifestError}</p>}
        {!manifests && !manifestError && <p className="inventory-empty" role="status">正在加载清单…</p>}
        {manifests?.length === 0 && <p className="inventory-empty">当前没有 Manifest 清单。</p>}
        {!!manifests?.length && (
          <div className="inventory-table-scroll">
            <table className="inventory-table" aria-label="Manifest 清单列表">
              <thead><tr><th>名称 / 命名空间</th><th>模板资源</th><th>模板命名空间</th><th>API 版本</th></tr></thead>
              <tbody>
                {manifests.map((item) => {
                  const source = nativePublishing && sourceFromManifest(item);
                  return (
                    <tr key={item.namespace + '/' + item.name}>
                      <td><strong>{item.name}</strong><small>{item.namespace || 'default'}</small></td>
                      <td>{item.templateKind || '—'} / {item.templateName || '—'}
                        {source && <div><button type="button" className="label-edit" onClick={() => setOrigin({ type: 'source', id: source })}><FilePenLine size={14}/>编辑工作负载</button></div>}</td>
                      <td>{item.templateNamespace || '—'}</td><td>{item.templateAPIVersion || '—'}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>
      {nativePublishing && origin && <WorkloadPublishDialog origin={origin} onClose={() => setOrigin(undefined)} onPublished={onPublished}/>}
    </div>
  );
}
