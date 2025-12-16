#!/usr/bin/env bash
# Shared logging + command helpers for bv e2e scripts.
# Goals: timestamped logs, captured stdout/stderr per step, jq-friendly outputs.
set -euo pipefail

BV_E2E_LOG_DIR="${BV_E2E_LOG_DIR:-$(pwd)/.e2e-logs}"
mkdir -p "$BV_E2E_LOG_DIR"

ts() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

log() {
  echo "[e2e $(ts)] $*" >&2
}

# run <name> <cmd...>
# Captures stdout/stderr to files <name>.out / <name>.err in BV_E2E_LOG_DIR.
run() {
  local name="$1"; shift
  log "RUN  $name: $*"
  local out="$BV_E2E_LOG_DIR/${name}.out"
  local err="$BV_E2E_LOG_DIR/${name}.err"
  if "$@" >"$out" 2>"$err"; then
    log "OK   $name"
  else
    local code=$?
    log "FAIL $name (exit $code) — stdout:$out stderr:$err"
    if command -v jq >/dev/null; then
      log "Tip: jq '.' $out | head"
    fi
    return $code
  fi
}

# jq_field <file> <jq expression>
# Convenience for quick assertions without verbose harness code.
jq_field() {
  local file="$1"; shift
  local expr="$*"
  jq -e "$expr" "$file" >/dev/null
}

# section <title>
section() {
  log "----- $* -----"
}
