# Nira Chain

> A sovereign PoS L1 where bonded Core Validators finalise blocks with BFT consensus, native EVM runs in the same state, and permissionless Community Nodes earn only for cryptographically verified work.

## Status
- Mode: production
- Phase: build (design and stack ADRs accepted; component specs partial, see Components)
- Last updated: 2026-11-25 by stack-cosmos-ignite
- Next step: tokenomics spec + simulation (protocol-tokenomics, Q3) using genesis/networks values, then threat model (quality-security)

## Idea
Full text: `docs/idea.md`. Hybrid L1: professional Core Validators run Tendermint-style BFT consensus with delegated, non-custodial PoS. Ordinary home machines run Community Nodes that do randomly assigned, verifiable work and never sit on the finality path. Native EVM with Ethereum JSON-RPC shares one state with native modules. Rewards split between capital, validator responsibility and verified community service, with governed issuance.

## Requirements
### Functional
- R-F1 Permissionless validator creation; active set = top-N by bonded power, min self-bond
- R-F2 Non-custodial delegation, redelegation, undelegation with unbonding; rewards + commission
- R-F3 Slashing + jailing for double sign and downtime, propagated to delegators
- R-F4 Per-validator voting-power cap (ADR-0004)
- R-F5 Permissionless Community Node registration with bond
- R-F6 Unpredictable, verifiable task assignment to Community Nodes
- R-F7 Community rewards only for on-chain-verified work
- R-F8 Divergence alerts from Community Nodes, advisory and off the finality path
- R-F9 Ethereum txs (legacy, EIP-2930, EIP-1559), Solidity deploy/call, logs, receipts
- R-F10 Ethereum JSON-RPC (eth_, net_, web3_, filters) on full nodes
- R-F11 One account/key model and one balance for native and EVM
- R-F12 On-chain governance over params, treasury and upgrades; no admin key
- R-F13 State sync snapshots; pruned (community) and archive modes
### Non-functional (proposed unless stated)
- R-N1 Block time ≤ 2 s p50, ≤ 4 s p95 at 100 validators (proposed; benchmark pending)
- R-N2 Finality = 1 block (BFT)
- R-N3 Active validators ≤ 100 at genesis params (governable up to 150)
- R-N4 ≥ 300 TPS simple native/EVM transfers on reference hardware (proposed baseline, benchmark pending)
- R-N5 Community node: 4 cores / 8 GB / 500 GB SSD / 25 Mbps, pruned; state sync to head < 30 min
- R-N6 Community layer outage → 0 impact on block production
### Constraints
- R-C1 Cosmos SDK v0.53 + CometBFT v0.38 Ignite scaffold already exists (Go)
- R-C2 EVM must be in-state, not a sidechain
### Non-goals
- Community nodes voting in consensus; fixed APY promises; custom consensus algorithms; a bridge between native and EVM state

## Architecture
Detail: `docs/spec/architecture.md`
- Architecture class: sovereign PoS L1 + off-finality community participation layer (ADR-0001)
- Node roles: Core Validator, Delegator, Community Node, Full Node, Archive Node
- Trust assumptions: < 1/3 Byzantine power (safety), > 2/3 online (liveness, else halt), weak subjectivity within unbonding period

## Stack
- Framework + versions: Cosmos SDK v0.53.6, CometBFT v0.38.21, ibc-go v10.4.0, cosmos/evm v0.6.0 (go-ethereum → cosmos/go-ethereum v1.16.2-cosmos-1), Go 1.26, EVM chain ID 47201 (proposed, Q2)
- Adapter skill: stack-cosmos-ignite + stack-cosmos-evm
- Decision: ADR-0005

| Component | Surface | Location | ADR |
|---|---|---|---|
| CONS | CometBFT engine | config, genesis consensus params | ADR-0002 |
| STAKE | native x/staking, x/slashing; cap hook | app/, x/policy | ADR-0004 |
| EXEC | x/vm, x/feemarket, x/erc20, precompiles | app/ | ADR-0005 |
| TOKEN | x/mint, x/distribution, community budget | app/, x/community | — |
| COMM | native module | x/community | ADR-0003 |
| GOV | x/gov | app/ | — |

## Components
| Component | protocol-* skill | Spec | Status | Evidence |
|---|---|---|---|---|
| CONS | protocol-consensus | architecture.md §1,6 | tested (4-validator net) | tests/genesis/check_localtest.sh: 1/4 down live, 2/4 down halts, recovery with identical hashes |
| STAKE | protocol-staking-validators | ADR-0004 (spec pending) | tested (unit + localnet) | x/policy/keeper/power_cap_test.go (INV-STAKE-CAP-1..3, randomised 2000 ops); tests/cosmos-e2e (MsgDelegate/Undelegate), tests/evm-e2e (staking precompile delegate/undelegate). Gap: live cap *rejection* needs a ≥10-validator net |
| EXEC | protocol-state-execution | ADR-0005 | tested (localnet) | tests/evm-e2e/e2e.mjs — 9/9 pass: deploy, logs/filters, receipts, revert, EIP-1559 + legacy tx, bank==EVM balance, staking precompile |
| TOKEN | protocol-tokenomics | missing | designed | |
| COMM | protocol-verifiable-participation | docs/spec/community.md v0.2 | tested (unit + localnet, Cosmos and EVM) | x/community/keeper/community_test.go (real IAVL proofs, all C-checks, INV-COMM-1..10 randomised); tests/cosmos-e2e/community.sh 6/6 (CLI client); tests/evm-e2e/community.mjs 9/9 via precompile 0x…0900 with Cosmos REST cross-checks |
| RAND | protocol-randomness | community.md §5 | tested | x/community/types/derive_test.go (spec vectors, INV-COMM-9) |
| GOV | protocol-governance | missing | designed | |
| NET | — (no skill) | missing | todo | |

