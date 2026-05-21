# wanix-macapp

`wanix-macapp` runs Wanix in a native macOS `WKWebView` and exposes a small
Plan 9-style macOS namespace inside Wanix. The native service is available as
Wanix device `#macos` and is bound to `/mnt/macos` by the app bootstrap.
The default bootstrap starts `rc.wasm` in a terminal and renders it with
`<wanix-term>`.

The native host mounts:

```text
/mnt/macos/status
/mnt/macos/app/name
/mnt/macos/app/ctl
/mnt/macos/window/0/title
/mnt/macos/window/0/ctl
/mnt/macos/pasteboard/text
/mnt/macos/alert/title
/mnt/macos/alert/message
/mnt/macos/alert/style
/mnt/macos/alert/buttons
/mnt/macos/alert/show
/mnt/macos/alert/result
/mnt/macos/dialog/open/ctl
/mnt/macos/dialog/open/result
```

Each endpoint is a file in the Wanix namespace. Writes dispatch to AppKit on
the macOS main thread.

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

From Wanix, write the native namespace:

```sh
echo 'Wanix Native' >/mnt/macos/window/0/title
echo center >/mnt/macos/window/0/ctl
echo 'copied from Wanix' >/mnt/macos/pasteboard/text
echo 'Hello from rc' >/mnt/macos/alert/title
echo 'This alert was opened through #macos.' >/mnt/macos/alert/message
echo OK >/mnt/macos/alert/show
cat /mnt/macos/alert/result
cat /mnt/macos/status
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

`dialog/open` is reserved but not implemented yet.
