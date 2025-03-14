import { useState } from 'react';
import { faDiagramProject } from '@fortawesome/free-solid-svg-icons';

import { RadioButtonGroup } from '@grafana/ui';

import { ComponentGraph } from '../features/graph/ComponentGraph';
import Page from '../features/layout/Page';
import { useComponentInfo } from '../hooks/componentInfo';

function Graph() {
  const [configType, setConfigType] = useState<string>('local');
  const [components] = useComponentInfo('', configType === 'remote');

  const options = [
    { label: 'Local', value: 'local' },
    { label: 'Remote', value: 'remote' },
  ];

  const handleConfigChange = (value: string) => {
    setConfigType(value);
  };

  return (
    <Page name="Graph" desc="Relationships between defined components" icon={faDiagramProject}>
      <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
        <div style={{ marginBottom: '16px' }}>
          <RadioButtonGroup options={options} value={configType} onChange={handleConfigChange} size="sm" />
        </div>
        <div style={{ flex: 1, minHeight: 0 }}>{components.length > 0 && <ComponentGraph components={components} />}</div>
      </div>
    </Page>
  );
}

export default Graph;
