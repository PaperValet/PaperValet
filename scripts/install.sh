#!/bin/bash
# PaperValet installer for Linux / macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/master/scripts/install.sh | bash
#
# It only installs the binary and registers a shell command. Everything else
# (language, API credentials, login, background service) is done by
#   <command> initialize
#
# Options:
#   --install | --upgrade | --uninstall   skip the menu
#   --name <cmd>       command name (default papervalet; one name = one instance)
#   --version <tag>    release tag (default latest)
#   --repo <owner/repo>
#   -y, --yes          accept defaults, never prompt
#
# Env: PAPERVALET_REPO, PAPERVALET_VERSION, PAPERVALET_INSTALL_URL,
#      PAPERVALET_BUNDLE (use a local bundle archive instead of downloading)

# Piped runs (curl | bash) read the script from stdin, so prompts would eat
# the script itself. Re-download to a temp file and re-exec on the terminal.
# Only when the script itself arrives on stdin (BASH_SOURCE is empty then);
# `bash install.sh` never re-execs, and PAPERVALET_REEXEC stops any loop.
if [ -z "${BASH_SOURCE[0]:-}" ] && [ -z "${PAPERVALET_REEXEC:-}" ]; then
    _self_url="${PAPERVALET_INSTALL_URL:-https://raw.githubusercontent.com/PaperValet/PaperValet/master/scripts/install.sh}"
    _self_tmp="$(mktemp /tmp/papervalet-install.XXXXXX.sh)"
    if command -v curl >/dev/null 2>&1 && curl -fsSL "$_self_url" -o "$_self_tmp" 2>/dev/null && [ -s "$_self_tmp" ]; then
        export PAPERVALET_REEXEC=1
        if (exec 3</dev/tty) 2>/dev/null; then
            exec bash "$_self_tmp" "$@" </dev/tty
        fi
        exec bash "$_self_tmp" "$@" </dev/null
    fi
    rm -f "$_self_tmp"
    echo "✗ Failed to download the installer | 下载安装脚本失败" >&2
    exit 1
fi
# bash keeps the script open, so the re-exec temp copy can go right away.
case "$0" in /tmp/papervalet-install.*.sh) rm -f "$0" ;; esac

set -eo pipefail

REPO="${PAPERVALET_REPO:-PaperValet/PaperValet}"
VERSION="${PAPERVALET_VERSION:-latest}"
ACTION=""
NAME=""
YES=0
MARKER="# papervalet-wrapper"

while [ $# -gt 0 ]; do
    case "$1" in
        --install)   ACTION=install; shift ;;
        --upgrade)   ACTION=upgrade; shift ;;
        --uninstall) ACTION=uninstall; shift ;;
        --name)      NAME="${2:-}"; shift 2 ;;
        --version)   VERSION="${2:-}"; shift 2 ;;
        --repo)      REPO="${2:-}"; shift 2 ;;
        -y|--yes|--non-interactive) YES=1; shift ;;
        -h|--help)   sed -n '2,19p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        *) echo "✗ Unknown option | 未知参数: $1" >&2; exit 2 ;;
    esac
done
[ -t 0 ] || YES=1

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    B=$'\033[1m'; D=$'\033[2m'; R=$'\033[31m'; G=$'\033[32m'; Y=$'\033[33m'; C=$'\033[36m'; N=$'\033[0m'
else
    B=; D=; R=; G=; Y=; C=; N=
fi
ok()   { echo "  ${G}✓${N} $*"; }
warn() { echo "  ${Y}!${N} $*"; }
err()  { echo "  ${R}✗${N} $*" >&2; }
hint() { echo "  ${D}$*${N}"; }
die()  { err "$*"; exit 1; }

# ask <label> <default> → $REPLY
ask() {
    if [ "$YES" = 1 ]; then REPLY="$2"; return 0; fi
    local shown=""
    [ -n "$2" ] && shown=" ${D}[$2]${N}"
    read -r -p "  ${C}›${N} $1$shown: " REPLY || REPLY=""
    [ -n "$REPLY" ] || REPLY="$2"
}

# confirm <label> <y|n> → exit status
confirm() {
    local def="$2" hint_yn="[y/N]" ans
    [ "$def" = y ] && hint_yn="[Y/n]"
    if [ "$YES" = 1 ]; then [ "$def" = y ]; return; fi
    read -r -p "  ${C}›${N} $1 ${D}$hint_yn${N} " ans || ans=""
    [ -n "$ans" ] || ans="$def"
    case "$ans" in y|Y|yes|YES|是) return 0 ;; *) return 1 ;; esac
}

# ===== Platform =====
case "$(uname -s)" in
    Linux)  OS=linux ;;
    Darwin) OS=darwin ;;
    *) die "Only Linux / macOS; Windows uses scripts/install.ps1 | 仅支持 Linux / macOS，Windows 请用 scripts/install.ps1" ;;
esac
case "$(uname -m)" in
    x86_64|amd64)  ARCH=amd64 ;;
    aarch64|arm64) ARCH=arm64 ;;
    *) die "Unsupported CPU | 不支持的架构: $(uname -m)" ;;
