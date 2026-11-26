#!/usr/bin/env bash
# Remote signing (G-SEC-7): the validator node runs with NO consensus key on disk;
# a separate signer process holds the key and the double-sign state and signs over
# CometBFT's privval protocol (the protocol TMKMS / Horcrux use).
#   tests/consensus/remote_signer.sh     (stop other local nodes first)
set -euo pipefail
REPO="$(cd "$(dirname "$0")/../.." && pwd)"
BIN=${BIN:-nirad}
HOME_DIR=$HOME/.nira
SIGNER_DIR=$HOME_DIR-signer
LOG=$HOME_DIR/remote-signer-test.log
PASSED=0
fail() { echo "FAIL: $*" >&2; cleanup; exit 1; }
ok() { PASSED=$((PASSED + 1)); echo "ok  $*"; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); print(eval(sys.argv[1], {'d': d}))" "$1"; }
height() { curl -sf -m 3 http://127.0.0.1:26657/status | json "int(d['result']['sync_info']['latest_block_height'])"; }
cleanup() { [ -n "${NODE:-}" ] && kill "$NODE" 2>/dev/null; [ -n "${SIGNER:-}" ] && kill "$SIGNER" 2>/dev/null; sleep 1; true; }
trap cleanup EXIT

(cd "$REPO" && go build -o build/remotesigner ./tools/remotesigner)
BIN=$BIN "$REPO/scripts/localnet.sh" >/dev/null
CHAIN_ID=$(python3 -c "import json;print(json.load(open('$HOME_DIR/config/genesis.json'))['chain_id'])")
VALADDR=$($BIN comet show-address --home "$HOME_DIR")

# 1. move the key (and state) off the node; point the node at the signer socket
rm -rf "$SIGNER_DIR"; mkdir -p "$SIGNER_DIR"
mv "$HOME_DIR/config/priv_validator_key.json" "$SIGNER_DIR/"
cp "$HOME_DIR/data/priv_validator_state.json" "$SIGNER_DIR/"
python3 - "$HOME_DIR/config/config.toml" <<'PY'
import re, sys
p = sys.argv[1]; s = open(p).read()
s = re.sub(r'^priv_validator_laddr = ".*"', 'priv_validator_laddr = "tcp://127.0.0.1:26659"', s, flags=re.M)
open(p, "w").write(s)
PY
[ ! -e "$HOME_DIR/config/priv_validator_key.json" ] || fail "key still on node"

# 2. without its signer the node cannot sign: it blocks on the signer socket and produces nothing
: >"$LOG"
$BIN start --home "$HOME_DIR" >>"$LOG" 2>&1 & NODE=$!
sleep 20
h0=$(height 2>/dev/null || echo 0)
[ "$h0" -le 0 ] || fail "node produced blocks without a signer (height $h0)"
grep -q "SignerListener" "$LOG" || fail "node is not waiting on the signer socket (see $LOG)"
kill "$NODE" 2>/dev/null || true; wait "$NODE" 2>/dev/null || true; NODE=
ok "validator node with no key on disk produces no blocks without its signer (waits on the privval socket)"

# 3. signer first (it keeps dialling), then node → blocks signed by the key held only by the signer
"$REPO/build/remotesigner" --key "$SIGNER_DIR/priv_validator_key.json" --state "$SIGNER_DIR/priv_validator_state.json" \
  --node 127.0.0.1:26659 --chain-id "$CHAIN_ID" >>"$LOG" 2>&1 & SIGNER=$!
$BIN start --home "$HOME_DIR" >>"$LOG" 2>&1 & NODE=$!
for _ in $(seq 1 60); do [ "$(height 2>/dev/null || echo 0)" -ge 5 ] && break; sleep 1; done
h1=$(height); [ "$h1" -ge 5 ] || fail "no blocks with the signer connected (see $LOG)"
signer=$(curl -sf "http://127.0.0.1:26657/commit?height=$((h1 - 1))" | json "d['result']['signed_header']['commit']['signatures'][0]['validator_address']")
[ "$signer" = "$(python3 -c "
import json;print(json.load(open('$SIGNER_DIR/priv_validator_key.json'))['address'])")" ] || fail "commit signed by $signer"
ok "signer + node: blocks produced (height $h1), commits signed by the validator key held only by the signer"

# 4. signer outage = node stops signing (no fallback key); signer back = chain resumes
kill "$SIGNER" || true; SIGNER=
sleep 8; a=$(height 2>/dev/null || echo -1); sleep 6; b=$(height 2>/dev/null || echo -1)
if [ "$b" = "-1" ]; then
  b=$a; ok "signer stopped → node stops (it cannot sign; no fallback key)"
  $BIN start --home "$HOME_DIR" >>"$LOG" 2>&1 & NODE=$!
else
  [ "$b" -le "$((a + 1))" ] || fail "blocks continued without the signer ($a → $b)"
fi
"$REPO/build/remotesigner" --key "$SIGNER_DIR/priv_validator_key.json" --state "$SIGNER_DIR/priv_validator_state.json" \
  --node 127.0.0.1:26659 --chain-id "$CHAIN_ID" >>"$LOG" 2>&1 & SIGNER=$!
for _ in $(seq 1 60); do [ "$(height)" -gt "$((b + 3))" ] && break; sleep 1; done
[ "$(height)" -gt "$((b + 3))" ] || fail "chain did not resume after signer restart"
ok "signer stopped → validator stops signing at $b (no key fallback); signer restarted → chain resumes to $(height)"

# 5. double-sign protection lives with the signer: its state file tracks the last signed height
last=$(python3 -c "import json;print(int(json.load(open('$SIGNER_DIR/priv_validator_state.json'))['height']))")
[ "$last" -ge "$b" ] || fail "signer state height $last"
ok "signer's double-sign state advanced to height $last (kept off the node)"

cleanup; trap - EXIT
# restore a normal localnet
BIN=$BIN "$REPO/scripts/localnet.sh" >/dev/null
echo
echo "$PASSED checks passed"
