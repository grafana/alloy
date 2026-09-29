"""Vanilla Target Allocator manifests (JSON is also valid YAML)."""

import hashlib
import json


def manifests(settings, namespace, release):
    name = "target-allocator"
    labels = {"app.kubernetes.io/name": name}
    config = {
        "collector_namespace": namespace,
        "collector_selector": {"matchLabels": {
            "app.kubernetes.io/name": "alloy", "app.kubernetes.io/instance": release}},
        "allocation_strategy": settings["ta_strategy"],
        "filter_strategy": "relabel-config",
        "prometheus_cr": {"enabled": False},
        "listen_addr": ":8080",
        "config": {"scrape_configs": [{
            "job_name": "target-allocation",
            "scrape_interval": f"{settings['scrape_interval_seconds']}s",
            "scrape_timeout": f"{min(10, settings['scrape_interval_seconds'])}s",
            "kubernetes_sd_configs": [{
                "role": "pod", "namespaces": {"names": [namespace]},
                "selectors": [{"role": "pod", "label": "app.kubernetes.io/part-of=target-allocation-targets"}],
            }],
            "relabel_configs": [
                {"source_labels": ["__meta_kubernetes_pod_ready", "__meta_kubernetes_pod_container_port_name"],
                 "regex": "true;metrics", "action": "keep"},
                {"source_labels": ["__meta_kubernetes_pod_name"], "target_label": "instance"},
                {"source_labels": ["__meta_kubernetes_pod_label_size_class"], "target_label": "size_class"},
            ],
        }]},
    }
    content = json.dumps(config, indent=2) + "\n"

    def resource(api, kind, **fields):
        return {"apiVersion": api, "kind": kind,
                "metadata": {"name": name, "namespace": namespace}, **fields}

    return {"apiVersion": "v1", "kind": "List", "items": [
        resource("v1", "ServiceAccount"),
        resource("rbac.authorization.k8s.io/v1", "Role", rules=[{
            "apiGroups": [""], "resources": ["pods"], "verbs": ["get", "list", "watch"]}]),
        resource("rbac.authorization.k8s.io/v1", "RoleBinding",
                 roleRef={"apiGroup": "rbac.authorization.k8s.io", "kind": "Role", "name": name},
                 subjects=[{"kind": "ServiceAccount", "name": name, "namespace": namespace}]),
        resource("v1", "ConfigMap", data={"targetallocator.yaml": content}),
        resource("v1", "Service", spec={"selector": labels,
                 "ports": [{"name": "http", "port": 8080, "targetPort": "http"}]}),
        resource("apps/v1", "Deployment", spec={
            "replicas": 1, "strategy": {"type": "Recreate"},
            "selector": {"matchLabels": labels},
            "template": {
                "metadata": {"labels": labels, "annotations": {
                    "checksum/config": hashlib.sha256(content.encode()).hexdigest()}},
                "spec": {
                    "serviceAccountName": name,
                    "containers": [{
                        "name": name, "image": settings["ta_image"],
                        "ports": [{"name": "http", "containerPort": 8080}],
                        "resources": settings["ta_resources"],
                        "readinessProbe": {"httpGet": {"path": "/readyz", "port": "http"}},
                        "livenessProbe": {"httpGet": {"path": "/livez", "port": "http"}, "initialDelaySeconds": 10},
                        "volumeMounts": [{"name": "config", "mountPath": "/conf", "readOnly": True}],
                    }],
                    "volumes": [{"name": "config", "configMap": {"name": name}}],
                },
            },
        }),
    ]}
