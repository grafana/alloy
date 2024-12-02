import { useEffect, useState } from 'react';

import { FeedData } from '../features/graph/feedDataType';

export const useLiveGraph = (setData: React.Dispatch<React.SetStateAction<FeedData[]>>) => {
  const [error, setError] = useState('');
  useEffect(() => {
    const abortController = new AbortController();

    const fetchData = async () => {
      try {
        const response = await fetch(`./api/v0/web/livegraph`, {
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

        while (true) {
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
        console.log(error);
        if ((error as Error).name !== 'AbortError') {
          setError((error as Error).message);
        }
      }
    };

    fetchData();

    return () => {
      abortController.abort();
    };
  }, [setData]);

  return { error };
};
