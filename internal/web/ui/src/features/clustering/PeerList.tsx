import { PeerInfo } from '../clustering/types';

import Table from './Table';

import styles from './PeerList.module.css';

interface PeerListProps {
  peers: PeerInfo[];
}

const TABLEHEADERS = ['Node Name', 'Advertised Address', 'Current State', 'Local Node'];

const PeerList = ({ peers }: PeerListProps) => {
  const tableStyles = { width: '130px' };

  /**
   * Custom renderer for table data
   */
  const renderTableData = () => {
    return peers.map(({ name, addr, state, isSelf }) => (
      <tr key={name} style={{ lineHeight: '2.5' }}>
        <td>
          <span className={styles.idName}>{name}</span>
        </td>
        <td>
          <span className={styles.idName}>{addr}</span>
        </td>
        <td>
          <span className={styles.idName}>{state}</span>
        </td>
        <td>
          <span> {isSelf ? '✅' : ' '}</span>
        </td>
      </tr>
    ));
  };

  return (
    <div className={styles.list}>
      <Table tableHeaders={TABLEHEADERS} renderTableData={renderTableData} style={tableStyles} />
    </div>
  );
};

export default PeerList;
