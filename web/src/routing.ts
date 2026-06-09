import { getBasePath } from './basePath';

export type DashboardPage = 'overview' | 'managed-clusters' | 'subscriptions' | 'propagation-status' | 'diagnostics';

const routeByPage: Record<DashboardPage, string> = {
  overview: '/overview',
  'managed-clusters': '/clusters',
  subscriptions: '/subscriptions',
  'propagation-status': '/propagation',
  diagnostics: '/diagnostics',
};

const pageByRoute: Record<string, DashboardPage> = {
  '/': 'overview',
  '/overview': 'overview',
  '/clusters': 'managed-clusters',
  '/subscriptions': 'subscriptions',
  '/propagation': 'propagation-status',
  '/diagnostics': 'diagnostics',
};

export function pageFromPath(pathname: string, basePath = getBasePath()): DashboardPage {
  const normalizedBase = basePath || '';
  let route = pathname;
  if (normalizedBase && route.startsWith(normalizedBase)) {
    route = route.slice(normalizedBase.length) || '/';
  }
  if (!route.startsWith('/')) {
    route = `/${route}`;
  }
  route = route.replace(/\/$/, '') || '/';
  return pageByRoute[route] || 'overview';
}

export function pathForPage(page: DashboardPage, basePath = getBasePath()): string {
  return `${basePath || ''}${routeByPage[page]}`;
}
