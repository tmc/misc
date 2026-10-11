#!/usr/bin/env python3
"""Asset admission and actual pinned-runtime chat-template controls."""
import hashlib
import importlib.util
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest
from collections.abc import Mapping

SCRIPT = pathlib.Path(__file__).with_name("hf_tokenize.py")
spec = importlib.util.spec_from_file_location("hf_tokenize", SCRIPT)
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)
TASK = pathlib.Path.home() / "tmp/xk6-llm-perf/20261010-parity-accuracy/test-tmp"
TASK.mkdir(parents=True, exist_ok=True)


class Admission(unittest.TestCase):
    def test_json(self):
        for data in (b'{"text":"a","text":"b"}', b'{"text":NaN}', b'{"text":"\xff"}'):
            with self.assertRaises((ValueError, UnicodeError)):
                bridge.decode(data)

    def test_asset_tree(self):
        with tempfile.TemporaryDirectory(dir=TASK) as tmp:
            root = pathlib.Path(tmp)
            (root / "tokenizer_config.json").write_text("{}")
            manifest = {"tokenizer_config.json": bridge.digest(b"{}")}
            self.assertEqual(len(bridge.verify_assets(root, manifest)), 64)
            (root / "extra.json").write_text("{}")
            with self.assertRaises(ValueError):
                bridge.verify_assets(root, manifest)
            (root / "extra.json").unlink()
            (root / "link").symlink_to("tokenizer_config.json")
            with self.assertRaises(ValueError):
                bridge.verify_assets(root, manifest)
            (root / "link").unlink()
            (root / "custom.py").write_text("raise Exception('must not execute')")
            with self.assertRaises(ValueError):
                bridge.verify_assets(root, manifest)

    def test_missing_template_not_inferred(self):
        class NoTemplate:
            def apply_chat_template(self, *args, **kwargs):
                raise ValueError("no chat template")
        with self.assertRaises(ValueError):
            bridge.tokenize(NoTemplate(), {"messages": [{"role": "user", "content": "hi"}]})

    def test_actual_hf_template(self):
        os.environ["USE_TORCH"] = "0"
        os.environ["USE_TF"] = "0"
        os.environ["USE_FLAX"] = "0"
        os.environ["OPENBLAS_NUM_THREADS"] = "1"
        os.environ["OMP_NUM_THREADS"] = "1"
        os.environ["MKL_NUM_THREADS"] = "1"
        os.environ["TOKENIZERS_PARALLELISM"] = "false"
        try:
            import tokenizers
            import transformers
        except ImportError:
            self.skipTest("optional pinned runtime unavailable")
        from tokenizers import Tokenizer, models, pre_tokenizers
        from transformers import PreTrainedTokenizerFast
        with tempfile.TemporaryDirectory(dir=TASK) as tmp:
            root = pathlib.Path(tmp)
            assets = root / "assets"
            assets.mkdir()
            raw = Tokenizer(models.WordLevel({"[UNK]": 0, "user": 1, "hello": 2,
                                             "assistant": 3, "lookup": 4, "[IMAGE]": 5}, unk_token="[UNK]"))
            raw.pre_tokenizer = pre_tokenizers.WhitespaceSplit()
            tokenizer = PreTrainedTokenizerFast(tokenizer_object=raw, unk_token="[UNK]")
            tokenizer.chat_template = "{% for tool in tools|default([]) %}{{tool.function.name}} {% endfor %}{% for m in messages %}{{m.role}} {% if m.content is string %}{{m.content}}{% else %}{% for part in m.content %}{% if part.type == 'text' %}{{part.text}}{% else %}[IMAGE]{% endif %} {% endfor %}{% endif %} {% endfor %}{% if add_generation_prompt %}assistant{% endif %}"
            tokenizer.save_pretrained(assets)
            manifest = {p.name: bridge.digest(p.read_bytes()) for p in assets.iterdir()}
            manifest_path = root / "manifest.json"
            manifest_path.write_text(json.dumps(manifest))
            args = [sys.executable, str(SCRIPT), "--assets", str(assets), "--manifest", str(manifest_path),
                    "--manifest-sha256", bridge.digest(manifest_path.read_bytes()), "--model", "fixture",
                    "--revision", "local", "--transformers-version", transformers.__version__,
                    "--tokenizers-version", tokenizers.__version__, "--ids"]
            request = {"messages": [{"role": "user", "content": [{"type": "text", "text": "hello"},
                      {"type": "image", "image": "local-reference"}]}], "add_generation_prompt": True,
                      "tools": [{"type": "function", "function": {"name": "lookup"}}]}
            result = subprocess.run(args, input=json.dumps(request), text=True, capture_output=True, timeout=90)
            self.assertEqual(result.returncode, 0, result.stderr)
            got = json.loads(result.stdout)
            expected = tokenizer.apply_chat_template(request["messages"], tools=request["tools"],
                                                     add_generation_prompt=True, tokenize=True)
            if isinstance(expected, Mapping):
                expected = expected["input_ids"]
            self.assertEqual(got["ids"], expected)
            self.assertEqual(expected, [4, 1, 2, 5, 3])
            self.assertEqual(got["count"], 5)
            self.assertEqual(got["measurement"]["inputTokens"], 5)
            bad = subprocess.run(args[:-1] + ["--transformers-version", "0.invalid"],
                                 input='{"text":"hello"}', text=True, capture_output=True, timeout=90)
            self.assertNotEqual(bad.returncode, 0)
            (assets / "tokenizer_config.json").write_text("{}")
            bad = subprocess.run(args, input='{"text":"hello"}', text=True, capture_output=True, timeout=90)
            self.assertNotEqual(bad.returncode, 0)
            self.assertIn("refused (ValueError)", bad.stderr)


if __name__ == "__main__":
    unittest.main()
