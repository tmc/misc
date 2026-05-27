# Apple filesystem API candidates

This note summarizes a `plan9-fs-api-design` pass over selected
`github.com/tmc/apple` examples. The notebook artifacts are under:

`~/.plan9-fs-designs/apple-framework-fs-apis/`

The design rule is to split by lifecycle and authority boundary, not by Apple
framework package name. A service should be its own capability when it has its
own TCC prompt, entitlement, long-lived stream, or host mutation boundary.

This design is inspired by `tractordev/toolkit-go`, but it should not copy that
API. The useful ideas are composition through mounted filesystems, wrappers for
read-only behavior, explicit mutable interfaces, watch/event streams, and small
filesystem adapters. `wanix-macapp` should keep its own internal API shaped by
Wanix and Plan 9 conventions.

## Taxonomy

| Service | Status | Authority boundary | Lifecycle pattern | Backing examples |
| --- | --- | --- | --- | --- |
| `appkit(4)` | implement now | Current app UI | singleton app/window/pasteboard; clone modal calls | `examples/appkit/{hello-window,alert-dialog,file-picker,menubar-clock,pbd}` |
| `notify(4)` | implement now | User notification permission | service auth plus clone notification requests | `examples/usernotifications/usernotifications-local-demo` |
| `touchid(4)` | implement now | LocalAuthentication UI | clone evaluation requests | `examples/localauthentication/localauthentication-evaluate-demo` |
| `vision(4)` | implemented | Local image analysis over caller bytes | clone request/response jobs | `examples/vision/ocrjson` |
| `image(4)` | implemented | Local image transforms over caller bytes | clone request/response jobs | `examples/coreimage/imgfilter` |
| `document(4)` | implemented | Local document transforms over caller bytes | clone request/response jobs | `examples/pdfkit/pdfextract` |
| `speech(4)` | implemented | Speaker/audio-output authority | clone utterance jobs | `examples/avfaudio/audiosay` |
| `mic(4)` | implemented bounded | Microphone TCC and audio devices | clone one-shot capture jobs | `examples/avfaudio/{inputtap,speechbuffers,audiosay}` |
| `screen(4)` | implemented bounded | Screen Recording TCC | target registry plus clone one-shot captures | `examples/screencapturekit/{windowlist,screenshot,streamframes}` |
| `ax(4)` | implemented | Accessibility trust; control of other apps | target app sessions plus element sessions | `applicationservices`, `x/axuiautomation` |
| `keychain(4)` | implemented | Security/keychain access | clone read-only queries | `examples/security/{keychain-query-demo,trust-evaluate-demo,auth-dialog}` |
| `reachability(4)` | implemented | Host network state observation | clone target snapshots | `examples/network`, Network.framework |
| `vz(4)` | implemented efi | Virtualization entitlement, VM resources | proc-style clone VM dirs | `examples/virtualization/{vminfo,plan9front-vz}` |
| `xpc(4)` | research only | launchd/bootstrap namespace | listener endpoints and call/return pairs | `examples/foundation/xpc-listener-demo` |
| `webkit(4)` | research only | Embedded page/runtime control | page/session dirs | `examples/webkit/{wanix-host,webkit-url-scheme-demo}` |
| `fskit(4)` | do not expose yet | System Settings enabled filesystem extension | installed provider lifecycle | `examples/fskit/{tinyfsgo,9pfsgo}` |
| `cloudkit(4)` | do not expose yet | iCloud entitlement, signed app identity, account state | app-specific sync graph | `examples/cloudkit/{cloudkit-status,cloudkit-syncdirs}` |
| `netext(4)` | do not expose yet | NetworkExtension/system extension policy | provider lifecycle and traffic callbacks | `examples/networkextension/*` |
| `endpointsecurity(4)` | do not expose yet | EndpointSecurity entitlement, host-wide event authority | event stream plus authorization replies | `examples/endpointsecurity/*` |
| `sm(4)` | do not expose yet | durable launchd/login-item mutation | register/unregister operations | `examples/servicemanagement/helper-bundle-smoke` |

## Toolkit-Inspired Model

The implementation should be a small host filesystem framework, not one large
JavaScript switch. The framework should borrow the following concepts from
`toolkit-go` without importing its public surface as the design contract.

### Mount composition

Each Apple capability is an independently registered service mounted under a
stable root:

```go
type Service interface {
	Name() string
	Open(name string) (File, error)
}
```

The root namespace is a mount table. `appkit` owns `appkit/*`; `notify` owns
`notify/*`; `touchid` owns `touchid/*`. Mount lookup is generic. Native Apple
code does not inspect global paths and global routing code does not know
service internals.

This is the useful part of `mountfs` and `mountablefs`: compose small
filesystems by path. Wanix does not need their exact interfaces.

The long-term API boundary is `applefs.Root`, not the `.app` process. The
native app, a future helper, or a future Wanix-native transport should all
connect by implementing the filesystem host callback that runs after a
successful write. This keeps service naming, clone schemas, read-only policy,
and file state independent of the packaging used to obtain macOS authority.

### File adapters

Most files fall into a few reusable shapes:

| Shape | Use | Behavior |
| --- | --- | --- |
| `TextFile` | `title`, `body`, `reason`, `target` | read/write UTF-8 state |
| `StatusFile` | `status`, `result`, `meta` | read-only tagged text |
| `CtlFile` | `ctl` | write verbs, return errors for unknown commands |
| `CloneFile` | `clone` | opening the file allocates `$id`; reading the fd returns the id as text |
| `StreamFile` | `data`, `frame`, `console`, `event` | blocking byte stream |
| `ReadOnly` | research or unsafe surfaces | rejects all mutation |