esac

if [ "$(id -u)" = 0 ]; then
    BIN_DIR=/usr/local/bin
else
    BIN_DIR="$HOME/.local/bin"
fi

valid_name() { [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$ ]]; }

# Default name keeps the classic ~/.papervalet; other names get ~/.<name>.
home_for() {
    if [ "$1" = papervalet ]; then echo "$HOME/.papervalet"; else echo "$HOME/.$1"; fi
}

wrapper_path() { echo "$BIN_DIR/$1"; }
is_ours() { [ -f "$1" ] && grep -qF "$MARKER" "$1" 2>/dev/null; }

# Read PAPERVALET_HOME back from an installed wrapper.
wrapper_home() {
    sed -n 's/^export PAPERVALET_HOME="\(.*\)"$/\1/p' "$1" | head -n1
}

pick_name() {
    local purpose="$1"
    while :; do
        ask "Command name | 命令名" "${NAME:-papervalet}"
        NAME="$REPLY"
        if ! valid_name "$NAME"; then
            err "Letters, digits, - and _ only | 只能用字母、数字、- 和 _"
            [ "$YES" = 1 ] && exit 1
            NAME=""; continue
        fi
        WRAPPER="$(wrapper_path "$NAME")"
        if [ "$purpose" = install ]; then
            local other
            other="$(command -v "$NAME" 2>/dev/null || true)"
            if [ -n "$other" ] && [ "$other" != "$WRAPPER" ] && ! is_ours "$other"; then
                err "$NAME is taken by $other | $NAME 已被 $other 占用"
                [ "$YES" = 1 ] && exit 1
                NAME=""; continue
            fi
            HOME_DIR="$(home_for "$NAME")"
        else
            if ! is_ours "$WRAPPER"; then
                err "No PaperValet command named $NAME | 没有叫 $NAME 的 PaperValet 命令"
                [ "$YES" = 1 ] && exit 1
                NAME=""; continue
            fi
            HOME_DIR="$(wrapper_home "$WRAPPER")"
            [ -n "$HOME_DIR" ] || HOME_DIR="$(home_for "$NAME")"
        fi
        return 0
    done
}

# ===== Download =====
fetch_bundle() {
    local out="$1"
    if [ -n "${PAPERVALET_BUNDLE:-}" ]; then
        [ -f "$PAPERVALET_BUNDLE" ] || die "PAPERVALET_BUNDLE not found: $PAPERVALET_BUNDLE"
        cp "$PAPERVALET_BUNDLE" "$out"
        return 0
    fi
    command -v curl >/dev/null 2>&1 || die "curl is required | 需要 curl"
    local name="papervalet-$OS-$ARCH.tar.gz" url
    if [ "$VERSION" = latest ]; then
        url="https://github.com/$REPO/releases/latest/download/$name"
    else
        url="https://github.com/$REPO/releases/download/$VERSION/$name"
    fi
    hint "↓ $url"
    curl -fSL --progress-bar -o "$out" "$url" || die "Download failed | 下载失败"
}

# Unpack the bundle and copy bin + bundled plugins into the data home.
deploy() {
    local tmp
    tmp="$(mktemp -d)"
    fetch_bundle "$tmp/bundle.tar.gz"
    tar -xzf "$tmp/bundle.tar.gz" -C "$tmp" --strip-components=1 || { rm -rf "$tmp"; die "Bad archive | 安装包损坏"; }
    [ -f "$tmp/bin/papervalet" ] || { rm -rf "$tmp"; die "Archive has no bin/papervalet | 安装包缺少 bin/papervalet"; }
    mkdir -p "$HOME_DIR/bin" "$HOME_DIR/plugins"
    chmod 700 "$HOME_DIR"
    # Replace via rename so a running service keeps its old inode.
    cp "$tmp/bin/papervalet" "$HOME_DIR/bin/.papervalet.new"
    chmod 755 "$HOME_DIR/bin/.papervalet.new"
    mv -f "$HOME_DIR/bin/.papervalet.new" "$HOME_DIR/bin/papervalet"
    if [ -d "$tmp/plugins" ]; then
        find "$tmp/plugins" -maxdepth 1 -type f -name '*.so' -exec cp -f {} "$HOME_DIR/plugins/" \;
    fi
    rm -rf "$tmp"
    "$HOME_DIR/bin/papervalet" version >/dev/null 2>&1 || die "The binary does not run on this machine | 二进制无法在本机运行"
}

write_wrapper() {
    mkdir -p "$BIN_DIR"
    cat >"$WRAPPER" <<EOF
#!/bin/sh
$MARKER
export PAPERVALET_HOME="$HOME_DIR"
export PAPERVALET_CMD="$NAME"
exec "\$PAPERVALET_HOME/bin/papervalet" "\$@"
EOF
    chmod 755 "$WRAPPER"
}

