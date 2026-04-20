#!/usr/bin/env sh
set -eu

ACTION="${1:-up}"
ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT"

ensure_env_file() {
  if [ ! -f .env ]; then
    cp .env.example .env
    echo "Creato .env da .env.example"
  fi
}

case "$ACTION" in
  init)
    ensure_env_file
    go mod tidy
    ;;
  build)
    ensure_env_file
    docker compose build
    ;;
  up)
    ensure_env_file
    docker compose up --build -d
    echo "API:        http://localhost:8080/health"
    echo "Metrics:    http://localhost:8080/metrics"
    echo "Prometheus: http://localhost:9090"
    echo "Grafana:    http://localhost:3000"
    ;;
  down)
    docker compose down
    ;;
  restart)
    ensure_env_file
    docker compose up --build -d
    ;;
  logs)
    docker compose logs -f --tail=200
    ;;
  status)
    docker compose ps
    ;;
  smoke)
    curl -fsS http://localhost:8080/health
    curl -fsS http://localhost:8080/metrics >/dev/null
    curl -fsS http://localhost:9090/-/ready
    curl -fsS http://localhost:3000/api/health
    ;;
  clean)
    docker compose down -v --remove-orphans
    ;;
  *)
    echo "Uso: $0 {init|build|up|down|restart|logs|status|smoke|clean}" >&2
    exit 1
    ;;
esac
