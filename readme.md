# Nira Chain

**A sovereign Proof-of-Stake Layer 1 where professional validators secure consensus, Ethereum contracts run natively, and ordinary computers earn rewards for verifiable work.**

> **Status: in development.** Runs as local and multi-validator test networks. Not audited and not production ready. See [docs/STATUS.md](docs/STATUS.md).

## What it is

| Layer | What | Built on |
|---|---|---|
| **Consensus** | Core Validators run Tendermint BFT: single-block finality, halts instead of forking, top-N stake-weighted active set | CometBFT 0.38 (unmodified) |
| **Staking** | Non-custodial delegation, unbonding, slashing; a **per-validator voting-power cap** so no validator grows unbounded on delegations | Cosmos SDK x/staking + Nira `x/policy` |
| **Execution** | One state machine: native modules **and a native EVM** share accounts, balances and blocks. One key works in MetaMask and the Cosmos CLI | Cosmos SDK 0.53 + cosmos/evm 0.6 |
| **Community Nodes** | Permissionless, bonded nodes are picked at random each epoch and submit **Merkle proofs of chain state**, verified on-chain. Paid only for verified work; never needed for finality | Nira `x/community` + EVM precompile `0x…0900` |
| **Governance** | On-chain proposals control parameters, treasury (community pool) and upgrades; no admin keys | Cosmos SDK x/gov |

Design documents: [architecture](docs/spec/architecture.md) · [community layer spec](docs/spec/community.md) · [decisions (ADRs)](docs/decisions/) · [project state, risks, open questions](CHAIN.md) · [original vision](docs/idea.md)

## Quick start

```bash
# prerequisites: Go 1.26.7+, gcc/clang, make, python3 (Node.js 18+ for EVM tests). Details: docs/INSTALL.md
git clone https://github.com/nnlgsakib/nira-chain.git && cd nira-chain
make install                 # builds and installs `nirad`
make localnet                # 1-validator dev chain in ~/.nira
nirad start
```

Then:
- **MetaMask:** network RPC `http://127.0.0.1:8545`, chain ID `47201`, symbol `NIRA`. Dev key: `nirad keys unsafe-export-eth-key dev1 --keyring-backend test` (test key, never use with real funds).
- **Cosmos CLI:** `nirad q bank balances $(nirad keys show dev1 -a --keyring-backend test)`
- **Explore:** `nirad q community epoch`, `nirad q policy params`, `curl localhost:1317/nira/community/v1/nodes`

## Chain facts

| | |
|---|---|
| Binary / home | `nirad` / `~/.nira` |
| Token | `NIRA`, 18 decimals; base unit `anira` (1 NIRA = 10¹⁸ anira) |
| EVM chain ID | `47201` (fixed) |
| Accounts | eth_secp256k1. The same 20 bytes as `nira1…` (bech32) or `0x…`. HD path m/44'/60' |
| Ethereum JSON-RPC | `:8545` (HTTP), `:8546` (WS) |
| Cosmos APIs | REST `:1317`, gRPC `:9090`, CometBFT RPC `:26657`. Standard modules under `/cosmos/...`, Nira modules under `/nira/community/v1`, `/nira/policy/v1` |
| Precompiles | staking `0x…0800`, distribution `0x…0801`, bank `0x…0804`, gov `0x…0805`, slashing `0x…0806`, **community `0x…0900`** ([ICommunity.sol](precompiles/community/ICommunity.sol)) |

## Documentation

| Doc | For |
|---|---|
| [docs/INSTALL.md](docs/INSTALL.md) | Prerequisites per OS, build, make targets, troubleshooting |
| [docs/OPERATIONS.md](docs/OPERATIONS.md) | Running validators, full/archive/RPC nodes and community nodes; ports; systemd; security |
| [genesis/README.md](genesis/README.md) | Defining the economy and initial operators; the genesis ceremony |
| [docs/STATUS.md](docs/STATUS.md) | Completed work, test evidence, known vulnerabilities, remaining work |
| [CHAIN.md](CHAIN.md) | Requirements, components, decisions, open questions, risks |

## Repository layout

```
app/                 app wiring (modules, EVM, ante handlers, precompile registration)
cmd/nirad/           node and CLI binary
x/community/         community participation layer (keeper, client CLI, types)
x/policy/            chain policy: validator voting-power cap (staking hooks)
precompiles/community/  EVM precompile + Solidity interface for x/community
proto/nira/          protobuf APIs of the Nira modules
genesis/             network definition files (economy, initial operators) + ceremony runbook
scripts/             localnet, genesis builder, 4-validator ceremony
tests/               Cosmos e2e (bash), EVM e2e (ethers.js), genesis tests
tools/proofprobe/    diagnostic: which stores yield verifiable state proofs
docs/                spec, ADRs, install/operations/status
```

## Testing

```bash
make test                         # go vet + unit/invariant tests + genesis builder tests
make localnet && nirad start      # then, in another terminal:
make e2e-deps && make e2e         # Cosmos + EVM end-to-end (32 checks, with cross-interface checks)
make localtest && make localtest-check && make localtest-stop   # 4 validators + BFT fault tolerance
```

## Where it stands

**Done and tested:**
- native EVM with Ethereum JSON-RPC,
- non-custodial staking from both the Cosmos and EVM sides,
- the validator voting-power cap,
- the community layer end to end (registration, random selection, real state proofs, rewards, unbonding, alerts) from both interfaces,
- a declarative genesis builder and ceremony,
- a 4-validator network that keeps running with 1/4 down and halts without forking with 2/4 down.

**Next:** economic simulation, threat model and audit preparation, slashing/rotation/partition tests at ≥ 15 validators, governance and upgrade rehearsals, performance benchmarks, state sync. Full list: [docs/STATUS.md](docs/STATUS.md#remaining-work-prioritised).

## License

Apache License 2.0: see [LICENSE](LICENSE) and [NOTICE](NOTICE). Dependencies keep their own licenses (Cosmos SDK, CometBFT and cosmos/evm Apache-2.0; ibc-go MIT; go-ethereum library code LGPL-3.0).
