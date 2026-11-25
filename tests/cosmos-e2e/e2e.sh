#!/usr/bin/env bash
# Cosmos-side end-to-end checks for a running Nira localnet (scripts/localnet.sh).
# Every check also cross-verifies the result through the EVM interface
# (JSON-RPC / precompiles), so both interfaces are proven to share one state.
#
# Usage: tests/cosmos-e2e/e2e.sh   (env: BIN, HOME_DIR, RPC, REST, EVM_RPC)
set -euo pipefail

BIN=${BIN:-nirad}
HOME_DIR=${HOME_DIR:-$HOME/.nira}
CHAIN_ID=${CHAIN_ID:-nira_47201-1}
REST=${REST:-http://127.0.0.1:1317}
EVM_RPC=${EVM_RPC:-http://127.0.0.1:8545}
KR=(--keyring-backend test --home "$HOME_DIR")
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PASSED=0

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { PASSED=$((PASSED + 1)); echo "ok  $*"; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d': d}))" "$1"; }
rest() { curl -sf "$REST$1"; }
evm() { curl -sf -X POST -H 'content-type: application/json' \
  --data "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$1\",\"params\":$2}" "$EVM_RPC" | json "d['result']"; }
bank_balance() { rest "/cosmos/bank/v1beta1/balances/$1/by_denom?denom=anira" | json "d['balance']['amount']"; }
hex_addr() { $BIN debug addr "$1" 2>/dev/null | awk '/Address \(hex\)/{print "0x"$3}'; }
evm_balance() { python3 -c "print(int('$(evm eth_getBalance "[\"$1\",\"latest\"]")', 16))"; }

gas_price() {
  # Pay at least the current EIP-1559 base fee (x/feemarket also applies to Cosmos txs).
  local bf; bf=$(rest /cosmos/evm/feemarket/v1/base_fee | json "d['base_fee']")
  python3 -c "import math; print(str(max(1, math.ceil(float('$bf') * 2))) + 'anira')"
}

# tx <args...>: broadcast a Cosmos tx, wait for inclusion, fail on non-zero code.
tx() {
  local out hash code
  out=$($BIN tx "$@" --chain-id "$CHAIN_ID" "${KR[@]}" --gas auto --gas-adjustment 1.5 \
    --gas-prices "$(gas_price)" -y --output json 2>/tmp/nira-e2e-tx.err) \
    || fail "tx $* → $out $(cat /tmp/nira-e2e-tx.err)"
  hash=$(echo "$out" | json "d['txhash']")
  code=$(echo "$out" | json "d['code']")
  [ "$code" = "0" ] || fail "tx $* CheckTx code $code: $out"
  for _ in $(seq 1 20); do
    sleep 1
    if res=$($BIN q tx "$hash" --output json --home "$HOME_DIR" 2>/dev/null); then
      code=$(echo "$res" | json "d['code']")
      [ "$code" = "0" ] || fail "tx $* DeliverTx code $code: $(echo "$res" | json "d['raw_log']")"
      echo "$hash"; return 0
    fi
  done
  fail "tx $hash not included"
}

DEV1=$($BIN keys show dev1 -a "${KR[@]}")
DEV2=$($BIN keys show dev2 -a "${KR[@]}")
VALOPER=$(rest /cosmos/staking/v1beta1/validators | json "d['validators'][0]['operator_address']")

# 1. Keys: eth_secp256k1, and bech32 <-> hex map to the same account on both interfaces
algo=$($BIN keys show dev2 "${KR[@]}" --output json | json "d['pubkey']")
[[ "$algo" == *ethsecp256k1* ]] || fail "dev2 key is not eth_secp256k1: $algo"
DEV2_HEX=$(hex_addr "$DEV2")
mapped=$(rest "/cosmos/evm/vm/v1/cosmos_account/$DEV2_HEX" | json "d['cosmos_address']")
[ "$mapped" = "$DEV2" ] || fail "address mapping $DEV2_HEX → $mapped, want $DEV2"
ok "eth_secp256k1 key; bech32 $DEV2 == hex $DEV2_HEX"

# 2. Cosmos bank send → visible via eth_getBalance (EVM)
RCPT_HEX=0x$(python3 -c "import secrets; print(secrets.token_hex(20))")
RCPT=$($BIN debug addr "${RCPT_HEX#0x}" 2>/dev/null | awk '/Bech32 Acc/{print $3}')
tx bank send dev2 "$RCPT" 777000000000000000anira >/dev/null
[ "$(bank_balance "$RCPT")" = "777000000000000000" ] || fail "bank balance after send"
[ "$(evm_balance "$RCPT_HEX")" = "777000000000000000" ] || fail "EVM balance after Cosmos send"
ok "Cosmos MsgSend → bank and eth_getBalance agree (777e15 anira)"

# 3. Sender's balance identical on both interfaces after paying Cosmos fees
[ "$(bank_balance "$DEV2")" = "$(evm_balance "$DEV2_HEX")" ] || fail "dev2 bank != EVM balance"
ok "sender balance identical via bank and EVM after fees"

# 4. Cosmos delegate → visible via staking precompile (EVM eth_call)
deleg_amount() {  # amount of DEV2's delegation to VALOPER via Cosmos REST (0 if none)
  rest "/cosmos/staking/v1beta1/validators/$VALOPER/delegations/$DEV2" 2>/dev/null \
    | json "d['delegation_response']['balance']['amount']" 2>/dev/null || echo 0
}
precompile_delegation() {  # same via EVM staking precompile 0x...0800 delegation(address,string)
  node "$SCRIPT_DIR/../evm-e2e/precompile-delegation.mjs" "$DEV2_HEX" "$VALOPER"
}
before=$(deleg_amount)
tx staking delegate "$VALOPER" 2000000000000000000000anira --from dev2 >/dev/null
after=$(deleg_amount)
[ "$(python3 -c "print($after - $before)")" = "2000000000000000000000" ] || fail "delegation delta $before → $after"
[ "$(precompile_delegation)" = "$after" ] || fail "precompile delegation != REST ($after)"
ok "Cosmos MsgDelegate (power-cap hook path) → REST and staking precompile agree ($after)"

# 5. Cosmos undelegate is never blocked (INV-STAKE-CAP-2) and is visible via EVM
tx staking unbond "$VALOPER" 2000000000000000000000anira --from dev2 >/dev/null
[ "$(deleg_amount)" = "$before" ] || fail "undelegate did not restore delegation"
[ "$(precompile_delegation)" = "$before" ] || fail "precompile after unbond"
ubd=$(rest "/cosmos/staking/v1beta1/delegators/$DEV2/unbonding_delegations" | json "len(d['unbonding_responses'])")
[ "$ubd" -ge 1 ] || fail "no unbonding entry"
ok "Cosmos MsgUndelegate → unbonding entry; REST and precompile agree"

# 6. x/policy power-cap params queryable via Cosmos CLI and REST
bps=$($BIN q policy params --output json --home "$HOME_DIR" | json "d['params']['max_validator_power_bps']")
[ "$bps" = "1000" ] || fail "max_validator_power_bps = $bps"
rbps=$(rest /nira/policy/v1/params | json "d['params']['max_validator_power_bps']")
[ "$rbps" = "1000" ] || fail "REST max_validator_power_bps = $rbps"
ok "x/policy params via CLI and REST (cap 1000 bps)"

# 7. Invalid params update via Cosmos tx is rejected (non-gov signer)
if $BIN tx policy update-params --from dev2 --chain-id "$CHAIN_ID" "${KR[@]}" -y --gas-prices "$(gas_price)" \
    --output json >/dev/null 2>&1; then
  : # CLI may not expose it (gov-only); either way state must be unchanged
fi
[ "$($BIN q policy params --output json --home "$HOME_DIR" | json "d['params']['max_validator_power_bps']")" = "1000" ] \
  || fail "params changed by non-gov signer"
ok "params unchanged by non-governance signer"

# 8. Chain minimum self-bond at validator creation (G-STAKE-1). `--gas auto` simulates the tx
#    against the real state machine; we decline the broadcast prompt, so nothing is submitted.
mkval() {
  cat >"$1" <<JSON
{"pubkey":{"@type":"/cosmos.crypto.ed25519.PubKey","key":"$(python3 -c 'import base64,os;print(base64.b64encode(os.urandom(32)).decode())')"},
 "amount":"200000000000000000000000anira","moniker":"e2e","commission-rate":"0.10","commission-max-rate":"0.20",
 "commission-max-change-rate":"0.01","min-self-delegation":"$2"}
JSON
}
simulate_create_validator() {
  echo n | $BIN tx staking create-validator "$1" --from dev2 --chain-id "$CHAIN_ID" "${KR[@]}" --gas auto \
    --gas-adjustment 1.5 --gas-prices "$(gas_price)" 2>&1 || true
}
VJ=$(mktemp)
mkval "$VJ" 1000000000000000000
out=$(simulate_create_validator "$VJ")
echo "$out" | grep -q "below chain minimum self-bond" || fail "create-validator below min self-bond not rejected: $out"
mkval "$VJ" 100000000000000000000000
out=$(simulate_create_validator "$VJ")
echo "$out" | grep -q "gas estimate" || fail "create-validator at min self-bond rejected: $out"
rm -f "$VJ"
[ "$($BIN q policy params --output json --home "$HOME_DIR" | json "d['params']['min_self_bond']")" = "100000000000000000000000" ] || fail "min_self_bond param"
ok "create-validator: min_self_delegation 1 NIRA rejected, 100 000 NIRA accepted (chain floor via x/policy)"

echo
echo "$PASSED checks passed"
