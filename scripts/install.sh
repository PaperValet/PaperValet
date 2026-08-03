#!/bin/bash
# PaperValet 一键安装脚本（macOS / Linux）—— 交互菜单 + 命令行双形态。
#
# ===== 菜单形态（默认） =====
#   curl -fsSL https://.../install.sh | bash
#   curl -fsSL https://.../install.sh | bash -s -- --menu
#
# 菜单项：
#   1) install      首次安装到 $HOME/.papervalet（默认）
#   2) reinstall    覆盖式重装（保留 config.json / session.json / sessions.db）
#   3) upgrade      升级到指定 version（保留全部数据）
#   4) uninstall    删除安装（提供保留 config / data 的子选项）
#   5) status       查看当前安装信息（路径 / 版本 / 二进制 SHA / .so 计数）
#   6) latest       列出 GitHub 最新 release 的版本与资产
#   7) set-phone    写入 PAPERVALET_PHONE 等登录环境变量到 config 与 ~/.profile
#   8) run          前台启动 bot（带 PATH 提示）
#   9) doctor       自检（curl / jq / 磁盘 / 架构兼容 / release 可达性）
#   0) exit         退出
#
# ===== 命令行形态 =====
#   --non-interactive   跳过菜单直接 install（CI 用）
#   --version <tag>     指定 release tag（默认 latest）
#   --home <dir>        安装目录（默认 $HOME/.papervalet）
#   --phone <+E164>     写入 PAPERVALET_PHONE（同时设置 ~/.profile 持久化）
#   --api-id <int>      写入 config.telegram.api_id
#   --api-hash <str>    写入 config.telegram.api_hash
#   --no-config         不生成 config.json
#   --keep-config       重装 / 升级时保留已有 config.json
#   --keep-data         重装 / 升级时保留已有 session.json / sessions.db
#   --repo <owner/repo> 指定仓库（默认 PaperValet/PaperValet）
#   --help              显示帮助
#
# 环境变量：
#   PAPERVALET_REPO          覆盖默认仓库
#   PAPERVALET_HOME          覆盖默认安装目录
#   PAPERVALET_VERSION       覆盖默认 version（latest / vX.Y.Z）
#   PAPERVALET_MENU=0        强制命令行形态（即使无参数）

set -euo pipefail

# ===== 默认值 =====
REPO="${PAPERVALET_REPO:-PaperValet/PaperValet}"
VERSION="${PAPERVALET_VERSION:-latest}"
HOME_DIR="${PAPERVALET_HOME:-$HOME/.papervalet}"
PHONE=""
API_ID=""
API_HASH=""
WRITE_CONFIG=1
KEEP_CONFIG=0
KEEP_DATA=0
USE_MENU="auto"      # auto | yes | no
NON_INTERACTIVE=0
ACTION="install"     # install | reinstall | upgrade | uninstall | status | latest | set-phone | run | doctor

# ===== 帮助 =====
usage() {
    sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'
    exit "${1:-0}"
}

# ===== 参数解析 =====
while [ $# -gt 0 ]; do
    case "$1" in
        --menu)            USE_MENU="yes"; shift ;;
        --no-menu)         USE_MENU="no"; shift ;;
        --non-interactive) USE_MENU="no"; NON_INTERACTIVE=1; shift ;;
        --action)          ACTION="$2"; USE_MENU="no"; shift 2 ;;
        --version)         VERSION="$2"; shift 2 ;;
        --home)            HOME_DIR="$2"; shift 2 ;;
        --phone)           PHONE="$2"; shift 2 ;;
        --api-id)          API_ID="$2"; shift 2 ;;
        --api-hash)        API_HASH="$2"; shift 2 ;;
        --no-config)       WRITE_CONFIG=0; shift ;;
        --keep-config)     KEEP_CONFIG=1; shift ;;
        --keep-data)       KEEP_DATA=1; shift ;;
        --repo)            REPO="$2"; shift 2 ;;
        -h|--help)         usage 0 ;;
        *) echo "未知参数: $1" >&2; usage 1 ;;
    esac
