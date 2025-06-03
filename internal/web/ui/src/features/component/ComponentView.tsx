import { FC, Fragment, ReactElement } from 'react';
import { Link } from 'react-router-dom';
import { useLocation } from 'react-router-dom';
import { faBug, faCubes, faDiagramProject, faLink } from '@fortawesome/free-solid-svg-icons';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';

import { partitionBody } from '../../utils/partition';

import ComponentBody from './ComponentBody';
import ComponentList from './ComponentList';
import ForeachList from './ForeachList';
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
  const liveDebuggingEnabled = props.component.liveDebuggingEnabled;

  const argsPartition = partitionBody(props.component.arguments, 'Arguments');
  const exportsPartition = props.component.exports && partitionBody(props.component.exports, 'Exports');
  const debugPartition = props.component.debugInfo && partitionBody(props.component.debugInfo, 'Debug info');
  const location = useLocation();
  const useRemotecfg = location.pathname.startsWith('/remotecfg');

  const isModule = props.component.moduleInfo && props.component.name !== 'foreach';
  const isForeach = props.component.moduleInfo && props.component.name === 'foreach';
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

  function liveDebuggingButton(): ReactElement | string {
    if (!liveDebuggingEnabled) {
      return 'Live debugging is not yet available for this component';
    }

    return (
      <div className={styles.debugLink}>
        <a href={`debug/${pathJoin([props.component.moduleID, props.component.localID])}`}>
          <FontAwesomeIcon icon={faBug} /> Live debugging
        </a>
      </div>
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
          <a href={`https://grafana.com/docs/alloy/latest/reference/components/${props.component.name}`}>
            <FontAwesomeIcon icon={faLink} /> Documentation
          </a>
        </div>

        {isModule && (
          <div className={styles.debugLink}>
            <a href={`graph/${pathJoin([props.component.moduleID, props.component.localID])}`}>
              <FontAwesomeIcon icon={faDiagramProject} /> Graph
            </a>
          </div>
        )}

        {liveDebuggingButton()}

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
              <ComponentList components={referencesTo} useRemotecfg={useRemotecfg} />
            </div>
          </section>
        )}

        {props.component.referencedBy.length > 0 && (
          <section id="dependants">
            <h2>Dependants</h2>
            <div className={styles.sectionContent}>
              <ComponentList components={referencedBy} useRemotecfg={useRemotecfg} />
            </div>
          </section>
        )}

        {isModule && props.component.moduleInfo && (
          <section id="module">
            <h2>Module components</h2>
            <div className={styles.sectionContent}>
              <ComponentList components={props.component.moduleInfo} useRemotecfg={useRemotecfg} />
            </div>
          </section>
        )}

        {isForeach && (
          <section id="foreach">
            <h2>Foreach components</h2>
            <div className={styles.sectionContent}>
              <ForeachList foreach={props.component} useRemotecfg={useRemotecfg} />
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
