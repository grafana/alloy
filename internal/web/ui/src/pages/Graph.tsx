import { faDiagramProject } from '@fortawesome/free-solid-svg-icons';
import { useEffect, useState } from 'react';
import { useParams } from 'react-router';

import SliderInput from '../components/SliderInput';
import ComponentGraph from '../features/graph/ComponentGraph';
import { Legend } from '../features/graph/Legend';
import Page from '../features/layout/Page';
import { useComponentInfo } from '../hooks/componentInfo';
import styles from './LiveDebugging.module.css';

const DEFAULT_WINDOW = 5;
const MIN_WINDOW = 1;
const MAX_WINDOW = 60;

function Graph() {
  const { '*': id } = useParams();
  const moduleID = id || '';
  const [components, setComponents] = useComponentInfo(moduleID, moduleID.startsWith('remotecfg/'));
  const [window, setWindow] = useState(DEFAULT_WINDOW);
  const [sliderWindow, setSliderWindow] = useState(DEFAULT_WINDOW);
  const [enabled, setEnabled] = useState(true);

  // Reset component state when moduleID changes
  useEffect(() => {
    setEnabled(false);
    setComponents([]);
    setTimeout(() => {
      setEnabled(true);
    }, 200);
  }, [moduleID, setComponents]);

  function handleWindowChangeComplete(value: number) {
    setWindow(value);
    if (enabled) {
      setEnabled(false);
      setTimeout(() => setEnabled(true), 200);
    }
  }

  const controls = (
    <>
      <SliderInput
        label="Window"
        min={MIN_WINDOW}
        max={MAX_WINDOW}
        value={sliderWindow}
        defaultValue={DEFAULT_WINDOW}
        onChange={setSliderWindow}
        onCommit={handleWindowChangeComplete}
      />
      <Legend></Legend>
    </>
  );

  return (
    <Page
      name="Graph"
      desc="Visualize data flow per second for components."
      icon={faDiagramProject}
      controls={controls}
      infoText={
        <div className={styles.infoText}>Only edges from components that support live debugging will be colored.</div>
      }
    >
      {components.length > 0 && (
        <ComponentGraph components={components} moduleID={moduleID} enabled={enabled} window={window} />
      )}
    </Page>
  );
}

export default Graph;
