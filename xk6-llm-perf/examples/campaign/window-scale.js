// A single-VU request-rate controller. It caps rate; synchronous requests can
// reduce realized load. Use an arrival-rate executor for independent fixed load.
import llm from 'k6/x/llm-perf';
import { sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';
const windows = integer('SCALE_WINDOWS', 6, 2, 1000);
const period = integer('SCALE_WINDOW_MS', 5000, 100, 600000);
const minimum = integer('SCALE_MIN_REQUESTS', 2, 1, 100000);
const maximum = integer('SCALE_MAX_RATE', 32, 1, 100000);
const sustain = integer('SCALE_SUSTAIN_WINDOWS', 2, 1, windows);
const limit = integer('SCALE_LATENCY_MS', 2000, 1, 600000);
const receipts = new Counter('campaign_scale_windows');
const completed = new Counter('campaign_scale_completed');
const sustainFailed = new Counter('campaign_scale_sustain_failed');
const lag = new Trend('campaign_scale_arrival_lag', true);
export const options = { vus: 1, iterations: 1,
    thresholds: { campaign_scale_windows: ['count>=2'], campaign_scale_completed: ['count>0'], campaign_scale_sustain_failed: ['count==0'] },
};
const client = new llm.Client({ baseURL: __ENV.ENDPOINT_URL, model: __ENV.MODEL || 'fixture',
    apiKey: __ENV.OPENAI_API_KEY, strictMeasurement: true });
function integer(name, fallback, min, max) {
    const v = Number(__ENV[name] || fallback);
    if (!Number.isSafeInteger(v) || v < min || v > max) throw new Error(`invalid ${name}`);
    return v;
}
export default function () {
    sustainFailed.add(0);
    let rate = 1, lastPassing = null, sustaining = false, remaining = sustain;
    for (let window = 0; window < windows; window++) {
        const start = Date.now(), end = start + period;
        let count = 0, failures = 0, excluded = 0, dropped = 0, worst = 0, scheduled = start;
        while (scheduled < end) {
            const delay = scheduled - Date.now();
            if (delay > 0) sleep(delay / 1000);
            if (Date.now() >= end) break;
            lag.add(Math.max(0, Date.now() - scheduled));
            const response = client.chat.completions.create({
                messages: [{ role: 'user', content: 'Return ok.' }], stream: false,
                scheduledAt: scheduled,
            });
            if (Date.now() > end) excluded++;
            else if (response instanceof Error || !response.measurement ||
                response.measurement.full_response_latency === null) failures++;
            else { count++; worst = Math.max(worst, response.measurement.full_response_latency); }
            // Skip expired offered arrivals. Never silently burst catch-up work.
            scheduled += 1000 / rate;
            while (scheduled < Date.now() && scheduled < end) { scheduled += 1000 / rate; dropped++; }
        }
        if (Date.now() < end) sleep((end - Date.now()) / 1000);
        const passed = failures === 0 && count >= minimum && worst <= limit;
        receipts.add(1); completed.add(count);
        console.log(JSON.stringify({ schema_version: 1, type: 'scale_window', window,
            start, end, offered_rate: rate, realized_rate: count / (period / 1000),
            completed: count, failures, excluded, dropped, max_latency_ms: count ? worst : null,
            minimum_completed: minimum, latency_limit_ms: limit, passed, sustaining }));
        if (sustaining) { if (!passed) { sustainFailed.add(1); return; } if (--remaining === 0) return; continue; }
        if (passed) { lastPassing = rate; if (rate === maximum) sustaining = true;
            else rate = Math.min(maximum, rate * 2); }
        else { if (lastPassing === null) return; rate = lastPassing; sustaining = true; }
    }
}
