// Deterministic workload helpers. The caller retains source assets and hashes.
import { sleep } from 'k6';

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
    return rows.map((row, index) => {
        if (!row || typeof row.prompt !== 'string' || row.prompt.length > 1024 * 1024) throw new Error('invalid dataset prompt');
        const id = row.id === undefined ? String(index) : String(row.id);
        if (ids.has(id)) throw new Error('duplicate dataset row identity');
        ids.add(id);
        return { ...row, id };
    });
}

export function sample(rows, count, seed) {
    if (!Number.isInteger(count) || count < 0 || count > 100000 || !rows.length) throw new Error('invalid sample size');
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
        if (!node || typeof node.id !== 'string' || !Array.isArray(node.dependsOn) || pending.has(node.id)) throw new Error('invalid DAG node identity');
        pending.set(node.id, node);
    }
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

export function conversation(client, turns, initial = []) {
    if (!Array.isArray(turns) || turns.length > 1000 || !Array.isArray(initial)) throw new Error('invalid conversation');
    const messages = initial.slice();
    const results = [];
    for (const turn of turns) {
        if (!turn || typeof turn.prompt !== 'string' || !Number.isFinite(turn.gap || 0) || (turn.gap || 0) < 0 || (turn.gap || 0) > 3600) throw new Error('invalid conversation turn');
        if (turn.gap) sleep(turn.gap);
        messages.push({ role: 'user', content: turn.prompt });
        const result = client.chat.completions.create({ messages, stream: true });
        results.push(result);
        if (result instanceof Error) break;
        messages.push(result.choices[0].message);
    }
    return results;
}