These correspond to toolkit ideas such as mutable filesystem methods,
`readonlyfs`, `watchfs`, and stream-capable files, but the Wanix API should use
plain Plan 9 file semantics: read, write, open, close, stat, and readdir.

### Clone sessions

Any operation with per-call state uses the same clone/session helper:

```text
service/
|-- clone
`-- $id/
    |-- ctl
    |-- status
    `-- ...
```

Opening `clone` allocates a directory owned by the service; reading that open
file descriptor returns the decimal id. The session object holds configuration
files, result files, stream endpoints, and cleanup state. This pattern covers
alerts, file pickers, notifications, Touch ID evaluations, Vision jobs,
image/document transforms, mic capture sessions, screen captures,
Accessibility app/element sessions, keychain queries, reachability monitors,
and VMs.

### Event and stream files

`event` is the common asynchronous surface. It is a blocking read stream of
tagged records or JSON lines. Services that need binary output use a separate
binary file such as `data`, `frame`, or `console`. Events are for control-plane
state; binary streams are for payloads.

Each event file must document its record format before implementation. Prefer
newline-delimited tagged text for small fixed records and newline-delimited JSON
when fields are sparse or nested. Do not mix formats within one file.

This is the useful part of `watchfs`: readers observe change without polling.
The first implementation can be single-reader and in-memory; the API should not
preclude fanout later.

### Read-only and research services

Research-only and not-yet-exposed services should still be mountable as
read-only directories with `status` and `README` files. That makes policy
visible to guests without granting authority. Unknown writes must fail with a
permission error. Unknown paths must fail with a not-found error.

### Bridge boundary

The WebKit bridge should expose one generic filesystem transport:

```text
open path mode
read fid count offset
write fid bytes offset
clunk fid
stat path
readdir path
```

JavaScript should not encode Apple service logic. It should translate Wanix
filesystem calls to the host transport. Go should own the service registry,
path routing, session state, and native Apple calls. AppKit/WebKit main-thread
rules are enforced by the host callback implementation, not by the bridge
transport.

## Implementation Plan

1. Add an internal `applefs` package with `Root`, `Service`, file adapters, and
   clone/session helpers.
2. Port the existing AppKit behavior to `applefs`, bind the canonical Wanix
   path at `macos`, and keep legacy paths mounted as aliases.
3. Add read-only stubs for all research and do-not-expose services so the
   namespace is honest before sensitive APIs are implemented.
4. Add `notify` and `touchid` as the first non-AppKit services because they are
   request/response capabilities with clear TCC/auth boundaries.
5. Add stream-capable and high-authority services only after event and binary
   stream files have tests: `vision`, `image`, `document`, `speech`, `mic`,
   `screen`, `ax`, `reachability`, `keychain`, and `vz`.

## Implement Now

### `appkit(4)` — host app UI

`appkit` is a separate capability because it controls only the current host
application shell: `NSApplication`, one `NSWindow`, text pasteboard, and modal
AppKit panels. It must not absorb notification, biometric, keychain, capture,
or VM authority.

```text
macos/appkit
|-- status
|-- app/
|   |-- ctl
|   `-- status
|-- window/
|   |-- ctl
|   |-- title
|   `-- status
|-- pasteboard/
|   |-- ctl
|   `-- text
|-- alert/
|   |-- clone
|   `-- $id/{ctl,title,message,style,buttons,result}
|-- picker/
|   |-- clone
|   `-- $id/{ctl,mode,prompt,types,result}
`-- indicator/
    |-- clone
    `-- $id/{ctl,text,icon,event}
