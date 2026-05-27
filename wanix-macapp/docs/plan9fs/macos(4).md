# macos(4)

## Name

macos - Wanix filesystem for host macOS capabilities

## Synopsis

```text
macos/status
macos/appkit/...
macos/notify/...
macos/touchid/...
macos/vision/...
macos/speech/...
macos/mic/...
macos/screen/...
macos/ax/...
macos/keychain/...
macos/reachability/...
macos/image/...
macos/document/...
macos/vz/...
```

## Description

`#macos` is a host-backed Wanix device that exposes macOS capabilities as a
synthetic Plan 9-style filesystem. The canonical mount point is `macos`.
`mnt/macos` is a compatibility bind of the same device; new programs should use
`macos`.

The namespace is split by authority boundary. AppKit host-window control,
notifications, local authentication, image analysis, speech, microphone input,
screen capture, Accessibility, keychain, reachability, media transforms, and
Virtualization.framework are separate services. Possession of a mounted service
is the authority to use the subset documented for that service. There is no
per-call auth token file unless the Apple API itself needs per-call
authorization.

The root status file is read-only tagged text:

```text
api macos
status ok
```

## Conventions

Paths are relative to `macos` unless shown with the full mount point.

`clone` allocates a numbered session directory. Reading `service/clone` returns
the decimal id and creates `service/$id` or the documented nested directory such
as `ax/app/$id`.

`ctl` files are write-only command files. Commands are lower-case verbs with
optional arguments. Unknown verbs fail. Empty writes do nothing only where the
current host handler already treats them as no-ops.

`status`, `result`, and `meta` are read-only report files. Small fixed reports
use tagged text, one `key value` per line. Sparse or nested records use JSON.

`data`, `frames`, `console`, and `event` are payload or stream files. Binary
payloads are never mixed with event metadata. `event` records are tagged text or
JSON lines and report control-plane changes such as start, finish, cancellation,
authorization errors, and target disappearance.

Unsupported paths fail with not-found. Writes to read-only files fail with
permission denied. Unknown `ctl` verbs fail with bad-ctl style errors. Sensitive
or unimplemented capabilities are exposed, if at all, as read-only policy
directories with `status` and `README`.

## WebKit bridge

WebKit is transport, not an API namespace. The bridge should expose generic
filesystem operations:

```text
open path mode
read fid count offset
write fid bytes offset
clunk fid
stat path
readdir path
```

The current bridge already routes `readFile`, `writeFile`, `readDir`, and
`isDir` through a generic `native.fs` message. The durable contract is the
filesystem protocol above. JavaScript may cache directory entries or clone
schemas for performance, but it must not contain Apple service rules. Go owns
service registration, path routing, clone allocation, file state, policy, and
native Apple calls. Main-thread AppKit/WebKit requirements are host
implementation details.

## Service summary

| Service | Authority boundary | Lifecycle |
| --- | --- | --- |
| `appkit` | Current host application UI | Singletons plus clone modal/status-item sessions |
| `notify` | User notification permission | Service auth plus clone notification requests |
| `touchid` | LocalAuthentication UI | Clone one-shot evaluations |
| `vision` | Local Vision analysis over caller bytes | Clone request/response jobs |
| `speech` | Speaker output | Clone utterance jobs |
| `mic` | Microphone TCC and input devices | Clone bounded capture or stream sessions |
| `screen` | Screen Recording TCC | Target registry plus clone captures |
| `ax` | Accessibility trust and other-app control | Target app and element sessions |
| `keychain` | Security/keychain access | Clone read-only queries and trust evaluation |
| `reachability` | Host network state observation | Service snapshot plus clone checks |
| `image` | Local image transforms over caller bytes | Clone request/response jobs |
| `document` | Local document extraction over caller bytes | Clone request/response jobs |
| `vz` | Virtualization entitlement and VM resources | Proc-style clone VM directories |

## appkit

