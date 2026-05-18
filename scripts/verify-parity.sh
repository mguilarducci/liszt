#!/usr/bin/env bash
set -uo pipefail

B=${BASE:-/tmp/liszt-baseline}
N=${RUN:-/tmp/liszt-newrun}
FAIL=0

note() { echo "FAIL: $*"; FAIL=1; }

DET_STDOUT="plugin-list skill-list agent-list command-list outdated \
            install-skill install-agent install-plugin install-command install-hook \
            usage-empty usage-bogus usage-skill"
for f in $DET_STDOUT; do
  if ! diff -u "$B/old-$f.out" "$N/new-$f.out" > /dev/null; then
    note "stdout drift: $f"
    diff -u "$B/old-$f.out" "$N/new-$f.out"
  fi
done

for f in liszt.toml liszt.lock; do
  if ! diff -u "$B/old-$f" "$N/new-$f" > /dev/null; then
    note "$f drift"
    diff -u "$B/old-$f" "$N/new-$f"
  fi
done

DET_CODE="plugin-list skill-list agent-list command-list outdated usage-empty usage-bogus usage-skill"
for f in $DET_CODE; do
  if ! diff -u "$B/old-$f.code" "$N/new-$f.code" > /dev/null; then
    note "exit-code drift: $f"
    diff -u "$B/old-$f.code" "$N/new-$f.code"
  fi
done

for k in hook mcp lsp; do
  sort "$B/old-$k-list.out" > "/tmp/cmp-old-$k"
  sort "$N/new-$k-list.out" > "/tmp/cmp-new-$k"
  if ! diff -u "/tmp/cmp-old-$k" "/tmp/cmp-new-$k" > /dev/null; then
    note "set drift: $k-list"
    diff -u "/tmp/cmp-old-$k" "/tmp/cmp-new-$k"
  fi
done

if [ "$FAIL" -ne 0 ]; then
  echo "PARITY CHECK FAILED"
  exit 1
fi
echo "PARITY CHECK PASSED"
