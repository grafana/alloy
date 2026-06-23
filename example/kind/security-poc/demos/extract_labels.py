#!/usr/bin/env python3
"""Extract the `labels` field from flag5-output.log lines.

Each log line is logfmt and may contain a `labels="..."` field. The value is a
Prometheus/Loki labelset, e.g. `{key="value", other="..."}`. Some label values
are themselves (escaped) JSON, so we parse those into nested JSON too.

The script collects one JSON object per line that has a labels field,
deduplicates identical objects, and writes the unique ones to an output file.
"""

import argparse
import json
import re
import sys

# Captures the logfmt value of labels="...", honoring backslash-escaped chars.
LABELS_RE = re.compile(r'labels="((?:[^"\\]|\\.)*)"')

# Captures key="value" pairs inside a labelset, honoring escaped chars in value.
PAIR_RE = re.compile(r'(\w+)="((?:[^"\\]|\\.)*)"')


def unescape(quoted_inner: str) -> str:
    """Turn the inside of a quoted string (with \\\" \\\\ etc.) into its value."""
    return json.loads('"' + quoted_inner + '"')


def maybe_json(value: str):
    """If value is itself JSON (object/array), parse it; otherwise keep string."""
    stripped = value.lstrip()
    if stripped[:1] in ("{", "["):
        try:
            return json.loads(value)
        except (json.JSONDecodeError, ValueError):
            pass
    return value


def parse_labels(raw_labels: str) -> dict:
    """raw_labels is the captured logfmt value (still escaped once)."""
    labelset = unescape(raw_labels)  # e.g. {filename="...", k8s_configmaps="{...}"}
    result = {}
    for key, raw_value in PAIR_RE.findall(labelset):
        value = unescape(raw_value)
        result[key] = maybe_json(value)
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input", nargs="?", default="flag5-output.log")
    parser.add_argument("-o", "--output", default="flag5-labels.json")
    args = parser.parse_args()

    unique = []  # preserve first-seen order
    seen = set()
    lines_with_labels = 0

    with open(args.input, "r", encoding="utf-8", errors="replace") as f:
        for line in f:
            m = LABELS_RE.search(line)
            if not m:
                continue
            lines_with_labels += 1
            obj = parse_labels(m.group(1))
            # Canonical form for dedup: sorted keys, stable separators.
            canonical = json.dumps(obj, sort_keys=True, ensure_ascii=False)
            if canonical in seen:
                continue
            seen.add(canonical)
            unique.append(obj)

    with open(args.output, "w", encoding="utf-8") as out:
        json.dump(unique, out, indent=2, ensure_ascii=False)
        out.write("\n")

    print(
        f"lines with labels: {lines_with_labels}, "
        f"unique label objects: {len(unique)} -> {args.output}",
        file=sys.stderr,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
