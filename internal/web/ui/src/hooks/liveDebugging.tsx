import { useEffect, useState } from 'react';

export const useLiveDebugging = (
  componentID: string,
  enabled: boolean,
  sampleProb: number,
  setData: React.Dispatch<React.SetStateAction<string[]>>
) => {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const maxLines = 5000; // TODO: should we make this configurable?

  useEffect(() => {
    const abortController = new AbortController();

    const fetchData = async () => {
      if (!enabled) {
        setLoading(false);
        return;
      }

      setLoading(true);

      try {
        const response = await fetch(`./api/v0/web/debug/${componentID}?sampleProb=${sampleProb}`, {
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

          const decodedChunk = decoder.decode(value, { stream: true });

          setData((prevValue) => {
            const newValue = decodedChunk.split('|;|');
            newValue.pop(); // last element is empty because of the split, we discard it

            if (newValue.length > maxLines) {
              console.warn(
                'Received %s lines but the buffer has a maximum of %s. Some lines will be dropped.',
                newValue.length,
                maxLines
              );
            }

            let dataArr = prevValue.concat(newValue);
            if (dataArr.length > maxLines) {
              dataArr = dataArr.slice(-maxLines); // truncate the array to keep the last {maxLines} lines
            }
            return dataArr;
          });
        }
      } catch (error) {
        if ((error as Error).name !== 'AbortError') {
          setError((error as Error).message);
        }
      } finally {
        setLoading(false);
      }
    };

    fetchData();

    return () => {
      abortController.abort();
    };
  }, [componentID, enabled, sampleProb, setData]);

  return { loading, error };
};
