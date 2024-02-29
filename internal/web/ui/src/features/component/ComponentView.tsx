import { FC, Fragment, ReactElement } from 'react';
import { Link } from 'react-router-dom';
import { faCubes, faLink } from '@fortawesome/free-solid-svg-icons';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';

import { partitionBody } from '../../utils/partition';

import ComponentBody from './ComponentBody';
import ComponentList from './ComponentList';
import { HealthLabel } from './HealthLabel';
import { ComponentDetail, ComponentInfo, PartitionedBody } from './types';

import styles from './ComponentView.module.css';

export interface ComponentViewProps {
  component: ComponentDetail;
  info: Record<string, ComponentInfo>;
}

export const ComponentView: FC<ComponentViewProps> = (props) => {
  // TODO(rfratto): expand/collapse icon for sections (treat it like Row in grafana dashboard)

  const referencedBy = props.component.referencedBy.filter((id) => props.info[id] !== undefined).map((id) => props.info[id]);
  const referencesTo = props.component.referencesTo.filter((id) => props.info[id] !== undefined).map((id) => props.info[id]);

  const argsPartition = partitionBody(props.component.arguments, 'Arguments');
  const exportsPartition = props.component.exports && partitionBody(props.component.exports, 'Exports');
  const debugPartition = props.component.debugInfo && partitionBody(props.component.debugInfo, 'Debug info');

  function partitionTOC(partition: PartitionedBody): ReactElement {
    return (
      <li>
        <Link to={'#' + partition.key.join('-')} target="_top">
          {partition.displayName[partition.displayName.length - 1]}
        </Link>
        {partition.inner.length > 0 && (
          <ul>
            {partition.inner.map((next, idx) => {
              return <Fragment key={idx.toString()}>{partitionTOC(next)}</Fragment>;
            })}
          </ul>
        )}
      </li>
    );
  }

  return (
    <div className={styles.page}>
      <nav>
        <h1>Sections</h1>
        <hr />
        <ul>
          <li>
            <Link to={'#' + props.component.localID} target="_top">
              {props.component.localID}
            </Link>
          </li>
          {argsPartition && partitionTOC(argsPartition)}
          {exportsPartition && partitionTOC(exportsPartition)}
          {debugPartition && partitionTOC(debugPartition)}
          {props.component.referencesTo.length > 0 && (
            <li>
              <Link to="#dependencies" target="_top">
                Dependencies
              </Link>
            </li>
          )}
          {props.component.referencedBy.length > 0 && (
            <li>
              <Link to="#dependants" target="_top">
                Dependants
              </Link>
            </li>
          )}
          {props.component.moduleInfo && (
            <li>
              <Link to="#module" target="_top">
                Module components
              </Link>
            </li>
          )}
        </ul>
      </nav>

      <main className={styles.content}>
        <h1 id={props.component.localID}>
          <span className={styles.icon}>
            <FontAwesomeIcon icon={faCubes} />
          </span>
          {props.component.localID}
          &nbsp; {/* space to separate the component name and label so double-click selections work */}
          <span className={styles.healthLabel}>
            <HealthLabel health={props.component.health.state} />
          </span>
        </h1>

        <div className={styles.docsLink}>
          <a href={`https://grafana.com/docs/agent/latest/flow/reference/components/${props.component.name}`}>
            Documentation <FontAwesomeIcon icon={faLink} />
          </a>
        </div>

        {props.component.health.message && (
          <blockquote>
            <h1>
              Latest health message{' '}
              {props.component.health.updatedTime && (
                <span className={styles.updateTime}>({props.component.health.updatedTime})</span>
              )}
            </h1>
            <p>{props.component.health.message}</p>
          </blockquote>
        )}

        <ComponentBody partition={argsPartition} />
        {exportsPartition && <ComponentBody partition={exportsPartition} />}
        {debugPartition && <ComponentBody partition={debugPartition} />}

        {props.component.referencesTo.length > 0 && (
          <section id="dependencies">
            <h2>Dependencies</h2>
            <div className={styles.sectionContent}>
              <ComponentList components={referencesTo} moduleID={props.component.moduleID} />
            </div>
          </section>
        )}

        {props.component.referencedBy.length > 0 && (
          <section id="dependants">
            <h2>Dependants</h2>
            <div className={styles.sectionContent}>
              <ComponentList components={referencedBy} moduleID={props.component.moduleID} />
            </div>
          </section>
        )}

        {props.component.moduleInfo && (
          <section id="module">
            <h2>Module components</h2>
            <div className={styles.sectionContent}>
              <ComponentList
                components={props.component.moduleInfo}
                moduleID={pathJoin([props.component.moduleID, props.component.localID])}
              />
            </div>
          </section>
        )}
      </main>
    </div>
  );
};

function pathJoin(paths: (string | undefined)[]): string {
  return paths.filter((p) => p && p !== '').join('/');
}
