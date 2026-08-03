#!/bin/bash
# PaperValet 一键安装脚本（macOS / Linux）。
#
# 用法：
#   curl -fsSL https://raw.githubusercontent.com/PaperValet/PaperValet/main/scripts/install.sh | bash
#   curl -fsSL .../install.sh | bash -s -- --version v0.2.0
#   curl -fsSL .../install.sh | bash -s -- --home /opt/papervalet
#   curl -fsSL .../install.sh | bash -s -- --phone +8613800138000
#
# 选项：
#   --version <tag>         指定 release tag（默认 latest）
#   --home <dir>            安装目录（默认 $HOME/.papervalet）
#   --phone <+E164>         写入 config 并设 PAPERVALET_PHONE 供非交互登录
#   --api-id <int>          写入 config.telegram.api_id
#   --api-hash <str>        写入 config.telegram.api_hash
#   --no-config             不生成 config.json（用户手动编辑）
#   --help                  显示帮助

set -euo pipefail

# ===== 参数解析 =====
REPO="PaperValet/PaperValet"
VERSION="latest"
HOME_DIR="${PAPERVALET_HOME:-$HOME/.papervalet}"
PHONE=""
API_ID=""
API_HASH=""
WRITE_CONFIG=1

usage() {
    sed -n '2,18p' "$0" | sed 's/^# \{0,1\}//'
    exit "${1:-0}"
}

while [ $# -gt 0 ]; do
    case "$1" in
        --version) VERSION="$2"; shift 2 ;;
        --home)    HOME_DIR="$2"; shift 2 ;;
        --phone)   PHONE="$2"; shift 2 ;;
        --api-id)  API_ID="$2"; shift 2 ;;
        --api-hash) API_HASH="$2"; shift 2 ;;
        --no-config) WRITE_CONFIG=0; shift ;;
        --repo)    REPO="$2"; shift 2 ;;
        -h|--help) usage 0 ;;
        *) echo "未知参数: $1" >&2; usage 1 ;;
    esac
done

# ===== 架构探测 =====
OS_RAW="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS_RAW" in
    linux)  OS="linux" ;;
    darwin) OS="darwin" ;;
    *) echo "❌ 不支持的操作系统: $OS_RAW（仅支持 macOS / Linux）" >&2
       echo "   Windows 用户请用 scripts/install.ps1" >&2
       exit 1 ;;
esac

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
    x86_64|amd64)   ARCH="amd64" ;;
    aarch64|arm64)  ARCH="arm64" ;;
    *) echo "❌ 不支持的架构: $ARCH_RAW" >&2; exit 1 ;;
esac

# 注意：CI 当前只在 amd64 编 .so；arm64 用户会拿到 linux/amd64 .so 集合。
# 见 README 备注。
if [ "$ARCH" = "arm64" ]; then
    SO_ARCH="amd64"
    echo "⚠️  arm64 检测到，bundle 里 .so 为 amd64（见 README 备注）"
else
    SO_ARCH="$ARCH"
fi

# ===== 工具检查 =====
for cmd in curl tar; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "❌ 缺少依赖: $cmd" >&2
        exit 1
    fi
done

# ===== 解析 release 元数据 =====
echo "📡 解析 $REPO @ $VERSION ..."
if [ "$VERSION" = "latest" ]; then
    API_URL="https://api.github.com/repos/$REPO/releases/latest"
else
    API_URL="https://api.github.com/repos/$REPO/releases/tags/$VERSION"
fi

RELEASE_JSON="$(curl -fsSL "$API_URL")" || {
    echo "❌ 获取 release 信息失败: $API_URL" >&2
    echo "   检查 --version / --repo 参数或网络" >&2
    exit 1
}

# 选 bundle 名：papervalet-$OS-$ARCH.tar.gz
BUNDLE_NAME="papervalet-$OS-$ARCH.tar.gz"
BUNDLE_URL="$(echo "$RELEASE_JSON" \
    | grep '"browser_download_url"' \
    | grep -F "$BUNDLE_NAME\"" \
    | head -n1 \
    | sed -E 's/.*"([^"]+)".*/\1/')"

