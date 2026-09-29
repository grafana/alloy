#!/usr/bin/env python3
"""Build the operator checkout's TA, load it into kind, and switch the demo."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys

import demo


def output(args, **kwargs):
    return subprocess.check_output([str(a) for a in args], text=True, **kwargs).strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--operator-dir", type=Path, default=Path.home() / "workspace/opentelemetry-operator")
    parser.add_argument("--settings", type=Path, default=demo.HERE / "settings.json")
    parser.add_argument("--kubeconfig", type=Path, default=demo.KIND_ROOT / "build/kubeconfig.yaml")
    parser.add_argument("--cluster", default="alloy-example")
    args = parser.parse_args()
    repo = args.operator_dir.expanduser().resolve()
    dockerfile = repo / "cmd/otel-allocator/Dockerfile"
    if not dockerfile.is_file():
        raise ValueError(f"Operator checkout not found at {repo}")
    settings = demo.load_settings(args.settings)
    active = output(["docker", "ps", "--filter", "label=io.x-k8s.kind.cluster",
                     "--format", '{{.Label "io.x-k8s.kind.cluster"}}']).splitlines()
    if set(active) != {args.cluster}:
        raise ValueError(f"Only the running {args.cluster} kind cluster is allowed")
    kube = ["kubectl", "--kubeconfig", args.kubeconfig, "--context", f"kind-{args.cluster}"]
    nodes = json.loads(output(kube + ["get", "nodes", "-o", "json"]))["items"]
    architectures = {n["status"]["nodeInfo"]["architecture"] for n in nodes}
    if len(architectures) != 1 or not architectures <= {"arm64", "amd64"}:
        raise ValueError(f"Unsupported node architectures: {architectures}")
    arch = architectures.pop()
    revision = output(["git", "rev-parse", "HEAD"], cwd=repo)
    status = output(["git", "status", "--porcelain"], cwd=repo)
    env = os.environ.copy()
    # Limit compiler parallelism; the soft heap limit is per Go process.
    env.update(GOMAXPROCS="2", GOMEMLIMIT="1GiB", GOFLAGS="-p=2 -mod=readonly")
    # An inherited GOROOT from another Go installation breaks cross-compilation.
    env.pop("GOROOT", None)
    demo.run(["make", "targetallocator", "GOOS=linux", f"ARCH={arch}"], cwd=repo, env=env)
    binary = repo / f"cmd/otel-allocator/bin/targetallocator_{arch}"
    context = demo.BUILD / "local-image"
    (context / "bin").mkdir(parents=True, exist_ok=True)
    shutil.copy2(binary, context / "bin" / binary.name)
    shutil.copy2(dockerfile, context / "Dockerfile")
    digest = hashlib.sha256(binary.read_bytes() + dockerfile.read_bytes()).hexdigest()
    image = f"target-allocator-local:{revision[:12]}-{digest[:16]}"
    demo.run(["docker", "build", "--platform", f"linux/{arch}", "--build-arg", f"TARGETARCH={arch}",
              "-t", image, context])
    demo.run(["kind", "load", "docker-image", image, "--name", args.cluster])
    settings.update(discovery_mode="ta", ta_image=image, ta_image_pull_policy="Never", run_id="ta-local")
    profile = demo.BUILD / "local-settings.json"
    profile.write_text(json.dumps(settings, indent=2) + "\n")
    metadata = {"operator_dir": str(repo), "revision": revision, "working_tree_status": status,
                "architecture": arch, "image": image,
                "image_id": output(["docker", "image", "inspect", image, "--format", "{{.Id}}"])}
    (demo.BUILD / "local-build.json").write_text(json.dumps(metadata, indent=2) + "\n")
    if output(["git", "status", "--porcelain"], cwd=repo) != status:
        raise ValueError("Operator working tree changed during build; inspect before deploying")
    demo.run([sys.executable, demo.HERE / "demo.py", "switch", "--settings", profile,
              "--kubeconfig", args.kubeconfig, "--cluster", args.cluster])
    pods = json.loads(output(kube + ["-n", demo.NAMESPACE, "get", "pods", "-l",
                                    "app.kubernetes.io/name=target-allocator", "-o", "json"]))["items"]
    pod = next(p for p in pods if p["spec"]["containers"][0]["image"] == image)
    runtime_id = pod["status"]["containerStatuses"][0]["imageID"]
    # Kubernetes reports a manifest digest; Docker's image ID is its config digest.
    images = json.loads(output(["docker", "exec", pod["spec"]["nodeName"],
                                "crictl", "images", "--output", "json"]))["images"]
    if not any(i["id"] == metadata["image_id"] and runtime_id in i.get("repoDigests", []) for i in images):
        raise ValueError("Running TA image does not match the local build")
    metadata["runtime_image_id"] = runtime_id
    (demo.BUILD / "local-build.json").write_text(json.dumps(metadata, indent=2) + "\n")
    print(f"Local TA deployed: {image}\nSource revision: {revision}")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, subprocess.CalledProcessError) as error:
        raise SystemExit(str(error)) from None
