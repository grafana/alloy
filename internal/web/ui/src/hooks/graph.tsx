import { useEffect, useState } from 'react';
import { Node } from '@xyflow/react';

import { DebugData } from '../features/graph/debugDataType';

export const useGraph = (
  setData: React.Dispatch<React.SetStateAction<DebugData[]>>,
  moduleID: string,
  window: number,
  enabled: boolean,
  layoutedNodes: Node[]
) => {
  const [error, setError] = useState('');
  useEffect(() => {
    const abortController = new AbortController();

    const fetchData = async () => {
      if (!enabled) {
        return;
      }
      try {
        console.log('start fetching data data!');
        const url = moduleID === '' ? `./api/v0/web/graph` : `./api/v0/web/graph/${moduleID}`;
        const response = await fetch(url + `?window=${window}`, {
          signal: abortController.signal,
          cache: 'no-cache',
          credentials: 'same-origin',
        });
        if (!response.ok || !response.body) {
          const text = await response.text();
          const errorMessage = `Failed to connect, status code: ${response.status}, reason: ${text}`;
          throw new Error(errorMessage);
        }

        const reader = response.body.getReader();
        const decoder = new TextDecoder();

        while (enabled) {
          const { value, done } = await reader.read();
          if (done) {
            break;
          }

          const decodedChunks = decoder
            .decode(value, { stream: true })
            .split('|;|')
            .filter((entry) => entry.length != 0);
          setData(() => decodedChunks.map((chunk) => JSON.parse(chunk)));
        }
      } catch (error) {
        if ((error as Error).name !== 'AbortError') {
          setError((error as Error).message);
        }
      }
    };

    fetchData();

    return () => {
      abortController.abort();
    };
  }, [setData, layoutedNodes, window, enabled]);

  return { error };
};
