# macos(4) source manifest

This manifest records the source corpus used for the Plan 9/Wanix Apple
capability API specs in [macos(4).md](macos(4).md). The repo state was dirty
before this doc pass; implementation files were inspected but not edited.

## API identity

- Canonical API name: Wanix macOS Apple capability filesystem.
- Canonical service root: `macos`.
- Compatibility root: `mnt/macos`; compatibility only, not the source of new
  API names.
- Host device: `#macos`.
- Transport boundary: generic filesystem calls across WebKit; Apple service
  semantics live in Go.

## Sources

| Source | Role | Use | Caveat |
| --- | --- | --- | --- |
| `README.md` | User-facing contract | Current mounted namespace, canonical `macos` path, compatibility bind, run/self-test commands | Short overview, not a complete API reference |
| `DESIGN.md` | Prior design note | AppKit service shape, generic bridge direction, AppKit boundary | AppKit-focused and superseded by the broader spec |
| `APPLE-FS-APIS.md` | Prior broad design pass | Service taxonomy, clone/ctl/status/data/event conventions, service boundaries | Planning note with research-only and do-not-expose surfaces mixed in |
| `internal/applefs/fs.go` | Current filesystem framework | `Root`, service mount table, aliases, clone allocation, read/write/append semantics, errors | Dirty before this pass; source inspected only |
| `internal/applefs/services.go` | Current synthetic tree | Built-in service names, file names, initial status text, ctl verbs, read-only blocked services | Stub semantics are not always final native behavior |
| `internal/applefs/fs_test.go` | Contract tests | Mounted services, aliases, clone schemas, unknown-ctl fail-closed behavior, blocked-service read-only behavior | Tests verify local framework behavior, not every native TCC path |
| `internal/webkithost/script.go` | Current WebKit bridge | Injected `native.fs` bridge, cache, clone-schema hydration, `macos` and `mnt/macos` binding | Current JS cache is not the desired long-term transport API |
| `internal/webkithost/native.go` | Native dispatch | Host write dispatch, AppKit, notify, touchid/auth, native filesystem message methods | Contains legacy `native.call`; specs keep filesystem API canonical |
| `internal/webkithost/native_*.go` | Native service handlers | Vision, speech, mic, screen, AX, keychain, reachability, image/document, VZ semantics and result formats | Some live operations require macOS TCC prompts or entitlements |
| `internal/webkithost/*_test.go` | Native unit tests | Parser/formatter/lifecycle expectations for sensitive services | Does not replace live OS permission testing |

## Lifecycle facts preserved

- Namespace is capability-shaped: each Apple authority boundary is a sibling
  service under `macos`.
- Per-call or stateful work uses `clone` to allocate a numbered session
  directory.
- `ctl` files accept newline-terminated verbs and reject unknown verbs.
- `status`, `result`, and `meta` files report tagged text unless structured
  JSON is needed.
- `data`, `frames`, `console`, and `event` are stream or payload files; events
  are control-plane records, not binary payloads.
- Permission failures are explicit. Unknown paths fail not-found. Unsupported
  services are visible as read-only policy directories, not silently absent.
- The WebKit bridge is generic transport. It must not encode AppKit, Vision,
  VZ, or other Apple service behavior.
