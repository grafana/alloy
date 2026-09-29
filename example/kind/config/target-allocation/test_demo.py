from contextlib import redirect_stdout
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

import demo


class SettingsTest(unittest.TestCase):
    def setUp(self):
        self.settings = json.loads((demo.HERE / "settings.json").read_text())

    def load(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "settings.json"
            path.write_text(json.dumps(self.settings))
            return demo.load_settings(path)

    def test_default_ingestion_budget(self):
        settings = self.load()
        total = sum(settings[f"{size}_targets"] * settings[f"{size}_series"]
                    for size in ("large", "small"))
        self.assertEqual(total, 2640)
        self.assertEqual(total / settings["scrape_interval_seconds"], 88)

    def test_reject_invalid_counts_and_interval(self):
        for key in ("alloy_replicas", "scrape_interval_seconds", "large_targets",
                    "large_series", "small_targets", "small_series"):
            original = self.settings[key]
            for invalid in (0, -1, True, 1.5, "700"):
                with self.subTest(key=key, value=invalid):
                    self.settings[key] = invalid
                    with self.assertRaisesRegex(ValueError, key):
                        self.load()
            self.settings[key] = original

    def test_reject_unpinned_image(self):
        for image in ("grafana/alloy", "grafana/alloy:latest", "grafana/alloy:"):
            self.settings["alloy_image"] = image
            with self.assertRaisesRegex(ValueError, "alloy_image"):
                self.load()

    def test_ta_render_removes_kubernetes_access_and_double_sharding(self):
        self.settings["discovery_mode"] = "ta"
        with tempfile.TemporaryDirectory() as directory:
            build = Path(directory)
            with patch.object(demo, "BUILD", build), redirect_stdout(io.StringIO()):
                demo.render(self.settings)
            values = json.loads((build / "alloy-values.json").read_text())
            config = values["alloy"]["configMap"]["content"]
            self.assertFalse(values["alloy"]["clustering"]["enabled"])
            self.assertFalse(values["rbac"]["create"])
            self.assertFalse(values["serviceAccount"]["automountServiceAccountToken"])
            self.assertNotIn("discovery.kubernetes", config)
            self.assertNotIn("clustering {", config)
            self.assertIn('collector_id=" + sys.env("POD_NAME")', config)
            self.assertRegex(config, r'target_label\s*=\s*"instance"')
            resources = json.loads((build / "allocator.json").read_text())["items"]
            role = next(r for r in resources if r["kind"] == "Role")
            self.assertEqual(role["rules"][0]["resources"], ["pods"])
            cm = next(r for r in resources if r["kind"] == "ConfigMap")
            ta = json.loads(cm["data"]["targetallocator.yaml"])
            self.assertFalse(ta["prometheus_cr"]["enabled"])
            self.assertEqual(ta["collector_selector"]["matchLabels"]["app.kubernetes.io/instance"], demo.RELEASE)

    def test_baseline_and_ta_render_identical_workload(self):
        with tempfile.TemporaryDirectory() as directory:
            build = Path(directory)
            with patch.object(demo, "BUILD", build), redirect_stdout(io.StringIO()):
                demo.render(self.settings)
                original = (build / "targets.json").read_text()
                self.settings["discovery_mode"] = "ta"
                self.settings["ta_strategy"] = "least-weighted"
                demo.render(self.settings)
                self.assertEqual(original, (build / "targets.json").read_text())

    def test_switch_does_not_apply_workload(self):
        # Exercise the CLI path with subprocesses mocked: a mode switch must never
        # apply targets.json, even when valid workload settings are supplied.
        commands = []

        def fake_run(args, **kwargs):
            args = [str(a) for a in args]
            commands.append(args)
            if "get" in args and "statefulset" in args:
                size = args[args.index("statefulset") + 1].removeprefix("targets-")
                return subprocess.CompletedProcess(args, 0, json.dumps(demo.workload(self.settings, size)))
            return subprocess.CompletedProcess(args, 0, "")

        with tempfile.TemporaryDirectory() as directory:
            with patch.object(demo, "BUILD", Path(directory)), patch.object(demo, "run", fake_run), \
                 patch.object(demo, "credentials", return_value={k: "placeholder" for k in demo.SECRET_KEYS}), \
                 patch.object(sys, "argv", ["demo.py", "switch", "--mode", "ta"]), \
                 redirect_stdout(io.StringIO()):
                demo.main()
        self.assertTrue(any("upgrade" in args for args in commands))
        self.assertFalse(any("apply" in args and any(a.endswith("targets.json") for a in args) for args in commands))

    def test_reject_invalid_allocation_settings(self):
        for key, value in (("discovery_mode", "unknown"), ("ta_strategy", "per-node"),
                           ("ta_image", "target-allocator:latest"),
                           ("http_refresh_interval_seconds", 0)):
            original = self.settings[key]
            self.settings[key] = value
            with self.subTest(key=key), self.assertRaises(ValueError):
                self.load()
            self.settings[key] = original


if __name__ == "__main__":
    unittest.main()
