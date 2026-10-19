"""Offline controls for native/reference diagnostic comparison."""
import contextlib
import io
import json
import pathlib
import tempfile
import types
import unittest

import reference


class NativeValueTest(unittest.TestCase):
    def test_values(self):
        for reading, want in [({"value": 0}, 0),
                              ({"is_int": True}, 0),
                              ({"is_int": True, "int_value": 2**53 + 1,
                                "value": float(2**53)}, 2**53 + 1)]:
            with self.subTest(reading=reading):
                self.assertEqual(reference.native_value(reading), want)

    def test_invalid(self):
        for reading in [{}, {"value": None}, {"value": True},
                        {"value": float("nan")}, {"is_int": 1},
                        {"is_int": True, "int_value": -1},
                        {"is_int": True, "int_value": 2**64},
                        {"is_int": True, "int_value": 1.0}]:
            with self.subTest(reading=reading):
                with self.assertRaises(ValueError):
                    reference.native_value(reading)


class CompareTest(unittest.TestCase):
    def compare(self, reading, reference_value=0, reference_uuid="GPU-test",
                reference_unit="milliwatts"):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            native = root / "native.jsonl"
            ref = root / "reference.jsonl"
            reading_values = reading
            reading = dict(name="power_milliwatts", unit="milliwatts",
                           device_uuid="GPU-test", timestamp_unix=1000,
                           evidence="measured")
            reading.update(reading_values)
            native.write_text("\n".join(map(json.dumps, [
                {"manifest": {"schema_version": 2}},
                {"sample": {"readings": [reading]}},
                {"manifest": {"schema_version": 2, "is_complete": True}}])))
            ref.write_text(json.dumps({"device_uuid": reference_uuid,
                                      "query_start_unix": 1000,
                                      "query_end_unix": 1000,
                                      "readings": {"power_milliwatts": {
                                          "value": reference_value, "unit": reference_unit}}}))
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                code = reference.compare(types.SimpleNamespace(
                    native=native, reference=ref, max_skew_ms=250))
            return code, json.loads(output.getvalue())

    def test_zero(self):
        for reading in [{"value": 0}, {"is_int": True}]:
            code, report = self.compare(reading)
            self.assertEqual(code, 0)
            self.assertEqual(report["checks"][0]["delta"], 0)

    def test_integer_authority(self):
        code, report = self.compare({"is_int": True, "int_value": 27578,
                                     "value": 14520}, 14520)
        self.assertEqual(code, 1)
        self.assertEqual(report["checks"][0]["delta"], 13058)
        self.assertEqual(report["checks"][0]["tolerance"], 10000)

    def test_power_reference_unit(self):
        with self.assertRaises(ValueError):
            self.compare({"value": 1}, reference_value=1, reference_unit="watts")

    def test_no_comparison(self):
        for reading, uuid in [({"value": 0}, "GPU-other"),
                              ({"evidence": "unavailable"}, "GPU-test")]:
            with self.assertRaises(ValueError):
                self.compare(reading, reference_uuid=uuid)

    def test_missing_value(self):
        with self.assertRaises(ValueError):
            self.compare({})


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
                                            (54, "GPU-test", "GAP", 1)]:
            with self.subTest(value=value, device=device):
                result, report = self.compare(self.fixture(value), self.refs(device))
                self.assertEqual(result, code)
                self.assertEqual(report["checks"][0]["status"], status)

    def test_reference_invalid_values(self):
        for value in [True, False, "50", float("nan"), float("inf"), 10**400]:
            with self.subTest(value=value):
                with self.assertRaises(ValueError):
                    self.compare(self.fixture(), self.refs(value=value))

    def test_reference_units(self):
        for unit in [None, "watts", "milliwatts"]:
            refs = self.refs()
            refs[0]["readings"]["temperature_celsius"]["unit"] = unit
            with self.subTest(unit=unit):
                with self.assertRaises(ValueError):
                    self.compare(self.fixture(), refs)
        refs = self.refs()
        del refs[0]["readings"]["temperature_celsius"]["unit"]
        with self.assertRaises(ValueError):
            self.compare(self.fixture(), refs)

    def test_reference_intervals(self):
        for start, end in [(1001000000, 999000000), (True, 1001000000),
                           (999000000, False), (float("nan"), 1001000000),
                           (999000000, float("inf")), (0, 1001000000),
                           (999000000, 2**63), (999000000.0, 1001000000)]:
            refs = self.refs()
            refs[0].update(query_start_unix=start, query_end_unix=end)
            with self.subTest(start=start, end=end):
                with self.assertRaises(ValueError):
                    self.compare(self.fixture(), refs)

    def test_reference_numeric_positives(self):
        for value in [0, 0.0, 50, 50.5]:
            with self.subTest(value=value):
                code, report = self.compare(self.fixture(value), self.refs(value=value))
                self.assertEqual(code, 0)
                self.assertEqual(report["checks"][0]["delta"], 0)

    def test_native_timestamp_type(self):
        for stamp in [True, float("nan"), float("inf"), 2**63]:
            with self.subTest(stamp=stamp):
                with self.assertRaises(ValueError):
                    self.compare(self.fixture(stamp=stamp), self.refs())

    def test_stale_pair_is_not_pass(self):
        with self.assertRaises(ValueError):
            self.compare(self.fixture(stamp=2000000000), self.refs())

    def test_incomplete_or_lost_capture_rejected(self):
        for field, value in [("is_complete", False), ("lost_records", 1)]:
            native = self.fixture()
            native[-1]["manifest"][field] = value
            with self.assertRaises(ValueError):
                self.compare(native, self.refs())

    def test_unavailable_does_not_become_zero(self):
        native = self.fixture()
        native[0]["sample"]["readings"][0]["evidence"] = "unavailable"
        with self.assertRaises(ValueError):
            self.compare(native, self.refs(value=0))



if __name__ == "__main__":
    unittest.main()
