#!/usr/bin/env bash
# Full genesis ceremony on one machine with N validators (default 4):
#   keys → network file → pregenesis → per-validator gentx → finalize → start N nodes.
# Same flow as a real launch (genesis/README.md), with local ports.
#   scripts/genesis/localtest.sh          build + start
#   scripts/genesis/localtest.sh stop     stop all nodes
set -euo pipefail

BIN=${BIN:-nira-chaind}
N=${N:-4}
ROOT=${ROOT:-$HOME/.nira-localtest}
CHAIN_ID=nira_47201-77
REPO="$(cd "$(dirname "$0")/../.." && pwd)"
OUT=$ROOT/build
KR=(--keyring-backend test)
# Dev mnemonics shared with scripts/localnet.sh (public, test only)
DEV1_MNEMONIC="copper push brief egg scan entry inform record adjust fossil boss egg comic alien upon aspect dry avoid interest fury window hint race symptom"
DEV2_MNEMONIC="maximum display century economy unlock van census kite error heart snow filter midnight usage egg venture cash kick motor survey drastic edge muffin visual"

port() { echo $(($1 + 100 * $2)); }   # base port, validator index → node i uses base+100*i

stop_all() {
  for p in "$ROOT"/val*/node.pid; do [ -f "$p" ] && kill "$(cat "$p")" 2>/dev/null || true; done
  sleep 1
}
[ "${1:-}" = "stop" ] && { stop_all; echo "stopped"; exit 0; }

stop_all
rm -rf "$ROOT"
mkdir -p "$OUT/gentx"

# 1. Each validator creates its own home and key (in production: on their own machine)
for i in $(seq 0 $((N - 1))); do
  H=$ROOT/val$i
  $BIN init "core-$i" --chain-id $CHAIN_ID --default-denom anira --home "$H" >/dev/null 2>&1
  $BIN keys add val "${KR[@]}" --home "$H" --algo eth_secp256k1 >/dev/null 2>&1
done
# dev accounts + a community node operator live in val0's keyring (used by e2e tests)
echo "$DEV1_MNEMONIC" | $BIN keys add dev1 --recover --algo eth_secp256k1 "${KR[@]}" --home "$ROOT/val0" >/dev/null
echo "$DEV2_MNEMONIC" | $BIN keys add dev2 --recover --algo eth_secp256k1 "${KR[@]}" --home "$ROOT/val0" >/dev/null
$BIN keys add vester --algo eth_secp256k1 "${KR[@]}" --home "$ROOT/val0" >/dev/null 2>&1
$BIN keys add gnode --algo eth_secp256k1 "${KR[@]}" --home "$ROOT/val0" >/dev/null 2>&1
TIP=$($BIN tx community onion-init --length 100000 --home "$ROOT/val0" | awk '/onion_tip/{print $2}')

addr() { $BIN keys show "$1" -a "${KR[@]}" --home "$2"; }

# 2. Coordinator writes the network file from the collected addresses
python3 - "$OUT/localtest.toml" <<PY
import sys
vals = """$(for i in $(seq 0 $((N - 1))); do echo "core-$i $(addr val "$ROOT/val$i")"; done)""".split("\n")
dev1, dev2 = "$(addr dev1 "$ROOT/val0")", "$(addr dev2 "$ROOT/val0")"
vester, gnode, tip = "$(addr vester "$ROOT/val0")", "$(addr gnode "$ROOT/val0")", "$TIP"
toml = f'''[chain]
chain_id = "$CHAIN_ID"
denom = "anira"
[rules]
min_validators = 4
max_genesis_power_share = "0.3333"
[supply]
total = "{100_000_000 * 3 + 50_000_000 + $N * 1_010_000 + 1_000 + 100}"
[[allocation]]
name = "Protocol reserve"
kind = "community_pool"
amount = "50000000"
[[allocation]]
name = "dev1"
address = "{dev1}"
amount = "100000000"
[[allocation]]
name = "dev2"
address = "{dev2}"
amount = "100000000"
[[allocation]]
name = "Vesting test"
address = "{vester}"
amount = "100000000"
vesting = {{ start = "2026-01-01T00:00:00Z", end = "2036-01-01T00:00:00Z" }}
'''
for line in vals:
    m, a = line.split()
    toml += f'[[validator]]\nmoniker = "{m}"\naddress = "{a}"\nbalance = "1010000"\nself_bond = "1000000"\n'
toml += f'''[[community_node]]
moniker = "genesis-node"
operator = "{gnode}"
bond = "1000"
balance = "100"
onion_tip = "{tip}"
onion_length = 100000
[params.feemarket]
min_gas_price = "1000000000.000000000000000000"
[params.community]
epoch_blocks = "20"
apphash_window = 10
target_committee = 1000
max_proofs_per_epoch = 2000
unbonding_blocks = "15"
alert_ttl_blocks = "200"
[params.slashing]
signed_blocks_window = "100"
'''
open(sys.argv[1], "w").write(toml)
PY

