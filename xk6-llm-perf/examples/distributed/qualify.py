#!/usr/bin/env python3
"""Run and clean up one operator qualification in a new, owned namespace.

Artifacts are lifecycle evidence. They do not establish clock synchronization or
complete participation; retrieve worker JSON and supply independent clock receipts.
"""
import argparse
import json
from pathlib import Path
import subprocess
import signal
import time
import uuid


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

    def kubectl(self, *args, required=True):
        self.number += 1
        command = ["kubectl", "--context", self.context, "--request-timeout=15s", *args]
        result = subprocess.run(command, capture_output=True, text=True, timeout=90 if args[0] == "wait" else 30)
        (self.output / (str(self.number) + ".json")).write_text(json.dumps({
            "command": command, "returncode": result.returncode,
            "stdout": result.stdout, "stderr": result.stderr}, indent=2))
        if required and result.returncode:
            raise RuntimeError("kubectl failed; see recorded command " + str(self.number))
        return result

    def validate(self, manifest):
        result = self.kubectl("create", "--dry-run=client", "-n", self.namespace, "-f", str(manifest), "-o", "json")
        value = json.loads(result.stdout)
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
        self.kubectl("get", "testruns,jobs,pods,pvc", "-n", self.namespace, "-o", "json", required=False)
        self.kubectl("get", "events", "-n", self.namespace, "-o", "json", required=False)
        pods = self.kubectl("get", "pods", "-n", self.namespace, "-o", "json", required=False)
        if pods.returncode == 0:
            for pod in json.loads(pods.stdout).get("items", []):
                self.kubectl("logs", pod["metadata"]["name"], "-n", self.namespace,
                             "--all-containers=true", "--prefix=true", required=False)

    def cleanup(self):
        if self.uid is None:
            return
        result = self.kubectl("get", "namespace", self.namespace, "-o", "json")
        if json.loads(result.stdout)["metadata"]["uid"] != self.uid:
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
        metadata = json.loads(result.stdout).get("metadata", {})
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
            self.kubectl("apply", "--dry-run=server", "-n", self.namespace, "-f", str(manifest))
            self.kubectl("apply", "-n", self.namespace, "-f", str(manifest))
            start = time.monotonic()
            while True:
                elapsed = time.monotonic() - start
                if self.cancel_after is not None and elapsed >= self.cancel_after:
                    return "cancelled"
                if elapsed >= self.timeout:
                    raise TimeoutError("operator qualification exceeded its deadline")
                result = self.kubectl("get", "testrun", name, "-n", self.namespace, "-o", "json")
                stage = json.loads(result.stdout).get("status", {}).get("stage")
                if stage == "finished":
                    return "finished; worker accounting and clock qualification still required"
                if stage == "error":
                    raise RuntimeError("operator reported startup or execution failure")
                time.sleep(2)
        finally:
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
