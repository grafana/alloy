import json
from pathlib import Path
import tempfile
import unittest

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


if __name__ == "__main__":
    unittest.main()
