#!/usr/bin/env bash
# DBAWORK py-backend 容器启动脚本：同时拉起 gRPC 服务与 FastAPI
set -e
cd "$(dirname "$0")/.."

python -m rpc.server &
GRPC_PID=$!

python app.py &
API_PID=$!

term() {
  kill "$GRPC_PID" "$API_PID" 2>/dev/null || true
  wait "$GRPC_PID" "$API_PID" 2>/dev/null || true
}
trap term TERM INT

# 任一进程退出则整体退出
wait -n "$GRPC_PID" "$API_PID" || true
term
