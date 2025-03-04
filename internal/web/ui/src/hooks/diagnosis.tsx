import { useState } from 'react';

export interface DiagnosisInsight {
  level: string;
  msg: string;
  link: string;
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
        throw new Error(`Error fetching diagnosis data: ${response.statusText}`);
      }
      const data = await response.json();
      setInsights(data);
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') {
        setError('Diagnosis cancelled');
      } else {
        setError(err instanceof Error ? err.message : 'Unknown error occurred');
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
