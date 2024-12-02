export interface FeedData {
  componentID: string;
  count: number;
  type: FeedDataType;
}

export enum FeedDataType {
  TARGET = 'target',
  PROMETHEUS_METRIC = 'prometheus_metric',
  LOKI_LOG = 'loki_log',
  OTEL_METRIC = 'otel_metric',
  OTEL_LOG = 'otel_log',
  OTEL_TRACE = 'otel_trace',
}
