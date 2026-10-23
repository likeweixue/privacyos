#!/bin/bash
# ============================================================================
# 在本机后台运行 PrivacyOS 演示服务（开发模式，模拟硬件/Docker）
#
#   ./scripts/start-demo.sh            # 使用已有构建产物（缺失时自动构建）
#   ./scripts/start-demo.sh --build    # 强制重新构建前端与后端
#   ./scripts/stop-demo.sh             # 停止
#
# 默认监听 0.0.0.0:8080，局域网内其他设备也可访问。
# 数据写入 dev-data/，不会触碰真实磁盘或系统配置。
# ============================================================================
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNTIME="$ROOT/runtime"
PIDFILE="$RUNTIME/privacyos.pid"
LOG="$RUNTIME/privacyos.log"
BUILD=0
[ "${1:-}" = "--build" ] && BUILD=1

# 常见安装位置，兼容未加载 shell profile 的情况
export PATH="/usr/local/go/bin:$HOME/.local/bin:$HOME/go/bin:$PATH"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export GOSUMDB="${GOSUMDB:-sum.golang.google.cn}"
export GOCACHE="${GOCACHE:-$RUNTIME/go-cache}"

log() { printf '\033[1;36m[demo]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[demo]\033[0m %s\n' "$*" >&2; exit 1; }

command -v go >/dev/null 2>&1 || die "未找到 go。请安装 Go 1.24+（或把 /usr/local/go/bin 加入 PATH）"
command -v npm >/dev/null 2>&1 || die "未找到 npm。请安装 Node.js 20+（或把 ~/.local/bin 加入 PATH）"

mkdir -p "$RUNTIME"

if [ -f "$PIDFILE" ]; then
  OLD_PID="$(cat "$PIDFILE" 2>/dev/null || true)"
  if [ -n "$OLD_PID" ] && kill -0 "$OLD_PID" 2>/dev/null; then
    log "旧实例仍在运行 (PID $OLD_PID)，先停止"
    kill "$OLD_PID" 2>/dev/null || true
    sleep 1
  fi
  rm -f "$PIDFILE"
fi

if [ "$BUILD" -eq 1 ] || [ ! -f "$ROOT/frontend/dist/index.html" ]; then
  log "构建前端…"
  ( cd "$ROOT/frontend" && if [ ! -d node_modules ]; then npm install --no-audit --no-fund; fi && npm run build )
fi
if [ "$BUILD" -eq 1 ] || [ ! -x "$RUNTIME/privacyos-server" ]; then
  log "构建后端…"
  ( cd "$ROOT/backend" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$RUNTIME/privacyos-server" ./cmd/privacyos-server )
fi

export PRIVACYOS_DEV=1
export PRIVACYOS_LISTEN="0.0.0.0:8080"
export PRIVACYOS_DATA_DIR="$ROOT/dev-data/data"
export PRIVACYOS_CONFIG_DIR="$ROOT/dev-data/etc"
export PRIVACYOS_LOG_DIR="$ROOT/dev-data/log"
export PRIVACYOS_WEB_DIR="$ROOT/frontend/dist"

mkdir -p "$PRIVACYOS_DATA_DIR" "$PRIVACYOS_CONFIG_DIR" "$PRIVACYOS_LOG_DIR"

log "启动 PrivacyOS（模拟模式）…"
nohup "$RUNTIME/privacyos-server" > "$LOG" 2>&1 &
PID=$!
echo "$PID" > "$PIDFILE"

for _ in $(seq 1 40); do
  if curl -fsS -m 1 http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
    break
  fi
  sleep 0.25
done
if ! curl -fsS -m 2 http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
  log "启动失败，最近日志："
  tail -30 "$LOG" >&2 || true
  exit 1
fi

LAN_IP="$(ipconfig getifaddr en0 2>/dev/null || ipconfig getifaddr en1 2>/dev/null || true)"
[ -n "$LAN_IP" ] || LAN_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
[ -n "$LAN_IP" ] || LAN_IP="127.0.0.1"

cat <<MSG

  PrivacyOS 已在后台运行
  ------------------------------------------------
  本机访问 : http://127.0.0.1:8080
  局域网   : http://${LAN_IP}:8080
  管理员   : padmin / privacyos
  演示用户 : demo / demo1234
  数据目录 : ${ROOT}/dev-data
  日志     : ${LOG}
  PID      : ${PID}

  停止服务 : ./scripts/stop-demo.sh
  重新构建 : ./scripts/start-demo.sh --build

  提示：这是开发演示模式，磁盘/Docker/监控为模拟数据，
        仅建议在可信局域网内使用默认密码体验。
MSG
