#!/bin/sh
# Community node container: full node + the real prove loop (nirad tx community run).
# Restart-safe: identity is the operator key + on-chain node, the onion secret is in the home.
set -eu
HOME_DIR=/data/$NODE
CHAIN_ID=$(python3 -c "import json;print(json.load(open('/data/testnet.json'))['chain_id'])")
nirad start --home "$HOME_DIR" &
NODE_PID=$!
trap 'kill $NODE_PID 2>/dev/null' TERM INT
# wait until the local node is synced
until [ "$(curl -sf localhost:26657/status 2>/dev/null | python3 -c 'import json,sys; print(json.load(sys.stdin)["result"]["sync_info"]["catching_up"])' 2>/dev/null)" = "False" ]; do
  sleep 2
done
echo "node synced; starting community prove loop"
exec nirad tx community run --from cnode --keyring-backend test --home "$HOME_DIR" --chain-id "$CHAIN_ID" \
  --node tcp://localhost:26657 --gas auto --gas-adjustment 1.6 --gas-prices 2000000000anira -y --interval 5s
