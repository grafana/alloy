"""Sends every supported signal for every tenant to the producers, then checks
at the sink that each signal of each tenant was delivered by exactly one
consumer.

Uses only the standard library: remote-write protobuf and snappy are
hand-encoded (snappy as literal-only blocks, which is valid snappy).
"""

import json
import os
import random
import struct
import sys
import time
import urllib.request

TARGET = os.environ.get("TARGET", "http://lb:8080")
SINK = os.environ.get("SINK", "http://sink:9999")
TENANTS = os.environ.get("TENANTS", "tenant-a,tenant-b,tenant-c,tenant-d").split(",")
ROUNDS = int(os.environ.get("ROUNDS", "10"))

# Sink path → signal. Each signal has its own topic.
SIGNALS = {
    "/api/v1/push": "metrics",
    "/v1/metrics": "metrics",
    "/loki/api/v1/push": "logs",
    "/v1/logs": "logs",
    "/v1/traces": "traces",
    "/ingest": "profiles",
}
PATHS = list(SIGNALS)


def varint(n):
    out = b""
    while True:
        b = n & 0x7F
        n >>= 7
        if n:
            out += bytes([b | 0x80])
        else:
            return out + bytes([b])


def pb_bytes(field, payload):
    return varint(field << 3 | 2) + varint(len(payload)) + payload


def snappy_literal(data):
    out = varint(len(data))
    for i in range(0, len(data), 65536):
        chunk = data[i:i + 65536]
        n = len(chunk) - 1
        if n < 60:
            out += bytes([n << 2])
        elif n < 256:
            out += bytes([60 << 2, n])
        else:
            out += bytes([61 << 2]) + struct.pack("<H", n)
        out += chunk
    return out


def prom_rw(tenant, i):
    labels = [("__name__", "demo_requests_total"), ("job", "loadgen"), ("round", str(i))]
    ts = b"".join(pb_bytes(1, pb_bytes(1, n.encode()) + pb_bytes(2, v.encode())) for n, v in labels)
    sample = b"\x09" + struct.pack("<d", float(i)) + b"\x10" + varint(int(time.time() * 1000))
    ts += pb_bytes(2, sample)
    return snappy_literal(pb_bytes(1, ts))


def now_ns():
    return str(time.time_ns())


def requests_for(tenant, i):
    resource = {"attributes": [{"key": "service.name", "value": {"stringValue": "loadgen"}}]}
    return [
        ("/api/v1/push", prom_rw(tenant, i), {"Content-Type": "application/x-protobuf", "Content-Encoding": "snappy"}),
        ("/loki/api/v1/push",
         json.dumps({"streams": [{"stream": {"job": "loadgen"}, "values": [[now_ns(), f"hello from {tenant} #{i}"]]}]}).encode(),
         {"Content-Type": "application/json"}),
        (f"/ingest?name=loadgen.cpu%7Benv%3Ddemo%7D&from={int(time.time()) - 10}&until={int(time.time())}&format=folded&sampleRate=100&spyName=demo",
         b"main;work 100\nmain;idle 50\n", {"Content-Type": "text/plain"}),
        ("/v1/metrics",
         json.dumps({"resourceMetrics": [{"resource": resource, "scopeMetrics": [{"metrics": [
             {"name": "demo_gauge", "gauge": {"dataPoints": [{"asDouble": float(i), "timeUnixNano": now_ns()}]}}]}]}]}).encode(),
         {"Content-Type": "application/json"}),
        ("/v1/logs",
         json.dumps({"resourceLogs": [{"resource": resource, "scopeLogs": [{"logRecords": [
             {"timeUnixNano": now_ns(), "body": {"stringValue": f"otlp log from {tenant}"}}]}]}]}).encode(),
         {"Content-Type": "application/json"}),
        ("/v1/traces",
         json.dumps({"resourceSpans": [{"resource": resource, "scopeSpans": [{"spans": [{
             "traceId": "%032x" % random.getrandbits(128), "spanId": "%016x" % random.getrandbits(64),
             "name": "op", "kind": 1, "startTimeUnixNano": now_ns(), "endTimeUnixNano": now_ns()}]}]}]}).encode(),
         {"Content-Type": "application/json"}),
    ]


def post(path, body, headers):
    req = urllib.request.Request(TARGET + path, data=body, method="POST", headers=headers)
    with urllib.request.urlopen(req, timeout=30) as resp:
        return resp.status


def wait_for_producers():
    for _ in range(60):
        try:
            post("/v1/traces", b"{}", {"Content-Type": "application/json", "X-Scope-OrgID": "tenant-unknown"})
        except urllib.error.HTTPError as e:
            if e.code == 403:  # Producer is up and rejects the unknown tenant.
                return
        except Exception:
            pass
        time.sleep(1)
    sys.exit("producers never became ready")


def main():
    wait_for_producers()
    urllib.request.urlopen(urllib.request.Request(SINK + "/", method="DELETE"))

    sent = 0
    for i in range(ROUNDS):
        for tenant in TENANTS:
            for path, body, headers in requests_for(tenant, i):
                post(path, body, {**headers, "X-Scope-OrgID": tenant})
                sent += 1
    print(f"sent {sent} requests ({ROUNDS} rounds x {len(TENANTS)} tenants x {len(PATHS)} paths)")

    # Rejections.
    for tenant, want in [(None, 401), ("tenant-a|tenant-b", 400), ("tenant-x", 403)]:
        headers = {"Content-Type": "application/json"}
        if tenant:
            headers["X-Scope-OrgID"] = tenant
        try:
            post("/v1/traces", b"{}", headers)
            got = 200
        except urllib.error.HTTPError as e:
            got = e.code
        print(f"tenant={tenant!r}: HTTP {got} (want {want})")
        if got != want:
            sys.exit(1)

    time.sleep(int(os.environ.get("SETTLE_SECONDS", "15")))
    stats = json.load(urllib.request.urlopen(SINK + "/"))
    print(json.dumps(stats, indent=2))

    ok = True
    for tenant in TENANTS:
        by_signal = {}
        for consumer, paths in stats.get(tenant, {}).items():
            for path in paths:
                by_signal.setdefault(SIGNALS.get(path, path), set()).add(consumer)
        for signal in sorted(set(SIGNALS.values())):
            consumers = by_signal.get(signal, set())
            if len(consumers) != 1:
                print(f"FAIL {tenant} {signal}: delivered by {sorted(consumers)} (want exactly one consumer)")
                ok = False
            else:
                print(f"OK   {tenant} {signal}: delivered by {next(iter(consumers))}")
        missing = set(PATHS) - {p for c in stats.get(tenant, {}).values() for p in c}
        if missing:
            print(f"FAIL {tenant}: missing paths {sorted(missing)}")
            ok = False
    if "<none>" in stats:
        print(f"FAIL deliveries without tenant: {stats['<none>']}")
        ok = False
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
