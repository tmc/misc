// Deterministic workload helpers. The caller retains source assets and hashes.
import { sleep } from 'k6';
import crypto from 'k6/crypto';

const maxTokens = 1000000;
function tokenBudget(value, minimum = 0) {
    return Number.isSafeInteger(value) && value >= minimum && value <= maxTokens;
}

export function random(seed) {
    if (!Number.isSafeInteger(seed)) throw new Error('seed must be a safe integer');
    let state = seed >>> 0;
    return () => {
        state = (Math.imul(state, 1664525) + 1013904223) >>> 0;
        return (state + 0.5) / 4294967296;
    };
}

// Gamma uses Marsaglia-Tsang rejection, with a bounded retry budget.
function gamma(shape, rng) {
    if (shape < 1) return gamma(shape + 1, rng) * Math.pow(rng(), 1 / shape);
    const d = shape - 1 / 3;
    const c = 1 / Math.sqrt(9 * d);
    for (let i = 0; i < 10000; i++) {
        const x = Math.sqrt(-2 * Math.log(rng())) * Math.cos(2 * Math.PI * rng());
        const v = Math.pow(1 + c * x, 3);
        if (v <= 0) continue;
        const u = rng();
        if (u < 1 - 0.0331 * Math.pow(x, 4) || Math.log(u) < x * x / 2 + d * (1 - v + Math.log(v))) return d * v;
    }
    throw new Error('gamma sampling exhausted retry budget');
}

// Returns absolute offsets in milliseconds, beginning at zero. Shape=1 gives
// exponential intervals for Poisson arrivals. These are intended arrivals;
// actual dispatch may lag when workers are busy.
export function arrivals(count, rate, mode, seed, shape = 1) {
    if (!Number.isInteger(count) || count < 0 || count > 100000 ||
        !Number.isFinite(rate) || rate <= 0 || !Number.isFinite(shape) || shape <= 0 || shape > 10000 ||
        !['constant', 'poisson', 'gamma'].includes(mode)) throw new Error('invalid arrival distribution');
    const rng = random(seed);
    const offsets = [];
    let at = 0;
    for (let i = 0; i < count; i++) {
        offsets.push(at);
        const delta = mode === 'constant' ? 1 : mode === 'poisson' ? -Math.log(rng()) : gamma(shape, rng) / shape;
        at += delta * 1000 / rate;
        if (!Number.isFinite(at)) throw new Error('arrival offset overflow');
    }
    return offsets;
}

// Loads already parsed JSON; k6 open/SharedArray owns asset acquisition.
export function dataset(rows) {
    if (!Array.isArray(rows) || rows.length === 0 || rows.length > 100000) throw new Error('dataset must contain 1..100000 rows');
    const ids = new Set();
    let bytes = 0;
    return rows.map((row, index) => {
        if (!row || typeof row.prompt !== 'string' || row.prompt.length > 1024 * 1024) throw new Error('invalid dataset prompt');
        bytes += row.prompt.length * 3;
        if (bytes > 16 * 1024 * 1024) throw new Error('dataset exceeds aggregate payload budget');
        const id = row.id === undefined ? String(index) : String(row.id);
        if (id.length > 256 || ids.has(id)) throw new Error('duplicate or oversized dataset row identity');
        ids.add(id);
        return { ...row, id };
    });
}

export function sample(rows, count, seed) {
    if (!Number.isInteger(count) || count < 0 || count > 100000 || !Array.isArray(rows) || !rows.length || rows.length > 100000) throw new Error('invalid sample size');
    const rng = random(seed);
    return Array.from({ length: count }, () => rows[Math.floor(rng() * rows.length)]);
}

// Prefix strings are authored text. Exact token budgets require tokenizer
// verification; repeated words do not claim a realized token distribution.
export function prefixPrompt(prefix, suffix, maxBytes = 1024 * 1024) {
    if (typeof prefix !== 'string' || typeof suffix !== 'string' || !Number.isInteger(maxBytes) || maxBytes < 0 || maxBytes > 1024 * 1024) throw new Error('invalid prefix payload');
    const text = prefix + suffix;
    // UTF-8 upper bound avoids exceeding the declared byte budget.
    if (text.length * 3 > maxBytes) throw new Error('prefix payload exceeds byte budget');
    return text;
}

