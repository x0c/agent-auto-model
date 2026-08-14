#!/usr/bin/env bash
# 发版收尾：本机建/更新 GitHub Release、上传本平台二进制、更新 Homebrew 配方（防回退）。
# 用法（在 cli/ 目录）：
#   bash scripts/publish-release.sh            # 版本号取 VERSION
#   bash scripts/publish-release.sh v0.2.0     # 显式指定
#
# 可选环境变量：
#   CMM_SKIP_BINARIES=1  跳过构建/上传二进制
#   CMM_SKIP_TAP=1       跳过更新 Homebrew 配方
#   CMM_SKIP_CI_GATE=1   跳过发版前 go test
#   HOMEBREW_TAP_TOKEN   写 tap 仓库用的令牌（默认取 gh auth token）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

TAP_REPO="${CMM_TAP_REPO:-x0c/homebrew-tap}"
SOURCE_REPO="${CMM_REPO:-x0c/cursor-mode-model}"
FORMULA_NAME="agent-auto-model.rb"
ALIAS_FORMULA_NAME="cursor-mode-model.rb"

die() { echo "错误：$*" >&2; exit 1; }

command -v gh >/dev/null || die "未找到 gh 命令，无法操作 GitHub Release"
gh auth status >/dev/null 2>&1 || die "gh 未登录，先执行 gh auth login"

TAG="${1:-}"
if [ -z "$TAG" ]; then
  TAG="v$(tr -d '[:space:]' < VERSION)"
fi
VERSION="${TAG#v}"
echo "==> 发布 ${TAG}"

if [ "${CMM_SKIP_CI_GATE:-0}" = "1" ]; then
  echo "==> 跳过发版前测试（CMM_SKIP_CI_GATE=1）"
else
  echo "==> 发版前强制 go test + node register 测试"
  go test ./... || die "go test 未过，禁止发版收尾"
  if command -v node >/dev/null 2>&1; then
    node --test internal/assets/register.test.mjs || die "register.test.mjs 未过"
  fi
fi

git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null \
  || die "本地没有 ${TAG} 标签，先打好标签再跑本脚本"
git ls-remote --exit-code --tags github "refs/tags/${TAG}" >/dev/null 2>&1 \
  || die "${TAG} 还没推到 github 远端，先 git push github --tags"

# ---- 1. Release 本体 ----
if gh release view "$TAG" --repo "$SOURCE_REPO" >/dev/null 2>&1; then
  echo "==> Release ${TAG} 已存在"
else
  echo "==> 创建 Release ${TAG}"
  gh release create "$TAG" --repo "$SOURCE_REPO" --title "$TAG" --generate-notes
fi

# ---- 2. 本机平台二进制 ----
UPLOAD_FAILED=0
if [ "${CMM_SKIP_BINARIES:-0}" = "1" ]; then
  echo "==> 跳过二进制构建（CMM_SKIP_BINARIES=1）"
else
  DIST="$(mktemp -d)"
  trap 'rm -rf "$DIST"' EXIT
  OS="$(uname -s)"
  ARCH_RAW="$(uname -m)"
  case "$ARCH_RAW" in
    x86_64|amd64) ARCH=amd64 ;;
    arm64|aarch64) ARCH=arm64 ;;
    *) die "不支持的架构：$ARCH_RAW" ;;
  esac
  case "$OS" in
    Darwin)
      GOOS=darwin
      EXT=""
      ARCHIVE="agent-auto-model_${VERSION}_darwin_${ARCH}.tar.gz"
      SKIPPED="Linux / Windows 预编译包（需在对应平台发版或等 CI 补齐）"
      ;;
    Linux)
      GOOS=linux
      EXT=""
      ARCHIVE="agent-auto-model_${VERSION}_linux_${ARCH}.tar.gz"
      SKIPPED="macOS / Windows 预编译包（需在对应平台发版或等 CI 补齐）"
      ;;
    *) die "不支持的发版平台：$OS" ;;
  esac

  echo "==> 构建 ${GOOS}/${ARCH}"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$ARCH" go build \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o "$DIST/agent-auto-model${EXT}" ./cmd/agent-auto-model
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$ARCH" go build \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o "$DIST/cursor-mode-model${EXT}" ./cmd/cursor-mode-model
  tar -C "$DIST" -czf "$DIST/$ARCHIVE" "agent-auto-model${EXT}" "cursor-mode-model${EXT}"
  (
    cd "$DIST"
    shasum -a 256 "$ARCHIVE" > "${ARCHIVE}.sha256"
  )

  echo "==> 上传二进制"
  if ! gh release upload "$TAG" --repo "$SOURCE_REPO" "$DIST/$ARCHIVE" "$DIST/${ARCHIVE}.sha256" --clobber; then
    echo "!!  上传失败，等 8 秒重试一次"
    sleep 8
    if ! gh release upload "$TAG" --repo "$SOURCE_REPO" "$DIST/$ARCHIVE" "$DIST/${ARCHIVE}.sha256" --clobber; then
      UPLOAD_FAILED=1
      echo "!!  二进制上传仍未成功；继续更新 Homebrew 配方"
    fi
  fi
  echo "==> 本轮未覆盖：${SKIPPED}"
