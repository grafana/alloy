import { useEffect, useState } from 'react';
import { faDiagnoses } from '@fortawesome/free-solid-svg-icons';

import { Alert, Button, Checkbox, Field, Input } from '@grafana/ui';

import ComponentDiagnosis from '../features/diagnosis/ComponentDiagnosis';
import Page from '../features/layout/Page';
import { useDiagnosis } from '../hooks/diagnosis';

import styles from './Diagnosis.module.css';

const PageDiagnosis = () => {
  const [record, setRecord] = useState(false);
  const [window, setWindow] = useState(60);
  const { insights, loading, error, fetchDiagnosis, cancelDiagnosis } = useDiagnosis(record ? window : 0);
  const [hasRun, setHasRun] = useState(false);
  const [recordingProgress, setRecordingProgress] = useState(0);
  const [isRecording, setIsRecording] = useState(false);

  const handleWindowChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = parseInt(e.currentTarget.value, 10);
    if (!isNaN(value)) {
      setWindow(value);
    }
  };

  const handleFetchDiagnosis = () => {
    setHasRun(true);
    if (record && window > 0) {
      setIsRecording(true);
      setRecordingProgress(0);
    }
    fetchDiagnosis();
  };

  const handleCancelDiagnosis = () => {
    cancelDiagnosis();
    setIsRecording(false);
    setRecordingProgress(0);
    setHasRun(false);
  };

  // Timer effect for recording progress
  useEffect(() => {
    let timer: NodeJS.Timeout | null = null;

    if (isRecording && record && window > 0) {
      const interval = 100; // Update every 100ms for smooth animation
      const totalSteps = window * (1000 / interval);
      let currentStep = 0;

      timer = setInterval(() => {
        currentStep++;
        const progress = Math.min((currentStep / totalSteps) * 100, 100);
        setRecordingProgress(progress);

        if (progress >= 100) {
          setIsRecording(false);
          if (timer) clearInterval(timer);
        }
      }, interval);
    }

    return () => {
      if (timer) clearInterval(timer);
    };
  }, [isRecording, record, window]);

  const controls = (
    <div className={styles.customCard}>
      <h2 className={styles.cardTitle}>Diagnosis Configuration</h2>
      <p className={styles.cardDescription}>Configure and run diagnostics to check your pipelines for issues</p>
      <div className={styles.cardContent}>
        <div className={styles.checkboxContainer}>
          <Checkbox
            value={record}
            label="Record data (live debugging must be enabled)"
            onChange={() => setRecord(!record)}
          />
          {record && (
            <div className={styles.fieldContainer}>
              <Field label="How many seconds to record data">
                <Input id="window" type="number" value={window} onChange={handleWindowChange} />
              </Field>
            </div>
          )}
          {isRecording && (
            <div className={styles.progressContainer}>
              <div className={styles.progressHeader}>
                <span>Diagnosis in progress...</span>
                <span>{Math.round(recordingProgress)}%</span>
              </div>
              <div className={styles.progressBar}>
                <div className={styles.progressFill} style={{ width: `${recordingProgress}%` }} />
              </div>
            </div>
          )}
          <div className={styles.buttonContainer}>
            {isRecording ? (
              <Button onClick={handleCancelDiagnosis} variant="destructive">
                Cancel Diagnosis
              </Button>
            ) : (
              <Button onClick={handleFetchDiagnosis} disabled={loading && !isRecording} variant="primary">
                Run Diagnosis
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  );

  if (error) {
    return (
      <Alert title="Error" severity="error">
        {error}
      </Alert>
    );
  }

  if (!insights || insights.length === 0) {
    return (
      <div className={styles.container}>
        {controls}
        {hasRun && insights && insights.length === 0 && !loading && !error && (
          <div className={styles.noResultsMessage}>No issues detected; Alloy is running optimally.</div>
        )}
      </div>
    );
  }

  return (
    <Page name="Diagnosis" desc="Use the pipeline diagnostics feature to scan for problems and discover best practice recommendations" icon={faDiagnoses}>
      <ComponentDiagnosis insights={insights} />
    </Page>
  );
};

export default PageDiagnosis;
