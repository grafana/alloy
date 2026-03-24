import { DebugDataTypeColorMap } from './debugDataType';
import styles from './Legend.module.css';

export const Legend: React.FC = () => {
  return (
    <div className={styles.legend}>
      {Object.entries(DebugDataTypeColorMap)
        .filter(([key]) => key !== 'undefined')
        .map(([key, color]) => (
          <div key={key} className={styles.legendItem}>
            <div className={styles.colorBox} style={{ backgroundColor: color }}></div>
            {key}
          </div>
        ))}
    </div>
  );
};
