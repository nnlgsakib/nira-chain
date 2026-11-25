#!/usr/bin/env bash
# Genesis export → import with the production binary (state migration path for
# "genesis restart" upgrades and chain forks). Regression for the InitGenesis-order
# bug: x/staking hooks read x/policy params while loading delegations.
#   tests/upgrade/export_import.sh     (no other node running)
set -euo pipefail

REPO="$(cd "$(dirname "$0")/../.." && pwd)"
BIN=${BIN:-$(command -v nirad)}
HOME_DIR=$HOME/.nira
LOG=$HOME_DIR/export-import.log
KR=(--keyring-backend test --home "$HOME_DIR")
PASSED=0
fail() { echo "FAIL: $*" >&2; stop; exit 1; }
ok() { PASSED=$((PASSED + 1)); echo "ok  $*"; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d': d}))" "$1"; }
rest() { curl -sf -m 10 "http://127.0.0.1:1317$1"; }
height() { curl -sf -m 5 http://127.0.0.1:26657/status | json "int(d['result']['sync_info']['latest_block_height'])"; }
start() { nohup "$BIN" start --home "$HOME_DIR" >>"$LOG" 2>&1 & echo $! >"$HOME_DIR/node.pid"; }
stop() { [ -f "$HOME_DIR/node.pid" ] && kill "$(cat "$HOME_DIR/node.pid")" 2>/dev/null || true; sleep 3; }
wait_up() { for _ in $(seq 1 60); do height >/dev/null 2>&1 && curl -sf -m 2 -o /dev/null http://127.0.0.1:1317/cosmos/base/tendermint/v1beta1/blocks/latest && return 0; sleep 1; done; fail "node not up"; }
txh() { python3 -c "import json,sys
for l in sys.stdin:
    l=l.strip()
    if l.startswith('{'): print(json.loads(l)['txhash'])"; }
tx() { local from=$1; shift
  h=$("$BIN" tx "$@" --from "$from" --chain-id nira_47201-1 "${KR[@]}" --gas auto --gas-adjustment 1.6 --gas-prices 2000000000anira -y --output json 2>/dev/null | txh)
  for _ in $(seq 1 20); do sleep 1; r=$("$BIN" q tx "$h" --output json --home "$HOME_DIR" 2>/dev/null) && { [ "$(echo "$r" | json "d['code']")" = 0 ] || fail "tx $*"; return 0; }; done; fail "tx $h"; }

BIN=$BIN "$REPO/scripts/localnet.sh" >/dev/null
: >"$LOG"; start; wait_up; sleep 2

# 1. build state that exercises the hooks and modules on import
VALOPER=$(rest /cosmos/staking/v1beta1/validators | json "d['validators'][0]['operator_address']")
DEV1=$("$BIN" keys show dev1 -a "${KR[@]}"); DEV2=$("$BIN" keys show dev2 -a "${KR[@]}")
tx dev1 staking delegate "$VALOPER" 5000000000000000000000anira
TIP=$("$BIN" tx community onion-init --length 1000 --overwrite --home "$HOME_DIR" | awk '/onion_tip/{print $2}')
tx dev2 community register-node export-test 1000000000000000000000anira "$TIP" 1000
sleep 3
snap() {
  echo "$(rest "/cosmos/bank/v1beta1/balances/$DEV1/by_denom?denom=anira" | json "d['balance']['amount']")"
  echo "$(rest "/cosmos/staking/v1beta1/validators/$VALOPER/delegations/$DEV1" | json "d['delegation_response']['balance']['amount']")"
  echo "$(rest "/nira/community/v1/nodes/$DEV2" | json "d['node']['status']+' '+d['node']['bond']")"
  echo "$(rest /nira/policy/v1/params | json "d['params']['min_self_bond']")"
}
before=$(snap)

# 2. stop, export, reset, import
stop
H0=$("$BIN" status --home "$HOME_DIR" 2>/dev/null | json "int(d['sync_info']['latest_block_height'])" 2>/dev/null || true)
"$BIN" export --home "$HOME_DIR" >"$HOME_DIR/exported.json" 2>>"$LOG" || fail "export failed (see $LOG)"
EXP_H=$(python3 -c "import json;print(json.load(open('$HOME_DIR/exported.json'))['initial_height'])")
"$BIN" comet unsafe-reset-all --home "$HOME_DIR" >/dev/null 2>&1
cp "$HOME_DIR/exported.json" "$HOME_DIR/config/genesis.json"
"$BIN" genesis validate --home "$HOME_DIR" >/dev/null || fail "exported genesis does not validate"
python3 - "$HOME_DIR/exported.json" <<'PY' || fail "exported supply != Σ balances"
import json, sys
g = json.load(open(sys.argv[1]))["app_state"]["bank"]
total = sum(int(c["amount"]) for b in g["balances"] for c in b["coins"] if c["denom"] == "anira")
sup = [int(c["amount"]) for c in g["supply"] if c["denom"] == "anira"]
assert sup and sup[0] == total, (sup, total)
PY
ok "exported state at height $((EXP_H - 1)) validates as a new genesis (initial_height $EXP_H); supply = Σ balances"

start; wait_up
for _ in $(seq 1 30); do [ "$(height)" -gt "$((EXP_H + 3))" ] && break; sleep 1; done
[ "$(height)" -gt "$((EXP_H + 3))" ] || fail "chain did not continue after import (see $LOG)"
grep -q "panic" "$LOG" && fail "panic in node log"
ok "imported chain continues from height $EXP_H (now $(height)) with no panic"

after=$(snap)
[ "$before" = "$after" ] || { echo "before:"; echo "$before"; echo "after:"; echo "$after"; fail "state differs across export/import"; }
ok "balances, delegation, community node (status + bond) and policy params identical after import"

evm_h=$(python3 -c "print(int('$(curl -sf -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' http://127.0.0.1:8545 | json "d['result']")', 16))")
[ "$evm_h" -ge "$EXP_H" ] || fail "EVM JSON-RPC height $evm_h"
ok "EVM JSON-RPC serves the imported chain (height $evm_h)"

stop
echo
echo "$PASSED checks passed"
