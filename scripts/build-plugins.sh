#!/bin/bash
# Build all external plugins as .so files.
#
# 用法：
#   ./scripts/build-plugins.sh                       # 相对路径自动推断
#   PAPERVALET_REPO_ROOT=/path/to/repo ./scripts/... # 显式指定仓库根
#   BUILD_PLUGINS_TARGET=linux-gnu ./scripts/...     # 指定目标三元组（预留）
#
# 行为：
#   1. 在每个 plugins-external/<name>/ 下执行 go build -buildmode=plugin
#   2. 跳过没有 go.mod 或 main 包的目录
#   3. 失败不中断，列出每个插件的成功/失败状态
#   4. 退出码：全部成功=0；至少一个失败=1

set -u  # 不开 -e：单个插件失败要继续走下一个

# 解析仓库根：优先环境变量；否则取脚本所在目录的父目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${PAPERVALET_REPO_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
PLUGINS_DIR="${PAPERVALET_PLUGINS_DIR:-$REPO_ROOT/plugins-external}"
BUILD_DIR="${PAPERVALET_BUILD_DIR:-$REPO_ROOT/build/plugins}"

if [ ! -d "$PLUGINS_DIR" ]; then
    echo "❌ plugins dir not found: $PLUGINS_DIR" >&2
    echo "   set PAPERVALET_REPO_ROOT to your PaperValet checkout" >&2
    exit 2
fi

mkdir -p "$BUILD_DIR"

echo "📦 Building external plugins"
echo "   repo root:  $REPO_ROOT"
echo "   plugins:    $PLUGINS_DIR"
echo "   output:     $BUILD_DIR"
echo ""

success=0
failed=0
skipped=0
failed_list=()

for plugin_dir in "$PLUGINS_DIR"/*/; do
    [ -d "$plugin_dir" ] || continue
    plugin=$(basename "$plugin_dir")

    # 跳过隐藏目录与模板目录
    case "$plugin" in
        _*|.|..) skipped=$((skipped+1)); continue ;;
    esac

    if [ ! -f "$plugin_dir/go.mod" ]; then
        echo "  ⚠ $plugin 缺 go.mod，跳过"
        skipped=$((skipped+1))
        continue
    fi

    # 必须有 package main（plugin 入口约定）
    if ! grep -qE '^package main' "$plugin_dir"/*.go 2>/dev/null; then
        echo "  ⚠ $plugin 非 package main，跳过"
        skipped=$((skipped+1))
        continue
    fi

    printf "  🔨 %-12s ... " "$plugin"
    out="$BUILD_DIR/$plugin.so"
    if (
        cd "$plugin_dir"
        go mod tidy >/dev/null 2>&1
        go build -buildmode=plugin -o "$out" .
    ); then
        echo "✓ $(stat -c%s "$out" 2>/dev/null || stat -f%z "$out") bytes"
        success=$((success+1))
    else
        echo "✗ FAILED"
        failed=$((failed+1))
        failed_list+=("$plugin")
    fi
done

echo ""
echo "📊 summary: ✓ $success · ✗ $failed · ⚠ $skipped skipped"
ls -la "$BUILD_DIR"/*.so 2>/dev/null || true

if [ $failed -gt 0 ]; then
    echo ""
    echo "❌ failed plugins: ${failed_list[*]}" >&2
    exit 1
fi
exit 0