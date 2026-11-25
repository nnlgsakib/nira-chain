#!/usr/bin/env bash
# Software-upgrade rehearsal with a real binary switch (G-OPS-1, G-TEST-5):
#   1. fresh localnet on the release binary (no "rehearsal" handler)
#   2. governance passes MsgSoftwareUpgrade{name: rehearsal, height: H}
#   3. the release binary halts at H with "UPGRADE NEEDED" (no fork, no skip)
#   4. operator switches to the rehearsal binary (-tags rehearsal), which applies the
#      handler at H and continues; state and the EVM survive the switch
# Usage: tests/upgrade/rehearsal.sh   (needs `make install` and build/nirad-rehearsal; no other node running)
set -euo pipefail

REPO="$(cd "$(dirname "$0")/../.." && pwd)"
OLD=${OLD:-$(command -v nirad)}
NEW=${NEW:-$REPO/build/nirad-rehearsal}
HOME_DIR=$HOME/.nira
CHAIN_ID=nira_47201-1
LOG=$HOME_DIR/rehearsal-node.log
KR=(--keyring-backend test --home "$HOME_DIR")
PASSED=0
fail() { echo "FAIL: $*" >&2; stop; exit 1; }
ok() { PASSED=$((PASSED + 1)); echo "ok  $*"; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d': d}))" "$1"; }
rest() { curl -sf -m 10 "http://127.0.0.1:1317$1"; }
height() { curl -sf -m 5 http://127.0.0.1:26657/status | json "int(d['result']['sync_info']['latest_block_height'])"; }
start() { nohup "$1" start --home "$HOME_DIR" >>"$LOG" 2>&1 & echo $! >"$HOME_DIR/node.pid"; }
stop() { [ -f "$HOME_DIR/node.pid" ] && kill "$(cat "$HOME_DIR/node.pid")" 2>/dev/null || true; sleep 2; }
wait_up() { for _ in $(seq 1 60); do height >/dev/null 2>&1 && curl -sf -m 2 -o /dev/null http://127.0.0.1:1317/cosmos/base/tendermint/v1beta1/blocks/latest && return 0; sleep 1; done; fail "node not up"; }

[ -x "$NEW" ] || fail "build the rehearsal binary first: go build -tags rehearsal -o build/nirad-rehearsal ./cmd/nirad"
BIN=$OLD "$REPO/scripts/localnet.sh" >/dev/null
: >"$LOG"
start "$OLD"; wait_up; sleep 3

gas_price() { local bf; bf=$(rest /cosmos/evm/feemarket/v1/base_fee | json "d['base_fee']"); python3 -c "import math; print(str(max(1, math.ceil(float('$bf') * 2))) + 'anira')"; }
txh() { python3 -c "import json,sys
for l in sys.stdin:
    l=l.strip()
    if l.startswith('{'): print(json.loads(l)['txhash'])"; }
tx() { local from=$1; shift
  h=$("$OLD" tx "$@" --from "$from" --chain-id $CHAIN_ID "${KR[@]}" --gas auto --gas-adjustment 1.6 --gas-prices "$(gas_price)" -y --output json 2>/dev/null | txh)
  for _ in $(seq 1 20); do sleep 1; r=$("$OLD" q tx "$h" --output json --home "$HOME_DIR" 2>/dev/null) && { echo "$r"; return 0; }; done; fail "tx $h"; }

GOV=$("$OLD" q auth module-account gov --output json --home "$HOME_DIR" | json "(d['account']['value'] if 'value' in d['account'] else d['account']['base_account'])['address']")
DEV1=$("$OLD" keys show dev1 -a "${KR[@]}")
UH=$(( $(height) + 40 ))
P=$(mktemp)
python3 - "$P" "$GOV" "$UH" <<'PY'
import json, sys
p, gov, h = sys.argv[1:]
json.dump({"messages": [{"@type": "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade", "authority": gov,
                         "plan": {"name": "rehearsal", "height": h, "info": "rehearsal"}}],
           "deposit": "10000000000000000000000anira", "title": "Upgrade rehearsal", "summary": "binary switch"}, open(p, "w"))
PY
id=$(tx dev1 gov submit-proposal "$P" | json "[a['value'] for e in d['events'] if e['type']=='submit_proposal' for a in e['attributes'] if a['key']=='proposal_id'][0]")
tx validator gov vote "$id" yes >/dev/null
for _ in $(seq 1 60); do
  [ "$(rest "/cosmos/gov/v1/proposals/$id" | json "d['proposal']['status']")" = "PROPOSAL_STATUS_PASSED" ] && break; sleep 1
done
plan=$(rest /cosmos/upgrade/v1beta1/current_plan | json "d['plan']['name']+'@'+d['plan']['height']")
[ "$plan" = "rehearsal@$UH" ] || fail "current plan $plan"
ok "proposal $id passed: plan rehearsal@$UH scheduled"

bal_before=$(rest "/cosmos/bank/v1beta1/balances/$DEV1/by_denom?denom=anira" | json "d['balance']['amount']")
for _ in $(seq 1 120); do grep -q "UPGRADE \"rehearsal\" NEEDED at height: $UH" "$LOG" && break; sleep 1; done
grep -q "UPGRADE \"rehearsal\" NEEDED at height: $UH" "$LOG" || fail "release binary did not halt at $UH"
last=$(height); sleep 4
[ "$(height)" = "$last" ] || fail "chain advanced past the upgrade height on the old binary"
[ "$last" = "$UH" ] || [ "$last" = "$((UH - 1))" ] || fail "halted at $last, expected $UH"
ok "release binary halted at height $UH (chain stopped at $last): UPGRADE NEEDED"

stop
start "$NEW"; wait_up
for _ in $(seq 1 60); do [ "$(height)" -gt "$((UH + 3))" ] && break; sleep 1; done
[ "$(height)" -gt "$((UH + 3))" ] || fail "chain did not continue after the binary switch"
applied=$(rest /cosmos/upgrade/v1beta1/applied_plan/rehearsal | json "int(d['height'])")
[ "$applied" = "$UH" ] || fail "applied at $applied, planned $UH"
grep -q "applying upgrade \"rehearsal\" at height: $UH" "$LOG" || fail "handler log line missing"
ok "rehearsal binary applied the upgrade handler at height $UH and produced blocks to $(height)"

bal_after=$(rest "/cosmos/bank/v1beta1/balances/$DEV1/by_denom?denom=anira" | json "d['balance']['amount']")
[ "$bal_after" = "$bal_before" ] || fail "state changed across the switch: $bal_before → $bal_after"
evm_h=$(python3 -c "print(int('$(curl -sf -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' http://127.0.0.1:8545 | json "d['result']")', 16))")
[ "$evm_h" -gt "$UH" ] || fail "EVM JSON-RPC at $evm_h"
ok "state preserved across the switch (dev1 balance), EVM JSON-RPC serving height $evm_h"

stop; rm -f "$P"
echo
echo "$PASSED checks passed"
