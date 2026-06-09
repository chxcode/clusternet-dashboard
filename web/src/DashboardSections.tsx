const sections = [
  {
    id: 'overview',
    title: 'Overview',
    description: 'Parent cluster health, child cluster totals, subscription counts, and recent abnormal signals.',
  },
  {
    id: 'managed-clusters',
    title: 'Managed Clusters',
    description: 'Child cluster health, labels, Kubernetes version, capacity, heartbeat, and abnormal status.',
  },
  {
    id: 'subscriptions',
    title: 'Subscriptions',
    description: 'Clusternet subscription relationships, feeds, subscribers, target clusters, and sync status.',
  },
  {
    id: 'propagation-status',
    title: 'Propagation Status',
    description: 'Resource propagation results across target clusters, including observed, missing, failed, and offline targets.',
  },
  {
    id: 'diagnostics',
    title: 'Diagnostics',
    description: 'Clusternet component health, CRD discovery, RBAC readability, and version compatibility checks.',
  },
];

export function DashboardSections() {
  return (
    <nav className="section-nav" aria-label="Dashboard sections">
      {sections.map((section) => (
        <a href={`#${section.id}`} className="section-nav-card" key={section.id}>
          <strong>{section.title}</strong>
          <span>{section.description}</span>
        </a>
      ))}
    </nav>
  );
}
