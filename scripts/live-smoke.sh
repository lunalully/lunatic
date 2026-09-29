#!/usr/bin/env bash
# live-smoke.sh - live smoke test of every default, no-key Lunatic source.
#
# NOTE: this script could NOT be run during development: the build environment's
# egress proxy blocked live requests to the providers, so all adapters were
# tested offline only (httptest fixtures). Run it on your own machine to verify
# the adapters against the real services.
#
# It builds lunatic, then runs each default source that needs no API key one at
# a time (sequentially, with a pause in between so provider rate limits are
# respected) against a well-known public program domain, and prints a table:
#   SOURCE  EXIT  COUNT
# Exit codes: 0 ok, 2 source failed (see stderr in the log), 3 partial, 1 usage.
#
# Usage: scripts/live-smoke.sh [-d domain] [-p pause_seconds] [-t timeout_seconds]
#   defaults: -d hackerone.com  -p 5  -t 60
# Stdout (.out) and stderr (.stderr) of each run are saved under $OUT_DIR (default: ./smoke-logs).
set -u

DOMAIN="hackerone.com"; PAUSE=5; TIMEOUT=60
while getopts "d:p:t:h" opt; do
  case $opt in
    d) DOMAIN=$OPTARG ;;
    p) PAUSE=$OPTARG ;;
    t) TIMEOUT=$OPTARG ;;
    *) sed -n '2,17p' "$0"; exit 1 ;;
  esac
done

cd "$(dirname "$0")/.." || exit 1
OUT_DIR=${OUT_DIR:-smoke-logs}
mkdir -p "$OUT_DIR" bin
go build -o bin/lunatic ./cmd/lunatic || { echo "build failed" >&2; exit 1; }
BIN=./bin/lunatic

# Ignore any keys from the user's environment/config so only no-key sources are picked.
export XDG_CONFIG_HOME; XDG_CONFIG_HOME=$(mktemp -d)
for v in $(env | awk -F= '/^LUNATIC_/{print $1}'); do unset "$v"; done

# NAME AUTH STATUS DEFAULT NOTE  -> default sources that are ready without a key
mapfile -t SOURCES < <($BIN --list-sources | awk 'NR>1 && $2!="required" && $3=="ready" && $4=="yes" {print $1}')
if [ ${#SOURCES[@]} -eq 0 ]; then echo "no no-key default sources found" >&2; exit 1; fi

printf '%-16s %-5s %s\n' SOURCE EXIT COUNT
fail=0
for src in "${SOURCES[@]}"; do
  log="$OUT_DIR/$src.stderr"
  $BIN -d "$DOMAIN" -s "$src" --timeout "$TIMEOUT" -v --no-color >"$OUT_DIR/$src.out" 2>"$log"
  code=$?
  count=$(wc -l <"$OUT_DIR/$src.out" | tr -d ' ')
  printf '%-16s %-5s %s\n' "$src" "$code" "$count"
  [ "$code" -eq 0 ] || fail=$((fail+1))
  sleep "$PAUSE"
done
echo
echo "${#SOURCES[@]} source(s) tested, $fail with non-zero exit. Logs: $OUT_DIR/<source>.stderr"
[ "$fail" -eq 0 ]
