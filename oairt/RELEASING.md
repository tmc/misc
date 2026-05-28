# Releasing oairt

Manual tag-based release flow. No `goreleaser`. Binary artifacts are not
published in `v0.1.0` — users `go install` the CLI.

The agent does not push tags; the maintainer runs the push step.

## 1. Pre-flight

From a clean `master`:

    git status                          # clean
    go test ./...                       # green
    go vet ./...                        # clean
    golangci-lint run                   # clean (when configured)
    go doc github.com/tmc/misc/oairt    # renders without warnings

Promote `[Unreleased]` in `CHANGELOG.md` to a dated `[X.Y.Z]` section.
Commit the CHANGELOG bump.

## 2. Tag

Module tag (subdirectory module convention under `tmc/misc`):

    git tag oairt/vX.Y.Z -m "oairt vX.Y.Z"
    git push origin oairt/vX.Y.Z

The exact tag path (`oairt/vX.Y.Z` vs `vX.Y.Z`) depends on the final module
layout — confirm with `go list -m` before tagging.

## 3. Verify

From a scratch `GOPATH` (or a temp dir with its own module cache):

    go install github.com/tmc/misc/oairt/cmd/oairt@vX.Y.Z

Prime `pkg.go.dev`:

    curl -sSf https://proxy.golang.org/github.com/tmc/misc/oairt/@v/vX.Y.Z.info

The page typically appears within 30 minutes.

## 4. Post-release

Reopen `[Unreleased]` in `CHANGELOG.md`. Announce as appropriate.

## Binary artifacts

Out of scope for `v0.1.0`. A future minor may add prebuilt binaries; the
manual flow here is intentionally minimal.