done

# 当通过 curl pipe 进来时 stdin 是 tty，但 PAPERVALET_MENU=0 或显式带参数 → 走 CLI
if [ "${PAPERVALET_MENU:-}" = "0" ]; then
    USE_MENU="no"
fi
if [ "$USE_MENU" = "auto" ]; then
    if [ -t 0 ] && [ -z "$PHONE$API_ID$API_HASH" ]; then
        USE_MENU="yes"
    else
        USE_MENU="no"
    fi
fi

# ===== 通用函数 =====
RED=$'\033[31m'; GREEN=$'\033[32m'; YELLOW=$'\033[33m'; BLUE=$'\033[34m'; BOLD=$'\033[1m'; RESET=$'\033[0m'
info()  { echo "${BLUE}ℹ${RESET}  $*"; }
ok()    { echo "${GREEN}✓${RESET}  $*"; }
warn()  { echo "${YELLOW}⚠${RESET}  $*"; }
fail()  { echo "${RED}❌${RESET} $*" >&2; }

detect_os_arch() {
    OS_RAW="$(uname -s | tr '[:upper:]' '[:lower:]')"
    case "$OS_RAW" in
        linux)  OS="linux" ;;
        darwin) OS="darwin" ;;
        *) fail "不支持的 OS: $OS_RAW（仅 macOS / Linux）"; echo "   Windows 用户请用 scripts/install.ps1" >&2; return 1 ;;
    esac
    ARCH_RAW="$(uname -m)"
    case "$ARCH_RAW" in
        x86_64|amd64)   ARCH="amd64" ;;
        aarch64|arm64)  ARCH="arm64" ;;
        *) fail "不支持的架构: $ARCH_RAW"; return 1 ;;
    esac
    if [ "$ARCH" = "arm64" ]; then
        warn "arm64：bundle 内 .so 为 amd64（README 已说明）"
        SO_ARCH="amd64"
    else
        SO_ARCH="$ARCH"
    fi
}

require_cmd() {
    for cmd in "$@"; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            fail "缺少依赖: $cmd"
            return 1
        fi
    done
}

# ===== Release 元数据 =====
fetch_release_json() {
    local version="$1"
    local api_url
    if [ "$version" = "latest" ]; then
        api_url="https://api.github.com/repos/$REPO/releases/latest"
    else
        api_url="https://api.github.com/repos/$REPO/releases/tags/$version"
    fi
    curl -fsSL "$api_url" || return 1
}

resolve_bundle_url() {
    local json="$1" name="$2"
    echo "$json" | grep '"browser_download_url"' \
        | grep -F "$name\"" \
        | head -n1 \
        | sed -E 's/.*"([^"]+)".*/\1/'
}

list_release_assets() {
    local json="$1"
    echo "$json" | grep '"name"' | sed -E 's/.*"name": "([^"]+)".*/\1/'
}

download_bundle() {
    local json="$1" os="$2" arch="$3" out_dir="$4"
    local ext="tar.gz"
    [ "$os" = "windows" ] && ext="zip"
    local bundle_name="papervalet-$os-$arch.$ext"
    local url
    url="$(resolve_bundle_url "$json" "$bundle_name")"
    if [ -z "$url" ]; then
        fail "release 中找不到 $bundle_name"
        info "可用资产:"
        list_release_assets "$json" | sed 's/^/  /'
        return 1
    fi
    local archive="$out_dir/$bundle_name"
    info "下载: $url"
    curl -fSL --progress-bar -o "$archive" "$url"
    echo "$archive"
}

