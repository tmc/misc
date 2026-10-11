import contextlib
import io
import json
import pathlib
import tempfile
import types
import unittest

import reference


class ReferenceTest(unittest.TestCase):
    def compare(self, native, refs):
        with tempfile.TemporaryDirectory() as name:
            directory = pathlib.Path(name)
            capture = directory / "native.jsonl"
            capture.write_text("\n".join(json.dumps(r) for r in native))
            ref = directory / "reference.jsonl"
            ref.write_text("\n".join(json.dumps(r) for r in refs))
            with contextlib.redirect_stdout(io.StringIO()) as out:
                code = reference.compare(types.SimpleNamespace(native=capture, reference=ref, max_skew_ms=250))
            return code, json.loads(out.getvalue())

    def fixture(self, value=50, stamp=1000000000):
        return [{"sample": {"readings": [{"name": "temperature_celsius", "unit": "celsius",
                  "device_uuid": "GPU-test", "evidence": "measured", "value": value,
                  "timestamp_unix": stamp}]}},
                {"manifest": {"schema_version": 1, "is_complete": True, "lost_records": 0}}]

    def refs(self, device="GPU-test", value=50):
        return [{"device_uuid": device, "query_start_unix": 999000000,
                 "query_end_unix": 1001000000,
                 "readings": {"temperature_celsius": {"value": value, "unit": "celsius"}}}]

    def test_units_tolerances_and_device_identity(self):
        for value, device, status, code in [(52, "GPU-test", "PASS", 0),
                                            (54, "GPU-test", "GAP", 1),
                                            (50, "other-device", "UNTESTED", 0)]:
            with self.subTest(value=value, device=device):
                result, report = self.compare(self.fixture(value), self.refs(device))
                self.assertEqual(result, code)
                self.assertEqual(report["checks"][0]["status"], status)

    def test_stale_pair_is_not_pass(self):
        _, report = self.compare(self.fixture(stamp=2000000000), self.refs())
        self.assertEqual(report["checks"][0]["status"], "UNTESTED")

    def test_incomplete_or_lost_capture_rejected(self):
        for field, value in [("is_complete", False), ("lost_records", 1)]:
            native = self.fixture()
            native[-1]["manifest"][field] = value
            with self.assertRaises(ValueError):
                self.compare(native, self.refs())

    def test_unavailable_does_not_become_zero(self):
        native = self.fixture()
        native[0]["sample"]["readings"][0]["evidence"] = "unavailable"
        _, report = self.compare(native, self.refs(value=0))
        self.assertEqual(report["checks"][0]["status"], "UNAVAILABLE")


if __name__ == "__main__":
    unittest.main()
