#!/usr/bin/env bash
# Node synchronisation on a running 4-validator localtest (G-OPS-2):
#   1. state sync: a fresh node joins from a light-client-verified snapshot (no replay from genesis)
#   2. archive vs pruned: an archive node serves historical state; a pruned node can't
# Setup: scripts/genesis/localtest.sh (node 0 serves snapshots every 20 blocks)
set -euo pipefail

BIN=${BIN:-nirad}
ROOT=${ROOT:-$HOME/.nira-localtest}
CHAIN_ID=$(python3 -c "import tomllib;print(tomllib.load(open('$ROOT/build/localtest.toml','rb'))['chain']['chain_id'])")
PASSED=0
fail() { echo "FAIL: $*" >&2; cleanup; exit 1; }
ok() { PASSED=$((PASSED + 1)); echo "ok  $*"; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d': d}))" "$1"; }
rpc() { curl -sf -m 5 "http://127.0.0.1:$1$2"; }
PEERS=$(for i in 0 1 2 3; do echo -n "$($BIN comet show-node-id --home "$ROOT/val$i")@127.0.0.1:$((26656 + 100 * i)),"; done)
cleanup() { for n in statesync archive pruned; do [ -f "$ROOT/$n/node.pid" ] && kill "$(cat "$ROOT/$n/node.pid")" 2>/dev/null; done; true; }

# new_node <name> <index> <pruning> : init a non-validator node at ports base + 100·index
new_node() {
  local name=$1 i=$2 pruning=$3 H=$ROOT/$1
  rm -rf "$H"
  $BIN init "$name" --chain-id "$CHAIN_ID" --home "$H" >/dev/null 2>&1
  cp "$ROOT/build/genesis.json" "$H/config/genesis.json"
  python3 - "$H" "$i" "${PEERS%,}" "$pruning" <<'PY'
import re, sys
h, i, peers, pruning = sys.argv[1], int(sys.argv[2]), sys.argv[3], sys.argv[4]
p = lambda b: b + 100 * i
cfg = open(f"{h}/config/config.toml").read()
for a, b in {'laddr = "tcp://127.0.0.1:26657"': f'laddr = "tcp://127.0.0.1:{p(26657)}"',
             'laddr = "tcp://0.0.0.0:26656"': f'laddr = "tcp://127.0.0.1:{p(26656)}"',
             'proxy_app = "tcp://127.0.0.1:26658"': f'proxy_app = "tcp://127.0.0.1:{p(26658)}"',
             'pprof_laddr = "localhost:6060"': 'pprof_laddr = ""'}.items():
    cfg = cfg.replace(a, b)
cfg = re.sub(r'^persistent_peers = ".*"', f'persistent_peers = "{peers}"', cfg, flags=re.M)
cfg = re.sub(r'^addr_book_strict = .*', 'addr_book_strict = false', cfg, flags=re.M)
cfg = re.sub(r'^allow_duplicate_ip = .*', 'allow_duplicate_ip = true', cfg, flags=re.M)
open(f"{h}/config/config.toml", "w").write(cfg)
app = open(f"{h}/config/app.toml").read()
app = re.sub(r'^minimum-gas-prices = .*', 'minimum-gas-prices = "1000000000anira"', app, flags=re.M)
app = re.sub(r'^pruning = .*', f'pruning = "{pruning}"', app, flags=re.M)
app = app.replace('address = "tcp://localhost:1317"', f'address = "tcp://localhost:{p(1317)}"')
app = app.replace('address = "localhost:9090"', f'address = "localhost:{p(9090)}"')
app = app.replace('geth-metrics-address = "127.0.0.1:8100"', f'geth-metrics-address = "127.0.0.1:{p(8100)}"')
open(f"{h}/config/app.toml", "w").write(app)
PY
}
start() { nohup $BIN start --home "$ROOT/$1" >"$ROOT/$1/node.log" 2>&1 & echo $! >"$ROOT/$1/node.pid"; }
height_of() { rpc "$1" /status | json "int(d['result']['sync_info']['latest_block_height'])"; }
wait_synced() { # port target
  for _ in $(seq 1 180); do
    s=$(rpc "$1" /status 2>/dev/null) || { sleep 1; continue; }
    [ "$(echo "$s" | json "d['result']['sync_info']['catching_up']")" = "False" ] &&
      [ "$(echo "$s" | json "int(d['result']['sync_info']['latest_block_height'])")" -ge "$2" ] && return 0
    sleep 1
  done; return 1
}

# wait for a snapshot on node 0 (every 20 blocks) plus margin
while [ "$(height_of 26657)" -lt 45 ]; do sleep 2; done

# 1. State sync from a light-client-verified snapshot
new_node statesync 10 default
# trust height/hash agreed by independent servers, written by the checkpoint tool (G-CONS-3)
python3 "$(dirname "$0")/../../scripts/trusted-checkpoint.py" --rpc http://127.0.0.1:26657 --rpc http://127.0.0.1:26757 \
  --rpc http://127.0.0.1:26857 --offset 10 --write "$ROOT/statesync/config/config.toml" >/dev/null || fail "checkpoint tool"
start statesync
TARGET=$(( $(height_of 26657) ))
wait_synced 27657 "$TARGET" || fail "state-sync node did not catch up (see $ROOT/statesync/node.log)"
earliest=$(rpc 27657 /status | json "int(d['result']['sync_info']['earliest_block_height'])")
[ "$earliest" -gt 1 ] || fail "state-sync node replayed from genesis (earliest $earliest)"
grep -q "Snapshot restored\|snapshot restored\|Restored snapshot\|Applied snapshot chunk" "$ROOT/statesync/node.log" || fail "no snapshot restore in log"
H=$(( $(height_of 27657) - 1 ))
[ "$(rpc 27657 "/block?height=$H" | json "d['result']['block']['header']['app_hash']")" = \
  "$(rpc 26657 "/block?height=$H" | json "d['result']['block']['header']['app_hash']")" ] || fail "app hash mismatch at $H"
ok "state sync: fresh node restored a light-client-verified snapshot (earliest block $earliest, not 1), same app hash at $H"

# 2. Archive vs aggressively pruned node: historical query at height 2
new_node archive 11 nothing
new_node pruned 12 everything
start archive; start pruned
TARGET=$(height_of 26657)
wait_synced 27757 "$TARGET" || fail "archive node did not sync"
wait_synced 27857 "$TARGET" || fail "pruned node did not sync"
sleep 3
q() { $BIN q bank total-supply --height 2 --node "tcp://127.0.0.1:$1" --output json 2>&1; }
q 27757 | json "d['supply'][0]['amount']" >/dev/null 2>&1 || fail "archive node cannot serve height 2: $(q 27757 | tail -1)"
if q 27857 | json "d['supply'][0]['amount']" >/dev/null 2>&1; then fail "pruned node unexpectedly served height 2"; fi
ok "archive node (pruning=nothing) serves state at height 2; pruned node (pruning=everything) refuses it"

cleanup
echo
echo "$PASSED checks passed"
