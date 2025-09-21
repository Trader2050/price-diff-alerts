#!/usr/bin/env bash
set -euo pipefail

# 通过 CLI 导出 CSV/PNG，默认导出最近 24 小时

REPO_DIR=${REPO_DIR:-$(pwd)}
BIN_PATH=${BIN_PATH:-$REPO_DIR/bin/usdewatcher}
CONFIG_PATH=${CONFIG_PATH:-$REPO_DIR/config.yaml}
OUT_DIR=${OUT_DIR:-$REPO_DIR/out}
FROM_TS=${FROM_TS:-}
TO_TS=${TO_TS:-}
PNG_PATH=${PNG_PATH:-$OUT_DIR/usde-susde.png}
CSV_PATH=${CSV_PATH:-$OUT_DIR/usde-susde.csv}
MAX_POINTS=${MAX_POINTS:-200000}

if [[ ! -x "$BIN_PATH" ]]; then
  echo "未找到可执行文件 $BIN_PATH，尝试使用 go run" >&2
  USE_GORUN=1
fi

if [[ ! -f "$CONFIG_PATH" ]]; then
  echo "未找到配置文件 $CONFIG_PATH" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"

CMD_ARGS=(--config "$CONFIG_PATH" export "--png" "$PNG_PATH" "--csv" "$CSV_PATH" "--max-points" "$MAX_POINTS")

if [[ -n "$FROM_TS" ]]; then
  CMD_ARGS+=("--from" "$FROM_TS")
fi
if [[ -n "$TO_TS" ]]; then
  CMD_ARGS+=("--to" "$TO_TS")
fi

if [[ -n ${USE_GORUN-} ]]; then
  go run ./cmd/usdewatcher "${CMD_ARGS[@]}"
else
  "$BIN_PATH" "${CMD_ARGS[@]}"
fi

echo "导出完成：$PNG_PATH / $CSV_PATH"
