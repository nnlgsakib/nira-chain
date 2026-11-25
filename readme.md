# Nira Chain

A sovereign Proof-of-Stake L1:
- **Core Validators** finalise blocks with Tendermint BFT (single-block finality).
- A **native EVM** shares one state with the Cosmos modules: one account and one balance on both interfaces.
- **Permissionless Community Nodes** earn only for cryptographically verified work: random state-possession proofs checked on-chain against committed app hashes. They are never on the finality path.

Project state, requirements, decisions and risks: [`CHAIN.md`](CHAIN.md). Architecture: [`docs/spec/architecture.md`](docs/spec/architecture.md).

## Build

Requires Go 1.26+ and a C compiler (cgo, for go-ethereum's secp256k1).

```bash
make install        # installs `nirad`
nirad version
```

| Item | Value |
|---|---|
| Binary / home | `nirad` / `~/.nira` |
| Token | `anira` (18 decimals), display `NIRA` |
| EVM chain ID | 47201 (constant) |
| Addresses | `nira1…` bech32, or `0x…`: the same eth_secp256k1 key and the same 20 bytes |
| APIs | Ethereum JSON-RPC `:8545`; Cosmos REST `:1317` (`/cosmos/...` standard modules, `/nira/...` Nira modules); gRPC `:9090` |
| Nira modules | `x/community` (REST `/nira/community/v1`, EVM precompile `0x…0900`), `x/policy` (validator power cap, REST `/nira/policy/v1`) |

## Run a local chain

```bash
scripts/localnet.sh && nirad start                 # 1 validator, dev keys dev1/dev2, 20-block community epochs
scripts/genesis/localtest.sh                       # 4 validators via the full genesis ceremony
```

MetaMask: RPC `http://127.0.0.1:8545`, chain ID `47201`, symbol `NIRA`. Dev key: `nirad keys unsafe-export-eth-key dev1 --keyring-backend test`.

## Run a community node

```bash
nirad tx community onion-init                          # local secret; prints the onion tip
nirad tx community register-node my-node 1000000000000000000000anira <tip> 100000 --from <key>
nirad tx community run --from <key>                    # proves assigned tasks every epoch; safe to stop/restart
```

The node must keep at least `apphash_window + epoch_blocks` recent state versions (the default pruning does). From Solidity or an Ethereum wallet, use `precompiles/community/ICommunity.sol`.

## Tests

```bash
go test ./x/... ./app/...                                       # unit + invariants (real IAVL proofs)
python3 -m unittest discover -s tests/genesis -p 'test_*.py'    # genesis builder rules
tests/cosmos-e2e/e2e.sh && tests/cosmos-e2e/community.sh        # Cosmos interface (on a running localnet)
KEY=$(echo y | nirad keys unsafe-export-eth-key dev1 --keyring-backend test)
(cd tests/evm-e2e && npm i && KEY=$KEY node e2e.mjs && KEY=$KEY node community.mjs)   # EVM interface
tests/genesis/check_localtest.sh                                # 4-validator network + BFT fault tolerance
```

## Genesis and launch

The economy (supply, allocations, vesting), the initial Core Validators, the initial Community Nodes and all parameters are defined in `genesis/networks/<network>.toml`. Build the genesis with `scripts/genesis/build_genesis.py`; the runbook is in [`genesis/README.md`](genesis/README.md).

**Status:** in development. Not audited. Not production ready (see CHAIN.md → Risks and Open questions).
