/**
 * Types related to Prometheus target search functionality.
 * These types match the API response structure defined in the backend.
 */

/**
 * Represents a single Prometheus target with its details.
 * Matches the Go struct in internal/web/api/api.go
 */
export interface PrometheusTarget {
  instance: string;
  componentID: string;
  labels: Record<string, string>;
  debugInfo: Record<string, string>;
}

/**
 * Response structure for the Prometheus target search API.
 */
export interface PrometheusTargetSearchResponse {
  targets: PrometheusTarget[];
}
