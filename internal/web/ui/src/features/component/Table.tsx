import React from 'react';

import TableHead from './TableHead';
import { SortOrder } from './types';

import styles from './Table.module.css';

interface Props {
  tableHeaders: string[];
  style?: React.CSSProperties;
  handleSorting?: (sortField: string, sortOrder: SortOrder) => void;
  renderTableData: () => JSX.Element[];
}

/**
 * Simple table component that accept a custom header, custom sorting function and custom render
 * function for the table data
 */
const Table = ({ tableHeaders, style = {}, handleSorting, renderTableData }: Props) => {
  return (
    <table className={styles.table}>
      <colgroup span={1} style={style} />
      <tbody>
        <TableHead headers={tableHeaders} handleSorting={handleSorting} />
        {renderTableData()}
      </tbody>
    </table>
  );
};

export default Table;
