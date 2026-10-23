#!/usr/bin/env bash
set -Eeuo pipefail

commit_sha="${1:-}"
service_uuid="${2:-}"

if [[ ! "$commit_sha" =~ ^[0-9a-f]{40}$ ]]; then
  echo "Invalid commit SHA." >&2
  exit 2
fi
if [[ ! "$service_uuid" =~ ^[a-zA-Z0-9_-]+$ ]]; then
  echo "Invalid Coolify service UUID." >&2
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

response="$(curl --config "$deploy_auth_config" --silent --show-error --fail-with-body \
  --request POST \
  "http://127.0.0.1:8000/api/v1/deploy?uuid=${service_uuid}&force=true")"
deployment_uuid="$(printf '%s' "$response" | python3 -c '
import json, re, sys
data = json.load(sys.stdin)
deployment_uuid = ""
def safe_message(value):
    message = str(value).replace("\n", " ")[:300]
    return re.sub(r"\b\d+\|[A-Za-z0-9]+\b", "<redacted>", message)

if isinstance(data, dict):
    deployment_uuid = data.get("deployment_uuid", "")
    items = data.get("deployments", [])
    if not deployment_uuid and isinstance(items, list) and items:
        first = items[0]
        if isinstance(first, dict):
            deployment_uuid = first.get("deployment_uuid", "")
        elif isinstance(first, str):
            deployment_uuid = first
    if not deployment_uuid:
        first = items[0] if isinstance(items, list) and items else None
        first_fields = sorted(first.keys()) if isinstance(first, dict) else []
        entry_message = safe_message(first.get("message", "")) if isinstance(first, dict) else ""
        item_count = len(items) if isinstance(items, (list, dict)) else "unknown"
        top_message = safe_message(data.get("message", ""))
        print(
            f"Coolify deploy response fields: {sorted(data.keys())}; "
            f"deployments type: {type(items).__name__}; count: {item_count}; "
            f"first entry type: {type(first).__name__}; first entry fields: {first_fields}; "
            f"message: {top_message}; entry message: {entry_message}",
            file=sys.stderr,
        )
elif isinstance(data, list) and data and isinstance(data[0], dict):
    deployment_uuid = data[0].get("deployment_uuid", "")
print(deployment_uuid)
')"
if [[ ! "$deployment_uuid" =~ ^[a-zA-Z0-9_-]+$ ]]; then
  echo "Coolify did not return a valid deployment UUID." >&2
  exit 1
fi
echo "Coolify queued deployment $deployment_uuid for $commit_sha."

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
