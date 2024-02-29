import { useState } from 'react';

import { SortOrder } from './types';

interface Props {
  headers: string[];
  handleSorting?: (header: string, sortOrder: SortOrder) => void;
}

const TableHead = ({ headers, handleSorting }: Props) => {
  const [sortField, setSortField] = useState('ID');
  const [order, setOrder] = useState(SortOrder.ASC);

  const handleSortingChange = (header: string) => {
    // User clicks on the new header, use default ASC sort order
    let sortOrder = SortOrder.ASC;

    // User clicks again on the header, we toggle the previous sort order
    if (header === sortField) {
      sortOrder = order === SortOrder.ASC ? SortOrder.DESC : SortOrder.ASC;
    }

    if (handleSorting !== undefined) {
      setSortField(header);
      setOrder(sortOrder);
      handleSorting(header, sortOrder);
    }
  };

  return (
    <tr>
      {headers.map((header) => {
        return (
          <th
            key={header}
            onClick={() => handleSortingChange(header)}
            data-sort-order={handleSorting && sortField === header ? order : undefined}
          >
            {header}
          </th>
        );
      })}
    </tr>
  );
};

export default TableHead;
