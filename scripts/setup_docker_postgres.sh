#!/usr/bin/env bash
set -euo pipefail

# 在服务器上执行该脚本，用于安装 Docker / Docker Compose，并启动本项目自带的 Postgres
# 依赖：Ubuntu/Debian 系统；如使用其它发行版，请自行调整安装命令。

REPO_DIR=${REPO_DIR:-$(pwd)}
COMPOSE_FILE=${COMPOSE_FILE:-docker-compose.yaml}
SERVICE_NAME=${SERVICE_NAME:-db}

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

require_root() {
  if [[ "$EUID" -ne 0 ]]; then
    log "请使用 sudo 或 root 权限运行该脚本"
    exit 1
  fi
}

install_docker() {
  if command -v docker >/dev/null 2>&1; then
    log "Docker 已安装，跳过"
    return
  fi

  log "安装 Docker..."
  apt-get update -y
  apt-get install -y ca-certificates curl gnupg
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg

  . /etc/os-release
  echo "deb [arch=\$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/$ID $VERSION_CODENAME stable" \
    >/etc/apt/sources.list.d/docker.list

  apt-get update -y
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  systemctl enable --now docker
  log "Docker 安装完成"
}

install_compose_plugin() {
  if docker compose version >/dev/null 2>&1; then
    log "Docker Compose 插件已就绪"
    return
  fi
  log "Docker Compose 插件安装可能失败，请检查 docker 安装"
  exit 1
}

start_compose() {
  cd "$REPO_DIR"

  if [[ ! -f ${COMPOSE_FILE} ]]; then
    log "未找到 ${COMPOSE_FILE}，请确认项目路径"
    exit 1
  fi

  log "启动/更新 docker compose 服务 ${SERVICE_NAME}"
  docker compose -f "$COMPOSE_FILE" pull "$SERVICE_NAME" || true
  docker compose -f "$COMPOSE_FILE" up -d "$SERVICE_NAME"
  log "运行状态："
  docker compose -f "$COMPOSE_FILE" ps "$SERVICE_NAME"
}

require_root
install_docker
install_compose_plugin
start_compose
