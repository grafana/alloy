import { faBullseye } from '@fortawesome/free-solid-svg-icons';

import TargetsList from '../features/targets/TargetsList';
import Page from '../features/layout/Page';
import { useTargets } from '../hooks/useTargets';

function TargetsPage() {
  const targets = useTargets();

  return (
    <Page name="Targets" desc="Scrape target status" icon={faBullseye}>
      <TargetsList targets={targets} />
    </Page>
  );
}

export default TargetsPage;
