import type { Component, Diagnostics } from './api';
import './clusters.css';
import './diagnostics.css';

type DiagnosticsPanelProps = {
  diagnostics?: Diagnostics;
  loading: boolean;
  error?: string;
};

const completionEvidence: Record<string, string> = {
  supported: '至少一个订阅已上报完成数',
  'not-observed': '当前订阅均未上报完成数，可查看目标集群的回报',
  unknown: '暂无样本，完成数支持情况未知',
};

function sectionFailure(reason: string) {
  const messages: Record<string, string> = {
    Forbidden: '当前身份缺少读取权限', Unauthorized: '集群身份验证失败',
    NotFound: '资源不存在', Timeout: '请求超时', ServiceUnavailable: '集群服务暂不可用',
  };
  return messages[reason] ?? '未能完成读取';
}

function RolloutDetails({ component }: { component: Component }) {
  const conditions = component.conditions ?? [];
  return (
    <details className="cw-rollout">
      <summary>查看副本与状态条件</summary>
      <dl className="cw-replica-counts">
        <div><dt>可用副本</dt><dd>{component.availableReplicas ?? '未上报'}</dd></div>
        <div><dt>已更新副本</dt><dd>{component.updatedReplicas ?? '未上报'}</dd></div>
        <div><dt>不可用副本</dt><dd>{component.unavailableReplicas ?? '未上报'}</dd></div>
      </dl>
      {component.generation != null && (
        <p>期望代数 {component.generation} / 已观测代数 {component.observedGeneration ?? '未上报'}</p>
      )}
      {conditions.length ? (
        <dl className="cw-deployment-conditions" aria-label="Deployment 状态条件">
          {conditions.map(condition => (
            <div key={condition.type}>
              <dt><strong>{condition.type}</strong> · {condition.status || '未上报'}</dt>
              {condition.reason && <dd><code>{condition.reason}</code></dd>}
              {condition.message && <dd>{condition.message}</dd>}
              {condition.lastTransitionTime && <dd>最近状态变化 <time dateTime={condition.lastTransitionTime}>
                {new Date(condition.lastTransitionTime).toLocaleString('zh-CN', { hour12: false })}
              </time></dd>}
            </div>
          ))}
        </dl>
      ) : <p>控制器未上报状态条件</p>}
    </details>
  );
}

export function DiagnosticsPanel({ diagnostics, loading, error }: DiagnosticsPanelProps) {
  if (!diagnostics) {
    if (error) return <section className="cw-panel cw-message cw-error" role="alert">诊断读取失败 · {error}</section>;
    return <section className="cw-panel cw-message" role="status">{loading ? '正在加载诊断信息…' : '暂无诊断信息'}</section>;
  }
  const components = diagnostics.components ?? [];
  const checks = diagnostics.readOnlyRBACChecks ?? [];
  const resources = diagnostics.resources ?? [];
  const sectionErrors = diagnostics.sectionErrors ?? {};
  return (
    <section className="cw-diagnostics" aria-label="诊断结果">
      {error && <p role="alert" className="cw-panel cw-message cw-error">刷新失败，显示上次读取的数据 · {error}</p>}
      <section className="cw-panel">
        <header className="cw-section-title"><h2>控制面组件</h2>{!sectionErrors.components && <span className="cw-count">{components.length} 个组件</span>}</header>
        {sectionErrors.components ? <p role="alert" className="cw-message cw-error">组件读取失败 · {sectionFailure(sectionErrors.components)}</p>
          : components.length === 0 ? <p className="cw-message">未发现控制面组件</p> : (
          <ul className="cw-diag-list">
            {components.map((item) => (
              <li key={`${item.namespace}/${item.name}`}>
                <div className="cw-diag-line">
                  <strong>{item.name}</strong>
                  <span className={`cw-state ${item.readyReplicas < item.desiredReplicas ? 'cw-warning' : item.desiredReplicas ? 'cw-good' : 'cw-neutral'}`}>就绪 {item.readyReplicas}/{item.desiredReplicas}</span>
                </div>
                <div className="cw-diag-meta">
                  <span>{item.kind} · {item.namespace}</span>{item.role && <span>{item.role}</span>}{item.version && <span>{item.version}</span>}
                </div>
                {item.image && <code className="cw-image">{item.image}</code>}
                <RolloutDetails component={item} />
              </li>
            ))}
          </ul>
        )}
      </section>
      <section className="cw-panel">
        <header className="cw-section-title"><h2>读取权限</h2><span className="cw-count">{checks.length} 项检查</span></header>
        {checks.length > 0 && <p className="cw-rbac-scope">按所有命名空间检查。单个命名空间的授权可能不同。</p>}
        {checks.length === 0 ? <p className="cw-message">暂无权限检查结果</p> : (
          <ul className="cw-diag-list">
            {checks.map((check) => (
              <li key={`${check.verb || 'list'}/${check.group}/${check.resource}`}>
                <div className="cw-diag-line">
                  <code>{check.verb || 'list'} {check.group || 'core'}/{check.resource}</code>
                  <span className={`cw-state ${check.error ? 'cw-warning' : check.allowed ? 'cw-good' : 'cw-bad'}`}>
                    {check.error ? '检查失败' : check.allowed ? '允许' : '拒绝'}
                  </span>
                </div>
                {check.error && <small className="cw-image">{check.error}</small>}
              </li>
            ))}
          </ul>
        )}
      </section>
      <section className="cw-panel cw-diag-wide">
        <header className="cw-section-title"><h2>订阅完成数</h2><code className="cw-field">status.completedReleases</code></header>
        {sectionErrors.subscriptions
          ? <p role="alert" className="cw-message cw-error">订阅读取失败 · {sectionFailure(sectionErrors.subscriptions)}</p>
          : <p className="cw-message">{completionEvidence[diagnostics.completedReleasesSupport] ?? '完成数支持情况未知'}</p>}
      </section>
      <section className="cw-panel cw-diag-wide">
        <header className="cw-section-title"><h2 id="diag-api-title">API 资源</h2>{!sectionErrors.resources && <span className="cw-count">{resources.length} 项</span>}</header>
        {sectionErrors.resources && <p role="alert" className="cw-message cw-error">API 资源发现未完成 · {sectionFailure(sectionErrors.resources)}</p>}
        {resources.length === 0 ? (!sectionErrors.resources && <p className="cw-message">未发现 Clusternet API 资源</p>) : (
          <table className="cw-table" aria-labelledby="diag-api-title">
            <thead><tr><th scope="col">资源</th><th scope="col">API 组 / 版本</th><th scope="col">范围</th><th scope="col">支持操作</th></tr></thead>
            <tbody>{resources.map((resource) => (
              <tr key={`${resource.group}/${resource.version}/${resource.name}`}>
                <th scope="row"><strong>{resource.name}</strong><small>{resource.kind}</small></th>
                <td data-label="API 组 / 版本">{resource.group || 'core'}/{resource.version}</td>
                <td data-label="范围">{resource.namespaced ? '命名空间' : '集群级'}</td>
                <td data-label="支持操作">{resource.verbs?.join(', ') || '未上报'}</td>
              </tr>
            ))}</tbody>
          </table>
        )}
      </section>
    </section>
  );
}
