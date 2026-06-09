declare global {
  interface Window {
    __CLUSTERNET_DASHBOARD_BASE_PATH__?: string;
  }
}

export function normalizeBasePath(value?: string): string {
  if (!value || value === '/') return '';
  const trimmed = value.trim();
  if (!trimmed || trimmed === '/') return '';
  const withLeadingSlash = trimmed.startsWith('/') ? trimmed : `/${trimmed}`;
  return withLeadingSlash.endsWith('/') ? withLeadingSlash.slice(0, -1) : withLeadingSlash;
}

export function getBasePath(): string {
  return normalizeBasePath(window.__CLUSTERNET_DASHBOARD_BASE_PATH__ || import.meta.env.BASE_URL);
}

export function apiPath(path: string): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  return `${getBasePath()}/api${normalizedPath}`;
}
