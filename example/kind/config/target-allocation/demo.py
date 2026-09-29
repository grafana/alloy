#!/usr/bin/env python3
"""Render, deploy, and switch the kind allocation demo using the standard library."""

import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import subprocess

import allocator


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
                "large_series", "small_targets", "small_series", "http_refresh_interval_seconds"):
        if type(settings.get(key)) is not int or settings[key] < 1:
            raise ValueError(f"{key} must be a positive integer")
    if not re.fullmatch(r"[a-zA-Z0-9_-]+", settings.get("run_id", "")):
        raise ValueError("run_id must contain letters, numbers, underscores or hyphens")
    repository, separator, tag = settings.get("alloy_image", "").rpartition(":")
    if not separator or not repository or not tag or tag == "latest":
        raise ValueError("alloy_image must have an explicit version tag")
    if settings.get("discovery_mode") not in ("alloy", "ta"):
        raise ValueError("discovery_mode must be alloy or ta")
    if settings.get("ta_strategy") not in ("consistent-hashing", "least-weighted"):
        raise ValueError("ta_strategy must be consistent-hashing or least-weighted")
    policy = settings.get("ta_image_pull_policy", "IfNotPresent")
    image = settings.get("ta_image", "")
    local = policy == "Never" and re.fullmatch(r"target-allocator-local:[0-9a-f]{12}-[0-9a-f]{16}", image)
    if policy not in ("Never", "IfNotPresent") or not (local or re.fullmatch(r".+@sha256:[0-9a-f]{64}", image)):
        raise ValueError("ta_image requires a digest, or a local build tag with pull policy Never")
    for key in ("alloy_resources", "ta_resources"):
        if not isinstance(settings.get(key), dict):
            raise ValueError(f"{key} must be a Kubernetes resources object")
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
    ta_enabled = settings["discovery_mode"] == "ta"
    config_file = "config-ta.alloy" if ta_enabled else "config.alloy"
    config = (HERE / config_file).read_text() + "\n" + (HERE / "common.alloy").read_text()
    (BUILD / "config.alloy").write_text(config)
    image_repo, image_tag = settings["alloy_image"].rsplit(":", 1)
    values = {
        "fullnameOverride": RELEASE,
        "image": {"repository": image_repo, "tag": image_tag},
        "crds": {"create": False},
        "controller": {"type": "statefulset", "replicas": settings["alloy_replicas"]},
        "alloy": {
            "enableReporting": False,
            "clustering": {"enabled": not ta_enabled, "name": "target-allocation"},
            "configMap": {"content": config},
            "extraArgs": [] if ta_enabled else ["--cluster.node-name=$(POD_NAME)"],
            "extraEnv": [
                {"name": "POD_NAME", "valueFrom": {"fieldRef": {"fieldPath": "metadata.name"}}},
                {"name": "POD_NAMESPACE", "valueFrom": {"fieldRef": {"fieldPath": "metadata.namespace"}}},
                {"name": "DEMO_HTTP_REFRESH_INTERVAL", "value": f"{settings['http_refresh_interval_seconds']}s"},
                {"name": "DEMO_RUN_ID", "value": settings["run_id"]},
                {"name": "DEMO_SCRAPE_INTERVAL", "value": f"{settings['scrape_interval_seconds']}s"},
            ],
            "envFrom": [{"secretRef": {"name": "grafana-cloud-metrics"}}],
            "resources": settings["alloy_resources"],
        },
        "rbac": {"create": not ta_enabled, "namespaces": [NAMESPACE]},
        "serviceAccount": {"automountServiceAccountToken": not ta_enabled},
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
                       ("settings.json", settings),
                       ("allocator.json", allocator.manifests(settings, NAMESPACE, RELEASE))):
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
    parser.add_argument("action", choices=("render", "up", "switch", "down", "status"))
    parser.add_argument("--settings", type=Path, default=HERE / "settings.json")
    parser.add_argument("--kubeconfig", type=Path, default=KIND_ROOT / "build/kubeconfig.yaml")
    parser.add_argument("--cluster", default="alloy-example")
    parser.add_argument("--mode", choices=("alloy", "ta"))
    parser.add_argument("--strategy", choices=("consistent-hashing", "least-weighted"))
    parser.add_argument("--run-id")
    args = parser.parse_args()
    if args.action in ("render", "up", "switch"):
        settings = load_settings(args.settings)
        if args.mode:
            settings["discovery_mode"] = args.mode
        if args.strategy:
            settings["ta_strategy"] = args.strategy
        if args.run_id:
            if not re.fullmatch(r"[a-zA-Z0-9_-]+", args.run_id):
                raise ValueError("Invalid run ID")
            settings["run_id"] = args.run_id
        elif args.mode:
            settings["run_id"] = ("baseline-onehot" if args.mode == "alloy" else
                                  "ta-consistent" if settings["ta_strategy"] == "consistent-hashing" else "ta-least-weighted")
        render(settings)
    if args.action == "render":
        return
    kube = ["kubectl", "--kubeconfig", args.kubeconfig, "--context", f"kind-{args.cluster}"]
    helm = ["helm", "--kubeconfig", args.kubeconfig, "--kube-context", f"kind-{args.cluster}"]
    if args.action in ("up", "switch"):
        if args.action == "switch":
            # Never create a cluster or mutate workload resources during a comparison.
            for size in ("large", "small"):
                current = json.loads(run(kube + ["-n", NAMESPACE, "get", "statefulset", f"targets-{size}", "-o", "json"], capture_output=True).stdout)
                expected = workload(settings, size)["spec"]
                if (current["spec"]["replicas"] != expected["replicas"] or
                    current["spec"]["template"]["spec"]["containers"][0]["args"] != expected["template"]["spec"]["containers"][0]["args"]):
                    raise ValueError("Workload settings differ from live targets; use up explicitly to resize")
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
        if args.action == "up":
            run(kube + ["apply", "-f", BUILD / "targets.json"])
        if settings["discovery_mode"] == "ta":
            run(kube + ["apply", "-f", BUILD / "allocator.json"])
            run(kube + ["-n", NAMESPACE, "rollout", "status", "deployment/target-allocator", "--timeout=180s"])
        # Secret updates trigger exactly one Helm rollout, without exposing the secret.
        values_path = BUILD / "alloy-values.json"
        chart_values = json.loads(values_path.read_text())
        chart_values["controller"]["podAnnotations"] = {
            "checksum/cloud-credentials": hashlib.sha256(json.dumps(values, sort_keys=True).encode()).hexdigest()}
        values_path.write_text(json.dumps(chart_values, indent=2) + "\n")
        run(helm + ["upgrade", "--install", RELEASE, REPO_ROOT / "operations/helm/charts/alloy",
                    "--namespace", NAMESPACE, "--values", BUILD / "alloy-values.json", "--wait", "--timeout", "5m"])
        for name in ("targets-large", "targets-small", RELEASE):
            run(kube + ["-n", NAMESPACE, "rollout", "status", f"statefulset/{name}", "--timeout=300s"])
        if settings["discovery_mode"] == "alloy":
            run(kube + ["delete", "-f", BUILD / "allocator.json", "--ignore-not-found=true"])
        print("Demo ready. Allow at least 5 minutes of warm-up before comparing load.")
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
