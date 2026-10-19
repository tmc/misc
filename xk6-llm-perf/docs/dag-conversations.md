# Async conversation branches

`dagConversations` in `examples/lib/workload.js` runs a bounded subset of the
AIPerf `dag_jsonl` conversation format. The existing `dag` helper remains a
synchronous dependency graph. It does not provide concurrent branches.

Pass parsed sessions and an async callback. The callback receives an isolated
request object and `{sessionId, turnIndex, parentId}`, and returns a Promise
for `{role: 'assistant', content: '...'}`. It owns transport timeouts, response
validation and any credential redaction. Use a scenario `maxDuration` to bound
the complete run, including background drain. A callback that never settles
can otherwise keep a join pending.

```js
import http from 'k6/http';
import { dagConversations } from './lib/workload.js';

const sessions = [
  {session_id: 'parent', turns: [
    {messages: [{role: 'user', content: 'start'}], spawns: ['child']},
    {messages: [{role: 'user', content: 'after child'}]},
  ]},
  {session_id: 'child', turns: [
    {messages: [{role: 'user', content: 'fresh context'}]},
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
  for (const [id, result] of results) {
    if (result.outcome !== 'success') throw new Error(id + ': ' + result.outcome);
  }
}
```

The four-request [runnable example](../examples/workloads/dag-conversations.js)
retains attempts and session outcomes and gates its checks. Set
`DAG_ENDPOINT_URL` to an owned endpoint and run it with a compatible k6 binary.
The callback must use a genuinely asynchronous transport for overlap. Returning
`Promise.resolve(syncRequest())` still blocks other branches during the
synchronous request. A non-Promise callback is recorded as a session failure.

The extension's `Client` remains synchronous. This example uses stock
`k6/http.asyncRequest`; its receipts do not carry extension measurement,
streaming, token-accounting or distributed merge provenance. The helper runs
within one VU. It does not implement AIPerf's worker credit router or cross-VU
sticky routing, concurrency admission or stop-condition accounting.

## Supported behavior

Each session appends its authored messages and the live assistant response to
its history. A fork copies that history after the spawning response; mutations
in a callback or sibling cannot alter another branch. A spawn starts with fresh
history. Child responses do not merge into the parent history.

Bare forks must occur on the parent's terminal turn. A fork object such as
`{child: 'child', background: true}` allows subsequent parent turns without a
join. Bare spawns join before the next parent turn. A spawn object such as
`{children: ['a', 'b'], join_at: 3}` allows intervening parent turns to overlap
children, and joins both children before turn 3. Indexes start at zero.
An implicit spawn on the terminal turn has no parent join. Explicit `join_at`
must be after the spawning turn and inside the parent's turns.

A parent join waits for each child's own terminal turn. The function's final
drain also waits for background descendants. Errors stop further turns and
branch creation in that session; already-started children continue. A failed
child releases its parent's join, matching the pinned upstream default error
policy. The result Map preserves successes and failures; unstarted sessions
are `skipped`. It never converts a failed child into success. Upstream's optional
fail-fast policy, which cancels the parent and siblings, is unsupported.

## Limits and unsupported features

The complete asset is validated before any callback: at most 256 sessions,
1000 turns per session, 4096 turns total and a 16 MiB UTF-8 upper-bound asset
budget. Every session may appear as a child only once, including spawn targets.
Unknown or reused targets and cycles are rejected. This stricter forest rule
does not implement upstream's repeated spawn templates. Each context is limited
to 1000 messages and a 1 MiB UTF-8 upper bound. Individual text content is
limited to 262144 JavaScript code units. UTF-8 upper bounds use three bytes per
code unit and include serialized message structure.

Only session fields `session_id` and `turns` are accepted. Turns support text
`messages`, optional `model`, `max_tokens`, `forks`, `spawns`, and absent or zero
`delay`. `max_tokens` must be an integer in 1..1000000. Messages contain only
`role` and string `content`, with system, user or assistant roles. Other fields,
multimodal/tool messages, tools/extra parameters, nonzero delays,
pre-session spawns, orchestrators/rounds, alternate context policies, trace
snapshot replay, cancellation propagation and dynamic routing are unsupported
source features. These remain implementation gaps, independent of hardware.

System messages are accepted on turn zero of roots and fresh-context spawn
children. They are rejected on later turns and on all fork-child turns, which
inherit the parent's context. This placement validation completes before any
callback, following the pinned loader's append-context policy.

The comparison target is AIPerf
`f10eb50a1776775d80d7f13396b0f668f14e2c03`:
`dataset/loader/dag_jsonl_models.py`, `dataset/loader/dag_jsonl.py`,
`dataset/loader/_dag_jsonl_helpers.py`, and `timing/branch_orchestrator.py`.
This subset does not establish whole-format or live-model parity.

Run CPU scheduling controls with `node scripts/test_dag_conversations.mjs`.
Those controls load the shipped helper with only k6 imports stubbed and verify
deferred branch overlap, joins, fork isolation, failure drain, cycles and
bounds. They do not replace actual k6 event-loop and transport qualification.
