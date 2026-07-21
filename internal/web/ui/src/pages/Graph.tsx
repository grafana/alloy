import '@grafana/alloy-pipeline-graph/style.css';

import { faDiagramProject } from '@fortawesome/free-solid-svg-icons';
import { PipelineGraph, type PipelineGraphData, type PipelineNode } from '@grafana/alloy-pipeline-graph';
import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router';

import SliderInput from '../components/SliderInput';
import { usePathPrefix } from '../contexts/usePathPrefix';
import Page from '../features/layout/Page';
import { buildPipelineGraph } from '../features/pipeline/buildPipelineGraph';
import type { DebugData } from '../features/pipeline/debugDataType';
import { overlayLiveMetrics } from '../features/pipeline/overlayLiveMetrics';
import styles from '../features/pipeline/PipelineGraphPage.module.css';
import { useComponentInfo } from '../hooks/componentInfo';
import { useGraph } from '../hooks/graph';
import { useModuleInternals } from '../hooks/useModuleInternals';

const DEFAULT_WINDOW = 5;
const MIN_WINDOW = 1;
const MAX_WINDOW = 60;

function Graph() {
  const { '*': id } = useParams();
  const moduleID = id || '';
  const isRemotecfg = moduleID.startsWith('remotecfg/');
  const pathPrefix = usePathPrefix();
  const [components, setComponents] = useComponentInfo(moduleID, isRemotecfg);
  const moduleInternals = useModuleInternals(components, isRemotecfg);

  const [window, setWindow] = useState(DEFAULT_WINDOW);
  const [sliderWindow, setSliderWindow] = useState(DEFAULT_WINDOW);
  const [enabled, setEnabled] = useState(true);
  const [debugData, setDebugData] = useState<DebugData[]>([]);
  const { error } = useGraph(setDebugData, moduleID, window, enabled);

  useEffect(() => {
    setEnabled(false);
    setComponents([]);
    setDebugData([]);
    setTimeout(() => setEnabled(true), 200);
  }, [moduleID, setComponents]);

  function handleWindowChangeComplete(value: number) {
    setWindow(value);
    if (enabled) {
      setEnabled(false);
      setTimeout(() => setEnabled(true), 200);
    }
  }

  const baseGraph = useMemo(() => buildPipelineGraph(components, moduleInternals), [components, moduleInternals]);

  const liveGraph: PipelineGraphData = useMemo(() => overlayLiveMetrics(baseGraph, debugData), [baseGraph, debugData]);

  // Give each node an `href` to its component page; the graph renders these as anchors.
  const graph: PipelineGraphData = useMemo(() => {
    const baseUrl = globalThis.window.location.origin + pathPrefix;
    const nodes = liveGraph.nodes.map((node: PipelineNode) => {
      const nodeModuleID = String(node.meta?.moduleID ?? '');
      const localID = String(node.meta?.localID ?? node.id);
      const remoteCfgPrefix = nodeModuleID.startsWith('remotecfg/') ? 'remotecfg/' : '';
      const path = nodeModuleID !== '' ? `component/${nodeModuleID}/${localID}` : `component/${localID}`;
      return { ...node, href: baseUrl + remoteCfgPrefix + path };
    });
    return { ...liveGraph, nodes };
  }, [liveGraph, pathPrefix]);

  const controls = (
    <SliderInput
      label="Window"
      min={MIN_WINDOW}
      max={MAX_WINDOW}
      value={sliderWindow}
      defaultValue={DEFAULT_WINDOW}
      onChange={setSliderWindow}
      onCommit={handleWindowChangeComplete}
    />
  );

  return (
    <Page
      name="Pipeline"
      desc="Visualize the configured pipeline by stage and signal type."
      icon={faDiagramProject}
      controls={controls}
      infoText={
        <div className={styles.infoText}>Only edges from components that support live debugging show flow rates.</div>
      }
    >
      {error ? (
        <p>Error: {error}</p>
      ) : (
        components.length > 0 && (
          <div className={styles.graphWrapper}>
            <PipelineGraph graph={graph} theme="light" />
          </div>
        )
      )}
    </Page>
  );
}

export default Graph;
