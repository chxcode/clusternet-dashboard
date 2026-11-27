import { apiPath } from './basePath';

export type ResourceCondition = {
  type: string;
  status?: string;
  reason?: string;
  message?: string;
  lastTransitionTime?: string;
};

export type Cluster = {
  uid?: string;
  clusterId?: string;
  name: string;
  namespace?: string;
  status: string;
  online: boolean;
  labels?: Record<string, string>;
  kubernetesVersion?: string;
  apiserverURL?: string;
  lastObservedTime?: string;
  platform?: string;
  agentVersion?: string;
  evidence?: {
    createdAt?: string;
    clusterCIDRs?: string[];
    serviceCIDRs?: string[];
    appPusher?: boolean;
    useSocket?: boolean;
    predictorEnabled?: boolean;
    predictorDirectAccess?: boolean;
    predictorEndpoint?: string;
    conditions?: ResourceCondition[];
    readyz?: boolean;
    livez?: boolean;
    healthz?: boolean;
    heartbeatFrequencySeconds?: number;
    clusterType?: string;
    syncMode?: string;
    taints?: { key: string; effect?: string }[];
    nodeStatistics?: Partial<Record<'readyNodes' | 'notReadyNodes' | 'unknownNodes' | 'lostNodes', number>>;
    capacity?: Partial<Record<'cpu' | 'memory' | 'pods', string>>;
    allocatable?: Partial<Record<'cpu' | 'memory' | 'pods', string>>;
  };
};

export type SubscriptionDistribution = {
  strategy: string;
  targetScope: 'all' | 'selector' | 'unknown' | 'none';
  selectors: (string | null)[];
  dividingType?: string;
  dynamicStrategy?: string;
  subgroupConfigured?: boolean;
  subgroupMinClusters?: (number | null)[];
  subscriberWeights?: (number | null)[];
};

export type Subscription = {
  name: string;
  namespace?: string;
  status: string;
  desiredReleases: number;
  completedReleases: number;
  observedReleases: number;
  completionKnown: boolean;
  bindingClusterCount: number | null;
  bindingClusters: string[] | null;
  schedulerName?: string;
  schedulingStrategy?: string;
  distribution?: SubscriptionDistribution;
  subscriberCount?: number;
  feedCount?: number;
  feedKinds?: string[];
};

export type Manifest = {
  name: string;
  namespace?: string;
  templateAPIVersion?: string;
  templateKind?: string;
  templateName?: string;
  templateNamespace?: string;
};

export type OverrideResource = {
  feed?: Feed;
  rules?: { name?: string; type?: string }[];
  policy?: string;
  priority?: number;
  clusterSelector?: string;
  invalidSelector?: boolean;
  name: string;
  namespace?: string;
  kind: string;
  overrideCount: number;
};

export type FeedInventory = {
  feeds?: Feed[];
  name: string;
  namespace?: string;
  feedCount: number;
};

export type HelmRelease = {
  resourceVersion?: string;
  deleting?: boolean;
  controlledByDescription?: boolean;
  atomic?: boolean;
  upgradeAtomic?: boolean;
  triage?: 'investigating' | 'resolved';
  triageIncident?: string;
  descriptionOwner?: { name: string; uid: string };
  uid?: string;
  name: string;
  namespace?: string;
  clusterName?: string;
  subscriptionName?: string;
  subscriptionNamespace?: string;
  releaseName?: string;
  targetNamespace?: string;
  chart?: string;
  chartVersion?: string;
  phase?: string;
  description?: string;
  revision?: number;
  firstDeployed?: string;
  lastDeployed?: string;
};

export type Feed = {
  apiVersion?: string;
  kind?: string;
  name?: string;
  namespace?: string;
};

export type TargetFeedReport = Feed & {
  available?: boolean;
  replicaStatus?: Record<string, number>;
};

export type SubscriptionTargetCluster = {
  name: string;
  namespace?: string;
  online: boolean;
  status: string;
  observed: boolean;
  available?: boolean;
  replicaStatus?: Record<string, number>;
  feedReports?: TargetFeedReport[];
  unattributedFeeds?: Feed[];
};

export type SubscriptionDetail = {
  subscriberSelectors?: (string | null)[];
  coverage?: {
    registeredClusters: number;
    labelMatchedClusters?: number;
    unboundMatches?: { namespace: string; name: string }[];
  };
  summary: Subscription;
  feeds: Feed[];
  targetClusters: SubscriptionTargetCluster[];
};

export type APIResource = {
  name: string;
  group?: string;
  version: string;
  kind: string;
  namespaced: boolean;
  verbs: string[];
};

export type Component = {
  name: string;
  namespace: string;
  kind: string;
  image?: string;
  version?: string;
  role?: string;
  readyReplicas: number;
  desiredReplicas: number;
  availableReplicas?: number;
  updatedReplicas?: number;
  unavailableReplicas?: number;
  generation?: number;
  observedGeneration?: number;
  conditions?: ResourceCondition[];
};

export type RBACCheck = {
  resource: string;
  verb?: string;
  group?: string;
  allowed: boolean;
  error?: string;
};

export type Diagnostics = {
  sectionErrors?: Partial<Record<'components' | 'subscriptions' | 'resources', string>>;
  resources: APIResource[];
  components: Component[];
  completedReleasesSupport: string;
  readOnlyRBACChecks: RBACCheck[];
};

