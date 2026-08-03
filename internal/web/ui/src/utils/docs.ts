/**
 * Builds the reference docs URL for a component. Component reference docs live
 * under a namespace subdirectory matching the component's prefix, e.g.
 * `prometheus.remote_write` is documented at
 * `reference/components/prometheus/prometheus.remote_write/`.
 *
 * TODO: Link to the docs for the running Alloy version instead of `latest`.
 * The GraphQL API exposes a `version` field, but there is no UI-side plumbing
 * for it yet.
 */
export function componentDocsUrl(componentName: string): string {
  const namespace = componentName.split('.')[0];
  return `https://grafana.com/docs/alloy/latest/reference/components/${namespace}/${componentName}/`;
}