export function waitArrival(epoch, offset, maxLag) {
    if (![epoch, offset, maxLag].every(Number.isFinite) || offset < 0 || maxLag < 0) throw new Error('invalid arrival bounds');
    const scheduledAt = epoch + offset;
    if (!Number.isSafeInteger(Math.floor(scheduledAt))) throw new Error('arrival epoch overflow');
    if (scheduledAt > Date.now()) sleep((scheduledAt - Date.now()) / 1000);
    const lag = Date.now() - scheduledAt;
    return { scheduledAt, lag, dropped: lag > maxLag };
}

// Dependencies execute once in input order among ready nodes. A failed parent
// skips its descendants. This synchronous adapter belongs to one VU; each
// conversation's context remains with that VU.
export function dag(nodes, execute) {
    if (!Array.isArray(nodes) || nodes.length > 10000 || typeof execute !== 'function') throw new Error('invalid DAG');
    const pending = new Map();
    for (const node of nodes) {
        if (!node || typeof node.id !== 'string' || node.id.length > 256 || !Array.isArray(node.dependsOn) || node.dependsOn.length > 1000 || pending.has(node.id)) throw new Error('invalid DAG node identity');
        pending.set(node.id, node);
    }
    if (nodes.reduce((sum, node) => sum + node.dependsOn.length, 0) > 100000) throw new Error('DAG exceeds dependency budget');
    for (const node of nodes) {
        if (new Set(node.dependsOn).size !== node.dependsOn.length || node.dependsOn.some(id => !pending.has(id))) throw new Error('invalid DAG dependency');
    }
    const remaining = new Map(pending);
    const ordered = [];
    const ready = new Set();
    while (remaining.size) {
        let progress = false;
        for (const [id, node] of remaining) {
            if (node.dependsOn.some(parent => !ready.has(parent))) continue;
            ordered.push(node);
            ready.add(id);
            remaining.delete(id);
            progress = true;
        }
        if (!progress) throw new Error('DAG contains a cycle');
    }
    const results = new Map();
    for (const node of ordered) {
        const failed = node.dependsOn.some(parent => results.get(parent).outcome !== 'success');
        let result;
        if (failed) result = { outcome: 'skipped' };
        else {
            try {
                const value = execute(node, results);
                if (value instanceof Error) throw value;
                result = { outcome: 'success', value };
            } catch (error) { result = { outcome: 'failure', error: String(error) }; }
        }
        results.set(node.id, result);
    }
    return results;
}

// Each turn may supply request options, such as max_tokens, work or engine
// extensions like {ignore_eos: true}. The helper owns messages and streaming;
// neither request options nor extensions can replace either.
export function conversation(client, turns, initial = [], policy = 'append') {
    if (!Array.isArray(turns) || turns.length > 1000 || !Array.isArray(initial) || initial.length > 1000 || !['append', 'independent'].includes(policy)) throw new Error('invalid conversation');
    for (const message of initial) if (!message || typeof message.role !== 'string' || typeof message.content !== 'string' || message.content.length > 1024 * 1024) throw new Error('invalid initial context');
    for (const turn of turns) if (!turn || typeof turn.prompt !== 'string' || turn.prompt.length > 1024 * 1024 || !Number.isFinite(turn.gap || 0) || (turn.gap || 0) < 0 || (turn.gap || 0) > 3600) throw new Error('invalid conversation turn');
    // fold matches keys the way encoding/json does: ASCII case plus the long s
    // and Kelvin sign, which fold to s and k.
    const fold = key => key.toLowerCase().replace(/\u017f/g, 's');
    const owned = key => ['messages', 'stream'].includes(fold(key));
    for (const turn of turns) {
        const request = turn.request;
        if (request === undefined) continue;
        if (!request || typeof request !== 'object' || Array.isArray(request) || Object.keys(request).some(owned)) throw new Error('invalid conversation request options');
        const keys = Object.keys(request).filter(key => fold(key) === 'extensions');
        if (keys.length > 1) throw new Error('invalid conversation request extensions');
        const extensions = keys.length ? request[keys[0]] : undefined;
        if (extensions !== undefined && (!extensions || typeof extensions !== 'object' || Array.isArray(extensions) || Object.keys(extensions).some(owned))) throw new Error('invalid conversation request extensions');
    }
    let messages = initial.slice();
    const results = [];
    for (const turn of turns) {
        if (turn.gap) sleep(turn.gap);
        if (policy === 'independent') messages = initial.slice();
        messages.push({ role: 'user', content: turn.prompt });
        const result = client.chat.completions.create({ ...turn.request, messages, stream: true });
        results.push(result);
        if (result instanceof Error) break;
        if (!result.choices || !result.choices.length || !result.choices[0].message) throw new Error('conversation response has no message');
        messages.push(result.choices[0].message);
    }
    return results;
}

