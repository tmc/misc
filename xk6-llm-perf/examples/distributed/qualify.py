#!/usr/bin/env python3
"""Run and clean up one operator qualification in a new, owned namespace.

Artifacts are lifecycle evidence. They do not establish clock synchronization or
complete participation; retrieve worker JSON and supply independent clock receipts.
"""
import argparse
import json
import os
import selectors
from pathlib import Path
import subprocess
import signal
import time
import uuid


OUTPUT_LIMIT = 1024 * 1024


def bounded_command(command, timeout):
    """Capture at most OUTPUT_LIMIT bytes from each pipe; kill on overflow."""
    buffers = {"stdout": bytearray(), "stderr": bytearray()}
    truncated = {"stdout": False, "stderr": False}
    timed_out = False
    deadline = time.monotonic() + timeout
    with subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE) as process:
        try:
            with selectors.DefaultSelector() as selector:
                selector.register(process.stdout, selectors.EVENT_READ, "stdout")
                selector.register(process.stderr, selectors.EVENT_READ, "stderr")
                while selector.get_map():
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        timed_out = True
                        break
                    for key, _ in selector.select(remaining):
                        chunk = os.read(key.fileobj.fileno(), 65536)
                        if not chunk:
                            selector.unregister(key.fileobj)
                            continue
                        name = key.data
                        available = OUTPUT_LIMIT - len(buffers[name])
                        buffers[name].extend(chunk[:available])
                        if len(chunk) > available:
                            truncated[name] = True
                            break
                    if any(truncated.values()):
                        break
            if timed_out or any(truncated.values()):
                process.kill()
            # A process may close its pipes and continue running.
            try:
                process.wait(timeout=max(0.001, deadline - time.monotonic()))
            except subprocess.TimeoutExpired:
                timed_out = True
                process.kill()
                process.wait(timeout=5)
        finally:
            if process.poll() is None:
                process.kill()
                process.wait(timeout=5)
    return {"command": command, "returncode": None if timed_out else process.returncode,
            "timed_out": timed_out, "timeout_seconds": timeout,
            "stdout": buffers["stdout"].decode(errors="replace"),
            "stderr": buffers["stderr"].decode(errors="replace"),
            "stdout_truncated": truncated["stdout"], "stderr_truncated": truncated["stderr"],
            "capture_complete": not timed_out and not any(truncated.values())}


def bounded_json(text):
    """Refuse oversized or excessively nested kubectl metadata before parsing."""
    if len(text) > OUTPUT_LIMIT or len(text.encode()) > OUTPUT_LIMIT:
        raise ValueError("kubectl JSON exceeds byte budget")
    depth, tokens = 0, 0
    quoted, escaped = False, False
    for char in text:
        if quoted:
            if escaped:
                escaped = False
            elif char == "\\":
                escaped = True
            elif char == '"':
                quoted = False
        elif char == '"':
            quoted = True
        elif char in "[{":
            depth += 1
            tokens += 1
        elif char in "]}":
            depth -= 1
        elif char in ",:":
            tokens += 1
        if depth > 64 or tokens > 100000:
            raise ValueError("kubectl JSON exceeds structure budget")
    value = json.loads(text)
    if not isinstance(value, dict):
        raise ValueError("kubectl JSON must be an object")
    return value


