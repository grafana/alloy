import { useEffect, useState } from 'react';

import { type TargetInfo, type TargetsResponse } from '../features/targets/types';

interface UseTargetsOptions {
  job?: string;
  health?: string;
  component?: string;
}

/**
 * useTargets retrieves the list of scrape targets from the API.
 */
export const useTargets = (options?: UseTargetsOptions): TargetInfo[] => {
  const [targets, setTargets] = useState<TargetInfo[]>([]);

  useEffect(
    function () {
      const worker = async () => {
        const params = new URLSearchParams();
        if (options?.job) params.set('job', options.job);
        if (options?.health) params.set('health', options.health);
        if (options?.component) params.set('component', options.component);

        const queryString = params.toString();
        const infoPath = `./api/v0/web/targets${queryString ? `?${queryString}` : ''}`;

        // Request is relative to the <base> tag inside of <head>.
        const resp = await fetch(infoPath, {
          cache: 'no-cache',
          credentials: 'same-origin',
        });
        const data: TargetsResponse = await resp.json();
        setTargets(data.data || []);
      };

      worker().catch(console.error);
    },
    [options?.job, options?.health, options?.component]
  );

  return targets;
};
