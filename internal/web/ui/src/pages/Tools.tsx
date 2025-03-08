import { useState } from 'react';
import ReactJson from 'react-json-view';
import { faTools } from '@fortawesome/free-solid-svg-icons';

import Page from '../features/layout/Page';

import styles from './Tools.module.css';

function PrometheusTargetSearch() {
  const [searchText, setSearchText] = useState('');
  const [searchResults, setSearchResults] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);

  const handleSearch = async () => {
    try {
      setError(null);

      const url = new URL('/api/v0/web/tools/cluster-prom-targets-debug-info', window.location.origin);
      if (searchText) {
        url.searchParams.append('query', searchText);
      }

      const response = await fetch(url.toString(), {
        method: 'GET',
        headers: {
          Accept: 'application/json',
        },
      });

      if (!response.ok) {
        throw new Error(`HTTP error ${response.status}`);
      }

      // Store the actual data object rather than a string
      const data = await response.json();
      setSearchResults(data);
    } catch (error) {
      console.error('Error fetching search results:', error);
      setError(`Error searching for targets: ${(error as Error).message}`);
      setSearchResults(null);
    }
  };

  return (
    <section id="prometheus-search" className={styles.toolSection}>
      <h2>Cluster-wide Prometheus Target Status Search</h2>
      <div className={styles.content}>
        <div className={styles.searchControls}>
          <input
            type="text"
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            placeholder="Enter search query (regex supported)"
            className={styles.searchInput}
          />
          <button onClick={handleSearch} className={styles.searchButton}>
            Search Cluster
          </button>
        </div>

        <div className={styles.resultsContainer}>
          {error ? (
            <div className={styles.errorMessage}>{error}</div>
          ) : searchResults ? (
            <div className={styles.jsonViewWrapper}>
              <ReactJson
                src={searchResults}
                theme="rjv-default"
                collapsed={false}
                enableClipboard={false}
                displayDataTypes={false}
                name={false}
                style={{
                  fontFamily: "'Fira Code', monospace",
                  fontSize: '14px',
                  backgroundColor: '#f8f9fa',
                  padding: '10px',
                  borderRadius: '3px',
                  border: '1px solid #e4e5e6',
                  minHeight: '500px',
                }}
              />
            </div>
          ) : (
            <div className={styles.placeholderMessage}>Search results from all cluster instances will appear here...</div>
          )}
        </div>
      </div>
    </section>
  );
}

function PageTools() {
  return (
    <Page name="Tools" desc="Collection of useful tools" icon={faTools}>
      <div className={styles.toolsContainer}>
        <PrometheusTargetSearch />
      </div>
    </Page>
  );
}

export default PageTools;