if [ -z "$BUNDLE_URL" ]; then
    echo "❌ 在 release 中找不到 $BUNDLE_NAME" >&2
    echo "   可用的资产：" >&2
    echo "$RELEASE_JSON" | grep '"name"' | sed 's/^/     /' >&2
    exit 1
fi

echo "   ✓ 选定 bundle: $BUNDLE_NAME"

# ===== 下载与解压 =====
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
ARCHIVE="$TMP/$BUNDLE_NAME"

echo "⬇️  下载中: $BUNDLE_URL"
curl -fSL --progress-bar -o "$ARCHIVE" "$BUNDLE_URL" || {
    echo "❌ 下载失败" >&2; exit 1
}

# 安装目录若已存在，提示确认
if [ -d "$HOME_DIR" ]; then
    echo "⚠️  目标目录已存在: $HOME_DIR"
    read -r -p "   是否覆盖? [y/N] " ans
    case "$ans" in
        y|Y|yes|YES) rm -rf "$HOME_DIR" ;;
        *) echo "已取消"; exit 0 ;;
    esac
fi

mkdir -p "$HOME_DIR"
echo "📦 解压到 $HOME_DIR ..."
tar -xzf "$ARCHIVE" -C "$HOME_DIR" --strip-components=1

# ===== config.json 处理 =====
if [ "$WRITE_CONFIG" = 1 ]; then
    CONFIG="$HOME_DIR/config.json"
    if [ ! -f "$CONFIG" ] && [ -f "$HOME_DIR/config.example.json" ]; then
        cp "$HOME_DIR/config.example.json" "$CONFIG"
        chmod 600 "$CONFIG"
        echo "✏️  已写入默认 config.json: $CONFIG"
    fi

    # 用 jq 改值（若用户提供了参数）
    if command -v jq >/dev/null 2>&1; then
        [ -n "$API_ID" ]   && jq --arg v "$API_ID"   '.telegram.api_id    = ($v|tonumber)' "$CONFIG" > "$CONFIG.tmp" && mv "$CONFIG.tmp" "$CONFIG"
        [ -n "$API_HASH" ] && jq --arg v "$API_HASH" '.telegram.api_hash  = $v'           "$CONFIG" > "$CONFIG.tmp" && mv "$CONFIG.tmp" "$CONFIG"
    else
        [ -n "$API_ID" ]   && echo "⚠️  缺 jq，无法写入 api_id" >&2
        [ -n "$API_HASH" ] && echo "⚠️  缺 jq，无法写入 api_hash" >&2
    fi
fi

# ===== 完成提示 =====
cat <<EOF

✅ PaperValet 已安装到: $HOME_DIR
   ├── bin/papervalet
   ├── plugins/        (含 $(ls "$HOME_DIR/plugins" 2>/dev/null | wc -l | tr -d ' ') 个 .so)
   ├── config.json     $([ -f "$HOME_DIR/config.json" ] && echo "✓" || echo "✗")
   └── run.sh

下一步：
EOF

if [ ! -f "$HOME_DIR/config.json" ]; then
    cat <<EOF
   1) cp $HOME_DIR/config.example.json $HOME_DIR/config.json
   2) 编辑 config.json 填入 api_id / api_hash（从 https://my.telegram.org 申请）
   3) $HOME_DIR/run.sh
EOF
elif [ -z "$API_ID" ] || [ -z "$API_HASH" ]; then
    cat <<EOF
   1) 编辑 $HOME_DIR/config.json 填入 api_id / api_hash
   2) $HOME_DIR/run.sh
EOF
else
    cat <<EOF
   • 直接启动: $HOME_DIR/run.sh
EOF
fi

if [ -n "$PHONE" ]; then
    cat <<EOF
   • 你传入了 --phone=$PHONE，请设置环境变量再启动：

         export PAPERVALET_PHONE='$PHONE'
         $HOME_DIR/run.sh

     非首次登录（已有 session）会自动跳过；首次登录会向 Telegram 发验证码，
     需要时仍会在终端提示 "Login code:"。
EOF
fi

echo ""
echo "💡 升级到更新版本：重新跑本脚本即可。"