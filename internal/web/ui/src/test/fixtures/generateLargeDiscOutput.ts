import { ValueType } from '../../features/alloy-syntax-js/types';

/**
 * Generates a single Kubernetes target object with the given index.
 * This mimics the structure of real discovery.kubernetes output.
 */
function generateTarget(index: number) {
  const ip = `192.0.2.${index % 256}`;
  const podName = `test-pod-${index}`;
  const nodeName = `test-node-${index % 10}`;
  const namespace = `test-namespace-${index % 5}`;
  const containerId = `${'0'.repeat(60)}${String(index).padStart(4, '0')}`;
  const uid = `00000000-0000-0000-0000-${String(index).padStart(12, '0')}`;

  return {
    type: ValueType.OBJECT,
    value: [
      { key: '__address__', value: { type: ValueType.STRING, value: `${ip}:9095` } },
      { key: '__meta_kubernetes_endpoint_address_target_kind', value: { type: ValueType.STRING, value: 'Pod' } },
      { key: '__meta_kubernetes_endpoint_address_target_name', value: { type: ValueType.STRING, value: podName } },
      { key: '__meta_kubernetes_endpoint_node_name', value: { type: ValueType.STRING, value: nodeName } },
      { key: '__meta_kubernetes_endpoint_port_name', value: { type: ValueType.STRING, value: 'grpc' } },
      { key: '__meta_kubernetes_endpoint_port_protocol', value: { type: ValueType.STRING, value: 'TCP' } },
      { key: '__meta_kubernetes_endpoint_ready', value: { type: ValueType.STRING, value: 'true' } },
      {
        key: '__meta_kubernetes_endpoints_annotation_endpoints_kubernetes_io_last_change_trigger_time',
        value: { type: ValueType.STRING, value: '2024-01-01T00:00:00Z' },
      },
      { key: '__meta_kubernetes_endpoints_name', value: { type: ValueType.STRING, value: `service-${index}` } },
      { key: '__meta_kubernetes_namespace', value: { type: ValueType.STRING, value: namespace } },
      {
        key: '__meta_kubernetes_pod_annotation_cni_projectcalico_org_containerID',
        value: { type: ValueType.STRING, value: containerId },
      },
      {
        key: '__meta_kubernetes_pod_annotation_cni_projectcalico_org_podIP',
        value: { type: ValueType.STRING, value: `${ip}/32` },
      },
      {
        key: '__meta_kubernetes_pod_container_image',
        value: { type: ValueType.STRING, value: 'docker.example.com/dummy-org/dummy-image:v1.0.0' },
      },
      { key: '__meta_kubernetes_pod_container_init', value: { type: ValueType.STRING, value: 'false' } },
      { key: '__meta_kubernetes_pod_container_name', value: { type: ValueType.STRING, value: 'main-container' } },
      { key: '__meta_kubernetes_pod_container_port_name', value: { type: ValueType.STRING, value: 'grpc' } },
      { key: '__meta_kubernetes_pod_container_port_number', value: { type: ValueType.STRING, value: '9095' } },
      { key: '__meta_kubernetes_pod_container_port_protocol', value: { type: ValueType.STRING, value: 'TCP' } },
      { key: '__meta_kubernetes_pod_controller_kind', value: { type: ValueType.STRING, value: 'ReplicaSet' } },
      {
        key: '__meta_kubernetes_pod_controller_name',
        value: { type: ValueType.STRING, value: `deployment-${index}-abc12345` },
      },
      { key: '__meta_kubernetes_pod_host_ip', value: { type: ValueType.STRING, value: `10.0.0.${index % 256}` } },
      { key: '__meta_kubernetes_pod_ip', value: { type: ValueType.STRING, value: ip } },
      { key: '__meta_kubernetes_pod_label_name', value: { type: ValueType.STRING, value: `app-${index}` } },
      { key: '__meta_kubernetes_pod_name', value: { type: ValueType.STRING, value: podName } },
      { key: '__meta_kubernetes_pod_node_name', value: { type: ValueType.STRING, value: nodeName } },
      { key: '__meta_kubernetes_pod_phase', value: { type: ValueType.STRING, value: 'Running' } },
      { key: '__meta_kubernetes_pod_ready', value: { type: ValueType.STRING, value: 'true' } },
      { key: '__meta_kubernetes_pod_uid', value: { type: ValueType.STRING, value: uid } },
      { key: '__meta_kubernetes_service_label_name', value: { type: ValueType.STRING, value: `service-${index}` } },
      { key: '__meta_kubernetes_service_name', value: { type: ValueType.STRING, value: `service-${index}` } },
    ],
  };
}

/**
 * Generates a discovery.kubernetes output with the specified number of targets and label.
 */
export function generateDiscOutput(label: string, targetCount: number) {
  const targets = [];
  for (let i = 0; i < targetCount; i++) {
    targets.push(generateTarget(i));
  }

  return {
    name: 'discovery.kubernetes',
    type: 'block',
    localID: `discovery.kubernetes.${label}`,
    moduleID: '',
    label: label,
    referencesTo: [],
    referencedBy: [`discovery.relabel.${label}`],
    dataFlowEdgesTo: [`discovery.relabel.${label}`],
    health: {
      state: 'healthy',
      message: 'started component',
      updatedTime: '2024-01-01T00:00:00Z',
    },
    original: '',
    arguments: [
      {
        name: 'role',
        type: 'attr',
        value: { type: ValueType.STRING, value: 'endpoints' },
      },
    ],
    exports: [
      {
        name: 'targets',
        type: 'attr',
        value: {
          type: ValueType.ARRAY,
          value: targets,
        },
      },
    ],
  };
}

// Heavy fixture - 50000 targets, requires download button
export const heavyDiscOutput = generateDiscOutput('heavy', 50000);

// Light fixture - 3 targets, renders inline without download button
export const lightDiscOutput = generateDiscOutput('light', 3);

// Backwards compatibility alias
export const largeDiscOutput = heavyDiscOutput;
export function generateLargeDiscOutput(targetCount = 20000) {
  return generateDiscOutput('heavy', targetCount);
}
