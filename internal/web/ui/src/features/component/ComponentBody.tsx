import React, { Fragment } from 'react';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';

import { riverStringify } from '../river-js/stringify';

import { style } from './style';
import Table from './Table';
import { PartitionedBody } from './types';

import styles from './ComponentView.module.css';

interface ComponentBodyProps {
  partition: PartitionedBody;
}

const TABLEHEADERS = ['Name', 'Value'];

const ComponentBody = ({ partition }: ComponentBodyProps) => {
  const sectionClass = partition.key.length === 1 ? '' : styles.nested;

  const renderTableData = () => {
    return partition.attrs.map(({ name, value }, index) => {
      return (
        <tr key={name}>
          <td className={styles.nameColumn}>{name}</td>
          <td>
            <pre className={styles.pre}>
              <SyntaxHighlighter language="javascript" style={style}>
                {riverStringify(value)}
              </SyntaxHighlighter>
            </pre>
          </td>
        </tr>
      );
    });
  };

  return (
    <>
      <section id={partition.key.join('-')} className={sectionClass}>
        {
          // If the partition only has 1 key, then make it an h2.
          // Otherwise, make it an h3.
          partition.displayName.length === 1 ? (
            <h2>{partition.displayName}</h2>
          ) : (
            <h3>
              {partition.displayName.map((val, idx) => {
                return (
                  <Fragment key={val}>
                    <span>{val}</span>
                    {idx + 1 < partition.key.length && <span> / </span>}
                  </Fragment>
                );
              })}
            </h3>
          )
        }
        <div className={styles.sectionContent}>
          {partition.attrs.length === 0 ? (
            <em className={styles.informative}>(No set attributes in this block)</em>
          ) : (
            <div className={styles.list}>
              <Table tableHeaders={TABLEHEADERS} renderTableData={renderTableData} style={{ width: '210px' }} />
            </div>
          )}
        </div>
      </section>
      {partition.inner.map((body) => {
        return <ComponentBody key={body.key.join('.')} partition={body} />;
      })}
    </>
  );
};

export default ComponentBody;
