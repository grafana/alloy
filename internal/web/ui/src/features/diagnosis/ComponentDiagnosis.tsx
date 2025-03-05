import { DiagnosisInsight } from '../../hooks/diagnosis';

import Table from './Table';

import styles from './Table.module.css';

interface ComponentDiagnosisProps {
  insights: DiagnosisInsight[];
}

const ComponentDiagnosis = ({ insights }: ComponentDiagnosisProps) => {
  const getLevelColor = (level: string): string => {
    switch (level.toLowerCase()) {
      case 'error':
        return 'red';
      case 'warning':
        return 'orange';
      case 'info':
        return 'blue';
      default:
        return 'gray';
    }
  };

  const tableStyles = { width: '130px' };

  // Check if all modules are empty
  const allModulesEmpty = insights.every((insight) => !insight.module || insight.module.trim() === '');

  // Adjust table headers based on whether all modules are empty
  const tableHeaders = allModulesEmpty ? ['Level', 'Message', 'Link'] : ['Level', 'Message', 'Module', 'Link'];

  /**
   * Custom renderer for table data
   */
  const renderTableData = () => {
    // Sort insights by level priority: error > warning > info > others
    const sortedInsights = [...insights].sort((a, b) => {
      const levelPriority: Record<string, number> = {
        error: 0,
        warning: 1,
        info: 2,
      };

      const levelA = a.level.toLowerCase();
      const levelB = b.level.toLowerCase();

      return (
        (levelPriority[levelA] !== undefined ? levelPriority[levelA] : 3) -
        (levelPriority[levelB] !== undefined ? levelPriority[levelB] : 3)
      );
    });

    return sortedInsights.map(({ level, msg, module, link }) => {
      const displayModule = !module || module.trim() === '' ? '' : module;
      const truncatedModule = displayModule.length > 30 ? `${displayModule.substring(0, 27)}...` : displayModule;

      return (
        <tr key={`${displayModule}-${msg}`} style={{ lineHeight: '2.5' }}>
          <td>
            <span style={{ color: getLevelColor(level) }}>{level.toUpperCase()}</span>
          </td>
          <td>
            <span>{msg}</span>
          </td>
          {!allModulesEmpty && (
            <td>
              <span title={displayModule}>{truncatedModule}</span>
            </td>
          )}
          <td>
            {link ? (
              <a href={link} target="_blank" rel="noopener noreferrer">
                Learn more
              </a>
            ) : (
              <span></span>
            )}
          </td>
        </tr>
      );
    });
  };

  return (
    <div className={styles.list}>
      <Table tableHeaders={tableHeaders} renderTableData={renderTableData} style={tableStyles} />
    </div>
  );
};

export default ComponentDiagnosis;
