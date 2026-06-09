import { apiPath } from './basePath';

export type Cluster = {
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
};

export type Subscription = {
  name: string;
  namespace?: string;
  status: string;
  desiredReleases: number;
  completedReleases: number;
  observedReleases: number;
  completionKnown: boolean;
  bindingClusterCount: number;
  bindingClusters: string[];
  schedulerName?: string;
  schedulingStrategy?: string;
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
  name: string;
  namespace?: string;
  kind: string;
  overrideCount: number;
};

export type FeedInventory = {
  name: string;
  namespace?: string;
  feedCount: number;
};

export type HelmRelease = {
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

export type SubscriptionTargetCluster = {
  name: string;
  namespace?: string;
  online: boolean;
  status: string;
  observed: boolean;
  available?: boolean;
  replicaStatus?: Record<string, unknown>;
};

export type SubscriptionDetail = {
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
};

export type RBACCheck = {
  resource: string;
  group?: string;
  allowed: boolean;
  error?: string;
};

export type Diagnostics = {
  resources: APIResource[];
  components: Component[];
  completedReleasesSupport: string;
  readOnlyRBACChecks: RBACCheck[];
};

async function fetchJSON<T>(path: string, key: string): Promise<T[]> {
  const response = await fetch(apiPath(path));
  if (!response.ok) {
    throw new Error(`Failed to load ${key}: ${response.status}`);
  }
  const payload = await response.json() as Record<string, T[] | undefined>;
  return payload[key] ?? [];
}

export async function fetchClusters(): Promise<Cluster[]> {
  return fetchJSON<Cluster>('/clusters', 'clusters');
}

export async function fetchSubscriptions(): Promise<Subscription[]> {
  return fetchJSON<Subscription>('/subscriptions', 'subscriptions');
}

export async function fetchManifests(): Promise<Manifest[]> {
  return fetchJSON<Manifest>('/manifests', 'manifests');
}

export async function fetchGlobalizations(): Promise<OverrideResource[]> {
  return fetchJSON<OverrideResource>('/globalizations', 'globalizations');
}

export async function fetchLocalizations(): Promise<OverrideResource[]> {
  return fetchJSON<OverrideResource>('/localizations', 'localizations');
}

export async function fetchFeedInventories(): Promise<FeedInventory[]> {
  return fetchJSON<FeedInventory>('/feedinventories', 'feedInventories');
}

export async function fetchHelmReleases(): Promise<HelmRelease[]> {
  return fetchJSON<HelmRelease>('/helmreleases', 'helmReleases');
}

export async function fetchSubscriptionDetail(namespace: string, name: string): Promise<SubscriptionDetail> {
  const response = await fetch(apiPath(`/subscriptions/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`));
  if (!response.ok) {
    throw new Error(`Failed to load subscription detail: ${response.status}`);
  }
  return response.json() as Promise<SubscriptionDetail>;
}

export async function fetchDiagnostics(): Promise<Diagnostics> {
  const response = await fetch(apiPath('/diagnostics'));
  if (!response.ok) {
    throw new Error(`Failed to load diagnostics: ${response.status}`);
  }
  return response.json() as Promise<Diagnostics>;
}
