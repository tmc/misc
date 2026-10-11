#!/usr/bin/env python3
"""Bounded functional control of serving manifests, work, lifecycle and pairs.

This runs owned CPU HTTP fixtures, not models, GPUs or a performance campaign.
"""
import argparse
import hashlib
import http.server
import json
import os
import pathlib
import platform
import signal
import socket
import subprocess
import sys

BODY = b'{"model":"capacity-fixture","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}'
GEOMETRY = 'openai-chat/json/prompt-utf8-1/content-utf8-2/output-reported-1/v1'


def sha(path):
    h = hashlib.sha256()
    with open(path, 'rb') as stream:
        for block in iter(lambda: stream.read(1024*1024), b''):
            h.update(block)
    return h.hexdigest()


def encoded(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':')).encode()


def content_sha(value):
    return hashlib.sha256(value).hexdigest()


def manifest(endpoint, workload):
    settings = dict(cache='disabled', compile='disabled', device='cpu',
                    dtype='fixture-bytes', max_output_tokens=1,
                    resources=dict(handler_threads=1), scheduler='sequential-http',
                    seed=0, temperature=0, top_k=0, top_p=1)
    return dict(schema_version=1,
        arm=dict(binary_digest=sha(sys.executable),source_revision=sha(__file__),
                 backend='fixed-response-python',endpoint_identity=endpoint),
        model_digest=content_sha(BODY),
        tokenizer_digest=content_sha(b'fixture-reported-count-one-v1'),
        template_digest=content_sha(b'prompt-x-v1'),workload_digest=workload,
        environment_digest=content_sha(encoded(dict(machine=platform.machine(),
            kernel=platform.release(),python=platform.python_version(),cpus=os.cpu_count()))),
        settings={key:dict(configured=value,effective=value,
                    evidence_digest=content_sha(encoded({key:value})))
                  for key,value in settings.items()})


def valid_fixture_request(request):
    # CompletionRequest uses omitempty for stream=false. OpenAI's default is
    # nonstreaming; explicit null and nonboolean values are not that default.
    return (isinstance(request, dict)
            and request.get('model') == 'capacity-fixture'
            and request.get('messages') == [dict(role='user', content='x')]
            and request.get('stream', False) is False
            and type(request.get('max_tokens')) is int
            and request['max_tokens'] == 1)


def serve(a):
    observed = encoded(manifest(a.endpoint_id, a.workload_sha))
    class Handler(http.server.BaseHTTPRequestHandler):
        def setup(self):
            super().setup(); self.connection.settimeout(1)
        def reply(self, body, content_type='application/json'):
            self.send_response(200);self.send_header('Content-Type',content_type)
            self.send_header('Content-Length',str(len(body)));self.end_headers()
            self.wfile.write(body)
        def do_GET(self):
            if self.path == '/observe': self.reply(observed)
            elif self.path in ('/ready','/drain'): self.reply(b'{}')
            else: self.send_error(404)
        def do_POST(self):
            length=int(self.headers.get('Content-Length','0'))
            if not 0<=length<=1024*1024:self.send_error(413);return
            body=self.rfile.read(length)
            if self.path != '/v1/chat/completions':self.send_error(404);return
            request=json.loads(body)
            if not valid_fixture_request(request):
                self.send_error(400);return
            with open(a.wire_file,'a') as stream:
                stream.write(json.dumps(dict(path=self.path,request_sha256=content_sha(body),
                    request_body_bytes=len(body),response_body_bytes=len(BODY)))+'\n')
            self.reply(BODY)
        def log_message(self,*_):pass
    server=http.server.HTTPServer(('127.0.0.1',a.port),Handler)
    server.serve_forever(poll_interval=.1)


