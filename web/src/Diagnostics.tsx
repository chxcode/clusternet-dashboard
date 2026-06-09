import type { Diagnostics } from './api';

type DiagnosticsPanelProps = {
  diagnostics?: Diagnostics;
  loading: boolean;
  error?: string;
};

export function DiagnosticsPanel({ diagnostics, loading, error }: DiagnosticsPanelProps) {
  if (loading) {
    return <section className="detail-panel">正在加载诊断信息...</section>;
  }
  if (error) {
    return <section className="detail-panel detail-panel--error">{error}</section>;
  }
  if (!diagnostics) {
    return <section className="detail-panel">暂无诊断信息</section>;
  }

  const components = diagnostics.components ?? [];
  const resources = diagnostics.resources ?? [];
  const readOnlyRBACChecks = diagnostics.readOnlyRBACChecks ?? [];

  return (
    <section className="detail-panel" aria-label="Diagnostics">
      <div className="section-heading">
        <p className="eyebrow">Diagnostics</p>
        <h2>诊断与兼容性</h2>
        <span className="section-badge">completedReleases {diagnostics.completedReleasesSupport}</span>
      </div>

      <div className="detail-grid">
        <div>
          <h3>Components</h3>
          <div className="resource-list">
            {components.map((component) => (
              <article className="resource-row" key={`${component.namespace}/${component.name}`}>
                <strong>{component.name}</strong>
                <div className="label-list">
                  {component.role && <span className="label-chip">{component.role}</span>}
                  {component.version && <span className="label-chip">{component.version}</span>}
                </div>
                <span>{component.kind} · {component.namespace} · {component.readyReplicas}/{component.desiredReplicas} ready</span>
                {component.image && <span>{component.image}</span>}
              </article>
            ))}
          </div>
        </div>

        <div>
          <h3>CRD / API Resources</h3>
          <div className="resource-list">
            {resources.slice(0, 12).map((resource) => (
              <article className="resource-row" key={`${resource.group}/${resource.version}/${resource.name}`}>
                <strong>{resource.name}</strong>
                <span>{resource.group || 'core'} / {resource.version} · {resource.kind}</span>
              </article>
            ))}
          </div>
        </div>
      </div>

      <div className="diagnostics-rbac">
        <h3>Read-only RBAC</h3>
        <div className="label-list">
          {readOnlyRBACChecks.map((check) => (
            <span className="label-chip" key={`${check.group}/${check.resource}`}>
              {check.group || 'core'} / {check.resource} {check.allowed ? 'allowed' : 'not allowed'}
            </span>
          ))}
        </div>
      </div>
    </section>
  );
}
