#!/usr/bin/env bash
# Lean community node (sustainable community layer): with the `community` node profile a
# node keeps only the last N state versions, yet keeps proving tasks every epoch.
#   - old state versions are really pruned (an ABCI proof query below the window fails)
#   - state inside the window is still provable (Cosmos CLI `prove` + REST units_total)
#   - the same node's state is visible through the EVM (community precompile)
# N is lowered for the test (KEEP, default 60) so pruning happens within minutes; the
# production value is max(2000, 2 × (apphash_window + epoch_blocks)).
#   tests/community/lean_node.sh        (stop other local nodes first)
set -euo pipefail
REPO="$(cd "$(dirname "$0")/../.." && pwd)"
BIN=${BIN:-nirad}
HOME_DIR=$HOME/.nira
KEEP=${KEEP:-60}
CHAIN_ID=nira_93000-1
REST=http://127.0.0.1:1317
KR=(--keyring-backend test --home "$HOME_DIR")
PASSED=0
fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { PASSED=$((PASSED + 1)); echo "ok  $*"; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d': d}))" "$1"; }
height() { curl -sf -m 3 http://127.0.0.1:26657/status | json "int(d['result']['sync_info']['latest_block_height'])"; }
wait_h() { for _ in $(seq 1 600); do [ "$(height 2>/dev/null || echo 0)" -ge "$1" ] && return; sleep 1; done; fail "height $1 not reached"; }
cleanup() { [ -n "${NODE:-}" ] && kill "$NODE" 2>/dev/null; sleep 1; true; }
trap cleanup EXIT

BIN=$BIN "$REPO/scripts/localnet.sh" >/dev/null
# localnet uses pruning-interval 100 in the lean profile: pruning runs every 100 blocks
python3 "$REPO/scripts/node-profile.py" community --home "$HOME_DIR" --keep-recent "$KEEP" | sed 's/^/   /'
grep -q '^pruning = "custom"' "$HOME_DIR/config/app.toml" || fail "pruning not custom"
grep -q "^pruning-keep-recent = \"$KEEP\"" "$HOME_DIR/config/app.toml" || fail "keep-recent not $KEEP"
grep -q '^indexer = "null"' "$HOME_DIR/config/config.toml" || fail "tx index still on"
# the default (no --keep-recent) is derived from genesis and floored at 2000
python3 - "$REPO" "$HOME_DIR" <<'PY' || fail "default keep-recent"
import importlib.util, sys
from pathlib import Path
spec = importlib.util.spec_from_file_location("np", Path(sys.argv[1]) / "scripts/node-profile.py")
m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
assert m.lean_keep(Path(sys.argv[2]), None) == 2000, m.lean_keep(Path(sys.argv[2]), None)
PY
# the lean profile turns JSON-RPC off (not needed to prove); re-enable it here for the EVM cross-check
sed -i '/^\[json-rpc\]/,/^\[/ s/^enable = false/enable = true/' "$HOME_DIR/config/app.toml"
ok "community profile = lean node: pruning custom keep-recent $KEEP (production default 2000), min-retain-blocks $KEEP, no tx index"

$BIN start --home "$HOME_DIR" >"$HOME_DIR/lean-node.log" 2>&1 & NODE=$!
wait_h 3
TXF=(--from dev2 --chain-id "$CHAIN_ID" "${KR[@]}" -y)
gp() { local bf; bf=$(curl -sf $REST/cosmos/evm/feemarket/v1/base_fee | json "d['base_fee']"); python3 -c "import math;print(str(max(1,math.ceil(float('$bf')*2)))+'anira')"; }
TIP=$($BIN tx community onion-init --length 10000 --overwrite --home "$HOME_DIR" | awk '/onion_tip/{print $2}')
$BIN tx community register-node lean-node 1000000000000000000000anira "$TIP" 10000 "${TXF[@]}" --gas 400000 --gas-prices "$(gp)" >/dev/null
OP=$($BIN keys show dev2 -a "${KR[@]}")
for _ in $(seq 1 20); do sleep 1; st=$(curl -sf "$REST/nira/community/v1/nodes/$OP" | json "d['node']['status']" 2>/dev/null) && break; done
[ "${st:-}" = "NODE_STATUS_ACTIVE" ] || fail "node not registered"

# run until pruning has removed early versions: past 3 × KEEP and past a pruning-interval multiple
TARGET=$(( (3 * KEEP / 100 + 2) * 100 ))
echo "   waiting for height $TARGET (pruning every 100 blocks) ..."
wait_h "$TARGET"

# 1. old versions are gone, recent ones are provable
old=$(curl -sf "http://127.0.0.1:26657/abci_query?path=%22/store/acc/key%22&data=0x01&height=5&prove=true" | json "d['result']['response']")
echo "$old" | grep -qiE "version does not exist|not found|unavailable|pruned" || [ "$(echo "$old" | python3 -c "import ast,sys;print(ast.literal_eval(sys.stdin.read()).get('code',0))")" != "0" ] \
  || fail "height 5 still provable after pruning: $old"
h=$(height); recent=$(curl -sf "http://127.0.0.1:26657/abci_query?path=%22/store/acc/key%22&data=0x01&height=$((h - 5))&prove=true" | json "d['result']['response']['code']")
[ "$recent" = "0" ] || fail "recent height $((h - 5)) not provable (code $recent)"
ok "state versions pruned: height 5 no longer provable; height $((h - 5)) still provable (window $KEEP)"

# 2. the node keeps doing verified work after pruning (several epochs)
before=$(curl -sf "$REST/nira/community/v1/nodes/$OP" | json "int(d['node'].get('units_total', 0))")
proved=0
for _ in $(seq 1 6); do
  out=$($BIN tx community prove "${TXF[@]}" --gas-prices "$(gp)" 2>&1) || true
  echo "$out" | grep -q "submitted [1-9]" && proved=$((proved + 1))
  ep=$(curl -sf $REST/nira/community/v1/epoch | json "int(d['next_epoch_height'])"); wait_h $((ep + 2))
done
sleep 3
after=$(curl -sf "$REST/nira/community/v1/nodes/$OP" | json "int(d['node'].get('units_total', 0))")
[ "$proved" -ge 3 ] && [ "$after" -gt "$before" ] || fail "proofs after pruning: $proved epochs, units $before → $after"
ok "lean node proved tasks in $proved/6 epochs after pruning (units_total $before → $after, Cosmos CLI + REST)"

# 3. EVM view of the same node agrees
cd "$REPO/tests/evm-e2e"
HEX=$($BIN debug addr "$OP" 2>/dev/null | awk '/Address \(hex\)/{print "0x"$3}')
evm_units=$(EVM_RPC=http://127.0.0.1:8545 node -e '
import("ethers").then(async ({ ethers }) => {
  const p = new ethers.JsonRpcProvider(process.env.EVM_RPC);
  const c = new ethers.Contract("0x0000000000000000000000000000000000000900",
    ["function getNode(address) view returns ((string moniker,uint256 bond,uint8 status,uint64 registeredEpoch,uint64 onionBaseEpoch,uint64 unitsTotal,uint32 unitsThisEpoch,int64 unbondCompleteHeight,uint256 rewardsTotal,uint256 slashedTotal) node)"], p);
  console.log((await c.getNode(process.argv[1])).unitsTotal.toString());
});' "$HEX" 2>&1) || true
[ "$evm_units" = "$after" ] || fail "precompile unitsTotal $evm_units != REST $after"
ok "community precompile (EVM) reports the same unitsTotal ($evm_units) for the lean node"
cd "$REPO"

echo
du -sh "$HOME_DIR/data/application.db" "$HOME_DIR/data/blockstore.db" 2>/dev/null | sed 's/^/   /'
echo "$PASSED checks passed"
