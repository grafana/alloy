#!/usr/bin/env python3
"""Render and deploy the kind scrape baseline with only Python's standard library."""

import argparse
import base64
import json
import os
from pathlib import Path
import re
import shlex
import subprocess


HERE = Path(__file__).resolve().parent
KIND_ROOT = HERE.parents[1]
REPO_ROOT = KIND_ROOT.parents[1]
BUILD = KIND_ROOT / "build/target-allocation"
NAMESPACE = "target-allocation"
RELEASE = "alloy-target-allocation"
TARGET_LABELS = {"app.kubernetes.io/part-of": "target-allocation-targets"}
SECRET_KEYS = (
    "GRAFANA_CLOUD_PROMETHEUS_URL",
    "GRAFANA_CLOUD_PROMETHEUS_USERNAME",
    "GRAFANA_CLOUD_METRICS_WRITE_TOKEN",
)


def load_settings(path):
    settings = json.loads(path.read_text())
    for key in ("alloy_replicas", "scrape_interval_seconds", "large_targets",
                "large_series", "small_targets", "small_series"):
        if type(settings.get(key)) is not int or settings[key] < 1:
            raise ValueError(f"{key} must be a positive integer")
    if not re.fullmatch(r"[a-zA-Z0-9_-]+", settings.get("run_id", "")):
        raise ValueError("run_id must contain letters, numbers, underscores or hyphens")
    repository, separator, tag = settings.get("alloy_image", "").rpartition(":")
    if not separator or not repository or not tag or tag == "latest":
        raise ValueError("alloy_image must have an explicit version tag")
    return settings


def workload(settings, size):
    labels = {**TARGET_LABELS, "size_class": size}
    return {
        "apiVersion": "apps/v1", "kind": "StatefulSet",
        "metadata": {"name": f"targets-{size}", "namespace": NAMESPACE},
        "spec": {
            "serviceName": "targets", "podManagementPolicy": "Parallel",
            "replicas": settings[f"{size}_targets"],
            "selector": {"matchLabels": labels},
            "template": {
                "metadata": {"labels": labels},
                "spec": {"containers": [{
                    "name": "prom-gen", "image": "prom-gen:latest",
                    "imagePullPolicy": "Never",
                    "command": ["/app/main"],
                    "args": [f"--series={settings[f'{size}_series']}"],
                    "ports": [{"name": "metrics", "containerPort": 9001}],
                    "readinessProbe": {"httpGet": {"path": "/metrics", "port": "metrics"}},
                    "resources": {"requests": {"cpu": "10m", "memory": "16Mi"}},
                }]},
            },
        },
    }


def render(settings):
    BUILD.mkdir(parents=True, exist_ok=True)
    image_repo, image_tag = settings["alloy_image"].rsplit(":", 1)
    values = {
        "fullnameOverride": RELEASE,
        "image": {"repository": image_repo, "tag": image_tag},
        "crds": {"create": False},
        "controller": {"type": "statefulset", "replicas": settings["alloy_replicas"]},
        "alloy": {
            "enableReporting": False,
            "clustering": {"enabled": True, "name": "target-allocation"},
            "configMap": {"content": (HERE / "config.alloy").read_text()},
            "extraArgs": ["--cluster.node-name=$(POD_NAME)"],
            "extraEnv": [
                {"name": "POD_NAME", "valueFrom": {"fieldRef": {"fieldPath": "metadata.name"}}},
                {"name": "POD_NAMESPACE", "valueFrom": {"fieldRef": {"fieldPath": "metadata.namespace"}}},
                {"name": "DEMO_RUN_ID", "value": settings["run_id"]},
                {"name": "DEMO_SCRAPE_INTERVAL", "value": f"{settings['scrape_interval_seconds']}s"},
            ],
            "envFrom": [{"secretRef": {"name": "grafana-cloud-metrics"}}],
            "resources": {"requests": {"cpu": "100m", "memory": "256Mi"}},
        },
        "rbac": {"namespaces": [NAMESPACE]},
    }
    service = {
        "apiVersion": "v1", "kind": "Service",
        "metadata": {"name": "targets", "namespace": NAMESPACE},
        "spec": {"clusterIP": "None", "selector": TARGET_LABELS,
                 "ports": [{"name": "metrics", "port": 9001, "targetPort": "metrics"}]},
    }
    targets = {"apiVersion": "v1", "kind": "List",
               "items": [service, workload(settings, "large"), workload(settings, "small")]}
    for name, data in (("alloy-values.json", values), ("targets.json", targets),
                       ("settings.json", settings)):
        (BUILD / name).write_text(json.dumps(data, indent=2) + "\n")
    count = settings["large_targets"] * settings["large_series"] + settings["small_targets"] * settings["small_series"]
    print(f"Workload: {count:,} series, {count / settings['scrape_interval_seconds']:.1f} samples/s, plus monitoring overhead")
    print(f"Rendered non-secret files into {BUILD}", flush=True)


