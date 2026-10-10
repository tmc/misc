# Network calibration

Calibration is opt-in and bounded. Run it before `beginMeasurement`; the k6
exports reject calibration while a measurement window is active. Probe traffic
does not enter inference attempt counters. Retain the complete returned report
beside the benchmark manifest.

```javascript
import { calibrateTCP, calibrateHTTP } from 'k6/x/llm-perf';

const tcp = calibrateTCP({
  url: 'http://localhost:8000', count: 5, timeout: '250ms', interval: '10ms',
});
const health = calibrateHTTP({
  url: 'http://localhost:8000/health', method: 'HEAD', count: 5,
  timeout: '250ms', maxBodyBytes: 1024,
});
```

`calibrateTCP` resolves the host once, then opens a fresh plain TCP connection
per sample. DNS duration is separate. HTTPS targets still use plain TCP to the
HTTPS port: no TLS handshake is included. This follows pinned AIPerf's
`network_latency/probe.py` TCP-connect approach. The selected resolved address is
recorded; this does not establish coverage of other load balancer addresses.
Resolution failures retain failed samples with null RTT. Connect failures and
timeouts also retain failed samples. Context cancellation stops the sequence,
and the host API returns the partial report with an error.

`calibrateHTTP` issues serial HEAD or GET requests on the extension's shared HTTP
transport, retaining completed DNS/connect/TLS stages and connection reuse.
Redirects are returned as failed probes without following another target.
Reused sockets have null connect RTT. The HTTP first-byte duration includes
health endpoint processing and is recorded separately; it is unsuitable as a
pure network RTT basis. The Go API accepts an explicit client and requires its
transport provenance label. The k6 export supplies that label itself.

All durations use milliseconds. Counts are bounded to 1–1000; timeouts to 30s,
intervals to 1s, and the combined worst-case sequence to five minutes. HTTP body
budgets are required, at most 1MiB per probe and 16MiB across configured probes.
Credential-bearing URLs, queries and fragments are refused. Error stages are
bounded labels rather than raw errors that could reveal credentials.

`meanRTT` averages only successful fresh connection samples. `meanFirstByte`
averages only successful HTTP samples. Neither uses failed samples as zeros.
`complete` requires all configured probes to succeed; `probes` preserves the
failed population. Runtime source identities, options, probe timestamps,
resolution duration, and total blocking duration provide replay provenance.
Total duration measures calibration time, including configured waits, not its
effect on subsequent workload latency. Quantify that effect with independently
repeated calibration-enabled and disabled runs on the target network.

A supplied RTT can support separately labeled arithmetic subtraction from
request-start-anchored latencies. It does not measure server latency, remove
server queueing, or justify adjustment of ITL. Local HTTP/TLS/TCP fixtures qualify
the host implementation; live target network representativeness and probe
impact remain separate gates.
