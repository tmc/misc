import { calibrateHTTP, calibrateTCP, beginMeasurement, endMeasurement } from 'k6/x/llm-perf';

import { Counter } from 'k6/metrics';
const passed = new Counter('fixture_passed');
export const options = { vus: 1, iterations: 1, thresholds: { fixture_passed: ['count==1'] } };

function require(value, message) {
  if (!value) throw new Error(message);
}

export default function () {
  const url = __ENV.ENDPOINT_URL;
  const health = calibrateHTTP({ url, method: 'GET', count: 2, timeout: '1s', maxBodyBytes: 1024 });
  require(health.complete && health.successes === 2 && health.probes.length === 2, 'health probes');
  require(health.probes[0].trace.connect !== null && health.probes[1].trace.reused, 'reuse trace');
  require(health.probes[1].rtt === null && health.meanFirstByte !== null, 'missing RTT');
  const tcp = calibrateTCP({ url, count: 2, timeout: '1s' });
  require(tcp.complete && tcp.rttSamples === 2 && tcp.meanRTT !== null, 'TCP probes');
  require(tcp.meanFirstByte === null && tcp.resolutionDuration !== null, 'TCP provenance');
  const failed = calibrateHTTP({ url: url + '/failed', method: 'HEAD', count: 2, timeout: '1s', maxBodyBytes: 1024 });
  require(!failed.complete && failed.successes === 0 && failed.probes.length === 2, 'failed probes retained');
  require(failed.meanRTT === null && failed.meanFirstByte === null, 'missing failure means');
  let invalid = false;
  try { calibrateTCP({ url, count: 0, timeout: '1s' }); } catch (_) { invalid = true; }
  require(invalid, 'bounds');
  beginMeasurement({ runId: 'calibration-guard', maxRecords: 1 });
  let guarded = false;
  try { calibrateTCP({ url, count: 1, timeout: '1s' }); } catch (_) { guarded = true; }
  require(guarded, 'active measurement guard');
  require(endMeasurement().attempts === 0, 'probes are not inference attempts');
  passed.add(1);
}
