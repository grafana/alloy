import { NavLink } from 'react-router';

import { HealthLabel } from '../component/HealthLabel';
import { type ComponentInfo, SortOrder } from '../component/types';
import styles from './ComponentList.module.css';
import Table from './Table';

interface ComponentListProps {
  components: ComponentInfo[];
  overrideModuleID?: string;
  useRemotecfg: boolean;
  handleSorting?: (sortField: string, sortOrder: SortOrder) => void;
}

const TABLEHEADERS = ['Health', 'ID'];

// overrideModuleID is a workaround for the remote config page because the remotecfg component has the moduleID of its controller,
// it should not be fetched as a module.
const ComponentList = ({ components, overrideModuleID, useRemotecfg, handleSorting }: ComponentListProps) => {
  const tableStyles = { width: '130px' };
  const urlPrefix = useRemotecfg ? '/remotecfg' : '';
  /**
   * Custom renderer for table data
   */
  const renderTableData = () => {
    return components.map(({ health, localID: id, moduleID }) => (
      <tr key={id}>
        <td>
          <HealthLabel health={health.state} />
        </td>
        <td className={styles.idColumn}>
          <span className={styles.idName}>{id}</span>
          <NavLink
            to={
              urlPrefix +
              '/component/' +
              (overrideModuleID !== undefined ? overrideModuleID : moduleID ? moduleID + '/' : '') +
              id
            }
            className={styles.viewButton}
          >
            View
          </NavLink>
        </td>
      </tr>
    ));
  };

  return (
    <div className={styles.list}>
      <Table
        tableHeaders={TABLEHEADERS}
        renderTableData={renderTableData}
        handleSorting={handleSorting}
        style={tableStyles}
      />
    </div>
  );
};

export default ComponentList;
