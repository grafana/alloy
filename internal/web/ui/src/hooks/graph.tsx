import { useEffect, useState } from 'react';

import { DebugData } from '../features/graph/debugDataType';

export const useGraph = (
  setData: React.Dispatch<React.SetStateAction<DebugData[]>>,
  moduleID: string,
  window: number,
  enabled: boolean
) => {
  const [error, setError] = useState('');
  useEffect(() => {
    const abortController = new AbortController();

    const fetchData = async () => {
      if (!enabled) {
        return;
      }
      try {
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
        let buffer = '';
        let skipCount = 0;
        const MAX_SKIP_COUNT = 30;

        while (enabled) {
          const { value, done } = await reader.read();
          if (done) break;

          // It happens sometimes that the message is partially received, so we need to buffer it
          // and then parse it once we have a complete message.
          buffer += decoder.decode(value, { stream: true });

          try {
            const data = JSON.parse(buffer) as DebugData[];
            skipCount = 0;
            buffer = '';
            setData(data);
          } catch (err) {
            skipCount++;
            // This is a safeguard to avoid growing the buffer indefinitely if the server is sending invalid data.
            if (skipCount >= MAX_SKIP_COUNT) {
              console.error(
                'Failed to parse data. There is probably an issue with the response from the server. Current buffer:',
                buffer
              );
              skipCount = 0;
              buffer = '';
            }
            continue;
          }
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
  }, [setData, window, enabled, moduleID]);

  return { error };
};
