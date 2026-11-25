# Genesis — allocations, initial operators and the launch ceremony

The initial economy and the initial operators are **data, not code**. Everything that may still change before launch lives in one reviewable file per network:

```
genesis/networks/<network>.toml      ← edit this
scripts/genesis/build_genesis.py     ← validates and builds from it
build/genesis/                       ← generated: pregenesis.json, genesis.json, genesis.sha256, report.md
```

Start from `networks/mainnet.example.toml`. Every value in it is a proposal (CHAIN.md Q1/Q3), and every address in it is a placeholder.

## What the network file controls

| Section | What it is | Notes |
|---|---|---|
| `[chain]` | chain-id, genesis time | chain-id must be `nira_47201-<n>`; the EVM chain ID 47201 is compiled in |
| `[supply] total` | total NIRA at genesis | the builder requires every anira to be allocated exactly once |
| `[[allocation]]` | named buckets: `kind = "account"` (optionally `vesting`) or `kind = "community_pool"` (protocol reserve, spent only by governance) | vesting is linear from `start` to `end`. Omitting `start` gives a single unlock at `end`. A cliff plus linear unlock = `start` at the cliff date |
| `[[validator]]` | initial Core Validators: moniker, operator address, balance, `self_bond` | each must submit a gentx with exactly `self_bond` |
| `[[community_node]]` | initial Community Nodes: operator, bond (escrowed in x/community), liquid balance, onion tip and length | active from epoch 1; the operator keeps the onion secret |
| `[params.<module>]` | economic and policy parameters for any module (mint, distribution, staking, slashing, gov, feemarket, nirachain, community, …) | `"X NIRA"` strings convert to anira; unknown fields are rejected |
| `[app_state.…]` | raw overrides outside module params (e.g. `mint.minter.inflation`) | |
| `[consensus]` | consensus params overrides | `block.max_gas` defaults to 100 000 000 |
| `[rules]` | launch safety rules: `min_validators`, `max_genesis_power_share` | |

Addresses may be bech32 (`nira1…`) or hex (`0x…`). They are the same 20 bytes, and the builder catches duplicates across both forms.

## Commands

```bash
# 1. Validate and see the allocation report (supply table, %, vesting, voting power, Nakamoto coefficient)
python3 scripts/genesis/build_genesis.py check genesis/networks/mainnet.toml

# 2. Build the pregenesis (accounts + params, no validators yet) and publish it with its sha256
python3 scripts/genesis/build_genesis.py pregenesis genesis/networks/mainnet.toml --out build/genesis

# 3. Each validator, on their own machine, with the pregenesis as config/genesis.json:
nira-chaind genesis gentx <key> <self_bond>anira --chain-id nira_47201-1 --moniker core-1 \
  --commission-rate 0.10 --commission-max-rate 0.20 --commission-max-change-rate 0.01 \
  --output-document core-1.json
#    … and sends core-1.json to the coordinator (public data, no secrets)

# 4. Finalize: rebuild deterministically, verify every gentx against the network file, collect, validate
python3 scripts/genesis/build_genesis.py finalize genesis/networks/mainnet.toml --gentx-dir gentxs/ --out build/genesis

# 5. Publish build/genesis/genesis.json + genesis.sha256; every operator checks the hash before starting
```

Community node operators send their onion tip: `nira-chaind tx community onion-init` prints it, and the secret stays in `<home>/community/onion.json`.

## What the builder refuses

- Allocations that don't sum to `supply.total` exactly
- An address used twice (bech32 and hex forms are compared)
- Fewer than `rules.min_validators` validators, or any validator at or above `rules.max_genesis_power_share` of genesis voting power (it could halt the chain alone)
- `self_bond` greater than the validator's balance
- A gentx that is missing, duplicated, for an unlisted validator, or with a self-bond different from the file
- Community node bonds below `params.community.min_bond`; invalid onion tips
- Parameter fields that don't exist in genesis (typos fail loudly)

Warnings, which don't block the build: validators that start above the voting-power cap (normal with few genesis validators).

## Rehearsal: full ceremony with 4 validators on one machine

```bash
scripts/genesis/localtest.sh           # keys → network file → pregenesis → 4 gentxs → finalize → start 4 nodes
tests/genesis/check_localtest.sh       # allocations (Cosmos + EVM), vesting, pool, genesis community node,
                                       # 1/4 down = live, 2/4 down = halt (no fork), recovery
scripts/genesis/localtest.sh stop
python3 -m unittest discover -s tests/genesis -p 'test_*.py'   # builder validation rules
```

Node i listens on the base port + 100·i: CometBFT RPC 26657, REST 1317, JSON-RPC 8545. The existing e2e suites run against node 0 with `HOME_DIR=~/.nira-localtest/val0 CHAIN_ID=nira_47201-77`.

## Before mainnet

- Replace every placeholder address; collect addresses from their owners through a signed, out-of-band channel.
- Decide the economics (Q1/Q3) and record the decision as an ADR. `check` produces the report to review.
- Set `params.feemarket.min_gas_price`, and make every operator set `minimum-gas-prices` in app.toml (risk K6).
- Freeze the network file in a tagged commit; the genesis must be reproducible from that commit plus the gentxs.