```

Per-file semantics:

| Path | Mode | Semantics |
| --- | --- | --- |
| `status` | `-r--r--r--` | Tagged text: `api appkit`, `status ok`. |
| `app/ctl` | `--w--w----` | Verbs: `activate`, `quit`. Unknown verbs fail `Ebadarg`. |
| `app/status` | `-r--r--r--` | Tagged text for activation/visibility when implementable; otherwise `status unknown`. |
| `window/ctl` | `--w--w----` | Verbs: `center`, `fullscreen`, `toggle-fullscreen`, `minimize`, `miniaturize`, `close`. |
| `window/title` | `-rw-rw----` | Read/write title text. Reads may be cached unless native readback is implemented. |
| `window/status` | `-r--r--r--` | Tagged text: fullscreen/minimized/visible if available. |
| `pasteboard/ctl` | `--w--w----` | Verb: `clear`; clears the service-owned text pasteboard contents. |
| `pasteboard/text` | `-rw-rw----` | Write UTF-8 text to the general pasteboard; reads return last value written through the service until native readback exists. |
| `alert/clone` | `-r--r--r--` | Open allocates `alert/$id`; reading the fd returns the decimal id. |
| `alert/$id/ctl` | `--w--w----` | Verb: `show`; uses current parameter files and writes `result`. |
| `alert/$id/{title,message,style,buttons}` | `-rw-rw----` | Alert parameters. `style` is `informational`, `warning`, or `critical`; `buttons` is comma- or newline-separated labels. |
| `alert/$id/result` | `-r--r--r--` | Blocks until dismissal where possible; returns tagged text `button`, `index`, `response`. |
| `picker/clone` | `-r--r--r--` | Open allocates `picker/$id`; reading the fd returns the decimal id. |
| `picker/$id/ctl` | `--w--w----` | Verb: `show`; opens `NSOpenPanel`/`NSSavePanel` style UI. |
| `picker/$id/{mode,prompt,types}` | `-rw-rw----` | Request configuration. `mode` is `open-file`, `open-dir`, `save-file`; `types` is newline-separated UTIs/extensions. |
| `picker/$id/result` | `-r--r--r--` | Newline-separated selected paths, or tagged cancellation. |
| `indicator/clone` | `-r--r--r--` | Open allocates `indicator/$id`; reading the fd returns the decimal id. |
| `indicator/$id/ctl` | `--w--w----` | Verbs: `show`, `hide`, `destroy`. |
| `indicator/$id/text` | `-rw-rw----` | Text displayed in the macOS menu bar status item. |
| `indicator/$id/icon` | `--w--w----` | PNG/image bytes for the status item. |
| `indicator/$id/event` | `-r--r--r--` | Blocking newline-delimited tagged click/menu events. |

Mapping:

| Example behavior | File(s) | Read returns | Write does |
| --- | --- | --- | --- |
| App activation/quit | `app/ctl` | none | Calls `NSApplication` lifecycle methods. |
| Window title/control | `window/{title,ctl}` | cached title/status | Calls `NSWindow` setters/actions. |
| Text pasteboard | `pasteboard/text` | last service-written text | Clears and writes string pasteboard data. |
| Alert/dialog examples | `alert/$id/*` | tagged result | Shows `NSAlert`. |
| File picker example | `picker/$id/*` | paths or cancellation | Shows open/save panel. |
| Menu bar status items | `indicator/$id/*` | click events | Creates and controls `NSStatusItem`. |

Supported/fails-closed:

| Supported | Fails closed |
| --- | --- |
| Current app/window operations | Multi-window ownership |
| Text pasteboard writes | Rich pasteboard promises/types |
| Modal alerts and basic file panels | Custom `NSView` trees and sheets |
| Menu bar status items | Custom menu popovers and arbitrary views |
| Tagged status/results | Arbitrary Objective-C handles |

Example:

```sh
% id=`{cat macos/appkit/alert/clone}
% echo Confirm >macos/appkit/alert/$id/title
% echo 'Continue?' >macos/appkit/alert/$id/message
% echo 'Cancel,OK' >macos/appkit/alert/$id/buttons
% echo show >macos/appkit/alert/$id/ctl
% cat macos/appkit/alert/$id/result
button OK
index 1
response 1001
```

### `notify(4)` — local user notifications

`notify` is separate because it requests notification authorization and can
interrupt the user outside the app window.

```text
macos/notify
|-- status
|-- ctl
|-- event
|-- clone
`-- $id/{ctl,title,body,subtitle,sound,delay,result}
```

Per-file semantics:

| Path | Mode | Semantics |
| --- | --- | --- |
| `status` | `-r--r--r--` | Tagged authorization state: `authorized true/false`, `status denied/not-determined/authorized`. |
| `ctl` | `--w--w----` | Verbs: `request-auth`, `refresh`. `request-auth` may trigger OS UI. |
| `event` | `-r--r--r--` | Blocking JSON-line or tagged event stream for delivery/settings callbacks if delegate support is implemented. |
| `clone` | `-r--r--r--` | Open allocates `$id`; reading the fd returns the decimal id. |
| `$id/ctl` | `--w--w----` | Verbs: `schedule`, `cancel`. |
| `$id/{title,body,subtitle,sound,delay}` | `-rw-rw----` | Request configuration. `delay` accepts duration text such as `5s`; nonpositive delays become `1s`. `sound` is `default`, `none`, `silent`, or a bundled sound name. |
| `$id/result` | `-r--r--r--` | Tagged result: `status queued`, `id <request-id>`, `delay <duration>`, `status cancelled`, or `error ...`. |

Mapping:

| Example behavior | File(s) | Read returns | Write does |
| --- | --- | --- | --- |
| `RequestAuthorizationWithOptions` | `ctl`, `status` | authorization state | Requests alert/sound authorization. |
| Mutable notification content | `$id/{title,body,subtitle,sound}` | configured values | Sets request content. |
| Time interval trigger | `$id/delay` | duration | Sets one-shot delay. |
| Add notification request | `$id/ctl`, `$id/result` | queued/error | Schedules request. |
| Remove notification request | `$id/ctl`, `$id/result` | cancelled/error | Removes matching pending and delivered notifications. |

Supported/fails-closed:

| Supported | Fails closed |
| --- | --- |
| Local one-shot notifications | Remote push notifications |
| Authorization request/status | Reading notification history |
| Basic title/body/subtitle/sound/delay/cancel | Categories, actions, attachments |

Example:

```sh
% cat macos/notify/status
status not-determined
% echo request-auth >macos/notify/ctl
% n=`{cat macos/notify/clone}
% echo 'Build done' >macos/notify/$n/title
% echo 'wanix task completed' >macos/notify/$n/body
% echo 5s >macos/notify/$n/delay
% echo schedule >macos/notify/$n/ctl
% cat macos/notify/$n/result
status queued
```

### `touchid(4)` — local authentication

`touchid` is separate because it grants biometric/password authentication
power, not desktop UI control.

```text
macos/touchid
|-- status
|-- clone
`-- $id/{ctl,reason,result}
```

Per-file semantics:

| Path | Mode | Semantics |
| --- | --- | --- |
| `status` | `-r--r--r--` | Tagged availability: `available true/false`, `policy device-owner-authentication`. |
| `clone` | `-r--r--r--` | Open allocates `$id`; reading the fd returns the decimal id. |
| `$id/reason` | `-rw-rw----` | Localized prompt reason. |
| `$id/ctl` | `--w--w----` | Verb: `evaluate`; starts OS authentication and returns after dispatch. |
| `$id/result` | `-r--r--r--` | Blocks until completion; tagged text: `ok true` or `error user-cancel`. |

Mapping:

| Example behavior | File(s) | Read returns | Write does |
| --- | --- | --- | --- |
| Create/evaluate `LAContext` | `clone`, `$id/ctl` | id/result | Starts one evaluation. |
| Localized reason | `$id/reason` | prompt string | Sets reason shown to user. |
| Completion/error | `$id/result` | tagged outcome | none |

Supported/fails-closed:

| Supported | Fails closed |
| --- | --- |
| One-shot local auth check | Credential storage |
| User cancel/error reporting | Keychain mutation |
| Device-owner policy | Reusing external `LAContext` |

Example:

```sh
% a=`{cat macos/touchid/clone}
% echo 'unlock build secrets' >macos/touchid/$a/reason
% echo evaluate >macos/touchid/$a/ctl
% cat macos/touchid/$a/result
ok true
```

## Design Next

### `vision(4)` — local image analysis

`vision` runs local Vision requests over caller-provided image bytes. It does
not acquire pixels. Live capture belongs to `screen`, camera, or mic services.

```text
macos/vision
|-- status
|-- requests
|-- clone
`-- $id/{ctl,in,out,status,meta}
```

Per-file semantics:

| Path | Mode | Semantics |
| --- | --- | --- |
| `status` | `-r--r--r--` | Framework availability and model/request support. |
| `requests` | `-r--r--r--` | Tagged list: `ocr`, `barcode`, `classify`, `rectangles`, and only other implemented local requests. |
| `clone` | `-r--r--r--` | Open allocates `$id`; reading the fd returns the decimal id. |
| `$id/ctl` | `--w--w----` | Verbs: `ocr [fast|accurate]`, `barcode`, `classify`, `rectangles`, `lang <tag>`, `format text|json`, `run`, `abort`, `destroy`. |
| `$id/in` | `--w--w----` | Whole input image bytes. No host paths. |
| `$id/out` | `-r--r--r--` | Blocks until complete; UTF-8 text or JSON result records. |
| `$id/status` | `-r--r--r--` | Tagged job state and error. |
| `$id/meta` | `-r--r--r--` | Image dimensions, request revision, language hints, timing. |

Lifecycle: write the complete payload to `$id/in`, write request verbs and
`run` to `$id/ctl`, then read `$id/out`. Execution freezes the input bytes for
that run; later writes require a new `run`.

Mapping:

| Example behavior | File(s) | Read returns | Write does |
| --- | --- | --- | --- |
| Vision OCR JSON | `$id/{ctl,in,out,meta}` | text or JSON boxes | Selects OCR level/languages. |
| Barcode request | `$id/{ctl,in,out}` | JSON barcode records | Selects barcode recognition. |
| Classification request | `$id/{ctl,in,out}` | JSON classification records | Selects image classification. |
| Rectangle detection | `$id/{ctl,in,out}` | normalized rectangle records | Selects rectangle detection. |

Supported/fails-closed:

| Supported | Fails closed |
| --- | --- |
| Caller-provided image bytes | Live camera or screen capture |
| OCR, barcode, classification, rectangle detection, and bounded local requests | Arbitrary Vision request graphs |
| Text/JSON outputs | Model downloads or network inference |

Example:

```sh
% v=`{cat macos/vision/clone}
% cat page.png >macos/vision/$v/in
% echo 'ocr accurate' >macos/vision/$v/ctl
% echo 'format json' >macos/vision/$v/ctl
% echo run >macos/vision/$v/ctl
% cat macos/vision/$v/out
[{"text":"Recognized text","confidence":0.98}]
```

### `image(4)` — local image transforms

`image` owns CoreImage-style transforms over caller-provided image bytes. It is
separate from `vision` because transforms return media payloads, while Vision
returns analysis.

```text
macos/image
|-- filters
|-- clone
`-- $id/{ctl,in,out,status,meta}
```

Per-file semantics:

| Path | Mode | Semantics |
| --- | --- | --- |
| `filters` | `-r--r--r--` | Tagged list of supported filter names and parameters. |
| `clone` | `-r--r--r--` | Open allocates `$id`; reading the fd returns the decimal id. |
| `$id/ctl` | `--w--w----` | Verbs: `filter <name> [key=value...]`, `format png|jpeg|tiff`, `run`, `abort`, `destroy`. |
| `$id/in` | `--w--w----` | Whole input image bytes. |
| `$id/out` | `-r--r--r--` | Blocks until complete; encoded image bytes. |
| `$id/status` | `-r--r--r--` | Tagged job state and error. |
| `$id/meta` | `-r--r--r--` | Input/output dimensions, format, filter timing. |

Lifecycle: write the complete payload to `$id/in`, write filter/output verbs
and `run` to `$id/ctl`, then read `$id/out`. Execution freezes the input bytes
for that run; later writes require a new `run`.

Supported/fails-closed:

| Supported | Fails closed |
| --- | --- |
| Small allowlisted filters | Arbitrary CoreImage graphs |
| Caller-provided image bytes | Plugin loading |
| Encoded output bytes | GPU resource export |

Example:

```sh
% i=`{cat macos/image/clone}
% cat in.png >macos/image/$i/in
% echo 'filter grayscale' >macos/image/$i/ctl
% echo 'format png' >macos/image/$i/ctl
% echo run >macos/image/$i/ctl
% cat macos/image/$i/out >out.png
```

### `document(4)` — local document transforms

`document` handles local document extraction over caller-provided bytes. It is
separate from `vision` and `image` because PDF/page semantics are structured
and may produce text, page metadata, or derived assets.

```text
macos/document
|-- formats
|-- clone
`-- $id/{ctl,in,out,status,meta}
```

Per-file semantics:

| Path | Mode | Semantics |
| --- | --- | --- |
| `formats` | `-r--r--r--` | Tagged list such as `pdf-text`. |
| `clone` | `-r--r--r--` | Open allocates `$id`; reading the fd returns the decimal id. |
| `$id/ctl` | `--w--w----` | Verbs: `pdf-text`, `pages <range>`, `format text|json`, `run`, `abort`, `destroy`. |
| `$id/in` | `--w--w----` | Whole input document bytes. |
| `$id/out` | `-r--r--r--` | Blocks until complete; UTF-8 text or JSON records. |
| `$id/status` | `-r--r--r--` | Tagged job state and error. |
| `$id/meta` | `-r--r--r--` | Page count, title/author if available, timing. |

Lifecycle: write the complete payload to `$id/in`, write transform verbs and
`run` to `$id/ctl`, then read `$id/out`. Execution freezes the input bytes for
that run; later writes require a new `run`.

Supported/fails-closed:

| Supported | Fails closed |
| --- | --- |
| PDF text extraction | Host path reads |
| Page metadata | PDF form mutation/signing |
| Caller-provided document bytes | Network document loading |

Example:

```sh
% d=`{cat macos/document/clone}
% cat report.pdf >macos/document/$d/in
% echo pdf-text >macos/document/$d/ctl
% echo run >macos/document/$d/ctl
% cat macos/document/$d/out
```

### `mic(4)` — microphone capture

`mic` is a microphone capability: allocate, configure, capture bounded PCM
bytes, or start a background capture session that appends PCM bytes to `data`
until stopped.

```text
macos/mic
|-- status
|-- ctl
|-- devices
|-- clone
`-- $id/{ctl,status,format,duration,data,event}
```

Per-file semantics:

| Path | Mode | Semantics |
| --- | --- | --- |
| `status` | `-r--r--r--` | Tagged authorization state: `authorized`, `denied`, or `not-determined`. |
| `ctl` | `--w--w----` | Verb: `request-auth`; triggers the OS microphone TCC prompt. |
| `devices` | `-r--r--r--` | Tagged list of input devices when enumeration exists; otherwise `default true`. |
| `clone` | `-r--r--r--` | Open allocates `$id`; reading the fd returns the decimal id. |
| `$id/ctl` | `--w--w----` | Verbs: `device <name>`, `format pcm16`, `duration <d>`, `oneshot`, `start`, `stop`, `destroy`. |
| `$id/format` | `-r--r--r--` | Effective sample format after start. |
| `$id/duration` | `-rw-rw----` | Bounded capture duration for `oneshot`, default `1s`. |
| `$id/status` | `-r--r--r--` | Tagged state, frames, drops, errors. |
| `$id/data` | `-rw-rw----` | PCM16 mono 24 kHz bytes from `oneshot` or the current `start` session. |
| `$id/event` | `-r--r--r--` | Optional tagged events for device changes/errors. |

Mapping:

| Example behavior | File(s) | Read returns | Write does |
| --- | --- | --- | --- |
| AVAudioEngine input tap | `$id/{ctl,data}` | PCM bytes | Runs bounded or background capture. |
| Converter/output format | `$id/{ctl,format}` | effective format | Sets rate/channels/sample type. |
| Speech buffers | `$id/data` | audio buffers | none |
| Audio say | separate `speech(4)` service | status | plays text/audio |

Supported/fails-closed:

| Supported | Fails closed |
| --- | --- |
| Default mic PCM one-shot and append session | True blocking fd stream |
| Basic format conversion | Hardware gain/routing |
| Capture status counters | Multi-app aggregate devices |

Example:

```sh
% m=`{cat macos/mic/clone}
% echo 'format pcm16' >macos/mic/$m/ctl
% echo 'duration 1s' >macos/mic/$m/ctl
% echo oneshot >macos/mic/$m/ctl
% cat macos/mic/$m/data >/tmp/mic.pcm
% echo start >macos/mic/$m/ctl
% sleep 3
% echo stop >macos/mic/$m/ctl
% echo 'rate 24000' >macos/mic/$m/ctl
% echo start >macos/mic/$m/ctl
% dd -bs 4800 -count 10 <macos/mic/$m/data >sample.pcm
% echo stop >macos/mic/$m/ctl
```

### `speech(4)` — text-to-speech output

`speech` is distinct from `mic`: it is an audio-output and utterance queue
service, not microphone capture. It can ship after `appkit`/`notify`/`touchid`
once the host has a clear policy for guest-controlled speaker output.

```text
macos/speech
|-- voices
|-- clone
`-- $id/{ctl,text,ssml,voice,rate,pitch,volume,assistive,status,event}
```

Per-file semantics:

| Path | Mode | Semantics |
| --- | --- | --- |
| `voices` | `-r--r--r--` | Tagged list of available voice identifiers and languages. |
| `clone` | `-r--r--r--` | Open allocates `$id`; reading the fd returns the decimal id. |
| `$id/ctl` | `--w--w----` | Verbs: `speak`, `pause`, `resume`, `stop`, `rate <n>`, `voice <id-or-lang>`, `pitch <n>`, `volume <n>`, `assistive <bool>`, `destroy`. |
| `$id/text` | `-rw-rw----` | UTF-8 text to speak. |
| `$id/ssml` | `-rw-rw----` | Optional SSML representation. When set, it is used instead of `text`. |
| `$id/voice` | `-rw-rw----` | Voice identifier or language tag. |
| `$id/rate` | `-rw-rw----` | Speech rate as tagged text/decimal. |
| `$id/pitch` | `-rw-rw----` | Pitch multiplier as decimal or `default`. |
| `$id/volume` | `-rw-rw----` | Output volume as decimal or `default`. |
| `$id/assistive` | `-rw-rw----` | Whether to prefer assistive technology settings: `true`, `false`, or `default`. |
| `$id/status` | `-r--r--r--` | Tagged state: `queued`, `speaking`, `finished`, `stopped`, `error`. |
| `$id/event` | `-r--r--r--` | Utterance events: `start`, `range <off> <len>`, `pause`, `resume`, `finish`, `cancel`. |

Mapping:

| Example behavior | File(s) | Read returns | Write does |
| --- | --- | --- | --- |
| AVSpeech utterance text | `$id/text` | current text | Sets utterance text. |
| SSML utterance | `$id/ssml` | current SSML | Sets structured utterance text. |
| Voice/rate/pitch/volume selection | `$id/{voice,rate,pitch,volume}` | current setting | Sets synthesizer options. |
| Speak/stop | `$id/ctl`, `$id/status` | state | Starts or stops output. |

Supported/fails-closed:

| Supported | Fails closed |
| --- | --- |
| Text-to-speech through system voices | Capturing generated PCM bytes |
| Basic voice/rate/pitch/volume controls and SSML | Siri/private voices unless explicitly exposed |
| Utterance status/events | Arbitrary audio graph routing |

Example:

```sh
% s=`{cat macos/speech/clone}
% echo 'build complete' >macos/speech/$s/text
% echo 'rate 0.55' >macos/speech/$s/ctl
% echo speak >macos/speech/$s/ctl
% cat macos/speech/$s/status
state speaking
```

### `screen(4)` — screen and window capture

`screen` owns Screen Recording TCC and must not be folded into `appkit`.

```text
macos/screen
|-- status
|-- ctl
|-- displays
|-- windows
|-- clone
`-- $id/{ctl,status,format,target,fps,frames,event}
```

Per-file semantics:

| Path | Mode | Semantics |
| --- | --- | --- |
| `status` | `-r--r--r--` | Tagged authorization state: `authorized`, `denied`, or `not-determined`. |
| `ctl` | `--w--w----` | Verb: `request-auth`; triggers the OS Screen Recording TCC prompt. |
| `displays`, `windows` | `-r--r--r--` | JSON arrays because consumers need ids, titles, bounds, owner bundle ids; may be empty or fail if unauthorized. |
| `clone` | `-r--r--r--` | Open allocates `$id`; reading the fd returns the decimal id. |
| `$id/ctl` | `--w--w----` | Verbs: `target display <id>`, `target window <id>`, `fps <n>`, `format png`, `oneshot`, `start`, `stop`, `destroy`. |
| `$id/format` | `-r--r--r--` | Effective pixel format/dimensions. |
| `$id/target` | `-rw-rw----` | Capture target, empty for the first display, or `display <id>` / `window <id>`. |
| `$id/fps` | `-rw-rw----` | Capture rate for `start`, clamped to 1-30 fps. |
| `$id/frames` | `-rw-rw----` | PNG bytes from `oneshot`, or appended `frame N bytes M\n<png>\n` records from `start`. |
| `$id/event` | `-r--r--r--` | Frame drops, TCC denial, target disappearance. |
| `$id/status` | `-r--r--r--` | Tagged state and counters. |

Mapping:

| Example behavior | File(s) | Read returns | Write does |
| --- | --- | --- | --- |
| Window list | `windows` | JSON window records | none |
| Screenshot | `$id/{ctl,frames}` | one frame | `oneshot target ...` |
| Stream frames | `$id/{ctl,frames,event}` | appended PNG records | `start` / `stop` |

Supported/fails-closed:

| Supported | Fails closed |
| --- | --- |
| Display/window enumeration | Input injection |
| One-shot screenshots | Capturing hidden/protected content |
| Appended PNG frame records | Audio capture |

Example:

```sh
% cat macos/screen/windows | jq '.[0]'
% s=`{cat macos/screen/clone}
% echo 'target window 123' >macos/screen/$s/ctl
% echo 'format png' >macos/screen/$s/ctl
% echo oneshot >macos/screen/$s/ctl
% read -c 1048576 <macos/screen/$s/frames >shot.png
```

### `ax(4)` — Accessibility automation

`ax` is a separate high-authority capability. It can inspect and act on other
applications after Accessibility trust is granted, so it must not be part of
`appkit`. Start with bounded read-only queries, then add explicit actions.

```text
macos/ax
|-- status
|-- ctl
|-- apps
|-- system/
|   `-- focused
|-- app/
|   |-- clone
|   `-- $id/{ctl,pid,bundleid,status,tree,focused,event}
`-- element/
    |-- clone
    `-- $id/{ctl,attrs,actions,value,frame,result,event}
```

Per-file semantics:

| Path | Mode | Semantics |
| --- | --- | --- |
| `status` | `-r--r--r--` | Tagged trust/API state: `trusted true/false`, `api enabled/disabled`. |
| `ctl` | `--w--w----` | Verbs: `request-trust`, `refresh`. `request-trust` may open System Settings or trigger the OS prompt. |
| `apps` | `-r--r--r--` | JSON or tagged list of visible/running app targets: pid, bundle id, name. |
| `system/focused` | `-r--r--r--` | Bounded description of focused app/window/element. |
| `app/clone` | `-r--r--r--` | Open allocates `app/$id`; reading the fd returns the decimal id. |
| `app/$id/pid` | `-rw-rw----` | Target process id. |
| `app/$id/bundleid` | `-rw-rw----` | Target bundle id. |
| `app/$id/ctl` | `--w--w----` | Verbs: `attach`, `refresh`, `watch <notification>`, `unwatch <notification>`, `destroy`. |
| `app/$id/tree` | `-r--r--r--` | Bounded JSON accessibility tree; depth and count limits are required. |
| `app/$id/focused` | `-r--r--r--` | Focused element summary within this app. |
| `app/$id/status` | `-r--r--r--` | Attach/trust/error state. |
| `app/$id/event` | `-r--r--r--` | AXObserver notifications as tagged records or JSON lines. |
| `element/clone` | `-r--r--r--` | Open allocates `element/$id`; reading the fd returns the decimal id. |
| `element/$id/ctl` | `--w--w----` | Verbs: `from-focused`, `from-path <app-id> <path>`, `press`, `set-value`, `focus`, `destroy`. Mutating verbs require explicit allowlist. |
| `element/$id/attrs` | `-r--r--r--` | JSON attributes: role, title, value summary, enabled, selected, children count. |
| `element/$id/actions` | `-r--r--r--` | Supported action names. |
| `element/$id/value` | `-rw-rw----` | Current value for safe editable controls only. |
| `element/$id/frame` | `-r--r--r--` | Position/size if available. |
| `element/$id/result` | `-r--r--r--` | Result of last action. |
| `element/$id/event` | `-r--r--r--` | Element-specific notification stream when watched. |

Mapping:

| Example behavior | File(s) | Read returns | Write does |
| --- | --- | --- | --- |
| Trust check/prompt | `status`, `ctl` | trust state | Requests Accessibility permission. |
| Attach to app | `app/$id/{pid,bundleid,ctl,status}` | attach status | Creates `AXUIElementCreateApplication` target. |
| Focused element | `system/focused`, `app/$id/focused` | bounded element summary | none |
| Attribute/action query | `element/$id/{attrs,actions,frame}` | JSON attributes/actions | none |
| Explicit action | `element/$id/ctl`, `result` | action result | Performs allowlisted `AXPress`, focus, or safe value writes. |
| Notifications | `app/$id/event`, `element/$id/event` | AXObserver events | Registers explicit notifications. |

Supported/fails-closed:

| Supported | Fails closed |
| --- | --- |
| Visible trust state and permission prompt | Hidden automation when untrusted |
| Attach by pid or bundle id | Arbitrary global element walks |
| Bounded read-only tree/attribute queries | Raw `AXUIElementRef` handles |
| Explicit allowlisted actions | Keylogging or input monitoring |
| Observer events for selected targets | Private `_AXUIElementGetWindow` unless separately gated |

Example:

```sh
% cat macos/ax/status
trusted false
% echo request-trust >macos/ax/ctl
% a=`{cat macos/ax/app/clone}
% echo com.apple.TextEdit >macos/ax/app/$a/bundleid
% echo attach >macos/ax/app/$a/ctl
% cat macos/ax/app/$a/focused
```

### `keychain(4)` — credential and trust queries

Start read-only. Mutating secrets is a separate, much higher-risk design.

```text
macos/keychain
|-- clone
`-- $id/{ctl,query,results,status}
```

Per-file semantics:

| Path | Mode | Semantics |
| --- | --- | --- |
| `clone` | `-r--r--r--` | Open allocates `$id`; reading the fd returns the decimal id. |
| `$id/ctl` | `--w--w----` | Verbs: `query generic-password`, `query certificate`, `trust-evaluate`, `destroy`. |
| `$id/query` | `-rw-rw----` | JSON dictionary for Security query constraints, because the API is key/value structured. |
| `$id/results` | `-r--r--r--` | JSON result records without secret data by default. |
| `$id/status` | `-r--r--r--` | Tagged OSStatus/error. |

Mapping:

| Example behavior | File(s) | Read returns | Write does |
| --- | --- | --- | --- |
| Generic password query | `$id/{query,ctl,results}` | attributes | Sets service/account constraints and runs query. |
| Trust evaluation | `$id/{query,ctl,results}` | trust result | Evaluates `certificate_pem` or base64 `certificate_der` with optional `policy` and `hostname`. |
| Auth dialog | likely `touchid` or explicit `authorize` verb | authorization result | Triggers user auth only when requested. |

Supported/fails-closed:

| Supported | Fails closed |
| --- | --- |
| Read-only attribute queries | Adding/updating/deleting items |
| Trust evaluation | Exporting private keys/passwords |
| Explicit user auth hook | Silent credential prompts |

Supported trust policies are `basic-x509`, `ssl-server`, and `ssl-client`.
When `hostname` is present and no policy is specified, `ssl-server` is used;
otherwise trust evaluation defaults to `basic-x509`.

Example:

```sh
% k=`{cat macos/keychain/clone}
% echo '{"service":"com.example","account":"me"}' >macos/keychain/$k/query
% echo 'query generic-password' >macos/keychain/$k/ctl
% cat macos/keychain/$k/results
[{"class":"generic-password","service":"com.example","account":"me"}]
```

### `reachability(4)` — network state inspection

This is read-only SystemConfiguration state. It must not mutate host network
preferences.

```text
macos/reachability
|-- proxies
|-- clone
`-- $id/{target,flags,status,event}
```

Per-file semantics:

| Path | Mode | Semantics |
| --- | --- | --- |
| `proxies` | `-r--r--r--` | JSON or tagged snapshot of current proxy settings. |
| `clone` | `-r--r--r--` | Open allocates `$id`; reading the fd returns the decimal id. |
| `$id/target` | `-rw-rw----` | Hostname or address. |
| `$id/flags` | `-r--r--r--` | Tagged reachability flags. |
| `$id/status` | `-r--r--r--` | State/error for monitor. |
| `$id/event` | `-r--r--r--` | Optional blocking changes if callback mode is implemented. |

Mapping:

| Example behavior | File(s) | Read returns | Write does |
| --- | --- | --- | --- |
| `SCNetworkReachabilityCreateWithName` | `$id/target` | target | Sets target. |
| `SCNetworkReachabilityGetFlags` | `$id/flags` | flags | none |
| `SCDynamicStoreCopyProxies` | `proxies` | proxy snapshot | none |

Supported/fails-closed:

| Supported | Fails closed |
| --- | --- |
| Host/proxy state reads | Proxy preference writes |
| Per-target reachability | VPN/provider activation |
| Optional event stream | `SCPreferencesCommitChanges` |

Example:

```sh
% r=`{cat macos/reachability/clone}
% echo localhost >macos/reachability/$r/target
% cat macos/reachability/$r/flags
reachable true
local-address true
% cat macos/reachability/proxies
http-enable false
```

### `vz(4)` — Virtualization.framework VMs

`vz` is proc-shaped: clone a VM, configure it, start it, interact through
console/status/event files, then destroy it.

```text
macos/vz
|-- status
|-- clone
`-- $id/{ctl,config,disk,net,status,console,event,report}
```

Per-file semantics:

| Path | Mode | Semantics |
| --- | --- | --- |
| `status` | `-r--r--r--` | Entitlement/framework availability. |
| `clone` | `-r--r--r--` | Open allocates `$id`; reading the fd returns the decimal id. |
| `$id/ctl` | `--w--w----` | Verbs: `validate`, `start`, `pause`, `resume`, `stop`, `kill`, `destroy`. |
| `$id/config` | `-rw-rw----` | JSON hardware config: `cpus`, `memoryMiB`, `bootMode`, optional `disk`, `efiVars`, `serialLog`, `network`, `readOnly`, `appendSerial`. |
| `$id/disk` | `-rw-rw----` | Disk image path used when `config.disk` is empty. |
| `$id/net` | `-rw-rw----` | Network mode such as `nat`; enables a VZ NAT network attachment. |
| `$id/status` | `-r--r--r--` | Tagged VM state and resource summary. |
| `$id/console` | `-rw-rw----` | Reserved console file; serial output is currently written to `config.serialLog` or a generated temp log. |
| `$id/event` | `-r--r--r--` | Blocking lifecycle events. |
| `$id/report` | `-r--r--r--` | JSON probe/run report when enabled. |

Mapping:

| Example behavior | File(s) | Read returns | Write does |
| --- | --- | --- | --- |
| Plan9front-style config | `$id/{config,disk,net}` | config | Sets VM hardware/storage/network. |
| VZVirtualMachine start/stop | `$id/ctl`, `$id/status` | state | Starts/stops VM. |
| Serial log | `$id/report` | configured log path | Creates VZ file serial attachment. |
| Probe report | `$id/report` | structured report | none |

Supported/fails-closed:

| Supported | Fails closed |
| --- | --- |
| EFI VM lifecycle | Entitlement-free use |
| Raw disk image path | NBD and arbitrary host filesystem passthrough |
| File-backed serial log | Interactive console input |
| NAT network mode | Bridged/private networking policy |

Example:

```sh
% v=`{cat macos/vz/clone}
% echo '{"cpus":4,"memoryMiB":4096,"bootMode":"efi","network":true}' >macos/vz/$v/config
% echo '/Users/me/plan9.img' >macos/vz/$v/disk
% echo start >macos/vz/$v/ctl
% echo nat >macos/vz/$v/net
% echo start >macos/vz/$v/ctl
% tail -f macos/vz/$v/console
```

## Research Only

### `xpc(4)` — XPC listener/connection shape

A general XPC capability is not ready because it crosses launchd/bootstrap
namespaces. The useful research shape is a call/return service for explicit
Wanix peer communication.

```text
macos/xpc
|-- listeners
|-- clone
`-- $id/{ctl,endpoint,call,return,status,event}
```

Minimum semantics: `ctl` creates an anonymous listener or connects to an
explicit endpoint; `call` is a blocking JSON-line request stream; `return` is a
write-only response pipe. Do not expose launchd service registration until the
bootstrap authority boundary is designed.

### `webkit(4)` — embedded WebKit page/runtime control

Wanix already runs inside WebKit, so a general WebKit FS is research-only. It
becomes useful only for a workbench that intentionally exposes page navigation,
custom URL scheme state, downloads, or WebExtension data.

```text
macos/webkit
|-- pages
|-- clone
`-- $id/{ctl,url,status,console,download,event}
```

Fails closed: arbitrary JS heap handles, cross-origin bypass, browser profile
secrets.

## Do Not Expose Yet

### `fskit(4)`

FSKit hosts filesystems into macOS; it is not a simple guest-visible Apple
capability. The examples require app-extension bundles, signing, install, and
System Settings enablement. Keep only an artifact/design note until those gates
are productized.

Minimum shape, if it later ships:

```text
macos/fskit
|-- status
|-- providers
|-- clone
`-- $id/{ctl,bundle,mount,status,event}
```

Fails closed today: dynamic unapproved extension install, host mount mutation,
and arbitrary backend exports.

### `cloudkit(4)`

CloudKit requires signed app identity, iCloud entitlements, container setup,
and account state. Treat it as an app-specific sync feature, not a generic
capability yet.

Minimum future shape:

```text
macos/cloudkit
|-- account
|-- containers
|-- clone
`-- $id/{ctl,container,record,in,out,status}
```

Fails closed today: provisioning profile management, container creation,
background sync, conflict policy hidden from the caller.

### `netext(4)` and `endpointsecurity(4)`

NetworkExtension and EndpointSecurity observe or mutate host-wide network and
security state. They require system extensions, preference writes, special
entitlements, or real traffic subscription. Do not expose them until there is a
host-admin product boundary.

Minimum future shape would use provider clone dirs plus `event` and `return`
for authorization callbacks; shipping that without entitlement/UI policy would
be unsafe.

### `sm(4)`

ServiceManagement registers login items, launch agents, or helpers. That is
durable host mutation, not a normal guest capability.

Minimum future shape:

```text
macos/sm
|-- services
|-- clone
`-- $id/{ctl,bundle,status,result}
```

Fails closed today: registering/unregistering helpers, privileged helper
install, persistent launchd state changes.