// Strict JSON/JSONL imports keep source hash receipts outside request metadata.
export function importDataset(text, format = 'json') {
    if (typeof text !== 'string' || text.length > 16 * 1024 * 1024 || !['json', 'jsonl'].includes(format)) throw new Error('invalid dataset asset');
    const rows = format === 'json' ? JSON.parse(text) : text.split('\n').filter(line => line.trim()).map(line => JSON.parse(line));
    return dataset(rows);
}

function normal(rng) {
    return Math.sqrt(-2 * Math.log(rng())) * Math.cos(2 * Math.PI * rng());
}

// Length pairs use explicit weights, means and standard deviations. Returned
// integers are target budgets, not claims about generated/provider output.
export function sequences(pairs, count, seed) {
    if (!Array.isArray(pairs) || !pairs.length || pairs.length > 256 || !Number.isInteger(count) || count < 0 || count > 100000) throw new Error('invalid sequence distribution');
    let total = 0;
    for (const pair of pairs) {
        if (![pair.input, pair.output, pair.weight, pair.inputSD || 0, pair.outputSD || 0].every(Number.isFinite) ||
            pair.input < 1 || pair.output < 1 || pair.input > 1000000 || pair.output > 1000000 || pair.weight <= 0 ||
            (pair.inputSD || 0) < 0 || (pair.outputSD || 0) < 0 || (pair.inputSD || 0) > 1000000 || (pair.outputSD || 0) > 1000000) throw new Error('invalid sequence pair');
        total += pair.weight;
    }
    if (!Number.isFinite(total)) throw new Error('sequence weight overflow');
    const rng = random(seed);
    return Array.from({length: count}, (_, index) => {
        let choice = rng() * total, pairIndex = 0;
        for (; pairIndex < pairs.length - 1 && choice >= pairs[pairIndex].weight; pairIndex++) choice -= pairs[pairIndex].weight;
        const pair = pairs[pairIndex];
        const draw = (mean, sd) => Math.max(1, Math.min(1000000, Math.round(mean + (sd ? normal(rng) * sd : 0))));
        return {id: String(index), pairIndex, inputTokens: draw(pair.input, pair.inputSD), outputBudget: draw(pair.output, pair.outputSD)};
    });
}

// A pinned offline tokenizer supplies count(text). The realization receipt
// preserves both target and actual counts; an unrealizable budget is an error.
// Finite candidate pools avoid assuming token counts compose at text boundaries.
export function realize(targets, candidates, count, provenance) {
    if (!Array.isArray(targets) || targets.length > 100000 || !Array.isArray(candidates) || !candidates.length || candidates.length > 10000 ||
        typeof count !== 'function' || !provenance || !/^[a-f0-9]{64}$/.test(provenance.assetHash || '') || !provenance.tokenizer) throw new Error('invalid realization assets');
    const byCount = new Map();
    for (const prompt of candidates) {
        if (typeof prompt !== 'string' || prompt.length > 1024 * 1024) throw new Error('invalid candidate payload');
        const actual = count(prompt);
        if (!Number.isInteger(actual) || actual < 0 || actual > 1000000) throw new Error('invalid tokenizer count');
        if (!byCount.has(actual)) byCount.set(actual, prompt);
    }
    let bytes = 0;
    return targets.map(target => {
        if (!target || !tokenBudget(target.inputTokens) || !tokenBudget(target.outputBudget, 1)) throw new Error('invalid target budget');
        if (!byCount.has(target.inputTokens)) throw new Error(`unrealizable input token budget: ${target.inputTokens}`);
        bytes += byCount.get(target.inputTokens).length * 3;
        if (bytes > 16 * 1024 * 1024) throw new Error('realization exceeds aggregate payload budget');
        return {...target, prompt: byCount.get(target.inputTokens), realizedInputTokens: target.inputTokens, tokenizer: provenance.tokenizer, assetHash: provenance.assetHash};
    });
}