extract_bundle() {
    local archive="$1" target="$2"
    mkdir -p "$target"
    case "$archive" in
        *.tar.gz) tar -xzf "$archive" -C "$target" --strip-components=1 ;;
        *.zip)    (cd "$target" && unzip -oq "$(basename "$archive")") ;;
        *) fail "未知归档: $archive"; return 1 ;;
    esac
}

write_config() {
    local cfg="$1"
    [ "$WRITE_CONFIG" = 1 ] || return 0
    local example="$HOME_DIR/config.example.json"
    if [ ! -f "$cfg" ] && [ -f "$example" ]; then
        cp "$example" "$cfg"
        chmod 600 "$cfg"
        ok "已写入默认 config: $cfg"
    fi
    if command -v jq >/dev/null 2>&1; then
        [ -n "$API_ID" ]   && jq --arg v "$API_ID"   '.telegram.api_id    = ($v|tonumber)' "$cfg" > "$cfg.tmp" && mv "$cfg.tmp" "$cfg"
        [ -n "$API_HASH" ] && jq --arg v "$API_HASH" '.telegram.api_hash  = $v'           "$cfg" > "$cfg.tmp" && mv "$cfg.tmp" "$cfg"
        [ -n "$PHONE" ]    && jq --arg v "$PHONE"    '.__phone = $v'                       "$cfg" > "$cfg.tmp" && mv "$cfg.tmp" "$cfg" || true
    else
        [ -n "$API_ID" ]   && warn "缺 jq，未写入 api_id"
        [ -n "$API_HASH" ] && warn "缺 jq，未写入 api_hash"
    fi
}

# ===== 动作：install / reinstall / upgrade =====
do_install() {
    detect_os_arch || return 1
    require_cmd curl tar || return 1

    local json
    json="$(fetch_release_json "$VERSION")" || { fail "获取 release 元数据失败"; return 1; }

    # 安装目录已存在 → 提示
    if [ -d "$HOME_DIR" ]; then
        warn "$HOME_DIR 已存在"
        if [ "$ACTION" = "install" ]; then
            if [ "$NON_INTERACTIVE" = 1 ]; then
                fail "已存在且 --non-interactive；改用 --action upgrade 或 --action reinstall"
                return 1
            fi
            local ans
            read -r -p "   是否覆盖? [y/N] " ans
            case "$ans" in y|Y|yes|YES) rm -rf "$HOME_DIR" ;; *) echo "已取消"; return 0 ;; esac
        elif [ "$ACTION" = "reinstall" ]; then
            backup_keep_then_clean
        elif [ "$ACTION" = "upgrade" ]; then
            backup_keep_then_clean
        fi
    fi

    local tmp; tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' RETURN

    local archive
    archive="$(download_bundle "$json" "$OS" "$ARCH" "$tmp")" || return 1

    mkdir -p "$HOME_DIR"
    info "解压到 $HOME_DIR"
    extract_bundle "$archive" "$HOME_DIR"

    # restore keep data
    [ -d "$tmp/keep/config.json" ] && cp "$tmp/keep/config.json" "$HOME_DIR/config.json" && ok "保留 config.json"
    [ -d "$tmp/keep/data" ] && cp -r "$tmp/keep/data" "$HOME_DIR/" && ok "保留 session / database"

    write_config "$HOME_DIR/config.json"
    set_phone_env_persist "$PHONE"

    print_finish_banner "installed"
}

backup_keep_then_clean() {
    local tmp_keep="$(mktemp -d)"
    [ "$KEEP_CONFIG" = 1 ] && [ -f "$HOME_DIR/config.json" ] && cp "$HOME_DIR/config.json" "$tmp_keep/config.json"
    [ "$KEEP_DATA" = 1 ] && {
        mkdir -p "$tmp_keep/data"
        for f in session.json sessions.db sessions.db-wal sessions.db-shm; do
            [ -f "$HOME_DIR/$f" ] && cp "$HOME_DIR/$f" "$tmp_keep/data/$f"
        done
    }
    # 隐式默认：reinstall 保留 config；upgrade 保留 config + data
    if [ "$ACTION" = "upgrade" ] && [ "$KEEP_DATA" = 0 ]; then KEEP_DATA=1; fi
    if [ "$KEEP_CONFIG" = 0 ]; then KEEP_CONFIG=1; fi

    mkdir -p "$tmp/keep"
    cp -r "$tmp_keep/." "$tmp/keep/" 2>/dev/null || true
    rm -rf "$HOME_DIR" "$tmp_keep"
}

