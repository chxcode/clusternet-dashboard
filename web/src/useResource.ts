import { useEffect, useState } from 'react';

export type DataResult<T> = {
  data?: T;
  loading: boolean;
  error?: string;
  updatedAt?: number;
};

type Loader<T> = (signal: AbortSignal) => Promise<T>;

export function useResource<T>(load: Loader<T>, enabled: boolean, refresh: number, scope = ''): DataResult<T> {
  const [result, setResult] = useState<DataResult<T> & { source: Loader<T>; scope: string }>({ source: load, scope, loading: true });

  useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    setResult(previous => previous.source === load && previous.scope === scope
      ? { ...previous, loading: true }
      : { source: load, scope, loading: true });
    load(controller.signal).then(data => {
      if (!controller.signal.aborted) setResult({ source: load, scope, data, updatedAt: Date.now(), loading: false });
    }).catch((cause: unknown) => {
      if (!controller.signal.aborted) setResult(previous => ({
        ...previous, loading: false, error: cause instanceof Error ? cause.message : '请求失败',
      }));
    });
    return () => controller.abort();
  }, [load, enabled, refresh, scope]);

  return result.source === load && result.scope === scope ? result : { loading: enabled };
}
