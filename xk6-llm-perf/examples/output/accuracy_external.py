#!/usr/bin/env python3
"""One-case pinned AIPerf grader bridge. Run only through Linux bubblewrap."""
import argparse
import asyncio
import hashlib
import importlib.metadata
import json
import os
from pathlib import Path
import resource
import shutil
import subprocess
import sys
import tempfile
from types import SimpleNamespace

REVISION = "f10eb50a1776775d80d7f13396b0f668f14e2c03"
EVALUATORS = {
    "math-symbolic-v1": ("math", "MathGrader", ["sympy", "latex2sympy2-extended"]),
    "lighteval-expr-v1": ("lighteval_grader", "LightevalExprGrader", ["lighteval"]),
    "lighteval-latex-v1": ("lighteval_grader", "LightevalLatexGrader", ["lighteval"]),
    "lighteval-gpqa-v1": ("lighteval_grader", "LightevalGPQAGrader", ["lighteval"]),
    "code-execution-v1": ("code_execution", "CodeExecutionGrader", ["lighteval"]),
}


def read_request():
    data = sys.stdin.buffer.read((256 << 10) + 1)
    if len(data) > 256 << 10:
        raise ValueError("request exceeds 256 KiB")
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError("duplicate JSON key")
            result[key] = value
        return result
    request = json.loads(data, object_pairs_hook=pairs)
    expected = {"schema_version", "evaluator", "prediction", "reference", "dependency_versions"}
    if not isinstance(request, dict) or set(request) != expected or type(request["schema_version"]) is not int or request["schema_version"] != 1:
        raise ValueError("invalid request schema")
    if not isinstance(request["evaluator"], str) or request["evaluator"] not in EVALUATORS:
        raise ValueError("unsupported evaluator")
    for key in ("prediction", "reference"):
        if not isinstance(request[key], str) or len(request[key].encode()) > 65536:
            raise ValueError("invalid answer size")
    versions = request["dependency_versions"]
    if not isinstance(versions, dict) or not versions or len(versions) > 128:
        raise ValueError("missing dependency version pins")
    for package, version in versions.items():
        if not isinstance(package, str) or not isinstance(version, str) or not package or not version or len(package) > 128 or len(version) > 128:
            raise ValueError("invalid dependency version pin")
    for package in EVALUATORS[request["evaluator"]][2]:
        if package not in versions:
            raise ValueError("missing required package pin: " + package)
    return request, data


def limits():
    resource.setrlimit(resource.RLIMIT_CPU, (20, 20))
    resource.setrlimit(resource.RLIMIT_AS, (2 << 30, 2 << 30))
    resource.setrlimit(resource.RLIMIT_FSIZE, (1 << 20, 1 << 20))
    resource.setrlimit(resource.RLIMIT_NOFILE, (128, 128))
    resource.setrlimit(resource.RLIMIT_NPROC, (64, 64))
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))


