#!/usr/bin/env bash
# Generate a one-time, 5-minute filebrowser auto-login URL for a given
# absolute filesystem path. Intended for sharing into contexts (mobile
# in-app browsers, kiosk webviews) whose cookie jar is not shared with the
# primary login session.
#
# Usage:
#   ./scripts/share-url.sh <absolute-filesystem-path>
#
# Env (optional):
#   FB_HOST            default: files.bingsucouple.com
#   FB_ADMIN_USER      default: admin
#   FB_ADMIN_PASSWORD  required (or sourced from ~/.claude/.secrets/filebrowser-env)
#
# Security notes:
#   - The emitted URL embeds a JWT whose ExpiresAt is now+5 min and which
#     is server-side single-use. After the recipient browser exchanges it
#     for a regular session JWT, the URL is no longer usable, and the SPA
#     strips `?auth=` from `window.location` before the network round-trip
#     so it never lands in browser history.
#   - The URL must still NOT be shared with third parties: any browser
#     that opens the link within the 5-minute window before the legitimate
#     recipient gets the session.
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <absolute-filesystem-path>" >&2
  exit 2
fi
path="$1"

: "${FB_HOST:=files.bingsucouple.com}"
: "${FB_ADMIN_USER:=admin}"

if [[ -z "${FB_ADMIN_PASSWORD:-}" && -f "$HOME/.claude/.secrets/filebrowser-env" ]]; then
  # shellcheck disable=SC1091
  set -a; source "$HOME/.claude/.secrets/filebrowser-env"; set +a
  FB_ADMIN_PASSWORD="${FB_ADMIN_PASSWORD:-${FILEBROWSER_ADMIN_PASSWORD:-}}"
fi

if [[ -z "${FB_ADMIN_PASSWORD:-}" ]]; then
  echo "FB_ADMIN_PASSWORD not set and ~/.claude/.secrets/filebrowser-env did not provide one" >&2
  exit 3
fi

# 1) Log in to get a regular JWT.
jwt=$(curl -sS -X POST "https://$FB_HOST/api/login" \
        -H 'Content-Type: application/json' \
        -d "$(jq -nc --arg u "$FB_ADMIN_USER" --arg p "$FB_ADMIN_PASSWORD" \
               '{username:$u, password:$p, recaptcha:""}')")
if [[ -z "$jwt" || $(printf '%s' "$jwt" | tr -cd '.' | wc -c) -ne 2 ]]; then
  echo "login failed: $jwt" >&2
  exit 4
fi

# 2) Request a 5-minute one-time-use short token.
short=$(curl -sS -X POST "https://$FB_HOST/api/short-token" -H "X-Auth: $jwt")
if [[ -z "$short" || $(printf '%s' "$short" | tr -cd '.' | wc -c) -ne 2 ]]; then
  echo "short-token issuance failed: $short" >&2
  exit 5
fi

# 3) Compose the URL. Filebrowser exposes the absolute path under /files/
#    with the leading slash dropped (see ~/.claude/rules/share-image-outputs.md).
encoded_path="${path#/}"
printf 'https://%s/files/%s?auth=%s\n' "$FB_HOST" "$encoded_path" "$short"
