#!/bin/bash
# PaperValet bundle 启动脚本（POSIX）。
# 在解压后的 papervalet-$OS-$ARCH/ 目录里执行；自动定位 config.json。
#
# 用法：
#   ./run.sh                # 启动
#   ./run.sh --version      # 透传任意参数
#
# 优先级：
#   1. $PAPERVALET_HOME 环境变量（指向 bundle 根）
#   2. 当前脚本所在目录

set -e

# 解析 bundle 根：脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOME_DIR="${PAPERVALET_HOME:-$SCRIPT_DIR}"
BIN="$HOME_DIR/bin/papervalet"
CONFIG="$HOME_DIR/config.json"

# 缺 config.json 时从 example 复制一次
if [ ! -f "$CONFIG" ] && [ -f "$HOME_DIR/config.example.json" ]; then
    echo "⚠️  $CONFIG 不存在，已从 config.example.json 复制"
    echo "   请编辑后填入 api_id / api_hash 再启动"
    cp "$HOME_DIR/config.example.json" "$CONFIG"
    chmod 600 "$CONFIG"
fi

if [ ! -x "$BIN" ]; then
    echo "❌ 找不到可执行文件: $BIN" >&2
    exit 1
fi

# 把 HOME_DIR 暴露给子进程，session/db 落在 bundle 内便于打包带走
export PAPERVALET_HOME="$HOME_DIR"
cd "$HOME_DIR"

exec "$BIN" -config "$CONFIG" "$@"