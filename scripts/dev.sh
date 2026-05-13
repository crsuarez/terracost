#!/usr/bin/env bash
# dev.sh — local Docker-based build/test/run driver for the terracost fork.
#
# All Go compilation, test execution, and migrations happen INSIDE the
# `runner` container defined in docker-compose.dev.yml. By default the
# runner is wired to a bundled MySQL container at the static IP
# 172.44.0.2:3306 (matching the original hardcoded DSN). The Go sources
# now honor the TERRACOST_DSN env var, so you can also point the runner
# at an external database (see "EXTERNAL DATABASE" below).
#
# Run `./scripts/dev.sh help` for the full command list.

set -euo pipefail

# ─── locate repo root regardless of where the script is invoked from ───
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." &>/dev/null && pwd)"
cd "${REPO_ROOT}"

COMPOSE_FILE="docker-compose.dev.yml"
PROJECT_NAME="terracost-dev"
RUNNER_SVC="runner"
DB_SVC="database"

# ─── DB connection settings (env-overridable) ───
# Defaults reproduce the original hardcoded bundled-MySQL DSN.
BUNDLED_DB_HOST="172.44.0.2"
TERRACOST_DB_HOST="${TERRACOST_DB_HOST:-${BUNDLED_DB_HOST}}"
TERRACOST_DB_PORT="${TERRACOST_DB_PORT:-3306}"
TERRACOST_DB_USER="${TERRACOST_DB_USER:-root}"
TERRACOST_DB_PASSWORD="${TERRACOST_DB_PASSWORD:-terracost}"
TERRACOST_DB_NAME="${TERRACOST_DB_NAME:-terracost_test}"
# If TERRACOST_DSN isn't set explicitly, derive it from the parts above.
if [[ -z "${TERRACOST_DSN:-}" ]]; then
  TERRACOST_DSN="${TERRACOST_DB_USER}:${TERRACOST_DB_PASSWORD}@tcp(${TERRACOST_DB_HOST}:${TERRACOST_DB_PORT})/${TERRACOST_DB_NAME}?multiStatements=true"
fi
export TERRACOST_DSN TERRACOST_DB_HOST TERRACOST_DB_PORT TERRACOST_DB_USER TERRACOST_DB_PASSWORD TERRACOST_DB_NAME

# External-DB mode: skip bundled MySQL when host is non-default OR opt-out is set.
NO_BUNDLED_DB="${TERRACOST_NO_BUNDLED_DB:-0}"

# CLI flag override: --no-db / --external-db before the subcommand.
while [[ "${1:-}" == --* ]]; do
  case "$1" in
    --no-db|--external-db) NO_BUNDLED_DB=1 ;;
    --)                    shift; break ;;
    *)
      echo "unknown flag: $1" >&2
      exit 64
      ;;
  esac
  shift
done

is_external_db() {
  if [[ "${NO_BUNDLED_DB}" == "1" ]]; then return 0; fi
  if [[ "${TERRACOST_DB_HOST}" != "${BUNDLED_DB_HOST}" ]]; then return 0; fi
  return 1
}

# Prefer `docker compose` (v2) and fall back to `docker-compose`.
if docker compose version >/dev/null 2>&1; then
  DC=(docker compose -f "${COMPOSE_FILE}" -p "${PROJECT_NAME}")
elif command -v docker-compose >/dev/null 2>&1; then
  DC=(docker-compose -f "${COMPOSE_FILE}" -p "${PROJECT_NAME}")
else
  echo "error: neither 'docker compose' nor 'docker-compose' is available on PATH" >&2
  exit 127
fi

# Run a command inside the runner container. Brings the stack up first if needed.
# Always propagates TERRACOST_* env so Go sees the right DSN.
exec_in_runner() {
  ensure_up
  "${DC[@]}" exec -T \
    -e TERRACOST_DSN \
    -e TERRACOST_DB_HOST \
    -e TERRACOST_DB_PORT \
    -e TERRACOST_DB_USER \
    -e TERRACOST_DB_PASSWORD \
    -e TERRACOST_DB_NAME \
    "${RUNNER_SVC}" "$@"
}

