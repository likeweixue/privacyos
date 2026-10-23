#!/bin/bash
# 停止由 start-demo.sh 启动的 PrivacyOS 演示服务
set -u
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNTIME="$ROOT/runtime"
PIDFILE="$RUNTIME/privacyos.pid"

STOPPED=0
if [ -f "$PIDFILE" ]; then
  PID="$(cat "$PIDFILE" 2>/dev/null || true)"
  if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID" 2>/dev/null || true
    for _ in $(seq 1 20); do
      kill -0 "$PID" 2>/dev/null || break
      sleep 0.2
    done
    STOPPED=1
  fi
  rm -f "$PIDFILE"
fi
if [ "$STOPPED" -eq 0 ]; then
  pkill -f "$RUNTIME/privacyos-server" 2>/dev/null || true
fi
printf '[demo] 已停止\n'
