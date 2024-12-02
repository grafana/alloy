import { faDiagramProject } from '@fortawesome/free-solid-svg-icons';

import ComponentLiveGraph from '../features/graph/ComponentLiveGraph';
import Page from '../features/layout/Page';
import { useComponentInfo } from '../hooks/componentInfo';

function Graph() {
  const [components] = useComponentInfo('', false);

  return (
    <Page name="LiveGraph" desc="Data passing through the components" icon={faDiagramProject}>
      {components.length > 0 && <ComponentLiveGraph components={components} />}
    </Page>
  );
}

export default Graph;
