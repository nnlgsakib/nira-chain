#!/usr/bin/env bash
# Verifies a network built by scripts/genesis/localtest.sh against its network
# file, on both interfaces, then exercises BFT fault tolerance.
set -euo pipefail

BIN=${BIN:-nira-chaind}
ROOT=${ROOT:-$HOME/.nira-localtest}
NET=$ROOT/build/localtest.toml
REPO="$(cd "$(dirname "$0")/../.." && pwd)"
KR=(--keyring-backend test --home "$ROOT/val0")
PASSED=0
fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { PASSED=$((PASSED + 1)); echo "ok  $*"; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d': d}))" "$1"; }
rest() { curl -sf -m 10 "http://127.0.0.1:$((1317 + 100 * ${2:-0}))$1"; }
rpc() { curl -sf -m 5 "http://127.0.0.1:$((26657 + 100 * $2))$1"; }
height() { rpc /status "$1" | json "int(d['result']['sync_info']['latest_block_height'])"; }
evm_balance() {
  python3 -c "print(int('$(curl -sf -X POST -H 'content-type: application/json' \
    --data "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"eth_getBalance\",\"params\":[\"$1\",\"latest\"]}" \
    http://127.0.0.1:8545 | json "d['result']")', 16))"
}
toml() { python3 -c "import tomllib,sys; d=tomllib.load(open('$NET','rb')); print(eval(sys.argv[1], {'d': d}))" "$1"; }
hexaddr() { python3 -c "import sys; sys.path.insert(0,'$REPO/scripts/genesis'); import build_genesis as b; print('0x'+b.addr_bytes('$1').hex())"; }
base() { python3 -c "import sys; sys.path.insert(0,'$REPO/scripts/genesis'); import build_genesis as b; print(b.nira_to_base('$1'))"; }

# 1. Validator set = network file; equal power; everyone signs
N=$(toml "len(d['validator'])")
vals=$(rest /cosmos/staking/v1beta1/validators?status=BOND_STATUS_BONDED | json "len(d['validators'])")
[ "$vals" = "$N" ] || fail "bonded validators $vals != $N"
H=$(height 0)
sigs=$(rpc "/commit?height=$((H - 1))" 0 | json "sum(1 for s in d['result']['signed_header']['commit']['signatures'] if s['block_id_flag']==2)")
[ "$sigs" = "$N" ] || fail "commit at $((H - 1)) has $sigs/$N signatures"
ok "$N bonded validators from the network file; all $N signed block $((H - 1))"

# 2. Allocations land exactly, visible on Cosmos REST and EVM
for name in dev1 dev2; do
  a=$(toml "[x['address'] for x in d['allocation'] if x['name']=='$name'][0]")
  want=$(base "$(toml "[x['amount'] for x in d['allocation'] if x['name']=='$name'][0]")")
  bank=$(rest "/cosmos/bank/v1beta1/balances/$a/by_denom?denom=anira" | json "d['balance']['amount']")
  evm=$(evm_balance "$(hexaddr "$a")")
  # dev accounts may have paid fees if other suites ran first: require ≤ and close
  python3 -c "assert $bank == $evm, 'bank != evm'; assert $want - 10**21 <= $bank <= $want" || fail "$name: want $want bank $bank evm $evm"
done
ok "allocations dev1/dev2 match the network file via bank REST and eth_getBalance"

# 3. Vesting allocation is locked
V=$(toml "[x['address'] for x in d['allocation'] if x['name']=='Vesting test'][0]")
total=$(rest "/cosmos/bank/v1beta1/balances/$V/by_denom?denom=anira" | json "int(d['balance']['amount'])")
spend=$(rest "/cosmos/bank/v1beta1/spendable_balances/$V" | json "sum(int(c['amount']) for c in d['balances'] if c['denom']=='anira')")
python3 -c "assert 0 < $spend < $total // 2" || fail "vesting: spendable $spend of $total"
ok "vesting account: $spend of $total anira spendable (linear 2026→2036)"

# 4. Protocol reserve in the community pool
pool=$(rest /cosmos/distribution/v1beta1/community_pool | json "int(float([c['amount'] for c in d['pool'] if c['denom']=='anira'][0]))")
want=$(base "$(toml "[x['amount'] for x in d['allocation'] if x.get('kind')=='community_pool'][0]")")
python3 -c "assert $pool >= $want" || fail "community pool $pool < $want"
ok "community pool ≥ protocol reserve allocation ($want anira)"

# 5. Genesis community node is active (REST + EVM precompile) and can work
G=$(toml "d['community_node'][0]['operator']")
st=$(rest "/nira/community/v1/nodes/$G" | json "d['node']['status']")
[ "$st" = "NODE_STATUS_ACTIVE" ] || fail "genesis node status $st"
evmstatus=$(cd "$REPO/tests/evm-e2e" && node --input-type=module -e "
import { ethers } from 'ethers'; import { readFileSync } from 'node:fs';
const abi = JSON.parse(readFileSync('../../precompiles/community/abi.json')).abi;
const c = new ethers.Contract('0x0000000000000000000000000000000000000900', abi, new ethers.JsonRpcProvider('http://127.0.0.1:8545'));
const n = await c.getNode('$(hexaddr "$G")'); console.log(n.status.toString(), n.bond.toString());")
[ "$evmstatus" = "1 $(base 1000)" ] || fail "precompile getNode: $evmstatus"
while [ "$(rest /nira/community/v1/epoch | json "int(d['epoch']['number'])")" -lt 2 ]; do sleep 2; done
sleep 2
out=$($BIN tx community prove --from gnode "${KR[@]}" --chain-id "$(toml "d['chain']['chain_id']")" \
  --gas auto --gas-adjustment 1.5 --gas-prices 2000000000anira -y --output json 2>&1) || fail "prove: $out"
echo "$out" | grep -q "submitted 2 proof" || fail "prove: $out"
hash=$(echo "$out" | grep -o '"txhash":"[A-F0-9]*"' | cut -d'"' -f4)
for _ in $(seq 1 15); do
  sleep 1
  res=$($BIN q tx "$hash" --home "$ROOT/val0" --output json 2>/dev/null) && break
done
[ "$(echo "$res" | json "d['code']")" = "0" ] || fail "prove tx: $(echo "$res" | json "d['raw_log']")"
# cumulative counter: robust to the epoch rolling over right after the proof
units=$(rest "/nira/community/v1/nodes/$G" | json "int(d['node']['units_total'])")
[ "$units" -ge 2 ] || fail "units_total $units"
ok "genesis community node ACTIVE (REST + precompile) and proved 2 tasks on the 4-validator network"

# 6. BFT fault tolerance: 3/4 online → live; 2/4 → halt without fork; recovery
stop() { kill "$(cat "$ROOT/val$1/node.pid")"; }
start() { nohup $BIN start --home "$ROOT/val$1" >>"$ROOT/val$1/node.log" 2>&1 & echo $! >"$ROOT/val$1/node.pid"; }
stop 3
h0=$(height 0); sleep 8; h1=$(height 0)
python3 -c "assert $h1 >= $h0 + 3" || fail "with 3/4 online height $h0 → $h1"
ok "1 of 4 validators down (25 % < 1/3): chain stays live ($h0 → $h1)"
stop 2
sleep 4; h0=$(height 0); sleep 10; h1=$(height 0)
[ "$h1" -le "$((h0 + 1))" ] || fail "with 2/4 online chain advanced $h0 → $h1 (safety!)"
ok "2 of 4 down (50 % ≥ 1/3): chain halts at $h1 instead of finalising (safety over liveness)"
start 2; start 3
sleep 15; h2=$(height 0)
python3 -c "assert $h2 > $h1 + 2" || fail "no recovery: $h1 → $h2"
for i in 1 2 3; do
  hi=$(height $i)
  python3 -c "assert abs($hi - $h2) <= 3" || fail "node $i at $hi vs $h2"
done
chk=$((h1 + 1))
hashes=$(for i in 0 1 2 3; do rpc "/block?height=$chk" $i | json "d['result']['block_id']['hash']"; done | sort -u | wc -l)
[ "$hashes" = "1" ] || fail "nodes disagree on block $chk (fork!)"
ok "restart → chain resumes ($h1 → $h2), all nodes caught up, identical block hash at $chk (no fork)"

echo
echo "$PASSED checks passed"
