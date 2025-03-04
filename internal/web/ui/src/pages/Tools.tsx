import { useState } from 'react';
import { faTools } from '@fortawesome/free-solid-svg-icons';

import Page from '../features/layout/Page';
import { PrometheusTarget, PrometheusTargetSearchResponse } from '../types/prometheusTypes';

import styles from './Tools.module.css';

function PrometheusTargetSearch() {
  const [searchText, setSearchText] = useState('');
  const [searchResults, setSearchResults] = useState('');

  const handleSearch = async () => {
    try {
      // Make the actual API request to our new endpoint
      const response = await fetch('/api/v0/web/tools/prometheus-target-search', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ searchQuery: searchText }),
      });

      if (!response.ok) {
        throw new Error(`HTTP error ${response.status}`);
      }

      const data = (await response.json()) as PrometheusTargetSearchResponse;

      // Format the results
      if (data.targets && data.targets.length > 0) {
        // Format target data if we have results
        const formattedResults = data.targets
          .map((target: PrometheusTarget) => {
            const labelString = Object.entries(target.labels || {})
              .map(([key, value]) => `${key}=${value}`)
              .join(', ');

            const debugInfoString = Object.entries(target.debugInfo || {})
              .map(([key, value]) => `  ${key}: ${value}`)
              .join('\n');

            return `Instance: ${target.instance}\nComponent: ${target.componentID}\nLabels: ${labelString}\nDebug Info:\n${debugInfoString}`;
          })
          .join('\n\n');

        setSearchResults(formattedResults);
      } else {
        // No results found
        setSearchResults(`No targets found matching: "${searchText}"`);
      }
    } catch (error) {
      console.error('Error fetching search results:', error);
      setSearchResults(`Error searching for targets: ${(error as Error).message}`);
    }
  };

  return (
    <section id="prometheus-search" className={styles.toolSection}>
      <h2>Prometheus Target Search</h2>
      <div className={styles.content}>
        <div className={styles.searchControls}>
          <input
            type="text"
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            placeholder="Enter search query..."
            className={styles.searchInput}
          />
          <button onClick={handleSearch} className={styles.searchButton}>
            Search
          </button>
        </div>

        <div className={styles.resultsContainer}>
          <textarea
            readOnly
            value={searchResults}
            className={styles.resultsArea}
            placeholder="Search results will appear here..."
            rows={10}
          />
        </div>
      </div>
    </section>
  );
}

function ClusterComponentHealth() {
  return (
    <section id="cluster-health" className={styles.toolSection}>
      <h2>Cluster-wide Component Health</h2>
      <div className={styles.content}>
        <p>Placeholder for cluster-wide component health monitoring.</p>
        {/* Add cluster health monitoring UI elements here */}
      </div>
    </section>
  );
}

function PageTools() {
  return (
    <Page name="Tools" desc="Collection of useful tools" icon={faTools}>
      <div className={styles.toolsContainer}>
        <PrometheusTargetSearch />
        <ClusterComponentHealth />
      </div>
    </Page>
  );
}

export default PageTools;