`appkit` controls only the current host app shell: `NSApplication`, the main
window, the general pasteboard text surface, modal panels, and menu-bar status
items. It must not absorb notification, biometric, keychain, capture,
Accessibility, or VM authority.

```text
macos/appkit
|-- status
|-- app/{ctl,status}
|-- window/{ctl,title,status}
|-- pasteboard/{ctl,text}
|-- alert/{clone,$id/{ctl,title,message,style,buttons,result}}
|-- picker/{clone,$id/{ctl,mode,prompt,types,result}}
`-- indicator/{clone,$id/{ctl,text,icon,status,event}}
```

| File | Mode | Semantics |
| --- | --- | --- |
| `status` | read | `api appkit`, `status ok`. |
| `app/ctl` | write | `activate`, `quit`. |
| `app/status` | read | Activation or termination state when known; otherwise `status unknown`. |
| `window/ctl` | write | `center`, `fullscreen`, `toggle-fullscreen`, `minimize`, `miniaturize`, `close`. |
| `window/title` | read/write | UTF-8 title text. |
| `window/status` | read | Last host window action or native state summary. |
| `pasteboard/ctl` | write | `clear`. |
| `pasteboard/text` | read/write | UTF-8 general pasteboard text owned through this service. |
| `alert/clone` | read | Allocates `alert/$id`. |
| `alert/$id/ctl` | write | `show`. |
| `alert/$id/{title,message,style,buttons}` | read/write | Alert parameters. `style` is `informational`, `warning`, or `critical`; `buttons` is comma- or newline-separated. |
| `alert/$id/result` | read | Tagged modal result: `button`, `index`, `response`. |
| `picker/clone` | read | Allocates `picker/$id`. |
| `picker/$id/ctl` | write | `show`. |
| `picker/$id/{mode,prompt,types}` | read/write | `mode` is `open-file`, `open-dir`, or `save-file`; `types` is newline-separated UTIs/extensions. |
| `picker/$id/result` | read | Selected paths or tagged cancellation. |
| `indicator/clone` | read | Allocates `indicator/$id`. |
| `indicator/$id/ctl` | write | `show`, `hide`, `destroy`. |
| `indicator/$id/text` | read/write | Menu-bar status item title. |
| `indicator/$id/icon` | write | Image bytes for a future icon-capable implementation. |
| `indicator/$id/status` | read | `status visible`, `hidden`, or `destroyed`. |
| `indicator/$id/event` | read | Tagged click/menu events when implemented. |

Fails closed: multi-window ownership, raw Objective-C handles, rich pasteboard
types, file promises, custom panel view trees, arbitrary menu popovers.

## notify

`notify` owns local user notifications. It may trigger notification
authorization UI and may interrupt the user outside the Wanix window.

```text
macos/notify
|-- status
|-- ctl
|-- event
|-- clone
`-- $id/{ctl,title,body,subtitle,sound,delay,result}
```

| File | Mode | Semantics |
| --- | --- | --- |
| `status` | read | Tagged authorization state: `status`, `authorized`. |
| `ctl` | write | `request-auth`, `refresh`. |
| `event` | read | Delivery/settings callback stream when delegate support exists. |
| `clone` | read | Allocates `$id`. |
| `$id/ctl` | write | `schedule`, `cancel`. |
| `$id/{title,body,subtitle,sound,delay}` | read/write | Notification content. Empty title defaults to `Wanix`. `delay` is Go duration text; empty or nonpositive becomes `1s`. `sound` is `default`, `none`, `silent`, or a bundled sound name. |
| `$id/result` | read | `status queued`, `id wanix-$id`, `delay <d>`, `status cancelled`, or `status error`. |

Fails closed: remote push notifications, history scraping, categories, actions,
attachments, arbitrary delivered-notification reads.

## touchid

