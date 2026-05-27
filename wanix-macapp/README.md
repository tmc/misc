# wanix-macapp

`wanix-macapp` runs Wanix in a native macOS `WKWebView` and exposes a small
Plan 9-style macOS namespace inside Wanix. The native service is available as
Wanix device `#macos` and is bound to `macos` by the app bootstrap.
The default bootstrap starts `rc.wasm` in a terminal and renders it with
`<wanix-term>`.

The native host mounts:

```text
macos/status
macos/appkit/{app,window,pasteboard,alert,picker,indicator}
macos/notify
macos/touchid
macos/vision
macos/image
macos/document
macos/speech
macos/mic
macos/screen
macos/ax
macos/keychain
macos/reachability
macos/vz
```

Each endpoint is a file in the Wanix namespace. AppKit, notification,
authentication, capture, Accessibility, keychain, reachability, and
Virtualization operations are split into separate service directories so each
authority boundary stays visible to the guest. The app also binds `#macos` at
`mnt/macos` as a compatibility path, but new code should use `macos`. Legacy
`macos/app`, `macos/window`, `macos/pasteboard`, and `macos/alert` aliases
remain for the original AppKit surface.

## Run

Build Wanix browser assets first:

```sh
cd /Volumes/tmc/go/src/github.com/tmc/wanix-worktree-native-shell
make wasm-go
make js
make -C rc build
```

Run the native host:

```sh
cd /Users/tmc/go/src/github.com/tmc/misc/wanix-macapp
go run ./cmd/wanix-macapp host \
  -assets-dir /Volumes/tmc/go/src/github.com/tmc/wanix-worktree-native-shell/dist
```

If `rc.wasm` is not in the asset directory, `wanix-macapp` looks for it at
`../rc/rc.wasm` relative to the asset directory. Use `-rc-wasm` to pass a
different shell binary.

Run the native filesystem verifier:

```sh
go run ./cmd/wanix-macapp host \
  -assets-dir /Volumes/tmc/go/src/github.com/tmc/wanix-worktree-native-shell/dist \
  -self-test -visible=false -ready-timeout=60s
```

The default self-test avoids prompts and VM boot, but it still requires the
loaded Wanix assets to post the normal runtime-ready message. To include live
TCC-gated microphone capture, set `WANIX_MACAPP_SELFTEST_LIVE=1`. To also
validate a VZ disk config, set
`WANIX_MACAPP_SELFTEST_VZ_DISK=/path/to/disk.img`.

From Wanix, write the native namespace:

```sh
echo 'Wanix Native' >macos/window/title
echo center >macos/window/ctl
echo 'copied from Wanix' >macos/pasteboard/text
id=`{cat macos/alert/clone}
echo 'Hello from rc' >macos/alert/$id/title
echo 'This alert was opened through #macos.' >macos/alert/$id/message
echo OK >macos/alert/$id/buttons
echo show >macos/alert/$id/ctl
cat macos/alert/$id/result
cat macos/status
```

## Build A .app

```sh
go run ./cmd/wanix-macapp build \
  -name WanixNative \
  -assets-dir /Volumes/tmc/go/src/github.com/tmc/wanix-worktree-native-shell/dist \
  -out ./dist/WanixNative.app
```

Then:

```sh
open ./dist/WanixNative.app
```

## Scope

This is a small native proof. It keeps AppKit object lifetimes in the host and
does not expose raw Objective-C handles to Wanix. The durable guest contract is
the file namespace.
