#!/usr/bin/env bash

set -Eeuo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/run-visual-e2e.sh [check|update] [-- <Playwright arguments>]

  check   Compare the application with committed visual baselines (default).
  update  Regenerate visual baselines before running the comparison.
EOF
}

mode="${1:-check}"
if [[ $# -gt 0 ]]; then
  shift
fi
if [[ "${1:-}" == "--" ]]; then
  shift
fi

case "$mode" in
  check | update) ;;
  -h | --help)
    usage
    exit 0
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
set -a
# shellcheck disable=SC1091
source "$repo_root/.env.test.example"
set +a

project="${COMPOSE_PROJECT_NAME:?set COMPOSE_PROJECT_NAME in .env.test.example}"
web_port="${WEB_HOST_PORT:?set WEB_HOST_PORT in .env.test.example}"
api_port="${API_HOST_PORT:?set API_HOST_PORT in .env.test.example}"

if [[ ! "$project" =~ ^[a-z0-9][a-z0-9_-]*$ ]] || [[ "$project" == "glazz" ]]; then
  echo "COMPOSE_PROJECT_NAME must be a safe name other than the development project 'glazz'." >&2
  exit 2
fi

export COMPOSE_PROJECT_NAME="$project"

compose=(
  docker compose
  --env-file "$repo_root/.env.test.example"
  --project-name "$project"
  --file "$repo_root/deploy/compose.yaml"
  --file "$repo_root/deploy/compose.e2e.yaml"
)

cleanup() {
  status=$?
  trap - EXIT INT TERM
  if ((status != 0)); then
    "${compose[@]}" logs --no-color --tail=200 api worker web >&2 || true
  fi
  "${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
  exit "$status"
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

cd "$repo_root"
"${compose[@]}" up --build --detach --wait

playwright_args=(tests/e2e/visual.spec.ts --reporter=line)
if [[ "$mode" == "update" ]]; then
  playwright_args+=(--update-snapshots=all)
fi
playwright_args+=("$@")

E2E_BASE_URL="$GLAZZ_E2E_WEB_ORIGIN" \
  E2E_API_URL="$GLAZZ_E2E_API_ORIGIN" \
  pnpm --filter @glazz/web exec playwright test "${playwright_args[@]}"
