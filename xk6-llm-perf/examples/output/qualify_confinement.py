#!/usr/bin/env python3
"""Qualify the bridge mount policy on an owned, bounded Linux cgroup.

This tests confinement, not upstream graders or correctness of model answers.
"""
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile

spec = importlib.util.spec_from_file_location("bridge", Path(__file__).with_name("accuracy_external.py"))
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)
bridge.check_cgroup_limits()
with tempfile.TemporaryDirectory(prefix="qualification-", dir=Path(__file__).parent) as directory:
    root = Path(directory)
    source = root / "source"
    (source / "src").mkdir(parents=True)
    runtime = root / "runtime"
    subprocess.run(["python3", "-m", "venv", "--without-pip", str(runtime)], check=True)
    secret = root / "secret"
    secret.write_text("host-secret-not-mounted")
    probe = root / "probe.py"
    probe.write_text('''import json,os,pathlib,socket,subprocess,resource
out={}
out["rlimit_cpu"]=resource.getrlimit(resource.RLIMIT_CPU)[0]==20
out["rlimit_as"]=resource.getrlimit(resource.RLIMIT_AS)[0]==2<<30
out["rlimit_nproc"]=resource.getrlimit(resource.RLIMIT_NPROC)[0]==64
out["network_namespace"]=os.readlink("/proc/self/ns/net")
out["host_secret_absent"]=not pathlib.Path(HOST_SECRET_PATH).exists()
out["host_home_absent"]=not pathlib.Path("/root").exists()
try:
 pathlib.Path("/source/src/write").write_text("forbidden")
 out["source_readonly"]=False
except OSError:
 out["source_readonly"]=True
out["private_work_writable"]=False
pathlib.Path("/work/test").write_text("allowed")
out["private_work_writable"]=True
try:
 socket.create_connection(("127.0.0.1",9),timeout=.2)
 out["network_blocked"]=False
except OSError:
 out["network_blocked"]=True
out["environment_cleared"]="QUALIFICATION_SECRET" not in os.environ
print(json.dumps(out))
'''.replace("HOST_SECRET_PATH", repr(str(secret))))
    import os
    os.environ["QUALIFICATION_SECRET"] = "not-for-sandbox"
    command = bridge.sandbox_command("bwrap", source, runtime, probe)
    command += ["/runtime/bin/python", "/worker.py"]
    result = subprocess.run(command, capture_output=True, text=True, timeout=5, preexec_fn=bridge.limits)
    if result.returncode:
        raise RuntimeError("confinement unavailable: " + result.stderr)
    receipt = json.loads(result.stdout)
    receipt["network_namespace_distinct"] = receipt["network_namespace"] != os.readlink("/proc/self/ns/net")
    if not all(value for key, value in receipt.items() if key != "network_namespace"):
        raise RuntimeError("confinement negative control failed: " + json.dumps(receipt))
    probe.write_text("import subprocess,time\nsubprocess.Popen(['/runtime/bin/python','-c','import time;time.sleep(60)'])\ntime.sleep(60)\n")
    process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, preexec_fn=bridge.limits)
    try:
        process.communicate(timeout=1)
        raise RuntimeError("timeout control unexpectedly completed")
    except subprocess.TimeoutExpired:
        process.kill()
        process.communicate(timeout=3)
    import time
    time.sleep(.1)
    children = []
    for entry in Path("/proc").iterdir():
        if entry.name.isdigit():
            try:
                arguments = (entry / "cmdline").read_bytes().split(b"\0")
                if b"/worker.py" in arguments or b"/runtime/bin/python" in arguments:
                    children.append(entry.name)
            except (FileNotFoundError, ProcessLookupError):
                pass
    receipt["timeout_process_tree_terminated"] = not children
    if children:
        raise RuntimeError("timeout retained confined descendants: " + str(children))
    print(json.dumps(receipt, sort_keys=True))
