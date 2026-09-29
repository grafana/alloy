#!/usr/bin/env python3
"""Lightweight live TA contract check; no build, credentials, or Cloud query needed."""

import argparse
from collections import Counter
import json
from pathlib import Path
import subprocess

import demo


def value(node):
    if node["type"] == "object":
        return {item["key"]: value(item["value"]) for item in node["value"]}
    if node["type"] == "array":
        return [value(item) for item in node["value"]]
    return node["value"]


def verify(kube, settings):
    def get(*args):
        return json.loads(subprocess.check_output(kube + ["get", *args], text=True))

    root = f"/api/v1/namespaces/{demo.NAMESPACE}"
    allocator_url = f"{root}/services/target-allocator:8080/proxy"
    assert set(get("--raw", allocator_url + "/jobs")) == {"target-allocation"}, "Unexpected scrape jobs"
    collectors = get("--raw", allocator_url + "/jobs/target-allocation/targets")
    assert set(collectors) == {f"{demo.RELEASE}-{i}" for i in range(settings["alloy_replicas"])}, "Unexpected collector membership"
    all_assigned = []
    all_scraped = []
    rows = []
    for i in range(settings["alloy_replicas"]):
        peer = f"{demo.RELEASE}-{i}"
        assignments = get("--raw", f"{root}/services/target-allocator:8080/proxy/jobs/target-allocation/targets?collector_id={peer}")
        assigned = [g["labels"]["__meta_kubernetes_pod_name"] for g in assignments for _ in g["targets"]]
        component = f"{root}/pods/{peer}:12345/proxy/api/v0/web/components/"
        discovery = get("--raw", component + "discovery.http.demo")
        discovered = next(value(a["value"]) for a in discovery["exports"] if a["name"] == "targets")
        assert sorted(t["__meta_kubernetes_pod_name"] for t in discovered) == sorted(assigned), f"{peer}: stale HTTP discovery"
        scrape = get("--raw", component + "prometheus.scrape.demo")
        targets = [{a["name"]: value(a["value"]) for a in block["body"]}
                   for block in scrape.get("debugInfo", []) if block["name"] == "target"]
        scraped = [t["labels"]["instance"] for t in targets]
        assert sorted(scraped) == sorted(assigned), f"{peer}: scrape and TA assignments differ"
        assert all(t["health"] == "up" for t in targets), f"{peer}: unhealthy target"
        for t in targets:
            assert t["labels"]["size_class"] == t["labels"]["instance"].split("-")[1]
        pod = get("pod", peer, "-n", demo.NAMESPACE, "-o", "json")
        assert not any("serviceAccountToken" in source for volume in pod["spec"].get("volumes", [])
                       for source in volume.get("projected", {}).get("sources", [])), f"{peer}: API token still mounted"
        all_assigned.extend(assigned)
        all_scraped.extend(scraped)
        rows.append({"peer": peer, "targets": len(targets),
                     "expected_series": sum(settings[t["labels"]["size_class"] + "_series"] for t in targets)})
    expected = {f"targets-{size}-{i}" for size in ("large", "small") for i in range(settings[size + "_targets"])}
    assert set(all_assigned) == expected, "Incomplete allocation"
    assert all(n == 1 for n in Counter(all_assigned).values()), "Overlapping assignments"
    assert all(n == 1 for n in Counter(all_scraped).values()), "Overlapping scraping"
    return rows


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--kubeconfig", default=str(demo.KIND_ROOT / "build/kubeconfig.yaml"))
    parser.add_argument("--cluster", default="alloy-example")
    parser.add_argument("--settings", type=Path, default=demo.BUILD / "settings.json")
    args = parser.parse_args()
    settings = demo.load_settings(args.settings)
    if settings["discovery_mode"] != "ta":
        raise SystemExit("This check requires TA mode")
    kube = ["kubectl", "--kubeconfig", args.kubeconfig, "--context", f"kind-{args.cluster}"]
    print(json.dumps(verify(kube, settings), indent=2))