def credentials():
    # Parse dotenv values as data. Never execute the credentials file as shell code.
    values = {}
    path = KIND_ROOT / ".env.credentials"
    if path.exists():
        for line in path.read_text().splitlines():
            if not line.strip() or line.lstrip().startswith("#"):
                continue
            key, separator, value = line.partition("=")
            key = key.strip().removeprefix("export ")
            if separator and key in SECRET_KEYS:
                parts = shlex.split(value, comments=True)
                if len(parts) != 1:
                    raise ValueError(f"Set a single value for {key} in .env.credentials")
                values[key] = parts[0]
    for key in SECRET_KEYS:
        values[key] = os.environ.get(key) or values.get(key)
        if not values[key]:
            raise ValueError(f"Missing {key}; set it in example/kind/.env.credentials")
    if not values[SECRET_KEYS[0]].startswith("https://"):
        raise ValueError("The Grafana Cloud remote-write URL must use HTTPS")
    return values


def run(args, **kwargs):
    return subprocess.run([str(arg) for arg in args], check=True, text=True, **kwargs)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("render", "up", "down", "status"))
    parser.add_argument("--settings", type=Path, default=HERE / "settings.json")
    parser.add_argument("--kubeconfig", type=Path, default=KIND_ROOT / "build/kubeconfig.yaml")
    parser.add_argument("--cluster", default="alloy-example")
    args = parser.parse_args()
    if args.action in ("render", "up"):
        settings = load_settings(args.settings)
        render(settings)
    if args.action == "render":
        return
    kube = ["kubectl", "--kubeconfig", args.kubeconfig, "--context", f"kind-{args.cluster}"]
    helm = ["helm", "--kubeconfig", args.kubeconfig, "--kube-context", f"kind-{args.cluster}"]
    if args.action == "up":
        values = credentials()
        namespace = {"apiVersion": "v1", "kind": "Namespace", "metadata": {"name": NAMESPACE}}
        run(kube + ["apply", "-f", "-"], input=json.dumps(namespace))
        secret = {
            "apiVersion": "v1", "kind": "Secret", "type": "Opaque",
            "metadata": {"name": "grafana-cloud-metrics", "namespace": NAMESPACE},
            "data": {key: base64.b64encode(value.encode()).decode() for key, value in values.items()},
        }
        # stdin keeps credentials out of command arguments and generated files.
        # Server-side apply also avoids a last-applied annotation containing secrets.
        run(kube + ["apply", "--server-side", "--field-manager=target-allocation-demo", "-f", "-"], input=json.dumps(secret))
        run(kube + ["apply", "-f", BUILD / "targets.json"])
        run(helm + ["upgrade", "--install", RELEASE, REPO_ROOT / "operations/helm/charts/alloy",
                    "--namespace", NAMESPACE, "--values", BUILD / "alloy-values.json", "--wait", "--timeout", "5m"])
        # New pods pick up secret changes even if the Helm values are unchanged.
        run(kube + ["-n", NAMESPACE, "rollout", "restart", f"statefulset/{RELEASE}"])
        for name in ("targets-large", "targets-small", RELEASE):
            run(kube + ["-n", NAMESPACE, "rollout", "status", f"statefulset/{name}", "--timeout=300s"])
        print("Baseline ready. Allow at least 5 minutes of warm-up before comparing load.")
    elif args.action == "down":
        run(helm + ["uninstall", RELEASE, "-n", NAMESPACE, "--ignore-not-found", "--wait"])
        run(kube + ["delete", "namespace", NAMESPACE, "--ignore-not-found", "--wait=true"])
    else:
        run(kube + ["-n", NAMESPACE, "get", "pods", "-o", "wide"])


if __name__ == "__main__":
    try:
        main()
    except (ValueError, subprocess.CalledProcessError) as error:
        raise SystemExit(str(error)) from None