# ===== 动作：uninstall =====
do_uninstall() {
    if [ ! -d "$HOME_DIR" ]; then
        warn "$HOME_DIR 不存在"
        return 0
    fi
    info "即将卸载 $HOME_DIR"
    local keep_cfg="N" keep_data="N"
    if [ "$NON_INTERACTIVE" = 0 ]; then
        read -r -p "   保留 config.json? [y/N] " keep_cfg
        read -r -p "   保留 session.json / sessions.db? [y/N] " keep_data
        read -r -p "   确认删除? 输入 YES 继续: " ans
        [ "$ans" = "YES" ] || { echo "已取消"; return 0; }
    else
        ans="YES"
    fi
    local bak=""
    if [[ "$keep_cfg" =~ ^[Yy]$ ]] && [ -f "$HOME_DIR/config.json" ]; then
        bak="$(mktemp -d)"; cp "$HOME_DIR/config.json" "$bak/config.json"
    fi
    if [[ "$keep_data" =~ ^[Yy]$ ]]; then
        bak="${bak:-$(mktemp -d)}"
        mkdir -p "$bak/data"
        for f in session.json sessions.db sessions.db-wal sessions.db-shm; do
            [ -f "$HOME_DIR/$f" ] && cp "$HOME_DIR/$f" "$bak/data/$f"
        done
    fi
    rm -rf "$HOME_DIR"
    ok "已卸载"
    if [ -n "${bak:-}" ] && [ -d "$bak" ]; then
        echo "   备份: $bak"
        echo "   恢复: cp -r $bak/* $HOME_DIR/"
    fi
}

# ===== 动作：status =====
do_status() {
    echo "${BOLD}PaperValet 安装状态${RESET}"
    echo "  安装目录: ${HOME_DIR}"
    if [ -d "$HOME_DIR" ]; then
        local bin="$HOME_DIR/bin/papervalet"
        if [ -x "$bin" ]; then
            local ver
            ver="$("$bin" --version 2>&1 | head -n1 || echo '?')"
            local sha
            sha="$(sha256sum "$bin" 2>/dev/null | awk '{print substr($1,1,12)}')"
            echo "  二进制:   $bin  ${GREEN}${ver}${RESET}  (sha256:${sha}...)"
        else
            warn "  二进制缺失或不可执行"
        fi
        local cfg="$HOME_DIR/config.json"
        [ -f "$cfg" ] && echo "  config:   $cfg (size $(stat -c%s "$cfg" 2>/dev/null || stat -f%z "$cfg") bytes)" \
            || warn "  config:   缺失"
        local so_count
        so_count="$(ls -1 "$HOME_DIR/plugins"/*.so 2>/dev/null | wc -l | tr -d ' ')"
        echo "  .so:      ${so_count} 个"
        for f in session.json sessions.db; do
            [ -f "$HOME_DIR/$f" ] && echo "  $f: 存在 ($(stat -c%s "$HOME_DIR/$f" 2>/dev/null || stat -f%z "$HOME_DIR/$f") bytes)"
        done
        local profile="$HOME/.papervalet.env"
        [ -f "$profile" ] && echo "  环境变量持久化: $profile"
    else
        warn "未安装"
    fi
}