def paired_policy():
    """The explicit disabled SLO and measurement population used by all arms."""
    return dict(measurement_policy='dispatch-content-v1/success-token-counts-v1/success-slo-v1',
                window_policy='attempt started and completed inside window; successful token totals',
                warmup_policy='phase=measurement and per-client warmup exhausted',
                slo=dict(max_ttft_ms=0, max_ttfo_ms=0, max_chunk_latency_ms=0, max_tpot_ms=0, max_e2e_ms=0))


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--fixture-server',action='store_true')
    p.add_argument('--port',type=int);p.add_argument('--endpoint-id')
    p.add_argument('--workload-sha');p.add_argument('--wire-file')
    p.add_argument('--k6');p.add_argument('--llm-serving');p.add_argument('--capacity')
    p.add_argument('--output')
    a=p.parse_args()
    if a.fixture_server:
        if not a.port or not a.endpoint_id or not a.workload_sha or not a.wire_file:p.error('fixture parameters required')
        serve(a);return
    if not all((a.k6,a.llm_serving,a.capacity,a.output)):p.error('--k6 --llm-serving --capacity --output required')
    root=pathlib.Path(a.output).resolve();root.mkdir(parents=True,exist_ok=False)
    producer=pathlib.Path(__file__).resolve()
    k6=str(pathlib.Path(a.k6).resolve());cli=str(pathlib.Path(a.llm_serving).resolve())
    capacity=pathlib.Path(a.capacity).resolve()
    capacity_receipt=json.loads((capacity/'receipt.json').read_text())
    capacity_producer_sha=sha(producer.with_name('capacity-probe.py'))
    if capacity_receipt['probe_sha256']!=capacity_producer_sha or sha(capacity/'producer.py')!=capacity_producer_sha:
        raise ValueError('capacity producer differs from the installed pinned harness')
    if capacity_receipt['binary_sha256']!=sha(k6):raise ValueError('capacity client binary mismatch')
    source='''import {check} from 'k6';
import llm from 'k6/x/llm-perf';
import exec from 'k6/execution';
import http from 'k6/http';
export const options={scenarios:{cell:{executor:'constant-arrival-rate',rate:10,timeUnit:'1s',duration:'2s',preAllocatedVUs:4,maxVUs:4}},thresholds:{checks:['rate==1'],llm_failures:['count==0'],llm_work_rejected:['count==0'],llm_work_unavailable:['count==0']}};
const client=new llm.Client({baseURL:__ENV.BASE_URL,model:'capacity-fixture',timeout:'1s',maxConnsPerHost:16,maxTTFT:0,maxTTFO:0,maxTokenLatency:0});
export function setup(){
 const response=http.get(__ENV.OBSERVE_URL,{timeout:'1s'});
 if(response.status!==200)throw new Error('observation unavailable');
 llm.beginMeasurement({runId:__ENV.RUN_ID,maxRecords:100,servingManifest:response.json()});
}
export default function(){
 const scheduledAt=exec.scenario.startTime+exec.scenario.iterationInTest*100;
 const result=client.chat.completions.create({messages:[{role:'user',content:'x'}],stream:false,max_tokens:1,scheduledAt,work:{expectedModel:'capacity-fixture',allowedFinishReasons:['stop'],minOutputTokens:1,maxOutputTokens:1,requireExactUsage:true}});
 if(!check(result,{'protocol success':r=>r.measurement.outcome==='success','accepted work':r=>r.measurement.work.status==='accepted'}))throw new Error('work not accepted');
}
export function teardown(){llm.endMeasurement();}
export function handleSummary(){return {[__ENV.RUN_FILE]:JSON.stringify(llm.measurementSummary())};}
'''
    script=root/'load.js';script.write_text(source);workload=sha(script)
    runs=[]
    # Seed1 PlanPairs is confirmed by the CLI; the preregistered two-pair order is balanced.
    for pair,order in enumerate(('AB','BA')):
        for arm in order:
            endpoint=f'fixture-pair-{pair}-{arm}'
            directory=root/endpoint;directory.mkdir()
            with socket.socket() as sock:
                sock.bind(('127.0.0.1',0));port=sock.getsockname()[1]
            url=f'http://127.0.0.1:{port}'
            observation=encoded(manifest(endpoint,workload))
            (directory/'expected-observation.json').write_bytes(observation)
            wire=str(directory/'wire.jsonl');run=str(directory/'run.json')
            lifecycle=dict(server=dict(path=sys.executable,sha256=sha(sys.executable),
                args=[str(producer),'--fixture-server','--port',str(port),'--endpoint-id',endpoint,
                      '--workload-sha',workload,'--wire-file',wire]),
                workload=dict(path=k6,sha256=sha(k6),args=['run','--no-color',str(script)],
                    env=[f'BASE_URL={url}/v1',f'OBSERVE_URL={url}/observe',f'RUN_ID={endpoint}',f'RUN_FILE={run}']),
                ready=dict(url=url+'/ready',status=200),observe=dict(url=url+'/observe',status=200),
                expected_endpoint_identity=endpoint,expected_observation_sha256=content_sha(observation),
                warmup=[dict(url=url+'/v1/chat/completions',method='POST',body=json.dumps(dict(model='capacity-fixture',messages=[dict(role='user',content='x')],stream=False,max_tokens=1)),status=200)],drain=dict(url=url+'/drain',status=200),
                startup_ms=5000,workload_ms=10000,stop_ms=2000)
            runs.append(dict(pair=pair,arm=arm,lifecycle=lifecycle,run_artifact=run,
                capacity_artifact=str(capacity),capacity_producer_sha256=capacity_producer_sha,
                workload_geometry=GEOMETRY,offered_per_second=10,
                capacity_bounds=dict(max_lag_milliseconds=100,max_cpu_percent=400,
                    max_rss_bytes=1073741824,max_connection_fraction=.9,max_pipeline_wait_milliseconds=100)))
    config=dict(plan=dict(version=1,seed=1,orders=['AB','BA'],metrics=['full_response_latency.mean'],primary='full_response_latency.mean',policy=paired_policy()),runs=runs)
    config_file=root/'paired-config.json';config_file.write_bytes(encoded(config))
    command=[cli,'paired','-config',str(config_file),'-output',str(root/'paired-receipt.json')]
    process=None;failure=None;rc=-1
    try:
        with (root/'cli.log').open('w') as log:
            process=subprocess.Popen(command,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
            rc=process.wait(timeout=90)
    except Exception as exc:failure=str(exc)
    finally:
        if process is not None and process.poll() is None:
            os.killpg(process.pid,signal.SIGTERM)
            try:process.wait(timeout=15)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid,signal.SIGKILL);process.wait(timeout=2)
        receipt=dict(schema_version=1,kind='registered-k6-serving-qualification-functional-control',
            producer_sha256=sha(producer),k6_sha256=sha(k6),cli_sha256=sha(cli),
            load_sha256=workload,config_sha256=sha(config_file),command=command,exit_code=rc,failure=failure,
            limitations=['CPU fixed-response fixture only; no native engine parity or model performance claim',
                        'Two independent pairs exercise the pipeline; confidence intervals are not a performance finding'])
        (root/'control-receipt.json').write_bytes(encoded(receipt))
    if rc:raise SystemExit(rc)
    paired=json.loads((root/'paired-receipt.json').read_text())
    if not paired.get('complete') or len(paired.get('attempts',[]))!=4 or not paired.get('result',{}).get('qualified'):
        raise RuntimeError('chained fixture control did not qualify; inspect paired-receipt.json')
    for attempt in paired['attempts']:
        if not attempt['lifecycle']['complete'] or attempt.get('capacity_status')!='SAMPLED_CAPACITY':
            raise RuntimeError('missing lifecycle or sampled capacity qualification')
    print(json.dumps(dict(status='PASS',receipt=str(root/'control-receipt.json'))))

if __name__=='__main__':main()