async function getJSON<T>(path: string, signal?: AbortSignal): Promise<T> {
  const url = apiPath(path);
  const response = signal ? await fetch(url, { signal }) : await fetch(url);
  if (!response.ok) {
    const messages: Record<number, string> = {
      401: '集群身份验证失败', 403: '当前身份缺少读取权限', 404: '资源不存在',
      410: '原关联对象已被替换', 429: '请求过于频繁，请稍后重试', 503: '服务暂不可用', 504: '集群响应超时',
    };
    throw new Error(`${messages[response.status] || '请求失败'}（HTTP ${response.status}）`);
  }
  return response.json() as Promise<T>;
}

async function fetchJSON<T>(path: string, key: string, signal?: AbortSignal): Promise<T[]> {
  const payload = await getJSON<Record<string, T[] | undefined>>(path, signal);
  return payload[key] ?? [];
}

export async function fetchClusters(signal?: AbortSignal): Promise<Cluster[]> {
  return fetchJSON<Cluster>('/clusters', 'clusters', signal);
}

export async function fetchSubscriptions(signal?: AbortSignal): Promise<Subscription[]> {
  return fetchJSON<Subscription>('/subscriptions', 'subscriptions', signal);
}

export async function fetchManifests(signal?: AbortSignal): Promise<Manifest[]> {
  return fetchJSON<Manifest>('/manifests', 'manifests', signal);
}

export async function fetchGlobalizations(signal?: AbortSignal): Promise<OverrideResource[]> {
  return fetchJSON<OverrideResource>('/globalizations', 'globalizations', signal);
}

export async function fetchLocalizations(signal?: AbortSignal): Promise<OverrideResource[]> {
  return fetchJSON<OverrideResource>('/localizations', 'localizations', signal);
}

export async function fetchFeedInventories(signal?: AbortSignal): Promise<FeedInventory[]> {
  return fetchJSON<FeedInventory>('/feedinventories', 'feedInventories', signal);
}

export async function fetchHelmReleases(signal?: AbortSignal): Promise<HelmRelease[]> {
  return fetchJSON<HelmRelease>('/helmreleases', 'helmReleases', signal);
}

export async function fetchSubscriptionDetail(namespace: string, name: string, signal?: AbortSignal): Promise<SubscriptionDetail> {
  const detail = await getJSON<SubscriptionDetail>(`/subscriptions/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`, signal);
  if (!detail.summary?.name) throw new Error('未取得订阅详情');
  return detail;
}

export const fetchDiagnostics = (signal?: AbortSignal) => getJSON<Diagnostics>('/diagnostics', signal);

export type WorkspaceContext = {
  chartDeployment?: boolean;
  nativePublishing?: boolean;
  failedRegistrationDeletion?: boolean;
  chartSourceEditing?: boolean;
  releaseDeletion?: boolean;
  releaseTriage?: boolean;
  chartVersionEditing?: boolean;
  name: string;
  configured: boolean;
  inCluster?: boolean;
  displayName?: string;
  version?: string;
  labelEditing?: boolean;
};
export const fetchContext = (signal?: AbortSignal) => getJSON<WorkspaceContext>('/context', signal);

export const fetchTriageStates = (signal?: AbortSignal) =>
  getJSON<Record<string, 'investigating' | 'resolved'>>('/release-triage', signal);

export type ReleaseEvent = { name: string; type?: string; reason?: string; message?: string; lastSeen?: string };

export function fetchReleaseEvents(namespace: string, uid: string, signal?: AbortSignal) {
  const query = new URLSearchParams({ namespace, uid });
  return fetchJSON<ReleaseEvent>(`/helmrelease-events?${query}`, 'events', signal);
}

export function fetchDescriptionStatus(namespace: string, name: string, uid: string, signal?: AbortSignal) {
  const query = new URLSearchParams({ namespace, name, uid });
  return getJSON<{ phase?: string; reason?: string }>(`/description-status?${query}`, signal);
}

export type Registration = {
  uid?: string; resourceVersion?: string; deleting?: boolean; failureReason?: string;
  namespaceEvidence?: 'allocated' | 'occupied' | 'terminating' | 'available';
  deletionMode?: 'failed' | 'denied' | 'pendingNamespace';
  name: string; created?: string; clusterName?: string; requestedNamespace?: string; clusterID?: string;
  clusterType?: string; syncMode?: string; result?: string; managedCluster?: string; dedicatedNamespace?: string;
};
export type HelmChart = {
  name: string; namespace: string; chart?: string; version?: string; targetNamespace?: string; phase?: string;
  timeoutSeconds?: number; options?: Record<string, boolean>;
};
export type DistributionBase = {
  name: string; namespace: string; clusterName?: string; subscriptionName?: string;
  subscriptionNamespace?: string; feeds?: Feed[] | null;
};
export const fetchRegistrations = (signal?: AbortSignal) => fetchJSON<Registration>('/registrations', 'registrations', signal);
export const fetchHelmCharts = (signal?: AbortSignal) => fetchJSON<HelmChart>('/helmcharts', 'helmCharts', signal);
export const fetchBases = (signal?: AbortSignal) => fetchJSON<DistributionBase>('/bases', 'bases', signal);
