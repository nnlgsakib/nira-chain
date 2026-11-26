#!/usr/bin/env bash
# Foundry e2e (G-TEST-3): forge build/create + cast against a running Nira node, the way
# Ethereum developers use it. Cross-checks state through the Cosmos REST API.
#   tests/foundry/run.sh   (env: RPC, REST, KEY)
set -euo pipefail
cd "$(dirname "$0")"
F=${FOUNDRY:-$HOME/.foundry/bin}
RPC=${RPC:-http://127.0.0.1:8545}
REST=${REST:-http://127.0.0.1:1317}
KEY=${KEY:?export KEY=<hex private key of a funded account>}
PASSED=0
ok() { PASSED=$((PASSED + 1)); echo "ok  $*"; }
fail() { echo "FAIL: $*" >&2; exit 1; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d': d}))" "$1"; }
ME=$("$F/cast" wallet address "$KEY")

[ "$("$F/cast" chain-id --rpc-url "$RPC")" = "47201" ] || fail "chain id"
"$F/forge" build -q
ADDR=$("$F/forge" create src/Vault.sol:Vault --rpc-url "$RPC" --private-key "$KEY" --broadcast --legacy --json 2>/dev/null | json "d['deployedTo']")
[ "$("$F/cast" code "$ADDR" --rpc-url "$RPC" | wc -c)" -gt 10 ] || fail "no code at $ADDR"
ok "forge create deployed Vault at $ADDR (chain id 47201)"

RC=$("$F/cast" send "$ADDR" "deposit()" --value 5ether --rpc-url "$RPC" --private-key "$KEY" --json)
[ "$(echo "$RC" | json "int(d['status'],16)")" = "1" ] || fail "deposit"
[ "$(echo "$RC" | json "len(d['logs'])")" = "1" ] || fail "Deposited event"
[ "$("$F/cast" call "$ADDR" "balanceOf(address)(uint256)" "$ME" --rpc-url "$RPC" | awk '{print $1}')" = "5000000000000000000" ] || fail "balanceOf"
BECH=$(curl -sf "$REST/cosmos/evm/vm/v1/cosmos_account/$ADDR" | json "d['cosmos_address']")
BANK=$(curl -sf "$REST/cosmos/bank/v1beta1/balances/$BECH/by_denom?denom=anira" | json "d['balance']['amount']")
[ "$BANK" = "5000000000000000000" ] || fail "bank balance of contract $BANK"
ok "cast send deposit 5 NIRA: receipt status 1, event emitted, balanceOf = 5e18, Cosmos bank balance of the contract = 5e18"

if "$F/cast" send "$ADDR" "withdraw(uint256)" 6ether --rpc-url "$RPC" --private-key "$KEY" >/dev/null 2>&1; then fail "over-withdraw accepted"; fi
"$F/cast" send "$ADDR" "withdraw(uint256)" 2ether --rpc-url "$RPC" --private-key "$KEY" >/dev/null
[ "$("$F/cast" balance "$ADDR" --rpc-url "$RPC")" = "3000000000000000000" ] || fail "balance after withdraw"
ok "over-withdraw reverts (\"insufficient\"); withdraw 2 NIRA leaves 3 NIRA in the contract"

VAL=$(curl -sf "$REST/cosmos/staking/v1beta1/validators" | json "d['validators'][0]['operator_address']")
"$F/cast" send "$ADDR" "stake(string,uint256)" "$VAL" 1ether --gas-limit 500000 --rpc-url "$RPC" --private-key "$KEY" >/dev/null
DEL=$(curl -sf "$REST/cosmos/staking/v1beta1/validators/$VAL/delegations/$BECH" | json "d['delegation_response']['balance']['amount']")
[ "$DEL" = "1000000000000000000" ] || fail "contract delegation $DEL"
ok "contract delegated 1 NIRA through the staking precompile; Cosmos staking shows the contract as delegator"

echo
echo "$PASSED checks passed"
