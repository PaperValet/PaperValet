#!/bin/bash
# Build all external plugins as .so files.
#
# Environment variables:
#   PAPERVALET_BUILD_DIR  - output directory for .so files (required)
#   GOOS / GOARCH         - cross-compilation targets (optional)
#
# The script auto-discovers every immediate subdirectory of `plugins-external/`
# that contains a `main.go`, so new plugins are picked up without editing this file.
#
# 构建通过 go.work（仓库根目录）走 workspace 模式，避免 replace ../../../.. 在
# PR 合成 merge ref 下路径解析失败的问题。

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PLUGINS_DIR="$ROOT_DIR/plugins-external"

# workspace 模式：go.work 位于仓库根目录，包含主模块与所有插件
export GOWORK="${ROOT_DIR}/go.work"

if [ -z "${PAPERVALET_BUILD_DIR:-}" ]; then
    echo "❌ PAPERVALET_BUILD_DIR is not set" >&2
    exit 1
fi

mkdir -p "$PAPERVALET_BUILD_DIR"

if [ ! -d "$PLUGINS_DIR" ]; then
    echo "⚠ plugins-external directory not found at $PLUGINS_DIR, skipping"
    exit 0
fi

echo "Building external plugins..."
echo "  root:   $ROOT_DIR"
echo "  output: $PAPERVALET_BUILD_DIR"
echo "  gowork: $GOWORK"
echo "  goos:   ${GOOS:-<host>}  goarch: ${GOARCH:-<host>}"
echo ""

FAILED=()
BUILT=0

for plugin_path in "$PLUGINS_DIR"/*/; do
    [ -d "$plugin_path" ] || continue
    plugin="$(basename "$plugin_path")"

    # 只构建包含 main.go 目录（真正的可构建插件）
    if [ ! -f "$plugin_path/main.go" ]; then
        echo "⚠ $plugin: no main.go, skipping"
        continue
    fi

    echo "🔨 $plugin ..."
    (
        cd "$plugin_path"
        # tidy 在 workspace 模式下不需要（go.work 已提供映射），跳过避免污染
        if go build -buildmode=plugin -o "$PAPERVALET_BUILD_DIR/$plugin.so" . ; then
            echo "  ✓ $plugin.so"
        else
            echo "  ✗ $plugin FAILED"
            exit 1
        fi
    ) && BUILT=$((BUILT+1)) || FAILED+=("$plugin")
done

echo ""
echo "Built $BUILT plugin(s) in $PAPERVALET_BUILD_DIR"
ls -la "$PAPERVALET_BUILD_DIR"/*.so 2>/dev/null || true

if [ ${#FAILED[@]} -gt 0 ]; then
    echo ""
    echo "❌ failed plugins: ${FAILED[*]}" >&2
    exit 1
fi