# Non-root: make sure ~/.local/bin is on PATH for future shells.
ensure_path() {
    [ "$BIN_DIR" = /usr/local/bin ] && return 0
    case ":$PATH:" in *":$BIN_DIR:"*) return 0 ;; esac
    local line='export PATH="$HOME/.local/bin:$PATH"' rc
    for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
        if [ "$rc" = "$HOME/.zshrc" ] && [ ! -f "$rc" ] && [ "${SHELL##*/}" != zsh ]; then
            continue
        fi
        grep -qF "$line" "$rc" 2>/dev/null || printf '\n# PaperValet\n%s\n' "$line" >>"$rc"
    done
    PATH_ADDED=1
}

# Unit files of services that run this data home.
find_units() {
    local d
    for d in /etc/systemd/system "$HOME/.config/systemd/user"; do
        [ -d "$d" ] || continue
        grep -lxF "Environment=PAPERVALET_HOME=$HOME_DIR" "$d"/*.service 2>/dev/null || true
    done
}

systemctl_for() {
    case "$1" in
        "$HOME"/.config/systemd/user/*) systemctl --user "${@:2}" ;;
        *) systemctl "${@:2}" ;;
    esac
}

# ===== Actions =====
title() {
    [ -n "${MENU_SHOWN:-}" ] && return 0
    echo
    echo "  ${B}${C}PaperValet${N} ${D}$1 · $OS/$ARCH${N}"
    echo
}

do_install() {
    title "install | 安装"
    pick_name install
    if [ -x "$HOME_DIR/bin/papervalet" ] && is_ours "$WRAPPER"; then
        warn "$NAME is already installed | $NAME 已经装过了"
        confirm "Upgrade it instead? | 改为升级？" y || exit 0
        do_upgrade_named
        return
    fi
    deploy
    write_wrapper
    ensure_path
    echo
    ok "Installed | 已安装  ${B}$NAME${N}  ${D}→ $HOME_DIR${N}"
    local cmd="$NAME"
    if [ -n "${PATH_ADDED:-}" ]; then
        cmd="$WRAPPER"
        hint "Open a new terminal for $NAME to be on PATH | 新开终端后 $NAME 命令即可用"
    fi
    echo
    if [ "$YES" = 0 ] && confirm "Run setup now? | 现在开始初始化？" y; then
        exec "$WRAPPER" initialize
    fi
    echo "  Next | 下一步  ${C}$cmd initialize${N}"
    echo
}

do_upgrade_named() {
    local units old new
    old="$("$HOME_DIR/bin/papervalet" version 2>/dev/null | head -n1 || true)"
    deploy
    write_wrapper
    new="$("$HOME_DIR/bin/papervalet" version 2>/dev/null | head -n1 || true)"
    ok "Upgraded | 已升级  ${D}${old:-?} → ${new:-?}${N}"
    units="$(find_units)"
    local u
    for u in $units; do
        if systemctl_for "$u" try-restart "$(basename "$u")" 2>/dev/null; then
            ok "Restarted service | 已重启服务  $(basename "$u" .service)"
        fi
    done
    echo
}

do_upgrade() {
    title "upgrade | 升级"
    pick_name upgrade
    do_upgrade_named
}

do_uninstall() {
    title "uninstall | 卸载"
    pick_name upgrade
    local units u
    units="$(find_units)"
    for u in $units; do
        local svc
        svc="$(basename "$u")"
        systemctl_for "$u" disable --now "$svc" >/dev/null 2>&1 || true
        rm -f "$u"
        systemctl_for "$u" daemon-reload >/dev/null 2>&1 || true
        ok "Removed service | 已移除服务  ${svc%.service}"
    done
    rm -f "$WRAPPER"
    ok "Removed command | 已移除命令  $NAME"
    rm -f "$HOME_DIR/bin/papervalet"
    rmdir "$HOME_DIR/bin" 2>/dev/null || true
    if [ -d "$HOME_DIR" ]; then
        warn "Data holds your login session and settings | 数据目录里有登录会话和配置"
        if confirm "Delete $HOME_DIR as well? | 同时删除 $HOME_DIR？" n; then
            rm -rf "$HOME_DIR"
            ok "Deleted | 已删除  $HOME_DIR"
        else
            hint "Kept | 已保留  $HOME_DIR"
        fi
    fi
    echo
}

menu() {
    title "installer | 安装器"
    echo "    ${B}1${N}) Install   | 安装"
    echo "    ${B}2${N}) Upgrade   | 升级"
    echo "    ${B}3${N}) Uninstall | 卸载"
    echo
    MENU_SHOWN=1
    while :; do
        ask "Choose | 选择" 1
        case "$REPLY" in
            1) ACTION=install; return ;;
            2) ACTION=upgrade; return ;;
            3) ACTION=uninstall; return ;;
            *) err "Enter 1, 2 or 3 | 请输入 1、2 或 3" ;;
        esac
    done
}

[ -n "$ACTION" ] || { if [ "$YES" = 1 ]; then ACTION=install; else menu; fi; }
case "$ACTION" in
    install)   do_install ;;
    upgrade)   do_upgrade ;;
    uninstall) do_uninstall ;;
esac
