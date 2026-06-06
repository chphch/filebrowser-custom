#!/usr/bin/env bash
# Cloudflare cache purge — by URLs or whole zone.
#
#   ./scripts/cf-purge.sh url <URL> [<URL> ...]   # specific files
#   ./scripts/cf-purge.sh all                     # purge_everything (zone-wide)
#
# Env / secrets (loaded from ~/.claude/.secrets/.cloudflare-env if present):
#   CF_API_TOKEN  required. Token scope: Zone → Cache Purge: Purge.
#   CF_ZONE_ID    required. The zone id for bingsucouple.com.
#                 (Dashboard → zone → Overview right sidebar.)
#
# Cache header policy of the filebrowser fork (post fix-static-cache-on-404):
#   - 200 .js: Cache-Control max-age=86400 (gz/br variant served)
#   - 404 .js: no Cache-Control set → CDN respects default minimal TTL
#   - non-.js index handler: same (set only on success)
#
# Usage from another script:
#   CF_API_TOKEN=… CF_ZONE_ID=… scripts/cf-purge.sh url https://files.bingsucouple.com/foo
set -euo pipefail

if [[ -f "$HOME/.claude/.secrets/.cloudflare-env" ]]; then
  # shellcheck disable=SC1091
  set -a; source "$HOME/.claude/.secrets/.cloudflare-env"; set +a
fi

: "${CF_API_TOKEN:?CF_API_TOKEN unset — see ~/.claude/.secrets/.cloudflare-env}"
: "${CF_ZONE_ID:?CF_ZONE_ID unset — see ~/.claude/.secrets/.cloudflare-env}"

mode="${1:-help}"
shift || true

call() {
  local body="$1"
  curl -sS -X POST \
    "https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/purge_cache" \
    -H "Authorization: Bearer $CF_API_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$body"
}

case "$mode" in
  url)
    if [[ $# -eq 0 ]]; then
      echo "usage: $0 url <URL> [<URL> ...]" >&2
      exit 2
    fi
    files_json=$(python3 -c 'import json, sys; print(json.dumps({"files": sys.argv[1:]}))' "$@")
    echo "[cf-purge] purging ${#@} URL(s)…" >&2
    call "$files_json"
    echo
    ;;
  all)
    echo "[cf-purge] WARNING: purging EVERYTHING in zone $CF_ZONE_ID" >&2
    call '{"purge_everything":true}'
    echo
    ;;
  help|-h|--help|*)
    sed -n '2,15p' "$0" | sed 's/^# //; s/^#$//'
    ;;
esac
