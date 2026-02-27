import { type TargetInfo } from './types';
import styles from './TargetsList.module.css';
import Table from './Table';

interface TargetsListProps {
  targets: TargetInfo[];
}

const TABLEHEADERS = ['Component', 'Job', 'URL', 'Health', 'Labels', 'Last Scrape', 'Duration', 'Error'];

const TargetsList = ({ targets }: TargetsListProps) => {
  const getHealthClass = (health: string) => {
    const healthLower = health.toLowerCase();
    if (healthLower === 'up') return `${styles.health} ${styles.up}`;
    if (healthLower === 'down') return `${styles.health} ${styles.down}`;
    return `${styles.health} ${styles.unknown}`;
  };

  const formatLabels = (labels: Record<string, string>) => {
    return Object.entries(labels).map(([key, value]) => (
      <span key={key} className={styles.label}>
        {key}="{value}"
      </span>
    ));
  };

  const formatLastScrape = (lastScrape: string) => {
    if (!lastScrape) return '-';
    try {
      const date = new Date(lastScrape);
      return date.toLocaleString();
    } catch {
      return lastScrape;
    }
  };

  /**
   * Custom renderer for table data
   */
  const renderTableData = () => {
    if (targets.length === 0) {
      return [
        <tr key="empty">
          <td colSpan={8} className={styles.emptyMessage}>
            No scrape targets found
          </td>
        </tr>,
      ];
    }

    return targets.map((target, index) => (
      <tr key={`${target.component_id}-${target.url}-${index}`}>
        <td>
          <span className={styles.componentId}>{target.component_id}</span>
        </td>
        <td>{target.job}</td>
        <td>
          <span className={styles.url}>{target.url}</span>
        </td>
        <td>
          <span className={getHealthClass(target.health)}>{target.health}</span>
        </td>
        <td>
          <div className={styles.labels}>{formatLabels(target.labels || {})}</div>
        </td>
        <td>{formatLastScrape(target.last_scrape)}</td>
        <td>
          <span className={styles.duration}>{target.last_scrape_duration || '-'}</span>
        </td>
        <td>
          {target.last_error && <span className={styles.error}>{target.last_error}</span>}
        </td>
      </tr>
    ));
  };

  return (
    <div className={styles.list}>
      <Table tableHeaders={TABLEHEADERS} renderTableData={renderTableData} />
    </div>
  );
};

export default TargetsList;