# 3. Pregenesis → each validator signs its gentx against it
python3 "$REPO/scripts/genesis/build_genesis.py" pregenesis "$OUT/localtest.toml" --out "$OUT" --binary "$(command -v $BIN)" >/dev/null
for i in $(seq 0 $((N - 1))); do
  H=$ROOT/val$i
  cp "$OUT/pregenesis.json" "$H/config/genesis.json"
  $BIN genesis gentx val 1000000000000000000000000anira --chain-id $CHAIN_ID "${KR[@]}" --home "$H" \
    --moniker "core-$i" --commission-rate 0.10 --commission-max-rate 0.20 --commission-max-change-rate 0.01 \
    --output-document "$OUT/gentx/core-$i.json" >/dev/null 2>&1
done

# 4. Finalize (verifies gentxs against the network file)
python3 "$REPO/scripts/genesis/build_genesis.py" finalize "$OUT/localtest.toml" --out "$OUT" \
  --gentx-dir "$OUT/gentx" --binary "$(command -v $BIN)"

# 5. Distribute genesis, configure ports and peers, start
PEERS=""
for i in $(seq 0 $((N - 1))); do
  PEERS+="$($BIN comet show-node-id --home "$ROOT/val$i")@127.0.0.1:$(port 26656 $i),"
done
for i in $(seq 0 $((N - 1))); do
  H=$ROOT/val$i
  cp "$OUT/genesis.json" "$H/config/genesis.json"
  python3 - "$H" "$i" "${PEERS%,}" <<'PY'
import re, sys
h, i, peers = sys.argv[1], int(sys.argv[2]), sys.argv[3]
p = lambda base: base + 100 * i
cfg = open(f"{h}/config/config.toml").read()
for k, v in {
    "laddr = \"tcp://127.0.0.1:26657\"": f'laddr = "tcp://127.0.0.1:{p(26657)}"',
    "laddr = \"tcp://0.0.0.0:26656\"": f'laddr = "tcp://127.0.0.1:{p(26656)}"',
    "proxy_app = \"tcp://127.0.0.1:26658\"": f'proxy_app = "tcp://127.0.0.1:{p(26658)}"',
    "pprof_laddr = \"localhost:6060\"": 'pprof_laddr = ""',
}.items():
    cfg = cfg.replace(k, v)
cfg = re.sub(r'^persistent_peers = ".*"', f'persistent_peers = "{peers}"', cfg, flags=re.M)
cfg = re.sub(r'^addr_book_strict = .*', 'addr_book_strict = false', cfg, flags=re.M)
cfg = re.sub(r'^allow_duplicate_ip = .*', 'allow_duplicate_ip = true', cfg, flags=re.M)
cfg = re.sub(r'^timeout_commit = .*', 'timeout_commit = "1s"', cfg, flags=re.M)
open(f"{h}/config/config.toml", "w").write(cfg)
app = open(f"{h}/config/app.toml").read()
app = re.sub(r'^minimum-gas-prices = .*', 'minimum-gas-prices = "1000000000anira"', app, flags=re.M)
app = app.replace('address = "tcp://localhost:1317"', f'address = "tcp://localhost:{p(1317)}"')
app = app.replace('address = "localhost:9090"', f'address = "localhost:{p(9090)}"')
app = app.replace('geth-metrics-address = "127.0.0.1:8100"', f'geth-metrics-address = "127.0.0.1:{p(8100)}"')
for sec, en in (("[api]", True), ("[json-rpc]", True)):
    head, sep, tail = app.partition(sec)
    if en:
        tail = re.sub(r"^enable = false", "enable = true", tail, count=1, flags=re.M)
    if sec == "[json-rpc]":
        tail = re.sub(r'^address = ".*"', f'address = "127.0.0.1:{p(8545)}"', tail, count=1, flags=re.M)
        tail = re.sub(r'^ws-address = ".*"', f'ws-address = "127.0.0.1:{p(8546)}"', tail, count=1, flags=re.M)
        tail = re.sub(r'^metrics-address = ".*"', f'metrics-address = "127.0.0.1:{p(6065)}"', tail, count=1, flags=re.M)
    app = head + sep + tail
app = app.replace("allow-insecure-unlock = true", "allow-insecure-unlock = false")
open(f"{h}/config/app.toml", "w").write(app)
PY
  nohup $BIN start --home "$H" >"$H/node.log" 2>&1 &
  echo $! >"$H/node.pid"
done
echo "localtest: $N validators running (node i: RPC $(port 26657 0)+100i, REST 1317+100i, JSON-RPC 8545+100i), home $ROOT"
echo "genesis sha256: $(cut -d' ' -f1 "$OUT/genesis.sha256")"
