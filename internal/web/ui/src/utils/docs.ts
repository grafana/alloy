/**
 * Builds the reference docs URL for a component. Component reference docs live
 * under a namespace subdirectory matching the component's prefix, e.g.
 * `prometheus.remote_write` is documented at
 * `reference/components/prometheus/prometheus.remote_write/`.
 */
export function componentDocsUrl(componentName: string): string {
  const namespace = componentName.split('.')[0];
  return `https://grafana.com/docs/alloy/latest/reference/components/${namespace}/${componentName}/`;
}
