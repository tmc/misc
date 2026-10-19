#!/usr/bin/env node
// CPU scheduling controls against the shipped helper; this is not a k6 test.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
const file = new URL('../examples/lib/workload.js', import.meta.url);
const source = (await readFile(file, 'utf8'))
    .replace("import { sleep } from 'k6';", 'function sleep() { throw new Error("unexpected sleep"); }')
    .replace("import crypto from 'k6/crypto';", '')
    .replace("import { setTimeout } from 'k6/timers';", 'const setTimeout = (fn, ms) => globalThis.dagTimer(fn, ms);');
globalThis.dagTimer = (fn, ms) => queueMicrotask(fn);
const { dagConversations } = await import('data:text/javascript;base64,' + Buffer.from(source).toString('base64'));
const msg = content => ({ role: 'user', content });
const get = (results, sessionId) => [...results.values()].find(result => result.sessionId === sessionId);
const reply = content => ({ role: 'assistant', content });
const turn = (content, fields = {}) => ({ messages: [msg(content)], ...fields });
const session = (session_id, turns) => ({ session_id, turns });
const flush = async () => { for (let i = 0; i < 32; i++) await Promise.resolve(); };
const tests = [];
const test = (name, run) => tests.push([name, run]);
test('delayed spawn overlaps parent and gates only join_at', async () => {
    const calls = [];
    let release;
    const child = new Promise(resolve => { release = resolve; });
    const pending = dagConversations([
        session('root', [turn('zero', { spawns: [{ children: ['child'], join_at: 2 }] }), turn('one'), turn('two')]),
        session('child', [turn('child')]),
    ], async (request, id) => {
        calls.push([id.sessionId, id.turnIndex, request.messages]);
        if (id.sessionId === 'child') return child;
        return reply(id.sessionId + id.turnIndex);
    });
    await flush();
    assert.deepEqual(calls.map(c => c.slice(0, 2)), [['root', 0], ['child', 0], ['root', 1]]);
    assert.deepEqual(calls[1][2], [msg('child')]);
    release(reply('child-done'));
    const result = await pending;
    assert.deepEqual(calls.map(c => c.slice(0, 2)), [['root', 0], ['child', 0], ['root', 1], ['root', 2]]);
    assert.equal(get(result, 'root').outcome, 'success');
    assert.equal(calls[3][2].length, 5);
    assert(!calls[3][2].some(m => m.content === 'child-done'));
});
test('fork snapshots include response and siblings cannot mutate each other', async () => {
    const calls = [];
    const input = [session('p', [turn('p', { forks: ['a', 'b'] })]), session('a', [turn('a')]), session('b', [turn('b')])];
    const saved = JSON.stringify(input);
    const result = await dagConversations(input, async (request, id) => {
        calls.push([id.sessionId, structuredClone(request.messages)]);
        request.messages[0].content = 'mutated';
        return reply(id.sessionId);
    });
    assert.deepEqual(calls[1][1], [msg('p'), reply('p'), msg('a')]);
    assert.deepEqual(calls[2][1], [msg('p'), reply('p'), msg('b')]);
    assert.equal(JSON.stringify(input), saved);
    assert.equal(result.size, 3);
});
test('background fork continues parent but final drain waits child', async () => {
    const calls = [];
    let release, done = false;
    const child = new Promise(resolve => { release = resolve; });
    const pending = dagConversations([session('p', [turn('p', { forks: [{ child: 'c', background: true }] }), turn('next')]), session('c', [turn('c')])], async (_, id) => {
        calls.push(id.sessionId + id.turnIndex);
        return id.sessionId === 'c' ? child : reply('ok');
    }).then(result => { done = true; return result; });
    await flush();
    assert.deepEqual(calls, ['p0', 'c0', 'p1']);
    assert.equal(done, false);
    release(reply('done'));
    assert.equal(get(await pending, 'c').outcome, 'success');
});
test('default spawn blocks next turn; failed child releases join', async () => {
    const calls = [];
    let reject;
    const failed = new Promise((_, r) => { reject = r; });
    const pending = dagConversations([session('p', [turn('p', { spawns: ['c'] }), turn('after')]), session('c', [turn('c')])], async (_, id) => {
        calls.push(id.sessionId + id.turnIndex);
        return id.sessionId === 'c' ? failed : reply('ok');
    });
    await flush();
    assert.deepEqual(calls, ['p0', 'c0']);
    reject(new Error('child failure'));
    const result = await pending;
    assert.deepEqual(calls, ['p0', 'c0', 'p1']);
    assert.equal(get(result, 'c').outcome, 'failure');
    assert.equal(get(result, 'p').outcome, 'success');
});
test('terminal spawn and nested background descendants drain', async () => {
    let release, done = false;
    const leaf = new Promise(resolve => { release = resolve; });
    const calls = [];
    const pending = dagConversations([session('p', [turn('p', { spawns: ['c'] })]), session('c', [turn('c', { spawns: ['g'] })]), session('g', [turn('g')])], async (_, id) => {
        calls.push(id.sessionId);
        return id.sessionId === 'g' ? leaf : reply('ok');
    }).then(value => { done = true; return value; });
    await flush();
    assert.deepEqual(calls, ['p', 'c', 'g']);
    assert.equal(done, false);
    release(reply('done'));
    assert.equal((await pending).size, 3);
});
test('sibling branches both start before either settles', async () => {
    const releases = [], calls = [];
    const pending = dagConversations([session('p', [turn('p', { forks: ['a', 'b'] })]), session('a', [turn('a')]), session('b', [turn('b')])], async (_, id) => {
        calls.push(id.sessionId);
        if (id.sessionId === 'p') return reply('ok');
        return new Promise(resolve => releases.push(resolve));
    });
    await flush();
    assert.deepEqual(calls, ['p', 'a', 'b']);
    assert.equal(releases.length, 2);
    releases.forEach(resolve => resolve(reply('done')));
    assert.equal((await pending).size, 3);
});
test('failed root skips unstarted descendants and preserves unrelated roots', async () => {
    const result = await dagConversations([session('p', [turn('p', { forks: ['c'] })]), session('c', [turn('c')]), session('other', [turn('other')])], async (_, id) => {
        if (id.sessionId === 'p') throw new Error('root failure');
        return reply('ok');
    });
    assert.equal(get(result, 'p').outcome, 'failure');
    assert.equal(get(result, 'c').outcome, 'skipped');
    assert.equal(get(result, 'other').outcome, 'success');
});
test('invalid graphs and unsupported policies refuse before dispatch', async () => {
    const invalid = [
        [session('a', [turn('a', { forks: ['b'] })]), session('b', [turn('b', { spawns: ['a'] })])],
        [session('a', [turn('a', { spawns: ['missing'] })])],
        [session('a', [turn('a', { forks: ['b'] }), turn('late')]), session('b', [turn('b')])],
        [session('a', [turn('a', { spawns: [{ children: ['b'], join_at: 0 }] }), turn('late')]), session('b', [turn('b')])],
        [session('a', [turn('a', { spawns: [{ children: ['b'], join_at: 2 }] }), turn('late')]), session('b', [turn('b')])],
        [{ ...session('a', [turn('a')]), context_mode: 'message_array_with_responses' }],
        [session('a', [turn('a', { tools: [] })])],
        [session('a', [turn('a', { max_tokens: true })])],
        [session('a', [{ messages: [{ role: 'user', content: [] }] }])],
        [session('a', [turn('a')]), session('a', [turn('a')])],
        Array.from({ length: 257 }, (_, i) => session(String(i), [turn('a')])),
        [session('a', Array.from({ length: 1001 }, () => turn('a')))],
        [session('a', [turn('x'.repeat(262145))])],
        Array.from({ length: 5 }, (_, i) => session(String(i), Array.from({ length: 1000 }, () => turn('a')))),
        [session('', [turn('a')])],
        [session('a', [])],
    ];
    for (const graph of invalid) {
        let calls = 0;
        await assert.rejects(dagConversations(graph, async () => { calls++; return reply('ok'); }));
        assert.equal(calls, 0);
    }
});
test('context budget and malformed responses become retained failures', async () => {
    for (const response of [null, new Error('returned error'), { role: 'user', content: 'wrong' }, reply('x'.repeat(262145))]) {
        const result = await dagConversations([session('a', [turn('a')])], async () => response);
        assert.equal(get(result, 'a').outcome, 'failure');
    }
    const result = await dagConversations([session('a', Array.from({ length: 4 }, () => turn('x'.repeat(100000))))], async () => reply('x'.repeat(100000)));
    assert.equal(get(result, 'a').outcome, 'failure');
    assert.match(get(result, 'a').error, /context budget/);
    assert.equal(get(result, 'a').turns.at(-1).outcome, 'failure');
    const countLimit = await dagConversations([session('a', Array.from({ length: 501 }, () => turn('a')))], async () => reply('ok'));
    assert.equal(get(countLimit, 'a').outcome, 'failure');
    assert.match(get(countLimit, 'a').error, /context budget/);
});
test('root and spawn turn-zero systems are valid; forks inherit system', async () => {
    const system = { role: 'system', content: 'root system' };
    const spawnSystem = { role: 'system', content: 'spawn system' };
    const requests = new Map();
    const result = await dagConversations([
        session('p', [{ messages: [system, msg('root')], forks: ['fork'], spawns: ['spawn'] }]),
        session('fork', [turn('fork')]),
        session('spawn', [{ messages: [spawnSystem, msg('spawn')] }]),
    ], async (request, id) => { requests.set(id.sessionId, request); return reply('ok'); });
    assert([...result.values()].every(r => r.outcome === 'success'));
    assert.deepEqual(requests.get('fork').messages, [system, msg('root'), reply('ok'), msg('fork')]);
    assert.deepEqual(requests.get('spawn').messages, [spawnSystem, msg('spawn')]);
});
test('later and fork-child system messages reject before dispatch', async () => {
    const systemTurn = { messages: [{ role: 'system', content: 'late system' }, msg('turn')] };
    const invalid = [
        [session('root', [turn('first'), systemTurn])],
        [session('root', [turn('root', { forks: ['child'] })]), session('child', [systemTurn])],
        [session('root', [turn('root', { spawns: ['child'] })]), session('child', [turn('first'), systemTurn])],
        [session('root', [turn('root', { forks: [{ child: 'child', background: true }] }), turn('next')]), session('child', [systemTurn])],
    ];
    for (const graph of invalid) {
        let calls = 0;
        await assert.rejects(dagConversations(graph, async () => { calls++; return reply('ok'); }), /system message/);
        assert.equal(calls, 0);
    }
});
test('synchronous callbacks are refused', async () => {
    const result = await dagConversations([session('a', [turn('a')])], () => reply('ok'));
    assert.equal(get(result, 'a').outcome, 'failure');
    assert.match(get(result, 'a').error, /Promise/);
});
test('repeated and multiparent spawns allocate fresh identities and histories', async () => {
    const calls = [];
    const results = await dagConversations([
        session('a', [turn('a', { spawns: ['c'] }), turn('again', { spawns: ['c'] })]),
        session('b', [turn('b', { spawns: ['c'] })]), session('c', [turn('child')]),
    ], async (request, identity) => { calls.push({ request, identity }); return reply('ok'); });
    const children = [...results.values()].filter(r => r.sessionId === 'c');
    assert.equal(children.length, 3);
    assert.equal(new Set(children.map(r => r.instanceId)).size, 3);
    assert(children.every(r => r.parentInstanceId !== null));
    assert(calls.filter(c => c.identity.sessionId === 'c').every(c => c.request.messages.length === 1));
    assert.equal(results.size, 5);
});
test('pre-session spawn starts before parent zero and does not gate it', async () => {
    let release;
    const blocked = new Promise(resolve => { release = resolve; });
    const calls = [];
    const pending = dagConversations([{ ...session('p', [turn('parent')]), pre_session_spawns: ['c'] }, session('c', [turn('child')])], async (request, identity) => {
        calls.push(identity);
        return identity.sessionId === 'c' ? blocked : reply('ok');
    });
    await flush();
    assert.deepEqual(calls.map(c => c.sessionId), ['c', 'p']);
    assert.equal(calls[0].parentInstanceId, null);
    release(reply('done'));
    assert([...((await pending).values())].every(r => r.outcome === 'success'));
});
test('integer requestless rounds refire fresh children and wait between rounds', async () => {
    const releases = [], calls = [], delays = [];
    globalThis.dagTimer = (fn, ms) => { delays.push(ms); queueMicrotask(fn); };
    const pending = dagConversations([{ session_id: 'o', orchestrator: true, spawns: ['c'], rounds: 2, think_time_ms: 7 }, session('c', [turn('child', { delay: 3 })])], async (request, identity) => {
        calls.push({ request, identity });
        return new Promise(resolve => releases.push(resolve));
    });
    await flush();
    assert.equal(calls.length, 1);
    assert.equal(calls[0].identity.sessionId, 'c');
    releases[0](reply('first'));
    await flush();
    assert.equal(calls.length, 2);
    assert.notEqual(calls[0].identity.instanceId, calls[1].identity.instanceId);
    assert.deepEqual(calls.map(c => c.request.messages.length), [1, 1]);
    releases[1](reply('second'));
    const results = await pending;
    assert.equal(get(results, 'o').turns.length, 3);
    assert(get(results, 'o').turns.every(t => t.noRequest));
    assert.deepEqual(delays, [7, 3, 7, 3]);
});
test('authored rounds honor per-round delays and terminal gate has none', async () => {
    const delays = [], calls = [];
    globalThis.dagTimer = (fn, ms) => { delays.push(ms); queueMicrotask(fn); };
    const result = await dagConversations([{ session_id: 'o', orchestrator: true, rounds: [{ spawns: ['a'] }, { spawns: ['b'], think_time_ms: 9 }], think_time_ms: 4 }, session('a', [turn('a')]), session('b', [turn('b')])], async (_, identity) => { calls.push(identity.sessionId); return reply('ok'); });
    assert.deepEqual(calls, ['a', 'b']);
    assert.deepEqual(delays, [4, 9]);
    assert.deepEqual(get(result, 'o').turns.map(t => t.delayMs), [4, 9, 0]);
});
test('nonzero delay waits asynchronously before dispatch', async () => {
    const waits = [], calls = [];
    globalThis.dagTimer = (fn, ms) => waits.push({ fn, ms });
    const pending = dagConversations([session('p', [turn('later', { delay: 12 })])], async (_, identity) => { calls.push(identity); return reply('ok'); });
    await flush();
    assert.equal(calls.length, 0);
    assert.equal(waits[0].ms, 12);
    waits[0].fn();
    assert.equal(get(await pending, 'p').outcome, 'success');
    globalThis.dagTimer = (fn, ms) => queueMicrotask(fn);
});
test('invalid expansion and advanced fields reject before all callbacks', async () => {
    const invalid = [
        [{ session_id: 'o', orchestrator: true, spawns: ['c'], rounds: 1025 }, session('c', [turn('c')])],
        [{ session_id: 'o', orchestrator: true, rounds: [{ spawns: ['c'] }, { spawns: ['c'] }] }, session('c', [turn('c')])],
        [{ session_id: 'o', orchestrator: true, spawns: ['c'], pre_session_spawns: ['c'] }, session('c', [turn('c')])],
        [session('p', [turn('p', { forks: ['c'] })]), { ...session('a', [turn('a')]), pre_session_spawns: ['c'] }, session('c', [turn('c')])],
        [session('p', [turn('p', { forks: ['c'] })]), session('a', [turn('a', { forks: ['c'] })]), session('c', [turn('c')])],
        [session('p', [turn('p', { spawns: ['c', 'c'] })]), session('c', [turn('c')])],
        [session('p', [turn('p', { delay: 60001 })])],
        [session('p', [turn('p', { delay: true })])],
        [{ session_id: 'o', orchestrator: true, spawns: ['c'], rounds: 2, think_time_sigma: 1 }, session('c', [turn('c')])],
        [{ session_id: 'a', orchestrator: true, spawns: ['b'], rounds: 40 }, { session_id: 'b', orchestrator: true, spawns: ['c'], rounds: 40 }, session('c', [turn('c')])],
    ];
    for (const graph of invalid) {
        let calls = 0;
        await assert.rejects(dagConversations(graph, async () => { calls++; return reply('ok'); }));
        assert.equal(calls, 0);
    }
});
for (const [name, run] of tests) { await run(); console.log('PASS ' + name); }
console.log(`${tests.length} scheduling controls passed`);
