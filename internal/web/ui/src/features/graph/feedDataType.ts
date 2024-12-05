export interface FeedData {
  componentID: string;
  targetComponentIDs: Array<string>;
  count: number;
  type: FeedDataType;
}

export enum FeedDataType {
  UNDEFINED = 'undefined',
  TARGET = 'target',
  PROMETHEUS_METRIC = 'prometheus_metric',
  LOKI_LOG = 'loki_log',
  OTEL_METRIC = 'otel_metric',
  OTEL_LOG = 'otel_log',
  OTEL_TRACE = 'otel_trace',
}

export const FeedDataTypeColorMap: Record<FeedDataType, string> = {
  [FeedDataType.UNDEFINED]: '#000000', // Black
  [FeedDataType.TARGET]: '#0072B2', // Blue
  [FeedDataType.PROMETHEUS_METRIC]: '#D55E00', // Orange
  [FeedDataType.LOKI_LOG]: '#FFC0CB', // Pink
  [FeedDataType.OTEL_METRIC]: '#F39C12', // Yellow
  [FeedDataType.OTEL_LOG]: '#009E73', // Green
  [FeedDataType.OTEL_TRACE]: '#56B4E9', // Light Blue
};