# ===== 动作：latest =====
do_latest() {
    require_cmd curl || return 1
    local json
    json="$(fetch_release_json latest)" || { fail "获取 latest 失败"; return 1; }
    echo "${BOLD}最新 release${RESET}"
    echo "$json" | grep -E '"(tag_name|name|published_at|html_url)"' \
        | sed -E 's/^[[:space:]]*"([^"]+)":[[:space:]]*"([^"]+)".*/  \1: \2/'
    echo ""
    echo "可用 bundle:"
    list_release_assets "$json" | grep -E '^papervalet-' | sed 's/^/  /'
}

# ===== 动作：set-phone =====
do_set_phone() {
    if [ -z "$PHONE" ]; then
        read -r -p "   输入手机号（E.164，如 +8613800138000）: " PHONE
    fi
    [ -z "$PHONE" ] && { fail "未提供手机号"; return 1; }
    set_phone_env_persist "$PHONE"
    ok "PAPERVALET_PHONE 已设置"
    cat <<EOF

   后续手动启动：
     export PAPERVALET_PHONE='$PHONE'
     $HOME_DIR/run.sh
EOF
}

set_phone_env_persist() {
    local phone="$1"
    [ -z "$phone" ] && return 0
    local env_file="$HOME/.papervalet.env"
    {
        echo "# PaperValet login env (auto-generated by install.sh)"
        echo "export PAPERVALET_PHONE='$phone'"
        echo "export PAPERVALET_HOME='$HOME_DIR'"
        [ -n "${PAPERVALET_CODE:-}" ] && echo "export PAPERVALET_CODE='${PAPERVALET_CODE}'"
        [ -n "${PAPERVALET_2FA_PASSWORD:-}" ] && echo "export PAPERVALET_2FA_PASSWORD='${PAPERVALET_2FA_PASSWORD}'"
    } > "$env_file"
    ok "环境变量持久化到 $env_file（执行 'source $env_file' 生效）"
}

# ===== 动作：run =====
do_run() {
    local bin="$HOME_DIR/bin/papervalet"
    [ -x "$bin" ] || { fail "$bin 不可用；先 install"; return 1; }
    info "前台启动（Ctrl+C 退出）"
    [ -f "$HOME/.papervalet.env" ] && source "$HOME/.papervalet.env"
    cd "$HOME_DIR" && exec "$bin" -config "$HOME_DIR/config.json"
}

# ===== 动作：doctor =====
do_doctor() {
    echo "${BOLD}PaperValet 自检${RESET}"
    require_cmd curl tar jq 2>/dev/null && ok "依赖完整 (curl tar jq)" \
        || warn "缺 jq；config 写入部分字段会失败"
    df -h "$HOME" 2>/dev/null | awk 'NR==2 {print "  磁盘: 已用 "$3" / "$2" (剩余 "$4")"}'
    detect_os_arch && ok "OS=$OS ARCH=$ARCH SO_ARCH=$SO_ARCH"
    local json
    json="$(fetch_release_json latest 2>/dev/null)" && ok "GitHub API 可达" \
        || warn "GitHub API 不可达；离线环境无法安装/升级"
    [ -d "$HOME_DIR" ] && [ -x "$HOME_DIR/bin/papervalet" ] \
        && ok "本地安装: $HOME_DIR/bin/papervalet" \
        || warn "本地未安装或二进制缺失"
}

# ===== 收尾提示 =====
print_finish_banner() {
    local phase="${1:-installed}"
    local so_count=0
    [ -d "$HOME_DIR/plugins" ] && so_count="$(ls -1 "$HOME_DIR/plugins"/*.so 2>/dev/null | wc -l | tr -d ' ')"
    cat <<EOF

${GREEN}${BOLD}✅ PaperValet ${phase}${RESET} : $HOME_DIR
   ├── bin/papervalet
   ├── plugins/        (${so_count} 个 .so)
   ├── config.json     $([ -f "$HOME_DIR/config.json" ] && echo "✓" || echo "✗")
   └── run.sh

EOF
    if [ ! -f "$HOME_DIR/config.json" ]; then
        cat <<EOF
下一步：
   1) cp $HOME_DIR/config.example.json $HOME_DIR/config.json
   2) 编辑 config.json 填入 api_id / api_hash
   3) $HOME_DIR/run.sh
