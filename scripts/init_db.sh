#!/usr/bin/env bash
set -euo pipefail

# 根据 DATABASE_URL 或默认值执行项目内 SQL 迁移
# 使用前请确保 PostgreSQL 已启动且凭证正确

PROJECT_ROOT=${PROJECT_ROOT:-$(pwd)}
MIGRATION=${MIGRATION:-db/migrations/000001_init.up.sql}
DATABASE_URL=${DATABASE_URL:-postgres://usde:123456@localhost:5432/usde?sslmode=disable}

if ! command -v psql >/dev/null 2>&1; then
  echo "psql 未安装，请先安装 postgresql-client" >&2
  exit 1
fi

if [[ ! -f "${PROJECT_ROOT}/${MIGRATION}" ]]; then
  echo "未找到迁移文件 ${PROJECT_ROOT}/${MIGRATION}" >&2
  exit 1
fi

echo "使用数据库: ${DATABASE_URL}" >&2
psql "${DATABASE_URL}" --set ON_ERROR_STOP=on -f "${PROJECT_ROOT}/${MIGRATION}"
echo "数据库迁移完成"
