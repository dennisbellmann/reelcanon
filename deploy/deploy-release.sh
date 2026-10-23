#!/usr/bin/env bash
set -Eeuo pipefail

commit_sha="${1:-}"
service_uuid="${2:-}"
health_url="${3:-}"

if [[ ! "$commit_sha" =~ ^[0-9a-f]{40}$ ]]; then
  echo "Invalid commit SHA." >&2
  exit 2
fi
if [[ ! "$service_uuid" =~ ^[a-zA-Z0-9_-]+$ ]]; then
  echo "Invalid Coolify service UUID." >&2
  exit 2
fi
if [[ ! "$health_url" =~ ^https://[a-zA-Z0-9.-]+/reelcanon/api/v1/healthz$ ]]; then
  echo "Invalid Reel Canon health-check URL." >&2
  exit 2
fi

archive="/tmp/reelcanon-${commit_sha}.tar.gz"
context="/data/coolify/source/reelcanon"
stage="$(mktemp -d /tmp/reelcanon-stage.XXXXXX)"
deploy_auth_config="$(mktemp /tmp/reelcanon-coolify-deploy-auth.XXXXXX)"
read_auth_config="$(mktemp /tmp/reelcanon-coolify-read-auth.XXXXXX)"
script_path="$(readlink -f "$0")"
deploy_token=""
read_token=""
deployment_uuid=""
lock_file="$HOME/.reelcanon-deploy.lock"

cleanup() {
  rm -rf -- "$stage"
  rm -f -- "$deploy_auth_config" "$read_auth_config" "$archive" "$script_path"
}
trap cleanup EXIT

exec 9>"$lock_file"
flock 9

IFS= read -r deploy_token || {
  echo "Missing Coolify deploy token on stdin." >&2
  exit 2
}
IFS= read -r read_token || {
  echo "Missing Coolify read token on stdin." >&2
  exit 2
}
if [[ -z "$deploy_token" || -z "$read_token" ]]; then
  echo "A Coolify API token is empty." >&2
  exit 2
fi
printf 'header = "Authorization: Bearer %s"\n' "$deploy_token" > "$deploy_auth_config"
printf 'header = "Authorization: Bearer %s"\n' "$read_token" > "$read_auth_config"
chmod 600 "$deploy_auth_config" "$read_auth_config"
unset deploy_token read_token

if [[ ! -f "$archive" ]]; then
  echo "Release archive not found: $archive" >&2
  exit 2
fi
if [[ ! -d "$context" || ! -w "$context" ]]; then
  echo "Coolify build context must exist and be writable by the deploy user: $context" >&2
  exit 2
fi

python3 - "$archive" <<'PY'
import sys
import tarfile

archive = sys.argv[1]
with tarfile.open(archive, "r:gz") as bundle:
    for member in bundle.getmembers():
        name = member.name
        parts = name.split("/")
        if name.startswith("/") or ".." in parts:
            raise SystemExit(f"Unsafe archive path: {name}")
        if name in {".env", ".local", ".git"} or name.startswith((".local/", ".git/")):
            raise SystemExit(f"Release archive contains excluded local data: {name}")
        if name.startswith(".env.") and name != ".env.example":
            raise SystemExit(f"Release archive contains a possible secret file: {name}")
PY

tar -xzf "$archive" -C "$stage"
for required in Dockerfile deploy/coolify-compose.yaml deploy/init-api-role.sh; do
  if [[ ! -f "$stage/$required" ]]; then
    echo "Release archive is missing $required." >&2
    exit 2
  fi
done

# Replace the dedicated build context with tracked files from this exact commit.
# Preserve any local-only credentials if a previous setup put them here by mistake.
shopt -s dotglob nullglob
for entry in "$context"/*; do
  name="${entry##*/}"
  case "$name" in
    .env|.env.*|.local|.git) continue ;;
  esac
  rm -rf -- "$entry"
done
cp -a "$stage"/. "$context"/
printf '%s\n' "$commit_sha" > "$context/RELEASE_SHA"

response="$(curl --config "$deploy_auth_config" --silent --show-error --fail-with-body \
  --request POST \
  "http://127.0.0.1:8000/api/v1/deploy?uuid=${service_uuid}&force=true")"
deploy_result="$(printf '%s' "$response" | python3 -c '
import json, sys
data = json.load(sys.stdin)
deployment_uuid = ""
items = []
if isinstance(data, dict):
    deployment_uuid = data.get("deployment_uuid", "")
    items = data.get("deployments", [])
elif isinstance(data, list):
    items = data
if not deployment_uuid and isinstance(items, list) and items:
    first = items[0]
    if isinstance(first, dict):
        deployment_uuid = first.get("deployment_uuid", "")
    elif isinstance(first, str):
        deployment_uuid = first
entry = items[0] if isinstance(items, list) and items and isinstance(items[0], dict) else {}
message = str(entry.get("message", data.get("message", "") if isinstance(data, dict) else ""))
service_started = message.startswith("Service ") and ("started." in message or "request queued." in message)
print(deployment_uuid)
print("service" if service_started else "")
')"
deployment_uuid="${deploy_result%%$'\n'*}"
deploy_mode="${deploy_result#*$'\n'}"
if [[ "$deployment_uuid" =~ ^[a-zA-Z0-9_-]+$ ]]; then
  echo "Coolify queued deployment $deployment_uuid for $commit_sha."
elif [[ "$deploy_mode" == "service" ]]; then
  echo "Coolify queued the Compose service; waiting for public health to report build $commit_sha."
  deadline=$((SECONDS + 900))
  last_build=""
  while (( SECONDS < deadline )); do
    if health_response="$(curl --silent --show-error --fail "$health_url" 2>/dev/null)"; then
      health_info="$(printf '%s' "$health_response" | python3 -c '
import json, sys
data = json.load(sys.stdin)
print("{}\t{}".format(data.get("status", ""), data.get("build", "")))
' 2>/dev/null || true)"
      IFS=$'\t' read -r health_status health_build <<< "$health_info"
      if [[ "$health_status" == "ok" && "$health_build" == "$commit_sha" ]]; then
        echo "Deployment finished successfully for $commit_sha."
        exit 0
      fi
      if [[ -n "$health_build" && "$health_build" != "$last_build" ]]; then
        echo "Waiting for build $commit_sha; the live app currently reports $health_build."
        last_build="$health_build"
      fi
    fi
    sleep 10
  done
  echo "Timed out waiting for the live app to report build $commit_sha at $health_url." >&2
  exit 1
else
  echo "Coolify did not return a deployment UUID or a recognized service-start message." >&2
  exit 1
fi

deadline=$((SECONDS + 900))
while (( SECONDS < deadline )); do
  status_response="$(curl --config "$read_auth_config" --silent --show-error --fail-with-body \
    "http://127.0.0.1:8000/api/v1/deployments/${deployment_uuid}")"
  status="$(printf '%s' "$status_response" | python3 -c '
import json, sys
data = json.load(sys.stdin)
print(data.get("status", "unknown") if isinstance(data, dict) else "unknown")
')"
  case "$status" in
    finished|success|succeeded)
      echo "Deployment finished successfully for $commit_sha."
      exit 0
      ;;
    failed|error|cancelled|cancelled-by-user|timed_out)
      echo "Coolify deployment ended with status: $status. Review its deployment logs in Coolify." >&2
      exit 1
      ;;
    *)
      sleep 10
      ;;
  esac
done

echo "Timed out waiting for Coolify deployment $deployment_uuid. Check its status in Coolify." >&2
exit 1
