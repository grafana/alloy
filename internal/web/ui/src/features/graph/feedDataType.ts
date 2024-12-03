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
  [FeedDataType.UNDEFINED]: '#000000', // Dark green
  [FeedDataType.TARGET]: '#117733', // Dark green
  [FeedDataType.PROMETHEUS_METRIC]: '#44AA99', // Teal
  [FeedDataType.LOKI_LOG]: '#88CCEE', // Sky blue
  [FeedDataType.OTEL_METRIC]: '#DDCC77', // Sandy yellow
  [FeedDataType.OTEL_LOG]: '#CC6677', // Rose
  [FeedDataType.OTEL_TRACE]: '#AA4499', // Purple
};
