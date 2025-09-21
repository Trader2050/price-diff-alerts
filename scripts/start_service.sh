#!/usr/bin/env bash
set -euo pipefail

# 构建并以后台方式启动 usdewatcher 服务
# 可在服务器上执行：
#   REPO_DIR=/srv/price-diff-alerts ./scripts/start_service.sh
# 可通过 ENV 控制：GO_FLAGS, CONFIG_PATH, LOG_FILE

REPO_DIR=${REPO_DIR:-$(pwd)}
BIN_DIR=${BIN_DIR:-$REPO_DIR/bin}
BIN_PATH=${BIN_PATH:-$BIN_DIR/usdewatcher}
CONFIG_PATH=${CONFIG_PATH:-$REPO_DIR/config.yaml}
LOG_FILE=${LOG_FILE:-$REPO_DIR/logs/usdewatcher.log}
GO_FLAGS=${GO_FLAGS:-}

if [[ ! -f "$CONFIG_PATH" ]]; then
  echo "未找到配置文件 $CONFIG_PATH" >&2
  exit 1
fi

mkdir -p "$BIN_DIR" "$(dirname "$LOG_FILE")"

cd "$REPO_DIR"

echo "编译 usdewatcher..."
go build $GO_FLAGS -o "$BIN_PATH" ./cmd/usdewatcher

echo "以 nohup 方式启动..."
nohup "$BIN_PATH" --config "$CONFIG_PATH" >>"$LOG_FILE" 2>&1 &
PID=$!

echo "服务已启动，PID=$PID，日志输出到 $LOG_FILE"
