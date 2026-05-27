# appkit(4) filesystem shape

This note records the Plan 9 / Wanix filesystem API target for the native
AppKit host bridge. The current implementation binds the Apple capability
device at `macos` and keeps `mnt/macos` only as a compatibility path. The
durable service name for the host-app UI surface should be `appkit`; broader
Apple framework surfaces belong in separate capabilities. This follows the
`plan9-fs-api-design` skill output captured at:

`~/.plan9-fs-designs/wanix-macos-desktop/probes/00-mission-response.md`

## Implementation Model

The implementation should be inspired by `tractordev/toolkit-go` concepts,
not matched to its API. The useful ideas are small mounted filesystems,
read-only wrappers, explicit mutable operations, and event streams. The Wanix
host should keep a local, Plan 9-shaped API.

The host should expose AppKit as one service in a generic service registry:

```go
type Service interface {
	Name() string
	Open(name string) (File, error)
}
```

`macos` is a mount table. During the compatibility window the bootstrap also
mounts the same device at `mnt/macos`. The device should mount `appkit` and
also keep legacy aliases such as `window/title`. New Apple frameworks should
be added as sibling services, not as more cases in the AppKit implementation.

The durable boundary is the Apple capability filesystem, not the macOS app.
`applefs.Root` owns path lookup, clone allocation, file state, and policy
surface shape. A native host implements the small side-effect callback used
after successful filesystem writes. The current WebKit app is therefore an
adapter: it transports generic file operations and supplies the macOS
implementation of those side effects.

AppKit files should use reusable file adapters:

| Shape | AppKit use |
| --- | --- |
| `TextFile` | `window/title`, `pasteboard/text`, alert fields |
| `StatusFile` | `status`, `app/status`, `window/status`, `alert/$id/result` |
| `CtlFile` | `app/ctl`, `window/ctl`, `alert/$id/ctl` |
| `CloneFile` | `alert/clone`, future `picker/clone` |

The WebKit JavaScript bridge should only forward generic filesystem operations
to Go. It should not contain AppKit-specific routing. Go owns path routing,
session allocation, state, and all native calls. AppKit calls must still be
dispatched onto the AppKit main thread.

## Tree

```text
macos/appkit
|-- status
|-- app/
|   `-- ctl
|-- pasteboard/
|   `-- text
|-- window/
|   |-- ctl
|   `-- title
`-- alert/
    |-- clone
    `-- $id/
        |-- ctl
        |-- title
        |-- message
        |-- style
        |-- buttons
        `-- result
```

`app`, `pasteboard`, and `window` are singletons because the host owns one
`NSApplication`, one general pasteboard, and one `NSWindow`. If the host grows
multi-window support, `window` should become a clone-allocated namespace.

`alert` is clone-allocated because alerts are per-call state. A flat
`alert/title`, `alert/message`, `alert/show`, `alert/result` namespace would
let concurrent scripts overwrite each other's pending modal state.

## Semantics

`status` returns tagged text:

```text
api appkit
status ok
```

`app/ctl` accepts:

```text
activate
quit
```

`window/ctl` accepts:

```text
center
fullscreen
toggle-fullscreen
minimize
miniaturize
close
```

`window/title` is read/write text. `pasteboard/text` writes host clipboard
text; reads return the last value written through this namespace.

`alert/clone` returns a decimal id and creates `alert/$id/`. Configure an
alert by writing `title`, `message`, `style`, and `buttons`. Write `show` to
`alert/$id/ctl`. After the user selects a button, `alert/$id/result` contains
tagged text:

```text
button OK
index 0
response 1000
```

## Boundary

| Supported | Fails closed |
| --- | --- |
| Application activate and quit | Multi-application control |
| Single host window title and basic window verbs | Multiple host windows |
| Text pasteboard writes | Rich pasteboard types, file promises, and observing external pasteboard changes |
| Native modal alerts with text fields and buttons | Sheets, accessory views, async non-modal alerts |
| Tagged text status and alert results | Live AppKit object handles |

Unsupported paths and unknown ctl verbs return errors instead of falling back
silently.
