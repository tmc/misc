// Control traffic uses k6/http, with a separate metric tag. It never calls the
// inference client, and therefore never increments llm_* request counters.
import http from 'k6/http';

export function controlHook(origin, path, options = {}) {
    if (typeof origin !== 'string' || !/^https?:\/\/[^/?#@]+$/.test(origin) ||
        typeof path !== 'string' || !/^\/[A-Za-z0-9/_-]*$/.test(path) || path.includes('..')) throw new Error('invalid hook URL');
    const timeout = options.timeout === undefined ? 5 : options.timeout;
    if (!Number.isFinite(timeout) || timeout <= 0 || timeout > 60) throw new Error('invalid hook timeout');
    // No retries: a lost response must not silently repeat a state-changing hook.
    const response = http.post(origin + path, null, {timeout: `${timeout}s`, redirects: 0,
        responseType: 'none', tags: {traffic: 'control', name: `control:${path}`},
        headers: options.headers || {}});
    const receipt = {path, status: response.status, duration: response.timings.duration, success: response.status >= 200 && response.status < 300};
    if (!receipt.success) throw new Error(`control hook failed (${receipt.status}): ${path}`);
    return receipt;
}

// Each logical request selects exactly one client. A failed call is returned;
// automatic replay on another endpoint would duplicate inference work.
export function router(clients, policy = 'round-robin', seed = 0) {
    if (!Array.isArray(clients) || !clients.length || clients.length > 256 ||
        !['round-robin', 'random'].includes(policy) || !Number.isSafeInteger(seed)) throw new Error('invalid routing configuration');
    let next = 0, state = seed >>> 0;
    return request => {
        state = (Math.imul(state, 1664525) + 1013904223) >>> 0;
        const index = policy === 'round-robin' ? next++ % clients.length : Math.floor(state / 4294967296 * clients.length);
        return {endpointIndex: index, result: clients[index].chat.completions.create(request)};
    };
}
