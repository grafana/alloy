import { useState } from 'react';
import { NavLink } from 'react-router-dom';

import { HealthLabel } from '../component/HealthLabel';
import { ComponentDetail, ComponentInfo, SortOrder } from '../component/types';

import Table from './Table';

import styles from './ComponentList.module.css';
import foreachStyles from './ForeachList.module.css';

interface ComponentListProps {
  foreach: ComponentDetail;
  useRemotecfg: boolean;
  handleSorting?: (sortField: string, sortOrder: SortOrder) => void;
}

const ForeachList = ({ foreach, useRemotecfg, handleSorting }: ComponentListProps) => {
  const [expandedModules, setExpandedModules] = useState<Set<string>>(new Set());
  const tableStyles = { width: '130px' };
  const urlPrefix = useRemotecfg ? '/remotecfg' : '';

  // Group components by foreach children ids
  const moduleComponents = foreach.createdModuleIDs?.reduce((acc, moduleId) => {
    const components = foreach.moduleInfo?.filter((comp) => comp.moduleID === moduleId) || [];
    acc[moduleId] = components;
    return acc;
  }, {} as Record<string, ComponentInfo[]>);

  const toggleModule = (moduleId: string) => {
    setExpandedModules((prev) => {
      const next = new Set(prev);
      if (next.has(moduleId)) {
        next.delete(moduleId);
      } else {
        next.add(moduleId);
      }
      return next;
    });
  };

  const renderTableData = () => {
    if (!moduleComponents) return [];

    return Object.entries(moduleComponents).map(([moduleId, components]) => (
      <>
        <tr key={moduleId} onClick={() => toggleModule(moduleId)} className={foreachStyles.moduleRow}>
          <td colSpan={2}>
            <div className={foreachStyles.moduleHeader}>
              <span className={foreachStyles.expandIcon}>{expandedModules.has(moduleId) ? '▼' : '▶'}</span>
              <span className={foreachStyles.moduleId}>{moduleId.split('/').pop()}</span>
              <NavLink to={`/graph/${moduleId}`} className={styles.viewButton}>
                Graph
              </NavLink>
            </div>
          </td>
        </tr>
        {expandedModules.has(moduleId) &&
          components.map(({ health, localID, name }) => (
            <tr key={localID} className={foreachStyles.componentRow}>
              <td>
                <HealthLabel health={health.state} />
              </td>
              <td className={styles.idColumn}>
                <span className={styles.idName}>{name}</span>
                <NavLink to={`${urlPrefix}/component/${moduleId}/${localID}`} className={styles.viewButton}>
                  View
                </NavLink>
              </td>
            </tr>
          ))}
      </>
    ));
  };

  return (
    <div className={styles.list}>
      <Table tableHeaders={[]} renderTableData={renderTableData} handleSorting={handleSorting} style={tableStyles} />
    </div>
  );
};

export default ForeachList;
