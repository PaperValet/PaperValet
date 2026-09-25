#!/bin/bash
# PaperValet 启动脚本 — 由安装包携带。
# 用法: ./run.sh   （Ctrl+C 退出）
set -e
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ -f "$HOME/.papervalet.env" ] && source "$HOME/.papervalet.env"
cd "$DIR"
exec "$DIR/bin/papervalet" -config "$DIR/config.json"
