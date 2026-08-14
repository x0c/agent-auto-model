#!/usr/bin/env bash
# 一键安装 agent-auto-model（macOS / Linux）。
# 用法：curl -fsSL https://raw.githubusercontent.com/x0c/cursor-mode-model/main/install.sh | bash
set -euo pipefail

REPO="${CMM_REPO:-x0c/cursor-mode-model}"
PREFIX="${CMM_PREFIX:-$HOME/.local}"
BIN_DIR="${PREFIX}/bin"

die() { echo "错误：$*" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || die "未找到 curl"
command -v tar >/dev/null 2>&1 || die "未找到 tar"

VERSION="${CMM_VERSION:-}"
if [ -z "$VERSION" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
fi
[ -n "$VERSION" ] || die "无法解析最新版本号"
VERSION_NUMBER="${VERSION#v}"

MACHINE=$(uname -m)
case "$MACHINE" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) die "暂不支持处理器架构 ${MACHINE}" ;;
esac

SYSTEM=$(uname -s)
case "$SYSTEM" in
  Darwin) OS="darwin" ;;
  Linux) OS="linux" ;;
  *) die "此脚本仅支持 macOS 与 Linux；Windows 请用 install.ps1" ;;
esac

ASSET="agent-auto-model_${VERSION_NUMBER}_${OS}_${ARCH}.tar.gz"
LEGACY_ASSET="cursor-mode-model_${VERSION_NUMBER}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
LEGACY_URL="https://github.com/${REPO}/releases/download/${VERSION}/${LEGACY_ASSET}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "正在安装 agent-auto-model ${VERSION} ..."
if curl -fsSL "$URL" -o "$TMP/$ASSET"; then
  :
elif curl -fsSL "$LEGACY_URL" -o "$TMP/$ASSET"; then
  echo "回退到旧资产名 ${LEGACY_ASSET}"
else
  die "下载失败：${URL}
若 Release 尚无预编译包，可改用：
  go install github.com/x0c/cursor-mode-model/cmd/agent-auto-model@${VERSION}
  agent-auto-model install"
fi

tar -xzf "$TMP/$ASSET" -C "$TMP"
mkdir -p "$BIN_DIR"

MAIN=""
for cand in agent-auto-model cursor-mode-model; do
  if [ -f "$TMP/$cand" ]; then
    install -m 755 "$TMP/$cand" "$BIN_DIR/$cand"
    if [ -z "$MAIN" ]; then
      MAIN="$BIN_DIR/$cand"
    fi
  fi
done
[ -n "$MAIN" ] || die "压缩包里没有 agent-auto-model / cursor-mode-model 二进制"
if [ -x "$BIN_DIR/agent-auto-model" ]; then
  MAIN="$BIN_DIR/agent-auto-model"
fi
if [ ! -e "$BIN_DIR/cursor-mode-model" ] && [ -x "$BIN_DIR/agent-auto-model" ]; then
  ln -s agent-auto-model "$BIN_DIR/cursor-mode-model"
fi
if [ ! -e "$BIN_DIR/agent-auto-model" ] && [ -x "$BIN_DIR/cursor-mode-model" ]; then
  ln -s cursor-mode-model "$BIN_DIR/agent-auto-model"
  MAIN="$BIN_DIR/agent-auto-model"
fi

export PATH="${BIN_DIR}:$PATH"
"$MAIN" install || die "二进制已装好，但 hooks 安装失败；请手动运行：agent-auto-model install"

case ":${PATH}:" in
  *":${BIN_DIR}:"*)
    echo "安装完成。运行 agent-auto-model status 验证。"
    ;;
  *)
    echo ""
    echo "安装完成，但 ${BIN_DIR} 不在 PATH 中。"
    echo "请把下面这行加入 shell 配置后重开终端："
    echo "  export PATH=\"${BIN_DIR}:\$PATH\""
    ;;
esac

echo "前置条件：Cursor Agent CLI（https://cursor.com/install）和/或 Codex CLI。"
echo "cursor-mode-model 仍可作为兼容别名使用。"
