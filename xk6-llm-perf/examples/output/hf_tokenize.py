#!/usr/bin/env python3
"""Count local Hugging Face tokenizer inputs with explicit asset/runtime pins."""
import argparse
import hashlib
import importlib.metadata
import json
import os
import pathlib
import sys

MAX_INPUT = 1 << 20
MAX_ASSETS = 64 << 20


def digest(data):
    return hashlib.sha256(data).hexdigest()


def unique_object(items):
    result = {}
    for key, value in items:
        if key in result:
            raise ValueError("duplicate JSON key")
        result[key] = value
    return result


def decode(data):
    return json.loads(data.decode("utf-8"), object_pairs_hook=unique_object,
                      parse_constant=lambda _: (_ for _ in ()).throw(ValueError("nonfinite number")))


def verify_assets(root, manifest):
    if root.is_symlink() or not root.is_dir():
        raise ValueError("asset root must be a real directory")
    if not isinstance(manifest, dict) or not manifest:
        raise ValueError("empty asset manifest")
    actual = {}
    size = 0
    for path in root.rglob("*"):
        if path.is_symlink():
            raise ValueError("asset symlink refused")
        if path.is_dir():
            continue
        if not path.is_file():
            raise ValueError("nonregular asset refused")
        name = path.relative_to(root).as_posix()
        if path.suffix == ".py":
            raise ValueError("remote tokenizer code refused")
        size += path.stat().st_size
        if size > MAX_ASSETS:
            raise ValueError("assets exceed budget")
        actual[name] = digest(path.read_bytes())
    if actual != manifest:
        raise ValueError("asset manifest does not match complete local tree")
    if "tokenizer_config.json" not in actual:
        raise ValueError("tokenizer_config.json required")
    return digest(json.dumps(actual, sort_keys=True, separators=(",", ":")).encode())


def tokenize(tokenizer, request):
    if not isinstance(request, dict):
        raise ValueError("input must be an object")
    if "text" in request:
        if set(request) - {"text", "add_special_tokens"} or not isinstance(request["text"], str):
            raise ValueError("invalid text input")
        special = request.get("add_special_tokens", False)
        if type(special) is not bool:
            raise ValueError("add_special_tokens must be boolean")
        rendered = request["text"]
        ids = tokenizer.encode(rendered, add_special_tokens=special)
        scope = "text with special tokens" if special else "text without special tokens"
    else:
        if set(request) - {"messages", "tools", "add_generation_prompt", "continue_final_message", "enable_thinking"}:
            raise ValueError("unknown chat input field")
        messages = request.get("messages")
        if not isinstance(messages, list) or not messages or len(messages) > 1000:
            raise ValueError("messages must be a nonempty bounded list")
        for message in messages:
            if not isinstance(message, dict) or not isinstance(message.get("role"), str):
                raise ValueError("invalid message")
            if not isinstance(message.get("content"), (str, list)) and message.get("content") is not None:
                raise ValueError("invalid message content")
        options = {}
        for key in ("add_generation_prompt", "continue_final_message", "enable_thinking"):
            if key in request:
                if type(request[key]) is not bool:
                    raise ValueError(key + " must be boolean")
                options[key] = request[key]
        if "tools" in request:
            if not isinstance(request["tools"], list) or len(request["tools"]) > 100:
                raise ValueError("tools must be a bounded list")
            options["tools"] = request["tools"]
        rendered = tokenizer.apply_chat_template(messages, tokenize=False, **options)
        if not isinstance(rendered, str) or len(rendered.encode()) > MAX_INPUT:
            raise ValueError("rendered input exceeds budget")
        # Hugging Face chat templates already insert the required special tokens.
        ids = tokenizer.encode(rendered, add_special_tokens=False)
        scope = "model chat template; textual tokens only"
    if not isinstance(ids, list) or len(ids) > 100000 or any(type(i) is not int or i < 0 for i in ids):
        raise ValueError("invalid or oversized token IDs")
    return ids, digest(rendered.encode()), scope


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--assets", required=True, type=pathlib.Path)
    p.add_argument("--manifest", required=True, type=pathlib.Path)
    p.add_argument("--manifest-sha256", required=True)
    p.add_argument("--transformers-version", required=True)
    p.add_argument("--tokenizers-version", required=True)
    p.add_argument("--model", required=True)
    p.add_argument("--revision", required=True)
    p.add_argument("--ids", action="store_true")
    args = p.parse_args()
    if not args.model or len(args.model) > 1024 or not args.revision or len(args.revision) > 256:
        raise ValueError("missing or oversized model identity")
    with args.manifest.open("rb") as manifest_file:
        manifest_bytes = manifest_file.read(MAX_INPUT + 1)
    if len(manifest_bytes) > MAX_INPUT or digest(manifest_bytes) != args.manifest_sha256:
        raise ValueError("manifest hash mismatch or oversized manifest")
    asset_hash = verify_assets(args.assets, decode(manifest_bytes))
    versions = {"transformers": args.transformers_version, "tokenizers": args.tokenizers_version}
    for package, expected in versions.items():
        if importlib.metadata.version(package) != expected:
            raise ValueError(package + " version mismatch")
    data = sys.stdin.buffer.read(MAX_INPUT + 1)
    if len(data) > MAX_INPUT:
        raise ValueError("input exceeds budget")
    request = decode(data)
    os.environ["HF_HUB_OFFLINE"] = "1"
    os.environ["TRANSFORMERS_OFFLINE"] = "1"
    os.environ["USE_TORCH"] = "0"
    os.environ["USE_TF"] = "0"
    os.environ["USE_FLAX"] = "0"
    os.environ["OPENBLAS_NUM_THREADS"] = "1"
    os.environ["OMP_NUM_THREADS"] = "1"
    os.environ["MKL_NUM_THREADS"] = "1"
    os.environ["TOKENIZERS_PARALLELISM"] = "false"
    from transformers import AutoTokenizer
    tokenizer = AutoTokenizer.from_pretrained(str(args.assets.resolve()), local_files_only=True,
                                              trust_remote_code=False, use_fast=True)
    ids, framed_hash, scope = tokenize(tokenizer, request)
    result = {"schema_version": 1, "model": args.model, "model_revision": args.revision,
              "dependency_versions": versions, "asset_tree_sha256": asset_hash,
              "manifest_sha256": args.manifest_sha256, "framed_input_sha256": framed_hash,
              "worker_sha256": digest(pathlib.Path(__file__).read_bytes()),
              "count": len(ids), "scope": scope,
              "measurement": {"inputTokens": len(ids), "tokenizer": "offline-hf/assets:" + asset_hash}}
    if args.ids:
        result["ids"] = ids
    print(json.dumps(result, sort_keys=True, allow_nan=False))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        # Templates can put prompt contents in exception messages.
        print("hf tokenizer: refused (" + type(error).__name__ + ")", file=sys.stderr)
        sys.exit(1)
