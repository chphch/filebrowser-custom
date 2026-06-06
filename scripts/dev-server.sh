#!/usr/bin/env bash
# Local dev server for this filebrowser fork.
#
#   start   — boot Go backend (:8080) + Vite dev (:5173) in background
#   stop    — kill only the dev processes (prod service is never touched)
#   restart — stop then start
#   status  — PIDs + listening sockets
#   logs    — tail backend + vite logs together
#
# Invariants:
#   - Backend binds 127.0.0.1:8088 (vite.config.ts proxies /api there via VITE_BACKEND env).
#     :8080 is left alone so a local prod filebrowser default-port install keeps working.
#   - Test DB: /tmp/fb_dev/filebrowser.db   — separate from prod ~/.config/filebrowser/filebrowser.db.
#   - Test root: /tmp/fb_dev/root           — sandbox dir; created on first start.
#   - --noauth flag bypasses login. URL is http://localhost:5173.
#   - Logs + PIDs under /tmp/fb_dev/.
#
# Why not touch prod: ~/.claude/projects/-Users-chphch-Projects-filebrowser-src/memory/feedback_test_server_coexist.md
set -euo pipefail

cmd="${1:-help}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
state_dir="/tmp/fb_dev"
db_path="$state_dir/filebrowser.db"
root_path="$state_dir/root"
backend_log="$state_dir/backend.log"
vite_log="$state_dir/vite.log"
backend_pid="$state_dir/backend.pid"
vite_pid="$state_dir/vite.pid"
backend_port=8088
vite_port=5173

wait_for_port() {
  local port="$1" name="$2" tries=60
  while (( tries-- > 0 )); do
    if (echo > "/dev/tcp/127.0.0.1/$port") >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "[dev-server] $name did not start within 60s — see logs" >&2
  return 1
}

is_running() {
  local pidfile="$1"
  [[ -f "$pidfile" ]] && kill -0 "$(cat "$pidfile")" 2>/dev/null
}

cmd_start() {
  mkdir -p "$state_dir" "$root_path"

  if is_running "$backend_pid"; then
    echo "[dev-server] backend already up (pid $(cat "$backend_pid"))"
  else
    echo "[dev-server] starting backend on 127.0.0.1:$backend_port (db=$db_path root=$root_path)"
    (
      cd "$repo_root"
      nohup go run . \
        -a 127.0.0.1 -p "$backend_port" \
        -d "$db_path" -r "$root_path" \
        --noauth \
        >> "$backend_log" 2>&1 &
      echo $! > "$backend_pid"
    )
    wait_for_port "$backend_port" "backend"
  fi

  if [[ ! -d "$repo_root/frontend/node_modules" ]]; then
    echo "[dev-server] installing frontend deps"
    (cd "$repo_root/frontend" && pnpm install)
  fi

  if is_running "$vite_pid"; then
    echo "[dev-server] vite already up (pid $(cat "$vite_pid"))"
  else
    echo "[dev-server] starting vite on 127.0.0.1:$vite_port (proxy → :$backend_port)"
    (
      cd "$repo_root/frontend"
      VITE_BACKEND="http://127.0.0.1:$backend_port" \
        nohup pnpm run dev --port "$vite_port" --strictPort \
        >> "$vite_log" 2>&1 &
      echo $! > "$vite_pid"
    )
    wait_for_port "$vite_port" "vite"
  fi

  echo
  echo "[dev-server] up:"
  echo "  app:     http://localhost:$vite_port/"
  echo "  backend: http://127.0.0.1:$backend_port/ (proxied via vite /api)"
  echo "  logs:    $backend_log | $vite_log"
}

cmd_stop() {
  for pf in "$vite_pid" "$backend_pid"; do
    if [[ -f "$pf" ]]; then
      pid=$(cat "$pf")
      if kill -0 "$pid" 2>/dev/null; then
        echo "[dev-server] stopping $(basename "$pf" .pid) (pid $pid)"
        # Kill the whole process group — go run / pnpm spawn children.
        pkill -P "$pid" 2>/dev/null || true
        kill "$pid" 2>/dev/null || true
      fi
      rm -f "$pf"
    fi
  done
  # Sweep any lingering children from a previous crash.
  pkill -f "go run \\. -a 127.0.0.1 -p $backend_port" 2>/dev/null || true
  pkill -f "vite.*--port $vite_port" 2>/dev/null || true
}

cmd_restart() {
  cmd_stop
  sleep 1
  cmd_start
}

cmd_status() {
  for pair in "backend:$backend_pid:$backend_port" "vite:$vite_pid:$vite_port"; do
    IFS=: read -r name pf port <<<"$pair"
    if is_running "$pf"; then
      echo "$name: up (pid $(cat "$pf")) on 127.0.0.1:$port"
    else
      echo "$name: down"
    fi
  done
}

cmd_logs() {
  case "${2:-both}" in
    backend) tail -F "$backend_log" ;;
    vite)    tail -F "$vite_log" ;;
    *)       tail -F "$backend_log" "$vite_log" ;;
  esac
}

case "$cmd" in
  start)   cmd_start "$@" ;;
  stop)    cmd_stop "$@" ;;
  restart) cmd_restart "$@" ;;
  status)  cmd_status "$@" ;;
  logs)    cmd_logs "$@" ;;
  help|-h|--help|*) sed -n '2,16p' "$0" | sed 's/^# //; s/^#$//' ;;
esac
