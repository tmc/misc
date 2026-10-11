#!/usr/bin/env python3
"""Bounded Linux registered-k6 fixed-response client capacity control."""
import argparse
import hashlib
import http.server
import json
import os
import pathlib
import re
import signal
import subprocess
import threading
import time


def digest(path):
    h = hashlib.sha256()
    with open(path, 'rb') as stream:
        for block in iter(lambda: stream.read(1024*1024), b''):
            h.update(block)
    return h.hexdigest()


def resources(pid):
    root = pathlib.Path('/proc') / str(pid)
    fields = (root / 'stat').read_text().rsplit(')', 1)[1].split()
    sockets = 0
    descriptors = list((root / 'fd').iterdir())
    limit = None
    for line in (root / 'limits').read_text().splitlines():
        if line.startswith('Max open files'):
            raw=line.split()[3]
            limit=int(raw) if raw != 'unlimited' else None
    for fd in descriptors:
        try:
            sockets += os.readlink(fd).startswith('socket:[')
        except FileNotFoundError:
            pass
    return dict(at=time.time(), ticks=int(fields[11])+int(fields[12]),
                start_ticks=int(fields[19]), rss=int(fields[21])*os.sysconf('SC_PAGE_SIZE'),
                sockets=sockets,descriptors=len(descriptors),descriptor_limit=limit)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--k6', required=True)
    p.add_argument('--output', required=True)
    p.add_argument('--rate', type=int, default=10)
    p.add_argument('--duration', type=int, default=5)
    p.add_argument('--vus', type=int, default=4)
    p.add_argument('--max-conns-per-host',type=int,default=16)
    a = p.parse_args()
    if not (1 <= a.rate <= 1000 and 1 <= a.duration <= 30 and 1 <= a.vus <= 256 and 1 <= a.max_conns_per_host <= 256):
        p.error('probe bounds: rate1..1000 duration1..30 vus1..256')
    out = pathlib.Path(a.output); out.mkdir(parents=True, exist_ok=False)
    (out/'producer.py').write_bytes(pathlib.Path(__file__).read_bytes())
    wire = []
    class Handler(http.server.BaseHTTPRequestHandler):
        def do_POST(self):
            self.connection.settimeout(1)
            length = int(self.headers.get('Content-Length', '0'))
            if not 0 <= length <= 1024*1024:
                self.send_error(413); return
            self.rfile.read(length)
            wire.append(time.time())
            body = b'{"model":"capacity-fixture","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}'
            self.send_response(200); self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(body))); self.end_headers(); self.wfile.write(body)
        def log_message(self, *_): pass
    server = http.server.HTTPServer(('127.0.0.1',0),Handler)
    thread = threading.Thread(target=server.serve_forever,daemon=True);thread.start()
    script = '''import {sleep} from 'k6';
import exec from 'k6/execution';
import {Client,pipelineSnapshot,resetPipeline} from 'k6/x/llm-perf';
export const options={scenarios:{cell:{executor:'constant-arrival-rate',rate:RATE,timeUnit:'1s',duration:'DURATIONs',preAllocatedVUs:VUS,maxVUs:VUS}}};
export function setup(){sleep(1);}
let initialized=false;
const client=new Client({baseURL:'__TARGET__',model:'capacity-fixture',timeout:'1s',maxConnsPerHost:CONNCAP});
export default function(){
 if(!initialized){resetPipeline();initialized=true;}
 const start=Date.now();
 const result=client.chat.completions.create({messages:[{role:'user',content:'x'}],stream:false,max_tokens:1,work:{expectedModel:'capacity-fixture',allowedFinishReasons:['stop'],minOutputTokens:1,maxOutputTokens:1,requireExactUsage:true}});
 console.log('CAPACITY '+JSON.stringify({vu:exec.vu.idInTest,iteration:exec.scenario.iterationInTest,scheduledAt:exec.scenario.startTime+exec.scenario.iterationInTest*1000/RATE,lag:Math.max(0,start-(exec.scenario.startTime+exec.scenario.iterationInTest*1000/RATE)),start,finish:Date.now(),result,pipeline:pipelineSnapshot()}));
}
export function teardown(){sleep(1);}
export function handleSummary(data){return {__SUMMARY__:JSON.stringify(data)};}
'''.replace('CONNCAP',str(a.max_conns_per_host)).replace('RATE',str(a.rate)).replace('DURATION',str(a.duration)).replace('VUS',str(a.vus)).replace('__TARGET__',f'http://127.0.0.1:{server.server_port}/v1').replace('__SUMMARY__',json.dumps(str(out/'summary.json')))
    (out/'probe.js').write_text(script)
    command=[a.k6,'run','--no-color',str(out/'probe.js')]
    samples=[]
    process = None
    failure = None
    rc = -1
    try:
        with (out/'k6.log').open('w') as log:
            process=subprocess.Popen(command,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
            deadline=time.monotonic()+a.duration+15
            while process.poll() is None:
                if time.monotonic() >= deadline:
                    raise TimeoutError('bounded k6 probe deadline exceeded')
                try: samples.append(resources(process.pid))
                except FileNotFoundError:
                    if process.poll() is not None: break
                    raise
                except PermissionError:
                    if samples or time.monotonic() > deadline-a.duration-14.5: raise
                time.sleep(.05)
            rc=process.wait(timeout=2)
    except Exception as exc:
        failure = str(exc)
    finally:
        if process is not None and process.poll() is None:
            os.killpg(process.pid,signal.SIGTERM)
            try: process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid,signal.SIGKILL);process.wait(timeout=2)
        server.shutdown();server.server_close();thread.join(timeout=2)
    (out/'resources.json').write_text(json.dumps(samples))
    (out/'wire.json').write_text(json.dumps(wire))
    rows=[]
    for line in (out/'k6.log').read_text().splitlines():
        if 'CAPACITY ' in line:
            # k6 logs quote JSON; unwrap the quoted message before decoding.
            match=re.search(r'msg=("(?:\\.|[^"\\])*")',line)
            message=json.loads(match.group(1)) if match else line.split('CAPACITY ',1)[1]
            if message.startswith('CAPACITY '):message=message[len('CAPACITY '):]
            rows.append(json.loads(message))
    (out/'rows.json').write_text(json.dumps(rows))
    last={}
    for row in rows:last[row['vu']]=row['pipeline']
    ticks=os.sysconf('SC_CLK_TCK')
    cpu=[100*(b['ticks']-x['ticks'])/ticks/(b['at']-x['at']) for x,b in zip(samples,samples[1:]) if b['at']>x['at'] and b['start_ticks']==x['start_ticks']]
    receipt=dict(schema_version=1,kind='registered-k6-fixed-response-control',workload_geometry='openai-chat/json/prompt-utf8-1/content-utf8-2/output-reported-1/v1',command=command,
        binary_sha256=digest(a.k6),script_sha256=digest(out/'probe.js'),probe_sha256=digest(__file__),
        exit_code=rc,failure=failure,ticks_per_second=ticks,rate=a.rate,duration=a.duration,allocated_vus=a.vus,max_conns_per_host=a.max_conns_per_host,
        resources_sha256=digest(out/'resources.json'),wire_sha256=digest(out/'wire.json'),
        rows_sha256=digest(out/'rows.json'),
        summary_sha256=digest(out/'summary.json') if (out/'summary.json').exists() else None,
        attempted=len(rows),wire_requests=len(wire),pipeline_by_vu=last,
        max_dispatch_lag_ms=max((row['lag'] for row in rows),default=None),
        accepted=sum(row.get('result',{}).get('measurement',{}).get('work',{}).get('status')=='accepted' for row in rows),
        peak_cpu_percent=max(cpu) if cpu else None,peak_rss_bytes=max((x['rss'] for x in samples),default=None),
        peak_socket_descriptors=max((x['sockets'] for x in samples),default=None),
        peak_file_descriptors=max((x['descriptors'] for x in samples),default=None),
        file_descriptor_limit=samples[0]['descriptor_limit'] if samples else None,
        parsing_nanoseconds=sum(row["result"]["measurement"].get("parsing_latency_ms", 0)*1e6 for row in rows) if rows and all(row["result"]["measurement"].get("parsing_latency_ms") is not None for row in rows) else None,
        serialization_nanoseconds=sum(row["result"]["measurement"].get("serialization_latency_ms", 0)*1e6 for row in rows) if rows and all(row["result"]["measurement"].get("serialization_latency_ms") is not None for row in rows) else None,
        limitations=['Python fixture and client share a host; fixture CPU excluded from client PID sampler',
                      'Socket descriptor occupancy includes non-HTTP sockets and is conservative against the explicit shared transport cap',
                      'Resource sampling has finite cadence; unsampled spikes remain unknown',
                      'Parsing/serialization are operation sums, not CPU cost or production capacity proof'])
    (out/'receipt.json').write_text(json.dumps(receipt,indent=2))
    print(json.dumps(receipt,indent=2))
    if rc:raise SystemExit(rc)

if __name__=='__main__':main()
