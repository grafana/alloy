/**
 * Types related to Prometheus target search functionality.
 * These types match the API response structure defined in the backend.
 */

/**
 * Represents a single Prometheus target with its details.
 * Matches the Go struct in internal/web/api/tools.go
 */
export interface PrometheusTarget {
  instance: string;
  componentID: string;
  matchingArgs: Record<string, string>[];
  matchingExports: Record<string, string>[];
}

/**
 * Response structure for the Prometheus target search API.
 * Matches the Go struct in internal/web/api/tools.go
 */
export interface PrometheusTargetSearchResponse {
  results: Record<string, InstanceResults>;
  errors: string[];
}

export interface InstanceResults {
  components: Record<string, TargetResults>;
}

export interface TargetResults {
  args: Record<string, string>[];
  exports: Record<string, string>[];
}
