import { useEffect, useState } from 'react';

import { type DebugData } from '../features/graph/debugDataType';

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

        while (enabled) {
          const { value, done } = await reader.read();
          if (done) break;

          // It happens sometimes that the message is partially received (it can be late and arrive in multiple chunks
          // and can even merge with the previous message)
          buffer += decoder.decode(value, { stream: true });

          // Split on the delimiter and process each complete message
          const messages = buffer.split('|;|');

          // The last element will either be empty or an incomplete message that will be used as the buffer for the next message
          buffer = messages.pop() || '';

          for (const message of messages) {
            if (!message) continue; // Skip empty messages

            try {
              const data = JSON.parse(message) as DebugData[];
              setData(data);
            } catch (err) {
              console.error('Failed to parse message:', message, err);
            }
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
