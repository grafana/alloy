import { useState } from 'react';
import { useParams } from 'react-router-dom';
import AutoScroll from '@brianmcallister/react-auto-scroll';
import { faBroom, faBug, faCopy, faRoad, faStop } from '@fortawesome/free-solid-svg-icons';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';

import { Field, Input, Slider } from '@grafana/ui';

import Page from '../features/layout/Page';
import { useLiveDebugging } from '../hooks/liveDebugging';

import styles from './LiveDebugging.module.css';

function PageLiveDebugging() {
  const { '*': componentID } = useParams();
  const [enabled, setEnabled] = useState(true);
  const [data, setData] = useState<string[]>([]);
  const [sampleProb, setSampleProb] = useState(1);
  const [sliderProb, setSliderProb] = useState(100);
  const [filterValue, setFilterValue] = useState('');
  const { loading, error } = useLiveDebugging(String(componentID), enabled, sampleProb, setData);

  const filteredData = data.filter((n) => n.toLowerCase().includes(filterValue.toLowerCase()));

  function toggleEnableButton() {
    if (enabled) {
      return (
        <div className={styles.debugLink}>
          <button className={styles.stopButton} onClick={() => setEnabled(false)}>
            <FontAwesomeIcon icon={faStop} /> Stop
          </button>
        </div>
      );
    }
    return (
      <div className={styles.debugLink}>
        <button className={styles.resumeButton} onClick={() => setEnabled(true)}>
          <FontAwesomeIcon icon={faRoad} /> Resume
        </button>
      </div>
    );
  }

  function handleSampleChange(value?: number) {
    setSliderProb(value ? value : 0);
  }

  function handleSampleChangeComplete(value?: number) {
    setSampleProb((value ? value : 0) / 100.0);
    if (enabled) {
      setEnabled(false);
      setTimeout(() => setEnabled(true), 200);
    }
  }

  async function copyDataToClipboard(): Promise<void> {
    const dataToCopy = filteredData.join('\n');

    try {
      await navigator.clipboard.writeText(dataToCopy);
    } catch (err) {
      console.error('Failed to copy data to clipboard: ', err);
    }
  }

  const samplingControl = (
    <div className={styles.slider}>
      <span className={styles.sliderLabel}>Sample rate</span>
      <Slider
        included
        min={0}
        max={100}
        value={sliderProb}
        orientation="horizontal"
        onChange={handleSampleChange}
        onAfterChange={handleSampleChangeComplete}
      />
    </div>
  );

  function handleFilterChange(event: React.ChangeEvent<HTMLInputElement>) {
    setFilterValue(event.target.value);
  }

  const filterControl = (
    <Field className={styles.filter}>
      <Input placeholder="Filter data..." onChange={handleFilterChange} />
    </Field>
  );

  const controls = (
    <>
      {filterControl}
      {samplingControl}
      {toggleEnableButton()}
      <div className={styles.debugLink}>
        <button className={styles.clearButton} onClick={() => setData([])}>
          <FontAwesomeIcon icon={faBroom} /> Clear
        </button>
      </div>
      <div className={styles.debugLink}>
        <button className={styles.copyButton} onClick={copyDataToClipboard}>
          <FontAwesomeIcon icon={faCopy} /> Copy
        </button>
      </div>
    </>
  );

  return (
    <Page name="Live Debugging" desc="Live feed of debug data" icon={faBug} controls={controls}>
      {loading && <p>Listening for incoming data...</p>}
      {error && <p>Error: {error}</p>}
      <AutoScroll className={styles.autoScroll} height={document.body.scrollHeight - 260}>
        {filteredData.map((msg, index) => (
          <div className={styles.logLine} key={index}>
            {msg}
          </div>
        ))}
      </AutoScroll>
    </Page>
  );
}

export default PageLiveDebugging;
