#!/usr/bin/env bash
# Cosmos-side e2e for x/community on a running localnet (scripts/localnet.sh
# sets 20-block epochs). Drives the real community node client
# (onion-init → register-node → prove) and cross-checks escrow through the EVM.
set -euo pipefail

BIN=${BIN:-nirad}
HOME_DIR=${HOME_DIR:-$HOME/.nira}
CHAIN_ID=${CHAIN_ID:-nira_93000-1}
REST=${REST:-http://127.0.0.1:1317}
CMT=${CMT:-http://127.0.0.1:26657}
EVM_RPC=${EVM_RPC:-http://127.0.0.1:8545}
KR=(--keyring-backend test --home "$HOME_DIR")
PASSED=0
KEY=${KEY:-dev2}

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { PASSED=$((PASSED + 1)); echo "ok  $*"; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d': d}))" "$1"; }
rest() { curl -sf -m 10 "$REST$1"; }
q() { $BIN q community "$@" --output json --home "$HOME_DIR"; }
height() { curl -sf -m 5 "$CMT/status" | json "d['result']['sync_info']['latest_block_height']"; }
wait_height() { while [ "$(height)" -lt "$1" ]; do sleep 1; done; }
gas_price() {
  local bf; bf=$(rest /cosmos/evm/feemarket/v1/base_fee | json "d['base_fee']")
  python3 -c "import math; print(str(max(1, math.ceil(float('$bf') * 2))) + 'anira')"
}
TXF=(--from "$KEY" --chain-id "$CHAIN_ID" "${KR[@]}" --gas auto --gas-adjustment 1.5 -y --output json)
# tx_ok <txhash>: wait for inclusion, require code 0
tx_ok() {
  for _ in $(seq 1 20); do
    sleep 1
    if res=$($BIN q tx "$1" --output json --home "$HOME_DIR" 2>/dev/null); then
      [ "$(echo "$res" | json "d['code']")" = "0" ] || fail "tx $1: $(echo "$res" | json "d['raw_log']")"
      echo "$res"; return 0
    fi
  done
  fail "tx $1 not included"
}
txhash() { python3 -c "import json,sys
for line in sys.stdin:
    line=line.strip()
    if line.startswith('{'):
        print(json.loads(line)['txhash'])"; }

OP=$($BIN keys show "$KEY" -a "${KR[@]}")

# 1. Onion + registration through the CLI
TIP=$($BIN tx community onion-init --length 10000 --overwrite --home "$HOME_DIR" | awk '/onion_tip/{print $2}')
[ ${#TIP} = 64 ] || fail "onion tip: $TIP"
hash=$($BIN tx community register-node e2e-node 1000000000000000000000anira "$TIP" 10000 "${TXF[@]}" \
  --gas-prices "$(gas_price)" 2>/dev/null | txhash)
tx_ok "$hash" >/dev/null
status=$(q node "$OP" | json "d['node']['status']")
[ "$status" = "NODE_STATUS_ACTIVE" ] || fail "status $status"
bond=$(rest "/nira/community/v1/nodes/$OP" | json "d['node']['bond']")
[ "$bond" = "1000000000000000000000" ] || fail "REST bond $bond"
ok "onion-init + register-node via CLI; node ACTIVE via CLI and REST"

# 2. Escrow visible through the EVM interface (single state)
MOD_HEX=0x$($BIN q auth module-account community --output json --home "$HOME_DIR" \
  | json "d['account']['value']['address'] if 'value' in d['account'] else d['account']['base_account']['address']" \
  | xargs -I{} $BIN debug addr {} 2>/dev/null | awk '/Address \(hex\)/{print $3}')
evm_bal=$(python3 -c "print(int('$(curl -sf -X POST -H 'content-type: application/json' \
  --data "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"eth_getBalance\",\"params\":[\"$MOD_HEX\",\"latest\"]}" "$EVM_RPC" \
  | json "d['result']")', 16))")
python3 -c "assert $evm_bal >= 1000000000000000000000" || fail "EVM view of escrow $evm_bal"
ok "bond escrow visible via eth_getBalance of community module account ($MOD_HEX)"

# 3. Wait until the node is eligible (onion base epoch), then prove
BASE=$(q node "$OP" | json "d['node']['onion_base_epoch']")
while [ "$(q epoch | json "int(d['epoch']['number'])")" -lt "$BASE" ]; do sleep 2; done
# give the epoch a block so app hashes for the task window exist
sleep 2
out=$($BIN tx community prove "${TXF[@]}" --gas-prices "$(gas_price)" 2>&1) || fail "prove: $out"
echo "$out" | grep -q "submitted 2 proof" || fail "prove output: $out"
for h in $(echo "$out" | txhash); do tx_ok "$h" >/dev/null; done
units=$(q node "$OP" | json "d.get('units_this_epoch', 0)")
[ "$units" = "2" ] || fail "units_this_epoch $units"
EPOCH=$(q epoch | json "d['epoch']['number']")
ok "selected + 2 real ICS-23 state proofs accepted in epoch $EPOCH"

# 3b. Operator status command (G-COMM-3): selection computed locally from the onion, proven work shown
stj=$($BIN tx community status --from "$KEY" "${KR[@]}" --chain-id "$CHAIN_ID" --output json 2>/dev/null)
[ "$(echo "$stj" | json "d['status']")" = "NODE_STATUS_ACTIVE" ] || fail "status: $stj"
[ "$(echo "$stj" | json "d['selected_this_epoch']")" = "True" ] || fail "status not selected: $stj"
[ "$(echo "$stj" | json "int(d['tasks_proven_this_epoch'])")" = "2" ] || fail "status tasks: $stj"
[ "$(echo "$stj" | json "len(d['actions_required'])")" = "0" ] || fail "unexpected actions: $stj"
ok "tx community status: ACTIVE, selected (from local onion), 2/2 tasks proven, no action required"

# 4. Re-proving the same tasks is rejected (no double pay)
out=$($BIN tx community prove "${TXF[@]}" --gas-prices "$(gas_price)" 2>&1) || true
for h in $(echo "$out" | txhash); do
  res=$($BIN q tx "$h" --output json --home "$HOME_DIR" 2>/dev/null || true)
  [ -z "$res" ] || [ "$(echo "$res" | json "d['code']")" != "0" ] || fail "duplicate proof accepted"
done
ok "duplicate proofs not accepted"

# 5. Settlement at epoch end pays the node (event in block results)
SETTLE=$(q epoch | json "d['next_epoch_height']")
wait_height "$((SETTLE + 1))"
paid=$(curl -sf -m 10 "$CMT/block_results?height=$SETTLE" | python3 -c "
import json,sys
d=json.load(sys.stdin)['result']
for e in (d.get('finalize_block_events') or []):
    if e['type']=='community_reward':
        a={x['key']:x['value'] for x in e['attributes']}
        if a.get('operator')=='$OP': print(a['amount']); break
")
[ -n "$paid" ] && python3 -c "assert int('$paid') > 0" || fail "no community_reward for $OP at $SETTLE"
total=$(q node "$OP" | json "d['node']['rewards_total']")
[ "$total" = "$paid" ] || fail "rewards_total $total != paid $paid (G-COMM-3)"
ok "epoch $EPOCH settled at height $SETTLE: paid $paid anira for verified work"

# 6. Deregister → UNBONDING → bond refunded after unbonding_blocks
hash=$($BIN tx community deregister-node "${TXF[@]}" --gas-prices "$(gas_price)" 2>/dev/null | txhash)
tx_ok "$hash" >/dev/null
[ "$(q node "$OP" | json "d['node']['status']")" = "NODE_STATUS_UNBONDING" ] || fail "not unbonding"
DONE=$(q node "$OP" | json "d['node']['unbond_complete_height']")
wait_height "$((DONE + 1))"
if q node "$OP" >/dev/null 2>&1; then fail "node still present after unbonding"; fi
ok "deregister-node → UNBONDING → bond released at height $DONE"

echo
echo "$PASSED checks passed"