export function cacheSynthesis(prefixes, suffixes, count, provenance, bust = false) {
    if (!Array.isArray(prefixes) || !prefixes.length || !Array.isArray(suffixes) || !suffixes.length || prefixes.length * suffixes.length > 100000 ||
        typeof count !== 'function' || !provenance || !/^[a-f0-9]{64}$/.test(provenance.assetHash || '')) throw new Error('invalid cache synthesis');
    const rows = [];
    let bytes = 0;
    for (let p = 0; p < prefixes.length; p++) for (let s = 0; s < suffixes.length; s++) {
        const id = `${p}:${s}`;
        // Cache-bust marker precedes the shared prefix, changing the initial
        // prompt bytes. Provider cache behavior is a separate observation.
        const prompt = prefixPrompt(bust ? `[${id}]` + prefixes[p] : prefixes[p], suffixes[s]);
        bytes += prompt.length * 3;
        if (bytes > 16 * 1024 * 1024) throw new Error('synthesis exceeds aggregate payload budget');
        const inputTokens = count(prompt);
        if (!Number.isInteger(inputTokens) || inputTokens < 0 || inputTokens > 1000000) throw new Error('invalid synthesized count');
        rows.push({id, prompt, inputTokens, prefixGroup: bust ? id : String(p), cachePolicy: bust ? 'bust' : 'shared-prefix', assetHash: provenance.assetHash, tokenizer: provenance.tokenizer});
    }
    return rows;
}

export function trace(rows) {
    if (!Array.isArray(rows) || rows.length > 100000) throw new Error('invalid trace');
    const ids = new Set();
    let last = -1;
    return rows.map(row => {
        if (!row || typeof row.id !== 'string' || ids.has(row.id) || !Number.isFinite(row.offset) || row.offset < last || row.offset < 0 || row.offset > 86400000) throw new Error('invalid trace arrival');
        ids.add(row.id); last = row.offset;
        return {...row};
    });
}

// Accounting is retained even for work that never becomes an inference attempt.
export function replayArrival(row, epoch, maxLag, execute) {
    if (!row || typeof row.id !== 'string' || typeof execute !== 'function') throw new Error('invalid replay callback');
    const arrival = waitArrival(epoch, row.offset, maxLag);
    const receipt = {id: row.id, ...arrival, actualArrival: Date.now(), outcome: arrival.dropped ? 'dropped' : 'dispatched'};
    if (!arrival.dropped) receipt.result = execute(row, arrival);
    return receipt;
}

// Receipts come from tools/llm-tokenize. Match the exact rendered UTF-8 text,
// counting scope and asset identity; never reuse a count for modified payloads.
export function verifiedCounter(receipts, expectedAssetHash) {
    if (!Array.isArray(receipts) || !receipts.length || receipts.length > 10000 || !/^[a-f0-9]{64}$/.test(expectedAssetHash)) throw new Error('invalid token receipts');
    const counts = new Map();
    for (const receipt of receipts) {
        if (!receipt || receipt.schema_version !== 1 || receipt.tokenizer_sha256 !== expectedAssetHash || receipt.scope !== 'text without postprocessor' ||
            !/^[a-f0-9]{64}$/.test(receipt.framed_sha256 || '') || !Number.isInteger(receipt.count) || receipt.count < 0 || receipt.count > 1000000) throw new Error('invalid token receipt');
        if (counts.has(receipt.framed_sha256)) throw new Error('duplicate token receipt');
        counts.set(receipt.framed_sha256, receipt.count);
    }
    return text => {
        const hash = crypto.sha256(text, 'hex');
        if (!counts.has(hash)) throw new Error('missing exact payload token receipt');
        return counts.get(hash);
    };
}

export function assetReceipt(text) {
    if (typeof text !== 'string' || text.length > 16 * 1024 * 1024) throw new Error('invalid corpus asset');
    return {sha256: crypto.sha256(text, 'hex'), codeUnits: text.length};
}

// Describe the realized sample instead of presenting declared probabilities as
// realized percentages. Output budgets remain distinct from generated counts.
export function distributionReceipt(rows) {
    if (!Array.isArray(rows) || !rows.length || rows.length > 100000) throw new Error('invalid realized sample');
    const pairs = {};
    let inputSum = 0, outputSum = 0, maxInput = 0, minInput = Infinity;
    for (const row of rows) {
        if (!row || !tokenBudget(row.realizedInputTokens) || !tokenBudget(row.outputBudget, 1) || !Number.isInteger(row.pairIndex) || row.pairIndex < 0 || row.pairIndex > 255) throw new Error('invalid realized sequence row');
        pairs[row.pairIndex] = (pairs[row.pairIndex] || 0) + 1;
        inputSum += row.realizedInputTokens; outputSum += row.outputBudget;
        minInput = Math.min(minInput, row.realizedInputTokens); maxInput = Math.max(maxInput, row.realizedInputTokens);
    }
    return {count: rows.length, pairCounts: pairs, meanRealizedInputTokens: inputSum / rows.length, minRealizedInputTokens: minInput, maxRealizedInputTokens: maxInput, meanOutputBudget: outputSum / rows.length};
}
