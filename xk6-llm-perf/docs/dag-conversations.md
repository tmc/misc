# Async conversation branches

`dagConversations` in `examples/lib/workload.js` runs bounded text-only AIPerf
`dag_jsonl` conversation templates. The original `dag` helper remains a
synchronous dependency graph and does not provide concurrent branches.

Pass parsed sessions and an async callback. The callback receives an isolated
request object and `{sessionId, instanceId, turnIndex, parentId,
parentInstanceId}`, and returns a Promise for
`{role: 'assistant', content: '...'}`. It owns transport timeouts, response
validation and credential redaction. Use a scenario `maxDuration` to bound the
complete run, including background drain. An unsettled callback can otherwise
keep a join pending.

Results are a Map keyed by `JSON.stringify([sessionId, ordinal])`, where the
zero-based ordinal distinguishes repeated invocations of a template. Each
receipt includes `sessionId`, `instanceId`, `parent`, `parentInstanceId`, turn
outcomes and delay values. This intentionally changes the earlier helper's
session-ID result keys: repeated spawns cannot share one result. Authored IDs
cannot collide with generated instance identities. A pre-session child has a
null `parentInstanceId`, matching its fresh independent correlation, while
`parent` retains the template that declared it.

```js
import http from 'k6/http';
import { dagConversations } from './lib/workload.js';

const sessions = [
  {session_id: 'coordinator', orchestrator: true, spawns: ['worker'], rounds: 2,
   think_time_ms: 5},
  {session_id: 'worker', turns: [
    {messages: [{role: 'user', content: 'fresh request'}], delay: 2},
  ]},
];
export const options = {scenarios: {
  branches: {executor: 'shared-iterations', vus: 1, iterations: 1, maxDuration: '20s'},
}};
export default async function () {
  const results = await dagConversations(sessions, async request => {
    const response = await http.asyncRequest('POST', __ENV.ENDPOINT_URL,
      JSON.stringify({...request, model: 'fixture', stream: false}),
      {headers: {'Content-Type': 'application/json'}, timeout: '5s'});
    if (response.status !== 200) throw new Error('request failed');
    return response.json().choices[0].message;
  });
  for (const [instance, result] of results) {
    if (result.outcome !== 'success') throw new Error(instance + ': ' + result.outcome);
  }
}
```

The four-request [runnable example](../examples/workloads/dag-conversations.js)
uses two requestless coordinator rounds and a pre-session child per round.
It retains every request attempt and all five instance outcomes, with checks
thresholds. Set `DAG_ENDPOINT_URL` to an owned endpoint and run it with a
compatible k6 binary. The helper imports official `k6/timers` for asynchronous
millisecond delays. Returning `Promise.resolve(syncRequest())` still blocks
other branches while the synchronous request executes. A non-Promise callback
is recorded as a session failure.

The extension's `Client` remains synchronous. The example uses stock
`k6/http.asyncRequest`; these receipts do not carry extension measurement,
streaming, token-accounting or distributed merge provenance. The helper runs
within one VU and does not implement the worker credit router, cross-VU sticky
routing, global concurrency admission or phase slot accounting.

## Context and joins

Each session appends its authored messages and live assistant responses to its
history. Forks copy that history after the spawning response; siblings and
callback mutations cannot alter another instance. Spawns start fresh. A SPAWN
template may be used by multiple parents or on multiple turns, creating a new
instance each time. Child responses never merge into the parent history.

Bare forks must occur on the parent's terminal turn. A fork object such as
`{child: 'child', background: true}` allows the parent to continue without a
join. A FORK target may have only one declaration in the template graph, even
when its parent template itself is invoked repeatedly.

Bare spawns join before the next parent turn. A spawn object such as
`{children: ['a', 'b'], join_at: 3}` permits intervening parent turns to overlap
children and joins both before turn 3. Indexes start at zero. An implicit spawn
on the terminal turn has no parent join. Explicit `join_at` must be after the
spawning turn and inside the parent's turns. A target cannot appear twice in
the same turn's branches, including mixed fork/spawn declarations.

`pre_session_spawns: ['child']` schedules fresh background children before the
parent's turn zero, including before its delay. It does not gate that turn.
Pre-session lists reject duplicates and may not target any FORK child.

A join waits for each child's own terminal turn; final drain also waits for
background descendants. Errors stop further turns and branch creation in the
failed instance; already-started children continue. A failed child releases its
parent's join, matching the pinned default error policy. Every pre-expanded
instance retains success, failure or `skipped`; failed children are never
converted into successes. Upstream's optional fail-fast cancellation of parent
and siblings remains unsupported.

## Requestless rounds and delays

An orchestrator declares `orchestrator: true`, no authored turns, and either
`spawns` or authored `rounds`. Without `rounds`, its children are dispatched
once in the current invocation, with no parent request. Integer `rounds: N`
refires the same SPAWN templates N times, waiting for each round's children
before proceeding. Its N spawning turns and terminal gate are requestless.

List rounds, for example `rounds: [{spawns: ['a']}, {spawns: ['b'],
think_time_ms: 10}]`, declare different children per round. List-round child IDs
must be distinct across all rounds; conversation-level `spawns` must be omitted
or empty. Orchestrators cannot declare pre-session spawns.

Fixed `think_time_ms` supplies the wait before each round; per-round values
override it. The terminal gate has no think-time. Normal turn `delay` and fixed
round waits are milliseconds, implemented with asynchronous timers after any
join and before dispatch. Zero waits create no timer. Nonzero waits must be
finite numbers in 0..60000; booleans are refused. The pinned loader accepts a
boolean normal-turn delay by coercion, while this helper deliberately refuses
it. No stochastic sigma/clamp settings are silently accepted.

## Bounds and remaining source gaps

The complete asset and instance expansion are checked before any callback:
256 templates, 1000 normalized turns per template, 4096 normalized turns total,
1024 expanded instances, 16384 realized turns and 300000 ms of summed realized
delays. Integer and list rounds are limited to 999, allowing their terminal
gate within the per-template turn budget. Asset size uses a 16 MiB UTF-8 upper
bound. Contexts allow 1000 messages and a 1 MiB UTF-8 upper bound; individual
text content allows 262144 JavaScript code units. UTF-8 upper bounds use three
bytes per code unit and include serialized structure. These are explicit local
bounds; larger valid upstream workloads are refused rather than partly run.

Normal turns support text `messages`, optional `model`, `max_tokens`, `forks`,
`spawns` and `delay`. `max_tokens` is an integer in 1..1000000. Messages have
only `role` and string `content`, with system/user/assistant roles. System
messages are accepted only on turn zero of roots and fresh SPAWN children;
FORK children inherit them and cannot redeclare them. Unknown fields are
rejected. Tools/multimodal/extra parameters, alternate context policies,
stochastic round waits, trace snapshots, cancellation propagation and dynamic
routing remain unsupported source features, independent of hardware.

The comparison target is AIPerf
`f10eb50a1776775d80d7f13396b0f668f14e2c03`:
`dataset/loader/dag_jsonl_models.py`, `dataset/loader/dag_jsonl.py`,
`dataset/loader/_dag_jsonl_helpers.py`, and `timing/branch_orchestrator.py`.
This bounded implementation does not establish whole-format or live-model
parity. Run `node scripts/test_dag_conversations.mjs` for deterministic deferred
Promise/timer controls against the shipped source with k6 imports stubbed.
Actual k6 transport qualification remains separate from these CPU controls.
