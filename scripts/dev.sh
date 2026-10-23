#!/bin/bash
# ============================================================================
# PrivacyOS 本地开发环境：后端 (127.0.0.1:8080) + Vite (127.0.0.1:5173)
# 数据写入 dev-data/，使用模拟硬盘/Docker/监控数据，不会修改真实系统。
# ============================================================================
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO="${GO:-go}"
NPM="${NPM:-npm}"

export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export GOSUMDB="${GOSUMDB:-sum.golang.google.cn}"

command -v "${GO}" >/dev/null 2>&1 || { echo "未找到 go（需要 Go 1.24+）" >&2; exit 1; }
command -v "${NPM}" >/dev/null 2>&1 || { echo "未找到 npm（需要 Node 20+）" >&2; exit 1; }

if [ ! -d "${ROOT}/frontend/node_modules" ]; then
    echo "[dev] 安装前端依赖…"
    ( cd "${ROOT}/frontend" && "${NPM}" install --no-audit --no-fund )
fi

export PRIVACYOS_DEV=1
export PRIVACYOS_DATA_DIR="${ROOT}/dev-data/data"
export PRIVACYOS_CONFIG_DIR="${ROOT}/dev-data/etc"
export PRIVACYOS_LOG_DIR="${ROOT}/dev-data/log"
export PRIVACYOS_WEB_DIR="${ROOT}/frontend/dist"
export PRIVACYOS_LISTEN="127.0.0.1:8080"

mkdir -p "${PRIVACYOS_DATA_DIR}" "${PRIVACYOS_CONFIG_DIR}" "${PRIVACYOS_LOG_DIR}"

cleanup() {
    trap - INT TERM EXIT
    [ -n "${BACK_PID:-}" ] && kill "${BACK_PID}" 2>/dev/null || true
    [ -n "${WEB_PID:-}" ] && kill "${WEB_PID}" 2>/dev/null || true
    wait 2>/dev/null || true
}
trap cleanup INT TERM EXIT

echo "[dev] 启动后端 http://127.0.0.1:8080"
( cd "${ROOT}/backend" && exec "${GO}" run ./cmd/privacyos-server ) &
BACK_PID=$!

sleep 2

echo "[dev] 启动前端 http://127.0.0.1:5173"
( cd "${ROOT}/frontend" && exec "${NPM}" run dev -- --host 127.0.0.1 ) &
WEB_PID=$!

cat <<MSG

  PrivacyOS 开发环境已启动
  ------------------------
  Web 面板 : http://127.0.0.1:5173
  API      : http://127.0.0.1:8080/api/v1
  管理员   : padmin / privacyos
  演示用户 : demo / demo1234
  数据目录 : ${ROOT}/dev-data

  按 Ctrl+C 停止。

MSG

wait "${BACK_PID}" "${WEB_PID}"