## Decisions
| ADR | Title | Status |
|---|---|---|
| ADR-0001 | Sovereign PoS L1 with off-finality community layer | accepted |
| ADR-0002 | Tendermint-family BFT, bounded active set | accepted |
| ADR-0003 | Community layer: hash-onion sortition + ICS-23 state proofs | accepted |
| ADR-0004 | Per-validator voting-power cap at delegation time | accepted |
| ADR-0005 | Stack: Cosmos SDK + cosmos/evm via Ignite | accepted |
| ADR-0006 | x/community reachable from Cosmos and EVM (precompile 0x…0900) | accepted |

## Open questions
- [ ] Q7 Live power-cap *rejection* needs ≥ 15 validators (cap activation); 4-validator ceremony + BFT fault tests done (tests/genesis/check_localtest.sh)
- [ ] Q6 Dev bootstrap uses scripts/localnet.sh (eth_secp256k1 keys); `ignite chain serve` creates plain secp256k1 keys that can't sign EVM txs — don't use it for EVM testing
- [ ] Q1 Token supply, genesis allocation and vesting — now data-driven: edit genesis/networks/<net>.toml (template: mainnet.example.toml, all values proposed). Decision still needed from the user before mainnet
- [ ] Q2 EVM chain ID for mainnet/testnet (blocking for testnet; default proposed in EVM wiring)
- [ ] Q3 Reward split 50/25/15/10 from the idea: validators vs delegators is set by commission in the F1 model, not by protocol. Needs a tokenomics spec (protocol-tokenomics)
- [ ] Q4 v2 community task types (history serving, DA sampling) — design-spec
- [ ] Q5 Formal model of x/community settlement (quality-formal-verification)

## Risks
| ID | Risk | Likelihood | Impact | Mitigation | Owner skill |
|---|---|---|---|---|---|
| K1 | Community nodes outsource state proofs to public archive RPCs | high | medium | Short proof window, v2 local-data tasks; documented limitation | protocol-verifiable-participation |
| K2 | Proposer biases the epoch seed | low | low | Hash-onion per-node reveal; reward-fairness impact only | protocol-randomness |
| K3 | One operator runs several validators to bypass the cap | medium | high | Monitoring, operator disclosure, delegator UI | protocol-staking-validators |
| K4 | > 1/3 stake on one cloud provider | medium | high | Diversity monitoring, validator docs | ops-infrastructure |
| K5 | cosmos/evm wiring bugs (precompile + staking hook interaction) | medium | high | Found + fixed: transient stores are invisible inside EVM precompiles (hooks must use KV stores). Both-interface e2e suites run per change | quality-testing |
| K6 | Base fee decays to ~0 on empty blocks and feemarket min_gas_price = 0 → near-free spam | high | medium | Network template sets params.feemarket.min_gas_price = 10 gwei (proposed); operators must set minimum-gas-prices (genesis/README.md) | protocol-state-execution |
| K8 | ICS-23 can't prove non-membership next to empty-value leaves (collections KeySets) or in empty stores; sample stores must avoid them | medium | medium | Defaults acc/evm/distribution/slashing (200/200 verified live); tools/proofprobe check before changing; SDK upgrades may add KeySets to these stores → re-probe per upgrade | protocol-verifiable-participation |
| K7 | EVM mempool accepts eth_sendRawTransaction, then silently drops txs that fail CheckTx (client waits forever) | medium | medium | Surface CheckTx errors to RPC callers; e2e per-check timeouts added | stack-cosmos-evm |

## Genesis
Network definition (supply, allocations, vesting, initial validators and community nodes, all economic params): `genesis/networks/<net>.toml` → `scripts/genesis/build_genesis.py check|pregenesis|finalize`. Runbook: `genesis/README.md`.

## How to test (both interfaces)
`scripts/localnet.sh && nirad start` then `tests/cosmos-e2e/e2e.sh`, `tests/cosmos-e2e/community.sh`, and with `KEY=$(echo y | nirad keys unsafe-export-eth-key dev1 --keyring-backend test)`: `node tests/evm-e2e/e2e.mjs`, `node tests/evm-e2e/community.mjs`; unit: `go test ./x/... ./app/...`

## Production readiness
Not started. See docs/readiness.md (to be written by ops-production-readiness).
