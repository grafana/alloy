import { AttrStmt, Body as RiverBody } from '../river-js/types';

/**
 * ComponentInfo is high-level information for a component.
 */
export interface ComponentInfo {
  /** The moduleID that the component is defined in. moduleID may be the empty string. */
  moduleID: string;

  /** The id of the component uniquely identifies the component within a module. */
  localID: string;

  /**
   * The name of the component is the name of the block used to instantiate
   * the component. For example, the component ID
   * prometheus.remote_write.default would have a name of
   * "prometheus.remote_write".
   */
  name: string;

  /**
   * Label is an optional label for a component. Not all components may have
   * labels.
   *
   * For example, the prometheus.remote_write.default component would have a
   * label of "default".
   */
  label?: string;

  /**
   * Health information for a component. Components always have a health status
   * associated with them.
   */
  health: ComponentHealth;

  /**
   * IDs of components which are referencing this component.
   */
  referencedBy: string[];

  /**
   * IDs of components which this component is referencing.
   */
  referencesTo: string[];
}

/**
 * componentInfoByID partitions ComponentInfo by a component's ID.
 */
export function componentInfoByID(info: ComponentInfo[]): Record<string, ComponentInfo> {
  const res: Record<string, ComponentInfo> = {};
  info.forEach((elem) => {
    res[elem.localID] = elem;
  });
  return res;
}

/**
 * ComponentHealth represents the health of a specific component. A component's
 * health
 */
export interface ComponentHealth {
  /** Type of health. */
  state: ComponentHealthState;
  /** Message associated with health. */
  message?: string;
  /** Timestamp when health last changed. */
  updatedTime?: string;
}

/**
 * Known health states for a given component.
 */
export enum ComponentHealthState {
  HEALTHY = 'healthy',
  UNHEALTHY = 'unhealthy',
  UNKNOWN = 'unknown',
  EXITED = 'exited',
}

/*
 * ComponentDetail adds detailed information to ComponentInfo.
 */
export interface ComponentDetail extends ComponentInfo {
  /**
   * Arguments is the list of user-provided settings which configure an argument.
   * This is expected to be the *evaluated* arguments, not the raw expressions
   * a user may have used.
   */
  arguments: RiverBody;

  /**
   * Exports is the list of component-generated values which other components
   * can reference.
   */
  exports?: RiverBody;

  /**
   * Components may emit generic debug information, which would be contained
   * here.
   */
  debugInfo?: RiverBody;

  /**
   * If a component is a module loader, the IDs of modules it created are included here.
   */
  createdModuleIDs?: string[];

  /**
   * If a component is a module loader, the loaded components from the module are included here.
   */
  moduleInfo?: ComponentInfo[];
}

export interface PartitionedBody {
  /** key is a list of unique identifiers for this partitioned body. */
  key: string[];
  /** displayName is a list of friendly identifiers for this partitioned body. */
  displayName: string[];

  attrs: AttrStmt[];
  inner: PartitionedBody[];
}

/**
 * Sort order for component list
 */
export enum SortOrder {
  ASC = 'asc',
  DESC = 'desc',
}
