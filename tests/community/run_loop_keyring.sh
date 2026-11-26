#!/usr/bin/env bash
# The community work loop with a password-protected keyring (the desktop app's
# setup): the password is given ONCE on stdin and the loop must keep proving
# across several epochs. Regression: `run` rebuilt its client context every
# iteration, reopened the keyring and failed on the second pass.
#   tests/community/run_loop_keyring.sh     (needs a running localnet)
set -euo pipefail
BIN=${BIN:-nirad}
CHAIN_ID=${CHAIN_ID:-nira_93000-1}
REST=${REST:-http://127.0.0.1:1317}
P=loop-pass-123
T=$(mktemp -d)
trap 'rm -rf "$T"' EXIT
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d': d}))" "$1"; }
fail() { echo "FAIL: $*" >&2; exit 1; }
gp() { local bf; bf=$(curl -sf $REST/cosmos/evm/feemarket/v1/base_fee | json "d['base_fee']"); python3 -c "import math;print(str(max(1,math.ceil(float('$bf')*2)))+'anira')"; }

cp -r "$HOME/.nira/config" "$T/"
printf '%s\n%s\n' "$P" "$P" | $BIN keys add op --keyring-backend file --algo eth_secp256k1 --home "$T" >/dev/null 2>&1
OP=$(printf '%s\n' "$P" | $BIN keys show op -a --keyring-backend file --home "$T" 2>/dev/null | tail -1)
$BIN tx bank send dev1 "$OP" 2000000000000000000000anira --keyring-backend test --chain-id "$CHAIN_ID" \
  --gas auto --gas-adjustment 1.5 --gas-prices "$(gp)" -y >/dev/null
for _ in $(seq 1 20); do sleep 1; [ "$(curl -sf "$REST/cosmos/bank/v1beta1/balances/$OP/by_denom?denom=anira" | json "d['balance']['amount']")" != "0" ] && break; done
TIP=$($BIN tx community onion-init --length 1000 --overwrite --home "$T" | awk '/onion_tip/{print $2}')
printf '%s\n' "$P" | $BIN tx community register-node loop-test 1000000000000000000000anira "$TIP" 1000 --from op \
  --keyring-backend file --home "$T" --chain-id "$CHAIN_ID" --gas auto --gas-adjustment 1.5 --gas-prices "$(gp)" -y >/dev/null
for _ in $(seq 1 20); do sleep 1; curl -sf "$REST/nira/community/v1/nodes/$OP" >/dev/null && break; done

EPOCH_BLOCKS=$(curl -sf $REST/nira/community/v1/params | json "int(d['params']['epoch_blocks'])")
# ~4 epochs; the password is written once and stdin then closes
out=$(printf '%s\n' "$P" | timeout $((EPOCH_BLOCKS * 5)) $BIN tx community run --interval "${EPOCH_BLOCKS}s" --from op \
  --keyring-backend file --home "$T" --chain-id "$CHAIN_ID" --gas auto --gas-adjustment 1.5 --gas-prices "$(gp)" -y 2>&1 || true)
if echo "$out" | grep -q "passphrase"; then fail "work loop asked for the password again: $(echo "$out" | grep passphrase | head -1)"; fi
n=$(echo "$out" | grep -c "submitted [1-9]" || true)
[ "$n" -ge 2 ] || fail "only $n proving passes with a file keyring: $(echo "$out" | tail -3)"
units=$(curl -sf "$REST/nira/community/v1/nodes/$OP" | json "int(d['node']['units_total'])")
echo "ok  work loop with a password-protected (file) keyring, password given once: $n proving passes, units_total $units"
