export interface DebugData {
  componentID: string;
  targetComponentIDs: Array<string>;
  rate: number;
  type: DebugDataType;
}

export enum DebugDataType {
  UNDEFINED = 'undefined',
  TARGET = 'target',
  PROMETHEUS_METRIC = 'prometheus_metric',
  LOKI_LOG = 'loki_log',
  OTEL_METRIC = 'otel_metric',
  OTEL_LOG = 'otel_log',
  OTEL_TRACE = 'otel_trace',
}
