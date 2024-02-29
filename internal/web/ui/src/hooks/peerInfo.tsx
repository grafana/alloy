import { useEffect, useState } from 'react';

import { PeerInfo } from '../features/clustering/types';

/**
 * usePeerInfo retrieves the list of clustering peers from the API.
 *
 * @param fromPeer The peer requesting component info.
 */
export const usePeerInfo = (fromPeer?: string): PeerInfo[] => {
  const [peers, setPeers] = useState<PeerInfo[]>([]);

  useEffect(
    function () {
      const worker = async () => {
        const infoPath = './api/v0/web/peers';

        // Request is relative to the <base> tag inside of <head>.
        const resp = await fetch(infoPath, {
          cache: 'no-cache',
          credentials: 'same-origin',
        });
        setPeers(await resp.json());
      };

      worker().catch(console.error);
    },
    [fromPeer]
  );

  return peers;
};
