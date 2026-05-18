#!/usr/bin/env bash
set -euo pipefail

OLD_BIN=${OLD_BIN:-/tmp/liszt-old}
BASE=${BASE:-/tmp/liszt-baseline}
SRC_DIR=${SRC_DIR:-$(pwd)}

rm -rf "$BASE"
mkdir -p "$BASE"
cp "$SRC_DIR/repos.toml" "$BASE/repos.toml"
cp -R "$SRC_DIR/tmp" "$BASE/tmp"

cd "$BASE"
rm -f liszt.toml liszt.lock

run() {
  local label=$1
  shift
  set +e
  "$OLD_BIN" "$@" > "old-$label.out" 2> "old-$label.err"
  local rc=$?
  set -e
  echo "exit=$rc" > "old-$label.code"
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
cp liszt.toml old-liszt.toml
cp liszt.lock old-liszt.lock

run outdated      outdated
run usage-empty
run usage-bogus   bogus
run usage-skill   skill

echo "baseline captured at $BASE"
