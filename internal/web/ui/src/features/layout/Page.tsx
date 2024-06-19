import { FC, ReactNode } from 'react';
import { IconProp } from '@fortawesome/fontawesome-svg-core';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';

import styles from './Page.module.css';

export interface PageProps {
  name: string;
  desc: string;
  icon: IconProp;
  controls?: ReactNode;
  children?: ReactNode;
}

const Page: FC<PageProps> = (props) => {
  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div className={styles.icon}>
          <FontAwesomeIcon icon={props.icon} />
        </div>
        <div className={styles.info}>
          <h1>{props.name}</h1>
          <h2>{props.desc}</h2>
        </div>
        <div className={styles.controls}>{props.controls}</div>
      </header>
      <main>{props.children}</main>
    </div>
  );
};

export default Page;
