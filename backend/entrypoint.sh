#!/bin/sh
# 容器入口：迁移在 API 进程内执行（带 advisory lock），失败即退出。
set -e

echo "[nwbd-dev-workstation] 启动 API"
exec /usr/local/bin/api
