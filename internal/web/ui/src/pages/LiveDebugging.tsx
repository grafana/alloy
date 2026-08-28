import { faArrowDown, faBroom, faBug, faCopy, faRoad, faStop } from '@fortawesome/free-solid-svg-icons';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { useEffect, useId, useRef, useState } from 'react';
import { useParams } from 'react-router';

import SliderInput from '../components/SliderInput';
import Page from '../features/layout/Page';
import { useLiveDebugging } from '../hooks/liveDebugging';
import styles from './LiveDebugging.module.css';

const MIN_SAMPLE_RATE = 0;
const MAX_SAMPLE_RATE = 100;

function PageLiveDebugging() {
  const { '*': componentID } = useParams();
  const [enabled, setEnabled] = useState(true);
  const [data, setData] = useState<string[]>([]);
  const [sampleProb, setSampleProb] = useState(1);
  const [sliderProb, setSliderProb] = useState(100);
  const [filterValue, setFilterValue] = useState('');
  const [autoScroll, setAutoScroll] = useState(true);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const lastScrollTopRef = useRef<number>(0);
  const filterInputId = useId();
  const { loading, error } = useLiveDebugging(String(componentID), enabled, sampleProb, setData);

  const filteredData = data.filter((n) => n.toLowerCase().includes(filterValue.toLowerCase()));

  // Auto-scroll effect
  useEffect(() => {
    if (!autoScroll) {
      return;
    }

    const scrollToBottom = () => {
      if (scrollContainerRef.current) {
        scrollContainerRef.current.scrollTo({ top: scrollContainerRef.current.scrollHeight, behavior: 'smooth' });
      }
    };

    const interval = setInterval(scrollToBottom, 500);
    return () => clearInterval(interval);
  }, [autoScroll]);

  /**
   * Detect manual scroll to disable auto-scroll
   */
  const handleScroll = (event: React.UIEvent<HTMLDivElement>) => {
    if (!autoScroll) {
      return;
    }

    const currentScrollTop = event.currentTarget.scrollTop;
    const isScrollingUp = currentScrollTop < lastScrollTopRef.current;

    if (isScrollingUp) {
      setAutoScroll(false);
    }

    lastScrollTopRef.current = currentScrollTop;
  };

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

  function handleSampleChangeComplete(value: number) {
    setSampleProb(value / 100);
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
    <SliderInput
      label="Sample rate"
      min={MIN_SAMPLE_RATE}
      max={MAX_SAMPLE_RATE}
      value={sliderProb}
      defaultValue={MAX_SAMPLE_RATE}
      onChange={setSliderProb}
      onCommit={handleSampleChangeComplete}
    />
  );

  function handleFilterChange(event: React.ChangeEvent<HTMLInputElement>) {
    setFilterValue(event.target.value);
  }

  const filterControl = (
    <div className={styles.filter}>
      <label htmlFor={filterInputId}>Filter</label>
      <input id={filterInputId} type="text" placeholder="Filter data..." value={filterValue} onChange={handleFilterChange} />
    </div>
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
      <div className={styles.debugLink}>
        <button
          type="button"
          aria-pressed={autoScroll}
          className={`${styles.toggleButton} ${autoScroll ? styles.toggleButtonActive : ''}`}
          onClick={() => setAutoScroll((enabled) => !enabled)}
        >
          <FontAwesomeIcon icon={faArrowDown} /> Auto-scroll
        </button>
      </div>
    </>
  );

  return (
    <Page name="Live Debugging" desc="Live feed of debug data" icon={faBug} controls={controls}>
      <div
        ref={scrollContainerRef}
        onScroll={handleScroll}
        style={{
          height: '100%',
          overflowY: 'scroll',
        }}
      >
        {loading && <p>Listening for incoming data...</p>}
        {error && <p>Error: {error}</p>}
        {filteredData.map((msg, index) => (
          <div className={styles.logLine} key={index}>
            {msg}
          </div>
        ))}
      </div>
    </Page>
  );
}

export default PageLiveDebugging;
