#!/usr/bin/env bash
# Validator-set behaviour on a 16-validator network with max_validators = 15 (G-CONS-1, G-CONS-2):
#   rotation, live power-cap rejection (Cosmos tx + EVM precompile), double-sign slashing and
#   tombstoning with propagation to delegators, downtime jailing and unjailing, BFT safety.
# Setup: N=16 MAX_VALIDATORS=15 scripts/genesis/localtest.sh
set -euo pipefail

BIN=${BIN:-nirad}
ROOT=${ROOT:-$HOME/.nira-localtest}
REPO="$(cd "$(dirname "$0")/../.." && pwd)"
CHAIN_ID=$(python3 -c "import tomllib;print(tomllib.load(open('$ROOT/build/localtest.toml','rb'))['chain']['chain_id'])")
KR=(--keyring-backend test --home "$ROOT/val0")
PASSED=0
fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { PASSED=$((PASSED + 1)); echo "ok  $*"; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d': d}))" "$1"; }
rest() { curl -sf -m 10 "http://127.0.0.1:1317$1"; }
height() { curl -sf -m 5 http://127.0.0.1:26657/status | json "int(d['result']['sync_info']['latest_block_height'])"; }
wait_blocks() { local t=$(( $(height) + $1 )); while [ "$(height)" -lt "$t" ]; do sleep 1; done; }
NIRA=000000000000000000
txh() { python3 -c "import json,sys
for l in sys.stdin:
    l=l.strip()
    if l.startswith('{'): print(json.loads(l)['txhash'])"; }
tx() { # tx <home> <from> <args...>  (real broadcast; fails on non-zero code)
  local home=$1 from=$2; shift 2
  h=$($BIN tx "$@" --from "$from" --chain-id "$CHAIN_ID" --keyring-backend test --home "$home" --gas auto \
      --gas-adjustment 1.6 --gas-prices 2000000000anira -y --output json 2>/tmp/nira-tx.err | txh) || true
  [ -n "$h" ] || fail "tx $*: $(cat /tmp/nira-tx.err)"
  for _ in $(seq 1 30); do sleep 1
    if r=$($BIN q tx "$h" --output json --home "$home" 2>/dev/null); then
      [ "$(echo "$r" | json "d['code']")" = "0" ] || fail "tx $*: $(echo "$r" | json "d['raw_log']")"; return 0
    fi
  done; fail "tx $h not included"
}
vals() { rest "/cosmos/staking/v1beta1/validators?pagination.limit=100"; }
status_of() { vals | json "[v['status'] for v in d['validators'] if v['operator_address']=='$1'][0]"; }
tokens_of() { vals | json "int([v['tokens'] for v in d['validators'] if v['operator_address']=='$1'][0])"; }
valoper_of() { $BIN keys show val -a --bech val --keyring-backend test --home "$ROOT/val$1"; }

N=$(ls -d "$ROOT"/val* | wc -l)
bonded=$(vals | json "sum(1 for v in d['validators'] if v['status']=='BOND_STATUS_BONDED')")
maxv=$(rest /cosmos/staking/v1beta1/params | json "int(d['params']['max_validators'])")
[ "$N" -ge 16 ] && [ "$bonded" = "$maxv" ] && [ "$maxv" -lt "$N" ] || fail "need N≥16 with max_validators<N (N=$N bonded=$bonded max=$maxv)"
ok "$N validator candidates, active set capped at $maxv"

# 1. Rotation: delegate to the candidate outside the set → it enters, the weakest leaves
OUT=$(vals | json "[v['operator_address'] for v in d['validators'] if v['status']!='BOND_STATUS_BONDED'][0]")
tx "$ROOT/val0" dev1 staking delegate "$OUT" 200000${NIRA}anira
wait_blocks 3
[ "$(status_of "$OUT")" = "BOND_STATUS_BONDED" ] || fail "candidate did not rotate in"
left=$(vals | json "[v['description']['moniker'] for v in d['validators'] if v['status']!='BOND_STATUS_BONDED']")
[ "$(vals | json "sum(1 for v in d['validators'] if v['status']=='BOND_STATUS_BONDED')")" = "$maxv" ] || fail "set size changed"
ok "rotation: $(vals | json "[v['description']['moniker'] for v in d['validators'] if v['operator_address']=='$OUT'][0]") entered the active set; $left rotated out (set stays $maxv)"

# 2. Live power-cap rejection (cap active: ≥ 15 validators, ≥ 5 M NIRA bonded)
T=$(valoper_of 1)
out=$(echo n | $BIN tx staking delegate "$T" 1000000${NIRA}anira --from dev1 --chain-id "$CHAIN_ID" "${KR[@]}" \
      --gas auto --gas-prices 2000000000anira 2>&1 || true)
echo "$out" | grep -q "exceeds validator voting-power cap" || fail "cap did not reject: $out"
KEY=$(echo y | $BIN keys unsafe-export-eth-key dev1 "${KR[@]}" 2>/dev/null | tail -1)
(cd "$REPO/tests/evm-e2e" && KEY=$KEY node --input-type=module -e "
  import { ethers } from 'ethers';
  const w = new ethers.Wallet(process.env.KEY, new ethers.JsonRpcProvider('http://127.0.0.1:8545'));
  const st = new ethers.Contract('0x0000000000000000000000000000000000000800',
    ['function delegate(address,string,uint256) returns (bool)'], w);
  try { await st.delegate.staticCall(w.address, '$T', ethers.parseEther('1000000'), { gasLimit: 800000 }); process.exit(2); }
  catch (e) { if (!/cap/i.test(JSON.stringify(e.info ?? {}) + e.message)) { console.error(e.message); process.exit(3); } }
  await st.delegate.staticCall(w.address, '$T', ethers.parseEther('300000'), { gasLimit: 800000 });") || fail "EVM cap check"
tx "$ROOT/val0" dev1 staking delegate "$T" 300000${NIRA}anira
ok "power cap live: +1 M NIRA to one validator rejected via Cosmos tx and EVM precompile; +300 k accepted"

# 3. Double sign → evidence → 5 % slash, tombstone, delegators slashed pro rata
X=$(valoper_of 2)
DEV2=$($BIN keys show dev2 -a "${KR[@]}")
tx "$ROOT/val0" dev2 staking delegate "$X" 10000${NIRA}anira
d0=$(rest "/cosmos/staking/v1beta1/validators/$X/delegations/$DEV2" | json "int(d['delegation_response']['balance']['amount'])")
t0=$(tokens_of "$X")
(cd "$REPO" && go run ./tools/doublesign --home "$ROOT/val2" --rpc http://127.0.0.1:26657) || fail "evidence submission"
for _ in $(seq 1 30); do [ "$(vals | json "[v['jailed'] for v in d['validators'] if v['operator_address']=='$X'][0]")" = "True" ] && break; sleep 1; done
CONS=$($BIN comet show-address --home "$ROOT/val2")
tomb=$(rest "/cosmos/slashing/v1beta1/signing_infos/$CONS" | json "d['val_signing_info']['tombstoned']")
t1=$(tokens_of "$X")
d1=$(rest "/cosmos/staking/v1beta1/validators/$X/delegations/$DEV2" | json "int(d['delegation_response']['balance']['amount'])")
# SDK semantics: burn 5 % of the stake AT THE INFRACTION HEIGHT (self-bond 1 M; dev2's newer
# delegation wasn't there yet), then every current delegator loses the same share of its stake.
python3 -c "
t0,t1,d0,d1=$t0,$t1,$d0,$d1
infraction_stake = t0 - $d0
assert t0 - t1 == infraction_stake * 5 // 100, ('tokens', t0, t1)
assert abs(d1 - d0 * t1 // t0) <= 1, ('delegation pro rata', d0, d1)" || fail "slash amounts t:$t0→$t1 d:$d0→$d1"
[ "$tomb" = "True" ] || fail "not tombstoned"
[ "$(status_of "$X")" != "BOND_STATUS_BONDED" ] || fail "still bonded"
ok "double sign by core-2: jailed + tombstoned, 5 % of infraction-height stake burned ($t0 → $t1), delegator dev2 slashed pro rata ($d0 → $d1)"

# 4. Downtime → jailed (0.01 %); restart + unjail → bonded again
Y=""
for i in $(seq 3 $((N - 1))); do [ "$(status_of "$(valoper_of $i)")" = "BOND_STATUS_BONDED" ] && { Y=$i; break; }; done
[ -n "$Y" ] || fail "no bonded validator for the downtime test"
YV=$(valoper_of $Y)
kill "$(cat "$ROOT/val$Y/node.pid")"
for _ in $(seq 1 300); do [ "$(vals | json "[v['jailed'] for v in d['validators'] if v['operator_address']=='$YV'][0]")" = "True" ] && break; sleep 1; done
[ "$(vals | json "[v['jailed'] for v in d['validators'] if v['operator_address']=='$YV'][0]")" = "True" ] || fail "offline validator not jailed"
ok "downtime: core-$Y offline > 95 % of the 100-block window → jailed"
nohup $BIN start --home "$ROOT/val$Y" >>"$ROOT/val$Y/node.log" 2>&1 & echo $! >"$ROOT/val$Y/node.pid"
sleep 65   # downtime_jail_duration = 60 s, plus catch-up
for _ in $(seq 1 60); do [ "$(curl -sf -m 3 http://127.0.0.1:$((26657 + 100 * Y))/status | json "d['result']['sync_info']['catching_up']")" = "False" ] && break; sleep 2; done
tx "$ROOT/val$Y" val slashing unjail
wait_blocks 3
[ "$(status_of "$YV")" = "BOND_STATUS_BONDED" ] || [ "$(vals | json "[v['jailed'] for v in d['validators'] if v['operator_address']=='$YV'][0]")" = "False" ] || fail "unjail failed"
ok "core-$Y restarted, unjailed after the 60 s jail, eligible again (status $(status_of "$YV"))"

# 5. Safety: every running node agrees on a recent block
H=$(( $(height) - 2 ))
hashes=$(for i in $(seq 0 $((N - 1))); do curl -sf -m 3 "http://127.0.0.1:$((26657 + 100 * i))/block?height=$H" 2>/dev/null | json "d['result']['block_id']['hash']" 2>/dev/null; done | sort -u)
[ "$(echo "$hashes" | wc -l)" = "1" ] || fail "nodes disagree at $H"
ok "all reachable nodes agree on block $H after rotation, slashing and jailing (no fork)"

echo
echo "$PASSED checks passed"
