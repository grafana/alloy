#!/usr/bin/env python3
"""Find Kubernetes Opaque Secrets in a JSON file and base64-decode their data.

A match is any object that has `type == "Opaque"` and a sibling `data` object
whose values are base64-encoded strings. We walk the whole JSON tree, decode
each value, and print the findings.
"""

import argparse
import base64
import binascii
import json
import sys


def decode_value(value: str):
    """Base64-decode a string; return (decoded_text, ok)."""
    try:
        raw = base64.b64decode(value, validate=True)
    except (binascii.Error, ValueError):
        return value, False
    try:
        return raw.decode("utf-8"), True
    except UnicodeDecodeError:
        return raw.hex(), True  # binary payload, show hex


def find_opaque_secrets(node, found):
    """Recursively collect dicts that look like Opaque secrets."""
    if isinstance(node, dict):
        if node.get("type") == "Opaque" and isinstance(node.get("data"), dict):
            found.append(node)
        for v in node.values():
            find_opaque_secrets(v, found)
    elif isinstance(node, list):
        for item in node:
            find_opaque_secrets(item, found)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input", nargs="?", default="flag5-labels.json")
    args = parser.parse_args()

    with open(args.input, "r", encoding="utf-8") as f:
        data = json.load(f)

    secrets = []
    find_opaque_secrets(data, secrets)

    print(f"Found {len(secrets)} Opaque secret(s) in {args.input}\n")

    for i, secret in enumerate(secrets, 1):
        meta = secret.get("metadata", {}) if isinstance(secret.get("metadata"), dict) else {}
        name = meta.get("name", "<unknown>")
        namespace = meta.get("namespace", "<unknown>")
        print(f"[{i}] secret: {name}  (namespace: {namespace})")
        for key, value in secret["data"].items():
            decoded, ok = decode_value(value) if isinstance(value, str) else (value, False)
            marker = "" if ok else "  (not valid base64)"
            print(f"      {key} = {decoded}{marker}")
        print()

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
