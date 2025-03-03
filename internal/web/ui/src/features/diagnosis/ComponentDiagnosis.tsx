import { Alert, Box, Button, LoadingPlaceholder } from '@grafana/ui';

import { DiagnosisInsight, useDiagnosis } from '../../hooks/diagnosis';

interface ComponentDiagnosisProps {
  insights: DiagnosisInsight[];
  loading: boolean;
  error: string | null;
}

const ComponentDiagnosis = ({ insights, loading, error }: ComponentDiagnosisProps) => {
  const getLevelColor = (level: string): string => {
    switch (level.toLowerCase()) {
      case 'error':
        return 'red';
      case 'warning':
        return 'orange';
      case 'info':
        return 'blue';
      default:
        return 'gray';
    }
  };

  return (
    <Box padding={3}>
      {loading ? (
        <Box display="flex" alignItems="center" justifyContent="center">
          <LoadingPlaceholder text="Running diagnosis..." />
        </Box>
      ) : error ? (
        <Alert title="Error" severity="error">
          {error}
        </Alert>
      ) : insights.length === 0 && !loading ? (
        <p>Click "Run Diagnosis" to analyze your system for potential issues.</p>
      ) : insights.length === 0 ? (
        <Alert title="No issues found" severity="success">
          No diagnostic insights available. Your system appears to be running optimally.
        </Alert>
      ) : (
        <Box marginTop={3}>
          {insights.map((insight: DiagnosisInsight, index: number) => (
            <div
              key={index}
              style={{
                marginBottom: '16px',
                padding: '16px',
                border: '1px solid #ddd',
                borderRadius: '4px',
                backgroundColor: 'var(--background-secondary)',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', marginBottom: '8px' }}>
                <span
                  style={{
                    color: getLevelColor(insight.level),
                    marginRight: '8px',
                    fontWeight: 'bold',
                  }}
                >
                  {insight.level.toUpperCase()}
                </span>
                <span>{insight.msg}</span>
              </div>
              {insight.link && (
                <div style={{ marginTop: '8px' }}>
                  <a href={insight.link} target="_blank" rel="noopener noreferrer">
                    Learn more
                  </a>
                </div>
              )}
            </div>
          ))}
        </Box>
      )}
    </Box>
  );
};

export default ComponentDiagnosis;
