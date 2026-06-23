import { useEffect, useState } from 'react';

import type { ComponentInfo } from '../features/component/types';

/**
 * Fetches internal components for each module ID referenced by custom components.
 */
export function useModuleInternals(components: ComponentInfo[], isRemotecfg: boolean): Map<string, ComponentInfo[]> {
  const [internals, setInternals] = useState<Map<string, ComponentInfo[]>>(new Map());

  useEffect(() => {
    const moduleIds = new Set<string>();
    for (const component of components) {
      for (const moduleId of component.createdModuleIDs ?? []) {
        moduleIds.add(moduleId);
      }
    }

    if (moduleIds.size === 0) {
      setInternals(new Map());
      return;
    }

    const abortController = new AbortController();

    const load = async () => {
      const result = new Map<string, ComponentInfo[]>();
      for (const moduleId of moduleIds) {
        const url = isRemotecfg
          ? `./api/v0/web/remotecfg/modules/${moduleId}/components`
          : `./api/v0/web/modules/${moduleId}/components`;
        const response = await fetch(url, {
          cache: 'no-cache',
          credentials: 'same-origin',
          signal: abortController.signal,
        });
        if (response.ok) {
          result.set(moduleId, (await response.json()) as ComponentInfo[]);
        }
      }
      setInternals(result);
    };

    load().catch((error) => {
      if ((error as Error).name !== 'AbortError') {
        console.error(error);
      }
    });

    return () => abortController.abort();
  }, [components, isRemotecfg]);

  return internals;
}
