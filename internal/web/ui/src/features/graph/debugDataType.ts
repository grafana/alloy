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

export const DebugDataTypeColorMap: Record<DebugDataType, string> = {
  [DebugDataType.UNDEFINED]: '#000000', // Black
  [DebugDataType.TARGET]: '#0072B2', // Blue
  [DebugDataType.PROMETHEUS_METRIC]: '#D55E00', // Orange
  [DebugDataType.LOKI_LOG]: '#FFC0CB', // Pink
  [DebugDataType.OTEL_METRIC]: '#F39C12', // Yellow
  [DebugDataType.OTEL_LOG]: '#009E73', // Green
  [DebugDataType.OTEL_TRACE]: '#56B4E9', // Light Blue
};