`touchid` is the local authentication service. The name is historical; the
status reports Touch ID, Face ID, Optic ID, or no biometry as available. The
policy is device-owner authentication, so password fallback is allowed by the
OS.

```text
macos/touchid
|-- status
|-- ctl
|-- clone
`-- $id/{ctl,reason,result}
```

| File | Mode | Semantics |
| --- | --- | --- |
| `status` | read | `api touchid`, `status available/unavailable`, `available`, `biometry`, `biometry-available`, `policy device-owner-authentication`. |
| `ctl` | write | `refresh`. |
| `clone` | read | Allocates `$id`. |
| `$id/reason` | read/write | Localized prompt reason. Empty uses a host default. |
| `$id/ctl` | write | `evaluate`. |
| `$id/result` | read | `ok true`, `ok false`, and optional `error`. |

Fails closed: credential storage, keychain mutation, external `LAContext`
reuse, silent authentication, authentication without OS UI when the OS requires
it.

## vision

`vision` runs local Vision requests over caller-provided image bytes. It does
not acquire pixels. Live capture belongs to `screen`.

```text
macos/vision
|-- status
|-- requests
|-- clone
`-- $id/{ctl,in,out,status,meta,request,lang,level,format}
```

| File | Mode | Semantics |
| --- | --- | --- |
| `status` | read | Framework availability and service status. |
| `requests` | read | Supported requests: `ocr`, `barcode`, `classify`, `rectangles`. |
| `clone` | read | Allocates `$id`. |
| `$id/ctl` | write | `ocr [fast|accurate]`, `barcode`, `classify`, `rectangles`, `lang <tag>`, `format text|json`, `run`, `abort`, `destroy`. |
| `$id/in` | write | Complete input image bytes. No host paths. |
| `$id/out` | read | Text or JSON results. |
| `$id/status` | read | `status configured`, `done`, or `error`. |
| `$id/meta` | read | Image/request metadata when available. |
| `$id/{request,lang,level,format}` | read/write | Current request settings. |

Fails closed: camera/screen acquisition, arbitrary Vision request graphs, model
downloads, network inference, host path reads.

## speech

`speech` owns text-to-speech output. It is separate from `mic` because speaker
output and microphone capture are different authorities.

```text
macos/speech
|-- ctl
|-- voices
|-- clone
`-- $id/{ctl,text,ssml,voice,rate,pitch,volume,assistive,status,event}
```

| File | Mode | Semantics |
| --- | --- | --- |
| `ctl` | write | `refresh`, updating `voices`. |
| `voices` | read | JSON list of system voices with id, name, language, quality, gender, traits. |
| `clone` | read | Allocates `$id`. |
| `$id/ctl` | write | `speak`, `pause`, `resume`, `stop`, `rate <n>`, `voice <id-or-lang>`, `pitch <n>`, `volume <n>`, `assistive <bool>`, `destroy`. |
| `$id/text` | read/write | UTF-8 utterance text. |
| `$id/ssml` | read/write | Optional SSML text; when present it overrides `text`. |
| `$id/{voice,rate,pitch,volume,assistive}` | read/write | Current utterance settings. |
| `$id/status` | read | `status idle`, `speaking`, `paused`, `finished`, `stopped`, or `error`. |
| `$id/event` | read | Utterance events: `start`, `range`, `pause`, `resume`, `finish`, `cancel`. |

Fails closed: capturing generated PCM bytes, arbitrary audio routing, private
voices unless visible in the system voice list.

## mic

`mic` owns microphone capture and input-device visibility. `request-auth` may
trigger the macOS microphone TCC prompt.

```text
macos/mic
|-- status
|-- ctl
|-- devices
|-- clone
`-- $id/{ctl,status,format,duration,data,event}
```

