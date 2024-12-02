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
            console.log('done');
            break;
          }

          const decodedChunks = decoder.decode(value, { stream: true }).split('|;|'); //not sure if that's enough
          decodedChunks.pop(); // pop the last value because it should be empty

          setData((prevValue: FeedData[]) => {
            return decodedChunks.reduce((updatedData, chunk) => {
              const newFeed: FeedData = JSON.parse(chunk);

              const existingIndex = updatedData.findIndex(
                (obj) => obj.componentID === newFeed.componentID && obj.type === newFeed.type
              );

              if (existingIndex !== -1) {
                // Create a new array with updated count
                const newData = [...updatedData];
                newData[existingIndex] = {
                  ...newData[existingIndex],
                  count: newData[existingIndex].count + newFeed.count,
                };
                return newData;
              } else {
                // Add new feed
                return [...updatedData, newFeed];
              }
            }, prevValue);
          });
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