class Qualification:
    def __init__(self, context, output, timeout, cancel_after):
        self.context = context
        self.output = output
        self.timeout = timeout
        self.cancel_after = cancel_after
        self.nonce = uuid.uuid4().hex
        self.namespace = "llm-qualification-" + self.nonce[:12]
        self.uid = None
        self.number = 0
        self.deadline = None

    def kubectl(self, *args, required=True):
        self.number += 1
        command = ["kubectl", "--context", self.context, "--request-timeout=15s", *args]
        timeout = 90 if args[0] == "wait" else 30
        if self.deadline is not None:
            timeout = min(timeout, max(0.001, self.deadline - time.monotonic()))
        receipt = bounded_command(command, timeout)
        (self.output / (str(self.number) + ".json")).write_text(json.dumps(receipt, indent=2))
        if receipt["timed_out"]:
            raise subprocess.TimeoutExpired(command, timeout, output=receipt["stdout"], stderr=receipt["stderr"])
        if not receipt["capture_complete"]:
            raise RuntimeError("kubectl output truncated; see recorded command " + str(self.number))
        result = subprocess.CompletedProcess(command, receipt["returncode"], receipt["stdout"], receipt["stderr"])
        if required and result.returncode:
            raise RuntimeError("kubectl failed; see recorded command " + str(self.number))
        return result

    def validate(self, manifest):
        result = self.kubectl("create", "--dry-run=client", "-n", self.namespace, "-f", str(manifest), "-o", "json")
        value = bounded_json(result.stdout)
        items = value.get("items", [value]) if value.get("kind") == "List" else [value]
        kinds = []
        for item in items:
            kind = item.get("kind")
            if kind not in ("TestRun", "PersistentVolumeClaim"):
                raise ValueError("manifest may contain only TestRun and PersistentVolumeClaim")
            if item.get("metadata", {}).get("namespace") not in (None, self.namespace):
                raise ValueError("manifest must not select another namespace")
            kinds.append(kind)
        if kinds.count("TestRun") != 1:
            raise ValueError("manifest must contain exactly one TestRun")
        return next(item["metadata"]["name"] for item in items if item["kind"] == "TestRun")

    def capture(self):
        # Diagnostics get their own aggregate budget so a failed cluster cannot
        # postpone UID-safe cleanup once per pod or command.
        self.deadline = time.monotonic() + 60
        failures = []

        def attempt(*args):
            if time.monotonic() >= self.deadline:
                failures.append("capture deadline exceeded")
                return None
            try:
                result = self.kubectl(*args, required=False)
                if result.returncode:
                    failures.append("capture command failed: " + args[0] + " " + args[1])
                return result
            except Exception as error:
                failures.append(type(error).__name__ + ": " + str(error))
                return None

        try:
            attempt("get", "testruns,jobs,pods,pvc", "-n", self.namespace, "-o", "json")
            attempt("get", "events", "-n", self.namespace, "-o", "json")
            pods = attempt("get", "pods", "-n", self.namespace, "-o", "json")
            if pods is not None and pods.returncode == 0:
                try:
                    items = bounded_json(pods.stdout).get("items", [])
                    if len(items) > 16:
                        failures.append("pod log capture truncated to 16 pods")
                    for pod in items[:16]:
                        attempt("logs", pod["metadata"]["name"], "-n", self.namespace,
                                "--all-containers=true", "--prefix=true", "--tail=1000", "--limit-bytes=1048576")
                except (ValueError, KeyError, TypeError) as error:
                    failures.append("invalid pod inventory: " + str(error))
        finally:
            self.deadline = None
            (self.output / "capture.json").write_text(json.dumps({"complete": not failures, "errors": failures}, indent=2))

    def cleanup(self):
        if self.uid is None:
            return
        result = self.kubectl("get", "namespace", self.namespace, "-o", "json")
        if bounded_json(result.stdout)["metadata"]["uid"] != self.uid:
            raise RuntimeError("namespace identity changed; refusing deletion")
        options = self.output / "delete-options.json"
        options.write_text(json.dumps({"apiVersion": "v1", "kind": "DeleteOptions",
                                      "preconditions": {"uid": self.uid}}))
        # The GET is diagnostic; only the API precondition binds deletion to the
        # owned object if the namespace is replaced between GET and DELETE.
        result = self.kubectl("delete", "--raw=/api/v1/namespaces/" + self.namespace,
                              "-f", str(options), required=False)
        if result.returncode:
            if "Conflict" in result.stderr or "409" in result.stderr:
                raise RuntimeError("namespace ownership lost; refusing deletion")
            raise RuntimeError("namespace deletion failed; see recorded command " + str(self.number))
        self.kubectl("wait", "--for=delete", "namespace/" + self.namespace, "--timeout=60s")

    def recover_ownership(self):
        result = self.kubectl("get", "namespace", self.namespace, "-o", "json", required=False)
        if result.returncode:
            raise RuntimeError("namespace creation outcome unknown; inspect recorded namespace")
        metadata = bounded_json(result.stdout).get("metadata", {})
        if metadata.get("labels", {}).get("llm-perf-qualification-owner") != self.nonce:
            raise RuntimeError("namespace ownership differs; refusing deletion")
        uid = metadata.get("uid")
        if not isinstance(uid, str) or not uid:
            raise RuntimeError("namespace ownership UID unavailable")
        self.uid = uid

    def run(self, manifest, assets):
        name = self.validate(manifest)
        self.kubectl("version", "-o", "json")
        self.kubectl("get", "crd", "testruns.k6.io", "-o", "json")
        namespace_file = self.output / "namespace.json"
        namespace_file.write_text(json.dumps({"apiVersion": "v1", "kind": "Namespace",
            "metadata": {"name": self.namespace, "labels": {"llm-perf-qualification-owner": self.nonce}}}))
        # The nonce permits recovery when the API created the namespace but its
        # response was lost. An existing namespace without this nonce is never owned.
        try:
            self.kubectl("create", "-f", str(namespace_file), "-o", "json")
            self.recover_ownership()
            (self.output / "identity.json").write_text(json.dumps({
                "context": self.context, "namespace": self.namespace, "uid": self.uid, "owner": self.nonce}))
            self.kubectl("create", "configmap", "llm-perf-assets", "-n", self.namespace,
                         "--from-file=" + str(assets / "worker.js"), "--from-file=" + str(assets / "rows.json"))
            self.deadline = time.monotonic() + self.timeout
            self.kubectl("apply", "--dry-run=server", "-n", self.namespace, "-f", str(manifest))
            self.kubectl("apply", "-n", self.namespace, "-f", str(manifest))
            start = self.deadline - self.timeout
            while True:
                elapsed = time.monotonic() - start
                if self.cancel_after is not None and elapsed >= self.cancel_after:
                    return "cancelled"
                if elapsed >= self.timeout:
                    raise TimeoutError("operator qualification exceeded its deadline")
                result = self.kubectl("get", "testrun", name, "-n", self.namespace, "-o", "json")
                stage = bounded_json(result.stdout).get("status", {}).get("stage")
                if stage == "finished":
                    return "finished; worker accounting and clock qualification still required"
                if stage == "error":
                    raise RuntimeError("operator reported startup or execution failure")
                time.sleep(2)
        finally:
            self.deadline = None
            status = {"namespace": self.namespace, "owner": self.nonce, "complete": False}
            try:
                if self.uid is None:
                    self.recover_ownership()
                try:
                    self.capture()
                finally:
                    self.cleanup()
                status["complete"] = True
            except BaseException as error:
                status["error"] = type(error).__name__ + ": " + str(error)
                raise
            finally:
                (self.output / "cleanup.json").write_text(json.dumps(status, indent=2))


def interrupt(signum, frame):
    raise KeyboardInterrupt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--context", required=True)
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--assets", type=Path, default=Path(__file__).resolve().parent)
    parser.add_argument("--timeout", type=float, default=300)
    parser.add_argument("--cancel-after", type=float)
    args = parser.parse_args()
    if not 0 < args.timeout <= 600 or (args.cancel_after is not None and not 0 < args.cancel_after < args.timeout):
        parser.error("timeouts must be positive and at most 600 seconds")
    args.output.mkdir(parents=True, exist_ok=False)
    signal.signal(signal.SIGTERM, interrupt)
    qualification = Qualification(args.context, args.output, args.timeout, args.cancel_after)
    result = qualification.run(args.manifest.resolve(), args.assets.resolve())
    print(result)


if __name__ == "__main__":
    main()