| File | Mode | Semantics |
| --- | --- | --- |
| `status` | read | `api mic`, `status`, `authorized`, `devices`. |
| `ctl` | write | `request-auth`, `refresh`. |
| `devices` | read | JSON input-device list when refresh succeeds. |
| `clone` | read | Allocates `$id`. |
| `$id/ctl` | write | `device <name>`, `format pcm16`, `duration <d>`, `oneshot`, `start`, `stop`, `destroy`. |
| `$id/status` | read | `status configured`, `running`, `done`, `stopped`, `idle`, or `error`; includes byte/frame counters. |
| `$id/format` | read | Effective format, currently PCM16 mono 24 kHz in the host implementation. |
| `$id/duration` | read/write | Bounded capture duration for `oneshot`. |
| `$id/data` | read/write | PCM bytes. One-shot writes replace bytes; stream appends drain on read. |
| `$id/event` | read | Device/error events when implemented. |

Fails closed: unbounded hidden recording, hardware gain/routing, aggregate
device control, speaker capture, raw device handles.

## screen

`screen` owns Screen Recording TCC and screen/window capture. It must not be
folded into AppKit.

```text
macos/screen
|-- status
|-- ctl
|-- displays
|-- windows
|-- clone
`-- $id/{ctl,status,format,target,fps,frames,event}
```

| File | Mode | Semantics |
| --- | --- | --- |
| `status` | read | `api screen`, `status`, `authorized`, display/window counts, or error. |
| `ctl` | write | `request-auth`, `refresh`; both refresh shareable content in the current implementation. |
| `displays` | read | JSON display records. |
| `windows` | read | JSON window records with id, title, app name, bundle id, pid, bounds, visibility. |
| `clone` | read | Allocates `$id`. |
| `$id/ctl` | write | `target display <id>`, `target window <id>`, `fps <n>`, `format png`, `oneshot`, `start`, `stop`, `destroy`. |
| `$id/status` | read | Capture state and byte/frame counters. |
| `$id/format` | read | Effective image format, currently PNG. |
| `$id/target` | read/write | Empty for default display or `display <id>` / `window <id>`. |
| `$id/fps` | read/write | Stream rate, clamped to 1-30 fps. |
| `$id/frames` | read/write | One-shot PNG bytes, or stream records `frame N bytes M\n<png>\n`. |
| `$id/event` | read | TCC denial, target disappearance, frame drop events when implemented. |

Fails closed: input injection, hidden/protected content capture, audio capture,
arbitrary window-server handles.

## ax

`ax` owns Accessibility trust and explicit automation of other apps. It starts
with bounded read-only queries and only allowlisted actions.

```text
macos/ax
|-- status
|-- ctl
|-- apps
|-- system/focused
|-- app/{clone,$id/{ctl,pid,bundleid,status,tree,focused,event}}
`-- element/{clone,$id/{ctl,attrs,actions,value,frame,result,event}}
```

| File | Mode | Semantics |
| --- | --- | --- |
| `status` | read | `api ax`, `trusted true/false`. |
| `ctl` | write | `request-trust`, `refresh`. |
| `apps` | read | JSON app target list when implemented. |
| `system/focused` | read | JSON focused app/window/element summary. |
| `app/clone` | read | Allocates `app/$id`. |
| `app/$id/pid` | read/write | Target pid. |
| `app/$id/bundleid` | read/write | Target bundle id. |
| `app/$id/ctl` | write | `attach`, `refresh`, `destroy`; future `watch` verbs must be explicit. |
| `app/$id/status` | read | Attach/trust/error state. |
| `app/$id/tree` | read | Bounded JSON AX tree. Current bounds are depth 2, 128 nodes. |
| `app/$id/focused` | read | Focused element summary in the target app. |
| `app/$id/event` | read | AXObserver events when implemented. |
| `element/clone` | read | Allocates `element/$id`. |
| `element/$id/ctl` | write | `from-focused`, `press`, `focus`, `destroy`; value mutation requires an explicit future verb. |
| `element/$id/attrs` | read | JSON attributes. |
| `element/$id/actions` | read | JSON action names. |
| `element/$id/value` | read/write | Safe editable value only when supported. |
| `element/$id/frame` | read | JSON frame. |
| `element/$id/result` | read | Last action result. |
| `element/$id/event` | read | Element events when implemented. |

