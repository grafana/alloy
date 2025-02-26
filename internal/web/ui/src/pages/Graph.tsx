import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { faDiagramProject } from '@fortawesome/free-solid-svg-icons';

import { Slider } from '@grafana/ui';

import ComponentGraph from '../features/graph/ComponentGraph';
import { Legend } from '../features/graph/Legend';
import Page from '../features/layout/Page';
import { useComponentInfo } from '../hooks/componentInfo';

import styles from './LiveDebugging.module.css';

const DEFAULT_WINDOW = 5;

function Graph() {
  const { '*': id } = useParams();
  const moduleID = id || '';
  const [components] = useComponentInfo(moduleID, false);
  const [window, setWindow] = useState(DEFAULT_WINDOW);
  const [sliderWindow, setSliderWindow] = useState(DEFAULT_WINDOW);
  const [enabled, setEnabled] = useState(true);

  function handleWindowChange(value?: number) {
    setSliderWindow(value ? value : DEFAULT_WINDOW);
  }

  function handleWindowChangeComplete(value?: number) {
    setWindow(value ? value : DEFAULT_WINDOW);
    if (enabled) {
      setEnabled(false);
      setTimeout(() => setEnabled(true), 200);
    }
  }

  const controls = (
    <>
      <div className={styles.slider}>
        <span className={styles.sliderLabel}>Window</span>
        <Slider
          included
          min={1}
          max={60}
          value={sliderWindow}
          orientation="horizontal"
          onChange={handleWindowChange}
          onAfterChange={handleWindowChangeComplete}
        />
      </div>
      <Legend></Legend>
    </>
  );

  return (
    <Page
      name="Graph"
      desc="Visualize data flow per second for components that support live debugging."
      icon={faDiagramProject}
      controls={controls}
    >
      {components.length > 0 && (
        <ComponentGraph components={components} moduleID={moduleID} enabled={enabled} window={window} />
      )}
    </Page>
  );
}

export default Graph;
