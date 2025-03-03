import { faDiagnoses } from '@fortawesome/free-solid-svg-icons';

import { Button } from '@grafana/ui';

import ComponentDiagnosis from '../features/diagnosis/ComponentDiagnosis';
import Page from '../features/layout/Page';
import { useDiagnosis } from '../hooks/diagnosis';

const PageDiagnosis = () => {
  const { insights, loading, error, fetchDiagnosis } = useDiagnosis();

  const controls = (
    <>
      <Button onClick={fetchDiagnosis} disabled={loading} variant="primary">
        Run Diagnosis
      </Button>
    </>
  );

  return (
    <Page
      name="Diagnosis"
      desc="Run a diagnosis on your pipelines to check for issues and tips"
      icon={faDiagnoses}
      controls={controls}
    >
      <ComponentDiagnosis insights={insights} loading={loading} error={error} />
    </Page>
  );
};

export default PageDiagnosis;