ensure_up() {
  if ! "${DC[@]}" ps --status running --services 2>/dev/null | grep -qx "${RUNNER_SVC}"; then
    cmd_up
  fi
}

ensure_db_up() {
  if is_external_db; then return 0; fi
  if ! "${DC[@]}" ps --status running --services 2>/dev/null | grep -qx "${DB_SVC}"; then
    "${DC[@]}" up -d "${DB_SVC}"
  fi
}

# ──────────────────────── commands ────────────────────────

cmd_help() {
  cat <<'EOF'
terracost dev.sh — local Docker build/test/run driver

USAGE
    ./scripts/dev.sh [global-flags] <command> [args...]

GLOBAL FLAGS
    --no-db, --external-db
                    Do not start (or require) the bundled MySQL container.
                    Equivalent to TERRACOST_NO_BUNDLED_DB=1.

LIFECYCLE
    build           Build (or rebuild) the runner image from Dockerfile.dev.
    up              Start the runner (+ bundled MySQL unless --no-db).
    down            Stop the stack (keeps named volumes).
    clean           Stop the stack AND remove its volumes (DB data + Go cache).
    ps              Show stack status.
    logs [svc]      Tail logs from the stack (defaults to all services).

    db-up           Start ONLY the bundled MySQL container (no runner).
    db-down         Stop ONLY the bundled MySQL container.

BUILD / LINT
    compile         go build ./...               (compiles every package)
    vet             go vet ./...
    lint            golangci-lint run -v         (installs the linter on first run)
    generate        make generate                (regenerates mocks + enumer files)
    tidy            go mod tidy

DATABASE
    migrate         Run scripts/migrate.go to create the schema.
    seed [DUMP]     Inject a pricing dump into MySQL.
                    DUMP defaults to mysql/testdata/2023-02-23-pricing.sql.gz.
    db-cli          Open an interactive mysql shell against the configured DB.
    db-wait         Block until MySQL accepts connections.

TESTS
    test-unit       go test -short ./...
                    Fast — skips packages that touch MySQL (e2e, etc.).
    test-pkg PKG    go test -short ./PKG/...     e.g. test-pkg aws/terraform
    test-e2e        DB up (unless --no-db) → migrate → go test ./e2e/...
                    Required for the milestone goldens (SuccessLambda,
                    SuccessDynamoDB, SuccessAPIGateway, SuccessCloudFront,
                    SuccessRoute53).
    test            test-unit + test-e2e (full pipeline).

RUN
    estimate PLAN   Estimate the cost of a Terraform plan file.
    shell           Drop into a bash shell inside the runner container.
    run -- CMD ...  Run an arbitrary command inside the runner.

EXTERNAL DATABASE
  By default the script uses the bundled MySQL at 172.44.0.2:3306. To run
  against an external MySQL (e.g. RDS, a local host MySQL, a different
  container), set ANY of:

    TERRACOST_DSN              Full Go MySQL DSN. Overrides everything.
    TERRACOST_DB_HOST          Hostname or IP (must be reachable from
                               INSIDE the runner container — use
                               host.docker.internal on macOS/Windows).
    TERRACOST_DB_PORT          Defaults to 3306.
    TERRACOST_DB_USER          Defaults to root.
    TERRACOST_DB_PASSWORD      Defaults to terracost.
    TERRACOST_DB_NAME          Defaults to terracost_test.
    TERRACOST_NO_BUNDLED_DB=1  Skip starting the bundled MySQL.

  The script auto-detects external mode when TERRACOST_DB_HOST is not the
  bundled default OR when --no-db / TERRACOST_NO_BUNDLED_DB=1 is set.
  In external mode `up`, `test-e2e`, `db-wait`, etc. all skip the
  bundled container and talk to the configured host.

NOTES
  • From the host you can reach the bundled MySQL at 127.0.0.1:33060
    (useful for GUI tools).
  • The runner mounts the working tree at /workspace and persists the
    Go module + build cache in named volumes for fast incremental runs.

EXAMPLES
    ./scripts/dev.sh build
    ./scripts/dev.sh test-unit
    ./scripts/dev.sh test-e2e

    # Just the DB (e.g. for inspecting with a GUI):
    ./scripts/dev.sh db-up
    ./scripts/dev.sh db-cli

    # External DB on the host (macOS/Windows):
    TERRACOST_DB_HOST=host.docker.internal \
    TERRACOST_DB_PORT=3306 \
      ./scripts/dev.sh --no-db test-e2e
EOF
}

