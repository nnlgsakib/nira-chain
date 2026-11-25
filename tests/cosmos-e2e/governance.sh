#!/usr/bin/env bash
# Governance e2e on a running localnet (scripts/localnet.sh sets 20 s voting):
#   1. x/policy parameter change by proposal (validator votes via CLI, dev1 votes via EVM gov precompile)
#   2. false divergence alert → governance MsgResolveAlert(invalid) → reporter bond slashed (G-COMM-1)
#   (software-upgrade rehearsal with a real binary switch: tests/upgrade/rehearsal.sh)
set -euo pipefail

BIN=${BIN:-nirad}
HOME_DIR=${HOME_DIR:-$HOME/.nira}
CHAIN_ID=${CHAIN_ID:-nira_47201-1}
REST=${REST:-http://127.0.0.1:1317}
CMT=${CMT:-http://127.0.0.1:26657}
EVM_RPC=${EVM_RPC:-http://127.0.0.1:8545}
KR=(--keyring-backend test --home "$HOME_DIR")
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PASSED=0
fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { PASSED=$((PASSED + 1)); echo "ok  $*"; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d': d}))" "$1"; }
rest() { curl -sf -m 10 "$REST$1"; }
height() { curl -sf -m 5 "$CMT/status" | json "int(d['result']['sync_info']['latest_block_height'])"; }
gas_price() {
  local bf; bf=$(rest /cosmos/evm/feemarket/v1/base_fee | json "d['base_fee']")
  python3 -c "import math; print(str(max(1, math.ceil(float('$bf') * 2))) + 'anira')"
}
txhash() { python3 -c "import json,sys
for line in sys.stdin:
    line=line.strip()
    if line.startswith('{'):
        print(json.loads(line)['txhash'])"; }
tx_wait() {
  for _ in $(seq 1 20); do
    sleep 1
    if res=$($BIN q tx "$1" --output json --home "$HOME_DIR" 2>/dev/null); then
      [ "$(echo "$res" | json "d['code']")" = "0" ] || fail "tx $1: $(echo "$res" | json "d['raw_log']")"
      echo "$res"; return 0
    fi
  done
  fail "tx $1 not included"
}
tx() { # tx <from> <args...>
  local from=$1; shift
  $BIN tx "$@" --from "$from" --chain-id "$CHAIN_ID" "${KR[@]}" --gas auto --gas-adjustment 1.6 \
    --gas-prices "$(gas_price)" -y --output json 2>/dev/null | txhash | { read -r h; tx_wait "$h"; }
}
GOV=$($BIN q auth module-account gov --output json --home "$HOME_DIR" | json "(d['account'].get('value') or d['account'])['address'] if 'value' in d['account'] else d['account']['base_account']['address']")
DEPOSIT=10000000000000000000000anira   # = gov min_deposit (10 000 NIRA)

# propose <file> → proposal id; validator votes yes; waits until PASSED
propose_and_pass() {
  local file=$1 res id
  res=$(tx dev1 gov submit-proposal "$file")
  id=$(echo "$res" | json "[a['value'] for e in d['events'] if e['type']=='submit_proposal' for a in e['attributes'] if a['key']=='proposal_id'][0]")
  tx validator gov vote "$id" yes >/dev/null
  [ -n "${2:-}" ] && "$2" "$id"        # optional extra step while voting is open
  for _ in $(seq 1 60); do
    st=$(rest "/cosmos/gov/v1/proposals/$id" | json "d['proposal']['status']")
    [ "$st" = "PROPOSAL_STATUS_PASSED" ] && { echo "$id"; return 0; }
    [ "$st" = "PROPOSAL_STATUS_REJECTED" ] || [ "$st" = "PROPOSAL_STATUS_FAILED" ] && fail "proposal $id: $st"
    sleep 1
  done
  fail "proposal $id did not pass"
}
evm_vote() { # dev1 votes YES through the EVM gov precompile 0x…0805
  local id=$1 key
  key=$(echo y | $BIN keys unsafe-export-eth-key dev1 "${KR[@]}" 2>/dev/null | tail -1)
  (cd "$SCRIPT_DIR/../evm-e2e" && KEY=$key PID=$id node --input-type=module -e "
    import { ethers } from 'ethers';
    const w = new ethers.Wallet(process.env.KEY, new ethers.JsonRpcProvider('$EVM_RPC'));
    const g = new ethers.Contract('0x0000000000000000000000000000000000000805',
      ['function vote(address voter, uint64 proposalId, uint8 option, string metadata) returns (bool)'], w);
    const rc = await (await g.vote(w.address, BigInt(process.env.PID), 1, 'evm-e2e', { gasLimit: 500000 })).wait();
    if (rc.status !== 1) throw new Error('vote failed');") || fail "EVM gov vote"
  DEV1=$($BIN keys show dev1 -a "${KR[@]}")
  opt=$(rest "/cosmos/gov/v1/proposals/$id/votes/$DEV1" | json "d['vote']['options'][0]['option']")
  [ "$opt" = "VOTE_OPTION_YES" ] || fail "EVM vote not visible on Cosmos side: $opt"
}

# 1. Parameter change: raise x/policy min_self_bond 100 000 → 150 000 NIRA
P=$(mktemp)
cur=$(rest /nira/policy/v1/params | json "d['params']")
python3 - "$P" "$GOV" "$DEPOSIT" "$cur" <<'PY'
import ast, json, sys
p, gov, dep, cur = sys.argv[1], sys.argv[2], sys.argv[3], ast.literal_eval(sys.argv[4])
cur["min_self_bond"] = "150000000000000000000000"
json.dump({"messages": [{"@type": "/nira.policy.v1.MsgUpdateParams", "authority": gov, "params": cur}],
           "deposit": dep, "title": "Raise min self-bond", "summary": "e2e: policy param change"}, open(p, "w"))
PY
id=$(propose_and_pass "$P" evm_vote)
[ "$(rest /nira/policy/v1/params | json "d['params']['min_self_bond']")" = "150000000000000000000000" ] || fail "param not updated"
ok "proposal $id: x/policy min_self_bond 100k → 150k NIRA (validator voted via CLI, dev1 via EVM gov precompile)"

# 2. False alert → governance rejects → bond slashed (G-COMM-1)
OP=$($BIN keys show dev2 -a "${KR[@]}")
if ! rest "/nira/community/v1/nodes/$OP" >/dev/null 2>&1; then
  TIP=$($BIN tx community onion-init --length 10000 --overwrite --home "$HOME_DIR" | awk '/onion_tip/{print $2}')
  tx dev2 community register-node gov-e2e 1000000000000000000000anira "$TIP" 10000 >/dev/null
fi
REG=$(rest "/nira/community/v1/nodes/$OP" | json "int(d['node']['registered_epoch'])")
while [ "$(rest /nira/community/v1/epoch | json "int(d['epoch']['number'])")" -le "$REG" ]; do sleep 2; done
H=$(( $(height) - 3 ))
FAKE=$(python3 -c "import os;print(os.urandom(32).hex())")
res=$(tx dev2 community report-divergence "$H" "$FAKE")
AID=$(echo "$res" | json "[a['value'] for e in d['events'] if e['type']=='divergence_reported' for a in e['attributes'] if a['key']=='alert_id'][0]")
bond0=$(rest "/nira/community/v1/nodes/$OP" | json "int(d['node']['bond'])")
python3 - "$P" "$GOV" "$DEPOSIT" "$AID" <<'PY'
import json, sys
p, gov, dep, aid = sys.argv[1:]
json.dump({"messages": [{"@type": "/nira.community.v1.MsgResolveAlert", "authority": gov, "alert_id": aid, "valid": False}],
           "deposit": dep, "title": "Reject false alert", "summary": "e2e: false divergence alert"}, open(p, "w"))
PY
id=$(propose_and_pass "$P")
st=$(rest "/nira/community/v1/alerts/$AID" | json "d['alert']['status']")
[ "$st" = "ALERT_STATUS_REJECTED" ] || fail "alert status $st"
node=$(rest "/nira/community/v1/nodes/$OP")
bond1=$(echo "$node" | json "int(d['node']['bond'])"); slashed=$(echo "$node" | json "int(d['node']['slashed_total'])")
python3 -c "assert $bond1 == $bond0 - $bond0 // 10 and $slashed == $bond0 // 10" || fail "slash: bond $bond0 → $bond1, slashed $slashed"
ok "proposal $id: false alert #$AID rejected by governance → 10 % of bond slashed ($slashed anira)"

rm -f "$P"

echo
echo "$PASSED checks passed"