async def grade(request, source):
    # Dependency versions are checked before loading code supplied by those packages.
    versions = {p: importlib.metadata.version(p) for p in request["dependency_versions"]}
    if versions != request["dependency_versions"]:
        raise ValueError("installed dependency versions differ from pins")
    sys.path.insert(0, str(source / "src"))
    import importlib
    module, class_name, _ = EVALUATORS[request["evaluator"]]
    imported = importlib.import_module("aiperf.accuracy.graders." + module)
    if request["evaluator"] == "math-symbolic-v1" and not (imported._HAS_SYMPY and imported._HAS_LATEX2SYMPY):
        raise ValueError("full symbolic dependencies unavailable")
    cls = getattr(imported, class_name)
    cls.check_available()
    run = SimpleNamespace(cfg=SimpleNamespace(accuracy=SimpleNamespace()))
    grader = cls(run=run)
    try:
        result = await grader.grade(request["prediction"], request["reference"])
    finally:
        await grader.aclose()
    return {"schema_version": 1, "evaluator": request["evaluator"], "source_revision": REVISION,
            "dependency_versions": versions, "correct": bool(result.correct), "unparsed": bool(result.unparsed)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--allow-external", "--allow-confined-execution", action="store_true")
    parser.add_argument("--source", "--aiperf-source", type=Path, required=True)
    parser.add_argument("--runtime", "--runtime-root", type=Path, required=True, help="pinned virtualenv root")
    parser.add_argument("--source-revision", default=REVISION)
    parser.add_argument("--inside", action="store_true", help=argparse.SUPPRESS)
    args = parser.parse_args()
    if not args.allow_external or sys.platform != "linux":
        raise ValueError("external grading requires opt-in and Linux bubblewrap")
    if args.source_revision != REVISION:
        raise ValueError("unsupported source revision")
    request, data = read_request()
    source, runtime = args.source.resolve(), args.runtime.resolve()
    if args.inside:
        # This internal path is not a sandbox API: the outer invocation creates
        # namespaces and mounts. Refuse direct use unless that mount layout exists.
        if str(source) != "/source" or str(runtime) != "/runtime" or not Path("/sandbox-marker").is_file():
            raise ValueError("internal worker requires confined mount layout")
        result = asyncio.run(grade(request, source))
        print(json.dumps(result, sort_keys=True, separators=(",", ":")))
        return
    for name, maximum in (("memory.max", 2 << 30), ("pids.max", 64)):
        value = Path("/sys/fs/cgroup", name).read_text().strip()
        if not value.isdigit() or not 0 < int(value) <= maximum:
            raise ValueError("external grading requires bounded cgroup " + name)
    bwrap = shutil.which("bwrap")
    if not bwrap:
        raise ValueError("bubblewrap is required; there is no host execution fallback")
    revision = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip()
    dirty = subprocess.check_output(["git", "-C", str(source), "status", "--porcelain", "--ignored", "--", "src"], text=True)
    if revision != REVISION or dirty:
        raise ValueError("AIPerf source must be clean and pinned to " + REVISION)
    python = runtime / "bin/python"
    if not python.is_file() or not (runtime / "pyvenv.cfg").is_file():
        raise ValueError("runtime must contain bin/python")
    # Dependencies and source are read-only; no home, secrets or host network are
    # mounted. /work and /tmp are ephemeral. User namespaces must be enabled.
    command = [bwrap, "--unshare-all", "--new-session", "--die-with-parent", "--cap-drop", "ALL",
               "--clearenv", "--setenv", "PATH", "/runtime/bin:/usr/bin", "--setenv", "PYTHONPATH", "/source/src",
               "--setenv", "PYTHONDONTWRITEBYTECODE", "1", "--setenv", "HOME", "/work",
               "--dir", "/source", "--ro-bind", str(source / "src"), "/source/src", "--ro-bind", str(runtime), "/runtime",
               "--ro-bind", str(Path(__file__).resolve()), "/worker.py", "--proc", "/proc", "--dev", "/dev",
               "--size", str(64 << 20), "--tmpfs", "/tmp", "--size", str(64 << 20), "--tmpfs", "/work", "--chdir", "/work"]
    for path in ("/usr", "/lib", "/lib64"):
        if Path(path).exists():
            command += ["--ro-bind", path, path]
    with tempfile.TemporaryDirectory(prefix="accuracy-worker-") as work:
        marker = Path(work) / "marker"
        marker.write_text("bubblewrap accuracy worker v1\n")
        command += ["--ro-bind", str(marker), "/sandbox-marker", "/runtime/bin/python", "/worker.py",
                    "--allow-external", "--inside", "--source", "/source", "--runtime", "/runtime"]
        with tempfile.TemporaryFile() as output, tempfile.TemporaryFile() as errors:
            process = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=output, stderr=errors, preexec_fn=limits)
            try:
                process.communicate(data, timeout=30)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
                raise ValueError("external worker exceeded 30 seconds") from None
            output.seek(0)
            body = output.read((1 << 20) + 1)
            if process.returncode != 0 or len(body) > 1 << 20:
                raise ValueError("confined external worker failed")
            result = json.loads(body)
            result["request_sha256"] = hashlib.sha256(data).hexdigest()
            result["worker_sha256"] = hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
            result["isolation"] = "bubblewrap-unshare-all-readonly-v1"
            print(json.dumps(result, sort_keys=True, separators=(",", ":")))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.SubprocessError, json.JSONDecodeError) as error:
        print("accuracy worker: " + str(error), file=sys.stderr)
        sys.exit(1)