fi

# ---- 3. Homebrew 配方（源码归档编译，不依赖预编译附件）----
if [ "${CMM_SKIP_TAP:-0}" = "1" ]; then
  echo "==> 跳过 Homebrew 配方（CMM_SKIP_TAP=1）"
else
  TOKEN="${HOMEBREW_TAP_TOKEN:-$(gh auth token)}"
  [ -n "$TOKEN" ] || die "拿不到可写 ${TAP_REPO} 的令牌"
  ARCHIVE_URL="https://github.com/${SOURCE_REPO}/archive/refs/tags/${TAG}.tar.gz"
  echo "==> 计算源码归档校验和"
  SHA="$(curl -fsSL "$ARCHIVE_URL" | shasum -a 256 | awk '{print $1}')"
  WORK="$(mktemp -d)"
  git clone -q "https://x-access-token:${TOKEN}@github.com/${TAP_REPO}.git" "$WORK/tap"
  FORMULA="$WORK/tap/Formula/${FORMULA_NAME}"
  ALIAS_FORMULA="$WORK/tap/Formula/${ALIAS_FORMULA_NAME}"
  rc=0
  ARCHIVE_URL="$ARCHIVE_URL" SHA="$SHA" VERSION="$VERSION" \
    python3 "$ROOT/scripts/bump-homebrew-formula.py" "$FORMULA" || rc=$?
  alias_rc=0
  ARCHIVE_URL="$ARCHIVE_URL" SHA="$SHA" VERSION="$VERSION" \
    python3 "$ROOT/scripts/bump-homebrew-formula.py" "$ALIAS_FORMULA" || alias_rc=$?
  if [ "$rc" -eq 3 ] && [ "$alias_rc" -eq 3 ]; then
    echo "==> 配方已是更新版本，跳过写入"
    rm -rf "$WORK"
  elif [ "$rc" -ne 0 ] && [ "$rc" -ne 3 ]; then
    rm -rf "$WORK"; die "改写配方失败"
  elif [ "$alias_rc" -ne 0 ] && [ "$alias_rc" -ne 3 ]; then
    rm -rf "$WORK"; die "改写别名配方失败"
  else
    git -C "$WORK/tap" add "Formula/${FORMULA_NAME}" "Formula/${ALIAS_FORMULA_NAME}"
    if git -C "$WORK/tap" diff --cached --quiet -- "Formula/${FORMULA_NAME}" "Formula/${ALIAS_FORMULA_NAME}"; then
      echo "==> 配方已经指向 ${TAG}，无需改动"
    else
      git -C "$WORK/tap" -c user.name="x0c" -c user.email="x0c@users.noreply.github.com" \
        commit -q -m "agent-auto-model ${VERSION}"
      git -C "$WORK/tap" push -q origin main
      echo "==> 配方已更新到 ${VERSION}"
    fi
    rm -rf "$WORK"
  fi
fi

echo
echo "==> 收尾核对"
gh release view "$TAG" --repo "$SOURCE_REPO" --json tagName,assets \
  --jq '"Release \(.tagName)：\(.assets | length) 个附件"'
curl -fsSL "https://raw.githubusercontent.com/${TAP_REPO}/main/Formula/${FORMULA_NAME}" \
  | grep -E '^  url ' | sed 's/^/配方 /' || true
curl -fsSL "https://api.github.com/repos/${SOURCE_REPO}/releases/latest" \
  | python3 -c 'import json,sys; print("最新 Release：" + json.load(sys.stdin)["tag_name"])'
if [ "${UPLOAD_FAILED:-0}" = "1" ]; then
  echo "!!  注意：本机这轮二进制上传失败过，请对照附件数量确认是否需要重跑本脚本"
  exit 1
fi
echo "==> 完成"
