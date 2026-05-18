#!/usr/bin/env bash
set -euo pipefail

NEW_BIN=${NEW_BIN:-/tmp/liszt-new}
BASE=${BASE:-/tmp/liszt-baseline}
RUN=${RUN:-/tmp/liszt-newrun}

rm -rf "$RUN"
mkdir -p "$RUN"
cp "$BASE/repos.toml" "$RUN/repos.toml"
cp -R "$BASE/tmp" "$RUN/tmp"

cd "$RUN"
rm -f liszt.toml liszt.lock

run() {
  local label=$1
  shift
  set +e
  "$NEW_BIN" "$@" > "new-$label.out" 2> "new-$label.err"
  local rc=$?
  set -e
  echo "exit=$rc" > "new-$label.code"
}

run plugin-list plugin list
run skill-list skill list
run agent-list agent list
run command-list command list
run hook-list hook list
run mcp-list mcp list
run lsp-list lsp list

run install-skill   skill   install brainstorming   --flavor claude
run install-agent   agent   install code-reviewer   --flavor claude
run install-plugin  plugin  install hookify         --flavor claude
run install-command command install commit          --flavor copilot
run install-hook    hook    install SessionStart    --flavor copilot
cp liszt.toml new-liszt.toml
cp liszt.lock new-liszt.lock

run outdated      outdated
run usage-empty
run usage-bogus   bogus
run usage-skill   skill

echo "new captures at $RUN"
