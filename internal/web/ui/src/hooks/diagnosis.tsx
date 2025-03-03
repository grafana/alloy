import { useState } from 'react';

export interface DiagnosisInsight {
  level: string;
  msg: string;
  link: string;
}

export const useDiagnosis = () => {
  const [insights, setInsights] = useState<DiagnosisInsight[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchDiagnosis = async () => {
    setLoading(true);
    setError(null);

    try {
      const response = await fetch(`./api/v0/web/diagnosis`);
      if (!response.ok) {
        throw new Error(`Error fetching diagnosis data: ${response.statusText}`);
      }
      const data = await response.json();
      setInsights(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error occurred');
    } finally {
      setLoading(false);
    }
  };

  return {
    insights,
    loading,
    error,
    fetchDiagnosis,
  };
};
