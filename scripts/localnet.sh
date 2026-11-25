#!/usr/bin/env bash
# Single-validator local network for development and e2e tests.
# Uses eth_secp256k1 keys (coin type 60) so the same key signs Cosmos and
# Ethereum transactions (R-F11). NEVER use these keys on a public network.
set -euo pipefail

BIN=${BIN:-nirad}
HOME_DIR=${HOME_DIR:-$HOME/.nira}
CHAIN_ID=${CHAIN_ID:-nira_47201-1}
DENOM=anira
KR="--keyring-backend test --home $HOME_DIR"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# Publicly known dev mnemonics (test only).
VAL_MNEMONIC="gesture inject test cycle original hollow east ridge hen combine junk child bacon zero hope comfort vacuum milk pitch cage oppose unhappy lunar seat"
DEV1_MNEMONIC="copper push brief egg scan entry inform record adjust fossil boss egg comic alien upon aspect dry avoid interest fury window hint race symptom"
DEV2_MNEMONIC="maximum display century economy unlock van census kite error heart snow filter midnight usage egg venture cash kick motor survey drastic edge muffin visual"

rm -rf "$HOME_DIR"
$BIN init localnet --chain-id "$CHAIN_ID" --default-denom $DENOM --home "$HOME_DIR" >/dev/null 2>&1

echo "$VAL_MNEMONIC"  | $BIN keys add validator --recover --algo eth_secp256k1 $KR >/dev/null
echo "$DEV1_MNEMONIC" | $BIN keys add dev1 --recover --algo eth_secp256k1 $KR >/dev/null
echo "$DEV2_MNEMONIC" | $BIN keys add dev2 --recover --algo eth_secp256k1 $KR >/dev/null

# 100M NIRA each (18 decimals)
AMT=100000000000000000000000000$DENOM
for k in validator dev1 dev2; do
  $BIN genesis add-genesis-account "$($BIN keys show $k -a $KR)" $AMT --home "$HOME_DIR"
done

python3 "$SCRIPT_DIR/patch_genesis.py" "$HOME_DIR/config/genesis.json"

# 1M NIRA self-bond
$BIN genesis gentx validator 1000000000000000000000000$DENOM --chain-id "$CHAIN_ID" \
  --min-self-delegation 100000000000000000000000 --commission-rate 0.10 --commission-max-rate 0.20 \
  --commission-max-change-rate 0.01 $KR >/dev/null 2>&1
$BIN genesis collect-gentxs --home "$HOME_DIR" >/dev/null 2>&1
$BIN genesis validate --home "$HOME_DIR"

APP=$HOME_DIR/config/app.toml; CFG=$HOME_DIR/config/config.toml
sed -i 's/^minimum-gas-prices = .*/minimum-gas-prices = "0anira"/' "$APP"
sed -i 's/allow-insecure-unlock = true/allow-insecure-unlock = false/' "$APP"
sed -i 's/^timeout_commit = .*/timeout_commit = "1s"/' "$CFG"
# Enable Ethereum JSON-RPC (dev only: binds to localhost)
python3 - "$APP" <<'PY'
import re, sys
p = sys.argv[1]; s = open(p).read()
head, sep, tail = s.partition("[json-rpc]")
tail = re.sub(r"^enable = false", "enable = true", tail, count=1, flags=re.M)
# REST API (needed by e2e tests for cross-checking bank vs EVM state)
api_head, api_sep, api_tail = head.partition("[api]")
head = api_head + api_sep + re.sub(r"^enable = false", "enable = true", api_tail, count=1, flags=re.M)
tail = re.sub(r'^address = ".*"', 'address = "127.0.0.1:8545"', tail, count=1, flags=re.M)
tail = re.sub(r'^ws-address = ".*"', 'ws-address = "127.0.0.1:8546"', tail, count=1, flags=re.M)
open(p, "w").write(head + sep + tail)
PY
echo "localnet ready: $HOME_DIR (chain-id $CHAIN_ID)"
