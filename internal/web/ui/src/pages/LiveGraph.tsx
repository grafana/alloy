import { useParams } from 'react-router-dom';
import { faDiagramProject } from '@fortawesome/free-solid-svg-icons';

import ComponentLiveGraph from '../features/graph/ComponentLiveGraph';
import { Legend } from '../features/graph/Legend';
import Page from '../features/layout/Page';
import { useComponentInfo } from '../hooks/componentInfo';

function Graph() {
  const { '*': id } = useParams();
  const moduleID = id || '';
  const [components] = useComponentInfo(moduleID, false);

  const controls = <Legend></Legend>;

  return (
    <Page name="LiveGraph" desc="Data passing through the components" icon={faDiagramProject} controls={controls}>
      {components.length > 0 && <ComponentLiveGraph components={components} moduleID={moduleID} />}
    </Page>
  );
}

export default Graph;
