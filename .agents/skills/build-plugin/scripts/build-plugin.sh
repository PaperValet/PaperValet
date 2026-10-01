#!/usr/bin/env bash
# Build, test and load-check one PaperValet external plugin.
#
#   build-plugin.sh <plugin-dir> [out-dir]
#
# <plugin-dir>  a directory with main.go + go.mod (e.g. PaperValet-Plugins/plugins-external/weather)
# out-dir       where <name>.so goes (default: ~/.cache/papervalet-plugins)
#
# Env:
#   PAPERVALET_SRC   PaperValet checkout (default: ../../../PaperValet from the plugin dir,
#                    then the repo this script lives in)
#   GO_VERSION       toolchain (default: read from PaperValet's CI workflow)
#   SKIP_TESTS=1     skip go test / go vet
#   SKIP_LOAD=1      skip the load check
set -euo pipefail

die() { echo "✗ $*" >&2; exit 1; }
say() { echo "▸ $*"; }

[ $# -ge 1 ] || die "usage: $0 <plugin-dir> [out-dir]"
plugin_dir=$(cd "$1" && pwd) || die "no such dir: $1"
[ -f "$plugin_dir/main.go" ] && [ -f "$plugin_dir/go.mod" ] || die "$plugin_dir needs main.go and go.mod"
name=$(basename "$plugin_dir")
out_dir=${2:-$HOME/.cache/papervalet-plugins}
mkdir -p "$out_dir"
out_dir=$(cd "$out_dir" && pwd)

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
src=${PAPERVALET_SRC:-}
if [ -z "$src" ]; then
  for c in "$plugin_dir/../../../PaperValet" "$script_dir/../../../.."; do
    if [ -f "$c/pkg/plugin/sdk.go" ]; then src=$c; break; fi
  done
fi
[ -n "$src" ] && [ -f "$src/pkg/plugin/sdk.go" ] || die "PaperValet source not found; set PAPERVALET_SRC"
src=$(cd "$src" && pwd)

if [ -z "${GO_VERSION:-}" ]; then
  GO_VERSION=$(sed -n "s/^ *GO_VERSION: *'\{0,1\}\([0-9.]*\)'\{0,1\}.*/\1/p" "$src/.github/workflows/ci.yml" | head -1)
fi
[ -n "$GO_VERSION" ] || die "cannot determine GO_VERSION"

command -v go >/dev/null || PATH=$PATH:/usr/local/go/bin
command -v go >/dev/null || die "go not found"
export GOTOOLCHAIN=go$GO_VERSION
export CGO_ENABLED=1
# Plugin builds need room: never let them land in a small tmpfs.
export GOTMPDIR=${GOTMPDIR:-$HOME/.cache/papervalet-gotmp}
mkdir -p "$GOTMPDIR"

say "$name · go$GO_VERSION · PaperValet at $src"
cd "$plugin_dir"

# Workspace, never a bare replace: through replace, -trimpath records
# PaperValet@v0.1.0/... and the bot rejects the .so ("different version of package").
if [ ! -f go.work ]; then
  go work init . "$src"
elif ! grep -q "$src" go.work; then
  go work use "$src"
fi

unformatted=$(gofmt -l .)
[ -z "$unformatted" ] || die "gofmt needed: $unformatted"

if [ "${SKIP_TESTS:-0}" != 1 ]; then
  say "vet + test"
  go vet ./...
  go test ./...
fi

say "build"
go build -trimpath -buildmode=plugin -o "$out_dir/$name.so" .

if [ "${SKIP_LOAD:-0}" != 1 ]; then
  say "load check against this PaperValet build"
  (cd "$src" && GOWORK=off PAPERVALET_PLUGIN_SO="$out_dir/$name.so" \
    go test -count=1 -trimpath -run '^TestLoadSO$' -v ./internal/plugin/loader) \
    | grep -E 'loaded|FAIL|panic|Error|error' || true
  (cd "$src" && GOWORK=off PAPERVALET_PLUGIN_SO="$out_dir/$name.so" \
    go test -count=1 -trimpath -run '^TestLoadSO$' ./internal/plugin/loader >/dev/null) \
    || die "load check failed"
fi

size=$(du -h "$out_dir/$name.so" | cut -f1)
echo "✓ $out_dir/$name.so ($size)"