EOF
    elif [ -z "$API_ID" ] || [ -z "$API_HASH" ]; then
        cat <<EOF
下一步：
   编辑 $HOME_DIR/config.json 填入 api_id / api_hash，然后：
   $HOME_DIR/run.sh
EOF
    else
        cat <<EOF
下一步：
   直接启动: $HOME_DIR/run.sh
   或菜单选 8) run
EOF
    fi

    if [ -n "$PHONE" ]; then
        cat <<EOF

💡 已为你持久化 PAPERVALET_PHONE 到 ~/.papervalet.env
   立即生效: source ~/.papervalet.env
EOF
    fi
}

# ===== 菜单 =====
run_menu() {
    while true; do
        clear 2>/dev/null || true
        echo "${BOLD}PaperValet 安装器${RESET}  ($OS/$ARCH, repo=$REPO, version=$VERSION)"
        echo "  安装目录: $HOME_DIR"
        echo ""
        cat <<MENU
请选择操作：

  ${BOLD}1${RESET}) install       首次安装到 $HOME_DIR
  ${BOLD}2${RESET}) reinstall     覆盖重装（默认保留 config；可 --keep-data）
  ${BOLD}3${RESET}) upgrade       升级到 ${VERSION}（保留 config / session / db）
  ${BOLD}4${RESET}) uninstall     卸载（可选保留 config 与 data 的子菜单）
  ${BOLD}5${RESET}) status        查看当前安装信息
  ${BOLD}6${RESET}) latest        查看 GitHub 最新 release
  ${BOLD}7${RESET}) set-phone     设置 PAPERVALET_PHONE 等登录环境变量
  ${BOLD}8${RESET}) run           前台启动（PATH 已就绪）
  ${BOLD}9${RESET}) doctor        自检（依赖 / 磁盘 / 网络 / 架构）
  ${BOLD}0${RESET}) exit          退出

  ${BOLD}v${RESET}) 切换 version（当前: ${VERSION}）
  ${BOLD}h${RESET}) 切换 home dir（当前: ${HOME_DIR}）
MENU
        read -r -p "选择 [0-9vh]: " choice
        case "$choice" in
            1) ACTION="install";    do_install ;;
            2) ACTION="reinstall";  do_install ;;
            3) ACTION="upgrade";    do_install ;;
            4) ACTION="uninstall";  do_uninstall ;;
            5) ACTION="status";     do_status ;;
            6) ACTION="latest";     do_latest ;;
            7) ACTION="set-phone";  do_set_phone ;;
            8) ACTION="run";        do_run ;;
            9) ACTION="doctor";     do_doctor ;;
            0) echo "bye"; exit 0 ;;
            v|V) read -r -p "新 version（latest 或 vX.Y.Z）: " VERSION ;;
            h|H) read -r -p "新 home 目录: " HOME_DIR ;;
            *) warn "无效选择 '$choice'" ;;
        esac
        echo ""
        read -r -p "按 Enter 返回菜单，q 退出 ... " cont
        [ "$cont" = "q" ] && exit 0
    done
}

# ===== 入口 =====
if [ "$USE_MENU" = "yes" ] && [ "$NON_INTERACTIVE" = 0 ]; then
    detect_os_arch || true
    run_menu
else
    detect_os_arch || exit 1
    case "$ACTION" in
        install|reinstall|upgrade) do_install ;;
        uninstall) do_uninstall ;;
        status)    do_status ;;
        latest)    do_latest ;;
        set-phone) do_set_phone ;;
        run)       do_run ;;
        doctor)    do_doctor ;;
        *) fail "未知 action: $ACTION"; usage 1 ;;
    esac
fi