cmd_build() {
  "${DC[@]}" build "${RUNNER_SVC}"
}

cmd_up() {
  if is_external_db; then
    "${DC[@]}" up -d "${RUNNER_SVC}"
  else
    "${DC[@]}" up -d "${DB_SVC}" "${RUNNER_SVC}"
  fi
}

cmd_down() {
  "${DC[@]}" down
}

cmd_clean() {
  "${DC[@]}" down -v --remove-orphans
}

cmd_db_up() {
  if is_external_db; then
    echo "error: external DB mode is active (TERRACOST_DB_HOST=${TERRACOST_DB_HOST}); refusing to start bundled MySQL." >&2
    echo "       unset TERRACOST_DB_HOST/TERRACOST_DSN/TERRACOST_NO_BUNDLED_DB or omit --no-db to use the bundled DB." >&2
    exit 64
  fi
  "${DC[@]}" up -d "${DB_SVC}"
}

cmd_db_down() {
  "${DC[@]}" stop "${DB_SVC}" || true
  "${DC[@]}" rm -f "${DB_SVC}" || true
}

cmd_ps() {
  "${DC[@]}" ps
}

cmd_logs() {
  if [[ $# -eq 0 ]]; then
    "${DC[@]}" logs --tail=200 -f
  else
    "${DC[@]}" logs --tail=200 -f "$@"
  fi
}

cmd_compile() {
  exec_in_runner go build ./...
}

cmd_vet() {
  exec_in_runner go vet ./...
}

cmd_lint() {
  exec_in_runner bash -lc 'make lint'
}

cmd_generate() {
  exec_in_runner bash -lc 'make generate'
}

cmd_tidy() {
  exec_in_runner go mod tidy
}

cmd_migrate() {
  exec_in_runner go run scripts/migrate.go
}

# Run mysql/mysqladmin against the configured DB. In bundled mode we
# `exec` into the database container; in external mode we run the client
# from inside the runner (which has mysql-client installed) so the
# network/credentials match what the Go code will use.
mysql_in() {
  if is_external_db; then
    exec_in_runner mysql \
      -h"${TERRACOST_DB_HOST}" -P"${TERRACOST_DB_PORT}" \
      -u"${TERRACOST_DB_USER}" -p"${TERRACOST_DB_PASSWORD}" \
      "$@"
  else
    ensure_db_up
    "${DC[@]}" exec "${DB_SVC}" \
      mysql -uroot -pterracost "$@"
  fi
}

mysql_in_T() {
  if is_external_db; then
    exec_in_runner mysql \
      -h"${TERRACOST_DB_HOST}" -P"${TERRACOST_DB_PORT}" \
      -u"${TERRACOST_DB_USER}" -p"${TERRACOST_DB_PASSWORD}" \
      "$@"
  else
    ensure_db_up
    "${DC[@]}" exec -T "${DB_SVC}" \
      mysql -uroot -pterracost "$@"
  fi
}

cmd_seed() {
  local dump="${1:-mysql/testdata/2023-02-23-pricing.sql.gz}"
  if [[ ! -f "${dump}" ]]; then
    echo "error: dump file not found: ${dump}" >&2
    exit 1
  fi
  echo ">> seeding MySQL (${TERRACOST_DB_HOST}:${TERRACOST_DB_PORT}/${TERRACOST_DB_NAME}) from ${dump}"
  gunzip -c "${dump}" | mysql_in_T "${TERRACOST_DB_NAME}"
}

cmd_db_cli() {
  mysql_in "${TERRACOST_DB_NAME}"
}

cmd_db_wait() {
  echo -n ">> waiting for MySQL @ ${TERRACOST_DB_HOST}:${TERRACOST_DB_PORT} "
  for _ in {1..60}; do
    if is_external_db; then
      if exec_in_runner mysqladmin ping \
           -h"${TERRACOST_DB_HOST}" -P"${TERRACOST_DB_PORT}" \
           -u"${TERRACOST_DB_USER}" -p"${TERRACOST_DB_PASSWORD}" \
           --silent >/dev/null 2>&1; then
        echo "ready."
        return 0
      fi
    else
      ensure_db_up
      if "${DC[@]}" exec -T "${DB_SVC}" \
           mysqladmin ping -h 127.0.0.1 -uroot -pterracost --silent >/dev/null 2>&1; then
        echo "ready."
        return 0
      fi
    fi
    echo -n "."
    sleep 1
  done
  echo
  echo "error: MySQL did not become ready in 60s" >&2
  exit 1
}

cmd_test_unit() {
  exec_in_runner go test -short ./...
}

cmd_test_pkg() {
  local pkg="${1:-}"
  if [[ -z "${pkg}" ]]; then
    echo "usage: ./scripts/dev.sh test-pkg <package>" >&2
    exit 64
  fi
  exec_in_runner go test -short "./${pkg}/..."
}

cmd_test_e2e() {
  cmd_db_wait
  cmd_migrate
  exec_in_runner go test ./e2e/...
}

cmd_test() {
  cmd_test_unit
  cmd_test_e2e
}

cmd_estimate() {
  local plan="${1:-}"
  if [[ -z "${plan}" ]]; then
    echo "usage: ./scripts/dev.sh estimate <plan.json>" >&2
    exit 64
  fi
  if [[ ! -f "${plan}" ]]; then
    echo "error: plan file not found: ${plan}" >&2
    exit 1
  fi
  cmd_db_wait
  cmd_migrate
  echo ">> running examples/terracost.go against ${plan}"
  exec_in_runner go run ./examples/terracost.go "${plan}"
}

cmd_shell() {
  ensure_up
  "${DC[@]}" exec \
    -e TERRACOST_DSN \
    -e TERRACOST_DB_HOST \
    -e TERRACOST_DB_PORT \
    -e TERRACOST_DB_USER \
    -e TERRACOST_DB_PASSWORD \
    -e TERRACOST_DB_NAME \
    "${RUNNER_SVC}" bash
}

cmd_run() {
  if [[ "${1:-}" == "--" ]]; then shift; fi
  if [[ $# -eq 0 ]]; then
    echo "usage: ./scripts/dev.sh run -- <command> [args...]" >&2
    exit 64
  fi
  exec_in_runner "$@"
}

# ──────────────────────── dispatch ────────────────────────

main() {
  local cmd="${1:-help}"
  [[ $# -gt 0 ]] && shift || true

  case "${cmd}" in
    help|-h|--help)  cmd_help ;;
    build)           cmd_build ;;
    up)              cmd_up ;;
    down)            cmd_down ;;
    clean)           cmd_clean ;;
    db-up)           cmd_db_up ;;
    db-down)         cmd_db_down ;;
    ps)              cmd_ps ;;
    logs)            cmd_logs "$@" ;;
    compile)         cmd_compile ;;
    vet)             cmd_vet ;;
    lint)            cmd_lint ;;
    generate)        cmd_generate ;;
    tidy)            cmd_tidy ;;
    migrate)         cmd_migrate ;;
    seed)            cmd_seed "$@" ;;
    db-cli)          cmd_db_cli ;;
    db-wait)         cmd_db_wait ;;
    test-unit)       cmd_test_unit ;;
    test-pkg)        cmd_test_pkg "$@" ;;
    test-e2e)        cmd_test_e2e ;;
    test)            cmd_test ;;
    estimate)        cmd_estimate "$@" ;;
    shell)           cmd_shell ;;
    run)             cmd_run "$@" ;;
    *)
      echo "unknown command: ${cmd}" >&2
      echo "run './scripts/dev.sh help' for the command list." >&2
      exit 64
      ;;
  esac
}

main "$@"