Fails closed: hidden automation when untrusted, arbitrary global walks, raw
`AXUIElementRef` handles, keylogging, input monitoring, private AX APIs unless
separately gated.

## keychain

`keychain` starts read-only. Mutating secrets is a separate design and is not
part of this service.

```text
macos/keychain
|-- clone
`-- $id/{ctl,query,results,status}
```

| File | Mode | Semantics |
| --- | --- | --- |
| `clone` | read | Allocates `$id`. |
| `$id/ctl` | write | `query generic-password`, `query certificate`, `trust-evaluate`, `destroy`. |
| `$id/query` | read/write | JSON query. Fields include `service`, `account`, `label`, `certificate_der`, `certificate_pem`, `hostname`, `policy`. |
| `$id/results` | read | JSON records without secret values. Trust evaluation returns trust records. |
| `$id/status` | read | `status done`, `no-match`, `trusted`, `untrusted`, or `error`; may include `osstatus` or `trust-result`. |

Trust policies are `basic-x509`, `ssl-server`, and `ssl-client`. If
`hostname` is set and no policy is supplied, `ssl-server` is used. Otherwise
trust evaluation defaults to `basic-x509`.

Fails closed: adding, updating, deleting, or exporting items; private keys;
password material; silent credential prompts.

## reachability

`reachability` is read-only host network state. It does not mutate network
preferences.

```text
macos/reachability
|-- ctl
|-- status
|-- proxies
|-- clone
`-- $id/{ctl,target,flags,status,event}
```

| File | Mode | Semantics |
| --- | --- | --- |
| `ctl` | write | `refresh`. |
| `status` | read | `api reachability`, `status`, `reachable`. |
| `proxies` | read | Tagged proxy/path snapshot; current host reports `http-enable`, `expensive`, and `constrained`. |
| `clone` | read | Allocates `$id`. |
| `$id/ctl` | write | `check`, `destroy`. |
| `$id/target` | read/write | Hostname/address for future target-specific checks. Current monitor captures default path state. |
| `$id/flags` | read | Tagged reachability flags: DNS, IPv4, IPv6, expensive, constrained, interface classes, reason. |
| `$id/status` | read | Monitor state or error. |
| `$id/event` | read | Path-change events when callback mode is implemented. |

Fails closed: proxy preference writes, VPN/provider activation, local-network
permission bypass, `SCPreferencesCommitChanges`.

## image

`image` owns local image transforms over caller-provided bytes. It is separate
from Vision because it returns media payloads rather than analysis.

```text
macos/image
|-- filters
|-- clone
`-- $id/{ctl,in,out,status,meta,filter,format}
```

| File | Mode | Semantics |
| --- | --- | --- |
| `filters` | read | Supported filters such as `grayscale` and `sepia`. |
| `clone` | read | Allocates `$id`. |
| `$id/ctl` | write | `filter <name> [key=value...]`, `format png|jpeg|tiff`, `run`, `abort`, `destroy`. |
| `$id/in` | write | Complete input image bytes. |
| `$id/out` | read | Encoded output image bytes. |
| `$id/status` | read | `configured`, `done`, or `error`; includes output byte count. |
| `$id/meta` | read | Image dimensions/timing when available. |
| `$id/{filter,format}` | read/write | Current transform settings. |

Fails closed: arbitrary CoreImage graphs, plug-in loading, host path reads,
GPU resource export.

## document

`document` owns local document extraction over caller-provided bytes.

```text
macos/document
|-- formats
|-- clone
`-- $id/{ctl,in,out,status,meta,pages,format}
```

