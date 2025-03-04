import { useState } from 'react';

export interface DiagnosisInsight {
  level: string;
  msg: string;
  link: string;
  module: string;
}

export const useDiagnosis = (window: number) => {
  const [insights, setInsights] = useState<DiagnosisInsight[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [abortController, setAbortController] = useState<AbortController | null>(null);

  const fetchDiagnosis = async () => {
    setLoading(true);
    setError(null);

    // Create a new AbortController for this request
    const controller = new AbortController();
    setAbortController(controller);

    try {
      console.log(`Start a diagnosis with window ${window}`);
      const response = await fetch(`./api/v0/web/diagnosis?window=${window}`, {
        signal: controller.signal,
      });

      if (!response.ok) {
        // Try to get more detailed error information from the response
        let errorDetails = response.statusText;
        try {
          // Attempt to read the response body for more details
          const errorText = await response.text();
          if (errorText) {
            errorDetails = `${errorDetails}: ${errorText}`;
          }
        } catch (readError) {
          console.error('Failed to read error response body:', readError);
        }

        throw new Error(`Error fetching diagnosis data (${response.status}): ${errorDetails}`);
      }

      const data = await response.json();
      setInsights(data);
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') {
        setError('Diagnosis cancelled');
      } else if (err instanceof Error) {
        // Include more details from the error object
        const errorMessage = err.message;

        setError(`${errorMessage}`);
      } else {
        setError(`Unknown error occurred: ${String(err)}`);
      }
    } finally {
      setLoading(false);
      setAbortController(null);
    }
  };

  const cancelDiagnosis = () => {
    if (abortController) {
      abortController.abort();
    }
  };

  return {
    insights,
    loading,
    error,
    fetchDiagnosis,
    cancelDiagnosis,
  };
};