| File | Mode | Semantics |
| --- | --- | --- |
| `formats` | read | Supported transforms such as `pdf-text`. |
| `clone` | read | Allocates `$id`. |
| `$id/ctl` | write | `pdf-text`, `pages <range>`, `format text|json`, `run`, `abort`, `destroy`. |
| `$id/in` | write | Complete input document bytes. |
| `$id/out` | read | Text or JSON extraction result. |
| `$id/status` | read | `configured`, `done`, or `error`; includes page count when known. |
| `$id/meta` | read | JSON or tagged document metadata. |
| `$id/{pages,format}` | read/write | Current extraction settings. |

Fails closed: host path reads, network document loading, PDF mutation, forms,
signing, private document providers.

## vz

`vz` owns Virtualization.framework VM lifecycle. It is proc-shaped: clone a VM,
configure it, validate/start it, interact through status/console/report files,
then destroy it.

```text
macos/vz
|-- status
|-- clone
`-- $id/{ctl,config,disk,net,status,console,event,report}
```

| File | Mode | Semantics |
| --- | --- | --- |
| `status` | read | Framework support plus min/max CPU and memory limits after validation. |
| `clone` | read | Allocates `$id`. |
| `$id/ctl` | write | `validate`, `start`, `pause`, `resume`, `stop`, `kill`, `destroy`. |
| `$id/config` | read/write | JSON config: `cpus`, `memoryMiB`, `bootMode`, `disk`, `efiVars`, `serialLog`, `network`, `readOnly`, `appendSerial`. |
| `$id/disk` | read/write | Disk image path used if `config.disk` is empty. |
| `$id/net` | read/write | `nat` requests a NAT network attachment. |
| `$id/status` | read | `valid`, `invalid`, `starting`, VM state, `destroyed`, or `error`. |
| `$id/console` | read/write | Reserved console payload file. Current serial output is file-backed via `serialLog`. |
| `$id/event` | read | Lifecycle events when implemented. |
| `$id/report` | read | JSON validation/start report. |

Fails closed: entitlement-free use, arbitrary host filesystem passthrough, NBD,
bridged/private networking policy, interactive console input unless explicitly
designed, VM start without successful framework validation.

## Research and blocked services

`xpc` and `webkit` may appear as read-only research directories. They must not
grant launchd/bootstrap or browser-profile authority through this API without a
separate design.

`fskit`, `cloudkit`, `netext`, `endpointsecurity`, and `sm` are not exposed
capabilities. If mounted, they are read-only policy directories with
`status not-exposed` and `README`. Writes must fail permission denied.

## Examples

Set the host window title and show an alert:

```sh
echo 'Wanix Native' >macos/appkit/window/title
id=`{cat macos/appkit/alert/clone}
echo Confirm >macos/appkit/alert/$id/title
echo 'Continue?' >macos/appkit/alert/$id/message
echo 'Cancel,OK' >macos/appkit/alert/$id/buttons
echo show >macos/appkit/alert/$id/ctl
cat macos/appkit/alert/$id/result
```

Run OCR over caller-provided bytes:

```sh
v=`{cat macos/vision/clone}
cat page.png >macos/vision/$v/in
echo 'ocr accurate' >macos/vision/$v/ctl
echo 'format json' >macos/vision/$v/ctl
echo run >macos/vision/$v/ctl
cat macos/vision/$v/out
```

Capture one second of microphone PCM:

```sh
m=`{cat macos/mic/clone}
echo 'duration 1s' >macos/mic/$m/ctl
echo oneshot >macos/mic/$m/ctl
cat macos/mic/$m/data >mic.pcm
```

Validate a VZ configuration:

```sh
v=`{cat macos/vz/clone}
echo '{"cpus":2,"memoryMiB":2048,"bootMode":"efi"}' >macos/vz/$v/config
echo /Users/me/plan9.img >macos/vz/$v/disk
echo validate >macos/vz/$v/ctl
cat macos/vz/$v/report
```
